package unified

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/store"
)

// Stop 只停止指定 active gray，空 release_id 不具有停止全部的含义。
func (s *Service) Stop(ctx context.Context, req *configv1.StopRequest) *model.Response {
	actor, err := s.authorize(ctx, req.GetKey(), ActionManage)
	if err != nil {
		return stateResponse(nil, err)
	}
	if req.GetReleaseId() == "" {
		return stateResponse(nil, invalid("release_id is required"))
	}
	snapshot, err := s.storage.UpdateUnifiedConfig(ctx, req.GetKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		if err := checkSet(state, req.GetExpectedReleaseSetRevision()); err != nil {
			return nil, err
		}
		member, err := activeGray(state, req.GetReleaseId())
		if err != nil {
			return nil, err
		}
		member.Active = false
		if err := advanceSet(state); err != nil {
			return nil, err
		}
		return &store.UnifiedConfigChange{Runtime: true, Kind: "gray_stopped", Actor: actor}, nil
	})
	return stateResponse(snapshot, err)
}

// StopAllGray 显式停止该配置环境的全部灰度，不修改正式版或任何历史定义。
func (s *Service) StopAllGray(ctx context.Context, req *configv1.StopAllGrayRequest) *model.Response {
	actor, err := s.authorize(ctx, req.GetKey(), ActionManage)
	if err != nil {
		return stateResponse(nil, err)
	}
	snapshot, err := s.storage.UpdateUnifiedConfig(ctx, req.GetKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		if err := checkSet(state, req.GetExpectedReleaseSetRevision()); err != nil {
			return nil, err
		}
		changed := false
		for _, member := range state.GetMembers() {
			if member.Active && member.GetRelease().GetKind() == configv1.ReleaseKind_RELEASE_KIND_GRAY {
				member.Active = false
				changed = true
			}
		}
		if !changed {
			return nil, nil
		}
		if err := advanceSet(state); err != nil {
			return nil, err
		}
		return &store.UnifiedConfigChange{Runtime: true, Kind: "all_gray_stopped", Actor: actor}, nil
	})
	return stateResponse(snapshot, err)
}

// CopyReleaseToDraft 把指定历史发布的完整定义复制为新草稿，供后续审阅并显式发布。
// 它不改变任何发布成员关系，也不推进运行时观察游标。
func (s *Service) CopyReleaseToDraft(ctx context.Context, req *configv1.CopyReleaseToDraftRequest) *model.Response {
	actor, err := s.authorize(ctx, req.GetKey(), ActionManage)
	if err != nil {
		return stateResponse(nil, err)
	}
	snapshot, err := s.storage.UpdateUnifiedConfig(ctx, req.GetKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		if err := checkDraft(state, req.GetExpectedDraftRevision()); err != nil {
			return nil, err
		}
		if err := checkSet(state, req.GetExpectedReleaseSetRevision()); err != nil {
			return nil, err
		}
		source := findMember(state, req.GetReleaseId())
		if source == nil {
			return nil, missing("source release does not exist in this configuration")
		}
		if err := validateDefinition(ctx, source.Release.Definition); err != nil {
			return nil, err
		}
		revision, err := nextRevision(state.GetDraft().GetRevision())
		if err != nil {
			return nil, err
		}
		state.Draft = &configv1.ConfigDraft{Key: proto.Clone(req.Key).(*configv1.ConfigKey), Definition: proto.Clone(source.Release.Definition).(*configv1.ConfigDefinition), Revision: revision}
		return &store.UnifiedConfigChange{Kind: "release_copied_to_draft", Actor: actor}, nil
	})
	return stateResponse(snapshot, err)
}

// Rollback 从该配置的历史完整定义创建新 release，序号和事件继续前进。
// 灰度回滚必须指定被替换的 active gray；正式回滚只替换正式版，其他灰度保持不变。
func (s *Service) Rollback(ctx context.Context, req *configv1.RollbackRequest) *model.Response {
	actor, err := s.authorize(ctx, req.GetKey(), ActionManage)
	if err != nil {
		return stateResponse(nil, err)
	}
	snapshot, err := s.storage.UpdateUnifiedConfig(ctx, req.GetKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		if err := checkSet(state, req.GetExpectedReleaseSetRevision()); err != nil {
			return nil, err
		}
		source := findMember(state, req.GetSourceReleaseId())
		if source == nil {
			return nil, missing("source release does not exist in this configuration")
		}
		if err := validateDefinition(ctx, source.Release.Definition); err != nil {
			return nil, err
		}
		if err := validateRollout(source.Release.Kind, source.Release.Audience, source.Release.Priority); err != nil {
			return nil, err
		}
		if source.Release.Kind == configv1.ReleaseKind_RELEASE_KIND_GRAY {
			target, err := activeGray(state, req.GetReplaceReleaseId())
			if err != nil {
				return nil, err
			}
			target.Active = false
		} else {
			for _, member := range state.GetMembers() {
				if member.GetActive() && member.GetRelease().GetKind() == configv1.ReleaseKind_RELEASE_KIND_NORMAL && req.GetReplaceReleaseId() == "" {
					return nil, invalid("replace_release_id is required for an active normal rollback")
				}
			}
			if req.GetReplaceReleaseId() != "" {
				target := findMember(state, req.ReplaceReleaseId)
				if target == nil || !target.Active || target.Release.Kind != configv1.ReleaseKind_RELEASE_KIND_NORMAL {
					return nil, missing("specified active normal release does not exist")
				}
			}
			deactivateNormal(state)
		}
		sequence, err := nextSequence(state)
		if err != nil {
			return nil, err
		}
		release := proto.Clone(source.Release).(*configv1.ConfigRelease)
		release.Id, release.Sequence, release.SourceReleaseId = uuid.NewString(), sequence, source.Release.Id
		release.CreatedAt, release.CreatedBy = s.clock().UTC().Format(time.RFC3339Nano), actor
		state.Members = append(state.Members, &store.UnifiedConfigMember{Release: release, Active: true})
		state.Offline = false
		if err := advanceSet(state); err != nil {
			return nil, err
		}
		return &store.UnifiedConfigChange{Runtime: true, Kind: "rolled_back", Actor: actor}, nil
	})
	return stateResponse(snapshot, err)
}

// Offline 下线当前环境全部生效版本，保存草稿与发布历史；空内容仍是正常存在的值。
func (s *Service) Offline(ctx context.Context, req *configv1.OfflineRequest) *model.Response {
	actor, err := s.authorize(ctx, req.GetKey(), ActionManage)
	if err != nil {
		return stateResponse(nil, err)
	}
	snapshot, err := s.storage.UpdateUnifiedConfig(ctx, req.GetKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		if err := checkSet(state, req.GetExpectedReleaseSetRevision()); err != nil {
			return nil, err
		}
		if state.Incarnation == "" {
			return nil, missing("configuration does not exist")
		}
		if state.Offline {
			return nil, nil
		}
		for _, member := range state.GetMembers() {
			member.Active = false
		}
		state.Offline = true
		if err := advanceSet(state); err != nil {
			return nil, err
		}
		return &store.UnifiedConfigChange{Runtime: true, Kind: "offline", Actor: actor}, nil
	})
	return stateResponse(snapshot, err)
}
