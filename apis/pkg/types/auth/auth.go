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

package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"

	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/apis/pkg/utils"
)

var (
	// ErrorNoUser 没有找到对应的用户
	ErrorNoUser error = errors.New("no such user")

	// ErrorNoUserGroup 没有找到对应的用户组
	ErrorNoUserGroup error = errors.New("no such user group")
	// ErrorWrongUsernameOrPassword 用户或者密码错误
	ErrorWrongUsernameOrPassword error = errors.New("name or password is wrong")
	// ErrorTokenNotExist token 不存在
	ErrorTokenNotExist error = errors.New("token not exist")
	// ErrorTokenInvalid 非法的 token
	ErrorTokenInvalid error = errors.New("invalid token")
	// ErrorTokenDisabled token 已经被禁用
	ErrorTokenDisabled error = errors.New("token already disabled")
)

func ConvertToErrCode(err error) apimodel.Code {
	if errors.Is(err, ErrorTokenNotExist) || errors.Is(err, ErrorTokenInvalid) {
		return apimodel.Code_TokenNotExisted
	}

	if errors.Is(err, ErrorTokenDisabled) {
		return apimodel.Code_TokenDisabled
	}

	return apimodel.Code_NotAllowedAccess
}

const (
	OperatorRoleKey      string = "operator_role"
	OperatorIDKey        string = "operator_id"
	OperatorOwnerKey     string = "operator_owner"
	OperatorLinkStrategy string = "operator_link_strategy"
	PrincipalKey         string = "principal"
	LinkUsersKey         string = "link_users"
	LinkGroupsKey        string = "link_groups"
	RemoveLinkUsersKey   string = "remove_link_users"
	RemoveLinkGroupsKey  string = "remove_link_groups"

	TokenDetailInfoKey string = "TokenInfo"
	TokenForUser       string = "uid"
	TokenForUserGroup  string = "groupid"

	ResourceAttachmentKey string = "resource_attachment"

	PermissionCheckPurposeKey        string = "permission_check_purpose"
	PermissionCheckPurposeCapability string = "capability"
)

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[PrincipalUser-1]
	_ = x[PrincipalGroup-2]
}

const _PrincipalType_name = "PrincipalUserPrincipalGroup"

var _PrincipalType_index = [...]uint8{0, 13, 27}

func (i PrincipalType) String() string {
	i -= 1
	if i < 0 || i >= PrincipalType(len(_PrincipalType_index)-1) {
		return "PrincipalType(" + strconv.FormatInt(int64(i+1), 10) + ")"
	}
	return _PrincipalType_name[_PrincipalType_index[i]:_PrincipalType_index[i+1]]
}

//go:generate stringer -type=PrincipalType
type PrincipalType int

const (
	PrincipalUser  PrincipalType = 1
	PrincipalGroup PrincipalType = 2
	PrincipalRole  PrincipalType = 3
)

// CheckPrincipalType 检查鉴权策略成员角色信息
func CheckPrincipalType(role int) error {
	switch PrincipalType(role) {
	case PrincipalUser:
		return nil
	case PrincipalGroup:
		return nil
	case PrincipalRole:
		return nil
	default:
		return errors.New("invalid principal type")
	}
}

var (
	// PrincipalNames principal name map
	PrincipalNames = map[PrincipalType]string{
		PrincipalUser:  "user",
		PrincipalGroup: "group",
		PrincipalRole:  "role",
	}
)

const (

	// DefaultStrategySuffix 默认策略的名称前缀
	DefaultStrategySuffix string = "的默认策略"
)

// BuildDefaultStrategyName 构建默认鉴权策略的名称信息
func BuildDefaultStrategyName(role PrincipalType, name string) string {
	if role == PrincipalUser {
		return fmt.Sprintf("%s%s%s", "(用户) ", name, DefaultStrategySuffix)
	}
	return fmt.Sprintf("%s%s%s", "(用户组) ", name, DefaultStrategySuffix)
}

// ResourceOperation 资源操作
type ResourceOperation int16

