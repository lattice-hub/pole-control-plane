package store

import "context"

// UnifiedConfigInventory 提供环境删除保护和配置数量统计，包含草稿、下线与历史聚合。
// 存储失败必须返回错误，不能用零数量掩盖未知状态。
type UnifiedConfigInventory interface {
	CountUnifiedConfigs(context.Context, string) (uint64, error)
	CountUnifiedConfigsByEnvironment(context.Context) (map[string]uint64, error)
}
