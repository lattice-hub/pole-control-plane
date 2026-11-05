package unified_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/config/unified"
)

type testAuthorizer struct {
	denied      atomic.Bool
	unavailable atomic.Bool
}

func (a *testAuthorizer) Authorize(ctx context.Context, _ *configv1.ConfigKey, _ unified.Action) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if a.denied.Load() {
		return "", unified.ErrAccessDenied
	}
	if a.unavailable.Load() {
		return "", errors.New("sensitive credential")
	}
	return "verified-user", nil
}

// memoryRepository 模拟事务隔离和回调失败回滚；SQL 锁及持久性由 MySQL 测试验证。
type memoryRepository struct {
	mu     sync.Mutex
	states map[string]*store.UnifiedConfigAggregate
	cursor *configv1.Cursor
}

func newRepository() *memoryRepository {
	return &memoryRepository{states: map[string]*store.UnifiedConfigAggregate{}, cursor: &configv1.Cursor{Scope: "config", Epoch: "epoch"}}
}
func identity(k *configv1.ConfigKey) string {
	return fmt.Sprintf("%q/%q/%q", k.Environment, k.Group, k.Name)
}
func (m *memoryRepository) state(k *configv1.ConfigKey) *store.UnifiedConfigAggregate {
	if s := m.states[identity(k)]; s != nil {
		return s.Clone()
	}
	return &store.UnifiedConfigAggregate{Key: proto.Clone(k).(*configv1.ConfigKey)}
}
func (m *memoryRepository) ReadUnifiedConfig(ctx context.Context, k *configv1.ConfigKey) (*store.UnifiedConfigSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &store.UnifiedConfigSnapshot{State: m.state(k), Cursor: proto.Clone(m.cursor).(*configv1.Cursor)}, ctx.Err()
}
func (m *memoryRepository) UpdateUnifiedConfig(ctx context.Context, k *configv1.ConfigKey, mutate store.UnifiedConfigMutation) (*store.UnifiedConfigSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.state(k)
	change, err := mutate(state)
	if err != nil {
		return nil, err
	}
	if change != nil {
		m.states[identity(k)] = state.Clone()
		if change.Runtime {
			m.cursor.Position++
		}
	}
	return &store.UnifiedConfigSnapshot{State: m.state(k), Cursor: proto.Clone(m.cursor).(*configv1.Cursor)}, nil
}
func (m *memoryRepository) ReadUnifiedConfigEvents(context.Context, *configv1.Cursor, uint32) ([]*store.UnifiedConfigEvent, *configv1.Cursor, error) {
	return nil, nil, errors.New("native Discover must not watch events")
}

