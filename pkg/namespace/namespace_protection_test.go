package namespace

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	globalotel "go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	cacheapi "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/pkg/types"
	svctypes "github.com/pole-io/pole-server/apis/pkg/types/service"
	cachemock "github.com/pole-io/pole-server/pkg/cache/mock"
	"github.com/pole-io/pole-server/pkg/compatibility"
	storemock "github.com/pole-io/pole-server/plugin/store/mock"
)

func TestDeleteNamespaceRejectsProtectedNamespaces(t *testing.T) {
	t.Parallel()

	server := &Server{compatibilityResolver: compatibility.NewNamespaceResolver("dev", "dev")}
	for _, name := range []string{"dev", SystemNamespace} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			response := server.DeleteNamespace(context.Background(), &apimodel.Namespace{Name: name})

			require.Equal(t, uint32(apimodel.Code_InvalidParameter), response.GetCode())
			require.Contains(t, response.GetInfo(), "built-in namespace cannot be deleted")
		})
	}
}

func TestIsProtectedNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		protected bool
	}{
		{name: DefaultNamespace, protected: false},
		{name: SystemNamespace, protected: true},
		{name: ProductionNamespace, protected: false},
		{name: "DEFAULT", protected: false},
		{name: "", protected: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.protected, isProtectedNamespace(test.name))
		})
	}
}

func TestDeleteNamespacesRejectsProtectedNamespaces(t *testing.T) {
	t.Parallel()

	server := &Server{compatibilityResolver: compatibility.NewNamespaceResolver("dev", "dev")}
	response := server.DeleteNamespaces(context.Background(), []*apimodel.Namespace{
		{Name: "dev"},
		{Name: SystemNamespace},
	})

	require.Equal(t, uint32(apimodel.Code_InvalidParameter), response.GetCode())
	require.Len(t, response.GetResponses(), 2)
	for _, item := range response.GetResponses() {
		require.Equal(t, uint32(apimodel.Code_InvalidParameter), item.GetCode())
		require.Contains(t, item.GetInfo(), "built-in namespace cannot be deleted")
	}
}

func TestDeleteNamespaceRejectsOwnedGovernanceRules(t *testing.T) {
	controller := gomock.NewController(t)
	storage := storemock.NewMockStore(controller)
	tx := storemock.NewMockTransaction(controller)
	server := &Server{storage: storage}

	storage.EXPECT().CreateTransaction().Return(tx, nil)
	tx.EXPECT().Commit().Return(nil)
	tx.EXPECT().LockNamespace("prod").Return(&types.Namespace{Name: "prod"}, nil)
	storage.EXPECT().GetServices(map[string]string{"namespace": "prod"}, nil, nil, uint32(0), uint32(1)).Return(uint32(0), nil, nil)
	storage.EXPECT().CountAIResourcesByNamespace("prod").Return(uint32(0), nil)
	storage.EXPECT().CountUnifiedConfigs(gomock.Any(), "prod").Return(uint64(0), nil)
	storage.EXPECT().CountGovernanceRules("prod").Return(uint64(2), nil)

	response := server.DeleteNamespace(context.Background(), &apimodel.Namespace{Name: "prod"})

	require.Equal(t, uint32(apimodel.Code_NamespaceExistedGovernanceRules), response.GetCode())
	require.Contains(t, response.GetInfo(), "governance rules")
}

func TestDeleteNamespaceRejectsOwnedAIResources(t *testing.T) {
	controller := gomock.NewController(t)
	storage := storemock.NewMockStore(controller)
	tx := storemock.NewMockTransaction(controller)
	server := &Server{storage: storage}

	storage.EXPECT().CreateTransaction().Return(tx, nil)
	tx.EXPECT().Commit().Return(nil)
	tx.EXPECT().LockNamespace("prod").Return(&types.Namespace{Name: "prod"}, nil)
	storage.EXPECT().GetServices(
		map[string]string{"namespace": "prod"}, nil, nil, uint32(0), uint32(1),
	).Return(uint32(0), nil, nil)
	storage.EXPECT().CountAIResourcesByNamespace("prod").Return(uint32(2), nil)

	response := server.DeleteNamespace(context.Background(), &apimodel.Namespace{Name: "prod"})

	require.Equal(t, uint32(apimodel.Code_ExistedResource), response.GetCode())
	require.Contains(t, response.GetInfo(), "MCP Server or A2A Agent")
}

func TestCreateNamespaceRejectsSystemKindAndReservedName(t *testing.T) {
	t.Parallel()

	server := &Server{}
	for _, request := range []*apimodel.Namespace{
		{Name: "business-name", Kind: apimodel.NamespaceKind_NAMESPACE_KIND_SYSTEM},
		{Name: "unknown-kind", Kind: apimodel.NamespaceKind(2)},
		{Name: SystemNamespace, Kind: apimodel.NamespaceKind_NAMESPACE_KIND_BUSINESS},
	} {
		response := server.CreateNamespace(context.Background(), request)

		require.Equal(t, uint32(apimodel.Code_InvalidParameter), response.GetCode())
		require.Contains(t, response.GetInfo(), "system namespace")
	}
}

