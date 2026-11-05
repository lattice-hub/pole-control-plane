package agentworkbench

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
)

func configWorkbenchKey() *configv1.ConfigKey {
	return &configv1.ConfigKey{Environment: "default", Group: "orders", Name: "application.yaml"}
}

func configWorkbenchDefinition() *configv1.ConfigDefinition {
	return &configv1.ConfigDefinition{
		Content: []byte("timeout: {{{timeout}}}\n"), Engine: "pole-mustache-v1", Format: "yaml",
		Parameters:    []*configv1.Parameter{{Name: "timeout", Type: configv1.ValueType_VALUE_TYPE_STRING, Required: true}},
		DefaultValues: map[string]*configv1.Scalar{"timeout": {Type: configv1.ValueType_VALUE_TYPE_STRING, Value: "3s"}},
		ValueRules:    []*configv1.ValueRule{{Id: "test", Priority: 1, Audience: &configv1.Audience{Labels: map[string]string{"env": "test"}}, Values: map[string]*configv1.Scalar{"timeout": {Type: configv1.ValueType_VALUE_TYPE_STRING, Value: "5s"}}}},
		Compatibility: &configv1.ProtocolCompatibility{Apollo: true},
	}
}

func writeConfigResponse(t *testing.T, w http.ResponseWriter, status int, code apimodel.Code, data proto.Message) {
	t.Helper()
	response := &apimodel.Response{Code: uint32(code)}
	if data != nil {
		packed, err := anypb.New(data)
		require.NoError(t, err)
		response.Data = packed
	}
	payload, err := protojson.Marshal(response)
	require.NoError(t, err)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Id", "upstream-request")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

func TestHTTPConfigFilePortReadsAndSavesFullDefinitionWithCAS(t *testing.T) {
	key := configWorkbenchKey()
	original := configWorkbenchDefinition()
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		require.Equal(t, "pole-token", r.Header.Get("Authorization"))
		require.Equal(t, "console-request", r.Header.Get("X-Request-Id"))
		require.Empty(t, r.Header.Get("X-Pole-User"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		switch r.URL.Path {
		case "/config/v1/state":
			require.Equal(t, http.MethodPost, r.Method)
			var request configv1.GetStateRequest
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, protojson.Unmarshal(body, &request))
			require.True(t, proto.Equal(key, request.GetKey()))
			writeConfigResponse(t, w, http.StatusOK, apimodel.Code_ExecuteSuccess, &configv1.ConfigState{
				Key: key, Draft: &configv1.ConfigDraft{Key: key, Definition: original, Revision: 7},
			})
		case "/config/v1/draft":
			require.Equal(t, http.MethodPost, r.Method)
			var request configv1.SaveDraftRequest
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, protojson.Unmarshal(body, &request))
			require.True(t, proto.Equal(key, request.GetKey()))
			require.Equal(t, uint64(7), request.GetExpectedDraftRevision())
			want := proto.Clone(original).(*configv1.ConfigDefinition)
			want.Content = []byte("timeout: {{{timeout}}} # reviewed\n")
			require.True(t, proto.Equal(want, request.GetDefinition()), "save must preserve engine, format, parameters, values, rules and compatibility")
			writeConfigResponse(t, w, http.StatusOK, apimodel.Code_ExecuteSuccess, &configv1.ConfigState{
				Key: key, Draft: &configv1.ConfigDraft{Key: key, Definition: want, Revision: 8},
			})
		default:
			t.Errorf("obsolete or unexpected upstream route: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	port := NewHTTPConfigFilePort(strings.TrimPrefix(server.URL, "http://"), server.Client())
	actor := Actor{UserID: "alice", Token: "pole-token", RequestID: "console-request"}
	file, requestID, err := port.GetConfigFile(context.Background(), actor, ResourceRef{
		Kind: "config.file", Namespace: key.Environment, Group: key.Group, Name: key.Name,
	})
	require.NoError(t, err)
	require.Equal(t, "upstream-request", requestID)
	require.Equal(t, string(original.Content), file.Content)
	require.Equal(t, uint64(7), file.DraftRevision)
	require.True(t, proto.Equal(original, file.Definition))

	file.Content = "timeout: {{{timeout}}} # reviewed\n"
	requestID, err = port.UpdateConfigFile(context.Background(), actor, file)
	require.NoError(t, err)
	require.Equal(t, "upstream-request", requestID)
	require.Equal(t, []string{"POST /config/v1/state", "POST /config/v1/draft"}, paths)
}

func TestHTTPConfigFilePortPreservesPublicUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeConfigResponse(t, w, http.StatusForbidden, apimodel.Code_OperationRoleForbidden, nil)
	}))
	defer server.Close()
	port := NewHTTPConfigFilePort(server.URL, server.Client())
	_, _, err := port.GetConfigFile(context.Background(), Actor{Token: "token"}, ResourceRef{Namespace: "default", Group: "orders", Name: "application.yaml"})
	var upstream *UpstreamError
	require.ErrorAs(t, err, &upstream)
	require.Equal(t, uint32(apimodel.Code_OperationRoleForbidden), upstream.Code)
	require.Equal(t, http.StatusForbidden, upstream.Status)
	require.Equal(t, "upstream-request", upstream.RequestID)
}

