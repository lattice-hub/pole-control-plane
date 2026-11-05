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

package httpserver

import (
	"github.com/golang/protobuf/proto"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"
	apiservice "github.com/pole-io/specification/source/go/api/v1/service_manage"
)

// NamespaceArr 命名空间数组定义
type NamespaceArr []*apimodel.Namespace

// Reset 重置初始化
func (m *NamespaceArr) Reset() { *m = NamespaceArr{} }

// String return string
func (m *NamespaceArr) String() string { return proto.CompactTextString(m) }

// ProtoMessage return proto message
func (*NamespaceArr) ProtoMessage() {}

// ConfigReleaseArr 发布规则数组定义
type ConfigReleaseArr []*apiservice.Service

// Reset reset initialization
func (m *ConfigReleaseArr) Reset() { *m = ConfigReleaseArr{} }

// String return string
func (m *ConfigReleaseArr) String() string { return proto.CompactTextString(m) }

// ProtoMessage return proto message
func (*ConfigReleaseArr) ProtoMessage() {}

// ServiceAliasArr 服务实例数组定义
type ServiceAliasArr []*apiservice.ServiceAlias

// Reset reset initialization
func (m *ServiceAliasArr) Reset() { *m = ServiceAliasArr{} }

// String return string
func (m *ServiceAliasArr) String() string { return proto.CompactTextString(m) }

// ProtoMessage proto message
func (*ServiceAliasArr) ProtoMessage() {}

// UserArr 命名空间数组定义
type UserArr []*apisecurity.User

// Reset 清空数组
func (m *UserArr) Reset() { *m = UserArr{} }

// String return string
func (m *UserArr) String() string { return proto.CompactTextString(m) }

// ProtoMessage return proto message
func (*UserArr) ProtoMessage() {}

// UserGroupArr 命名空间数组定义
type UserGroupArr []*apisecurity.UserGroup

// Reset 清空数组
func (m *UserGroupArr) Reset() { *m = UserGroupArr{} }

// String return string
func (m *UserGroupArr) String() string { return proto.CompactTextString(m) }

// ProtoMessage return proto message
func (*UserGroupArr) ProtoMessage() {}

// ModifyGroupArr 命名空间数组定义
type ModifyGroupArr []*apisecurity.ModifyUserGroup

// Reset 清空数组
func (m *ModifyGroupArr) Reset() { *m = ModifyGroupArr{} }

// String return string
func (m *ModifyGroupArr) String() string { return proto.CompactTextString(m) }

// ProtoMessage return proto message
func (*ModifyGroupArr) ProtoMessage() {}

// StrategyArr 命名空间数组定义
type StrategyArr []*apisecurity.AuthStrategy

// Reset 清空数组
func (m *StrategyArr) Reset() { *m = StrategyArr{} }

// String return string
func (m *StrategyArr) String() string { return proto.CompactTextString(m) }

// ProtoMessage return proto message
func (*StrategyArr) ProtoMessage() {}

// ModifyStrategyArr 命名空间数组定义
type ModifyStrategyArr []*apisecurity.ModifyAuthStrategy

// Reset 清空数组
func (m *ModifyStrategyArr) Reset() { *m = ModifyStrategyArr{} }

// String return string
func (m *ModifyStrategyArr) String() string { return proto.CompactTextString(m) }

// ProtoMessage return proto message
func (*ModifyStrategyArr) ProtoMessage() {}

// AuthResourceArr 命名空间数组定义
type AuthResourceArr []*apisecurity.StrategyResources

// Reset 清空数组
func (m *AuthResourceArr) Reset() { *m = AuthResourceArr{} }

// String return string
func (m *AuthResourceArr) String() string { return proto.CompactTextString(m) }

// ProtoMessage return proto message
func (*AuthResourceArr) ProtoMessage() {}