const (
	// Read 只读动作
	Read ResourceOperation = 10
	// Create 创建动作
	Create ResourceOperation = 20
	// Modify 修改动作
	Modify ResourceOperation = 30
	// Delete 删除动作
	Delete ResourceOperation = 40
)

// BzModule 模块标识
type BzModule int16

const (
	// UnknowModule 未知模块
	UnknowModule BzModule = iota
	// CoreModule 核心模块
	CoreModule
	// DiscoverModule 服务模块
	DiscoverModule
	// ConfigModule 配置模块
	ConfigModule
	// AuthModule 鉴权模块
	AuthModule
	// MaintainModule 运维操作模块
	MaintainModule
	// BootstrapModule 初始化模块
	BootstrapModule
)

// UserRoleType 用户角色类型
type UserRoleType int

const (
	UnknownUserRole    UserRoleType = -1
	OwnerUserRole      UserRoleType = 20
	SubAccountUserRole UserRoleType = 50
)

var (
	UserRoleNames = map[UserRoleType]string{
		OwnerUserRole:      "main",
		SubAccountUserRole: "sub",
	}
)

// ResourceEntry 资源最简单信息
type ResourceEntry struct {
	Type     apisecurity.ResourceType
	ID       string
	Owner    string
	Metadata map[string]string
}

// User 用户
type User struct {
	ID          string
	Name        string
	Password    string
	Source      string
	Mobile      string
	Email       string
	Type        UserRoleType
	Metadata    map[string]string
	Token       string
	TokenEnable bool
	Valid       bool
	Comment     string
	CreateTime  time.Time
	ModifyTime  time.Time
}

func (u *User) GetToken() string {
	return u.Token
}

func (u *User) Disable() bool {
	return !u.TokenEnable
}

func (u *User) SelfID() string {
	return u.ID
}

func (u *User) ToSpec() *apisecurity.User {
	if u == nil {
		return nil
	}
	return &apisecurity.User{
		Id:          u.ID,
		Name:        u.Name,
		Password:    u.Password,
		Source:      u.Source,
		AuthToken:   u.Token,
		TokenEnable: u.TokenEnable,
		Comment:     u.Comment,
		UserType:    fmt.Sprintf("%d", u.Type),
	}
}

// UserGroupDetail 用户组详细（带用户列表）
type UserGroupDetail struct {
	*UserGroup

	// UserIds改为 map 的形式，加速查询
	UserIds map[string]struct{}
}

// ToUserIdSlice 将用户ID Map 专为 slice
func (ugd *UserGroupDetail) ToUserIdSlice() []string {
	uids := make([]string, 0, len(ugd.UserIds))
	for uid := range ugd.UserIds {
		uids = append(uids, uid)
	}
	return uids
}

func (ugd *UserGroupDetail) ListSpecUser() []*apisecurity.User {
	users := make([]*apisecurity.User, 0, len(ugd.UserIds))
	for i := range ugd.UserIds {
		users = append(users, &apisecurity.User{
			Id: i,
		})
	}
	return users
}

// ToSpec 将用户ID Map 专为 slice
func (ugd *UserGroupDetail) ToSpec() *apisecurity.UserGroup {
	if ugd == nil {
		return nil
	}
	return &apisecurity.UserGroup{
		Id:          ugd.ID,
		Name:        ugd.Name,
		AuthToken:   ugd.Token,
		TokenEnable: ugd.TokenEnable,
		Comment:     ugd.Comment,
		Ctime:       utils.Time2String(ugd.CreateTime),
		Mtime:       utils.Time2String(ugd.ModifyTime),
		Relation: &apisecurity.UserGroupRelation{
			GroupId: ugd.ID,
			Users:   ugd.ListSpecUser(),
		},
		UserCount: uint32(len(ugd.UserIds)),
	}
}

// UserGroup 用户组
type UserGroup struct {
	ID          string
	Name        string
	Token       string
	TokenEnable bool
	Metadata    map[string]string
	Valid       bool
	Comment     string
	Source      string
	CreateTime  time.Time
	ModifyTime  time.Time
}

func (u *UserGroup) GetToken() string {
	return u.Token
}

func (u *UserGroup) Disable() bool {
	return !u.TokenEnable
}

func (u *UserGroup) SelfID() string {
	return u.ID
}

