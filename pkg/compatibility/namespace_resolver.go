package compatibility

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/pkg/types"
	storeapi "github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/common/utils"
	"github.com/pole-io/pole-server/pkg/systemconfig"
)

const (
	Component = "pole-server"
	Domain    = "compatibility"

	NacosPublicNamespaceTargetKey = "server.compatibility.nacos.public_namespace_target"
	EurekaDefaultNamespaceKey     = "server.compatibility.eureka.default_namespace_target"
)

type Snapshot struct {
	Revision            int64
	NacosPublicTarget   string
	EurekaDefaultTarget string
	Source              systemconfig.SourceDescriptor
	ApplyStatus         string
	ApplyMessage        string
	AppliedAt           time.Time
}

type NamespaceLookup interface {
	GetNamespace(name string) (*types.Namespace, error)
}

type Resolver struct {
	current atomic.Pointer[Snapshot]
	mu      sync.Mutex
	static  Snapshot
}

func NewNamespaceResolver(nacosTarget, eurekaTarget string) *Resolver {
	nacosTarget = defaultTarget(nacosTarget)
	eurekaTarget = defaultTarget(eurekaTarget)
	resolver := &Resolver{static: Snapshot{
		NacosPublicTarget: nacosTarget, EurekaDefaultTarget: eurekaTarget,
		Source:      systemconfig.SourceDescriptor{Kind: systemconfig.SourceStaticFile, Reference: "apiservers"},
		ApplyStatus: "static", ApplyMessage: "当前使用 API Server 启动配置",
	}}
	initial := resolver.static
	resolver.current.Store(&initial)
	return resolver
}

func defaultTarget(value string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return "dev"
}

func (r *Resolver) Current() Snapshot {
	if r == nil || r.current.Load() == nil {
		return Snapshot{NacosPublicTarget: "dev", EurekaDefaultTarget: "dev", ApplyStatus: "static"}
	}
	return *r.current.Load()
}

func (r *Resolver) ConfigureNacosStatic(target string) {
	r.configureStatic(func(snapshot *Snapshot) { snapshot.NacosPublicTarget = defaultTarget(target) })
}

func (r *Resolver) ConfigureEurekaStatic(target string) {
	r.configureStatic(func(snapshot *Snapshot) { snapshot.EurekaDefaultTarget = defaultTarget(target) })
}

func (r *Resolver) configureStatic(update func(*Snapshot)) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	update(&r.static)
	if r.Current().Revision == 0 {
		next := r.static
		r.current.Store(&next)
	}
}

func (r *Resolver) ResolveNacos(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "public" {
		return r.Current().NacosPublicTarget
	}
	return raw
}

func (r *Resolver) ToNacosDiscovery(namespace string) string {
	if namespace == r.Current().NacosPublicTarget {
		return "public"
	}
	return namespace
}

func (r *Resolver) ToNacosConfig(namespace string) string {
	if namespace == r.Current().NacosPublicTarget {
		return ""
	}
	return namespace
}

func (r *Resolver) ResolveEureka(header string) string {
	if header = strings.TrimSpace(header); header != "" {
		return header
	}
	return r.Current().EurekaDefaultTarget
}

func (r *Resolver) IsEffectiveTarget(namespace string) bool {
	current := r.Current()
	return namespace == current.NacosPublicTarget || namespace == current.EurekaDefaultTarget
}

func (r *Resolver) Apply(revision int64, values map[string]any, namespaces NamespaceLookup) error {
	targets, err := ValidateValues(values, namespaces)
	if err != nil {
		r.Reject(revision, err)
		return err
	}
	next := Snapshot{
		Revision: revision, NacosPublicTarget: targets.NacosPublicTarget,
		EurekaDefaultTarget: targets.EurekaDefaultTarget,
		Source: systemconfig.SourceDescriptor{Kind: systemconfig.SourceDynamicRelease,
			Reference: fmt.Sprintf("%s/%s@%d", Component, Domain, revision)},
		ApplyStatus: "applied", ApplyMessage: fmt.Sprintf("动态版本 %d 已生效", revision),
		AppliedAt: time.Now().UTC(),
	}
	r.current.Store(&next)
	return nil
}

