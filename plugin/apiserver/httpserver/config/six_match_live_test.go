package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/config/unified"
	grpcv1 "github.com/pole-io/pole-server/plugin/apiserver/grpcserver/discover/v1"
	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	configgrpc "github.com/pole-io/specification/source/go/api/v1/config_manage"
	"github.com/pole-io/specification/source/go/api/v1/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// sixMatchStore 仅是事务/身份的隔离测试替身；HTTP、gRPC、选版、选值均运行正式实现。
// 深拷贝使失败回调不改变已提交状态；不用于验证 SQL 持久性。
type sixMatchStore struct {
	mu     sync.Mutex
	states map[string]*store.UnifiedConfigAggregate
	cursor *configv1.Cursor
}

func sixMatchIdentity(k *configv1.ConfigKey) string {
	return fmt.Sprintf("%q/%q/%q", k.Environment, k.Group, k.Name)
}
func (m *sixMatchStore) state(k *configv1.ConfigKey) *store.UnifiedConfigAggregate {
	if s := m.states[sixMatchIdentity(k)]; s != nil {
		return s.Clone()
	}
	return &store.UnifiedConfigAggregate{Key: proto.Clone(k).(*configv1.ConfigKey)}
}
func (m *sixMatchStore) ReadUnifiedConfig(ctx context.Context, k *configv1.ConfigKey) (*store.UnifiedConfigSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &store.UnifiedConfigSnapshot{State: m.state(k), Cursor: proto.Clone(m.cursor).(*configv1.Cursor)}, ctx.Err()
}
func (m *sixMatchStore) UpdateUnifiedConfig(ctx context.Context, k *configv1.ConfigKey, mutate store.UnifiedConfigMutation) (*store.UnifiedConfigSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	next := m.state(k)
	change, err := mutate(next)
	if err != nil {
		return nil, err
	}
	if change != nil {
		m.states[sixMatchIdentity(k)] = next.Clone()
		if change.Runtime {
			m.cursor.Position++
		}
	}
	return &store.UnifiedConfigSnapshot{State: next.Clone(), Cursor: proto.Clone(m.cursor).(*configv1.Cursor)}, nil
}
func (m *sixMatchStore) ReadUnifiedConfigEvents(context.Context, *configv1.Cursor, uint32) ([]*store.UnifiedConfigEvent, *configv1.Cursor, error) {
	return nil, nil, errors.New("test fixture does not implement event watching")
}

type sixMatchAuthorizer struct{}

func (sixMatchAuthorizer) Authorize(ctx context.Context, k *configv1.ConfigKey, _ unified.Action) (string, error) {
	if k.Environment != "ui-audit-six" {
		return "", unified.ErrAccessDenied
	}
	return "isolated-ui-audit", ctx.Err()
}

type sixMatchCase struct {
	name             string
	typ              configv1.MatchType
	value, hit, miss string
}

var sixMatchCases = []sixMatchCase{
	{"exact", configv1.MatchType_MATCH_TYPE_EXACT, "cn", "cn", "us"},
	{"regex", configv1.MatchType_MATCH_TYPE_REGEX, "^cn-[0-9]+$", "cn-12", "us-12"},
	{"not-equals", configv1.MatchType_MATCH_TYPE_NOT_EQUALS, "blocked", "allowed", "blocked"},
	{"in", configv1.MatchType_MATCH_TYPE_IN, "cn,us", "us", "jp"},
	{"not-in", configv1.MatchType_MATCH_TYPE_NOT_IN, "cn,us", "jp", "cn"},
	{"range", configv1.MatchType_MATCH_TYPE_RANGE, "10~20", "15", "21"},
}

type sixMatchFixture struct {
	http    *httptest.Server
	stream  configgrpc.ConfigGRPC_DiscoverClient
	service *unified.Service
}

