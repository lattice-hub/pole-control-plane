package config

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/pkg/types"
	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
	"github.com/pole-io/pole-server/apis/store"
	storemock "github.com/pole-io/pole-server/plugin/store/mock"
)

// assemblyStore 只隔离持久化，不替换 Initialize 装配的实际 authorizer 和领域入口。
type assemblyStore struct {
	store.Store
	reads atomic.Int32
	actor string
}

func (s *assemblyStore) ReadUnifiedConfig(_ context.Context, key *configv1.ConfigKey) (*store.UnifiedConfigSnapshot, error) {
	s.reads.Add(1)
	return &store.UnifiedConfigSnapshot{State: &store.UnifiedConfigAggregate{Key: proto.Clone(key).(*configv1.ConfigKey)}, Cursor: &configv1.Cursor{Scope: "config", Epoch: "epoch"}}, nil
}
func (s *assemblyStore) UpdateUnifiedConfig(ctx context.Context, key *configv1.ConfigKey, mutate store.UnifiedConfigMutation) (*store.UnifiedConfigSnapshot, error) {
	snapshot, err := s.ReadUnifiedConfig(ctx, key)
	if err != nil {
		return nil, err
	}
	change, err := mutate(snapshot.State)
	if err != nil {
		return nil, err
	}
	if change != nil {
		s.actor = change.Actor
	}
	return snapshot, nil
}
func (s *assemblyStore) ReadUnifiedConfigEvents(context.Context, *configv1.Cursor, uint32) ([]*store.UnifiedConfigEvent, *configv1.Cursor, error) {
	return nil, nil, errors.New("unexpected events read")
}
func preserveConfigRuntime(t *testing.T) {
	t.Helper()
	runtime.Lock()
	saved := runtime.service
	runtime.service = nil
	runtime.Unlock()
	t.Cleanup(func() { runtime.Lock(); runtime.service = saved; runtime.Unlock() })
}
func newAssemblyStore(t *testing.T) *assemblyStore {
	t.Helper()
	return &assemblyStore{Store: storemock.NewMockStore(gomock.NewController(t))}
}

func TestInitializeUnifiedServiceUsesRealCredentialAndPermissionAssembly(t *testing.T) {
	preserveConfigRuntime(t)
	f := newAuthFixture(t)
	repository := newAssemblyStore(t)
	require.NoError(t, Initialize(context.Background(), Config{Open: true}, repository, f.caches, f.users, f.checker))
	service, err := GetUnifiedService()
	require.NoError(t, err)
	f.environment(businessNamespace())
	credential := f.users.EXPECT().CheckCredential(gomock.Any()).DoAndReturn(authenticated)
	f.checker.EXPECT().CheckClientPermission(gomock.Any()).After(credential).Return(true, nil)
	ctx := context.WithValue(context.Background(), types.ContextOperator, "forged-staff")
	response := service.Discover(ctx, &configv1.DiscoverRequest{Key: f.key, Labels: map[string]string{"Staffname": "admin", "principal": "admin"}})
	require.Equal(t, uint32(apimodel.Code_ExecuteSuccess), response.Code)
	result := &configv1.ConfigResult{}
	require.NoError(t, response.Data.UnmarshalTo(result))
	require.Equal(t, configv1.ConfigPresence_CONFIG_PRESENCE_ABSENT, result.Presence)
	require.Equal(t, int32(1), repository.reads.Load())
	f.environment(businessNamespace())
	credential = f.users.EXPECT().CheckCredential(gomock.Any()).DoAndReturn(authenticated)
	f.checker.EXPECT().CheckConsolePermission(gomock.Any()).After(credential).Return(true, nil)
	response = service.SaveDraft(ctx, &configv1.SaveDraftRequest{Key: f.key, Definition: &configv1.ConfigDefinition{Engine: "literal", Content: []byte("test")}})
	require.Equal(t, uint32(apimodel.Code_ExecuteSuccess), response.Code)
	require.Equal(t, verifiedPrincipal.String(), repository.actor)
}

func TestInitializedUnifiedServiceRejectsLabelsAndStaffnameWithoutCredentials(t *testing.T) {
	for _, test := range []struct {
		name       string
		credential error
		code       apimodel.Code
	}{{"explicit refusal", authtypes.ErrorTokenInvalid, apimodel.Code_AuthTokenForbidden}, {"dependency failure", errors.New("sensitive-token in unavailable identity database"), apimodel.Code_ConfigUnavailable}} {
		t.Run(test.name, func(t *testing.T) {
			preserveConfigRuntime(t)
			f := newAuthFixture(t)
			repository := newAssemblyStore(t)
			require.NoError(t, Initialize(context.Background(), Config{Open: true}, repository, f.caches, f.users, f.checker))
			service, err := GetUnifiedService()
			require.NoError(t, err)
			f.environment(businessNamespace())
			f.users.EXPECT().CheckCredential(gomock.Any()).Return(test.credential)
			ctx := context.WithValue(context.Background(), types.ContextOperator, "Staffname-root")
			response := service.Discover(ctx, &configv1.DiscoverRequest{Key: f.key, Labels: map[string]string{"principal": "root", "owner": "owner"}})
			require.Equal(t, uint32(test.code), response.Code)
			require.Nil(t, response.Data)
			require.NotContains(t, response.Info, "sensitive-token")
			require.Zero(t, repository.reads.Load())
		})
	}
}

func TestInitializeOpenFalseAndFailedReinitializationCloseService(t *testing.T) {
	preserveConfigRuntime(t)
	f := newAuthFixture(t)
	repository := newAssemblyStore(t)
	ctx := context.Background()
	require.NoError(t, Initialize(ctx, Config{Open: true}, repository, f.caches, f.users, f.checker))
	_, err := GetUnifiedService()
	require.NoError(t, err)
	require.NoError(t, Initialize(ctx, Config{Open: false}, nil, nil, nil, nil))
	_, err = GetUnifiedService()
	require.Error(t, err)
	require.NoError(t, Initialize(ctx, Config{Open: true}, repository, f.caches, f.users, f.checker))
	require.Error(t, Initialize(ctx, Config{Open: true}, storemock.NewMockStore(gomock.NewController(t)), f.caches, f.users, f.checker))
	_, err = GetUnifiedService()
	require.Error(t, err)
	require.NoError(t, Initialize(ctx, Config{Open: true}, repository, f.caches, f.users, f.checker))
	require.Error(t, Initialize(ctx, Config{Open: true}, repository, nil, f.users, f.checker))
	_, err = GetUnifiedService()
	require.Error(t, err)
}

func TestUnifiedAuthorizerAndInitializationRejectMissingDependencies(t *testing.T) {
	preserveConfigRuntime(t)
	f := newAuthFixture(t)
	_, err := NewUnifiedAuthorizer(nil, f.users, f.checker)
	require.Error(t, err)
	_, err = NewUnifiedAuthorizer(f.caches, nil, f.checker)
	require.Error(t, err)
	_, err = NewUnifiedAuthorizer(f.caches, f.users, nil)
	require.Error(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, Initialize(ctx, Config{Open: true}, newAssemblyStore(t), f.caches, f.users, f.checker), context.Canceled)
	_, err = f.authorizer.Authorize(ctx, f.key, "read")
	require.ErrorIs(t, err, context.Canceled)
}
