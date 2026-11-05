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

package rules

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	cacheapi "github.com/pole-io/pole-server/apis/cache"
	cachetypes "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/pkg/types/rules"
	svctypes "github.com/pole-io/pole-server/apis/pkg/types/service"
	"github.com/pole-io/pole-server/apis/store"
	cachebase "github.com/pole-io/pole-server/pkg/cache/base"
	"github.com/pole-io/pole-server/pkg/common/syncs/container"
	matchs "github.com/pole-io/pole-server/pkg/common/utils/match"
	"go.uber.org/zap"
)

// rateLimitCache的实现
type rateLimitCache struct {
	*cachebase.BaseCache

	lock         sync.RWMutex
	waitFixRules map[string]struct{}
	svcCache     cachetypes.ServiceCache
	storage      store.Store

	// 用于控制台查询列表
	ids *container.SyncMap[string, *rules.RateLimit]
	// --------- 以下缓存均用于客户端数据查询 --------- //
	// increment cache
	rules *container.SyncMap[string, *rules.RateLimitRelease]
	// fetched service cache
	// key1: namespace, key2: service
	svcSpecificRules *container.SyncMap[string, *container.SyncMap[string, *rules.ServiceWithRateLimits]]
	// key1: namespace
	nsWildcardRules *container.SyncMap[string, *rules.ServiceWithRateLimits]
	// all rules are wildcard specific
	allWildcardRules *rules.ServiceWithRateLimits
}

// NewRateLimitCache 返回一个操作RateLimitCache的对象
func NewRateLimitCache(s store.Store, cacheMgr cachetypes.CacheManager) cachetypes.RateLimitCache {
	return &rateLimitCache{
		BaseCache:    cachebase.NewBaseCache(s, cacheMgr),
		storage:      s,
		waitFixRules: map[string]struct{}{},
	}
}

// Initialize 实现Cache接口的initialize函数
func (rlc *rateLimitCache) Initialize(_ map[string]interface{}) error {
	rlc.ids = container.NewSyncMap[string, *rules.RateLimit]()
	rlc.rules = container.NewSyncMap[string, *rules.RateLimitRelease]()
	rlc.svcSpecificRules = container.NewSyncMap[string, *container.SyncMap[string, *rules.ServiceWithRateLimits]]()
	rlc.nsWildcardRules = container.NewSyncMap[string, *rules.ServiceWithRateLimits]()
	rlc.allWildcardRules = rules.NewServiceWithRateLimits(svctypes.ServiceKey{
		Namespace: cachetypes.AllMatched,
		Name:      cachetypes.AllMatched,
	})
	rlc.svcCache = rlc.CacheMgr.GetCacher(cachetypes.CacheService).(cachetypes.ServiceCache)
	registerGovernanceRuleWatcher(rlc.storage, rlc.CacheMgr, rlc)
	return nil
}

// Update 实现Cache接口的update函数
func (rlc *rateLimitCache) Update() error {
	if ok, err := updateGovernanceRuleCache(rlc.storage); ok {
		return err
	}
	// 多个线程竞争，只有一个线程进行更新
	_, err, _ := rlc.GetSingle().Do(rlc.Name(), func() (interface{}, error) {
		return nil, rlc.DoCacheUpdate(rlc.Name(), rlc.realUpdate)
	})
	return err
}

