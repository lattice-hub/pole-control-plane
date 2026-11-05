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

package types

import (
	"fmt"
	"time"

	commontime "github.com/pole-io/pole-server/pkg/common/utils/time"
)

// OperationType Operating type
type OperationType string

// Define the type of operation containing
const (
	// OCreate create
	OCreate OperationType = "Create"
	// ODelete delete
	ODelete OperationType = "Delete"
	// OUpdate update
	OUpdate OperationType = "Update"
	// OUpdateIsolate Update isolation state
	OUpdateIsolate OperationType = "UpdateIsolate"
	// OUpdateToken Update token
	OUpdateToken OperationType = "UpdateToken"
	// OUpdateGroup Update user-user group association relationship
	OUpdateGroup OperationType = "UpdateGroup"
	// OEnableRateLimit Update enable state
	OUpdateEnable OperationType = "UpdateEnable"
	// ORollback Rollback resource
	ORollback OperationType = "Rollback"
)

// Resource Operating resources
type Resource string

// Define the type of resource type
const (
	RNamespace           Resource = "Namespace"
	RService             Resource = "Service"
	RRouting             Resource = "Routing"
	RCircuitBreaker      Resource = "CircuitBreaker"
	RInstance            Resource = "Instance"
	RRateLimit           Resource = "RateLimit"
	RUser                Resource = "User"
	RUserGroup           Resource = "UserGroup"
	RUserGroupRelation   Resource = "UserGroupRelation"
	RAuthStrategy        Resource = "AuthStrategy"
	RAuthRole            Resource = "Role"
	RCircuitBreakerRule  Resource = "CircuitBreakerRule"
	RFaultDetectRule     Resource = "FaultDetectRule"
	RServiceContract     Resource = "ServiceContract"
	RLaneGroup           Resource = "LaneGroup"
	RLaneRule            Resource = "LaneRule"
	RLosslessRule        Resource = "LosslessRule"
	RTrafficSecurityRule Resource = "TrafficSecurityRule"
	RTrafficMirrorRule   Resource = "TrafficMirrorRule"
	RTrafficMockRule     Resource = "TrafficMockRule"
	RMCPServer           Resource = "MCPServer"
	RMCPServerTool       Resource = "MCPServerTool"
	RA2AAgent            Resource = "A2AAgent"
	RA2AAgentSkill       Resource = "A2AAgentSkill"
)

// RecordEntry Operation records
type RecordEntry struct {
	ResourceType  Resource      `json:"resource_type"`
	ResourceName  string        `json:"resource_name"`
	Namespace     string        `json:"namespace"`
	Operator      string        `json:"operator"`
	OperationType OperationType `json:"operation_type"`
	Detail        string        `json:"detail"`
	Server        string        `json:"server"` // Server name, used to distinguish the server that generated the record
	HappenTime    time.Time     `json:"happen_time"`
}

func (r *RecordEntry) String() string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		commontime.Time2String(r.HappenTime),
		r.ResourceType,
		r.ResourceName,
		r.Namespace,
		r.OperationType,
		r.Operator,
		SanitizeOperationDetail(r.ResourceType, r.Detail),
		r.Server,
	)
}