func TestHTTPConfigFilePortRejectsMalformedAndMissingData(t *testing.T) {
	for _, body := range [][]byte{[]byte("not-json"), []byte(`{"code":200000}`)} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Request-Id", "request-43")
			_, _ = w.Write(body)
		}))
		port := NewHTTPConfigFilePort(server.URL, server.Client())
		_, _, err := port.GetConfigFile(context.Background(), Actor{Token: "token"}, ResourceRef{Namespace: "default", Group: "orders", Name: "application.yaml"})
		var upstream *UpstreamError
		require.ErrorAs(t, err, &upstream)
		require.Equal(t, downstreamFailureCode, upstream.Code)
		require.Equal(t, http.StatusBadGateway, upstream.Status)
		server.Close()
	}
}

func TestHTTPConfigFilePortRejectsBinaryDraft(t *testing.T) {
	key := configWorkbenchKey()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeConfigResponse(t, w, http.StatusOK, apimodel.Code_ExecuteSuccess, &configv1.ConfigState{
			Key: key, Draft: &configv1.ConfigDraft{Key: key, Revision: 1, Definition: &configv1.ConfigDefinition{Content: []byte{0xff, 0xfe}, Engine: "literal"}},
		})
	}))
	defer server.Close()
	port := NewHTTPConfigFilePort(server.URL, server.Client())
	_, _, err := port.GetConfigFile(context.Background(), Actor{Token: "token"}, ResourceRef{Namespace: key.Environment, Group: key.Group, Name: key.Name})
	require.ErrorIs(t, err, ErrSensitiveResource)
}

func TestHTTPConfigFilePortRejectsStateWithoutEditableDraft(t *testing.T) {
	key := configWorkbenchKey()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeConfigResponse(t, w, http.StatusOK, apimodel.Code_ExecuteSuccess, &configv1.ConfigState{Key: key})
	}))
	defer server.Close()
	port := NewHTTPConfigFilePort(server.URL, server.Client())
	_, _, err := port.GetConfigFile(context.Background(), Actor{Token: "token"}, ResourceRef{Namespace: key.Environment, Group: key.Group, Name: key.Name})
	require.ErrorIs(t, err, ErrSensitiveResource)
}

func TestHTTPConfigFilePortDoesNotMistakeOldBatchResponseForSave(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":200000,"responses":[{"code":200000}]}`))
	}))
	defer server.Close()
	port := NewHTTPConfigFilePort(server.URL, server.Client())
	_, err := port.UpdateConfigFile(context.Background(), Actor{Token: "token"}, ConfigFile{
		Namespace: "default", Group: "orders", Name: "application.yaml", Content: "text",
		Definition: &configv1.ConfigDefinition{Engine: "literal"}, DraftRevision: 1,
	})
	var upstream *UpstreamError
	require.ErrorAs(t, err, &upstream)
	require.Equal(t, downstreamFailureCode, upstream.Code)
}

func TestWorkbenchConfirmCASRejectsWriteAfterBaselineRecheck(t *testing.T) {
	key := configWorkbenchKey()
	definition := configWorkbenchDefinition()
	var stateReads, draftWrites atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/config/v1/state":
			stateReads.Add(1)
			writeConfigResponse(t, w, http.StatusOK, apimodel.Code_ExecuteSuccess, &configv1.ConfigState{
				Key: key, Draft: &configv1.ConfigDraft{Key: key, Definition: definition, Revision: 7},
			})
		case "/config/v1/draft":
			draftWrites.Add(1)
			var save configv1.SaveDraftRequest
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, protojson.Unmarshal(body, &save))
			require.Equal(t, uint64(7), save.GetExpectedDraftRevision())
			// 模拟再次读取后、保存事务前另一个作者推进草稿版本。
			writeConfigResponse(t, w, http.StatusConflict, apimodel.Code_DataConflict, nil)
		default:
			t.Errorf("unexpected upstream route: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	workbench := New(NewHTTPConfigFilePort(server.URL, server.Client()), Options{NewID: func() string { return "proposal-1" }})
	actor := Actor{UserID: "alice", Token: "token"}
	proposal, err := workbench.PrepareConfigFile(context.Background(), actor, PrepareConfigFileRequest{
		Namespace: key.Environment, Group: key.Group, Name: key.Name, DesiredContent: "timeout: {{{timeout}}} # revised\n",
	})
	require.NoError(t, err)
	_, err = workbench.Confirm(context.Background(), actor, ConfirmRequest{
		ProposalID: proposal.ID, ProposalVersion: proposal.Version, PreviewHash: proposal.PreviewHash, IdempotencyKey: "confirm-1",
	})
	require.ErrorIs(t, err, ErrStalePreview)
	require.Equal(t, int32(2), stateReads.Load())
	require.Equal(t, int32(1), draftWrites.Load())
}
