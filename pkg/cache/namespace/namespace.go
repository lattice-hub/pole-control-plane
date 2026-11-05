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

package namespace

import (
	"context"
	"math"
	"sort"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	cacheapi "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/apis/store"
	cachebase "github.com/pole-io/pole-server/pkg/cache/base"
	"github.com/pole-io/pole-server/pkg/common/eventhub"
	"github.com/pole-io/pole-server/pkg/common/log"
	commonotel "github.com/pole-io/pole-server/pkg/common/otel"
	"github.com/pole-io/pole-server/pkg/common/syncs/container"
	matchs "github.com/pole-io/pole-server/pkg/common/utils/match"
)

var (
	_ cacheapi.NamespaceCache = (*namespaceCache)(nil)
)

type namespaceCache struct {
	*cachebase.BaseCache
	storage store.Store
	ids     *container.SyncMap[string, *types.Namespace]
	updater *singleflight.Group
}

func NewNamespaceCache(storage store.Store, cacheMgr cacheapi.CacheManager) cacheapi.NamespaceCache {
	return &namespaceCache{
		BaseCache: cachebase.NewBaseCache(storage, cacheMgr),
		storage:   storage,
	}
}

// Initialize
func (nsCache *namespaceCache) Initialize(c map[string]interface{}) error {
	nsCache.ids = container.NewSyncMap[string, *types.Namespace]()
	nsCache.updater = new(singleflight.Group)
	return nil
}

// Update
func (nsCache *namespaceCache) Update() error {
	// 多个线程竞争，只有一个线程进行更新
	err, _ := nsCache.singleUpdate()
	return err
}

func (nsCache *namespaceCache) realUpdate() (map[string]time.Time, int64, error) {
	lastTime := nsCache.LastFetchTime()
	// cache 为空时强制从 epoch 全量拉取，避免 lastFetchTime 被误设为当前时间后一直拉不到数据
	if nsCache.ids.Len() == 0 {
		lastTime = time.Unix(1, 0)
		log.Info("[Cache][Namespace] cache empty, force full load from store")
	}
	ret, err := nsCache.storage.GetMoreNamespaces(lastTime)
	if err != nil {
		log.Error("[Cache][Namespace] get storage more", zap.Error(err))
		return nil, -1, err
	}
	lastMtimes := nsCache.setNamespaces(ret)
	return lastMtimes, int64(len(ret)), nil
}

func (nsCache *namespaceCache) setNamespaces(nsSlice []*types.Namespace) map[string]time.Time {
	lastMtime := nsCache.LastMtime(nsCache.Name()).Unix()

	for index := range nsSlice {
		ns := nsSlice[index]
		oldNs, hasOldVal := nsCache.ids.Load(ns.Name)
		eventType := eventhub.EventCreated
		if !ns.Valid {
			eventType = eventhub.EventDeleted
			nsCache.ids.Delete(ns.Name)
		} else {
			if !hasOldVal {
				eventType = eventhub.EventCreated
			} else {
				eventType = eventhub.EventUpdated
			}
			nsCache.ids.Store(ns.Name, ns)
		}
		_ = eventhub.Publish(eventhub.CacheNamespaceEventTopic, &eventhub.CacheNamespaceEvent{
			OldItem:   oldNs,
			Item:      ns,
			EventType: eventType,
		})
		lastMtime = int64(math.Max(float64(lastMtime), float64(ns.ModifyTime.Unix())))
	}

	return map[string]time.Time{
		nsCache.Name(): time.Unix(lastMtime, 0),
	}
}

// Clear .
func (nsCache *namespaceCache) Clear() error {
	nsCache.BaseCache.Clear()
	nsCache.ids = container.NewSyncMap[string, *types.Namespace]()
	return nil
}

// Name .
func (nsCache *namespaceCache) Name() string {
	return cacheapi.NamespaceName
}

// GetNamespace get namespace by id
func (nsCache *namespaceCache) GetNamespace(id string) *types.Namespace {
	val, ok := nsCache.ids.Load(id)
	if !ok {
		return nil
	}
	return val
}