func (r *Resolver) Reject(revision int64, cause error) {
	current := r.Current()
	current.ApplyStatus = "rejected"
	current.ApplyMessage = fmt.Sprintf("动态版本 %d 被拒绝：%v；继续使用上一次有效配置", revision, cause)
	r.current.Store(&current)
}

type Targets struct {
	NacosPublicTarget   string
	EurekaDefaultTarget string
}

func ValidateValues(values map[string]any, namespaces NamespaceLookup) (Targets, error) {
	if namespaces == nil {
		return Targets{}, errors.New("namespace lookup is unavailable")
	}
	targets := Targets{
		NacosPublicTarget:   stringValue(values[NacosPublicNamespaceTargetKey]),
		EurekaDefaultTarget: stringValue(values[EurekaDefaultNamespaceKey]),
	}
	checks := []struct {
		label string
		value string
	}{
		{label: "Nacos public namespace target", value: targets.NacosPublicTarget},
		{label: "Eureka default namespace target", value: targets.EurekaDefaultTarget},
	}
	for _, check := range checks {
		if check.value == "" {
			return Targets{}, fmt.Errorf("%s cannot be empty", check.label)
		}
		namespace, err := namespaces.GetNamespace(check.value)
		if err != nil {
			return Targets{}, fmt.Errorf("validate %s %q: %w", check.label, check.value, err)
		}
		if namespace == nil || !namespace.Valid {
			return Targets{}, fmt.Errorf("%s %q does not exist", check.label, check.value)
		}
		if namespace.Kind != apimodel.NamespaceKind_NAMESPACE_KIND_BUSINESS {
			return Targets{}, fmt.Errorf("%s %q must be a BUSINESS namespace", check.label, check.value)
		}
	}
	return targets, nil
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

type Runtime struct {
	resolver   *Resolver
	releases   storeapi.SystemConfigRuntimeStore
	namespaces NamespaceLookup
	instanceID string
	lastSeen   int64
}

func NewRuntime(resolver *Resolver, releases storeapi.SystemConfigRuntimeStore,
	namespaces NamespaceLookup) *Runtime {
	hostname, _ := os.Hostname()
	return &Runtime{
		resolver: resolver, releases: releases, namespaces: namespaces,
		instanceID: strings.TrimSpace(hostname) + ":" + utils.NewUUID(),
	}
}

func (r *Runtime) Reconcile() error {
	if r == nil || r.resolver == nil || r.releases == nil {
		return nil
	}
	release, err := r.releases.GetActiveSystemConfigRelease(Component, Domain)
	if err != nil {
		return err
	}
	if release == nil {
		return nil
	}
	if release.Revision != r.lastSeen {
		values := map[string]any{}
		if err = json.Unmarshal(release.Payload, &values); err == nil {
			err = r.resolver.Apply(release.Revision, values, r.namespaces)
		} else {
			r.resolver.Reject(release.Revision, err)
		}
		if err == nil {
			r.lastSeen = release.Revision
		}
	}
	current := r.resolver.Current()
	receipt := storeapi.SystemConfigApplyReceipt{
		Component: Component, Domain: Domain, Revision: release.Revision,
		InstanceID: r.instanceID, State: current.ApplyStatus, Message: current.ApplyMessage,
	}
	if current.ApplyStatus == "applied" && current.Revision == release.Revision {
		receipt.AppliedAt = current.AppliedAt
	}
	if receiptErr := r.releases.UpsertSystemConfigApplyReceipt(receipt); receiptErr != nil && err == nil {
		err = receiptErr
	}
	return err
}

func (r *Runtime) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	_ = r.Reconcile()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.Reconcile()
		}
	}
}

type resolverContextKey struct{}

func WithResolver(ctx context.Context, resolver *Resolver) context.Context {
	return context.WithValue(ctx, resolverContextKey{}, resolver)
}

func ResolverFromContext(ctx context.Context) (*Resolver, bool) {
	resolver, ok := ctx.Value(resolverContextKey{}).(*Resolver)
	return resolver, ok
}
