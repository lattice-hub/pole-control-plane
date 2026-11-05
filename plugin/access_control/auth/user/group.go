/**
 * Tencent is pleased to support the open source community by making Polaris available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Clause License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Clause
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package defaultuser

import (
	"context"
	"fmt"
	"maps"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"

	cachetypes "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/pkg/types"
	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
	storeapi "github.com/pole-io/pole-server/apis/store"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/common/utils"
	commontime "github.com/pole-io/pole-server/pkg/common/utils/time"
	"github.com/pole-io/pole-server/pkg/common/utils/valid"
)

type (
	// UserGroup2Api is the user group to api
	UserGroup2Api func(user *authtypes.UserGroup) *apisecurity.UserGroup
)

// CreateGroups 批量创建用户组
func (svr *Server) CreateGroups(ctx context.Context, reqs []*apisecurity.UserGroup) *apimodel.BatchWriteResponse {
	resp := api.NewAuthBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for index := range reqs {
		ret := svr.CreateGroup(ctx, reqs[index])
		api.Collect(resp, ret)
	}
	return resp
}

// CreateGroup create a group
func (svr *Server) CreateGroup(ctx context.Context, req *apisecurity.UserGroup) *apimodel.Response {
	ownerID := utils.ParseOwnerID(ctx)
	req.Owner = ownerID
	if rsp := svr.preCheckGroupRelation(req.GetRelation()); rsp != nil {
		return rsp
	}

	// 根据 owner + groupname 确定唯一的用户组信息
	group, err := svr.storage.GetGroupByName(req.GetName())
	if err != nil {
		log.Error("get group when create", utils.RequestID(ctx), zap.Error(err))
		return api.NewGroupResponse(storeapi.StoreCode2APICode(err), req)
	}

	if group != nil {
		return api.NewGroupResponse(apimodel.Code_UserGroupExisted, req)
	}

	data, err := svr.createGroupModel(req)
	if err != nil {
		log.Error("create group model", utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthResponseWithMsg(apimodel.Code_ExecuteException, err.Error())
	}

	tx, err := svr.storage.StartTx()
	if err != nil {
		log.Error("[Auth][User] create user_group begion storage tx", utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthResponse(apimodel.Code_ExecuteException)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := svr.storage.AddGroup(tx, data); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return api.NewAuthResponseWithMsg(storeapi.StoreCode2APICode(err), err.Error())
	}
	if err := svr.policySvr.PolicyHelper().CreatePrincipalPolicy(ctx, tx, authtypes.Principal{
		PrincipalID:   data.ID,
		PrincipalType: authtypes.PrincipalGroup,
		Name:          data.Name,
	}); err != nil {
		log.Error("[Auth][User] add user_group default policy rule", utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthResponse(storeapi.StoreCode2APICode(err))
	}
	if err := tx.Commit(); err != nil {
		log.Error("[Auth][User] create user_group commit storage tx", utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthResponse(apimodel.Code_ExecuteException)
	}

	log.Info("create group", zap.String("name", req.Name), utils.RequestID(ctx))
	svr.RecordHistory(userGroupRecordEntry(ctx, req, data.UserGroup, types.OCreate))

	req.Id = data.ID
	return api.NewGroupResponse(apimodel.Code_ExecuteSuccess, req)
}

// UpdateGroups 批量修改用户组
func (svr *Server) UpdateGroups(
	ctx context.Context, groups []*apisecurity.UserGroup) *apimodel.BatchWriteResponse {
	resp := api.NewAuthBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for index := range groups {
		ret := svr.UpdateGroup(ctx, groups[index])
		api.Collect(resp, ret)
	}
	return resp
}

// UpdateGroup 更新用户组
func (svr *Server) UpdateGroup(ctx context.Context, req *apisecurity.UserGroup) *apimodel.Response {
	if checkErrResp := svr.checkUpdateGroup(ctx, req); checkErrResp != nil {
		return checkErrResp
	}

	saveData, errResp := svr.getGroupFromDB(req.Id)
	if errResp != nil {
		return errResp
	}

	updateData, err := svr.createGroupModel(req)
	if err != nil {
		log.Info("create update group model", utils.RequestID(ctx),
			zap.String("group-id", req.GetId()), zap.Error(err))
		return api.NewGroupResponse(apimodel.Code_ExecuteException, req)
	}

	saveData, needUpdate := updateGroupFromModel(ctx, saveData, updateData)
	if !needUpdate {
		log.Info("update group data no change, no need update",
			utils.RequestID(ctx), zap.String("group-id", req.GetId()))
		return api.NewGroupResponse(apimodel.Code_NoNeedUpdate, req)
	}

	if err := svr.storage.UpdateGroup(saveData); err != nil {
		log.Error("update group", zap.Error(err), utils.RequestID(ctx))
		return api.NewAuthResponseWithMsg(storeapi.StoreCode2APICode(err), err.Error())
	}

	log.Info("update group", zap.String("name", saveData.Name), utils.RequestID(ctx))
	svr.RecordHistory(modifyUserGroupRecordEntry(ctx, req, saveData.UserGroup, types.OUpdateGroup))

	return api.NewGroupResponse(apimodel.Code_ExecuteSuccess, req)
}

// DeleteGroups 批量删除用户组
func (svr *Server) DeleteGroups(ctx context.Context, reqs []*apisecurity.UserGroup) *apimodel.BatchWriteResponse {
	resp := api.NewAuthBatchWriteResponse(apimodel.Code_ExecuteSuccess)
	for index := range reqs {
		ret := svr.DeleteGroup(ctx, reqs[index])
		api.Collect(resp, ret)
	}

	return resp
}

// DeleteGroup 删除用户组
func (svr *Server) DeleteGroup(ctx context.Context, req *apisecurity.UserGroup) *apimodel.Response {
	group, err := svr.storage.GetGroup(req.GetId())
	if err != nil {
		log.Error("get group from store", utils.RequestID(ctx), zap.Error(err))
		return api.NewGroupResponse(storeapi.StoreCode2APICode(err), req)
	}
	if group == nil {
		return api.NewGroupResponse(apimodel.Code_ExecuteSuccess, req)
	}
	tx, err := svr.storage.StartTx()
	if err != nil {
		log.Error("[Auth][User] delete user_group begion storage tx", utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthResponse(apimodel.Code_ExecuteException)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := svr.storage.DeleteGroup(tx, group); err != nil {
		log.Error("delete group from store", utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthResponseWithMsg(storeapi.StoreCode2APICode(err), err.Error())
	}
	if err := svr.policySvr.PolicyHelper().CleanPrincipal(ctx, tx, authtypes.Principal{
		PrincipalID:   group.ID,
		PrincipalType: authtypes.PrincipalGroup,
	}); err != nil {
		log.Error("[Auth][User] delete user_group from policy server", utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthResponse(storeapi.StoreCode2APICode(err))
	}

	if err := tx.Commit(); err != nil {
		log.Error("[Auth][User] delete user_group commit storage tx", utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthResponse(apimodel.Code_ExecuteException)
	}

	log.Info("delete group", utils.RequestID(ctx), zap.String("name", req.Name))
	svr.RecordHistory(userGroupRecordEntry(ctx, req, group.UserGroup, types.ODelete))

	return api.NewGroupResponse(apimodel.Code_ExecuteSuccess, req)
}

// GetGroups 查看用户组
func (svr *Server) GetGroups(ctx context.Context, filters map[string]string) *apimodel.BatchQueryResponse {
	offset, limit, _ := valid.ParseOffsetAndLimit(filters)
	total, groups, err := svr.cacheMgr.User().QueryUserGroups(ctx, cachetypes.UserGroupSearchArgs{
		Filters: filters,
		Offset:  offset,
		Limit:   limit,
	})
	if err != nil {
		log.Error("[Auth][Group] list user_group from store", utils.RequestID(ctx),
			zap.Any("filters", filters), zap.Error(err))
		return api.NewAuthBatchQueryResponse(storeapi.StoreCode2APICode(err))
	}

	resp := api.NewAuthBatchQueryResponse(apimodel.Code_ExecuteSuccess)
	resp.Amount = total
	resp.Size = uint32(len(groups))
	for i := range groups {
		if err := api.AddAnyDataIntoBatchQuery(resp, userGroup2Api(groups[i].UserGroup)); err != nil {
			log.Error("[Auth][Group] add user_group to query list", utils.RequestID(ctx), zap.Error(err))
			return api.NewAuthBatchQueryResponse(apimodel.Code_ExecuteException)
		}
	}
	return resp
}

// GetGroup 查看对应用户组下的用户信息
func (svr *Server) GetGroup(ctx context.Context, req *apisecurity.UserGroup) *apimodel.Response {
	if req.GetId() == "" {
		return api.NewAuthResponse(apimodel.Code_InvalidUserGroupID)
	}
	group, errResp := svr.getGroupFromDB(req.Id)
	if errResp != nil {
		return errResp
	}
	return api.NewGroupResponse(apimodel.Code_ExecuteSuccess, svr.userGroupDetail2Api(group))
}

// GetGroupToken 查看用户组的token
func (svr *Server) GetGroupToken(ctx context.Context, req *apisecurity.UserGroup) *apimodel.Response {
	if req.GetId() == "" {
		return api.NewAuthResponse(apimodel.Code_InvalidUserGroupID)
	}

	group := svr.cacheMgr.User().GetGroup(req.Id)
	if group == nil {
		return api.NewGroupResponse(apimodel.Code_NotFoundUserGroup, req)
	}

	req.AuthToken = group.Token
	req.TokenEnable = group.TokenEnable

	return api.NewGroupResponse(apimodel.Code_ExecuteSuccess, req)
}

// EnableGroupToken 调整用户组 token 的使用状态 (禁用｜开启)
func (svr *Server) EnableGroupToken(ctx context.Context, req *apisecurity.UserGroup) *apimodel.Response {
	group, errResp := svr.getGroupFromDB(req.Id)
	if errResp != nil {
		return errResp
	}

	group.TokenEnable = req.TokenEnable

	modifyReq := &authtypes.UserGroupDetail{
		UserGroup: &authtypes.UserGroup{
			ID:          group.ID,
			Token:       group.Token,
			TokenEnable: group.TokenEnable,
			Comment:     group.Comment,
		},
	}

	if err := svr.storage.UpdateGroup(modifyReq); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return api.NewAuthResponseWithMsg(storeapi.StoreCode2APICode(err), err.Error())
	}

	log.Info("update group token", zap.String("id", req.Id),
		zap.Bool("enable", group.TokenEnable), utils.RequestID(ctx))
	svr.RecordHistory(userGroupRecordEntry(ctx, req, group.UserGroup, types.OUpdateToken))

	return api.NewGroupResponse(apimodel.Code_ExecuteSuccess, req)
}

// ResetGroupToken 刷新用户组的token
func (svr *Server) ResetGroupToken(ctx context.Context, req *apisecurity.UserGroup) *apimodel.Response {
	var (
		group, errResp = svr.getGroupFromDB(req.Id)
	)

	if errResp != nil {
		return errResp
	}

	newToken, err := createGroupToken(group.ID, svr.authOpt.Salt)
	if err != nil {
		log.Error("reset group token", utils.RequestID(ctx), zap.Error(err))
		return api.NewAuthResponseWithMsg(apimodel.Code_ExecuteException, err.Error())
	}

	group.Token = newToken
	modifyReq := &authtypes.UserGroupDetail{
		UserGroup: &authtypes.UserGroup{
			ID:          group.ID,
			Token:       group.Token,
			TokenEnable: group.TokenEnable,
			Comment:     group.Comment,
		},
	}

	if err := svr.storage.UpdateGroup(modifyReq); err != nil {
		log.Error(err.Error(), utils.RequestID(ctx))
		return api.NewAuthResponseWithMsg(storeapi.StoreCode2APICode(err), err.Error())
	}

	log.Info("reset group token", zap.String("group-id", req.Id),
		utils.RequestID(ctx))
	svr.RecordHistory(userGroupRecordEntry(ctx, req, group.UserGroup, types.OUpdate))

	req.AuthToken = newToken
	return api.NewGroupResponse(apimodel.Code_ExecuteSuccess, req)
}

// getGroupFromDB 获取用户组
func (svr *Server) getGroupFromDB(id string) (*authtypes.UserGroupDetail, *apimodel.Response) {
	group, err := svr.storage.GetGroup(id)
	if err != nil {
		log.Error("get group from store", zap.Error(err))
		return nil, api.NewAuthResponseWithMsg(storeapi.StoreCode2APICode(err), err.Error())
	}
	if group == nil {
		return nil, api.NewAuthResponse(apimodel.Code_NotFoundUserGroup)
	}
	return group, nil
}

// preCheckGroupRelation 检查用户-用户组关联关系中，对应的用户信息是否存在，即不能添加一个不存在的用户到用户组
func (svr *Server) preCheckGroupRelation(req *apisecurity.UserGroupRelation) *apimodel.Response {
	// 检查该关系中所有的用户是否存在
	uIDs := make([]string, len(req.GetUsers()))
	for i := range req.GetUsers() {
		uIDs[i] = req.GetUsers()[i].GetId()
	}

	uIDs = utils.StringSliceDeDuplication(uIDs)
	for i := range uIDs {
		user := svr.cacheMgr.User().GetUserByID(uIDs[i])
		if user == nil {
			return api.NewGroupRelationResponse(apimodel.Code_NotFoundUser, req)
		}
	}

	return nil
}

// checkUpdateGroup 检查用户组的更新请求
func (svr *Server) checkUpdateGroup(ctx context.Context, req *apisecurity.UserGroup) *apimodel.Response {
	if req == nil {
		return api.NewGroupResponse(apimodel.Code_EmptyRequest, req)
	}
	if req.Id == "" {
		return api.NewGroupResponse(apimodel.Code_InvalidUserGroupID, req)
	}
	if rsp := svr.preCheckGroupRelation(req.GetRelation()); rsp != nil {
		return rsp
	}
	return nil
}

// updateGroupAttribute 更新计算用户组更新时的结构体数据，并判断是否需要执行更新操作
func updateGroupFromModel(ctx context.Context, saveData, newData *authtypes.UserGroupDetail) (*authtypes.UserGroupDetail, bool) {
	// Token state is changed only by EnableGroupToken, not ordinary edits.
	newData.TokenEnable = saveData.TokenEnable
	return updateGroupAttribute(ctx, saveData, newData)
}

func updateGroupAttribute(ctx context.Context, saveData, newData *authtypes.UserGroupDetail) (
	*authtypes.UserGroupDetail, bool) {
	needUpdate := false

	// 通过 UpdateUserGroups 权限检查的操作者可以修改用户组备注。
	// 不能以 owner 身份或旧值是否为空作为额外条件，否则 Console
	// 会在返回 HTTP 200 后静默丢弃已授权的备注修改。
	if saveData.Comment != newData.Comment {
		needUpdate = true
		saveData.Comment = newData.Comment
	}

	if saveData.Source != newData.Source {
		needUpdate = true
		saveData.Source = newData.Source
	}

	if saveData.TokenEnable != newData.TokenEnable {
		needUpdate = true
		saveData.TokenEnable = newData.TokenEnable
	}

	if !maps.Equal(saveData.Metadata, newData.Metadata) {
		needUpdate = true
		saveData.Metadata = newData.Metadata
	}

	if !maps.Equal(saveData.UserIds, newData.UserIds) {
		needUpdate = true
		saveData.UserIds = newData.UserIds
	}
	return saveData, needUpdate
}

// enhancedGroups2Api 数组专为 []*apisecurity.UserGroup
func enhancedGroups2Api(groups []*authtypes.UserGroupDetail, handler UserGroup2Api) []*apisecurity.UserGroup {
	out := make([]*apisecurity.UserGroup, 0, len(groups))
	for k := range groups {
		out = append(out, handler(groups[k].UserGroup))
	}

	return out
}

// createGroupModel 创建用户组的存储模型
func (svr *Server) createGroupModel(req *apisecurity.UserGroup) (group *authtypes.UserGroupDetail, err error) {
	ids := make(map[string]struct{}, len(req.GetRelation().GetUsers()))
	for index := range req.GetRelation().GetUsers() {
		ids[req.GetRelation().GetUsers()[index].GetId()] = struct{}{}
	}

	group = &authtypes.UserGroupDetail{
		UserGroup: &authtypes.UserGroup{
			ID:          utils.NewUUID(),
			Name:        req.GetName(),
			TokenEnable: true,
			Valid:       true,
			Comment:     req.GetComment(),
			CreateTime:  time.Now(),
			ModifyTime:  time.Now(),
			Metadata:    req.GetMetadata(),
		},
		UserIds: ids,
	}

	if group.Token, err = createGroupToken(group.ID, svr.authOpt.Salt); err != nil {
		return nil, err
	}
	return group, nil
}

// model.UserGroup 转为 api.UserGroup
func userGroup2Api(group *authtypes.UserGroup) *apisecurity.UserGroup {
	if group == nil {
		return nil
	}

	// note: 不包括token，token比较特殊
	out := &apisecurity.UserGroup{
		Id:          group.ID,
		Name:        group.Name,
		TokenEnable: group.TokenEnable,
		Comment:     group.Comment,
		Ctime:       commontime.Time2String(group.CreateTime),
		Mtime:       commontime.Time2String(group.ModifyTime),
	}

	return out
}

// model.UserGroupDetail 转为 api.UserGroup，并且主动填充 user 的信息数据
func (svr *Server) userGroupDetail2Api(group *authtypes.UserGroupDetail) *apisecurity.UserGroup {
	if group == nil {
		return nil
	}

	users := make([]*apisecurity.User, 0, len(group.UserIds))
	for id := range group.UserIds {
		user := svr.cacheMgr.User().GetUserByID(id)
		users = append(users, &apisecurity.User{
			Id:          user.ID,
			Name:        user.Name,
			Source:      user.Source,
			Comment:     user.Comment,
			TokenEnable: user.TokenEnable,
			Ctime:       commontime.Time2String(user.CreateTime),
			Mtime:       commontime.Time2String(user.ModifyTime),
		})
	}

	// note: 不包括token，token比较特殊
	out := &apisecurity.UserGroup{
		Id:          group.ID,
		Name:        group.Name,
		TokenEnable: group.TokenEnable,
		Comment:     group.Comment,
		Ctime:       commontime.Time2String(group.CreateTime),
		Mtime:       commontime.Time2String(group.ModifyTime),
		Relation: &apisecurity.UserGroupRelation{
			Users: users,
		},
		Metadata:  group.Metadata,
		UserCount: uint32(len(users)),
	}

	return out
}

// userGroupRecordEntry 生成用户组的记录entry
func userGroupRecordEntry(ctx context.Context, req *apisecurity.UserGroup, md *authtypes.UserGroup,
	operationType types.OperationType) *types.RecordEntry {

	detailBytes, _ := protojson.Marshal(req)
	datail := types.SanitizeOperationDetail(types.RUserGroup, string(detailBytes))

	entry := &types.RecordEntry{
		ResourceType:  types.RUserGroup,
		ResourceName:  fmt.Sprintf("%s(%s)", md.Name, md.ID),
		OperationType: operationType,
		Operator:      utils.ParseOperator(ctx),
		Detail:        datail,
		HappenTime:    time.Now(),
	}

	return entry
}

// 生成修改用户组的记录entry
func modifyUserGroupRecordEntry(ctx context.Context, req *apisecurity.UserGroup, md *authtypes.UserGroup,
	operationType types.OperationType) *types.RecordEntry {

	detailBytes, _ := protojson.Marshal(req)
	detail := types.SanitizeOperationDetail(types.RUserGroup, string(detailBytes))

	entry := &types.RecordEntry{
		ResourceType:  types.RUserGroup,
		ResourceName:  fmt.Sprintf("%s(%s)", md.Name, md.ID),
		OperationType: operationType,
		Operator:      utils.ParseOperator(ctx),
		Detail:        detail,
		HappenTime:    time.Now(),
	}

	return entry
}

// 生成用户-用户组关联关系的记录entry
func userRelationRecordEntry(ctx context.Context, req *apisecurity.UserGroupRelation, md *authtypes.UserGroup,
	operationType types.OperationType) *types.RecordEntry {

	detailBytes2, _ := protojson.Marshal(req)
	detail := types.SanitizeOperationDetail(types.RUserGroupRelation, string(detailBytes2))

	entry := &types.RecordEntry{
		ResourceType:  types.RUserGroupRelation,
		ResourceName:  fmt.Sprintf("%s(%s)", md.Name, md.ID),
		OperationType: operationType,
		Operator:      utils.ParseOperator(ctx),
		Detail:        detail,
		HappenTime:    time.Now(),
	}

	return entry
}

func defaultUserGroupPolicy(u *authtypes.UserGroupDetail) *authtypes.StrategyDetail {
	// Create the user's default weight policy
	return &authtypes.StrategyDetail{
		ID:        utils.NewUUID(),
		Name:      authtypes.BuildDefaultStrategyName(authtypes.PrincipalGroup, u.Name),
		Action:    apisecurity.AuthAction_READ_WRITE.String(),
		Default:   true,
		Revision:  utils.NewUUID(),
		Resources: []authtypes.StrategyResource{},
		Valid:     true,
		Comment:   "Default Strategy",
	}
}
