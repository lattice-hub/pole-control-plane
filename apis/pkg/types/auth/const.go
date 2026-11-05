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
	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"
)

const (
	MetadKeySystemDefaultPolicy = "internal-polaris-system-policy"
)

type ServerFunctionName string

// SDK 接口
const (
	DescribeSystemConfiguration ServerFunctionName = "DescribeSystemConfiguration"

	// 注册发现接口
	RegisterInstance      ServerFunctionName = "RegisterInstance"
	DeregisterInstance    ServerFunctionName = "DeregisterInstance"
	ReportServiceContract ServerFunctionName = "ReportServiceContract"
	DiscoverServices      ServerFunctionName = "DiscoverServices"
	DiscoverInstances     ServerFunctionName = "DiscoverInstances"
	UpdateInstance        ServerFunctionName = "UpdateInstance"

	// 服务治理接口
	DiscoverRouterRule          ServerFunctionName = "DiscoverRouterRule"
	DiscoverRateLimitRule       ServerFunctionName = "DiscoverRateLimitRule"
	DiscoverCircuitBreakerRule  ServerFunctionName = "DiscoverCircuitBreakerRule"
	DiscoverFaultDetectRule     ServerFunctionName = "DiscoverFaultDetectRule"
	DiscoverServiceContract     ServerFunctionName = "DiscoverServiceContract"
	DiscoverLaneRule            ServerFunctionName = "DiscoverLaneRule"
	DiscoverLosslessRule        ServerFunctionName = "DiscoverLosslessRule"
	DiscoverTrafficSecurityRule ServerFunctionName = "DiscoverTrafficSecurityRule"
	DiscoverTrafficMirrorRule   ServerFunctionName = "DiscoverTrafficMirrorRule"
	DiscoverTrafficMockRule     ServerFunctionName = "DiscoverTrafficMockRule"
)

// 命名空间
const (
	CreateNamespace        ServerFunctionName = "CreateNamespace"
	CreateNamespaces       ServerFunctionName = "CreateNamespaces"
	DeleteNamespaces       ServerFunctionName = "DeleteNamespaces"
	UpdateNamespaces       ServerFunctionName = "UpdateNamespaces"
	UpdateNamespaceToken   ServerFunctionName = "UpdateNamespaceToken"
	DescribeNamespaces     ServerFunctionName = "DescribeNamespaces"
	DescribeNamespaceToken ServerFunctionName = "DescribeNamespaceToken"
)

// 服务/服务别名
const (
	CreateServices        ServerFunctionName = "CreateServices"
	DeleteServices        ServerFunctionName = "DeleteServices"
	UpdateServices        ServerFunctionName = "UpdateServices"
	UpdateServiceToken    ServerFunctionName = "UpdateServiceToken"
	DescribeAllServices   ServerFunctionName = "DescribeAllServices"
	DescribeServices      ServerFunctionName = "DescribeServices"
	DescribeServicesCount ServerFunctionName = "DescribeServicesCount"
	DescribeServiceToken  ServerFunctionName = "DescribeServiceToken"
	DescribeServiceOwner  ServerFunctionName = "DescribeServiceOwner"

	CreateLogicalServices     ServerFunctionName = "CreateLogicalServices"
	UpdateLogicalServices     ServerFunctionName = "UpdateLogicalServices"
	DeleteLogicalServices     ServerFunctionName = "DeleteLogicalServices"
	DescribeLogicalServices   ServerFunctionName = "DescribeLogicalServices"
	BindServiceEnvironments   ServerFunctionName = "BindServiceEnvironments"
	UnbindServiceEnvironments ServerFunctionName = "UnbindServiceEnvironments"

	CreateServiceAlias     ServerFunctionName = "CreateServiceAlias"
	DeleteServiceAliases   ServerFunctionName = "DeleteServiceAliases"
	UpdateServiceAlias     ServerFunctionName = "UpdateServiceAlias"
	DescribeServiceAliases ServerFunctionName = "DescribeServiceAliases"
)

