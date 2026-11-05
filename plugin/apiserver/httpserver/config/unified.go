package config

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/emicklei/go-restful/v3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/pkg/types"
	httpcommon "github.com/pole-io/pole-server/plugin/apiserver/httpserver/utils"
)

// 管理请求包含完整定义，限制整个 JSON 文档，避免规则字段绕过正文大小限制。
const unifiedMaxBodyBytes = 4 << 20

// GetUnifiedAccessServer 只挂载配置管理动作；原生客户端读取使用 ConfigGRPC/Discover。
func (h *HTTPServer) GetUnifiedAccessServer(consoleEnabled bool, consoleInclude []string) *restful.WebService {
	ws := new(restful.WebService)
	ws.Path("/config/v1").Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	readManagement, writeManagement := false, false
	if consoleEnabled {
		if len(consoleInclude) == 0 {
			consoleInclude = []string{defaultAccess}
		}
		for _, include := range consoleInclude {
			switch include {
			case defaultReadAccess:
				readManagement = true
			case defaultAccess, configConsoleAccess:
				readManagement, writeManagement = true, true
			}
		}
	}
	if readManagement {
		ws.Route(unifiedPOST(ws, "/files").To(unifiedHandler(h, func() *configv1.ListConfigFilesRequest { return &configv1.ListConfigFilesRequest{} }, func(ctx context.Context, req *configv1.ListConfigFilesRequest) *apimodel.Response {
			return h.unifiedService.ListFiles(ctx, req)
		})))
		ws.Route(unifiedPOST(ws, "/file-template").To(unifiedHandler(h, func() *configv1.GetFileTemplateRequest { return &configv1.GetFileTemplateRequest{} }, func(ctx context.Context, req *configv1.GetFileTemplateRequest) *apimodel.Response {
			return h.unifiedService.GetFileTemplate(ctx, req)
		})))
		ws.Route(unifiedPOST(ws, "/state").To(unifiedHandler(h, func() *configv1.GetStateRequest { return &configv1.GetStateRequest{} }, func(ctx context.Context, req *configv1.GetStateRequest) *apimodel.Response {
			return h.unifiedService.GetState(ctx, req)
		})))
	}
	if writeManagement {
		ws.Route(unifiedPOST(ws, "/file-template-draft").To(unifiedHandler(h, func() *configv1.SaveFileTemplateRequest { return &configv1.SaveFileTemplateRequest{} }, func(ctx context.Context, req *configv1.SaveFileTemplateRequest) *apimodel.Response {
			return h.unifiedService.SaveFileTemplate(ctx, req)
		})))
		ws.Route(unifiedPOST(ws, "/draft").To(unifiedHandler(h, func() *configv1.SaveDraftRequest { return &configv1.SaveDraftRequest{} }, func(ctx context.Context, req *configv1.SaveDraftRequest) *apimodel.Response {
			return h.unifiedService.SaveDraft(ctx, req)
		})))
		ws.Route(unifiedPOST(ws, "/preview").To(unifiedHandler(h, func() *configv1.PreviewRequest { return &configv1.PreviewRequest{} }, func(ctx context.Context, req *configv1.PreviewRequest) *apimodel.Response {
			return h.unifiedService.Preview(ctx, req)
		})))
		ws.Route(unifiedPOST(ws, "/publish").To(unifiedHandler(h, func() *configv1.PublishRequest { return &configv1.PublishRequest{} }, func(ctx context.Context, req *configv1.PublishRequest) *apimodel.Response {
			return h.unifiedService.Publish(ctx, req)
		})))
		ws.Route(unifiedPOST(ws, "/promote-gray").To(unifiedHandler(h, func() *configv1.PromoteGrayRequest { return &configv1.PromoteGrayRequest{} }, func(ctx context.Context, req *configv1.PromoteGrayRequest) *apimodel.Response {
			return h.unifiedService.PromoteGray(ctx, req)
		})))
		ws.Route(unifiedPOST(ws, "/stop").To(unifiedHandler(h, func() *configv1.StopRequest { return &configv1.StopRequest{} }, func(ctx context.Context, req *configv1.StopRequest) *apimodel.Response {
			return h.unifiedService.Stop(ctx, req)
		})))
		ws.Route(unifiedPOST(ws, "/stop-all-gray").To(unifiedHandler(h, func() *configv1.StopAllGrayRequest { return &configv1.StopAllGrayRequest{} }, func(ctx context.Context, req *configv1.StopAllGrayRequest) *apimodel.Response {
			return h.unifiedService.StopAllGray(ctx, req)
		})))
		ws.Route(unifiedPOST(ws, "/copy-release-to-draft").To(unifiedHandler(h, func() *configv1.CopyReleaseToDraftRequest { return &configv1.CopyReleaseToDraftRequest{} }, func(ctx context.Context, req *configv1.CopyReleaseToDraftRequest) *apimodel.Response {
			return h.unifiedService.CopyReleaseToDraft(ctx, req)
		})))
		ws.Route(unifiedPOST(ws, "/rollback").To(unifiedHandler(h, func() *configv1.RollbackRequest { return &configv1.RollbackRequest{} }, func(ctx context.Context, req *configv1.RollbackRequest) *apimodel.Response {
			return h.unifiedService.Rollback(ctx, req)
		})))
		ws.Route(unifiedPOST(ws, "/offline").To(unifiedHandler(h, func() *configv1.OfflineRequest { return &configv1.OfflineRequest{} }, func(ctx context.Context, req *configv1.OfflineRequest) *apimodel.Response {
			return h.unifiedService.Offline(ctx, req)
		})))
	}
	return ws
}