func newSixMatchFixture(t *testing.T, addr string) *sixMatchFixture {
	t.Helper()
	core, err := unified.New(&sixMatchStore{states: map[string]*store.UnifiedConfigAggregate{}, cursor: &configv1.Cursor{Scope: "config", Epoch: "isolated-six-match"}}, sixMatchAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	container := restful.NewContainer()
	container.Add((&HTTPServer{unifiedService: core}).GetUnifiedAccessServer(true, nil))
	server := httptest.NewUnstartedServer(container)
	if addr != "" {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		server.Listener = listener
	}
	server.Start()
	t.Cleanup(server.Close)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	rpc := grpc.NewServer()
	configgrpc.RegisterConfigGRPCServer(rpc, grpcv1.NewConfigGRPCServer(grpcv1.WithUnifiedConfigServer(core)))
	go rpc.Serve(listener)
	t.Cleanup(rpc.Stop)
	connection, err := grpc.Dial(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	stream, err := configgrpc.NewConfigGRPCClient(connection).Discover(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return &sixMatchFixture{http: server, stream: stream, service: core}
}
func (f *sixMatchFixture) post(t *testing.T, action string, input proto.Message, status int, out proto.Message) {
	t.Helper()
	body, err := protojson.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Post(f.http.URL+"/config/v1/"+action, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != status {
		t.Fatalf("%s HTTP %d want %d: %s", action, response.StatusCode, status, payload)
	}
	var envelope model.Response
	if err := protojson.Unmarshal(payload, &envelope); err != nil {
		t.Fatal(err)
	}
	if status != 200 {
		if envelope.Data != nil {
			t.Fatal("failure leaked snapshot")
		}
		return
	}
	if envelope.GetCode() != uint32(model.Code_ExecuteSuccess) {
		t.Fatalf("failure %s", payload)
	}
	if out != nil {
		if err := envelope.Data.UnmarshalTo(out); err != nil {
			t.Fatal(err)
		}
	}
}
func sixMatchDefinition(name string, a *configv1.Audience) *configv1.ConfigDefinition {
	d := &configv1.ConfigDefinition{Content: []byte("mode: {{{mode}}}"), Engine: "pole-mustache-v1", Format: "yaml", Parameters: []*configv1.Parameter{{Name: "mode", Type: configv1.ValueType_VALUE_TYPE_STRING, Required: true}}, DefaultValues: map[string]*configv1.Scalar{"mode": {Type: configv1.ValueType_VALUE_TYPE_STRING, Value: name}}}
	if a != nil {
		d.ValueRules = []*configv1.ValueRule{{Id: "matched", Priority: 0, Audience: a, Values: map[string]*configv1.Scalar{"mode": {Type: configv1.ValueType_VALUE_TYPE_STRING, Value: "matched-" + name}}}}
	}
	return d
}
func (f *sixMatchFixture) seed(t *testing.T, tc sixMatchCase) (*configv1.ConfigKey, *configv1.ConfigDefinition, string, string) {
	t.Helper()
	key := &configv1.ConfigKey{Environment: "ui-audit-six", Group: "audit", Name: tc.name + ".yaml"}
	var state configv1.ConfigState
	f.post(t, "draft", &configv1.SaveDraftRequest{Key: key, Definition: sixMatchDefinition("normal", nil)}, 200, &state)
	f.post(t, "publish", &configv1.PublishRequest{Key: key, ExpectedDraftRevision: state.Draft.Revision, ExpectedReleaseSetRevision: state.ReleaseSetRevision, Kind: configv1.ReleaseKind_RELEASE_KIND_NORMAL}, 200, &state)
	normal := state.Releases[0].Id
	audience := &configv1.Audience{Conditions: []*configv1.LabelCondition{{Key: "client", Type: tc.typ, Value: tc.value}}}
	definition := sixMatchDefinition(tc.name, audience)
	f.post(t, "draft", &configv1.SaveDraftRequest{Key: key, Definition: definition, ExpectedDraftRevision: state.Draft.Revision}, 200, &state)
	f.post(t, "publish", &configv1.PublishRequest{Key: key, ExpectedDraftRevision: state.Draft.Revision, ExpectedReleaseSetRevision: state.ReleaseSetRevision, Kind: configv1.ReleaseKind_RELEASE_KIND_GRAY, Audience: audience, Priority: 1}, 200, &state)
	return key, definition, normal, state.Releases[0].Id
}

// TestSixMatchHTTPGRPC 经真实 JSON 管理端点及真正的 Discover 双向流断言命中版本与值，非前端模拟。
func TestSixMatchHTTPGRPC(t *testing.T) {
	f := newSixMatchFixture(t, "")
	for _, tc := range sixMatchCases {
		t.Run(tc.name, func(t *testing.T) {
			key, definition, normal, gray := f.seed(t, tc)
			for _, sample := range []struct {
				name               string
				labels             map[string]string
				id, value, preview string
			}{{"hit", map[string]string{"client": tc.hit}, gray, "matched-" + tc.name, "matched-" + tc.name}, {"miss", map[string]string{"client": tc.miss}, normal, "normal", tc.name}, {"missing", nil, normal, "normal", tc.name}} {
				var preview configv1.RenderSnapshot
				f.post(t, "preview", &configv1.PreviewRequest{Key: key, Definition: definition, Labels: sample.labels}, 200, &preview)
				if preview.Values["mode"].Value != sample.preview {
					t.Fatalf("preview %s got %q", sample.name, preview.Values["mode"].Value)
				}
				if err := f.stream.Send(&configv1.DiscoverRequest{Key: key, Labels: sample.labels}); err != nil {
					t.Fatal(err)
				}
				response, err := f.stream.Recv()
				if err != nil {
					t.Fatal(err)
				}
				var result configv1.ConfigResult
				if err := response.GetData().UnmarshalTo(&result); err != nil {
					t.Fatal(err)
				}
				if result.ReleaseId != sample.id || result.Snapshot.Values["mode"].Value != sample.value {
					t.Fatalf("%s wrong selected release/value: %v", sample.name, &result)
				}
				t.Logf("%s labels=%v selected=%s value=%s", sample.name, sample.labels, map[bool]string{true: "gray", false: "normal"}[sample.id == gray], sample.value)
			}
			invalidDefinition := proto.Clone(definition).(*configv1.ConfigDefinition)
			invalidDefinition.ValueRules[0].Audience.Conditions[0] = &configv1.LabelCondition{Key: "client", Type: configv1.MatchType_MATCH_TYPE_REGEX, Value: "["}
			f.post(t, "preview", &configv1.PreviewRequest{Key: key, Definition: invalidDefinition}, 400, nil)
		})
	}
}

// TestSixMatchLiveConsole 为浏览器人工/代理验收显式启用；只监听loopback，使用隔离测试身份/内存仓库。
// 生产 HTTP 与 gRPC 及领域逻辑不替换；不表示真实账号/SQL/外部SDK端到端验收。
func TestSixMatchLiveConsole(t *testing.T) {
	addr := os.Getenv("POLE_SIX_MATCH_LIVE_ADDR")
	if addr == "" {
		t.Skip("set POLE_SIX_MATCH_LIVE_ADDR=127.0.0.1:18086 for isolated browser verification")
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host != "127.0.0.1" {
		t.Fatal("live fixture must listen on IPv4 loopback")
	}
	f := newSixMatchFixture(t, addr)
	for _, tc := range sixMatchCases {
		f.seed(t, tc)
	}
	t.Logf("real configuration HTTP listening at %s; isolated test auth/store only", f.http.URL)
	deadline := time.NewTimer(20 * time.Minute)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-deadline.C:
			return
		case <-ticker.C:
			if _, err := os.Stat("/tmp/pole-six-match-live.stop"); err == nil {
				return
			}
		}
	}
}