func (rlc *rateLimitCache) realUpdate() (map[string]time.Time, int64, error) {
	allLastMtimes := map[string]time.Time{}

	rateLimits, err := rlc.storage.GetMoreRateLimits(rlc.LastFetchTime(), rlc.IsFirstUpdate())
	if err != nil {
		log.Errorf("[cache][rate_limit] console cache update err: %s", err.Error())
		return nil, -1, err
	}
	lastTime, upsert, del := rlc.setRateLimitConsole(rateLimits)
	log.Info("[cache][rate_limit] console cache update",
		zap.Int("pull-from-store", len(rateLimits)), zap.Int("upsert", upsert), zap.Int("delete", del),
		zap.Time("last", lastTime))
	allLastMtimes[rlc.Name()+"_console"] = lastTime

	releases, err := rlc.storage.GetMoreRateLimitReleases(rlc.LastFetchTime(), rlc.IsFirstUpdate())
	if err != nil {
		log.Errorf("[cache][rate_limit] client cache update err: %s", err.Error())
		return nil, -1, err
	}
	lastTime, upsert, del = rlc.setRateLimitClient(releases)
	log.Info("[cache][rate_limit] client cache update",
		zap.Int("pull-from-store", len(releases)), zap.Int("upsert", upsert), zap.Int("delete", del),
		zap.Time("last", lastTime))
	allLastMtimes[rlc.Name()] = lastTime

	return allLastMtimes, int64(len(rateLimits) + len(releases)), nil
}

// Name 获取资源名称
func (rlc *rateLimitCache) Name() string {
	return cachetypes.RateLimitConfigName
}

// Clear 实现Cache接口的clear函数
func (rlc *rateLimitCache) Clear() error {
	resetGovernanceRuleUpdateCache(rlc.storage)
	rlc.BaseCache.Clear()
	rlc.ids = container.NewSyncMap[string, *rules.RateLimit]()
	rlc.rules = container.NewSyncMap[string, *rules.RateLimitRelease]()
	rlc.svcSpecificRules = container.NewSyncMap[string, *container.SyncMap[string, *rules.ServiceWithRateLimits]]()
	rlc.nsWildcardRules = container.NewSyncMap[string, *rules.ServiceWithRateLimits]()
	rlc.allWildcardRules = rules.NewServiceWithRateLimits(svctypes.ServiceKey{
		Namespace: cachetypes.AllMatched,
		Name:      cachetypes.AllMatched,
	})
	return nil
}

func (rlc *rateLimitCache) toProto(item *rules.RateLimit) {
	item.ToSpec()
}

// setRateLimitConsole 更新限流规则到缓存中
func (rlc *rateLimitCache) setRateLimitConsole(rateLimits []*rules.RateLimit) (time.Time, int, int) {
	if len(rateLimits) == 0 {
		return time.Time{}, 0, 0
	}

	upsert := 0
	del := 0

	lastMtime := rlc.LastMtime(rlc.Name() + "_console").Unix()
	for _, item := range rateLimits {
		rlc.toProto(item)
		if item.ModifyTime.Unix() > lastMtime {
			lastMtime = item.ModifyTime.Unix()
		}

		// 待删除的rateLimit
		if !item.Valid {
			del++
			rlc.ids.Delete(item.ID)
			continue
		}
		upsert++
		rlc.ids.Store(item.ID, item)
	}

	return time.Unix(lastMtime, 0), upsert, del
}

// setRateLimitClient 更新限流规则到缓存中
func (rlc *rateLimitCache) setRateLimitClient(rateLimits []*rules.RateLimitRelease) (time.Time, int, int) {
	if len(rateLimits) == 0 {
		return time.Time{}, 0, 0
	}

	upsert := 0
	del := 0

	reloads := map[string]*rules.ServiceWithRateLimits{}

	lastMtime := rlc.LastMtime(rlc.Name()).Unix()
	for _, item := range rateLimits {
		rlc.toProto(item.Rule)
		if item.Mtime.Unix() > lastMtime {
			lastMtime = item.Mtime.Unix()
		}

		serviceKey, ok := rlc.resolveRuleServiceKey(item.Rule)
		if !ok {
			continue
		}
		rlc.svcSpecificRules.ComputeIfAbsent(serviceKey.Namespace, func(k string) *container.SyncMap[string, *rules.ServiceWithRateLimits] {
			return container.NewSyncMap[string, *rules.ServiceWithRateLimits]()
		})
		svcLimits, _ := rlc.svcSpecificRules.MustLoad(serviceKey.Namespace).ComputeIfAbsent(serviceKey.Name, func(k string) *rules.ServiceWithRateLimits {
			return rules.NewServiceWithRateLimits(serviceKey)
		})
		reloads[serviceKey.Namespace+"-"+serviceKey.Name] = svcLimits

		// 待删除的rateLimit
		if !item.Valid {
			del++
			svcLimits.DelRule(item.Id)
			continue
		}
		upsert++
		if item.Active {
			svcLimits.AddRule(item)
		} else {
			svcLimits.DelRule(item.Id)
		}
	}

	for _, svcLimits := range reloads {
		svcLimits.Reload()
	}

	return time.Unix(lastMtime, 0), upsert, del
}