// 服务接口定义
const (
	CreateServiceContracts          ServerFunctionName = "CreateServiceContracts"
	DescribeServiceContracts        ServerFunctionName = "DescribeServiceContracts"
	DescribeServiceContractVersions ServerFunctionName = "DescribeServiceContractVersions"
	DeleteServiceContracts          ServerFunctionName = "DeleteServiceContracts"

	CreateServiceContractInterfaces ServerFunctionName = "CreateServiceContractInterfaces"
	AppendServiceContractInterfaces ServerFunctionName = "AppendServiceContractInterfaces"
	DeleteServiceContractInterfaces ServerFunctionName = "DeleteServiceContractInterfaces"
)

// 服务实例
const (
	CreateInstances               ServerFunctionName = "CreateInstances"
	DeleteInstances               ServerFunctionName = "DeleteInstances"
	DeleteInstancesByHost         ServerFunctionName = "DeleteInstancesByHost"
	UpdateInstances               ServerFunctionName = "UpdateInstances"
	UpdateInstancesIsolate        ServerFunctionName = "UpdateInstancesIsolate"
	DescribeInstances             ServerFunctionName = "DescribeInstances"
	DescribeInstancesCount        ServerFunctionName = "DescribeInstancesCount"
	DescribeInstanceLabels        ServerFunctionName = "DescribeInstanceLabels"
	CleanInstance                 ServerFunctionName = "CleanInstance"
	BatchCleanInstances           ServerFunctionName = "BatchCleanInstances"
	DescribeInstanceLastHeartbeat ServerFunctionName = "DescribeInstanceLastHeartbeat"
)

// 配置使用环境资源授权；运行时读取、管理检查、编辑和预览分别审计。
const (
	DiscoverConfig ServerFunctionName = "DiscoverConfig"
	InspectConfig  ServerFunctionName = "InspectConfig"
	ManageConfig   ServerFunctionName = "ManageConfig"
	PreviewConfig  ServerFunctionName = "PreviewConfig"
)

// 路由
const (
	CreateRouteRules      ServerFunctionName = "CreateRouteRules"
	DeleteRouteRules      ServerFunctionName = "DeleteRouteRules"
	UpdateRouteRules      ServerFunctionName = "UpdateRouteRules"
	EnableRouteRules      ServerFunctionName = "EnableRouteRules"
	DescribeRouteRules    ServerFunctionName = "DescribeRouteRules"
	PublishRouteRules     ServerFunctionName = "PublishRouteRules"
	RollbackRouteRules    ServerFunctionName = "RollbackRouteRules"
	StopbetaRouteRules    ServerFunctionName = "StopbetaRouteRules"
	DescribeRouteReleases ServerFunctionName = "DescribeRouteReleases"
	DeleteRouteReleases   ServerFunctionName = "DeleteRouteReleases"
)

// 限流
const (
	CreateRateLimitRules      ServerFunctionName = "CreateRateLimitRules"
	DeleteRateLimitRules      ServerFunctionName = "DeleteRateLimitRules"
	UpdateRateLimitRules      ServerFunctionName = "UpdateRateLimitRules"
	EnableRateLimitRules      ServerFunctionName = "EnableRateLimitRules"
	DescribeRateLimitRules    ServerFunctionName = "DescribeRateLimitRules"
	PublishRateLimitRules     ServerFunctionName = "PublishRateLimitRules"
	RollbackRateLimitRules    ServerFunctionName = "RollbackRateLimitRules"
	StopbetaRateLimitRules    ServerFunctionName = "StopbetaRateLimitRules"
	DescribeRateLimitReleases ServerFunctionName = "DescribeRateLimitReleases"
	DeleteRateLimitReleases   ServerFunctionName = "DeleteRateLimitReleases"
)

// 熔断
const (
	CreateCircuitBreakerRules      ServerFunctionName = "CreateCircuitBreakerRules"
	DeleteCircuitBreakerRules      ServerFunctionName = "DeleteCircuitBreakerRules"
	UpdateCircuitBreakerRules      ServerFunctionName = "UpdateCircuitBreakerRules"
	DescribeCircuitBreakerRules    ServerFunctionName = "DescribeCircuitBreakerRules"
	PublishCircuitBreakerRules     ServerFunctionName = "PublishCircuitBreakerRules"
	RollbackCircuitBreakerRules    ServerFunctionName = "RollbackCircuitBreakerRules"
	StopbetaCircuitBreakerRules    ServerFunctionName = "StopbetaCircuitBreakerRules"
	DescribeCircuitBreakerReleases ServerFunctionName = "DescribeCircuitBreakerReleases"
	DeleteCircuitBreakerReleases   ServerFunctionName = "DeleteCircuitBreakerReleases"
)

