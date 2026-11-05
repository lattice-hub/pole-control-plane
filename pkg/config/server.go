// Package config 负责配置领域服务的启动装配，业务状态由 unified 与持久化存储拥有。
package config

import (
	"context"
	"errors"
	"sync"

	authapi "github.com/pole-io/pole-server/apis/access_control/auth"
	cacheapi "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/config/unified"
)

// Config 只控制统一配置模块是否开放，不再配置旧文件发布缓存或渲染链。
type Config struct {
	Open bool `yaml:"open"`
}

var runtime struct {
	sync.RWMutex
	service *unified.Service
}

// Initialize 装配唯一配置服务；Open=false 关闭服务，依赖缺失则启动失败。
// 例如合法主库存储与权限组件 → HTTP/Discover 共用同一服务；不支持统一存储 → 返回错误。
func Initialize(ctx context.Context, option Config, storage store.Store, caches cacheapi.CacheManager,
	users authapi.UserServer, checker authapi.AuthChecker) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	runtime.Lock()
	defer runtime.Unlock()
	// 重新装配失败时不得继续暴露上一份依赖和权限配置。
	runtime.service = nil
	if !option.Open {
		return nil
	}
	repository, ok := storage.(store.UnifiedConfigStore)
	if !ok {
		return errors.New("configuration store does not support unified configuration")
	}
	authorizer, err := NewUnifiedAuthorizer(caches, users, checker)
	if err != nil {
		return err
	}
	service, err := unified.New(repository, authorizer)
	if err != nil {
		return err
	}
	runtime.service = service
	return nil
}

// GetUnifiedService 返回已完成启动装配的服务；关闭或未初始化时明确拒绝创建入口。
func GetUnifiedService() (*unified.Service, error) {
	runtime.RLock()
	defer runtime.RUnlock()
	if runtime.service == nil {
		return nil, errors.New("configuration module is unavailable")
	}
	return runtime.service, nil
}
