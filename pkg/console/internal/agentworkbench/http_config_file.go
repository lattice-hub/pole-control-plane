package agentworkbench

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
)

const executeSuccessCode uint32 = uint32(apimodel.Code_ExecuteSuccess)

const downstreamFailureCode uint32 = 502001

const maxConfigResponseBytes = 4 << 20

type UpstreamError struct {
	Code      uint32
	Info      string
	RequestID string
	Status    int
}

func (e *UpstreamError) Error() string {
	if e.RequestID == "" {
		return fmt.Sprintf("pole-server request failed: code=%d info=%s", e.Code, e.Info)
	}
	return fmt.Sprintf("pole-server request failed: code=%d info=%s requestId=%s", e.Code, e.Info, e.RequestID)
}

func newDownstreamFailure(info, requestID string) *UpstreamError {
	return &UpstreamError{Code: downstreamFailureCode, Info: info, RequestID: requestID, Status: http.StatusBadGateway}
}

type HTTPConfigFilePort struct {
	baseURL string
	client  *http.Client
	timeout atomic.Int64
}

func NewHTTPConfigFilePort(address string, client *http.Client) *HTTPConfigFilePort {
	if client == nil {
		client = http.DefaultClient
	}
	baseURL := strings.TrimRight(address, "/")
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	port := &HTTPConfigFilePort{baseURL: baseURL, client: client}
	port.timeout.Store(int64(10 * time.Second))
	return port
}

func (p *HTTPConfigFilePort) SetTimeout(timeout time.Duration) {
	if timeout > 0 {
		p.timeout.Store(int64(timeout))
	}
}

func (p *HTTPConfigFilePort) requestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, time.Duration(p.timeout.Load()))
}

// GetConfigFile 从统一管理状态读取完整草稿；Workbench 只展示可安全编辑的 UTF-8 正文。
func (p *HTTPConfigFilePort) GetConfigFile(ctx context.Context, actor Actor, ref ResourceRef) (ConfigFile, string, error) {
	key := &configv1.ConfigKey{Environment: ref.Namespace, Group: ref.Group, Name: ref.Name}
	state, requestID, err := p.post(ctx, actor, "/config/v1/state", &configv1.GetStateRequest{Key: key})
	if err != nil {
		return ConfigFile{}, requestID, err
	}
	draft := state.GetDraft()
	if !proto.Equal(state.GetKey(), key) || (draft != nil && !proto.Equal(draft.GetKey(), key)) {
		return ConfigFile{}, requestID, newDownstreamFailure("pole-server config state does not match requested key", requestID)
	}
	if draft == nil || draft.GetDefinition() == nil {
		return ConfigFile{}, requestID, ErrSensitiveResource
	}
	content := draft.GetDefinition().GetContent()
	if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return ConfigFile{}, requestID, ErrSensitiveResource
	}
	return ConfigFile{
		Namespace: ref.Namespace, Group: ref.Group, Name: ref.Name,
		Content: string(content), Format: draft.GetDefinition().GetFormat(),
		DraftRevision: draft.GetRevision(),
		Definition:    proto.Clone(draft.GetDefinition()).(*configv1.ConfigDefinition),
	}, requestID, nil
}

// UpdateConfigFile 仅替换正文并以读取时的草稿 revision 做服务端原子 CAS。
func (p *HTTPConfigFilePort) UpdateConfigFile(ctx context.Context, actor Actor, file ConfigFile) (string, error) {
	if file.Definition == nil || !utf8.ValidString(file.Content) || strings.IndexByte(file.Content, 0) >= 0 {
		return "", ErrSensitiveResource
	}
	definition := proto.Clone(file.Definition).(*configv1.ConfigDefinition)
	definition.Content = []byte(file.Content)
	key := &configv1.ConfigKey{Environment: file.Namespace, Group: file.Group, Name: file.Name}
	_, requestID, err := p.post(ctx, actor, "/config/v1/draft", &configv1.SaveDraftRequest{
		Key: key, Definition: definition, ExpectedDraftRevision: file.DraftRevision,
	})
	return requestID, err
}

// post 使用标准 protobuf JSON 编解码 Response 和 Any；失败不读取业务 data。
func (p *HTTPConfigFilePort) post(ctx context.Context, actor Actor, path string, request proto.Message) (*configv1.ConfigState, string, error) {
	ctx, cancel := p.requestContext(ctx)
	defer cancel()
	payload, err := protojson.Marshal(request)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	setActorHeaders(req, actor)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, "", newDownstreamFailure("request pole-server configuration: "+err.Error(), "")
	}
	defer resp.Body.Close()
	requestID := resp.Header.Get("X-Request-Id")
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxConfigResponseBytes+1))
	if err != nil {
		return nil, requestID, newDownstreamFailure("read pole-server configuration response: "+err.Error(), requestID)
	}
	if len(body) > maxConfigResponseBytes {
		return nil, requestID, newDownstreamFailure("pole-server configuration response exceeds size limit", requestID)
	}
	var envelope apimodel.Response
	if err := protojson.Unmarshal(body, &envelope); err != nil {
		return nil, requestID, newDownstreamFailure("decode pole-server configuration response: "+err.Error(), requestID)
	}
	statusSuccess := resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
	codeSuccess := envelope.GetCode() == executeSuccessCode
	if !statusSuccess && codeSuccess {
		return nil, requestID, newDownstreamFailure("pole-server configuration response has inconsistent status", requestID)
	}
	if !statusSuccess || !codeSuccess {
		return nil, requestID, &UpstreamError{Code: envelope.GetCode(), Info: envelope.GetInfo(), RequestID: requestID, Status: resp.StatusCode}
	}
	if envelope.GetData() == nil {
		return nil, requestID, newDownstreamFailure("pole-server configuration response has no data", requestID)
	}
	var state configv1.ConfigState
	if err := envelope.GetData().UnmarshalTo(&state); err != nil {
		return nil, requestID, newDownstreamFailure("pole-server configuration response has invalid state: "+err.Error(), requestID)
	}
	return &state, requestID, nil
}

func setActorHeaders(req *http.Request, actor Actor) {
	req.Header.Set("Authorization", actor.Token)
	if actor.RequestID != "" {
		req.Header.Set("X-Request-Id", actor.RequestID)
	}
}
