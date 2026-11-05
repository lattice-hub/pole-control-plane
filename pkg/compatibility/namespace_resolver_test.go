package compatibility

import (
	"encoding/json"
	"errors"
	"testing"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	"github.com/stretchr/testify/require"

	"github.com/pole-io/pole-server/apis/pkg/types"
	storeapi "github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/systemconfig"
)

type namespaceLookupStub struct {
	values map[string]*types.Namespace
	err    error
}

func (s namespaceLookupStub) GetNamespace(name string) (*types.Namespace, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.values[name], nil
}

func businessNamespaces(names ...string) namespaceLookupStub {
	values := make(map[string]*types.Namespace, len(names))
	for _, name := range names {
		values[name] = &types.Namespace{Name: name, Valid: true,
			Kind: apimodel.NamespaceKind_NAMESPACE_KIND_BUSINESS}
	}
	return namespaceLookupStub{values: values}
}

func TestNamespaceResolverKeepsExplicitNamespacesAndHotReloadsDefaults(t *testing.T) {
	resolver := NewNamespaceResolver("dev", "dev")

	require.Equal(t, "dev", resolver.ResolveNacos(""))
	require.Equal(t, "dev", resolver.ResolveNacos("public"))
	require.Equal(t, "lane", resolver.ResolveNacos("lane"))
	require.Equal(t, "dev", resolver.ResolveEureka(""))
	require.Equal(t, "lane", resolver.ResolveEureka("lane"))

	err := resolver.Apply(7, map[string]any{
		NacosPublicNamespaceTargetKey: "tst",
		EurekaDefaultNamespaceKey:     "pre",
	}, businessNamespaces("tst", "pre"))
	require.NoError(t, err)
	require.Equal(t, "tst", resolver.ResolveNacos("public"))
	require.Equal(t, "pre", resolver.ResolveEureka(""))
	require.Equal(t, "lane", resolver.ResolveNacos("lane"))
	require.Equal(t, systemconfig.SourceDynamicRelease, resolver.Current().Source.Kind)
	require.Equal(t, "applied", resolver.Current().ApplyStatus)
}

func TestNamespaceResolverRejectsInvalidReleaseAndKeepsLastKnownGood(t *testing.T) {
	resolver := NewNamespaceResolver("dev", "dev")
	require.NoError(t, resolver.Apply(1, map[string]any{
		NacosPublicNamespaceTargetKey: "dev",
		EurekaDefaultNamespaceKey:     "dev",
	}, businessNamespaces("dev")))

	err := resolver.Apply(2, map[string]any{
		NacosPublicNamespaceTargetKey: "missing",
		EurekaDefaultNamespaceKey:     "dev",
	}, businessNamespaces("dev"))
	require.ErrorContains(t, err, "does not exist")
	require.Equal(t, "dev", resolver.ResolveNacos("public"))
	require.Equal(t, int64(1), resolver.Current().Revision)
	require.Equal(t, "rejected", resolver.Current().ApplyStatus)
}

type runtimeStoreStub struct {
	release  *storeapi.SystemConfigRelease
	receipts []storeapi.SystemConfigApplyReceipt
	err      error
}

func (s *runtimeStoreStub) GetActiveSystemConfigRelease(_, _ string) (*storeapi.SystemConfigRelease, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.release, nil
}

func (s *runtimeStoreStub) UpsertSystemConfigApplyReceipt(receipt storeapi.SystemConfigApplyReceipt) error {
	s.receipts = append(s.receipts, receipt)
	return nil
}

func TestRuntimeReconcileAppliesReleaseAndWritesReceipt(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		NacosPublicNamespaceTargetKey: "dev",
		EurekaDefaultNamespaceKey:     "dev",
	})
	require.NoError(t, err)
	store := &runtimeStoreStub{release: &storeapi.SystemConfigRelease{Revision: 9, Payload: payload}}
	resolver := NewNamespaceResolver("dev", "dev")
	runtime := NewRuntime(resolver, store, businessNamespaces("dev"))

	require.NoError(t, runtime.Reconcile())
	require.Equal(t, int64(9), resolver.Current().Revision)
	require.Len(t, store.receipts, 1)
	require.Equal(t, "applied", store.receipts[0].State)
	require.False(t, store.receipts[0].AppliedAt.IsZero())

	store.err = errors.New("database unavailable")
	require.ErrorContains(t, runtime.Reconcile(), "database unavailable")
	require.Equal(t, int64(9), resolver.Current().Revision)
}