// 主动探测
const (
	CreateFaultDetectRules      ServerFunctionName = "CreateFaultDetectRules"
	DeleteFaultDetectRules      ServerFunctionName = "DeleteFaultDetectRules"
	EnableFaultDetectRules      ServerFunctionName = "EnableFaultDetectRules"
	UpdateFaultDetectRules      ServerFunctionName = "UpdateFaultDetectRules"
	DescribeFaultDetectRules    ServerFunctionName = "DescribeFaultDetectRules"
	PublishFaultDetectRules     ServerFunctionName = "PublishFaultDetectRules"
	RollbackFaultDetectRules    ServerFunctionName = "RollbackFaultDetectRules"
	StopbetaFaultDetectRules    ServerFunctionName = "StopbetaFaultDetectRules"
	DescribeFaultDetectReleases ServerFunctionName = "DescribeFaultDetectReleases"
	DeleteFaultDetectReleases   ServerFunctionName = "DeleteFaultDetectReleases"
)

// 全链路灰度
const (
	CreateLaneGroups   ServerFunctionName = "CreateLaneGroups"
	DeleteLaneGroups   ServerFunctionName = "DeleteLaneGroups"
	EnableLaneGroups   ServerFunctionName = "EnableLaneGroups"
	UpdateLaneGroups   ServerFunctionName = "UpdateLaneGroups"
	DescribeLaneGroups ServerFunctionName = "DescribeLaneGroups"
	CreateLaneRules    ServerFunctionName = "CreateLaneRules"
	UpdateLaneRules    ServerFunctionName = "UpdateLaneRules"
	DeleteLaneRules    ServerFunctionName = "DeleteLaneRules"

	PublishLaneGroups         ServerFunctionName = "PublishLaneGroups"
	RollbackLaneGroups        ServerFunctionName = "RollbackLaneGroups"
	StopbetaLaneGroups        ServerFunctionName = "StopbetaLaneGroups"
	DescribeLaneGroupReleases ServerFunctionName = "DescribeLaneGroupReleases"
	DeleteLaneGroupReleases   ServerFunctionName = "DeleteLaneGroupReleases"
)

const (
	// 无损规则
	CreateLosslessRules   ServerFunctionName = "CreateLosslessRules"
	DeleteLosslessRules   ServerFunctionName = "DeleteLosslessRules"
	UpdateLosslessRules   ServerFunctionName = "UpdateLosslessRules"
	DescribeLosslessRules ServerFunctionName = "DescribeLosslessRules"

	PublishLosslessRules     ServerFunctionName = "PublishLosslessRules"
	RollbackLosslessRules    ServerFunctionName = "RollbackLosslessRules"
	StopbetaLosslessRules    ServerFunctionName = "StopbetaLosslessRules"
	DescribeLosslessReleases ServerFunctionName = "DescribeLosslessReleases"
	DeleteLosslessReleases   ServerFunctionName = "DeleteLosslessReleases"
)

