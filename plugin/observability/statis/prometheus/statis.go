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

package prometheus

import (
	otelmetric "go.opentelemetry.io/otel/metric"

	"github.com/pole-io/pole-server/apis"
	metricstypes "github.com/pole-io/pole-server/apis/pkg/types/metrics"
)

const (
	PluginName = "prometheus"
)

// PrometheusStatis is a struct for prometheus statistics
type StatisWorker struct {
	discoveryHandler *discoveryMetricHandle
	metricVecCaches  map[string]otelmetric.Int64Gauge
}

// Name 获取统计插件名称
func (s *StatisWorker) Name() string {
	return PluginName
}

// Initialize 初始化统计插件
func (s *StatisWorker) Initialize(conf *apis.ConfigEntry) error {
	s.metricVecCaches = make(map[string]otelmetric.Int64Gauge)
	s.discoveryHandler = newDiscoveryMetricHandle()
	// 设置统计打印周期
	interval, _ := conf.Option["interval"].(int)
	if interval == 0 {
		interval = 60
	}
	return nil
}

func (s *StatisWorker) Type() apis.PluginType {
	return apis.PluginTypeStatis
}

// Destroy 销毁统计插件
func (s *StatisWorker) Destroy() error {
	return nil
}

// ReportCallMetrics report call metrics info
func (s *StatisWorker) ReportCallMetrics(metric metricstypes.CallMetric) {
	// 只上报服务端接受客户端请求调用的结果
	if metric.Type != metricstypes.ServerCallMetric {
		return
	}
}

// ReportDiscoveryMetrics report discovery metrics
func (s *StatisWorker) ReportDiscoveryMetrics(metric ...metricstypes.DiscoveryMetric) {
	s.discoveryHandler.handle(metric)
}

// ReportDiscoverCall report discover service times
func (s *StatisWorker) ReportDiscoverCall(metric metricstypes.ClientDiscoverMetric) {
	// ignore not support this
}
