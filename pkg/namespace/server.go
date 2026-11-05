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
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/sync/singleflight"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	cacheapi "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/observability/history"
	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/compatibility"
)

const (
	// SystemNamespace polaris system namespace
	SystemNamespace = "pole-system"
	// DefaultNamespace default namespace
	DefaultNamespace = "default"
	// ProductionNamespace default namespace
	ProductionNamespace = "Production"
	// DefaultTLL default ttl
	DefaultTLL = 5
)

func isProtectedNamespace(name string) bool {
	return name == SystemNamespace
}

func isSystemNamespace(namespace *types.Namespace) bool {
	return namespace != nil && namespace.Kind == apimodel.NamespaceKind_NAMESPACE_KIND_SYSTEM
}

var _ NamespaceOperateServer = (*Server)(nil)

type namespaceCacheManager interface {
	Namespace() cacheapi.NamespaceCache
	Service() cacheapi.ServiceCache
}

type Server struct {
	storage               store.Store
	caches                namespaceCacheManager
	createNamespaceSingle *singleflight.Group
	cfg                   Config
	compatibilityResolver *compatibility.Resolver
}

// GetNamespace exposes a narrow read seam for runtime configuration guards.
func (s *Server) GetNamespace(name string) (*types.Namespace, error) {
	return s.storage.GetNamespace(name)
}

func (s *Server) isProtectedNamespace(name string) bool {
	if isProtectedNamespace(name) ||
		(s.compatibilityResolver != nil && s.compatibilityResolver.IsEffectiveTarget(name)) {
		return true
	}
	runtimeStore, ok := s.storage.(store.SystemConfigRuntimeStore)
	if !ok {
		return false
	}
	release, err := runtimeStore.GetActiveSystemConfigRelease(compatibility.Component, compatibility.Domain)
	if err != nil {
		return true
	}
	if release == nil {
		return false
	}
	values := map[string]any{}
	if err := json.Unmarshal(release.Payload, &values); err != nil {
		return true
	}
	return name == strings.TrimSpace(fmt.Sprint(values[compatibility.NacosPublicNamespaceTargetKey])) ||
		name == strings.TrimSpace(fmt.Sprint(values[compatibility.EurekaDefaultNamespaceKey]))
}

// RecordHistory server对外提供history插件的简单封装
func (s *Server) RecordHistory(entry *types.RecordEntry) {
	// 如果插件没有初始化，那么不记录history
	if history.GetHistory() == nil {
		return
	}
	// 如果数据为空，则不需要打印了
	if entry == nil {
		return
	}

	// 调用插件记录history
	history.GetHistory().Record(entry)
}
