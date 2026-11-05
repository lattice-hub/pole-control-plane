package config

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
)

// Discoverer 由统一 Service 实现，原子执行鉴权、发布选择、兼容开关和有效 values 解析。
type Discoverer interface {
	DiscoverForProtocol(context.Context, *configv1.DiscoverRequest, string) *apimodel.Response
}

// Rendered 同时保留存在状态与最终字节摘要；只有成功 PRESENT 才有正文。
type Rendered struct {
	Result  *configv1.ConfigResult
	Content []byte
	MD5     string
}

// Error 是适配边界的公共错误，不泄露渲染输入。
type Error struct{ Code apimodel.Code }

func (e *Error) Error() string { return fmt.Sprintf("configuration request failed: %s", e.Code) }

func (e *Error) HTTPStatus() int {
	switch e.Code {
	case apimodel.Code_ClientAPINotOpen, apimodel.Code_NotAllowedAccess, apimodel.Code_AuthTokenForbidden, apimodel.Code_OperationRoleForbidden:
		return http.StatusForbidden
	case apimodel.Code_Unauthorized:
		return http.StatusUnauthorized
	case apimodel.Code_InvalidParameter:
		return http.StatusBadRequest
	case apimodel.Code_NotFoundResource:
		return http.StatusNotFound
	default:
		return http.StatusServiceUnavailable
	}
}

// Read 每次从权威 Service 重新读取，失败不生成空配置或沿用缓存。
func Read(ctx context.Context, service Discoverer, protocol string, request *configv1.DiscoverRequest) (*Rendered, error) {
	if ctx.Err() != nil || service == nil || request == nil || request.Key == nil {
		return nil, &Error{apimodel.Code_ConfigUnavailable}
	}
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response := service.DiscoverForProtocol(readCtx, request, protocol)
	if readCtx.Err() != nil || response == nil {
		return nil, &Error{apimodel.Code_ConfigUnavailable}
	}
	if response.GetCode() != uint32(apimodel.Code_ExecuteSuccess) {
		return nil, &Error{apimodel.Code(response.GetCode())}
	}
	result := &configv1.ConfigResult{}
	if response.Data == nil || response.Data.UnmarshalTo(result) != nil {
		return nil, &Error{apimodel.Code_ConfigUnavailable}
	}
	if result.Key == nil || !proto.Equal(result.Key, request.Key) || result.ObservationCursor == nil || result.ObservationCursor.Scope == "" || result.ObservationCursor.Epoch == "" || result.SnapshotRevision == "" {
		return nil, &Error{apimodel.Code_ConfigUnavailable}
	}
	rendered := &Rendered{Result: result}
	switch result.Presence {
	case configv1.ConfigPresence_CONFIG_PRESENCE_ABSENT:
		if result.Snapshot != nil || result.ReleaseId != "" {
			return nil, &Error{apimodel.Code_ConfigUnavailable}
		}
		return rendered, nil
	case configv1.ConfigPresence_CONFIG_PRESENCE_PRESENT:
		if result.ReleaseId == "" || result.Snapshot == nil {
			return nil, &Error{apimodel.Code_ConfigUnavailable}
		}
		content, err := Render(ctx, result.Snapshot)
		if err != nil {
			return nil, &Error{apimodel.Code_ConfigRenderFailed}
		}
		sum := md5.Sum(content)
		rendered.Content = content
		rendered.MD5 = hex.EncodeToString(sum[:])
		return rendered, nil
	default:
		return nil, &Error{apimodel.Code_ConfigUnavailable}
	}
}

// Wait 以一秒节奏驱动外部长轮询；等待预算最多60秒，每批重读最多5秒。
// 首次及超时终检均重新鉴权，终检仍可额外占用一次5秒读取预算，不占用原生 Watch。
func Wait(ctx context.Context, timeout time.Duration, check func(context.Context) (bool, error)) error {
	if timeout < 0 {
		timeout = 0
	}
	if timeout > 60*time.Second {
		timeout = 60 * time.Second
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		changed, err := boundedCheck(ctx, check)
		if err != nil || changed || timeout == 0 {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			_, err := boundedCheck(ctx, check)
			return err
		case <-ticker.C:
		}
	}
}

func boundedCheck(ctx context.Context, check func(context.Context) (bool, error)) (bool, error) {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	changed, err := check(bounded)
	if err == nil && bounded.Err() != nil {
		return false, bounded.Err()
	}
	return changed, err
}
