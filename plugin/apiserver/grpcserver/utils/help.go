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

package utils

import (
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/pole-io/pole-server/apis/apiserver"
	"github.com/pole-io/pole-server/pkg/common/log"
)

// GetConfigClientOpenMethod .
func GetConfigClientOpenMethod(protocol string) (map[string]bool, error) {
	openMethods := []string{"Discover"}

	openMethod := make(map[string]bool)

	for _, item := range openMethods {
		openMethod[newMethod("ConfigGRPC", item)] = true
	}

	return openMethod, nil
}

// GetDiscoverClientOpenMethod 获取客户端openMethod
func GetDiscoverClientOpenMethod(include []string, protocol string) (map[string]bool, error) {
	clientAccess := make(map[string][]string)
	clientAccess[apiserver.DiscoverAccess] = []string{"Discover", "ReportClient", "ReportServiceContract", "GetServiceContract", "Issue", "Renew"}
	clientAccess[apiserver.RegisterAccess] = []string{"RegisterInstance", "DeregisterInstance"}
	clientAccess[apiserver.HealthcheckAccess] = []string{"Heartbeat", "BatchHeartbeat", "BatchGetHeartbeat", "BatchDelHeartbeat"}
	clientAccess[apiserver.ConfigAccess] = []string{"Discover"}

	openMethod := make(map[string]bool)
	// 如果为空，开启全部接口
	if len(include) == 0 {
		for key := range clientAccess {
			include = append(include, key)
		}
	}

	for _, item := range include {
		if methods, ok := clientAccess[item]; ok {
			for _, method := range methods {
				recordMethod := legacyMethod("Polaris", protocol, method)
				if item == apiserver.ConfigAccess {
					recordMethod = newMethod("ConfigGRPC", method)
				}
				if item == apiserver.HealthcheckAccess && method != "Heartbeat" {
					recordMethod = legacyMethod("PolarisHeartbeat", protocol, method)
				}
				if method == "ReportServiceContract" || method == "GetServiceContract" {
					recordMethod = legacyMethod("PolarisServiceContract", protocol, method)
				}
				openMethod[recordMethod] = true
				addSpecOpenMethod(openMethod, item, method)
			}
		} else {
			log.Errorf("method %s does not exist in %sserver client access", item, protocol)
			return nil, fmt.Errorf("method %s does not exist in %sserver client access", item, protocol)
		}
	}
	log.Info("[APIServer] client open method info", zap.Any("openMethod", openMethod))
	return openMethod, nil
}

func legacyMethod(service string, protocol string, method string) string {
	return "/v1." + service + strings.ToUpper(protocol) + "/" + method
}

func newMethod(service string, method string) string {
	return "/v1." + service + "/" + method
}

func addSpecOpenMethod(openMethod map[string]bool, access string, method string) {
	switch access {
	case apiserver.DiscoverAccess, apiserver.RegisterAccess, apiserver.HealthcheckAccess:
		if method == "Issue" || method == "Renew" {
			openMethod[newMethod("WorkloadCredentialService", method)] = true
			return
		}
		if access != apiserver.HealthcheckAccess || method == "Heartbeat" {
			openMethod[newMethod("DiscoverGRPC", method)] = true
		}
		if method == "BatchGetHeartbeat" || method == "BatchDelHeartbeat" {
			openMethod[newMethod("PoleHeartbeatGRPC", method)] = true
		}
	case apiserver.ConfigAccess:
		openMethod[newMethod("ConfigGRPC", method)] = true
	}
}
