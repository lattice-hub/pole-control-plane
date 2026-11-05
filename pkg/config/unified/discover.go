package unified

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/store"
)

func requireCursor(observed, minimum *configv1.Cursor) error {
	if observed == nil || observed.Scope == "" || observed.Epoch == "" {
		return errors.New("store returned no observation cursor")
	}
	if minimum == nil {
		return nil
	}
	if minimum.Scope == "" || minimum.Epoch == "" {
		return invalid("min_cursor requires scope and epoch")
	}
	if observed.Scope != minimum.Scope || observed.Epoch != minimum.Epoch {
		return &domainError{code: model.Code_ConfigCursorReset, message: "cursor scope or epoch changed; perform a full read"}
	}
	if observed.Position < minimum.Position {
		return errors.New("store has not observed the requested cursor")
	}
	return nil
}

// selectRelease 只从一个完整快照中选择：priority ASC、sequence DESC，未命中灰度才回退正式。
func selectRelease(state *store.UnifiedConfigAggregate, labels map[string]string) (*configv1.ConfigRelease, error) {
	if state.GetOffline() {
		return nil, nil
	}
	var normal, selected *configv1.ConfigRelease
	seen := make(map[uint64]bool)
	for _, member := range state.GetMembers() {
		if !member.GetActive() {
			continue
		}
		release := member.GetRelease()
		if release == nil || release.Id == "" || release.Sequence == 0 || seen[release.Sequence] || !proto.Equal(release.Key, state.Key) {
			return nil, errors.New("invalid release set snapshot")
		}
		seen[release.Sequence] = true
		if err := validateRollout(release.Kind, release.Audience, release.Priority); err != nil {
			return nil, errors.New("invalid release rollout")
		}
		switch release.Kind {
		case configv1.ReleaseKind_RELEASE_KIND_NORMAL:
			if normal != nil {
				return nil, errors.New("multiple active normal releases")
			}
			normal = release
		case configv1.ReleaseKind_RELEASE_KIND_GRAY:
			if !matches(release.Audience, labels) {
				continue
			}
			if selected == nil || release.Priority < selected.Priority || (release.Priority == selected.Priority && release.Sequence > selected.Sequence) {
				selected = release
			}
		default:
			return nil, errors.New("invalid release kind")
		}
	}
	if selected != nil {
		return selected, nil
	}
	return normal, nil
}

func renderSnapshot(ctx context.Context, definition *configv1.ConfigDefinition, labels map[string]string) (*configv1.RenderSnapshot, error) {
	if err := validateDefinition(ctx, definition); err != nil {
		return nil, err
	}
	values, err := effectiveValues(definition, labels)
	if err != nil {
		return nil, err
	}
	// 克隆公开输入，既不泄露其他规则，也不共享存储对象的可变引用。
	snapshot := &configv1.RenderSnapshot{Content: definition.Content, Engine: definition.Engine, Format: definition.Format, Parameters: definition.Parameters, Values: values}
	return proto.Clone(snapshot).(*configv1.RenderSnapshot), nil
}

func configResult(ctx context.Context, key *configv1.ConfigKey, incarnation string, release *configv1.ConfigRelease, labels map[string]string, cursor *configv1.Cursor) *model.Response {
	result := &configv1.ConfigResult{Key: key, Presence: configv1.ConfigPresence_CONFIG_PRESENCE_ABSENT, ObservationCursor: cursor}
	var identity []byte
	var err error
	if release == nil {
		identity, err = json.Marshal([]any{"absent", key, incarnation})
	} else {
		snapshot, failure := renderSnapshot(ctx, release.Definition, labels)
		if failure != nil {
			return errorResponse(failure)
		}
		result.Presence = configv1.ConfigPresence_CONFIG_PRESENCE_PRESENT
		result.ReleaseId = release.Id
		result.Snapshot = snapshot
		// 只绑定交付快照，不绑定未选规则、兼容开关和全局游标。
		identity, err = json.Marshal([]any{key, incarnation, release.Id, snapshot})
	}
	if err != nil {
		return errorResponse(err)
	}
	revision := sha256.Sum256(identity)
	result.SnapshotRevision = hex.EncodeToString(revision[:])
	return successResponse(result)
}

// Discover 授权后从一致快照选版、取值，原生客户端在本地渲染。
func (s *Service) Discover(ctx context.Context, req *configv1.DiscoverRequest) *model.Response {
	return s.discover(ctx, req, "")
}

// DiscoverForProtocol 与原生读取共用选版权限和游标；选中版本未开放协议时拒绝，绝不回落。
// 这里只取得渲染输入；入口是否启用与实际渲染由适配器负责。
func (s *Service) DiscoverForProtocol(ctx context.Context, req *configv1.DiscoverRequest, protocol string) *model.Response {
	switch protocol {
	case "apollo", "nacos", "consul":
	default:
		return errorResponse(invalid("unsupported compatibility protocol"))
	}
	return s.discover(ctx, req, protocol)
}
func (s *Service) discover(ctx context.Context, req *configv1.DiscoverRequest, protocol string) *model.Response {
	if _, err := s.authorize(ctx, req.GetKey(), ActionRead); err != nil {
		return errorResponse(err)
	}
	if err := validateLabels(req.GetLabels()); err != nil {
		return errorResponse(err)
	}
	var snapshot *store.UnifiedConfigSnapshot
	var err error
	if runtime, ok := s.storage.(store.UnifiedConfigRuntimeReader); ok {
		snapshot, err = runtime.ReadUnifiedConfigRuntime(ctx, req.GetKey())
	} else {
		snapshot, err = s.storage.ReadUnifiedConfig(ctx, req.GetKey())
	}
	if err != nil {
		return errorResponse(err)
	}
	if snapshot == nil || snapshot.State == nil {
		return errorResponse(errors.New("store returned no snapshot"))
	}
	if err := requireCursor(snapshot.Cursor, req.GetMinCursor()); err != nil {
		return errorResponse(err)
	}
	release, err := selectRelease(snapshot.State, req.GetLabels())
	if err != nil {
		return errorResponse(err)
	}
	if release != nil && protocol != "" {
		compatibility := release.GetDefinition().GetCompatibility()
		enabled := protocol == "apollo" && compatibility.GetApollo() || protocol == "nacos" && compatibility.GetNacos() || protocol == "consul" && compatibility.GetConsul()
		if !enabled {
			return errorResponse(&domainError{code: model.Code_ClientAPINotOpen, message: "selected configuration does not enable this protocol"})
		}
	}
	if err := ctx.Err(); err != nil {
		return errorResponse(err)
	}
	incarnation := snapshot.State.Incarnation
	if snapshot.State.ReleaseSetRevision == 0 {
		incarnation = ""
	}
	return configResult(ctx, req.GetKey(), incarnation, release, req.GetLabels(), snapshot.Cursor)
}

// Preview 只检查候选定义并选值；不保存、不产生发布身份，也不检查最终输出格式。
func (s *Service) Preview(ctx context.Context, req *configv1.PreviewRequest) *model.Response {
	if _, err := s.authorize(ctx, req.GetKey(), ActionPreview); err != nil {
		return errorResponse(err)
	}
	if err := validateLabels(req.GetLabels()); err != nil {
		return errorResponse(err)
	}
	snapshot, err := renderSnapshot(ctx, req.GetDefinition(), req.GetLabels())
	if err != nil {
		return errorResponse(err)
	}
	return successResponse(snapshot)
}