// ModifyUserGroup 用户组修改
type ModifyUserGroup struct {
	ID            string
	Token         string
	TokenEnable   bool
	Comment       string
	Metadata      map[string]string
	AddUserIds    []string
	RemoveUserIds []string
}

// UserGroupRelation 用户-用户组关联关系具体信息
type UserGroupRelation struct {
	GroupID    string
	UserIds    []string
	CreateTime time.Time
	ModifyTime time.Time
}

type Condition struct {
	Key         string
	Value       string
	CompareFunc string
}

// StrategyDetail 鉴权策略详细
type StrategyDetail struct {
	ID   string
	Name string
	// Action: 只有 allow 以及 deny
	Action  string
	Comment string
	Default bool
	// 来源
	Source string
	// CalleeMethods 允许访问的服务端接口
	CalleeMethods []string
	Resources     []StrategyResource
	Conditions    []Condition
	Principals    []Principal
	Valid         bool
	Revision      string
	Metadata      map[string]string
	CreateTime    time.Time
	ModifyTime    time.Time
}

func (s *StrategyDetail) GetAction() apisecurity.AuthAction {
	if s.Action == apisecurity.AuthAction_ALLOW.String() {
		return apisecurity.AuthAction_ALLOW
	}
	if s.Action == apisecurity.AuthAction_READ_WRITE.String() {
		return apisecurity.AuthAction_ALLOW
	}
	return apisecurity.AuthAction_DENY
}

func ParsePolicyRule(req *apisecurity.AuthStrategy) *StrategyDetail {
	s := &StrategyDetail{}
	s.FromSpec(req)
	return s
}

func (s *StrategyDetail) FromSpec(req *apisecurity.AuthStrategy) {
	s.ID = utils.NewUUID()
	s.Name = req.Name
	s.Action = req.GetAction().String()
	s.Comment = req.Comment
	s.Default = false
	s.Valid = true
	s.Source = req.Source
	s.Revision = utils.NewUUID()
	s.CreateTime = time.Now()
	s.ModifyTime = time.Now()
	s.Metadata = req.Metadata
	s.CalleeMethods = req.GetFunctions()
	s.Conditions = make([]Condition, 0, len(req.GetResourceLabels()))
	for i := range req.GetResourceLabels() {
		item := req.GetResourceLabels()[i]
		s.Conditions = append(s.Conditions, Condition{
			Key:         item.Key,
			Value:       item.Value,
			CompareFunc: item.CompareType,
		})
	}

	// 收集涉及的资源信息
	resEntry := make([]StrategyResource, 0, 20)
	for resType, ptrGetter := range resourceFieldPointerGetters {
		slicePtr := ptrGetter(req.Resources)
		if slicePtr.Elem().IsNil() {
			continue
		}
		resEntry = append(resEntry, collectResourceEntry(s.ID, resType, slicePtr.Elem(), false)...)
	}

	// 收集涉及的 principal 信息
	principals := make([]Principal, 0, 20)
	principals = append(principals, collectPrincipalEntry(s.ID, PrincipalUser,
		req.GetPrincipals().GetUsers())...)
	principals = append(principals, collectPrincipalEntry(s.ID, PrincipalGroup,
		req.GetPrincipals().GetGroups())...)
	principals = append(principals, collectPrincipalEntry(s.ID, PrincipalRole,
		req.GetPrincipals().GetRoles())...)

	s.Resources = resEntry
	s.Principals = principals
}

func (s *StrategyDetail) IsMatchAction(a string) bool {
	saveAction := s.Action
	if isAllowAction(saveAction) {
		saveAction = apisecurity.AuthAction_ALLOW.String()
	}
	if isAllowAction(a) {
		a = apisecurity.AuthAction_ALLOW.String()
	}
	return saveAction == a
}

func isAllowAction(s string) bool {
	switch s {
	case apisecurity.AuthAction_ALLOW.String(), apisecurity.AuthAction_READ_WRITE.String():
		return true
	default:
		return false
	}
}

func (s *StrategyDetail) IsDeny() bool {
	return s.Action == apisecurity.AuthAction_DENY.String()
}