// IteratorRateLimit 根据serviceID进行迭代回调
func (rlc *rateLimitCache) IteratorRateLimit(proc cachetypes.RateLimitIterProc) {
	rlc.ids.Range(func(key string, val *rules.RateLimit) {
		proc(val)
	})
}

// GetRateLimitByServiceID 根据serviceID获取限流数据
func (rlc *rateLimitCache) GetRateLimitRules(serviceKey svctypes.ServiceKey) ([]*rules.RateLimit, string) {
	return rlc.GetRateLimitRulesWithLabels(serviceKey, nil)
}

func (rlc *rateLimitCache) GetRateLimitRulesWithLabels(
	serviceKey svctypes.ServiceKey, labels map[string]string,
) ([]*rules.RateLimit, string) {
	// 获取对应服务的限流规则配置
	svcRules := rlc.GetRateLimitConfig(serviceKey.Name, serviceKey.Namespace)
	if svcRules == nil {
		return nil, ""
	}

	// 收集所有规则
	var releases []*rules.RateLimitRelease
	var bases []*rules.RuleRelease
	svcRules.Rules.Range(func(key string, release *rules.RateLimitRelease) {
		if release != nil && release.Rule != nil {
			releases = append(releases, release)
			bases = append(bases, &release.RuleRelease)
		}
	})
	selected, snapshotRevision := selectGovernanceReleases(bases, labels)
	selectedIDs := make(map[string]struct{}, len(selected))
	for _, release := range selected {
		selectedIDs[release.Id] = struct{}{}
	}
	rateLimitRules := make([]*rules.RateLimit, 0, len(selected))
	for _, release := range releases {
		if _, ok := selectedIDs[release.Id]; ok {
			rateLimitRules = append(rateLimitRules, release.Rule)
		}
	}
	return rateLimitRules, snapshotRevision
}

// GetRateLimitsCount 获取限流规则总数
func (rlc *rateLimitCache) GetRateLimitsCount() int {
	return rlc.ids.Len()
}

func (rlc *rateLimitCache) resolveRuleServiceKey(rateLimit *rules.RateLimit) (svctypes.ServiceKey, bool) {
	rlc.lock.Lock()
	defer rlc.lock.Unlock()
	if rateLimit == nil || rateLimit.ServiceID == "" {
		return svctypes.ServiceKey{}, false
	}
	svcId := rateLimit.ServiceID
	svc := rlc.svcCache.GetServiceByID(svcId)
	if svc == nil {
		svc2, err := rlc.storage.GetServiceByID(svcId)
		if err != nil {
			rlc.waitFixRules[rateLimit.ID] = struct{}{}
			return svctypes.ServiceKey{}, false
		}
		if svc2 == nil {
			delete(rlc.waitFixRules, rateLimit.ID)
			return svctypes.ServiceKey{}, false
		}
		svc = svc2
	}
	delete(rlc.waitFixRules, rateLimit.ID)
	return svctypes.ServiceKey{Namespace: svc.Namespace, Name: svc.Name}, true
}

// GetRule implements api.RateLimitCache.
func (rlc *rateLimitCache) GetRule(id string) *rules.RateLimit {
	rule, _ := rlc.ids.Load(id)
	return rule
}

