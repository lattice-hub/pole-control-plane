package httputils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	restful "github.com/emicklei/go-restful/v3"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"

	"github.com/pole-io/pole-server/apis/pkg/types"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	commonlog "github.com/pole-io/pole-server/pkg/common/log"
	"github.com/pole-io/pole-server/pkg/common/utils"
)

var (
	accesslog = commonlog.GetScopeOrDefaultByName(commonlog.APIServerLoggerName)
)

// Handler HTTP请求/回复处理器
type Handler struct {
	Request  *restful.Request
	Response *restful.Response
}

func HTTPRawResponse(rsp *restful.Response, s int, header map[string]string, data []byte) {
	// 返回正常的数据
	for k, v := range header {
		rsp.Header().Set(k, v)
	}
	rsp.WriteHeader(s)
	_, _ = rsp.Write(data)
}

// HTTPResponse http答复简单封装
func HTTPResponse(req *restful.Request, rsp *restful.Response, d api.Rsp) {
	handler := &Handler{
		Request:  req,
		Response: rsp,
	}
	handler.WriteHeaderAndData(d)
}

// Parse 解析请求
func (h *Handler) BindJSON(v interface{}) (context.Context, error) {
	requestID := h.Request.HeaderParameter(types.HeaderRequestId)
	if err := h.Request.ReadEntity(v); err != nil {
		accesslog.Error(err.Error(), utils.ZapRequestID(requestID))
		return nil, err
	}
	return ParseHeaderContext(h.Request, h.Response), nil
}

// ParseHeaderContext 将http请求header中携带的用户信息提取出来
func ParseHeaderContext(req *restful.Request, rsp *restful.Response) context.Context {
	requestID := req.HeaderParameter(types.HeaderRequestId)
	authToken := req.HeaderParameter(types.HeaderAuthorizationKey)

	ctx := req.Request.Context()
	ctx = context.WithValue(ctx, types.StringContext(types.HeaderRequestId), requestID)
	ctx = types.AppendRequestHeader(ctx, req.Request.Header)
	ctx = context.WithValue(ctx, types.ContextClientAddress, req.Request.RemoteAddr)
	if authToken != "" {
		ctx = context.WithValue(ctx, types.ContextAuthTokenKey, authToken)
	}

	var operator string
	addrSlice := strings.Split(req.Request.RemoteAddr, ":")
	if len(addrSlice) == 2 {
		operator = "HTTP:" + addrSlice[0]
	}
	if staffName := req.HeaderParameter("Staffname"); staffName != "" {
		operator = staffName
	}
	ctx = context.WithValue(ctx, types.StringContext("operator"), operator)

	return ctx
}

// ParseQueryParams 解析并获取HTTP的query params
func ParseQueryParams(req *restful.Request) map[string]string {
	queryParams := make(map[string]string)
	for key, value := range req.Request.URL.Query() {
		if len(value) > 0 {
			if key == "keys" || key == "values" {
				queryParams[key] = strings.Join(value, ",")
			} else {
				queryParams[key] = value[0] // 暂时默认只支持一个查询
			}
		}
	}
	return queryParams
}

// ParseJsonBody parse http body as json object
func ParseJsonBody(req *restful.Request, value interface{}) error {
	body, err := io.ReadAll(req.Request.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, value); err != nil {
		return err
	}
	return nil
}

func MarshalPBJson(pb proto.Message) (string, error) {
	m := jsonpb.Marshaler{Indent: " ", EmitDefaults: false}
	// Marshal the message to JSON
	jsonStr, err := m.MarshalToString(pb)
	if err != nil {
		return "", err
	}
	return jsonStr, nil
}

func MarshalPBJsonToMap(pb proto.Message) map[string]interface{} {
	m := jsonpb.Marshaler{Indent: " ", EmitDefaults: false}
	// Marshal the message to JSON
	jsonStr, _ := m.MarshalToString(pb)
	r := make(map[string]interface{})
	_ = json.Unmarshal([]byte(jsonStr), &r)
	return r
}

func UnmarshalArray[T proto.Message](decoder *json.Decoder, m func() T) ([]T, error) {
	// read open bracket
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	var messages []T
	for decoder.More() {
		protoMessage := m()
		if err := UnmarshalNext(decoder, protoMessage); err != nil {
			return nil, err
		}
		messages = append(messages, protoMessage)
	}
	return messages, nil
}

func UnmarshalNext(j *json.Decoder, m proto.Message) error {
	var jsonpbMarshaler = jsonpb.Unmarshaler{AllowUnknownFields: true}
	return jsonpbMarshaler.UnmarshalNext(j, m)
}

func Unmarshal(j io.Reader, m proto.Message) error {
	var jsonpbMarshaler = jsonpb.Unmarshaler{AllowUnknownFields: true}
	return jsonpbMarshaler.Unmarshal(j, m)
}

type commRsp interface {
	GetCode() *uint32
}

// WriteHeaderAndProto 返回Code和Proto
func (h *Handler) WriteHeaderAndData(obj api.Rsp) {
	requestID := h.Request.HeaderParameter(utils.PolarisRequestID)
	status := api.CalcCodeCommon(obj)

	if status != http.StatusOK {
		accesslog.Error(h.Request.Request.RequestURI+" "+fmt.Sprintf("%d", status), utils.ZapRequestID(requestID))
	}
	if code := obj.GetCode(); code != api.ExecuteSuccess {
		h.Response.AddHeader(utils.PolarisCode, fmt.Sprintf("%d", code))
		h.Response.AddHeader(utils.PolarisMessage, api.Code2Info(code))
	}
	h.Response.AddHeader(utils.PolarisRequestID, requestID)

	if err := h.Response.WriteHeaderAndJson(status, obj, restful.MIME_JSON); err != nil {
		accesslog.Error(err.Error(), utils.ZapRequestID(requestID))
	}
}