type fixture struct {
	service    *unified.Service
	repository *memoryRepository
	auth       *testAuthorizer
	key        *configv1.ConfigKey
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{repository: newRepository(), auth: &testAuthorizer{}, key: &configv1.ConfigKey{Environment: "prod", Group: "checkout", Name: "app"}}
	var err error
	f.service, err = unified.New(f.repository, f.auth)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func literal(value []byte) *configv1.ConfigDefinition {
	return &configv1.ConfigDefinition{Engine: "literal", Content: value, Format: "text"}
}
func unpack(t *testing.T, r *model.Response, out proto.Message) {
	t.Helper()
	if r.Code != uint32(model.Code_ExecuteSuccess) || r.Data == nil {
		t.Fatalf("unexpected response: %v", r)
	}
	if err := r.Data.UnmarshalTo(out); err != nil {
		t.Fatal(err)
	}
}
func requireCode(t *testing.T, r *model.Response, code model.Code) {
	t.Helper()
	if r.Code != uint32(code) || r.Data != nil {
		t.Fatalf("expected %v without data, got %v", code, r)
	}
}
func (f *fixture) get(t *testing.T) *configv1.ConfigState {
	t.Helper()
	r := &configv1.ConfigState{}
	unpack(t, f.service.GetState(context.Background(), &configv1.GetStateRequest{Key: f.key}), r)
	return r
}
func (f *fixture) save(t *testing.T, d *configv1.ConfigDefinition) *configv1.ConfigState {
	t.Helper()
	s := f.get(t)
	r := &configv1.ConfigState{}
	unpack(t, f.service.SaveDraft(context.Background(), &configv1.SaveDraftRequest{Key: f.key, Definition: d, ExpectedDraftRevision: s.GetDraft().GetRevision()}), r)
	return r
}
func (f *fixture) publish(t *testing.T, kind configv1.ReleaseKind, priority int32, labels map[string]string) *configv1.ConfigRelease {
	t.Helper()
	s := f.get(t)
	r := &configv1.ConfigState{}
	unpack(t, f.service.Publish(context.Background(), &configv1.PublishRequest{Key: f.key, Kind: kind, Priority: priority, Audience: &configv1.Audience{Labels: labels}, ExpectedDraftRevision: s.Draft.Revision, ExpectedReleaseSetRevision: s.ReleaseSetRevision}), r)
	if r.SelectedRelease != nil {
		t.Fatal("write response leaks history body")
	}
	detail := &configv1.ConfigState{}
	unpack(t, f.service.GetState(context.Background(), &configv1.GetStateRequest{Key: f.key, ReleaseId: r.Releases[0].Id}), detail)
	return detail.SelectedRelease
}
func (f *fixture) discover(t *testing.T, labels map[string]string) *configv1.ConfigResult {
	t.Helper()
	r := &configv1.ConfigResult{}
	unpack(t, f.service.Discover(context.Background(), &configv1.DiscoverRequest{Key: f.key, Labels: labels}), r)
	return r
}

func TestLiteralBytesPresenceAndDraftIsolation(t *testing.T) {
	for _, content := range [][]byte{nil, []byte("\ufeff{{anything}}\r\n"), {0, 0xff, 0x80}} {
		f := newFixture(t)
		absent := f.discover(t, nil)
		if absent.Presence != configv1.ConfigPresence_CONFIG_PRESENCE_ABSENT || absent.Snapshot != nil || absent.SnapshotRevision == "" {
			t.Fatal(absent)
		}
		f.save(t, literal(content))
		if !proto.Equal(absent, f.discover(t, nil)) {
			t.Fatal("draft changed runtime")
		}
		f.publish(t, configv1.ReleaseKind_RELEASE_KIND_NORMAL, 0, nil)
		r := f.discover(t, nil)
		if r.Presence != configv1.ConfigPresence_CONFIG_PRESENCE_PRESENT || r.Snapshot == nil || !bytes.Equal(content, r.Snapshot.Content) || r.ObservationCursor.Position != 1 {
			t.Fatal(r)
		}
	}
}

func template() *configv1.ConfigDefinition {
	return &configv1.ConfigDefinition{Engine: "pole-mustache-v1", Format: "json", Content: []byte(`{"host":"{{{host}}}"}`), Parameters: []*configv1.Parameter{{Name: "host", Type: configv1.ValueType_VALUE_TYPE_STRING, Required: true}}, DefaultValues: map[string]*configv1.Scalar{"host": {Type: configv1.ValueType_VALUE_TYPE_STRING, Value: `unescaped"value`}}, ValueRules: []*configv1.ValueRule{{Id: "one", Priority: 0, Audience: &configv1.Audience{Labels: map[string]string{"cohort": "one"}}, Values: map[string]*configv1.Scalar{"host": {Type: configv1.ValueType_VALUE_TYPE_STRING, Value: "selected"}}}, {Id: "other", Priority: 1, Audience: &configv1.Audience{Labels: map[string]string{"cohort": "other"}}, Values: map[string]*configv1.Scalar{"host": {Type: configv1.ValueType_VALUE_TYPE_STRING, Value: "other-secret"}}}}}
}
func TestNativeAndPreviewReturnOnlyRenderInput(t *testing.T) {
	f := newFixture(t)
	d := template()
	preview := &configv1.RenderSnapshot{}
	unpack(t, f.service.Preview(context.Background(), &configv1.PreviewRequest{Key: f.key, Definition: d}), preview)
	if !bytes.Equal(preview.Content, d.Content) || preview.Values["host"].Value != `unescaped"value` {
		t.Fatal(preview)
	}
	f.save(t, d)
	f.publish(t, configv1.ReleaseKind_RELEASE_KIND_NORMAL, 0, nil)
	r := f.discover(t, map[string]string{"cohort": "one"})
	if r.Snapshot.Values["host"].Value != "selected" || !bytes.Equal(r.Snapshot.Content, d.Content) {
		t.Fatal(r)
	}
	wire, _ := proto.Marshal(r)
	if bytes.Contains(wire, []byte("other-secret")) {
		t.Fatal("other audience leaked")
	}
	d.DefaultValues["host"].Value = "mutated"
	if f.discover(t, nil).Snapshot.Values["host"].Value == "mutated" {
		t.Fatal("release was mutable")
	}
}

func TestStaticValidationRejectsInvalidBranchesWithoutRendering(t *testing.T) {
	cases := map[string]func(*configv1.ConfigDefinition){"double": func(d *configv1.ConfigDefinition) { d.Content = []byte("{{host}}") }, "trailing delimiter": func(d *configv1.ConfigDefinition) { d.Content = []byte("{{{host}}}}}") }, "undeclared": func(d *configv1.ConfigDefinition) { d.Content = []byte("{{{secret}}}") }, "missing default": func(d *configv1.ConfigDefinition) { d.DefaultValues = nil }, "wrong type": func(d *configv1.ConfigDefinition) {
		d.ValueRules[0].Values["host"].Type = configv1.ValueType_VALUE_TYPE_INTEGER
	}, "unknown enum": func(d *configv1.ConfigDefinition) { d.Parameters[0].Type = 99 }, "duplicate parameter": func(d *configv1.ConfigDefinition) { d.Parameters = append(d.Parameters, d.Parameters[0]) }, "unknown format": func(d *configv1.ConfigDefinition) { d.Format = "shell" }, "ambiguous rules": func(d *configv1.ConfigDefinition) {
		d.ValueRules[1].Priority = 0
		d.ValueRules[1].Audience.Labels = map[string]string{"region": "east"}
	}, "negative": func(d *configv1.ConfigDefinition) { d.ValueRules[0].Priority = -1 }}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			d := template()
			mutate(d)
			requireCode(t, f.service.SaveDraft(context.Background(), &configv1.SaveDraftRequest{Key: f.key, Definition: d}), model.Code_InvalidParameter)
		})
	}
	f := newFixture(t)
	d := template()
	d.Content = []byte(strings.Repeat("{{{host}}}", 30000))
	d.DefaultValues["host"].Value = strings.Repeat("x", 100)
	f.save(t, d) // 输出超过上限由本地渲染器处理；服务端不生成展开结果。
}

