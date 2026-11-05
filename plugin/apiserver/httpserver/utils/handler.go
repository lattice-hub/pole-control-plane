/**
 * Tencent is pleased to support the open source community by making Polaris available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Clause License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Clause
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	restful "github.com/emicklei/go-restful/v3"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/pkg/types"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	commonlog "github.com/pole-io/pole-server/pkg/common/log"
	"github.com/pole-io/pole-server/pkg/common/utils"
	"github.com/pole-io/pole-server/plugin/apiserver/httpserver/i18n"
)

var (
	convert          MessageToCache
	protoCache       Cache
	enableProtoCache = false
	accesslog        = commonlog.GetScopeOrDefaultByName(commonlog.APIServerLoggerName)
)

// Handler HTTP请求/回复处理器
type Handler struct {
	Request  *restful.Request
	Response *restful.Response
}

// Parse 解析请求
func (h *Handler) BindJSON(v any) (context.Context, error) {
	requestID := h.Request.HeaderParameter(types.HeaderRequestId)
	if err := h.Request.ReadEntity(v); err != nil {
		accesslog.Error(err.Error(), utils.ZapRequestID(requestID))
		return nil, err
	}
	return h.ParseHeaderContext(), nil
}

// ParseArray 解析PB数组对象
func (h *Handler) ParseArray(createMessage func() proto.Message) (context.Context, error) {
	jsonDecoder := json.NewDecoder(h.Request.Request.Body)
	return h.parseArray(createMessage, jsonDecoder)
}

// ParseJSONArray 解析普通JSON数组对象（非proto.Message）
// 用法: ParseJSONArray(func() any { return &MyStruct{} })
func (h *Handler) ParseJSONArray(createMessage func() any) (context.Context, error) {
	requestID := h.Request.HeaderParameter(types.HeaderRequestId)
	jsonDecoder := json.NewDecoder(h.Request.Request.Body)

	// read open bracket
	if _, err := jsonDecoder.Token(); err != nil {
		accesslog.Error(err.Error(), utils.ZapRequestID(requestID))
		return nil, err
	}
	for jsonDecoder.More() {
		msg := createMessage()
		if err := jsonDecoder.Decode(msg); err != nil {
			accesslog.Error(err.Error(), utils.ZapRequestID(requestID))
			return nil, err
		}
	}
	return h.ParseHeaderContext(), nil
}

// ParseArrayByText 通过字符串解析PB数组对象
func (h *Handler) ParseArrayByText(createMessage func() proto.Message, text string) (context.Context, error) {
	jsonDecoder := json.NewDecoder(bytes.NewBuffer([]byte(text)))
	return h.parseArray(createMessage, jsonDecoder)
}

func (h *Handler) parseArray(createMessage func() proto.Message, jsonDecoder *json.Decoder) (context.Context, error) {
	requestID := h.Request.HeaderParameter(types.HeaderRequestId)
	// read open bracket
	if _, err := jsonDecoder.Token(); err != nil {
		accesslog.Error(err.Error(), utils.ZapRequestID(requestID))
		return nil, err
	}
	for jsonDecoder.More() {
		protoMessage := createMessage()
		if err := UnmarshalNext(jsonDecoder, protoMessage); err != nil {
			accesslog.Error(err.Error(), utils.ZapRequestID(requestID))
			return nil, err
		}
	}
	return h.ParseHeaderContext(), nil
}

// Parse 解析请求
func (h *Handler) Parse(message proto.Message) (context.Context, error) {
	requestID := h.Request.HeaderParameter(types.HeaderRequestId)
	if err := Unmarshal(h.Request.Request.Body, message); err != nil {
		accesslog.Error(err.Error(), utils.ZapRequestID(requestID))
		return nil, err
	}
	return h.ParseHeaderContext(), nil
}

// ParseHeaderContext 将http请求header中携带的用户信息提取出来
func (h *Handler) ParseHeaderContext() context.Context {
	requestID := h.Request.HeaderParameter(types.HeaderRequestId)
	token := h.Request.HeaderParameter("Polaris-Token")
	authToken := h.Request.HeaderParameter(types.HeaderAuthorizationKey)

	ctx := h.Request.Request.Context()
	ctx = context.WithValue(ctx, types.StringContext(types.HeaderRequestId), requestID)
	ctx = types.AppendRequestHeader(ctx, h.Request.Request.Header)
	ctx = context.WithValue(ctx, types.ContextClientAddress, h.Request.Request.RemoteAddr)
	if token != "" {
		ctx = context.WithValue(ctx, types.StringContext("polaris-token"), token)
	}
	if authToken != "" {
		ctx = context.WithValue(ctx, types.ContextAuthTokenKey, authToken)
	}

	var operator string
	addrSlice := strings.Split(h.Request.Request.RemoteAddr, ":")
	if len(addrSlice) == 2 {
		operator = "HTTP:" + addrSlice[0]
	}
	if staffName := h.Request.HeaderParameter("Staffname"); staffName != "" {
		operator = staffName
	}
	ctx = context.WithValue(ctx, types.StringContext("operator"), operator)

	return ctx
}

// WriteHeader 仅返回Code
func (h *Handler) WriteHeader(polarisCode uint32, httpStatus int) {
	requestID := h.Request.HeaderParameter(utils.PolarisRequestID)
	h.Request.SetAttribute(utils.PolarisCode, polarisCode) // api统计的时候，用该code

	// 对于非200000的返回，补充实际的code到header中
	if polarisCode != api.ExecuteSuccess {
		h.Response.AddHeader(utils.PolarisCode, fmt.Sprintf("%d", polarisCode))
		h.Response.AddHeader(utils.PolarisMessage, api.Code2Info(polarisCode))
	}
	h.Response.AddHeader(types.HeaderRequestId, requestID)
	h.Response.WriteHeader(httpStatus)
}

// WriteHeaderAndProto 返回Code和Proto
func (h *Handler) WriteHeaderAndProto(obj api.ResponseMessage) {
	requestID := h.Request.HeaderParameter(utils.PolarisRequestID)
	h.Request.SetAttribute(utils.PolarisCode, obj.GetCode())
	status := api.CalcCode(obj)

	if status != http.StatusOK {
		accesslog.Error(h.Request.Request.RequestURI+" "+obj.String(), utils.ZapRequestID(requestID))
	}
	if code := obj.GetCode(); code != api.ExecuteSuccess {
		h.Response.AddHeader(utils.PolarisCode, fmt.Sprintf("%d", code))
		h.Response.AddHeader(utils.PolarisMessage, api.Code2Info(code))
	}
	h.Response.AddHeader(utils.PolarisRequestID, requestID)
	h.Response.WriteHeader(status)

	if err := h.handleResponse(h.i18nAction(obj)); err != nil {
		accesslog.Error(err.Error(), utils.ZapRequestID(requestID))
	}
}

// WriteHeaderAndProtoV2 返回Code和Proto
func (h *Handler) WriteHeaderAndProtoV2(obj api.ResponseMessageV2) {
	requestID := h.Request.HeaderParameter(utils.PolarisRequestID)
	h.Request.SetAttribute(utils.PolarisCode, obj.GetCode())
	status := api.CalcCodeV2(obj)

	if status != http.StatusOK {
		accesslog.Error(obj.String(), utils.ZapRequestID(requestID))
	}
	if code := obj.GetCode(); code != api.ExecuteSuccess {
		h.Response.AddHeader(utils.PolarisCode, fmt.Sprintf("%d", code))
		h.Response.AddHeader(utils.PolarisMessage, api.Code2Info(code))
	}

	h.Response.AddHeader(utils.PolarisRequestID, requestID)
	h.Response.WriteHeader(status)

	m := newJsonpbMarshaler()
	if err := m.Marshal(h.Response, obj); err != nil {
		accesslog.Error(err.Error(), utils.ZapRequestID(requestID))
	}
}

// HTTPResponse http答复简单封装
func HTTPResponse(req *restful.Request, rsp *restful.Response, code uint32) {
	handler := &Handler{
		Request:  req,
		Response: rsp,
	}
	resp := api.NewResponse(apimodel.Code(code))
	handler.WriteHeaderAndProto(resp)
}

// i18nAction 依据resp.code进行国际化resp.info信息
// 当与header中的信息不匹配时, 则使用原文, 后续通过新定义code的方式增量解决
// 当header的msg 与 resp.info一致时, 根据resp.code国际化信息
func (h *Handler) i18nAction(obj api.ResponseMessage) api.ResponseMessage {
	hMsg := h.Response.Header().Get(utils.PolarisMessage)
	info := obj.GetInfo()
	if hMsg != info {
		return obj
	}
	code := obj.GetCode()
	msg, err := i18n.Translate(
		code, h.Request.QueryParameter("lang"), h.Request.HeaderParameter("Accept-Language"))
	if msg == "" || err != nil {
		return obj
	}
	info = msg
	return obj
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

func newJsonpbMarshaler() jsonpb.Marshaler {
	return jsonpb.Marshaler{Indent: " ", EmitDefaults: true}
}

func (h *Handler) handleResponse(obj api.ResponseMessage) error {
	if !enableProtoCache {
		m := newJsonpbMarshaler()
		return m.Marshal(h.Response, obj)
	}
	cacheVal := convert(obj)
	if cacheVal == nil {
		m := newJsonpbMarshaler()
		return m.Marshal(h.Response, obj)
	}
	if saveVal := protoCache.Get(cacheVal.CacheType, cacheVal.Key); saveVal != nil {
		if len(saveVal.GetBuf()) > 0 {
			_, err := h.Response.Write(saveVal.GetBuf())
			return err
		}
		return nil
	}

	if err := cacheVal.Marshal(obj); err != nil {
		accesslog.Warn("[Api-http][ProtoCache] prepare message fail, direct send msg", zap.String("key", cacheVal.Key),
			zap.Error(err))
		m := newJsonpbMarshaler()
		return m.Marshal(h.Response, obj)
	}

	cacheVal, ok := protoCache.Put(cacheVal)
	if !ok || cacheVal == nil {
		accesslog.Warn("[Api-http][ProtoCache] put cache ignore", zap.String("key", cacheVal.Key),
			zap.String("cacheType", cacheVal.CacheType))
		m := newJsonpbMarshaler()
		return m.Marshal(h.Response, obj)
	}
	if len(cacheVal.GetBuf()) > 0 {
		_, err := h.Response.Write(cacheVal.GetBuf())
		return err
	}
	return nil
}

func InitProtoCache(option map[string]interface{}, cacheTypes []string, discoverCacheConvert MessageToCache) {
	enableProtoCache = true
	cache, err := NewCache(option, cacheTypes)
	if err != nil {
		accesslog.Warn("[Api-http][Discover] new protobuf cache", zap.Error(err))
	}
	if cache != nil {
		protoCache = cache
		convert = discoverCacheConvert
	}
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

// WriteHeaderAndProto 返回Code和Proto
func (h *Handler) WriteHeaderAndData(obj *types.CommonResponse) {
	requestID := h.Request.HeaderParameter(utils.PolarisRequestID)
	status := api.CalcCodeCommon(obj)

	if status != http.StatusOK {
		accesslog.Error(h.Request.Request.RequestURI+" "+fmt.Sprintf("%d", status), utils.ZapRequestID(requestID))
	}
	if code := obj.Code; code != api.ExecuteSuccess {
		h.Response.AddHeader(utils.PolarisCode, fmt.Sprintf("%d", code))
		h.Response.AddHeader(utils.PolarisMessage, api.Code2Info(code))
	}
	h.Response.AddHeader(utils.PolarisRequestID, requestID)

	if err := h.Response.WriteHeaderAndJson(status, obj, restful.MIME_JSON); err != nil {
		accesslog.Error(err.Error(), utils.ZapRequestID(requestID))
	}
}
