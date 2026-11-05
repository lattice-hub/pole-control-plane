// Package unified 实现模板、values 与投放条件一起发布的配置聚合。
// 发布权威始终是不可变定义，客户端取得当前上下文的模板与有效值并本地渲染。
package unified

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/store"
)

// Action 区分运行时消费、定义管理和参数预览的授权边界。
type Action string

const (
	ActionRead    Action = "read"
	ActionInspect Action = "inspect"
	ActionManage  Action = "manage"
	ActionPreview Action = "preview"
)

// ErrAccessDenied 由鉴权适配器仅在凭据或资源权限明确拒绝时返回；依赖故障应保留为故障。
var ErrAccessDenied = errors.New("configuration access denied")

// Authorizer 对真实调用身份及配置作用域授权，返回用于审计的可信身份。
// 客户端 labels 只参加匹配，不能作为此接口的权限依据。
type Authorizer interface {
	Authorize(context.Context, *configv1.ConfigKey, Action) (string, error)
}

// Service 封装配置的领域操作与结果解析；持久化负责事务和游标，入口负责协议编码。
type Service struct {
	storage    store.UnifiedConfigStore
	authorizer Authorizer
	clock      func() time.Time
}

// New 创建完整服务；缺少持久化或授权组件时明确失败，不隐式启用内存存储或匿名权限。
func New(storage store.UnifiedConfigStore, authorizer Authorizer) (*Service, error) {
	if storage == nil || authorizer == nil {
		return nil, fmt.Errorf("unified configuration requires storage and authorizer")
	}
	return &Service{storage: storage, authorizer: authorizer, clock: time.Now}, nil
}

type domainError struct {
	code    model.Code
	message string
}

func (e *domainError) Error() string { return e.message }

func invalid(message string) error {
	return &domainError{code: model.Code_InvalidParameter, message: message}
}

func conflict(message string) error {
	return &domainError{code: model.Code_DataConflict, message: message}
}

func missing(message string) error {
	return &domainError{code: model.Code_NotFoundResource, message: message}
}

func errorResponse(err error) *model.Response {
	var failure *domainError
	if errors.As(err, &failure) {
		return &model.Response{Code: uint32(failure.code), Info: failure.message}
	}
	// 驱动错误可能带 SQL 或配置值；对外只报告依赖不可用。
	return &model.Response{Code: uint32(model.Code_ConfigUnavailable), Info: "configuration store is unavailable"}
}
func successResponse(data proto.Message) *model.Response {
	payload, err := anypb.New(data)
	if err != nil {
		return errorResponse(err)
	}
	return &model.Response{Code: uint32(model.Code_ExecuteSuccess), Data: payload}
}

func validateKey(key *configv1.ConfigKey) error {
	for _, value := range []string{key.GetEnvironment(), key.GetGroup(), key.GetName()} {
		if value == "" || len(value) > 128 || !utf8.ValidString(value) || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\x00\r\n") {
			return invalid("environment, group and name must be nonempty UTF-8 strings of at most 128 bytes without surrounding whitespace or control separators")
		}
	}
	return nil
}

func (s *Service) authorize(ctx context.Context, key *configv1.ConfigKey, action Action) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validateKey(key); err != nil {
		return "", err
	}
	actor, err := s.authorizer.Authorize(ctx, key, action)
	if err != nil {
		if errors.Is(err, ErrAccessDenied) {
			return "", &domainError{code: model.Code_AuthTokenForbidden, message: "configuration access denied"}
		}
		return "", err
	}
	return actor, nil
}

func checkDraft(state *store.UnifiedConfigAggregate, expected uint64) error {
	if state.GetDraft().GetRevision() != expected {
		return conflict("draft revision changed")
	}
	return nil
}

func checkSet(state *store.UnifiedConfigAggregate, expected uint64) error {
	if state.GetReleaseSetRevision() != expected {
		return conflict("release set revision changed")
	}
	return nil
}

func nextRevision(value uint64) (uint64, error) {
	if value == math.MaxUint64 {
		return 0, conflict("revision space exhausted")
	}
	return value + 1, nil
}

func stateResponse(snapshot *store.UnifiedConfigSnapshot, err error) *model.Response {
	if err != nil {
		return errorResponse(err)
	}
	return managementView(snapshot, nil)
}

// GetState 返回草稿、历史元数据页和可选单版本快照；运行时读取必须使用 Discover。
func (s *Service) GetState(ctx context.Context, req *configv1.GetStateRequest) *model.Response {
	if _, err := s.authorize(ctx, req.GetKey(), ActionInspect); err != nil {
		return stateResponse(nil, err)
	}
	snapshot, err := s.storage.ReadUnifiedConfig(ctx, req.GetKey())
	if err != nil {
		return stateResponse(nil, err)
	}
	return managementView(snapshot, req)
}

