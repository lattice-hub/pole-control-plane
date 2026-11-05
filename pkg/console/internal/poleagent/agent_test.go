package poleagent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pole-io/pole-server/pkg/console/internal/agentworkbench"
)

type modelFunc func(context.Context, ModelRequest) (ModelResponse, error)

func singleScope(namespace string) NamespaceScope {
	return NamespaceScope{Mode: NamespaceScopeSingle, Namespaces: []string{namespace}}
}

func namespaceTool(name string) ToolDefinition {
	return ToolDefinition{
		Name: name,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string"},
			},
		},
	}
}

func environmentTool(name string) ToolDefinition {
	return ToolDefinition{Name: name, InputSchema: map[string]any{"type": "object", "properties": map[string]any{"environment": map[string]any{"type": "string"}, "group": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"}}}}
}

func (f modelFunc) Complete(ctx context.Context, req ModelRequest) (ModelResponse, error) {
	return f(ctx, req)
}

type fakeToolPort struct {
	session *fakeToolSession
	openErr error
}

func (f *fakeToolPort) Open(context.Context, agentworkbench.Actor) (ToolSession, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	return f.session, nil
}

type fakeToolSession struct {
	tools              []ToolDefinition
	listErr            error
	result             ToolResult
	callErr            error
	callCount          int
	arguments          map[string]any
	businessNamespaces []string
	closed             bool
}

func (f *fakeToolSession) ListTools(context.Context) ([]ToolDefinition, error) {
	tools := append([]ToolDefinition(nil), f.tools...)
	if _, found := findTool(tools, namespaceDirectoryTool); !found {
		tools = append(tools, ToolDefinition{
			Name:        namespaceDirectoryTool,
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		})
	}
	return tools, f.listErr
}

func (f *fakeToolSession) CallTool(_ context.Context, name string, arguments map[string]any) (ToolResult, error) {
	if name == namespaceDirectoryTool {
		namespaces := f.businessNamespaces
		if len(namespaces) == 0 {
			namespaces = []string{"default", "development", "production"}
		}
		data := make([]map[string]any, 0, len(namespaces))
		for _, namespace := range namespaces {
			data = append(data, map[string]any{
				"name": namespace,
				"kind": "NAMESPACE_KIND_BUSINESS",
			})
		}
		encoded, _ := json.Marshal(map[string]any{"data": data})
		return ToolResult{Content: string(encoded)}, nil
	}
	f.callCount++
	f.arguments = arguments
	return f.result, f.callErr
}

func (f *fakeToolSession) Close() error {
	f.closed = true
	return nil
}

type fakeApprovalKernel struct {
	calls int
	input agentworkbench.PrepareConfigFileRequest
	err   error
}

func (f *fakeApprovalKernel) PrepareConfigFile(
	_ context.Context,
	_ agentworkbench.Actor,
	input agentworkbench.PrepareConfigFileRequest,
) (*agentworkbench.Proposal, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return nil, f.err
	}
	return &agentworkbench.Proposal{
		ID:        "proposal-1",
		Status:    agentworkbench.ProposalPreviewReady,
		Resource:  agentworkbench.ResourceRef{Kind: "config.file", Namespace: input.Namespace, Group: input.Group, Name: input.Name},
		ExpiresAt: time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC),
	}, nil
}

func TestAgentRunsModelThenPreparesPreviewWithoutConfirmOrPublishTool(t *testing.T) {
	var requests []ModelRequest
	model := modelFunc(func(_ context.Context, req ModelRequest) (ModelResponse, error) {
		requests = append(requests, req)
		if len(requests) == 1 {
			return ModelResponse{Message: Message{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{{
					ID:   "call-1",
					Name: PrepareConfigFileUpdateTool,
					Arguments: json.RawMessage(`{
						"namespace":"default",
						"group":"orders",
						"name":"app.yaml",
						"desiredContent":"timeout: 5s\n",
						"comment":"raise timeout"
					}`),
				}},
			}}, nil
		}
		return ModelResponse{
			Message:   Message{Role: RoleAssistant, Content: "临时视图已生成，请确认后再保存草稿。"},
			Model:     "gateway-model",
			RequestID: "model-request",
		}, nil
	})
	session := &fakeToolSession{tools: []ToolDefinition{environmentTool("get_config_state")}}
	kernel := &fakeApprovalKernel{}
	agent := readyAgent(model, &fakeToolPort{session: session}, kernel)

	result, err := agent.RunTurn(context.Background(), testActor(), TurnRequest{
		Message:        "把超时改成 5 秒",
		NamespaceScope: singleScope("default"),
		History: []ConversationMessage{
			{Role: RoleUser, Content: "我们处理 orders 配置"},
			{Role: RoleAssistant, Content: "好的"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "临时视图已生成，请确认后再保存草稿。", result.Message)
	require.NotNil(t, result.Proposal)
	require.Equal(t, "proposal-1", result.Proposal.ID)
	require.Equal(t, 1, kernel.calls)
	require.Equal(t, "timeout: 5s\n", kernel.input.DesiredContent)
	require.Len(t, requests, 2)
	require.Len(t, requests[0].Tools, 2)
	require.Empty(t, requests[1].Tools)
	for _, tool := range requests[0].Tools {
		require.NotContains(t, tool.Name, "confirm")
		require.NotContains(t, tool.Name, "publish")
	}
	require.True(t, session.closed)
}

func TestAgentRejectsMissingNamespaceScope(t *testing.T) {
	agent := readyAgent(modelFunc(func(context.Context, ModelRequest) (ModelResponse, error) {
		return ModelResponse{}, nil
	}), &fakeToolPort{session: &fakeToolSession{}}, &fakeApprovalKernel{})

	_, err := agent.RunTurn(context.Background(), testActor(), TurnRequest{Message: "查询配置"})
	var runtimeErr *RuntimeError
	require.ErrorAs(t, err, &runtimeErr)
	require.Equal(t, CategoryInvalidRequest, runtimeErr.Category)
	require.Equal(t, uint32(400007), runtimeErr.Code)
}

func TestAgentRejectsResourceOutsideNamespaceScope(t *testing.T) {
	agent := readyAgent(modelFunc(func(context.Context, ModelRequest) (ModelResponse, error) {
		return ModelResponse{}, nil
	}), &fakeToolPort{session: &fakeToolSession{}}, &fakeApprovalKernel{})

	_, err := agent.RunTurn(context.Background(), testActor(), TurnRequest{
		Message:        "查询配置",
		NamespaceScope: singleScope("development"),
		ResourceContext: &ResourceContext{
			Kind: "config.file", Namespace: "production", Group: "app", Name: "config.yaml",
		},
	})
	var runtimeErr *RuntimeError
	require.ErrorAs(t, err, &runtimeErr)
	require.Equal(t, CategoryInvalidRequest, runtimeErr.Category)
	require.Equal(t, uint32(400008), runtimeErr.Code)
}

func TestAgentRejectsInaccessibleNamespaceScope(t *testing.T) {
	session := &fakeToolSession{businessNamespaces: []string{"development"}}
	agent := readyAgent(modelFunc(func(context.Context, ModelRequest) (ModelResponse, error) {
		t.Fatal("model must not receive an unauthorized namespace scope")
		return ModelResponse{}, nil
	}), &fakeToolPort{session: session}, &fakeApprovalKernel{})

	_, err := agent.RunTurn(context.Background(), testActor(), TurnRequest{
		Message:        "查询生产环境",
		NamespaceScope: singleScope("production"),
	})

	var runtimeErr *RuntimeError
	require.ErrorAs(t, err, &runtimeErr)
	require.Equal(t, CategoryToolRejected, runtimeErr.Category)
	require.Equal(t, uint32(403009), runtimeErr.Code)
}

func TestAgentRejectsSystemNamespaceScope(t *testing.T) {
	session := &fakeToolSession{businessNamespaces: []string{"pole-system"}}
	agent := readyAgent(modelFunc(func(context.Context, ModelRequest) (ModelResponse, error) {
		t.Fatal("model must not receive a system namespace scope")
		return ModelResponse{}, nil
	}), &fakeToolPort{session: session}, &fakeApprovalKernel{})

	_, err := agent.RunTurn(context.Background(), testActor(), TurnRequest{
		Message:        "查询系统空间",
		NamespaceScope: singleScope("pole-system"),
	})

	var runtimeErr *RuntimeError
	require.ErrorAs(t, err, &runtimeErr)
	require.Equal(t, uint32(403009), runtimeErr.Code)
}

func TestAgentRejectsProposalInCrossEnvironmentScope(t *testing.T) {
	model := modelFunc(func(context.Context, ModelRequest) (ModelResponse, error) {
		return ModelResponse{Message: Message{
			Role: RoleAssistant,
			ToolCalls: []ToolCall{{
				ID:   "call-1",
				Name: PrepareConfigFileUpdateTool,
				Arguments: json.RawMessage(`{
					"namespace":"development",
					"group":"app",
					"name":"config.yaml",
					"desiredContent":"enabled: true"
				}`),
			}},
		}}, nil
	})
	kernel := &fakeApprovalKernel{}
	agent := readyAgent(model, &fakeToolPort{session: &fakeToolSession{}}, kernel)

	_, err := agent.RunTurn(context.Background(), testActor(), TurnRequest{
		Message: "比较后修改开发环境",
		NamespaceScope: NamespaceScope{
			Mode:       NamespaceScopeCrossEnvironment,
			Namespaces: []string{"development", "production"},
		},
	})
	var runtimeErr *RuntimeError
	require.ErrorAs(t, err, &runtimeErr)
	require.Equal(t, CategoryToolRejected, runtimeErr.Category)
	require.Equal(t, uint32(403008), runtimeErr.Code)
	require.Zero(t, kernel.calls)
}

func TestAgentRejectsWriteLikeRemoteMCPTool(t *testing.T) {
	modelCalls := 0
	model := modelFunc(func(context.Context, ModelRequest) (ModelResponse, error) {
		modelCalls++
		return ModelResponse{}, nil
	})
	session := &fakeToolSession{tools: []ToolDefinition{namespaceTool("publish_config_release")}}
	agent := readyAgent(model, &fakeToolPort{session: session}, &fakeApprovalKernel{})
	_, err := agent.RunTurn(context.Background(), testActor(), TurnRequest{
		Message: "发布配置", NamespaceScope: singleScope("default"),
	})
	var runtimeErr *RuntimeError
	require.ErrorAs(t, err, &runtimeErr)
	require.Equal(t, CategoryToolRejected, runtimeErr.Category)
	require.Zero(t, modelCalls)
}

func TestAgentBoundsToolLoop(t *testing.T) {
	call := 0
	model := modelFunc(func(context.Context, ModelRequest) (ModelResponse, error) {
		call++
		return ModelResponse{Message: Message{
			Role: RoleAssistant,
			ToolCalls: []ToolCall{{
				ID:        "call",
				Name:      "get_config_state",
				Arguments: json.RawMessage(`{}`),
			}},
		}}, nil
	})
	session := &fakeToolSession{
		tools:  []ToolDefinition{environmentTool("get_config_state")},
		result: ToolResult{Content: `{"namespaces":[]}`},
	}
	agent := New(Options{
		RuntimeMode:   "llm",
		PromptVersion: "v1",
		Model:         "test-model",
		MaxToolRounds: 2,
	}, model, &fakeToolPort{session: session}, &fakeApprovalKernel{})
	_, err := agent.RunTurn(context.Background(), testActor(), TurnRequest{
		Message: "一直查", NamespaceScope: singleScope("default"),
	})
	var runtimeErr *RuntimeError
	require.ErrorAs(t, err, &runtimeErr)
	require.Equal(t, CategoryToolLoopLimit, runtimeErr.Category)
	require.Equal(t, 2, call)
	require.Equal(t, 2, session.callCount)
}

func TestAgentMarksMCPBusinessErrorTraceFailed(t *testing.T) {
	step := 0
	model := modelFunc(func(context.Context, ModelRequest) (ModelResponse, error) {
		step++
		if step == 1 {
			return ModelResponse{Message: Message{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{{
					ID:        "call-1",
					Name:      "get_config_state",
					Arguments: json.RawMessage(`{}`),
				}},
			}}, nil
		}
		return ModelResponse{Message: Message{Role: RoleAssistant, Content: "查询失败，请检查权限。"}}, nil
	})
	session := &fakeToolSession{
		tools:  []ToolDefinition{environmentTool("get_config_state")},
		result: ToolResult{Content: "permission denied", IsError: true},
	}
	agent := readyAgent(model, &fakeToolPort{session: session}, &fakeApprovalKernel{})
	result, err := agent.RunTurn(context.Background(), testActor(), TurnRequest{
		Message: "查命名空间", NamespaceScope: singleScope("default"),
	})
	require.NoError(t, err)
	require.Len(t, result.Tools, 1)
	require.Equal(t, ToolTraceFailed, result.Tools[0].Status)
	require.True(t, result.Tools[0].IsError)
	require.Equal(t, "default", session.arguments["environment"])
	require.NotContains(t, session.arguments, "namespace")
}

func TestAgentRejectsUnknownProposalFields(t *testing.T) {
	model := modelFunc(func(context.Context, ModelRequest) (ModelResponse, error) {
		return ModelResponse{Message: Message{
			Role: RoleAssistant,
			ToolCalls: []ToolCall{{
				ID:   "call-1",
				Name: PrepareConfigFileUpdateTool,
				Arguments: json.RawMessage(`{
					"namespace":"default",
					"group":"orders",
					"name":"app.yaml",
					"desiredContent":"a: 2",
					"publish":true
				}`),
			}},
		}}, nil
	})
	kernel := &fakeApprovalKernel{}
	agent := readyAgent(model, &fakeToolPort{session: &fakeToolSession{}}, kernel)
	_, err := agent.RunTurn(context.Background(), testActor(), TurnRequest{
		Message: "修改并发布", NamespaceScope: singleScope("default"),
	})
	var runtimeErr *RuntimeError
	require.ErrorAs(t, err, &runtimeErr)
	require.Equal(t, CategoryValidationFailed, runtimeErr.Category)
	require.Zero(t, kernel.calls)
}

func TestAgentRuntimeProbesAuthenticatedMCPAndReturnsNoEndpointOrSecret(t *testing.T) {
	session := &fakeToolSession{tools: []ToolDefinition{{
		Name: "list_namespaces",
		InputSchema: map[string]any{
			"type": "object",
		},
	}}}
	agent := readyAgent(modelFunc(func(context.Context, ModelRequest) (ModelResponse, error) {
		return ModelResponse{}, errors.New("not used")
	}), &fakeToolPort{session: session}, &fakeApprovalKernel{})
	runtime := agent.Runtime(context.Background(), testActor())
	require.True(t, runtime.Ready)
	require.True(t, runtime.Configured)
	require.True(t, runtime.MCPConnected)
	require.Equal(t, []string{"list_namespaces"}, runtime.MCPTools)
	require.Equal(t, []string{"list_namespaces", PrepareConfigFileUpdateTool}, runtime.Tools)
	encoded, err := json.Marshal(runtime)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "apiKey")
	require.NotContains(t, string(encoded), "baseURL")
	require.True(t, session.closed)
}

func TestAgentRuntimeReportsSafeProbeFailure(t *testing.T) {
	agent := readyAgent(modelFunc(func(context.Context, ModelRequest) (ModelResponse, error) {
		return ModelResponse{}, nil
	}), &fakeToolPort{openErr: errors.New("http://secret-host?token=secret")}, &fakeApprovalKernel{})
	runtime := agent.Runtime(context.Background(), testActor())
	require.False(t, runtime.Ready)
	require.True(t, runtime.Configured)
	require.False(t, runtime.MCPConnected)
	require.Empty(t, runtime.MCPTools)
	require.Empty(t, runtime.Tools)
	encoded, err := json.Marshal(runtime)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"mcpTools":[]`)
	require.Contains(t, string(encoded), `"tools":[]`)
	require.Equal(t, "Pole MCP connection failed", runtime.Reason)
	require.NotContains(t, runtime.Reason, "secret")
}

func TestAgentStatusExplainsIncompleteRuntime(t *testing.T) {
	agent := New(Options{RuntimeMode: "deterministic"}, nil, nil, nil)
	status := agent.Status()
	require.False(t, status.Ready)
	require.Contains(t, status.Missing, "agent.runtimeMode=llm")
	require.Contains(t, status.Missing, "agent.model")
	require.Contains(t, status.Missing, "agent.mcp")
	require.Contains(t, status.Missing, "agent.approvalKernel")
}

func readyAgent(model ModelPort, tools ToolPort, kernel ChangeApprovalKernel) *Agent {
	return New(Options{
		ID:            "pole-control-plane",
		RuntimeMode:   "llm",
		PromptVersion: "v1",
		Model:         "test-model",
	}, model, tools, kernel)
}

func testActor() agentworkbench.Actor {
	return agentworkbench.Actor{UserID: "admin", Token: "pole-token", RequestID: "console-request"}
}

func TestAgentConfigStateScopeReachesActualToolCall(t *testing.T) {
	crossScope := NamespaceScope{Mode: NamespaceScopeCrossEnvironment, Namespaces: []string{"development", "production"}}
	for _, test := range []struct {
		name            string
		scope           NamespaceScope
		arguments       string
		wantEnvironment string
		wantError       string
	}{
		{"single-injection", singleScope("development"), `{"group":"app","name":"settings"}`, "development", ""},
		{"outside-single", singleScope("development"), `{"environment":"production","group":"app","name":"settings"}`, "", "outside the turn scope"},
		{"cross-needs-selection", crossScope, `{"group":"app","name":"settings"}`, "", "must select one environment"},
		{"cross-explicit-selection", crossScope, `{"environment":"production","group":"app","name":"settings"}`, "production", ""},
		{"cross-rejects-unselected", crossScope, `{"environment":"default","group":"app","name":"settings"}`, "", "outside the turn scope"},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			model := modelFunc(func(_ context.Context, request ModelRequest) (ModelResponse, error) {
				calls++
				if calls == 1 {
					tool, available := findTool(request.Tools, "get_config_state")
					require.True(t, available)
					require.Equal(t, "environment", toolScopeArgument(tool))
					return ModelResponse{Message: Message{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "read-config", Name: "get_config_state", Arguments: json.RawMessage(test.arguments)}}}}, nil
				}
				return ModelResponse{Message: Message{Role: RoleAssistant, Content: "已读取配置状态。"}}, nil
			})
			session := &fakeToolSession{tools: []ToolDefinition{environmentTool("get_config_state")}, result: ToolResult{Content: `{"code":200000}`}}
			agent := readyAgent(model, &fakeToolPort{session: session}, &fakeApprovalKernel{})
			result, err := agent.RunTurn(context.Background(), testActor(), TurnRequest{Message: "读取配置状态", NamespaceScope: test.scope})
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
				require.Zero(t, session.callCount)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, session.callCount)
			require.Equal(t, test.wantEnvironment, session.arguments["environment"])
			require.NotContains(t, session.arguments, "namespace")
			require.Len(t, result.Tools, 1)
			var traceArguments map[string]any
			require.NoError(t, json.Unmarshal(result.Tools[0].Arguments, &traceArguments))
			require.Equal(t, session.arguments, traceArguments)
		})
	}
}

func TestAgentConfigStateValidatesEntireRequestedScope(t *testing.T) {
	session := &fakeToolSession{tools: []ToolDefinition{environmentTool("get_config_state")}, businessNamespaces: []string{"development"}}
	agent := readyAgent(modelFunc(func(context.Context, ModelRequest) (ModelResponse, error) {
		t.Fatal("model must not receive a partially authorized cross-environment scope")
		return ModelResponse{}, nil
	}), &fakeToolPort{session: session}, &fakeApprovalKernel{})
	_, err := agent.RunTurn(context.Background(), testActor(), TurnRequest{Message: "读取开发环境配置", NamespaceScope: NamespaceScope{Mode: NamespaceScopeCrossEnvironment, Namespaces: []string{"development", "production"}}})
	var runtimeErr *RuntimeError
	require.ErrorAs(t, err, &runtimeErr)
	require.Equal(t, uint32(403009), runtimeErr.Code)
	require.Zero(t, session.callCount)
}
