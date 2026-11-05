package store

import (
	"bytes"
	"context"
	"errors"
	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	"google.golang.org/protobuf/proto"
)

var ErrConfigFileConflict = errors.New("configuration file revision changed")

// UnifiedConfigFileStore 独立存储共享正文与环境目录，不改变环境聚合。
type UnifiedConfigFileStore interface {
	ReadConfigFileTemplate(context.Context, *configv1.ConfigFileKey) (*configv1.ConfigFileTemplate, error)
	SaveConfigFileTemplate(context.Context, *configv1.ConfigFileTemplate, uint64) (*configv1.ConfigFileTemplate, error)
	ListConfigFiles(context.Context, *configv1.ListConfigFilesRequest) (*configv1.ConfigFileList, error)
}

// SameConfigFileSharedDefinition 比较共享正文，不把环境 values 和规则纳入比较。
func SameConfigFileSharedDefinition(a, b *configv1.ConfigDefinition) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return bytes.Equal(a.Content, b.Content) && a.Engine == b.Engine && a.Format == b.Format && proto.Equal(&configv1.ConfigDefinition{Parameters: a.Parameters, Compatibility: a.Compatibility}, &configv1.ConfigDefinition{Parameters: b.Parameters, Compatibility: b.Compatibility})
}
