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

package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	cachetypes "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/common/syncs/container"
)

var (
	cacheSet = map[string]int{}
)

const (
	// UpdateCacheInterval 缓存更新时间间隔
	UpdateCacheInterval = 1 * time.Second
)

var (
	// DefaultTimeDiff default time diff
	DefaultTimeDiff = -5 * time.Second
	ReportInterval  = 1 * time.Second
)

// CacheManager 名字服务缓存
type CacheManager struct {
	storage  store.Store
	caches   []cachetypes.Cache
	needLoad *container.SyncSet[string]
}

// Initialize 缓存对象初始化
func (nc *CacheManager) Initialize() error {
	if config != nil && config.DiffTime != 0 {
		DefaultTimeDiff = -1 * (config.DiffTime.Abs())
	}
	if DefaultTimeDiff > 0 {
		return fmt.Errorf("cache diff time to pull store must negative number: %+v", DefaultTimeDiff)
	}
	return nil
}

// OpenResourceCache 开启资源缓存
func (nc *CacheManager) OpenResourceCache(entries ...cachetypes.ConfigEntry) error {
	for _, obj := range nc.caches {
		var entryItem *cachetypes.ConfigEntry
		for _, entry := range entries {
			if obj.Name() == entry.Name {
				entryItem = &entry
				break
			}
		}
		if entryItem == nil {
			continue
		}
		if err := obj.Initialize(entryItem.Option); err != nil {
			return err
		}
		nc.needLoad.Add(entryItem.Name)
	}
	return nil
}

// warmUp 缓存更新
func (nc *CacheManager) warmUp() error {
	var wg sync.WaitGroup
	entries := nc.needLoad.ToSlice()
	for i := range entries {
		name := entries[i]
		index, exist := cacheSet[name]
		if !exist {
			return fmt.Errorf("cache resource %s not exists", name)
		}
		wg.Add(1)
		go func(c cachetypes.Cache) {
			defer wg.Done()
			_ = c.Update()
		}(nc.caches[index])
	}

	wg.Wait()
	return nil
}

// clear 清除caches的所有缓存数据
func (nc *CacheManager) clear() error {
	for _, obj := range nc.caches {
		if err := obj.Clear(); err != nil {
			return err
		}
	}

	return nil
}