// GetRateLimitConfig 根据 service name 和 namespace 获取限流规则
func (rlc *rateLimitCache) GetRateLimitConfig(name string, namespace string) *rules.ServiceWithRateLimits {
	log.Infof("[cache][rate_limit] GetRateLimitConfig: name %s, namespace %s", name, namespace)
	// check service specific
	rules := rlc.checkServiceSpecificCache(name, namespace)
	if nil != rules {
		return rules
	}
	if rules, ok := rlc.nsWildcardRules.Load(namespace); ok {
		return rules
	}
	return rlc.allWildcardRules
}

func (rlc *rateLimitCache) checkServiceSpecificCache(
	name string, namespace string) *rules.ServiceWithRateLimits {
	rlc.lock.RLock()
	defer rlc.lock.RUnlock()
	log.Infof(
		"[cache][rate_limit] checkServiceSpecificCache name %s, namespace %s, values %v", name, namespace, rlc.svcSpecificRules)
	svcRules, ok := rlc.svcSpecificRules.Load(namespace)
	if ok {
		if rule, exists := svcRules.Load(name); exists {
			return rule
		}
	}
	return nil
}

// QueryRateLimitRules
func (rlc *rateLimitCache) QueryRateLimitRules(ctx context.Context, args *cacheapi.RateLimitRuleArgs) (uint32, []*rules.RateLimit, error) {
	if err := rlc.Update(); err != nil {
		return 0, nil, err
	}

	predicates := cacheapi.LoadRatelimitRulePredicates(ctx)

	hasService := len(args.Service) != 0
	hasNamespace := len(args.Namespace) != 0
	limitType := args.Filter["limit_type"]

	res := make([]*rules.RateLimit, 0, 8)
	process := func(rule *rules.RateLimit) {
		if hasNamespace && args.Namespace != rule.Namespace {
			return
		}
		if hasService {
			serviceKey, ok := rlc.resolveRuleServiceKey(rule)
			if !ok {
				return
			}
			if args.Service != serviceKey.Name {
				return
			}
		}
		if args.ID != "" && args.ID != rule.ID {
			return
		}
		if args.Name != "" {
			name, _ := matchs.ParseWildName(args.Name)
			if !strings.Contains(rule.Name, name) {
				return
			}
		}
		if limitType != "" {
			if rule.Proto.GetType().String() != limitType {
				return
			}
		}

		if args.Disable != nil && *args.Disable != rule.Disable {
			return
		}

		for i := range predicates {
			if !predicates[i](ctx, rule) {
				return
			}
		}

		res = append(res, rule)
	}
	rlc.IteratorRateLimit(process)
	amount, routings := rlc.sortBeforeTrim(res, args)
	return amount, routings, nil
}

func (rlc *rateLimitCache) sortBeforeTrim(rules []*rules.RateLimit, args *cacheapi.RateLimitRuleArgs) (uint32, []*rules.RateLimit) {
	amount := uint32(len(rules))
	if args.Offset >= amount || args.Limit == 0 {
		return amount, nil
	}
	sort.Slice(rules, func(i, j int) bool {
		asc := strings.ToLower(args.OrderType) == "asc" || args.OrderType == ""
		if strings.ToLower(args.OrderField) == "priority" {
			return orderByRateLimitPriority(rules[i], rules[j], asc)
		}
		return orderByRateLimitModifyTime(rules[i], rules[j], asc)
	})
	endIdx := args.Offset + args.Limit
	if endIdx > amount {
		endIdx = amount
	}
	return amount, rules[args.Offset:endIdx]
}

func orderByRateLimitPriority(a, b *rules.RateLimit, asc bool) bool {
	if a.Priority < b.Priority {
		return asc
	}
	if a.Priority > b.Priority {
		// false && asc always false
		return false
	}
	return strings.Compare(a.ID, b.ID) < 0 && asc
}

func orderByRateLimitModifyTime(a, b *rules.RateLimit, asc bool) bool {
	if a.ModifyTime.After(b.ModifyTime) {
		return asc
	}
	if a.ModifyTime.Before(b.ModifyTime) {
		return false
	}
	return strings.Compare(a.ID, b.ID) < 0 && asc
}