// SaveDraft 使用 expected_draft_revision 比较并交换整份定义，不改变任何生效版本。
// 初次保存期望版本为 0；例如并发保存同一 revision 时只有一次成功。
func (s *Service) SaveDraft(ctx context.Context, req *configv1.SaveDraftRequest) *model.Response {
	actor, err := s.authorize(ctx, req.GetKey(), ActionManage)
	if err != nil {
		return stateResponse(nil, err)
	}
	if err = validateDefinition(ctx, req.GetDefinition()); err != nil {
		return stateResponse(nil, err)
	}
	definition := proto.Clone(req.GetDefinition()).(*configv1.ConfigDefinition)
	snapshot, err := s.storage.UpdateUnifiedConfig(ctx, req.GetKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		if err := checkDraft(state, req.GetExpectedDraftRevision()); err != nil {
			return nil, err
		}
		revision, err := nextRevision(state.GetDraft().GetRevision())
		if err != nil {
			return nil, err
		}
		if state.Incarnation == "" {
			state.Incarnation = uuid.NewString()
		}
		state.Draft = &configv1.ConfigDraft{Key: proto.Clone(req.Key).(*configv1.ConfigKey), Definition: definition, Revision: revision}
		return &store.UnifiedConfigChange{Kind: "draft_saved", Actor: actor}, nil
	})
	return stateResponse(snapshot, err)
}

func findMember(state *store.UnifiedConfigAggregate, id string) *store.UnifiedConfigMember {
	for _, member := range state.GetMembers() {
		if member.GetRelease().GetId() == id && id != "" {
			return member
		}
	}
	return nil
}

func deactivateNormal(state *store.UnifiedConfigAggregate) {
	for _, member := range state.GetMembers() {
		if member.GetRelease().GetKind() == configv1.ReleaseKind_RELEASE_KIND_NORMAL {
			member.Active = false
		}
	}
}

func advanceSet(state *store.UnifiedConfigAggregate) error {
	revision, err := nextRevision(state.GetReleaseSetRevision())
	if err != nil {
		return err
	}
	state.ReleaseSetRevision = revision
	return nil
}

func nextSequence(state *store.UnifiedConfigAggregate) (uint64, error) {
	var maximum uint64
	for _, member := range state.GetMembers() {
		if member.GetRelease().GetSequence() > maximum {
			maximum = member.Release.Sequence
		}
	}
	return nextRevision(maximum)
}

func activeGray(state *store.UnifiedConfigAggregate, id string) (*store.UnifiedConfigMember, error) {
	member := findMember(state, id)
	if member == nil || !member.Active || member.Release.Kind != configv1.ReleaseKind_RELEASE_KIND_GRAY {
		return nil, missing("specified active gray release does not exist")
	}
	return member, nil
}

// Publish 冻结草稿完整定义，在同一事务更新生效集合；正式发布不停止其他灰度。
// replace_release_id 校验并替换指定同类型 active 版本；stop_release_id 仅用于正式发布同时停止已审阅源灰度。
func (s *Service) Publish(ctx context.Context, req *configv1.PublishRequest) *model.Response {
	actor, err := s.authorize(ctx, req.GetKey(), ActionManage)
	if err != nil {
		return stateResponse(nil, err)
	}
	if err := validateRollout(req.GetKind(), req.GetAudience(), req.GetPriority()); err != nil {
		return stateResponse(nil, err)
	}
	if req.GetStopReleaseId() != "" && req.GetKind() != configv1.ReleaseKind_RELEASE_KIND_NORMAL {
		return stateResponse(nil, invalid("stop_release_id requires a normal publication"))
	}
	snapshot, err := s.storage.UpdateUnifiedConfig(ctx, req.GetKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		if err := checkDraft(state, req.GetExpectedDraftRevision()); err != nil {
			return nil, err
		}
		if err := checkSet(state, req.GetExpectedReleaseSetRevision()); err != nil {
			return nil, err
		}
		if state.Draft == nil {
			return nil, missing("configuration draft does not exist")
		}
		if err := validateDefinition(ctx, state.Draft.Definition); err != nil {
			return nil, err
		}
		if id := req.GetReplaceReleaseId(); id != "" {
			member := findMember(state, id)
			if member == nil || !member.Active || member.GetRelease().GetKind() != req.Kind {
				return nil, missing("specified active release of the same kind does not exist")
			}
			member.Active = false
		}
		if id := req.GetStopReleaseId(); id != "" {
			member, err := activeGray(state, id)
			if err != nil {
				return nil, err
			}
			member.Active = false
		}
		if req.Kind == configv1.ReleaseKind_RELEASE_KIND_NORMAL {
			deactivateNormal(state)
		}
		if req.Kind == configv1.ReleaseKind_RELEASE_KIND_GRAY {
			active := 0
			for _, member := range state.Members {
				if member.Active && member.Release.Kind == configv1.ReleaseKind_RELEASE_KIND_GRAY {
					active++
				}
			}
			if active >= 128 {
				return nil, invalid("at most 128 active gray releases are supported")
			}
		}
		sequence, err := nextSequence(state)
		if err != nil {
			return nil, err
		}
		release := &configv1.ConfigRelease{
			Id: uuid.NewString(), Key: proto.Clone(req.Key).(*configv1.ConfigKey),
			Definition: proto.Clone(state.Draft.Definition).(*configv1.ConfigDefinition),
			Kind:       req.Kind, Priority: req.Priority, Sequence: sequence,
			CreatedAt: s.clock().UTC().Format(time.RFC3339Nano), CreatedBy: actor,
		}
		if req.Audience != nil {
			release.Audience = proto.Clone(req.Audience).(*configv1.Audience)
		}
		state.Members = append(state.Members, &store.UnifiedConfigMember{Release: release, Active: true})
		state.Offline = false
		if err := advanceSet(state); err != nil {
			return nil, err
		}
		return &store.UnifiedConfigChange{Runtime: true, Kind: "published", Actor: actor}, nil
	})
	return stateResponse(snapshot, err)
}