func TestScalarCanonicalText(t *testing.T) {
	for _, c := range []struct {
		kind  configv1.ValueType
		value string
		valid bool
	}{{1, "", true}, {2, "TRUE", false}, {2, "false", true}, {3, "-0", false}, {3, "9007199254740993123456789", true}, {3, "01", false}, {4, "1.250", false}, {4, "1e3", false}, {4, "-0.01", true}, {4, "1", true}} {
		f := newFixture(t)
		d := template()
		d.ValueRules = nil
		d.Parameters[0].Type = c.kind
		d.DefaultValues["host"] = &configv1.Scalar{Type: c.kind, Value: c.value}
		r := f.service.Preview(context.Background(), &configv1.PreviewRequest{Key: f.key, Definition: d})
		if (r.Code == uint32(model.Code_ExecuteSuccess)) != c.valid {
			t.Fatalf("%v %q: %v", c.kind, c.value, r)
		}
	}
}

func TestGrayOrderingCopyRollbackAndOffline(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.save(t, literal([]byte("normal")))
	normal := f.publish(t, 1, 0, nil)
	f.save(t, literal([]byte("gray1")))
	g1 := f.publish(t, 2, 1, map[string]string{"cohort": "one"})
	f.save(t, literal([]byte("gray2")))
	g2 := f.publish(t, 2, 1, map[string]string{"cohort": "one"})
	labels := map[string]string{"cohort": "one"}
	if f.discover(t, labels).ReleaseId != g2.Id || f.discover(t, nil).ReleaseId != normal.Id {
		t.Fatal("selection order")
	}
	s := f.get(t)
	before := f.discover(t, labels)
	unpack(t, f.service.CopyReleaseToDraft(ctx, &configv1.CopyReleaseToDraftRequest{Key: f.key, ReleaseId: normal.Id, ExpectedDraftRevision: s.Draft.Revision, ExpectedReleaseSetRevision: s.ReleaseSetRevision}), &configv1.ConfigState{})
	if !proto.Equal(before, f.discover(t, labels)) || string(f.get(t).Draft.Definition.Content) != "normal" {
		t.Fatal("copy changed live set")
	}
	s = f.get(t)
	unpack(t, f.service.Rollback(ctx, &configv1.RollbackRequest{Key: f.key, SourceReleaseId: g1.Id, ReplaceReleaseId: g2.Id, ExpectedReleaseSetRevision: s.ReleaseSetRevision}), &configv1.ConfigState{})
	r := f.discover(t, labels)
	if r.ReleaseId == g1.Id || string(r.Snapshot.Content) != "gray1" {
		t.Fatal("rollback did not create new sequence")
	}
	s = f.get(t)
	unpack(t, f.service.Offline(ctx, &configv1.OfflineRequest{Key: f.key, ExpectedReleaseSetRevision: s.ReleaseSetRevision}), &configv1.ConfigState{})
	if f.discover(t, labels).Presence != configv1.ConfigPresence_CONFIG_PRESENCE_ABSENT || len(f.get(t).Releases) != 4 || f.get(t).Draft == nil {
		t.Fatal("offline removed history or kept active")
	}
}

