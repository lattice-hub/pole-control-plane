package sqldb

import (
	"context"
	"database/sql"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/pole-io/pole-server/pkg/config/unified"
	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
)

// 此测试仅使用隔离实例中的临时子库，分别验证 MySQL 重连、历史和业务回滚。
func TestUnifiedAudienceMySQLPersistence(t *testing.T) {
	dsn := os.Getenv("UNIFIED_CONFIG_TEST_DSN")
	if dsn == "" {
		t.Skip("set UNIFIED_CONFIG_TEST_DSN to an isolated local MySQL test database")
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	require.NoError(t, err)
	host, _, err := net.SplitHostPort(cfg.Addr)
	require.NoError(t, err)
	ip := net.ParseIP(host)
	require.True(t, cfg.Net == "tcp" && ip != nil && ip.IsLoopback())
	require.True(t, strings.HasPrefix(cfg.DBName, "unified_config_test_"))
	cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 5*time.Second, 10*time.Second, 10*time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	require.NoError(t, err)
	t.Cleanup(func() { admin.Close() })
	require.NoError(t, admin.PingContext(ctx))
	database := "unified_config_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.ExecContext(ctx, "CREATE DATABASE `"+database+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_bin")
	require.NoError(t, err)
	t.Cleanup(func() { _, err := admin.Exec("DROP DATABASE `" + database + "`"); require.NoError(t, err) })
	cfg.DBName = database
	newStore := func() *stableStore {
		db, err := sql.Open("mysql", cfg.FormatDSN())
		require.NoError(t, err)
		require.NoError(t, db.PingContext(ctx))
		t.Cleanup(func() { db.Close() })
		return &stableStore{master: &BaseDB{DB: db}}
	}
	first := newStore()
	require.NoError(t, ensureUnifiedConfigSchema(first.master))
	_, err = first.master.DB.ExecContext(ctx, `CREATE TABLE namespace (name VARCHAR(64) NOT NULL PRIMARY KEY, comment VARCHAR(1024), token VARCHAR(64) NOT NULL, owner VARCHAR(1024) NOT NULL, flag TINYINT NOT NULL DEFAULT 0, ctime TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, mtime TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, service_export_to TEXT, metadata TEXT, kind TINYINT NOT NULL DEFAULT 0) ENGINE=InnoDB`)
	require.NoError(t, err)
	_, err = first.master.DB.ExecContext(ctx, `INSERT INTO namespace (name,token,owner) VALUES ('dev','','owner')`)
	require.NoError(t, err)
	svc, err := unified.New(first, staleNamespaceAuthorization{})
	require.NoError(t, err)
	unpack := func(r *apimodel.Response, m proto.Message) {
		t.Helper()
		require.Equal(t, uint32(apimodel.Code_ExecuteSuccess), r.Code, "%v", r)
		require.NotNil(t, r.Data)
		require.NoError(t, r.Data.UnmarshalTo(m))
	}
	cases := []struct {
		typ              configv1.MatchType
		value, hit, miss string
	}{
		{configv1.MatchType_MATCH_TYPE_EXACT, "*", "*", "x"}, {configv1.MatchType_MATCH_TYPE_REGEX, "a+", "ba", "x"},
		{configv1.MatchType_MATCH_TYPE_NOT_EQUALS, "x", "y", "x"}, {configv1.MatchType_MATCH_TYPE_IN, "a,,b", "", "c"},
		{configv1.MatchType_MATCH_TYPE_NOT_IN, "a,,b", "c", ""}, {configv1.MatchType_MATCH_TYPE_RANGE, "-2~4", "4", "5"},
	}
	for _, tc := range cases {
		t.Run(tc.typ.String(), func(t *testing.T) {
			key := &configv1.ConfigKey{Environment: "dev", Group: "app", Name: tc.typ.String()}
			get := func(s *unified.Service) *configv1.ConfigState {
				state := &configv1.ConfigState{}
				unpack(s.GetState(ctx, &configv1.GetStateRequest{Key: key}), state)
				return state
			}
			save := func(def *configv1.ConfigDefinition) {
				s := get(svc)
				unpack(svc.SaveDraft(ctx, &configv1.SaveDraftRequest{Key: key, Definition: def, ExpectedDraftRevision: s.GetDraft().GetRevision()}), &configv1.ConfigState{})
			}
			publish := func(kind configv1.ReleaseKind, audience *configv1.Audience, replace string) string {
				s := get(svc)
				out := &configv1.ConfigState{}
				unpack(svc.Publish(ctx, &configv1.PublishRequest{Key: key, Kind: kind, Audience: audience, ReplaceReleaseId: replace, ExpectedDraftRevision: s.Draft.Revision, ExpectedReleaseSetRevision: s.ReleaseSetRevision}), out)
				return out.Releases[0].Id
			}
			save(&configv1.ConfigDefinition{Engine: "literal", Content: []byte("normal")})
			publish(configv1.ReleaseKind_RELEASE_KIND_NORMAL, nil, "")
			cond := &configv1.LabelCondition{Key: "cohort", Type: tc.typ, Value: tc.value}
			audience := &configv1.Audience{Conditions: []*configv1.LabelCondition{cond}}
			def := &configv1.ConfigDefinition{Engine: "pole-mustache-v1", Format: "text", Content: []byte("{{{v}}}"), Parameters: []*configv1.Parameter{{Name: "v", Type: configv1.ValueType_VALUE_TYPE_STRING, Required: true}}, DefaultValues: map[string]*configv1.Scalar{"v": {Type: configv1.ValueType_VALUE_TYPE_STRING, Value: "default"}}, ValueRules: []*configv1.ValueRule{{Id: "rule", Audience: audience, Values: map[string]*configv1.Scalar{"v": {Type: configv1.ValueType_VALUE_TYPE_STRING, Value: "selected"}}}}}
			save(def)
			old := publish(configv1.ReleaseKind_RELEASE_KIND_GRAY, audience, "")
			save(&configv1.ConfigDefinition{Engine: "literal", Content: []byte("replacement")})
			replacement := publish(configv1.ReleaseKind_RELEASE_KIND_GRAY, &configv1.Audience{Labels: map[string]string{"cohort": "legacy"}}, old)
			// 新建连接和服务，读取存储的草稿、历史完整 protobuf，避免沿用写入连接的状态。
			fresh := newStore()
			reader, err := unified.New(fresh, staleNamespaceAuthorization{})
			require.NoError(t, err)
			historic := &configv1.ConfigState{}
			unpack(reader.GetState(ctx, &configv1.GetStateRequest{Key: key, ReleaseId: old}), historic)
			require.True(t, proto.Equal(audience, historic.SelectedRelease.Audience))
			require.True(t, proto.Equal(def, historic.SelectedRelease.Definition))
			legacyResult := &configv1.ConfigResult{}
			unpack(reader.Discover(ctx, &configv1.DiscoverRequest{Key: key, Labels: map[string]string{"cohort": "legacy"}}), legacyResult)
			require.Equal(t, "replacement", string(legacyResult.Snapshot.Content))
			state := get(reader)
			unpack(reader.Rollback(ctx, &configv1.RollbackRequest{Key: key, SourceReleaseId: old, ReplaceReleaseId: replacement, ExpectedReleaseSetRevision: state.ReleaseSetRevision}), &configv1.ConfigState{})
			for _, check := range []struct {
				labels map[string]string
				hit    bool
			}{{map[string]string{"cohort": tc.hit}, true}, {map[string]string{"cohort": tc.miss}, false}, {nil, false}} {
				result := &configv1.ConfigResult{}
				unpack(reader.Discover(ctx, &configv1.DiscoverRequest{Key: key, Labels: check.labels}), result)
				preview := &configv1.RenderSnapshot{}
				unpack(reader.Preview(ctx, &configv1.PreviewRequest{Key: key, Definition: def, Labels: check.labels}), preview)
				if check.hit {
					require.Equal(t, "selected", preview.Values["v"].Value)
					require.True(t, proto.Equal(preview, result.Snapshot))
				} else {
					require.Equal(t, "default", preview.Values["v"].Value)
					require.Equal(t, "normal", string(result.Snapshot.Content))
				}
			}
			legacy := &configv1.ConfigState{}
			unpack(reader.GetState(ctx, &configv1.GetStateRequest{Key: key, ReleaseId: replacement}), legacy)
			require.Equal(t, map[string]string{"cohort": "legacy"}, legacy.SelectedRelease.Audience.Labels)
		})
	}
}
