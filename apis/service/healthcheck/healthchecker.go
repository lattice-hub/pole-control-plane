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

package healthcheck

import (
	"context"
	"fmt"

	"github.com/pole-io/pole-server/apis"
	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/pluginapi"
)

type CheckerPeer struct {
	Host string
	ID   string
	Port uint32
}

// CheckerPeerUpdater is implemented by distributed heartbeat checkers that
// need the current pole.checker membership to route heartbeat records.
type CheckerPeerUpdater interface {
	SetCheckerPeers([]CheckerPeer) error
}

// ReportRequest report heartbeat request
type ReportRequest struct {
	QueryRequest
	LocalHost  string
	CurTimeSec int64
	Count      int64
}

// CheckRequest check heartbeat request
type CheckRequest struct {
	QueryRequest
	ExpireDurationSec uint32
	CurTimeSec        func() int64
	Path              string
}

// CheckResponse check heartbeat response
type CheckResponse struct {
	Healthy              bool
	LastHeartbeatTimeSec int64
	StayUnchanged        bool
	Regular              bool
}

// QueryRequest query heartbeat request
type QueryRequest struct {
	InstanceId string
	Host       string
	Port       uint32
	Healthy    bool
}

// BatchQueryRequest batch query heartbeat request
type BatchQueryRequest struct {
	Requests []*QueryRequest
}

// QueryResponse query heartbeat response
type QueryResponse struct {
	Server           string
	Exists           bool
	LastHeartbeatSec int64
	Count            int64
}

// BatchQueryResponse batch query heartbeat response
type BatchQueryResponse struct {
	Responses []*QueryResponse
}

// AddCheckRequest add check request
type AddCheckRequest struct {
	Instances []string
	LocalHost string
}

// HealthCheckType health check type
type HealthCheckType int32

const (
	HealthCheckerHeartbeat   HealthCheckType = 1
	HealthCheckerDetectTCP   HealthCheckType = 2
	HealthCheckerDetectHTTP  HealthCheckType = 3
	HealthCheckerDetectUDP   HealthCheckType = 4
	HealthCheckerDetectGRPC  HealthCheckType = 5
	HealthCheckerDetectMYSQL HealthCheckType = 6
)

// HealthChecker health checker plugin interface
type HealthChecker interface {
	apis.Plugin
	// Type for health check plugin, only one same type plugin is allowed
	CheckType() HealthCheckType
	// Report process heartbeat info report
	Report(ctx context.Context, request *ReportRequest) error
	// Check process the instance check
	Check(request *CheckRequest) (*CheckResponse, error)
	// Query queries the heartbeat time
	Query(ctx context.Context, request *QueryRequest) (*QueryResponse, error)
	// BatchQuery batch queries the heartbeat time
	BatchQuery(ctx context.Context, request *BatchQueryRequest) (*BatchQueryResponse, error)
	// Suspend health checker for entire expired duration manually
	Suspend()
	// SuspendTimeSec get the suspend time in seconds
	SuspendTimeSec() int64
	// Delete delete the id
	Delete(ctx context.Context, id string) error
	// DebugHandlers return debug handlers
	DebugHandlers() []types.DebugHandler
}

// GetHealthChecker get the health checker by name
func GetHealthChecker(name string, cfg *apis.ConfigEntry) HealthChecker {
	if !pluginapi.ActiveRegistry().Contains(pluginapi.KindHealthCheck, name) {
		return nil
	}
	item, err := apis.ResolvePlugin(apis.PluginTypeHealthCheck, name)
	if err != nil {
		panic(fmt.Errorf("resolve HealthChecker plugin %q: %w", name, err))
	}

	if err := item.Initialize(cfg); err != nil {
		panic(fmt.Errorf("HealthChecker plugin init err: %s", err.Error()))
	}

	healthChecker, ok := item.(HealthChecker)
	if !ok {
		panic(fmt.Errorf("plugin target: %s not HealthChecker", name))
	}
	return healthChecker
}