// ModifyStrategyDetail 修改鉴权策略详细
type ModifyStrategyDetail struct {
	ID               string
	Name             string
	Action           string
	Comment          string
	Metadata         map[string]string
	CalleeMethods    []string
	Conditions       []Condition
	AddPrincipals    []Principal
	RemovePrincipals []Principal
	AddResources     []StrategyResource
	RemoveResources  []StrategyResource
	ModifyTime       time.Time
}

// Strategy 策略main信息
type Strategy struct {
	ID         string
	Name       string
	Principal  string
	Action     string
	Comment    string
	Owner      string
	Default    bool
	Valid      bool
	CreateTime time.Time
	ModifyTime time.Time
}

// StrategyResource 策略资源
type StrategyResource struct {
	StrategyID string
	ResType    apisecurity.ResourceType
	ResID      string
}

func (s StrategyResource) Key() string {
	return s.ResType.String() + "/" + s.ResID
}

// Principal 策略相关人
type Principal struct {
	StrategyID    string
	Name          string
	Owner         string
	PrincipalID   string
	PrincipalType PrincipalType
	Extend        map[string]string
}

func NewAnonymousPrincipal() Principal {
	return Principal{
		Name:          "__anonymous__",
		PrincipalType: PrincipalUser,
		PrincipalID:   "__anonymous__",
		Extend:        map[string]string{},
	}
}

func (p Principal) String() string {
	return fmt.Sprintf("%s/%s", p.PrincipalType.String(), p.PrincipalID)
}

// ParseUserRole 从ctx中解析用户角色
func ParseUserRole(ctx context.Context) UserRoleType {
	if ctx == nil {
		return SubAccountUserRole
	}

	role, _ := ctx.Value(types.ContextUserRoleIDKey).(UserRoleType)
	return role
}

const (
	// RoleTypeSystem 系统角色
	RoleTypeSystem int = 1
	// RoleTypeCustom 自定义角色
	RoleTypeCustom int = 2
)

type Role struct {
	ID         string
	Name       string
	Owner      string
	Source     string
	Type       int
	Metadata   map[string]string
	Valid      bool
	Comment    string
	CreateTime time.Time
	ModifyTime time.Time
	Users      []Principal
	UserGroups []Principal
}

func (r *Role) FromSpec(d *apisecurity.Role) {
	r.ID = d.Id
	r.Name = d.Name
	r.Owner = d.Owner
	r.Source = d.Source
	r.Metadata = d.Metadata
	r.Type = RoleTypeCustom
	r.Comment = d.Comment

	if len(d.Users) != 0 {
		users := make([]Principal, 0, len(d.Users))
		for i := range d.Users {
			users = append(users, Principal{PrincipalID: d.Users[i].Id})
		}
		r.Users = users
	}

	if len(d.UserGroups) != 0 {
		groups := make([]Principal, 0, len(d.UserGroups))
		for i := range d.UserGroups {
			groups = append(groups, Principal{PrincipalID: d.UserGroups[i].Id})
		}
		r.UserGroups = groups
	}
}

func (r *Role) ToSpec() *apisecurity.Role {
	d := &apisecurity.Role{}

	d.Id = r.ID
	d.Name = r.Name
	d.Owner = r.Owner
	d.Source = r.Source
	d.Metadata = r.Metadata
	d.Comment = r.Comment
	d.DefaultRole = r.Type == RoleTypeSystem
	d.Ctime = utils.Time2String(r.CreateTime)
	d.Mtime = utils.Time2String(r.ModifyTime)

	if len(r.Users) != 0 {
		users := make([]*apisecurity.User, 0, len(r.Users))
		for i := range r.Users {
			users = append(users, &apisecurity.User{
				Id: r.Users[i].PrincipalID,
			})
		}
		d.Users = users
	}

	if len(r.UserGroups) != 0 {
		groups := make([]*apisecurity.UserGroup, 0, len(r.UserGroups))
		for i := range r.UserGroups {
			groups = append(groups, &apisecurity.UserGroup{
				Id: r.UserGroups[i].PrincipalID,
			})
		}
		d.UserGroups = groups
	}

	return d
}