func TestCASAtomicFailureAndConcurrentSave(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.save(t, literal([]byte("normal")))
	f.publish(t, 1, 0, nil)
	s := f.get(t)
	before := f.discover(t, nil)
	requireCode(t, f.service.Publish(ctx, &configv1.PublishRequest{Key: f.key, Kind: 1, ExpectedDraftRevision: s.Draft.Revision, ExpectedReleaseSetRevision: s.ReleaseSetRevision, StopReleaseId: "missing"}), model.Code_NotFoundResource)
	if !proto.Equal(before, f.discover(t, nil)) {
		t.Fatal("failed compound publish changed runtime")
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := f.service.SaveDraft(ctx, &configv1.SaveDraftRequest{Key: f.key, Definition: literal([]byte("next")), ExpectedDraftRevision: s.Draft.Revision})
			if r.Code == uint32(model.Code_ExecuteSuccess) {
				successes.Add(1)
			} else if r.Code != uint32(model.Code_DataConflict) {
				t.Errorf("unexpected code %v", r)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal(successes.Load())
	}
	requireCode(t, f.service.SaveDraft(ctx, &configv1.SaveDraftRequest{Key: f.key, Definition: literal(nil)}), model.Code_DataConflict)
}

func TestCursorAuthorizationAndCompatibilityDoNotFallback(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	d := literal([]byte("normal"))
	d.Compatibility = &configv1.ProtocolCompatibility{Apollo: true}
	f.save(t, d)
	f.publish(t, 1, 0, nil)
	f.save(t, literal([]byte("gray")))
	f.publish(t, 2, 0, map[string]string{"cohort": "one"})
	request := &configv1.DiscoverRequest{Key: f.key, Labels: map[string]string{"cohort": "one"}}
	requireCode(t, f.service.DiscoverForProtocol(ctx, request, "apollo"), model.Code_ClientAPINotOpen)
	unpack(t, f.service.Discover(ctx, request), &configv1.ConfigResult{})
	request.Labels = nil
	unpack(t, f.service.DiscoverForProtocol(ctx, request, "apollo"), &configv1.ConfigResult{})
	request.MinCursor = &configv1.Cursor{Scope: "config", Epoch: "old"}
	requireCode(t, f.service.Discover(ctx, request), model.Code_ConfigCursorReset)
	request.MinCursor = &configv1.Cursor{Scope: "config", Epoch: "epoch", Position: 999}
	requireCode(t, f.service.Discover(ctx, request), model.Code_ConfigUnavailable)
	f.auth.denied.Store(true)
	requireCode(t, f.service.Discover(ctx, request), model.Code_AuthTokenForbidden)
	f.auth.denied.Store(false)
	f.auth.unavailable.Store(true)
	r := f.service.Discover(ctx, request)
	requireCode(t, r, model.Code_ConfigUnavailable)
	if strings.Contains(r.Info, "sensitive") {
		t.Fatal("sensitive error leaked")
	}
}

func TestStopActionsAndUnrelatedGrayKeepCurrentSnapshotIdentity(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.save(t, literal([]byte("normal")))
	f.publish(t, 1, 0, nil)
	normal := f.discover(t, nil)
	f.save(t, literal([]byte("gray")))
	gray := f.publish(t, 2, 0, map[string]string{"cohort": "one"})
	current := f.discover(t, nil)
	if current.SnapshotRevision != normal.SnapshotRevision || current.ObservationCursor.Position <= normal.ObservationCursor.Position {
		t.Fatal("unrelated gray changed snapshot identity or lost cursor")
	}
	state := f.get(t)
	requireCode(t, f.service.Stop(ctx, &configv1.StopRequest{Key: f.key, ExpectedReleaseSetRevision: state.ReleaseSetRevision}), model.Code_InvalidParameter)
	unpack(t, f.service.Stop(ctx, &configv1.StopRequest{Key: f.key, ReleaseId: gray.Id, ExpectedReleaseSetRevision: state.ReleaseSetRevision}), &configv1.ConfigState{})
	if f.discover(t, map[string]string{"cohort": "one"}).SnapshotRevision != normal.SnapshotRevision {
		t.Fatal("stop did not restore normal selection")
	}
	// 已停止的历史也可复制，且复制不推进生效集合或运行时游标。
	state = f.get(t)
	before := f.discover(t, nil)
	unpack(t, f.service.CopyReleaseToDraft(ctx, &configv1.CopyReleaseToDraftRequest{Key: f.key, ReleaseId: gray.Id, ExpectedDraftRevision: state.Draft.Revision, ExpectedReleaseSetRevision: state.ReleaseSetRevision}), &configv1.ConfigState{})
	if !proto.Equal(before, f.discover(t, nil)) {
		t.Fatal("historical copy changed runtime")
	}
	f.publish(t, 2, 0, map[string]string{"cohort": "two"})
	f.publish(t, 2, 1, map[string]string{"cohort": "three"})
	state = f.get(t)
	unpack(t, f.service.StopAllGray(ctx, &configv1.StopAllGrayRequest{Key: f.key, ExpectedReleaseSetRevision: state.ReleaseSetRevision}), &configv1.ConfigState{})
	if len(f.get(t).ActiveReleaseIds) != 1 || f.discover(t, nil).SnapshotRevision != normal.SnapshotRevision {
		t.Fatal("stop-all-gray affected normal")
	}
}

func TestStaticInputBudgetCountsUnfilledOptionalParameterNames(t *testing.T) {
	f := newFixture(t)
	definition := &configv1.ConfigDefinition{Engine: "pole-mustache-v1", Content: bytes.Repeat([]byte("x"), 512*1024), Parameters: []*configv1.Parameter{{Name: "optional", Type: configv1.ValueType_VALUE_TYPE_STRING}}}
	requireCode(t, f.service.Preview(context.Background(), &configv1.PreviewRequest{Key: f.key, Definition: definition}), model.Code_InvalidParameter)
}

func TestDiscoverCursorLowerBoundAndMalformedScope(t *testing.T) {
	f := newFixture(t)
	f.save(t, literal([]byte("live")))
	f.publish(t, 1, 0, nil)
	current := f.discover(t, nil)
	for _, minimum := range []*configv1.Cursor{current.ObservationCursor, {Scope: "config", Epoch: "epoch", Position: 0}} {
		got := &configv1.ConfigResult{}
		unpack(t, f.service.Discover(context.Background(), &configv1.DiscoverRequest{Key: f.key, MinCursor: minimum}), got)
		if !proto.Equal(current, got) {
			t.Fatal("valid cursor lower bound changed snapshot")
		}
	}
	for _, test := range []struct {
		cursor *configv1.Cursor
		code   model.Code
	}{{&configv1.Cursor{Epoch: "epoch"}, model.Code_InvalidParameter}, {&configv1.Cursor{Scope: "config"}, model.Code_InvalidParameter}, {&configv1.Cursor{Scope: "different", Epoch: "epoch"}, model.Code_ConfigCursorReset}} {
		requireCode(t, f.service.Discover(context.Background(), &configv1.DiscoverRequest{Key: f.key, MinCursor: test.cursor}), test.code)
	}
}

func TestConditionRulesPreviewAndDiscover(t *testing.T) {
	cases := []struct {
		typ              configv1.MatchType
		value, hit, miss string
	}{
		{configv1.MatchType_MATCH_TYPE_EXACT, "*", "*", "x"},
		{configv1.MatchType_MATCH_TYPE_REGEX, "a+", "ba", "z"},
		{configv1.MatchType_MATCH_TYPE_NOT_EQUALS, "x", "y", "x"},
		{configv1.MatchType_MATCH_TYPE_IN, "a,,b", "", "c"},
		{configv1.MatchType_MATCH_TYPE_NOT_IN, "a,,b", "c", ""},
		{configv1.MatchType_MATCH_TYPE_RANGE, "-2~4", "4", "5"},
	}
	for _, tc := range cases {
		t.Run(tc.typ.String(), func(t *testing.T) {
			f := newFixture(t)
			d := template()
			d.ValueRules = d.ValueRules[:1]
			d.ValueRules[0].Audience = &configv1.Audience{Conditions: []*configv1.LabelCondition{{Key: "cohort", Type: tc.typ, Value: tc.value}}}
			f.save(t, d)
			f.publish(t, configv1.ReleaseKind_RELEASE_KIND_NORMAL, 0, nil)
			for _, x := range []struct {
				labels map[string]string
				want   string
			}{{map[string]string{"cohort": tc.hit}, "selected"}, {map[string]string{"cohort": tc.miss}, `unescaped"value`}, {nil, `unescaped"value`}} {
				preview := &configv1.RenderSnapshot{}
				unpack(t, f.service.Preview(context.Background(), &configv1.PreviewRequest{Key: f.key, Definition: d, Labels: x.labels}), preview)
				got := f.discover(t, x.labels).Snapshot
				if preview.Values["host"].Value != x.want || got.Values["host"].Value != x.want {
					t.Fatalf("preview=%v discover=%v", preview, got)
				}
			}
		})
	}
}

func TestConditionGrayPublicationAndInvalidState(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.save(t, literal([]byte("normal")))
	f.publish(t, configv1.ReleaseKind_RELEASE_KIND_NORMAL, 0, nil)
	f.save(t, literal([]byte("gray")))
	s := f.get(t)
	invalidAudience := &configv1.Audience{Conditions: []*configv1.LabelCondition{{Key: "k", Type: configv1.MatchType_MATCH_TYPE_REGEX, Value: "["}}}
	requireCode(t, f.service.Publish(ctx, &configv1.PublishRequest{Key: f.key, Kind: configv1.ReleaseKind_RELEASE_KIND_GRAY, Audience: invalidAudience, ExpectedDraftRevision: s.Draft.Revision, ExpectedReleaseSetRevision: s.ReleaseSetRevision}), model.Code_InvalidParameter)
	if !proto.Equal(s, f.get(t)) {
		t.Fatal("invalid publication changed state")
	}
	audience := &configv1.Audience{Conditions: []*configv1.LabelCondition{{Key: "k", Type: configv1.MatchType_MATCH_TYPE_IN, Value: "a,b"}}}
	result := &configv1.ConfigState{}
	unpack(t, f.service.Publish(ctx, &configv1.PublishRequest{Key: f.key, Kind: configv1.ReleaseKind_RELEASE_KIND_GRAY, Audience: audience, ExpectedDraftRevision: s.Draft.Revision, ExpectedReleaseSetRevision: s.ReleaseSetRevision}), result)
	if got := string(f.discover(t, map[string]string{"k": "b"}).Snapshot.Content); got != "gray" {
		t.Fatal(got)
	}
	if got := string(f.discover(t, nil).Snapshot.Content); got != "normal" {
		t.Fatal(got)
	}
}