const (
	CreateTrafficSecurityRules      ServerFunctionName = "CreateTrafficSecurityRules"
	DeleteTrafficSecurityRules      ServerFunctionName = "DeleteTrafficSecurityRules"
	UpdateTrafficSecurityRules      ServerFunctionName = "UpdateTrafficSecurityRules"
	DescribeTrafficSecurityRules    ServerFunctionName = "DescribeTrafficSecurityRules"
	PublishTrafficSecurityRules     ServerFunctionName = "PublishTrafficSecurityRules"
	RollbackTrafficSecurityRules    ServerFunctionName = "RollbackTrafficSecurityRules"
	StopbetaTrafficSecurityRules    ServerFunctionName = "StopbetaTrafficSecurityRules"
	DescribeTrafficSecurityReleases ServerFunctionName = "DescribeTrafficSecurityReleases"
	DeleteTrafficSecurityReleases   ServerFunctionName = "DeleteTrafficSecurityReleases"

	CreateTrafficMirrorRules      ServerFunctionName = "CreateTrafficMirrorRules"
	DeleteTrafficMirrorRules      ServerFunctionName = "DeleteTrafficMirrorRules"
	UpdateTrafficMirrorRules      ServerFunctionName = "UpdateTrafficMirrorRules"
	DescribeTrafficMirrorRules    ServerFunctionName = "DescribeTrafficMirrorRules"
	PublishTrafficMirrorRules     ServerFunctionName = "PublishTrafficMirrorRules"
	RollbackTrafficMirrorRules    ServerFunctionName = "RollbackTrafficMirrorRules"
	StopbetaTrafficMirrorRules    ServerFunctionName = "StopbetaTrafficMirrorRules"
	DescribeTrafficMirrorReleases ServerFunctionName = "DescribeTrafficMirrorReleases"
	DeleteTrafficMirrorReleases   ServerFunctionName = "DeleteTrafficMirrorReleases"

	CreateTrafficMockRules      ServerFunctionName = "CreateTrafficMockRules"
	DeleteTrafficMockRules      ServerFunctionName = "DeleteTrafficMockRules"
	UpdateTrafficMockRules      ServerFunctionName = "UpdateTrafficMockRules"
	DescribeTrafficMockRules    ServerFunctionName = "DescribeTrafficMockRules"
	PublishTrafficMockRules     ServerFunctionName = "PublishTrafficMockRules"
	RollbackTrafficMockRules    ServerFunctionName = "RollbackTrafficMockRules"
	StopbetaTrafficMockRules    ServerFunctionName = "StopbetaTrafficMockRules"
	DescribeTrafficMockReleases ServerFunctionName = "DescribeTrafficMockReleases"
	DeleteTrafficMockReleases   ServerFunctionName = "DeleteTrafficMockReleases"
)

// 用户/用户组
const (
	CreateMCPServers       ServerFunctionName = "CreateMCPServers"
	UpdateMCPServers       ServerFunctionName = "UpdateMCPServers"
	DeleteMCPServers       ServerFunctionName = "DeleteMCPServers"
	DescribeMCPServers     ServerFunctionName = "DescribeMCPServers"
	DescribeMCPServerTools ServerFunctionName = "DescribeMCPServerTools"
	CreateA2AAgents        ServerFunctionName = "CreateA2AAgents"
	UpdateA2AAgents        ServerFunctionName = "UpdateA2AAgents"
	DeleteA2AAgents        ServerFunctionName = "DeleteA2AAgents"
	DescribeA2AAgents      ServerFunctionName = "DescribeA2AAgents"
	DescribeA2AAgentSkills ServerFunctionName = "DescribeA2AAgentSkills"
	DescribeA2AAgentCard   ServerFunctionName = "DescribeA2AAgentCard"
)

const (
	// 用户
	CreateUsers        ServerFunctionName = "CreateUsers"
	DeleteUsers        ServerFunctionName = "DeleteUsers"
	DescribeUsers      ServerFunctionName = "DescribeUsers"
	DescribeUserToken  ServerFunctionName = "DescribeUserToken"
	EnableUserToken    ServerFunctionName = "EnableUserToken"
	ResetUserToken     ServerFunctionName = "ResetUserToken"
	UpdateUser         ServerFunctionName = "UpdateUser"
	UpdateUserPassword ServerFunctionName = "UpdateUserPassword"

	// 用户组
	CreateUserGroup         ServerFunctionName = "CreateUserGroup"
	UpdateUserGroups        ServerFunctionName = "UpdateUserGroups"
	DeleteUserGroups        ServerFunctionName = "DeleteUserGroups"
	DescribeUserGroups      ServerFunctionName = "DescribeUserGroups"
	DescribeUserGroupDetail ServerFunctionName = "DescribeUserGroupDetail"
	DescribeUserGroupToken  ServerFunctionName = "DescribeUserGroupToken"
	EnableUserGroupToken    ServerFunctionName = "EnableUserGroupToken"
	ResetUserGroupToken     ServerFunctionName = "ResetUserGroupToken"
)

