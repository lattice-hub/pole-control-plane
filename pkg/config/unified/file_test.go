package unified

import (
	"context"
	"testing"

	"github.com/pole-io/pole-server/apis/store"
	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"github.com/pole-io/specification/source/go/api/v1/model"
	"google.golang.org/protobuf/proto"
)

type fileTestStore struct {
	store.UnifiedConfigStore
	template *configv1.ConfigFileTemplate
	saves    int
	drafts   map[string]*configv1.ConfigDraft
}

func (f *fileTestStore) ReadUnifiedConfig(_ context.Context, key *configv1.ConfigKey) (*store.UnifiedConfigSnapshot, error) {
	return &store.UnifiedConfigSnapshot{State: &store.UnifiedConfigAggregate{Key: key, Draft: f.drafts[key.Environment]}}, nil
}
func (f *fileTestStore) ReadConfigFileTemplate(context.Context, *configv1.ConfigFileKey) (*configv1.ConfigFileTemplate, error) {
	if f.template == nil {
		return nil, nil
	}
	return proto.Clone(f.template).(*configv1.ConfigFileTemplate), nil
}
func (f *fileTestStore) SaveConfigFileTemplate(_ context.Context, t *configv1.ConfigFileTemplate, expected uint64) (*configv1.ConfigFileTemplate, error) {
	f.saves++
	if f.template.GetRevision() != expected {
		return nil, store.ErrConfigFileConflict
	}
	f.template = proto.Clone(t).(*configv1.ConfigFileTemplate)
	f.template.Revision = expected + 1
	return f.template, nil
}
func (f *fileTestStore) ListConfigFiles(context.Context, *configv1.ListConfigFilesRequest) (*configv1.ConfigFileList, error) {
	return &configv1.ConfigFileList{}, nil
}

type fileTestAuth struct{ denied string }

func (a fileTestAuth) Authorize(_ context.Context, k *configv1.ConfigKey, _ Action) (string, error) {
	if k.Environment == a.denied {
		return "", ErrAccessDenied
	}
	return "actor", nil
}
func TestFileTemplateAuthorizationAndCAS(t *testing.T) {
	ctx := context.Background()
	fs := &fileTestStore{}
	s, _ := New(fs, fileTestAuth{})
	key := &configv1.ConfigFileKey{Group: "g", Name: "n"}
	template := &configv1.ConfigFileTemplate{Key: key, Environments: []string{"a", "b"}, Definition: &configv1.ConfigDefinition{Engine: "literal", Content: []byte("hello")}}
	save := func(expected uint64) *model.Response {
		return s.SaveFileTemplate(ctx, &configv1.SaveFileTemplateRequest{Template: template, ExpectedRevision: expected})
	}
	if r := save(0); r.Code != uint32(model.Code_ExecuteSuccess) {
		t.Fatalf("save: %v", r)
	}
	previousSaves := fs.saves
	if r := save(0); r.Code != uint32(model.Code_DataConflict) {
		t.Fatalf("CAS: %v", r)
	}
	if fs.saves != previousSaves {
		t.Fatal("mismatched observed revision reached storage CAS")
	}
	s.authorizer = fileTestAuth{denied: "b"}
	template.Environments = []string{"a"}
	if r := save(1); r.Code != uint32(model.Code_AuthTokenForbidden) {
		t.Fatalf("old environment must be authorized: %v", r)
	}
	if r := s.GetFileTemplate(ctx, &configv1.GetFileTemplateRequest{Key: key, Environment: "b"}); r.Code != uint32(model.Code_AuthTokenForbidden) {
		t.Fatalf("read denied: %v", r)
	}
	s.authorizer = fileTestAuth{}
	if r := s.GetFileTemplate(ctx, &configv1.GetFileTemplateRequest{Key: key, Environment: "c"}); r.Code != uint32(model.Code_NotFoundResource) {
		t.Fatalf("unbound environment: %v", r)
	}
	if fs.template.Revision != 1 || len(fs.template.Environments) != 2 {
		t.Fatal("failed saves changed template")
	}
}

func TestCreateFilePreservesExistingDraft(t *testing.T) {
	ctx := context.Background()
	key := &configv1.ConfigFileKey{Group: "g", Name: "n"}
	definition := &configv1.ConfigDefinition{Engine: "literal", Content: []byte("existing")}
	fs := &fileTestStore{drafts: map[string]*configv1.ConfigDraft{"a": {Definition: definition}}}
	s, _ := New(fs, fileTestAuth{})
	request := &configv1.SaveFileTemplateRequest{Template: &configv1.ConfigFileTemplate{Key: key, Environments: []string{"a"}, Definition: &configv1.ConfigDefinition{Engine: "literal", Content: []byte("different")}}}
	if r := s.SaveFileTemplate(ctx, request); r.Code != uint32(model.Code_DataConflict) || fs.saves != 0 {
		t.Fatalf("existing draft changed: %v", r)
	}
	request.Template.Definition = proto.Clone(definition).(*configv1.ConfigDefinition)
	if r := s.SaveFileTemplate(ctx, request); r.Code != uint32(model.Code_ExecuteSuccess) {
		t.Fatalf("matching source rejected: %v", r)
	}
}
