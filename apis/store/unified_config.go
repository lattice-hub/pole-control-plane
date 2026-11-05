package store

import (
	"context"
	"errors"

	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
)

var (
	// ErrUnifiedConfigKeyInvalid 表示配置标识缺失或回调试图改变聚合身份。
	ErrUnifiedConfigKeyInvalid = errors.New("unified config key is invalid")
	// ErrUnifiedConfigCursorInvalid 表示游标属于其他范围、旧 epoch 或超出当前日志位置。
	ErrUnifiedConfigCursorInvalid = errors.New("unified config cursor is invalid")
)

// UnifiedConfigSnapshot 是一次一致读取的配置聚合与日志观察位置；缺失聚合仍返回游标。
// 调用者拥有返回数据，修改它不得影响后续读取或已经提交的历史。
type UnifiedConfigSnapshot struct {
	State  *UnifiedConfigAggregate
	Cursor *configv1.Cursor
}

// UnifiedConfigChange 声明事务是否改变运行时视图；纯草稿变更不推进运行时游标。
type UnifiedConfigChange struct {
	Runtime bool
	Kind    string
	Actor   string
}

// UnifiedConfigMutation 在锁定的聚合上执行一次领域操作。
// 返回错误必须整体回滚，返回 nil change 表示无变更；回调不得执行外部 I/O 或保留可变引用。
type UnifiedConfigMutation func(*UnifiedConfigAggregate) (*UnifiedConfigChange, error)

// UnifiedConfigEvent 与运行时状态同事务提交，按提交可见顺序重放；不包含配置正文或敏感参数。
type UnifiedConfigEvent struct {
	Cursor *configv1.Cursor
	Key    *configv1.ConfigKey
	Kind   string
	Actor  string
}

// UnifiedConfigStore 是新配置聚合的持久化边界，不复用旧模板、Value 和绑定发布对象。
// 写入串行化同一聚合；运行时提交原子更新状态、持久游标和事件。
// 所有读取使用主库一致快照，不能给落后配置附上最新日志位置。
type UnifiedConfigStore interface {
	ReadUnifiedConfig(context.Context, *configv1.ConfigKey) (*UnifiedConfigSnapshot, error)
	UpdateUnifiedConfig(context.Context, *configv1.ConfigKey, UnifiedConfigMutation) (*UnifiedConfigSnapshot, error)
	// ReadUnifiedConfigEvents 返回 after 后的事件和一致快照高水位；nil after 从当前 epoch 起点开始。
	// 分页有数据时用最后 event.Cursor 续读，空页可推进到高水位；非法游标返回 ErrUnifiedConfigCursorInvalid。
	ReadUnifiedConfigEvents(context.Context, *configv1.Cursor, uint32) ([]*UnifiedConfigEvent, *configv1.Cursor, error)
}

// UnifiedConfigRuntimeReader 在同一主库快照中读取当前生效集合与游标。
// 返回状态不含草稿或 inactive 历史，active release 定义必须完整；缺失引用返回错误。
type UnifiedConfigRuntimeReader interface {
	ReadUnifiedConfigRuntime(context.Context, *configv1.ConfigKey) (*UnifiedConfigSnapshot, error)
}

// UnifiedConfigEpochRotator 是恢复工具使用的显式换代能力；恢复后开放服务前调用。
// 保留配置聚合，创建并持久化随机 epoch，同时原子归零游标和清除旧日志。
type UnifiedConfigEpochRotator interface {
	RotateUnifiedConfigEpoch(context.Context) (*configv1.Cursor, error)
}

// UnifiedConfigAggregate 是完整内部聚合，独立于有界的 ConfigState 管理投影。
// Members 包括全部历史；存储以独立不可变 release 行承载正文。
type UnifiedConfigAggregate struct {
	Key                *configv1.ConfigKey    `json:"key"`
	Draft              *configv1.ConfigDraft  `json:"draft,omitempty"`
	Members            []*UnifiedConfigMember `json:"members,omitempty"`
	ReleaseSetRevision uint64                 `json:"release_set_revision"`
	Incarnation        string                 `json:"incarnation"`
	Offline            bool                   `json:"offline"`
}

// UnifiedConfigMember 将可变生效关系与不可变发布分开。
type UnifiedConfigMember struct {
	Release *configv1.ConfigRelease `json:"release"`
	Active  bool                    `json:"active"`
}

// Clone 将聚合所有权转交给调用方，避免事务回调修改已提交数据。
func (s *UnifiedConfigAggregate) Clone() *UnifiedConfigAggregate {
	if s == nil {
		return nil
	}
	c := *s
	if s.Key != nil {
		c.Key = proto.Clone(s.Key).(*configv1.ConfigKey)
	}
	if s.Draft != nil {
		c.Draft = proto.Clone(s.Draft).(*configv1.ConfigDraft)
	}
	if s.Members != nil {
		c.Members = make([]*UnifiedConfigMember, len(s.Members))
	}
	for i, m := range s.Members {
		if m != nil {
			v := *m
			if m.Release != nil {
				v.Release = proto.Clone(m.Release).(*configv1.ConfigRelease)
			}
			c.Members[i] = &v
		}
	}
	return &c
}
func (s *UnifiedConfigAggregate) GetKey() *configv1.ConfigKey {
	if s == nil {
		return nil
	}
	return s.Key
}
func (s *UnifiedConfigAggregate) GetDraft() *configv1.ConfigDraft {
	if s == nil {
		return nil
	}
	return s.Draft
}
func (s *UnifiedConfigAggregate) GetMembers() []*UnifiedConfigMember {
	if s == nil {
		return nil
	}
	return s.Members
}
func (s *UnifiedConfigAggregate) GetReleaseSetRevision() uint64 {
	if s == nil {
		return 0
	}
	return s.ReleaseSetRevision
}
func (s *UnifiedConfigAggregate) GetIncarnation() string {
	if s == nil {
		return ""
	}
	return s.Incarnation
}
func (s *UnifiedConfigAggregate) GetOffline() bool { return s != nil && s.Offline }
func (s *UnifiedConfigMember) GetRelease() *configv1.ConfigRelease {
	if s == nil {
		return nil
	}
	return s.Release
}
func (s *UnifiedConfigMember) GetActive() bool { return s != nil && s.Active }