func TestCreateNamespaceIfAbsentAcceptsExistingSystemNamespace(t *testing.T) {
	controller := gomock.NewController(t)
	cacheManager := cachemock.NewMockCacheManager(controller)
	namespaceCache := cachemock.NewMockNamespaceCache(controller)
	server := &Server{caches: cacheManager}

	cacheManager.EXPECT().Namespace().Return(namespaceCache)
	namespaceCache.EXPECT().GetNamespace(SystemNamespace).Return(&types.Namespace{
		Name: SystemNamespace,
		Kind: apimodel.NamespaceKind_NAMESPACE_KIND_SYSTEM,
	})

	name, response := server.CreateNamespaceIfAbsent(context.Background(), &apimodel.Namespace{
		Name: SystemNamespace,
	})

	require.Equal(t, SystemNamespace, name)
	require.NotNil(t, response)
	require.Equal(t, uint32(apimodel.Code_ExecuteSuccess), response.GetCode())
}

func TestCreateNamespacePersistsBusinessKindByDefault(t *testing.T) {
	controller := gomock.NewController(t)
	storage := storemock.NewMockStore(controller)
	server := &Server{storage: storage}

	storage.EXPECT().GetNamespace("development").Return(nil, nil)
	storage.EXPECT().AddNamespace(gomock.Any()).DoAndReturn(func(namespace *types.Namespace) error {
		require.Equal(t, apimodel.NamespaceKind_NAMESPACE_KIND_BUSINESS, namespace.Kind)
		return nil
	})

	response := server.CreateNamespace(context.Background(), &apimodel.Namespace{Name: "development"})

	require.Equal(t, uint32(apimodel.Code_ExecuteSuccess), response.GetCode())
}

func TestGetNamespacesReturnsKindsAndAppliesKindFilter(t *testing.T) {
	previousProvider := globalotel.GetTracerProvider()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	globalotel.SetTracerProvider(provider)
	t.Cleanup(func() {
		globalotel.SetTracerProvider(previousProvider)
	})

	controller := gomock.NewController(t)
	storage := storemock.NewMockStore(controller)
	cacheManager := cachemock.NewMockCacheManager(controller)
	namespaceCache := cachemock.NewMockNamespaceCache(controller)
	serviceCache := cachemock.NewMockServiceCache(controller)
	server := &Server{storage: storage, caches: cacheManager}
	filter := map[string][]string{"kind": {"system"}}

	cacheManager.EXPECT().Namespace().Return(namespaceCache)
	namespaceCache.EXPECT().Query(gomock.Any(), &cacheapi.NamespaceArgs{
		Filter: filter,
		Offset: 0,
		Limit:  10,
	}).Return(uint32(1), []*types.Namespace{{
		Name: SystemNamespace,
		Kind: apimodel.NamespaceKind_NAMESPACE_KIND_SYSTEM,
	}}, nil)
	storage.EXPECT().CountUnifiedConfigsByEnvironment(gomock.Any()).Return(map[string]uint64{}, nil)
	cacheManager.EXPECT().Service().Return(serviceCache)
	serviceCache.EXPECT().GetNamespaceCntInfo(SystemNamespace).Return(svctypes.NamespaceServiceCount{
		InstanceCnt: &svctypes.InstanceCount{},
	})

	ctx, requestSpan := provider.Tracer("test").Start(context.Background(), "GET:/core/v1/namespaces")
	response := server.GetNamespaces(ctx, map[string][]string{
		"kind":   {"system"},
		"offset": {"0"},
		"limit":  {"10"},
	})
	requestSpan.End()

	require.Equal(t, uint32(apimodel.Code_ExecuteSuccess), response.GetCode())
	require.Len(t, response.GetData(), 1)
	namespace := &apimodel.Namespace{}
	require.NoError(t, response.GetData()[0].UnmarshalTo(namespace))
	require.Equal(t, apimodel.NamespaceKind_NAMESPACE_KIND_SYSTEM, namespace.GetKind())
	require.False(t, namespace.GetDeleteable())

	spans := recorder.Ended()
	require.Len(t, spans, 3)
	var serviceSpan sdktrace.ReadOnlySpan
	var storeSpan sdktrace.ReadOnlySpan
	for _, span := range spans {
		switch span.Name() {
		case "service.namespace.get":
			serviceSpan = span
		case "store.config.count_by_namespace":
			storeSpan = span
		}
	}
	require.NotNil(t, serviceSpan)
	require.NotNil(t, storeSpan)
	require.Equal(t, requestSpan.SpanContext().SpanID(), serviceSpan.Parent().SpanID())
	require.Equal(t, serviceSpan.SpanContext().SpanID(), storeSpan.Parent().SpanID())
}
