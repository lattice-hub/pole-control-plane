package unified

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pole-io/pole-server/apis/store"
	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"github.com/pole-io/specification/source/go/api/v1/model"
	"google.golang.org/protobuf/proto"
)

// PromoteGray 将生效灰度的冻结定义原子转为正式发布，不改变草稿或其他灰度。
func (s *Service) PromoteGray(ctx context.Context, req *configv1.PromoteGrayRequest) *model.Response {
	actor, err := s.authorize(ctx, req.GetKey(), ActionManage)
	if err != nil {
		return stateResponse(nil, err)
	}
	snapshot, err := s.storage.UpdateUnifiedConfig(ctx, req.GetKey(), func(state *store.UnifiedConfigAggregate) (*store.UnifiedConfigChange, error) {
		if err := checkSet(state, req.GetExpectedReleaseSetRevision()); err != nil {
			return nil, err
		}
		source, err := activeGray(state, req.GetReleaseId())
		if err != nil {
			return nil, err
		}
		if err := validateDefinition(ctx, source.Release.Definition); err != nil {
			return nil, err
		}
		definition := proto.Clone(source.Release.Definition).(*configv1.ConfigDefinition)
		sequence, err := nextSequence(state)
		if err != nil {
			return nil, err
		}
		deactivateNormal(state)
		source.Active = false
		release := &configv1.ConfigRelease{Id: uuid.NewString(), Key: proto.Clone(req.Key).(*configv1.ConfigKey), Definition: definition, Kind: configv1.ReleaseKind_RELEASE_KIND_NORMAL, Sequence: sequence, CreatedAt: s.clock().UTC().Format(time.RFC3339Nano), CreatedBy: actor, SourceReleaseId: source.Release.Id}
		state.Members = append(state.Members, &store.UnifiedConfigMember{Release: release, Active: true})
		state.Offline = false
		if err := advanceSet(state); err != nil {
			return nil, err
		}
		return &store.UnifiedConfigChange{Runtime: true, Kind: "gray_promoted", Actor: actor}, nil
	})
	return stateResponse(snapshot, err)
}