// 策略/角色
const (
	// 策略
	CreateAuthPolicy           ServerFunctionName = "CreateAuthPolicy"
	UpdateAuthPolicies         ServerFunctionName = "UpdateAuthPolicies"
	DeleteAuthPolicies         ServerFunctionName = "DeleteAuthPolicies"
	DescribeAuthPolicies       ServerFunctionName = "DescribeAuthPolicies"
	DescribeAuthPolicyDetail   ServerFunctionName = "DescribeAuthPolicyDetail"
	DescribePrincipalResources ServerFunctionName = "DescribePrincipalResources"
	AuthorizeResources         ServerFunctionName = "AuthorizeResources"

	// 角色
	CreateAuthRoles        ServerFunctionName = "CreateAuthRoles"
	UpdateAuthRoles        ServerFunctionName = "UpdateAuthRoles"
	DeleteAuthRoles        ServerFunctionName = "DeleteAuthRoles"
	DescribeAuthRoles      ServerFunctionName = "DescribeAuthRoles"
	DescribeAuthRoleDetail ServerFunctionName = "DescribeAuthRoleDetail"
)

// 运维接口
const (
	DescribeServerConnections ServerFunctionName = "DescribeServerConnections"
	DescribeServerConnStats   ServerFunctionName = "DescribeServerConnStats"
	CloseConnections          ServerFunctionName = "CloseConnections"
	FreeOSMemory              ServerFunctionName = "FreeOSMemory"
	DescribeLeaderElections   ServerFunctionName = "DescribeLeaderElections"
	ReleaseLeaderElection     ServerFunctionName = "ReleaseLeaderElection"
	DescribeGetLogOutputLevel ServerFunctionName = "DescribeGetLogOutputLevel"
	UpdateLogOutputLevel      ServerFunctionName = "UpdateLogOutputLevel"
	DescribeCMDBInfo          ServerFunctionName = "DescribeCMDBInfo"
)

type ServerFunctionGroup struct {
	Name      string               `json:"name"`
	Functions []ServerFunctionName `json:"functions"`
}

