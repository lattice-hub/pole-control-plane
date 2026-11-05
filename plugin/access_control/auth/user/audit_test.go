package defaultuser

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"

	"github.com/pole-io/pole-server/apis/pkg/types"
	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
	commonlog "github.com/pole-io/pole-server/pkg/common/log"
	storemock "github.com/pole-io/pole-server/plugin/store/mock"
)

func TestUserAuditExcludesCredentialsWithoutChangingRequest(t *testing.T) {
	request := &apisecurity.User{Id: "id", Name: "audit-user", Password: "audit-password-sentinel",
		AuthToken: "audit-token-sentinel", Comment: "legitimate comment", TokenEnable: true,
		Metadata: map[string]string{"team": "platform", "nested": `{"Password":"nested-secret-sentinel","name":"visible"}`}}
	original := proto.Clone(request)
	for _, operation := range []types.OperationType{types.OCreate, types.OUpdate, types.ODelete, types.OUpdateToken} {
		entry := userRecordEntry(context.Background(), request, &authtypes.User{ID: "id", Name: "audit-user"}, operation)
		require.NotContains(t, entry.Detail, "sentinel")
		require.Contains(t, entry.Detail, "legitimate comment")
		require.Contains(t, entry.Detail, "platform")
		require.Contains(t, entry.Detail, "visible")
		require.Equal(t, operation, entry.OperationType)
		require.Equal(t, "audit-user(id)", entry.ResourceName)
		require.True(t, proto.Equal(original, request))
	}
}

func TestUpdateGroupNoChangeDoesNotLogCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	scope := commonlog.RegisterScope(t.Name(), "isolated synthetic audit test", 0)
	require.NoError(t, commonlog.Configure(map[string]*commonlog.Options{t.Name(): {
		OutputPaths: []string{path}, ErrorOutputPaths: []string{"stderr"}, JSONEncoding: true,
	}}))
	previousLog := log
	log = scope
	t.Cleanup(func() { log = previousLog })
	storage := storemock.NewMockStore(gomock.NewController(t))
	storage.EXPECT().GetGroup("group-id").Return(&authtypes.UserGroupDetail{
		UserGroup: &authtypes.UserGroup{ID: "group-id", Name: "team", TokenEnable: true},
		UserIds:   map[string]struct{}{},
	}, nil)
	server := &Server{storage: storage, authOpt: DefaultUserConfig()}
	request := &apisecurity.UserGroup{Id: "group-id", Name: "team", AuthToken: "no-change-token-sentinel"}
	response := server.UpdateGroup(context.Background(), request)
	require.Equal(t, uint32(apimodel.Code_NoNeedUpdate), response.GetCode())
	require.NoError(t, scope.Sync())
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(contents), "update group data no change")
	require.Contains(t, string(contents), "group-id")
	require.NotContains(t, string(contents), "sentinel")
	require.Equal(t, "no-change-token-sentinel", request.AuthToken)
}

func TestGroupAuditExcludesNestedUserCredentials(t *testing.T) {
	request := &apisecurity.UserGroup{Name: "team", AuthToken: "group-token-sentinel", Relation: &apisecurity.UserGroupRelation{
		Users: []*apisecurity.User{{Id: "member-id", Password: "member-password-sentinel", AuthToken: "member-token-sentinel"}},
	}}
	original := proto.Clone(request)
	model := &authtypes.UserGroup{ID: "group-id", Name: "team"}
	entries := []*types.RecordEntry{
		userGroupRecordEntry(context.Background(), request, model, types.OCreate),
		modifyUserGroupRecordEntry(context.Background(), request, model, types.OUpdateGroup),
		userRelationRecordEntry(context.Background(), request.Relation, model, types.OUpdateGroup),
	}
	for _, entry := range entries {
		require.NotContains(t, entry.Detail, "sentinel")
		require.Contains(t, entry.Detail, "member-id")
	}
	require.True(t, proto.Equal(original, request))
}
