package builtinplugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pole-io/pole-server/apis"
	authapi "github.com/pole-io/pole-server/apis/access_control/auth"
	"github.com/pole-io/pole-server/apis/apiserver"
	storeapi "github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pluginapi"
)

type externalAPIServer struct{}

func (e *externalAPIServer) GetProtocol() string { return "external" }
func (e *externalAPIServer) GetPort() uint32     { return 0 }
func (e *externalAPIServer) Initialize(context.Context, map[string]interface{},
	map[string]apiserver.APIConfig) error {
	return nil
}
func (e *externalAPIServer) Run(chan error) {}
func (e *externalAPIServer) Stop()          {}
func (e *externalAPIServer) Restart(map[string]interface{}, map[string]apiserver.APIConfig,
	chan error) error {
	return nil
}

func TestNewRegistryContainsOfficialPlugins(t *testing.T) {
	registry, err := NewRegistry()
	require.NoError(t, err)

	assert.Len(t, registry.Descriptors(pluginapi.KindAPIServer), 7)
	assert.Len(t, registry.Descriptors(pluginapi.KindStore), 1)
	assert.Len(t, registry.Descriptors(pluginapi.KindAuthUser), 1)
	assert.Len(t, registry.Descriptors(pluginapi.KindAuthStrategy), 1)
	assert.Len(t, registry.Descriptors(pluginapi.KindHealthCheck), 3)
	assert.Len(t, registry.Descriptors(pluginapi.KindStatis), 3)
	assert.Len(t, registry.Descriptors(pluginapi.KindHistory), 3)
	assert.Len(t, registry.Descriptors(pluginapi.KindDiscoverEvent), 3)
	assert.Len(t, registry.Descriptors(pluginapi.KindCrypto), 2)
	assert.Len(t, registry.Descriptors(pluginapi.KindCMDB), 1)
	assert.Len(t, registry.Descriptors(pluginapi.KindRateLimit), 1)
	assert.Len(t, registry.Descriptors(pluginapi.KindWhitelist), 1)
}

func TestRegisterRejectsDuplicateBuiltinSet(t *testing.T) {
	registry, err := NewRegistry()
	require.NoError(t, err)

	assert.ErrorContains(t, Register(registry), "already registered")
}

func TestOfficialAndExternalPluginsShareRuntimeRegistry(t *testing.T) {
	registry, err := NewRegistry()
	require.NoError(t, err)
	require.NoError(t, apiserver.RegisterFactory(registry, pluginapi.Descriptor{
		Name:   "external",
		Origin: pluginapi.OriginExternal,
	}, func() (apiserver.Apiserver, error) {
		return &externalAPIServer{}, nil
	}))
	registry.Freeze()
	restore, err := pluginapi.Activate(registry)
	require.NoError(t, err)
	t.Cleanup(restore)

	firstStore, err := storeapi.ResolveStore("defaultStore")
	require.NoError(t, err)
	secondStore, err := storeapi.ResolveStore("defaultStore")
	require.NoError(t, err)
	assert.Same(t, firstStore, secondStore)

	userServer, err := authapi.ResolveUserServer(authapi.DefaultUserMgnPluginName)
	require.NoError(t, err)
	sameUserServer, err := authapi.ResolveUserServer(authapi.DefaultUserMgnPluginName)
	require.NoError(t, err)
	assert.Same(t, userServer, sameUserServer)

	statisPlugin, exists := apis.GetPlugin(apis.PluginTypeStatis, "otel")
	require.True(t, exists)
	sameStatisPlugin, exists := apis.GetPlugin(apis.PluginTypeStatis, "otel")
	require.True(t, exists)
	assert.Same(t, statisPlugin, sameStatisPlugin)

	externalServer, err := apiserver.Resolve("external")
	require.NoError(t, err)
	sameExternalServer, err := apiserver.Resolve("external")
	require.NoError(t, err)
	assert.Same(t, externalServer, sameExternalServer)
}
