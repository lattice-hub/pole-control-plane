package config

import (
	"context"
	"errors"
	"unicode/utf8"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/pkg/types"
	compatconfig "github.com/pole-io/pole-server/pkg/compatibility/config"
	nacosmodel "github.com/pole-io/pole-server/plugin/apiserver/nacosserver/model"
	nacospb "github.com/pole-io/pole-server/plugin/apiserver/nacosserver/v2/pb"
	"github.com/pole-io/pole-server/plugin/apiserver/nacosserver/v2/remote"
)

const ErrorConfigNotFound = 300

const ErrorConfigQueryConflict = 400

func failure(err error) *nacospb.Response {
	code := apimodel.Code_ConfigUnavailable
	var protocolError *compatconfig.Error
	if errors.As(err, &protocolError) {
		code = protocolError.Code
	}
	return &nacospb.Response{ResultCode: int(nacosmodel.Response_Fail.Code), ErrorCode: int(code), Message: "configuration request failed: " + code.String()}
}

func disabledWrite() *nacospb.Response {
	return &nacospb.Response{ResultCode: int(nacosmodel.Response_Fail.Code), ErrorCode: 405, Message: "use the unified configuration draft and CAS publish API"}
}

func (h *ConfigServer) handlePublishConfigRequest(context.Context, nacospb.BaseRequest, nacospb.RequestMeta) (nacospb.BaseResponse, error) {
	return &nacospb.ConfigPublishResponse{Response: disabledWrite()}, nil
}

func (h *ConfigServer) handleDeleteConfigRequest(context.Context, nacospb.BaseRequest, nacospb.RequestMeta) (nacospb.BaseResponse, error) {
	return &nacospb.ConfigRemoveResponse{Response: disabledWrite()}, nil
}

// read 使用连接声明的labels和传输client IP选流量，认证仍只来自独立上下文凭证。
func (h *ConfigServer) read(ctx context.Context, tenant, group, name string, meta nacospb.RequestMeta) (*compatconfig.Rendered, error) {
	labels := make(map[string]string, len(meta.Labels)+1)
	for key, value := range meta.Labels {
		labels[key] = value
	}
	labels[types.ClientLabel_IP] = meta.ClientIP
	rendered, err := compatconfig.Read(ctx, h.configSvr, "nacos", &configv1.DiscoverRequest{Key: &configv1.ConfigKey{Environment: nacosmodel.ToPolarisNamespace(tenant), Group: group, Name: name}, Labels: labels})
	if err != nil {
		return nil, err
	}
	// JSON string无法承载非UTF-8；读取和通知均拒绝，避免通知一个客户端无法安装的版本。
	if !utf8.Valid(rendered.Content) {
		return nil, &compatconfig.Error{Code: apimodel.Code_ConfigRenderFailed}
	}
	return rendered, nil
}

func (h *ConfigServer) handleGetConfigRequest(ctx context.Context, req nacospb.BaseRequest, meta nacospb.RequestMeta) (nacospb.BaseResponse, error) {
	query, ok := req.(*nacospb.ConfigQueryRequest)
	if !ok || query.ConfigRequest == nil {
		return nil, remote.ErrorInvalidRequestBodyType
	}
	rendered, err := h.read(ctx, query.Tenant, query.Group, query.DataId, meta)
	if err != nil {
		return &nacospb.ConfigQueryResponse{Response: failure(err)}, nil
	}
	if rendered.Result.Presence == configv1.ConfigPresence_CONFIG_PRESENCE_ABSENT {
		return &nacospb.ConfigQueryResponse{Response: &nacospb.Response{ResultCode: int(nacosmodel.Response_Fail.Code), ErrorCode: ErrorConfigNotFound, Message: "config data not exist"}}, nil
	}
	return &nacospb.ConfigQueryResponse{Response: &nacospb.Response{Success: true, ResultCode: int(nacosmodel.Response_Success.Code)}, Content: string(rendered.Content), Md5: rendered.MD5, ContentType: rendered.Result.Snapshot.Format}, nil
}

func (h *ConfigServer) handleWatchConfigRequest(ctx context.Context, req nacospb.BaseRequest, meta nacospb.RequestMeta) (nacospb.BaseResponse, error) {
	watch, ok := req.(*nacospb.ConfigBatchListenRequest)
	if !ok {
		return nil, remote.ErrorInvalidRequestBodyType
	}
	response := nacospb.NewConfigChangeBatchListenResponse()
	if len(watch.ConfigListenContexts) > 128 {
		response.Response = failure(&compatconfig.Error{Code: apimodel.Code_InvalidParameter})
		return response, nil
	}
	if !watch.Listen {
		h.removeInterests(meta.ConnectionID, watch.ConfigListenContexts)
		return response, nil
	}
	for _, item := range watch.ConfigListenContexts {
		rendered, err := h.read(ctx, item.Tenant, item.Group, item.DataId, meta)
		if err != nil {
			response.Response = failure(err)
			response.ChangedConfigs = nil
			return response, nil
		}
		if rendered.MD5 != item.Md5 {
			response.ChangedConfigs = append(response.ChangedConfigs, nacospb.ConfigContext{Tenant: item.Tenant, Group: item.Group, DataId: item.DataId})
		}
	}
	if err := h.watch(ctx, meta, watch.ConfigListenContexts); err != nil {
		response.Response = failure(err)
		response.ChangedConfigs = nil
	}
	return response, nil
}