// GetNamespacesByName batch get namespace by name
func (nsCache *namespaceCache) GetNamespacesByName(names []string) []*types.Namespace {
	nsArr := make([]*types.Namespace, 0, len(names))
	for _, name := range names {
		if ns := nsCache.GetNamespace(name); ns != nil {
			nsArr = append(nsArr, ns)
		}
	}

	return nsArr
}

// GetNamespaceList
//
//	@receiver nsCache
//	@return []*types.Namespace
func (nsCache *namespaceCache) GetNamespaceList() []*types.Namespace {
	nsArr := make([]*types.Namespace, 0, 8)

	nsCache.ids.Range(func(key string, ns *types.Namespace) {
		nsArr = append(nsArr, ns)
	})

	return nsArr
}

// forceQueryUpdate 为了确保读取的数据是最新的，这里需要做一个强制 update 的动作进行数据读取处理
func (nsCache *namespaceCache) forceQueryUpdate() error {
	err, shared := nsCache.singleUpdate()
	// shared == true，表示当前已经有正在 update 执行的任务，这个任务不一定能够读取到最新的数据
	// 为了避免读取到脏数据，在发起一次 singleUpdate
	if shared {
		log.Debug("[Cache][Namespace] force query update from store")
		err, _ = nsCache.singleUpdate()
	}
	return err
}

func (nsCache *namespaceCache) singleUpdate() (error, bool) {
	// 多个线程竞争，只有一个线程进行更新
	_, err, shared := nsCache.updater.Do(nsCache.Name(), func() (interface{}, error) {
		return nil, nsCache.DoCacheUpdate(nsCache.Name(), nsCache.realUpdate)
	})
	return err, shared
}

func (nsCache *namespaceCache) Query(ctx context.Context, args *cacheapi.NamespaceArgs) (uint32, []*types.Namespace, error) {
	ctx, end := commonotel.StartSpan(ctx, "cache.namespace.query", "cache")
	defer end(nil)

	if err := nsCache.forceQueryUpdate(); err != nil {
		end(err)
		return 0, nil, err
	}

	ret := make([]*types.Namespace, 0, 32)

	predicates := cacheapi.LoadNamespacePredicates(ctx)

	searchName, hasName := args.Filter["name"]
	searchOwner, hasOwner := args.Filter["owner"]
	searchKind, hasKind := args.Filter["kind"]

	nsCache.ids.ReadRange(func(key string, val *types.Namespace) {
		for i := range predicates {
			if !predicates[i](ctx, val) {
				return
			}
		}

		if hasName {
			matchOne := false
			for i := range searchName {
				if matchs.IsWildMatch(val.Name, searchName[i]) {
					matchOne = true
					break
				}
			}
			// 如果没有匹配到，直接返回
			if !matchOne {
				return
			}
		}

		if hasOwner {
			matchOne := false
			for i := range searchOwner {
				if matchs.IsWildMatch(val.Owner, searchOwner[i]) {
					matchOne = true
					break
				}
			}
			// 如果没有匹配到，直接返回
			if !matchOne {
				return
			}
		}

		if hasKind {
			matchesKind := false
			for _, kind := range searchKind {
				if (kind == "business" && val.Kind == apimodel.NamespaceKind_NAMESPACE_KIND_BUSINESS) ||
					(kind == "system" && val.Kind == apimodel.NamespaceKind_NAMESPACE_KIND_SYSTEM) {
					matchesKind = true
					break
				}
			}
			if !matchesKind {
				return
			}
		}

		ret = append(ret, val)
	})

	sort.Slice(ret, func(i, j int) bool {
		return ret[i].ModifyTime.After(ret[j].ModifyTime)
	})

	total, ret := nsCache.toPage(len(ret), ret, args)
	return uint32(total), ret, nil
}

func (c *namespaceCache) toPage(total int, items []*types.Namespace,
	args *cacheapi.NamespaceArgs) (int, []*types.Namespace) {
	if len(items) == 0 {
		return 0, []*types.Namespace{}
	}
	if args.Limit == 0 {
		return total, items
	}
	if args.Offset > total {
		return total, []*types.Namespace{}
	}
	endIdx := args.Offset + args.Limit
	if endIdx > total {
		endIdx = total
	}
	return total, items[args.Offset:endIdx]
}
