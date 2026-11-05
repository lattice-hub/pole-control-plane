package unified

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"

	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/store"
)

const maxMemberPageBytes = 256 * 1024

// managementPage 是不透明的分页位置，不是授权凭据。绑定存在期与集合 CAS，避免翻页混入另一版集合。
type managementPage struct {
	Key         *configv1.ConfigKey `json:"key"`
	Incarnation string              `json:"incarnation"`
	Revision    uint64              `json:"revision"`
	Before      uint64              `json:"before"`
}

func decodeManagementPage(state *store.UnifiedConfigAggregate, token string) (uint64, error) {
	if token == "" {
		return 0, nil
	}
	if len(token) > 4096 {
		return 0, invalid("invalid page_token")
	}
	wire, err := base64.RawURLEncoding.DecodeString(token)
	var page managementPage
	if err != nil || json.Unmarshal(wire, &page) != nil || page.Before == 0 || !proto.Equal(page.Key, state.Key) {
		return 0, invalid("invalid page_token")
	}
	if page.Incarnation != state.Incarnation || page.Revision != state.ReleaseSetRevision {
		return 0, conflict("release set changed; restart pagination")
	}
	return page.Before, nil
}

// managementView 在同一存储快照上裁剪公开响应，禁止写操作回包携带无界历史正文。
// 默认最多 20 条，按 sequence 降序；草稿与单个详情均有独立定义大小上限。
func managementView(snapshot *store.UnifiedConfigSnapshot, req *configv1.GetStateRequest) *model.Response {
	if snapshot == nil || snapshot.State == nil {
		return stateResponse(nil, errors.New("store returned no management snapshot"))
	}
	state := snapshot.State
	limit := req.GetPageSize()
	if limit > 100 {
		return stateResponse(nil, invalid("page_size exceeds 100"))
	}
	if limit == 0 {
		limit = 20
	}
	before, err := decodeManagementPage(state, req.GetPageToken())
	if err != nil {
		return stateResponse(nil, err)
	}
	view := &configv1.ConfigState{Key: proto.Clone(state.Key).(*configv1.ConfigKey),
		ReleaseSetRevision: state.ReleaseSetRevision, Incarnation: state.Incarnation, Offline: state.Offline}
	if state.Draft != nil {
		view.Draft = proto.Clone(state.Draft).(*configv1.ConfigDraft)
	}
	response := view
	members := append([]*store.UnifiedConfigMember(nil), state.Members...)
	sort.Slice(members, func(i, j int) bool {
		return members[i].GetRelease().GetSequence() > members[j].GetRelease().GetSequence()
	})
	pageBytes := 0
	for _, member := range members {
		release := member.GetRelease()
		if release == nil || release.Id == "" || release.Sequence == 0 {
			return stateResponse(nil, errors.New("invalid release history"))
		}
		if member.Active {
			response.ActiveReleaseIds = append(response.ActiveReleaseIds, release.Id)
		}
		if release.Id == req.GetReleaseId() {
			response.SelectedRelease = proto.Clone(release).(*configv1.ConfigRelease)
		}
		if before != 0 && release.Sequence >= before {
			continue
		}
		// 先去掉大正文再克隆；公开成员是元数据，不是可用于渲染的发布快照。
		entry := &configv1.ReleaseSummary{Id: release.Id, Kind: release.Kind,
			Audience: release.Audience, Priority: release.Priority, Sequence: release.Sequence,
			CreatedAt: release.CreatedAt, CreatedBy: release.CreatedBy, SourceReleaseId: release.SourceReleaseId, Active: member.Active}
		size := proto.Size(&configv1.ConfigState{Releases: []*configv1.ReleaseSummary{entry}})
		if size > maxMemberPageBytes {
			return stateResponse(nil, errors.New("release metadata exceeds page capacity"))
		}
		if response.NextPageToken == "" && len(view.Releases) < int(limit) && pageBytes+size <= maxMemberPageBytes {
			view.Releases = append(view.Releases, proto.Clone(entry).(*configv1.ReleaseSummary))
			pageBytes += size
			continue
		}
		if response.NextPageToken == "" {
			last := view.Releases[len(view.Releases)-1].Sequence
			wire, _ := json.Marshal(managementPage{Key: state.Key, Incarnation: state.Incarnation, Revision: state.ReleaseSetRevision, Before: last})
			response.NextPageToken = base64.RawURLEncoding.EncodeToString(wire)
		}
	}
	if req.GetReleaseId() != "" && response.SelectedRelease == nil {
		return stateResponse(nil, missing("specified release does not exist in this configuration"))
	}
	return successResponse(response)
}