var ServerFunctions = []ServerFunctionGroup{
	{
		Name: "Client",
		Functions: []ServerFunctionName{
			RegisterInstance,
			DeregisterInstance,
			ReportServiceContract,
			DiscoverServices,
			DiscoverInstances,
			UpdateInstance,
			DiscoverRouterRule,
			DiscoverRateLimitRule,
			DiscoverCircuitBreakerRule,
			DiscoverFaultDetectRule,
			DiscoverServiceContract,
			DiscoverLaneRule,
			DiscoverConfig,
		},
	},
	{
		Name: "Namespace",
		Functions: []ServerFunctionName{
			CreateNamespace,
			CreateNamespaces,
			DeleteNamespaces,
			UpdateNamespaces,
			// UpdateNamespaceToken,
			DescribeNamespaces,
			// DescribeNamespaceToken,
		},
	},
	{
		Name: "Service",
		Functions: []ServerFunctionName{
			CreateServices,
			DeleteServices,
			UpdateServices,
			// UpdateServiceToken,
			DescribeAllServices,
			DescribeServices,
			DescribeServicesCount,
			CreateLogicalServices,
			UpdateLogicalServices,
			DeleteLogicalServices,
			DescribeLogicalServices,
			BindServiceEnvironments,
			UnbindServiceEnvironments,
			// DescribeServiceToken,
			// DescribeServiceOwner,
			CreateServiceAlias,
			DeleteServiceAliases,
			UpdateServiceAlias,
			DescribeServiceAliases,
		},
	},
	{
		Name: "ServiceContract",
		Functions: []ServerFunctionName{
			CreateServiceContracts,
			DescribeServiceContracts,
			DescribeServiceContractVersions,
			DeleteServiceContracts,
			CreateServiceContractInterfaces,
			AppendServiceContractInterfaces,
			DeleteServiceContractInterfaces,
		},
	},
	{
		Name: "Instance",
		Functions: []ServerFunctionName{
			CreateInstances,
			DeleteInstances,
			DeleteInstancesByHost,
			UpdateInstances,
			UpdateInstancesIsolate,
			DescribeInstances,
			DescribeInstancesCount,
			DescribeInstanceLabels,
			CleanInstance,
			BatchCleanInstances,
			DescribeInstanceLastHeartbeat,
		},
	},
	{
		Name: "RouteRule",
		Functions: []ServerFunctionName{
			CreateRouteRules,
			DeleteRouteRules,
			UpdateRouteRules,
			EnableRouteRules,
			DescribeRouteRules,
		},
	},
	{
		Name: "RateLimitRule",
		Functions: []ServerFunctionName{
			CreateRateLimitRules,
			DeleteRateLimitRules,
			UpdateRateLimitRules,
			EnableRateLimitRules,
			DescribeRateLimitRules,
		},
	},
	{
		Name: "CircuitBreakerRule",
		Functions: []ServerFunctionName{
			CreateCircuitBreakerRules,
			DeleteCircuitBreakerRules,
			UpdateCircuitBreakerRules,
			DescribeCircuitBreakerRules,
			PublishCircuitBreakerRules,
			RollbackCircuitBreakerRules,
			StopbetaCircuitBreakerRules,
			DescribeCircuitBreakerReleases,
			DeleteCircuitBreakerReleases,
		},
	},
	{
		Name: "FaultDetectRule",
		Functions: []ServerFunctionName{
			CreateFaultDetectRules,
			DeleteFaultDetectRules,
			EnableFaultDetectRules,
			UpdateFaultDetectRules,
			DescribeFaultDetectRules,
		},
	},
	{
		Name: "LaneRule",
		Functions: []ServerFunctionName{
			CreateLaneGroups,
			DeleteLaneGroups,
			EnableLaneGroups,
			UpdateLaneGroups,
			DescribeLaneGroups,
		},
	},
	{
		Name:      "Config",
		Functions: []ServerFunctionName{DiscoverConfig, InspectConfig, ManageConfig, PreviewConfig},
	},
	{
		Name: "AINative",
		Functions: []ServerFunctionName{
			CreateMCPServers,
			UpdateMCPServers,
			DeleteMCPServers,
			DescribeMCPServers,
			DescribeMCPServerTools,
			CreateA2AAgents,
			UpdateA2AAgents,
			DeleteA2AAgents,
			DescribeA2AAgents,
			DescribeA2AAgentSkills,
			DescribeA2AAgentCard,
		},
	},
	{
		Name: "User",
		Functions: []ServerFunctionName{
			CreateUsers,
			DeleteUsers,
			DescribeUsers,
			DescribeUserToken,
			EnableUserToken,
			ResetUserToken,
			UpdateUser,
			UpdateUserPassword,
		},
	},
	{
		Name: "UserGroup",
		Functions: []ServerFunctionName{
			CreateUserGroup,
			UpdateUserGroups,
			DeleteUserGroups,
			DescribeUserGroups,
			DescribeUserGroupDetail,
			DescribeUserGroupToken,
			EnableUserGroupToken,
			ResetUserGroupToken,
		},
	},
	{
		Name: "AuthPolicy",
		Functions: []ServerFunctionName{
			CreateAuthPolicy,
			UpdateAuthPolicies,
			DeleteAuthPolicies,
			DescribeAuthPolicies,
			DescribeAuthPolicyDetail,
			DescribePrincipalResources,
		},
	},
	// "AuthRole": {
	// 	CreateAuthRoles,
	// 	UpdateAuthRoles,
	// 	DeleteAuthRoles,
	// 	DescribeAuthRoles,
	// 	DescribeAuthRoleDetail,
	// },
}

var (
	SearchTypeMapping = map[string]apisecurity.ResourceType{
		"0":  apisecurity.ResourceType_Namespaces,
		"1":  apisecurity.ResourceType_Services,
		"30": apisecurity.ResourceType_MCPServerResources,
		"31": apisecurity.ResourceType_A2AAgentResources,
	}
)