func unifiedPOST(ws *restful.WebService, path string) *restful.RouteBuilder {
	return ws.POST(path).Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
}

func unifiedHandler[T proto.Message](h *HTTPServer, request func() T, invoke func(context.Context, T) *apimodel.Response) restful.RouteFunction {
	return func(req *restful.Request, rsp *restful.Response) {
		parsed := request()
		req.Request.Body = http.MaxBytesReader(rsp, req.Request.Body, unifiedMaxBodyBytes)
		body, err := io.ReadAll(req.Request.Body)
		if err != nil {
			var maximum *http.MaxBytesError
			status := http.StatusBadRequest
			if errors.As(err, &maximum) {
				status = http.StatusRequestEntityTooLarge
			}
			writeUnified(req, rsp, unifiedFailure(apimodel.Code_InvalidParameter, "invalid request body"), status)
			return
		}
		if err := protojson.Unmarshal(body, parsed); err != nil {
			writeUnified(req, rsp, unifiedFailure(apimodel.Code_InvalidParameter, "invalid protobuf JSON request"), http.StatusBadRequest)
			return
		}
		if h.unifiedService == nil {
			writeUnified(req, rsp, unifiedFailure(apimodel.Code_ConfigUnavailable, "unified configuration is unavailable"), http.StatusServiceUnavailable)
			return
		}
		// 旧 Staffname 是客户端声明值，不能进入统一配置的认证与审计上下文。
		req.Request = req.Request.Clone(req.Request.Context())
		req.Request.Header.Del("Staffname")
		handler := httpcommon.Handler{Request: req, Response: rsp}
		result := invoke(handler.ParseHeaderContext(), parsed)
		if result == nil {
			result = unifiedFailure(apimodel.Code_ConfigUnavailable, "unified configuration is unavailable")
		}
		writeUnified(req, rsp, result, unifiedHTTPStatus(result.GetCode()))
	}
}

func unifiedFailure(code apimodel.Code, info string) *apimodel.Response {
	return &apimodel.Response{Code: uint32(code), Info: info}
}

func unifiedHTTPStatus(code uint32) int {
	switch apimodel.Code(code) {
	case apimodel.Code_ExecuteSuccess:
		return http.StatusOK
	case apimodel.Code_InvalidParameter, apimodel.Code_ClientAPINotOpen:
		return http.StatusBadRequest
	case apimodel.Code_DataConflict, apimodel.Code_ConfigCursorReset:
		return http.StatusConflict
	case apimodel.Code_NotFoundResource:
		return http.StatusNotFound
	case apimodel.Code_Unauthorized, apimodel.Code_NotAllowedAccess:
		return http.StatusUnauthorized
	case apimodel.Code_AuthTokenForbidden, apimodel.Code_OperationRoleForbidden:
		return http.StatusForbidden
	case apimodel.Code_ConfigUnavailable:
		return http.StatusServiceUnavailable
	case apimodel.Code_ConfigRenderFailed:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

func writeUnified(req *restful.Request, rsp *restful.Response, result *apimodel.Response, status int) {
	payload, err := protojson.Marshal(result)
	if err != nil {
		payload, _ = protojson.Marshal(unifiedFailure(apimodel.Code_ConfigUnavailable, "configuration response is unavailable"))
		status = http.StatusServiceUnavailable
	}
	rsp.Header().Set("Content-Type", restful.MIME_JSON)
	rsp.Header().Set(types.HeaderRequestId, req.HeaderParameter(types.HeaderRequestId))
	rsp.WriteHeader(status)
	_, _ = rsp.Write(payload)
}
