package config

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/emicklei/go-restful/v3"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"

	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/pkg/common/utils"
	httputils "github.com/pole-io/pole-server/pkg/common/utils/http"
	compatconfig "github.com/pole-io/pole-server/pkg/compatibility/config"
	"github.com/pole-io/pole-server/plugin/apiserver/nacosserver/model"
	nacoshttp "github.com/pole-io/pole-server/plugin/apiserver/nacosserver/v1/http"
)

func writeDisabled() error {
	return &model.NacosError{ErrCode: http.StatusMethodNotAllowed, ErrMsg: "use the unified configuration draft and CAS publish API"}
}

func (n *ConfigServer) handlePublishConfig(context.Context, *model.ConfigFile) (bool, error) {
	return false, writeDisabled()
}

func (n *ConfigServer) handleDeleteConfig(context.Context, *model.ConfigFile) (bool, error) {
	return false, writeDisabled()
}

// configRequestContext 保留原请求取消及认证头；accessToken 仅作为认证输入，不注入选择标签。
func configRequestContext(handler *nacoshttp.Handler) context.Context {
	ctx := httputils.ParseHeaderContext(handler.Request, handler.Response)
	if token := handler.Request.Request.FormValue("accessToken"); token != "" {
		ctx = context.WithValue(ctx, types.ContextAuthTokenKey, token)
	}
	return ctx
}

func nacosError(err error) error {
	var protocolError *compatconfig.Error
	if errors.As(err, &protocolError) {
		return &model.NacosError{ErrCode: int32(protocolError.HTTPStatus()), ErrMsg: protocolError.Error()}
	}
	return &model.NacosError{ErrCode: http.StatusServiceUnavailable, ErrMsg: "configuration request unavailable"}
}

func (n *ConfigServer) read(ctx context.Context, tenant, group, name string) (*compatconfig.Rendered, error) {
	return compatconfig.Read(ctx, n.configSvr, "nacos", &configv1.DiscoverRequest{Key: &configv1.ConfigKey{Environment: model.ToPolarisNamespace(tenant), Group: group, Name: name}, Labels: map[string]string{types.ClientLabel_IP: utils.ParseClientIP(ctx)}})
}

func (n *ConfigServer) handleGetConfig(ctx context.Context, req *model.ConfigFile, rsp *restful.Response) (string, error) {
	rendered, err := n.read(ctx, req.Namespace, req.Group, req.DataId)
	if err != nil {
		return "", nacosError(err)
	}
	if rendered.Result.Presence == configv1.ConfigPresence_CONFIG_PRESENCE_ABSENT {
		return "", &model.NacosError{ErrCode: http.StatusNotFound, ErrMsg: "config data not exist"}
	}
	disableCache(rsp)
	rsp.AddHeader(model.HeaderContentMD5, rendered.MD5)
	return string(rendered.Content), nil
}

// handleWatch 使用查询驱动长轮询；失败不通知删除，成功空正文的MD5仍非空。
func (n *ConfigServer) handleWatch(ctx context.Context, watch *model.ConfigWatchContext, rsp *restful.Response) {
	if len(watch.Items) > 128 {
		nacoshttp.WrirteNacosErrorResponse(model.InvalidProbeModify, rsp)
		return
	}
	timeout, longPoll := watch.IsSupportLongPolling()
	if !longPoll || watch.IsNoHangUp() {
		timeout = 0
	}
	if timeout > 60*time.Second {
		timeout = 60 * time.Second
	}
	var changed []*model.ConfigListenItem
	err := compatconfig.Wait(ctx, timeout, func(ctx context.Context) (bool, error) {
		changed = nil
		for _, item := range watch.Items {
			rendered, err := n.read(ctx, item.Tenant, item.Group, item.DataId)
			if err != nil {
				return false, err
			}
			if rendered.MD5 != item.Md5 {
				changed = append(changed, item)
			}
		}
		return len(changed) > 0, nil
	})
	if err != nil {
		nacoshttp.WrirteNacosErrorResponse(nacosError(err), rsp)
		return
	}
	disableCache(rsp)
	nacoshttp.WrirteSimpleResponse(md5ResultString(changed), http.StatusOK, rsp)
}

func md5ResultString(items []*model.ConfigListenItem) string {
	var result strings.Builder
	for _, item := range items {
		result.WriteString(item.DataId)
		result.WriteRune(model.WordSeparatorRune)
		result.WriteString(item.Group)
		if item.Tenant != "" {
			result.WriteRune(model.WordSeparatorRune)
			result.WriteString(item.Tenant)
		}
		result.WriteRune(model.LineSeparatorRune)
	}
	return url.QueryEscape(result.String())
}

func disableCache(rsp *restful.Response) {
	rsp.AddHeader(model.HeaderExpires, "0")
	rsp.AddHeader(model.HeaderPragma, "no-cache")
	rsp.AddHeader(model.HeaderCacheControl, "no-cache,no-store")
}
