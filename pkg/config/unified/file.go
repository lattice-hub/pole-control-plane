package unified

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/pole-io/pole-server/apis/store"
	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"github.com/pole-io/specification/source/go/api/v1/model"
	"google.golang.org/protobuf/proto"
)

func fileKey(k *configv1.ConfigFileKey, env string) *configv1.ConfigKey {
	return &configv1.ConfigKey{Environment: env, Group: k.GetGroup(), Name: k.GetName()}
}
func (s *Service) fileStore() (store.UnifiedConfigFileStore, error) {
	v, ok := s.storage.(store.UnifiedConfigFileStore)
	if !ok {
		return nil, errors.New("file store unavailable")
	}
	return v, nil
}

// GetFileTemplate 仅允许绑定环境的 inspect 调用者读取共享正文。
func (s *Service) GetFileTemplate(ctx context.Context, req *configv1.GetFileTemplateRequest) *model.Response {
	if _, err := s.authorize(ctx, fileKey(req.GetKey(), req.GetEnvironment()), ActionInspect); err != nil {
		return errorResponse(err)
	}
	st, err := s.fileStore()
	if err != nil {
		return errorResponse(err)
	}
	result, err := st.ReadConfigFileTemplate(ctx, req.GetKey())
	if err != nil {
		return errorResponse(err)
	}
	if result == nil {
		return errorResponse(missing("shared template does not exist"))
	}
	for _, env := range result.GetEnvironments() {
		if env == req.GetEnvironment() {
			return successResponse(result)
		}
	}
	return errorResponse(missing("shared template does not exist"))
}

// SaveFileTemplate 使用独立 CAS；所有旧及新环境均需管理权限。
func (s *Service) SaveFileTemplate(ctx context.Context, req *configv1.SaveFileTemplateRequest) *model.Response {
	t := req.GetTemplate()
	if t == nil || t.GetKey() == nil || t.GetDefinition() == nil || len(t.GetEnvironments()) == 0 || len(t.GetEnvironments()) > 64 || len(t.GetDescription()) > 1024 || !utf8.ValidString(t.GetDescription()) {
		return errorResponse(invalid("invalid shared template"))
	}
	d := t.GetDefinition()
	if len(d.GetDefaultValues()) > 0 || len(d.GetValueRules()) > 0 || len(d.GetConditionGroups()) > 0 || len(d.GetParameterBindings()) > 0 {
		return errorResponse(invalid("shared template cannot contain environment values"))
	}
	if proto.Size(d) > maxDefinitionBytes || len(d.Content) > maxContentBytes || len(d.Parameters) > maxParameters || len(d.Format) > 32 || !utf8.ValidString(d.Format) {
		return errorResponse(invalid("invalid shared definition size"))
	}
	if d.Engine == "literal" {
		if len(d.Parameters) != 0 {
			return errorResponse(invalid("literal definitions cannot contain parameters"))
		}
	} else if d.Engine == "pole-mustache-v1" {
		if !utf8.Valid(d.Content) {
			return errorResponse(invalid("template must be UTF-8"))
		}
		if _, err := validateTemplate(d); err != nil {
			return errorResponse(err)
		}
	} else {
		return errorResponse(invalid("unsupported template engine"))
	}
	seen := map[string]bool{}
	for _, env := range t.GetEnvironments() {
		if seen[env] {
			return errorResponse(invalid("duplicate environment"))
		}
		seen[env] = true
		if _, err := s.authorize(ctx, fileKey(t.GetKey(), env), ActionManage); err != nil {
			return errorResponse(err)
		}
	}
	st, err := s.fileStore()
	if err != nil {
		return errorResponse(err)
	}
	old, err := st.ReadConfigFileTemplate(ctx, t.GetKey())
	if err != nil {
		return errorResponse(err)
	}
	if old.GetRevision() != req.GetExpectedRevision() {
		return errorResponse(conflict("shared template revision changed"))
	}
	if old == nil {
		for _, env := range t.GetEnvironments() {
			snapshot, err := s.storage.ReadUnifiedConfig(ctx, fileKey(t.GetKey(), env))
			if err != nil {
				return errorResponse(err)
			}
			if snapshot == nil || snapshot.State == nil {
				return errorResponse(errors.New("missing configuration snapshot"))
			}
			if draft := snapshot.State.GetDraft(); draft != nil && !store.SameConfigFileSharedDefinition(draft.GetDefinition(), t.GetDefinition()) {
				return errorResponse(conflict("existing draft has different shared content"))
			}
		}
	}
	if old != nil {
		for _, env := range old.GetEnvironments() {
			if _, err := s.authorize(ctx, fileKey(t.GetKey(), env), ActionManage); err != nil {
				return errorResponse(err)
			}
			if !seen[env] {
				return errorResponse(invalid("removing environments is not supported"))
			}
		}
	}
	copy := proto.Clone(t).(*configv1.ConfigFileTemplate)
	result, err := st.SaveConfigFileTemplate(ctx, copy, req.GetExpectedRevision())
	if errors.Is(err, store.ErrConfigFileConflict) {
		return errorResponse(conflict("shared template revision changed"))
	}
	if err != nil {
		return errorResponse(err)
	}
	return successResponse(result)
}

// ListFiles 按单一授权环境返回有界目录。
func (s *Service) ListFiles(ctx context.Context, req *configv1.ListConfigFilesRequest) *model.Response {
	group := req.GetGroup()
	if group == "" {
		group = "_"
	}
	if _, err := s.authorize(ctx, &configv1.ConfigKey{Environment: req.GetEnvironment(), Group: group, Name: "_"}, ActionInspect); err != nil {
		return errorResponse(err)
	}
	if len(req.GetQuery()) > 128 || !utf8.ValidString(req.GetQuery()) || strings.ContainsAny(req.GetQuery(), "\x00\r\n") || req.GetPageSize() > 100 {
		return errorResponse(invalid("invalid file query"))
	}
	st, err := s.fileStore()
	if err != nil {
		return errorResponse(err)
	}
	list, err := st.ListConfigFiles(ctx, req)
	if errors.Is(err, store.ErrUnifiedConfigCursorInvalid) {
		return errorResponse(invalid("invalid page token"))
	}
	if err != nil {
		return errorResponse(err)
	}
	return successResponse(list)
}