// Close 关闭所有的 Cache 缓存
func (nc *CacheManager) Close() error {
	for _, obj := range nc.caches {
		if err := obj.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Start 缓存对象启动协程，定时更新缓存
func (nc *CacheManager) Start(ctx context.Context) error {
	log.Infof("[Cache] cache goroutine start")

	// 启动的时候，先更新一版缓存
	log.Infof("[Cache] cache update now first time")
	if err := nc.warmUp(); err != nil {
		return err
	}
	log.Infof("[Cache] cache update done")

	// 启动协程，开始定时更新缓存数据
	entries := nc.needLoad.ToSlice()
	for i := range entries {
		name := entries[i]
		index, exist := cacheSet[name]
		if !exist {
			return fmt.Errorf("cache resource %s not exists", name)
		}
		// 每个缓存各自在自己的协程内部按照期望的缓存更新时间完成数据缓存刷新
		go func(c cachetypes.Cache) {
			ticker := time.NewTicker(nc.GetUpdateCacheInterval())
			for {
				select {
				case <-ticker.C:
					_ = c.Update()
				case <-ctx.Done():
					ticker.Stop()
					return
				}
			}
		}(nc.caches[index])
	}

	return nil
}

// Clear 主动清除缓存数据
func (nc *CacheManager) Clear() error {
	return nc.clear()
}

// GetUpdateCacheInterval 获取当前cache的更新间隔
func (nc *CacheManager) GetUpdateCacheInterval() time.Duration {
	return UpdateCacheInterval
}

// GetReportInterval 获取当前cache的更新间隔
func (nc *CacheManager) GetReportInterval() time.Duration {
	return ReportInterval
}

// GetTimeDiff 获取拉取 store 的时间偏移（负值），供 BaseCache.LastFetchTime 使用
func (nc *CacheManager) GetTimeDiff() time.Duration {
	return DefaultTimeDiff
}

// Service 获取Service缓存信息
func (nc *CacheManager) Service() cachetypes.ServiceCache {
	return nc.caches[cachetypes.CacheService].(cachetypes.ServiceCache)
}

// Instance 获取Instance缓存信息
func (nc *CacheManager) Instance() cachetypes.InstanceCache {
	return nc.caches[cachetypes.CacheInstance].(cachetypes.InstanceCache)
}

// RoutingConfig 获取路由配置的缓存信息
func (nc *CacheManager) RoutingConfig() cachetypes.RouterRuleCache {
	return nc.caches[cachetypes.CacheRoutingConfig].(cachetypes.RouterRuleCache)
}

// RateLimit 获取限流规则缓存信息
func (nc *CacheManager) RateLimit() cachetypes.RateLimitCache {
	return nc.caches[cachetypes.CacheRateLimit].(cachetypes.RateLimitCache)
}

// CircuitBreaker 获取熔断规则缓存信息
func (nc *CacheManager) CircuitBreaker() cachetypes.CircuitBreakerCache {
	return nc.caches[cachetypes.CacheCircuitBreaker].(cachetypes.CircuitBreakerCache)
}

// FaultDetector 获取探测规则缓存信息
func (nc *CacheManager) FaultDetector() cachetypes.FaultDetectCache {
	return nc.caches[cachetypes.CacheFaultDetector].(cachetypes.FaultDetectCache)
}

// ServiceContract 获取服务契约缓存
func (nc *CacheManager) ServiceContract() cachetypes.ServiceContractCache {
	return nc.caches[cachetypes.CacheServiceContract].(cachetypes.ServiceContractCache)
}

// LaneRule 获取泳道规则缓存信息
func (nc *CacheManager) LaneRule() cachetypes.LaneCache {
	return nc.caches[cachetypes.CacheLaneRule].(cachetypes.LaneCache)
}

func (nc *CacheManager) Lossless() cachetypes.LosslessCache {
	return nc.caches[cachetypes.CacheLossLess].(cachetypes.LosslessCache)
}

func (nc *CacheManager) TrafficSecurity() cachetypes.TrafficGovernanceCache {
	return nc.caches[cachetypes.CacheTrafficSecurity].(cachetypes.TrafficGovernanceCache)
}

func (nc *CacheManager) TrafficMirror() cachetypes.TrafficGovernanceCache {
	return nc.caches[cachetypes.CacheTrafficMirror].(cachetypes.TrafficGovernanceCache)
}

func (nc *CacheManager) TrafficMock() cachetypes.TrafficGovernanceCache {
	return nc.caches[cachetypes.CacheTrafficMock].(cachetypes.TrafficGovernanceCache)
}

// User Get user information cache information
func (nc *CacheManager) User() cachetypes.UserCache {
	return nc.caches[cachetypes.CacheUser].(cachetypes.UserCache)
}

// AuthStrategy Get authentication cache information
func (nc *CacheManager) AuthStrategy() cachetypes.StrategyCache {
	return nc.caches[cachetypes.CacheAuthStrategy].(cachetypes.StrategyCache)
}

// Namespace Get namespace cache information
func (nc *CacheManager) Namespace() cachetypes.NamespaceCache {
	return nc.caches[cachetypes.CacheNamespace].(cachetypes.NamespaceCache)
}

// Client Get client cache information
func (nc *CacheManager) Client() cachetypes.ClientCache {
	return nc.caches[cachetypes.CacheClient].(cachetypes.ClientCache)
}

// Gray get Gray cache information
func (nc *CacheManager) Gray() cachetypes.GrayCache {
	return nc.caches[cachetypes.CacheGray].(cachetypes.GrayCache)
}

// Role get Role cache information
func (nc *CacheManager) Role() cachetypes.RoleCache {
	return nc.caches[cachetypes.CacheRole].(cachetypes.RoleCache)
}

// MCPServer get mcp server cache information
func (nc *CacheManager) MCPServer() cachetypes.MCPServerCache {
	return nc.caches[cachetypes.CacheMCPServer].(cachetypes.MCPServerCache)
}

func (nc *CacheManager) A2AAgent() cachetypes.A2AAgentCache {
	return nc.caches[cachetypes.CacheA2AAgent].(cachetypes.A2AAgentCache)
}

// GetCacher get cachetypes.Cache impl
func (nc *CacheManager) GetCacher(cacheIndex cachetypes.CacheIndex) cachetypes.Cache {
	return nc.caches[cacheIndex]
}

func (nc *CacheManager) RegisterCacher(cacheType cachetypes.CacheIndex, item cachetypes.Cache) {
	nc.caches[cacheType] = item
}

// GetStore get store
func (nc *CacheManager) GetStore() store.Store {
	return nc.storage
}

// RegisterCache 注册缓存资源
func RegisterCache(name string, index cachetypes.CacheIndex) {
	if _, exist := cacheSet[name]; exist {
		log.Warnf("existed cache resource: name = %s", name)
	}

	cacheSet[name] = int(index)
}
