package config

import (
	"context"
	"sync"
	"time"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	compatconfig "github.com/pole-io/pole-server/pkg/compatibility/config"
	nacospb "github.com/pole-io/pole-server/plugin/apiserver/nacosserver/v2/pb"
	"github.com/pole-io/pole-server/plugin/apiserver/nacosserver/v2/remote"
)

type subscription struct {
	mu     sync.Mutex
	ctx    context.Context
	meta   nacospb.RequestMeta
	items  map[nacospb.ConfigContext]nacospb.ConfigListenContext
	cancel context.CancelFunc
}

func interestKey(item nacospb.ConfigListenContext) nacospb.ConfigContext {
	return nacospb.ConfigContext{Tenant: item.Tenant, Group: item.Group, DataId: item.DataId}
}

// watch 每连接最多128项、一个查询循环；连接流取消或服务器Stop回收，正文不缓存。
func (h *ConfigServer) watch(ctx context.Context, meta nacospb.RequestMeta, items []nacospb.ConfigListenContext) error {
	if h.connMgr == nil {
		return &compatconfig.Error{Code: apimodel.Code_ConfigUnavailable}
	}
	client, ok := h.connMgr.GetClient(meta.ConnectionID)
	if !ok {
		return &compatconfig.Error{Code: apimodel.Code_ConfigUnavailable}
	}
	stream, ok := client.LoadStream()
	if !ok {
		return &compatconfig.Error{Code: apimodel.Code_ConfigUnavailable}
	}
	runCtx, cancel := context.WithCancel(stream.Context())
	candidate := &subscription{ctx: context.WithoutCancel(ctx), meta: meta, items: map[nacospb.ConfigContext]nacospb.ConfigListenContext{}, cancel: cancel}
	actual, loaded := h.watches.LoadOrStore(meta.ConnectionID, candidate)
	sub := actual.(*subscription)
	if loaded {
		cancel()
	}
	sub.mu.Lock()
	merged := make(map[nacospb.ConfigContext]nacospb.ConfigListenContext, len(sub.items)+len(items))
	for key, item := range sub.items {
		merged[key] = item
	}
	for _, item := range items {
		merged[interestKey(item)] = item
	}
	if len(merged) > 128 {
		sub.mu.Unlock()
		if !loaded {
			h.watches.Delete(meta.ConnectionID)
			cancel()
		}
		return &compatconfig.Error{Code: apimodel.Code_InvalidParameter}
	}
	sub.items = merged
	sub.ctx = context.WithoutCancel(ctx)
	sub.meta = meta
	sub.mu.Unlock()
	if !loaded {
		go h.poll(runCtx, meta.ConnectionID, sub, stream)
	}
	return nil
}

func (h *ConfigServer) removeInterests(id string, items []nacospb.ConfigListenContext) {
	actual, ok := h.watches.Load(id)
	if !ok {
		return
	}
	sub := actual.(*subscription)
	sub.mu.Lock()
	defer sub.mu.Unlock()
	for _, item := range items {
		delete(sub.items, interestKey(item))
	}
}

func (h *ConfigServer) poll(ctx context.Context, id string, sub *subscription, stream *remote.SyncServerStream) {
	defer h.watches.CompareAndDelete(id, sub)
	defer sub.cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		sub.mu.Lock()
		requestCtx, meta := sub.ctx, sub.meta
		items := make([]nacospb.ConfigListenContext, 0, len(sub.items))
		for _, item := range sub.items {
			items = append(items, item)
		}
		sub.mu.Unlock()
		for _, item := range items {
			// 单次读取具有5秒预算，连接关闭会同步取消当前读取。
			readCtx, cancel := context.WithTimeout(requestCtx, 5*time.Second)
			stop := context.AfterFunc(ctx, cancel)
			rendered, err := h.read(readCtx, item.Tenant, item.Group, item.DataId, meta)
			stop()
			cancel()
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				continue
			}
			if rendered.MD5 == item.Md5 {
				continue
			}
			notification := nacospb.NewConfigChangeNotifyRequest()
			notification.Tenant = item.Tenant
			notification.Group = item.Group
			notification.DataId = item.DataId
			payload, err := remote.MarshalPayload(notification)
			if err != nil {
				continue
			}
			if err := stream.SendMsg(payload); err != nil {
				return
			}
		}
	}
}

// Stop 释放所有兼容监听，避免协议重启遗留查询循环。
func (h *ConfigServer) Stop() {
	h.watches.Range(func(key, value any) bool { value.(*subscription).cancel(); h.watches.Delete(key); return true })
}
