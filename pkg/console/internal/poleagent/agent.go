package poleagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/pole-io/pole-server/pkg/console/internal/agentworkbench"
)

const (
	PrepareConfigFileUpdateTool = "prepare_config_file_update"

	defaultMaxToolRounds        = 8
	defaultMaxToolCallsPerRound = 4
	defaultMaxHistoryMessages   = 40
	defaultMaxMessageBytes      = 64 << 10
	maxDesiredConfigBytes       = 1 << 20
)

type ChangeApprovalKernel interface {
	PrepareConfigFile(
		ctx context.Context,
		actor agentworkbench.Actor,
		req agentworkbench.PrepareConfigFileRequest,
	) (*agentworkbench.Proposal, error)
}

type Options struct {
	ID                   string
	RuntimeMode          string
	PromptVersion        string
	OperatorInstructions string
	Model                string
	MaxToolRounds        int
	MaxToolCallsPerRound int
	MaxHistoryMessages   int
	MaxMessageBytes      int
}

type RuntimeStatus struct {
	Ready                    bool     `json:"ready"`
	Mode                     string   `json:"mode"`
	AgentID                  string   `json:"agentId"`
	PromptVersion            string   `json:"promptVersion"`
	ModelConfigured          bool     `json:"modelConfigured"`
	MCPConfigured            bool     `json:"mcpConfigured"`
	ApprovalKernelConfigured bool     `json:"approvalKernelConfigured"`
	Missing                  []string `json:"missing,omitempty"`
}

type RuntimeInspection struct {
	Ready             bool     `json:"ready"`
	Configured        bool     `json:"configured"`
	Mode              string   `json:"mode"`
	AgentID           string   `json:"agentId"`
	Model             string   `json:"model,omitempty"`
	PromptVersion     string   `json:"promptVersion"`
	MCPConnected      bool     `json:"mcpConnected"`
	MCPTools          []string `json:"mcpTools"`
	Tools             []string `json:"tools"`
	Reason            string   `json:"reason,omitempty"`
	EffectiveRevision string   `json:"effectiveRevision,omitempty"`
}

type ConversationMessage struct {
	Role    MessageRole `json:"role"`
	Content string      `json:"content"`
}

type ResourceContext struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Group     string `json:"group,omitempty"`
	Name      string `json:"name,omitempty"`
}

type TurnRequest struct {
	Message         string                `json:"message"`
	History         []ConversationMessage `json:"history,omitempty"`
	NamespaceScope  NamespaceScope        `json:"namespaceScope"`
	ResourceContext *ResourceContext      `json:"resourceContext,omitempty"`
}

type ToolTraceStatus string

const (
	ToolTraceSuccess ToolTraceStatus = "success"
	ToolTraceFailed  ToolTraceStatus = "failed"
)

type ToolTrace struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	Status    ToolTraceStatus `json:"status"`
	Summary   string          `json:"summary"`
	IsError   bool            `json:"isError,omitempty"`
}

type TurnResult struct {
	Message   string                   `json:"message"`
	Tools     []ToolTrace              `json:"tools"`
	Proposal  *agentworkbench.Proposal `json:"proposal,omitempty"`
	Runtime   RuntimeInspection        `json:"runtime"`
	Model     string                   `json:"model,omitempty"`
	RequestID string                   `json:"requestId,omitempty"`
}

type Agent struct {
	options  Options
	model    ModelPort
	tools    ToolPort
	approval ChangeApprovalKernel
}

func New(options Options, model ModelPort, tools ToolPort, approval ChangeApprovalKernel) *Agent {
	if strings.TrimSpace(options.ID) == "" {
		options.ID = "pole-control-plane"
	}
	if strings.TrimSpace(options.RuntimeMode) == "" {
		options.RuntimeMode = "deterministic"
	}
	if strings.TrimSpace(options.PromptVersion) == "" {
		options.PromptVersion = "v1"
	}
	if options.MaxToolRounds <= 0 {
		options.MaxToolRounds = defaultMaxToolRounds
	}
	if options.MaxToolCallsPerRound <= 0 {
		options.MaxToolCallsPerRound = defaultMaxToolCallsPerRound
	}
	if options.MaxHistoryMessages <= 0 {
		options.MaxHistoryMessages = defaultMaxHistoryMessages
	}
	if options.MaxMessageBytes <= 0 {
		options.MaxMessageBytes = defaultMaxMessageBytes
	}
	return &Agent{options: options, model: model, tools: tools, approval: approval}
}

func (a *Agent) Status() RuntimeStatus {
	status := RuntimeStatus{
		Mode:                     a.options.RuntimeMode,
		AgentID:                  a.options.ID,
		PromptVersion:            normalizedPromptVersion(a.options.PromptVersion),
		ModelConfigured:          a.model != nil && strings.TrimSpace(a.options.Model) != "",
		MCPConfigured:            a.tools != nil,
		ApprovalKernelConfigured: a.approval != nil,
	}
	if a.options.RuntimeMode != "llm" {
		status.Missing = append(status.Missing, "agent.runtimeMode=llm")
	}
	if !status.ModelConfigured {
		status.Missing = append(status.Missing, "agent.model")
	}
	if !status.MCPConfigured {
		status.Missing = append(status.Missing, "agent.mcp")
	}
	if !status.ApprovalKernelConfigured {
		status.Missing = append(status.Missing, "agent.approvalKernel")
	}
	if _, err := BuildSystemPrompt(a.options.PromptVersion, a.options.OperatorInstructions); err != nil {
		status.Missing = append(status.Missing, "agent.systemPrompt")
	}
	status.Ready = len(status.Missing) == 0
	return status
}

// Runtime performs an authenticated MCP capability probe. It intentionally
// returns neither the LLM endpoint nor any credential-bearing configuration.
func (a *Agent) Runtime(ctx context.Context, actor agentworkbench.Actor) RuntimeInspection {
	status := a.Status()
	inspection := RuntimeInspection{
		Configured:    status.Ready,
		Mode:          status.Mode,
		AgentID:       status.AgentID,
		Model:         a.options.Model,
		PromptVersion: status.PromptVersion,
		MCPTools:      make([]string, 0),
		Tools:         make([]string, 0),
	}
	if !status.Ready {
		inspection.Reason = "Pole Agent runtime configuration is incomplete: " + strings.Join(status.Missing, ", ")
		return inspection
	}
	if ctx == nil {
		ctx = context.Background()
	}
	session, err := a.tools.Open(ctx, actor)
	if err != nil {
		inspection.Reason = safeRuntimeProbeReason(err, "Pole MCP connection failed")
		return inspection
	}
	defer session.Close()
	remoteTools, err := session.ListTools(ctx)
	if err != nil {
		inspection.Reason = safeRuntimeProbeReason(err, "Pole MCP tool discovery failed")
		return inspection
	}
	catalog, err := buildToolCatalog(remoteTools)
	if err != nil {
		inspection.Reason = safeRuntimeProbeReason(err, "Pole MCP tool catalog was rejected")
		return inspection
	}
	inspection.MCPConnected = true
	for _, tool := range remoteTools {
		inspection.MCPTools = append(inspection.MCPTools, tool.Name)
	}
	for _, tool := range catalog {
		inspection.Tools = append(inspection.Tools, tool.Name)
	}
	inspection.Ready = true
	return inspection
}

func (a *Agent) RunTurn(ctx context.Context, actor agentworkbench.Actor, req TurnRequest) (*TurnResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	status := a.Status()
	if !status.Ready {
		return nil, runtimeError(CategoryRuntimeUnavailable, 503001,
			"Pole Agent runtime is not ready: "+strings.Join(status.Missing, ", "), false, ErrRuntimeUnavailable)
	}
	messages, err := a.initialMessages(req)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(actor.UserID) == "" || strings.TrimSpace(actor.Token) == "" {
		return nil, runtimeError(CategoryInvalidRequest, 400001,
			"authenticated actor is required", false, ErrInvalidRequest)
	}

	toolSession, err := a.tools.Open(ctx, actor)
	if err != nil {
		return nil, err
	}
	defer toolSession.Close()
	remoteTools, err := toolSession.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateNamespaceScopeAccess(ctx, toolSession, remoteTools, req.NamespaceScope); err != nil {
		return nil, err
	}
	remoteTools = namespaceScopedRemoteTools(remoteTools)
	toolDefinitions, err := buildToolCatalog(remoteTools)
	if err != nil {
		return nil, err
	}

	result := &TurnResult{Runtime: a.runtimeInspection(remoteTools, toolDefinitions)}
	proposalPrepared := false
	for round := 0; round < a.options.MaxToolRounds; round++ {
		availableTools := toolDefinitions
		if proposalPrepared {
			// Once a preview exists, the model gets one response-only round. It
			// cannot chain another operation behind the user's confirmation.
			availableTools = nil
		}
		completion, err := a.model.Complete(ctx, ModelRequest{
			Model:    a.options.Model,
			Messages: messages,
			Tools:    availableTools,
		})
		if err != nil {
			return nil, err
		}
		result.Model = completion.Model
		result.RequestID = completion.RequestID
		assistant := completion.Message
		assistant.Role = RoleAssistant
		if len(assistant.ToolCalls) == 0 {
			if strings.TrimSpace(assistant.Content) == "" {
				return nil, runtimeError(CategoryValidationFailed, 422001,
					"model returned neither text nor tool calls", false, nil)
			}
			result.Message = assistant.Content
			return result, nil
		}
		if proposalPrepared {
			return nil, runtimeError(CategoryToolRejected, 403001,
				"model attempted another tool call after a preview was prepared", false, nil)
		}
		if len(assistant.ToolCalls) > a.options.MaxToolCallsPerRound {
			return nil, runtimeError(CategoryToolRejected, 403002,
				"model requested too many tools in one round", false, nil)
		}
		if len(assistant.ToolCalls) > 1 {
			for _, call := range assistant.ToolCalls {
				if call.Name == PrepareConfigFileUpdateTool {
					return nil, runtimeError(CategoryToolRejected, 403006,
						"config file proposal must be the only tool call in its round", false, nil)
				}
			}
		}
		messages = append(messages, assistant)
		for _, call := range assistant.ToolCalls {
			tool, exists := findTool(toolDefinitions, call.Name)
			if !exists {
				return nil, runtimeError(CategoryToolRejected, 403003,
					"model requested an unavailable tool", false, nil)
			}
			arguments, err := decodeToolArguments(call.Arguments)
			if err != nil {
				return nil, err
			}
			if err := enforceToolNamespace(req.NamespaceScope, tool, arguments); err != nil {
				return nil, runtimeError(CategoryToolRejected, 403007,
					err.Error(), false, err)
			}
			scopedTraceArguments, err := json.Marshal(arguments)
			if err != nil {
				return nil, runtimeError(CategoryValidationFailed, 422004,
					"encode scoped tool arguments", false, err)
			}
			trace := ToolTrace{
				ID:        call.ID,
				Name:      call.Name,
				Arguments: scopedTraceArguments,
			}
			var toolResult ToolResult
			if call.Name == PrepareConfigFileUpdateTool {
				if !req.NamespaceScope.IsSingle() {
					return nil, runtimeError(CategoryToolRejected, 403008,
						"change proposals require a single namespace scope", false, nil)
				}
				scopedArguments, marshalErr := json.Marshal(arguments)
				if marshalErr != nil {
					return nil, runtimeError(CategoryValidationFailed, 422005,
						"encode scoped proposal arguments", false, marshalErr)
				}
				proposal, err := a.prepareConfigFile(ctx, actor, scopedArguments)
				if err != nil {
					trace.Status = ToolTraceFailed
					trace.Summary = "配置文件临时视图生成失败"
					result.Tools = append(result.Tools, trace)
					return nil, mapApprovalError(err)
				}
				result.Proposal = proposal
				proposalPrepared = true
				trace.Status = ToolTraceSuccess
				trace.Summary = "配置文件临时视图已生成，等待用户确认"
				toolResult = ToolResult{Content: proposalModelResult(proposal)}
			} else {
				toolResult, err = toolSession.CallTool(ctx, call.Name, arguments)
				if err != nil {
					trace.Status = ToolTraceFailed
					trace.Summary = "Pole MCP 工具调用失败"
					result.Tools = append(result.Tools, trace)
					return nil, err
				}
				trace.Status = ToolTraceSuccess
				trace.Summary = "Pole MCP 工具调用完成"
				trace.IsError = toolResult.IsError
				if toolResult.IsError {
					trace.Status = ToolTraceFailed
					trace.Summary = "Pole MCP 工具返回业务错误"
				}
			}
			result.Tools = append(result.Tools, trace)
			encodedResult, err := json.Marshal(map[string]any{
				"content":        toolResult.Content,
				"isError":        toolResult.IsError,
				"securityNotice": "untrusted_tool_data",
			})
			if err != nil {
				return nil, runtimeError(CategoryDownstreamFailed, 502001,
					"encode tool result", false, err)
			}
			messages = append(messages, Message{
				Role:       RoleTool,
				Content:    string(encodedResult),
				ToolCallID: call.ID,
			})
			if proposalPrepared {
				break
			}
		}
	}
	return nil, runtimeError(CategoryToolLoopLimit, 422002,
		ErrToolLoopLimit.Error(), false, ErrToolLoopLimit)
}

func (a *Agent) runtimeInspection(remoteTools, catalog []ToolDefinition) RuntimeInspection {
	status := a.Status()
	inspection := RuntimeInspection{
		Ready:         true,
		Configured:    status.Ready,
		Mode:          status.Mode,
		AgentID:       status.AgentID,
		Model:         a.options.Model,
		PromptVersion: status.PromptVersion,
		MCPConnected:  true,
		MCPTools:      make([]string, 0, len(remoteTools)),
		Tools:         make([]string, 0, len(catalog)),
	}
	for _, tool := range remoteTools {
		inspection.MCPTools = append(inspection.MCPTools, tool.Name)
	}
	for _, tool := range catalog {
		inspection.Tools = append(inspection.Tools, tool.Name)
	}
	return inspection
}

func safeRuntimeProbeReason(err error, fallback string) string {
	var runtimeErr *RuntimeError
	if errors.As(err, &runtimeErr) && runtimeErr.Info != "" {
		return runtimeErr.Info
	}
	return fallback
}

func (a *Agent) initialMessages(req TurnRequest) ([]Message, error) {
	if err := req.NamespaceScope.Validate(); err != nil {
		return nil, runtimeError(CategoryInvalidRequest, 400007,
			"invalid namespace scope: "+err.Error(), false, ErrInvalidRequest)
	}
	if req.ResourceContext != nil &&
		!req.NamespaceScope.Contains(req.ResourceContext.Namespace) {
		return nil, runtimeError(CategoryInvalidRequest, 400008,
			"resource context namespace is outside the turn scope", false, ErrInvalidRequest)
	}
	current := strings.TrimSpace(req.Message)
	if current == "" || len(current) > a.options.MaxMessageBytes {
		return nil, runtimeError(CategoryInvalidRequest, 400002,
			"message is empty or exceeds its size limit", false, ErrInvalidRequest)
	}
	prompt, err := BuildSystemPrompt(a.options.PromptVersion, a.options.OperatorInstructions)
	if err != nil {
		return nil, err
	}
	history := req.History
	if len(history) > a.options.MaxHistoryMessages {
		history = history[len(history)-a.options.MaxHistoryMessages:]
	}
	messages := []Message{{Role: RoleSystem, Content: prompt}}
	scope, err := json.Marshal(req.NamespaceScope)
	if err != nil {
		return nil, runtimeError(CategoryInvalidRequest, 400009,
			"invalid namespace scope", false, err)
	}
	messages[0].Content += "\n\n<namespace_scope trust=\"server-validated\">\n" +
		string(scope) + "\n</namespace_scope>"
	totalBytes := len(current)
	for _, item := range history {
		if item.Role != RoleUser && item.Role != RoleAssistant {
			return nil, runtimeError(CategoryInvalidRequest, 400003,
				"history may contain only user and assistant messages", false, ErrInvalidRequest)
		}
		if len(item.Content) > a.options.MaxMessageBytes {
			return nil, runtimeError(CategoryInvalidRequest, 400004,
				"history message exceeds its size limit", false, ErrInvalidRequest)
		}
		totalBytes += len(item.Content)
		if totalBytes > a.options.MaxMessageBytes*a.options.MaxHistoryMessages {
			return nil, runtimeError(CategoryInvalidRequest, 400005,
				"conversation history exceeds its size limit", false, ErrInvalidRequest)
		}
		messages = append(messages, Message{Role: item.Role, Content: item.Content})
	}
	if req.ResourceContext != nil {
		resource, err := json.Marshal(req.ResourceContext)
		if err != nil {
			return nil, runtimeError(CategoryInvalidRequest, 400006,
				"invalid resource context", false, err)
		}
		current += "\n\n<resource_context trust=\"untrusted-reference\">\n" +
			string(resource) +
			"\n</resource_context>"
	}
	messages = append(messages, Message{Role: RoleUser, Content: current})
	return messages, nil
}

func buildToolCatalog(remote []ToolDefinition) ([]ToolDefinition, error) {
	tools := make([]ToolDefinition, 0, len(remote)+1)
	seen := map[string]struct{}{}
	for _, tool := range remote {
		name := strings.TrimSpace(tool.Name)
		if name == "" || isWriteLikeRemoteTool(name) {
			return nil, runtimeError(CategoryToolRejected, 403004,
				"Pole MCP exposed a non-read-only tool to the Agent runtime", false, nil)
		}
		if _, exists := seen[name]; exists || name == PrepareConfigFileUpdateTool {
			return nil, runtimeError(CategoryToolRejected, 403005,
				"Pole MCP tool name conflicts with the local approval tool", false, nil)
		}
		tool.Name = name
		tools = append(tools, tool)
		seen[name] = struct{}{}
	}
	tools = append(tools, ToolDefinition{
		Name:        PrepareConfigFileUpdateTool,
		Description: "读取指定配置文件的当前草稿并生成不可变临时视图。此工具不会修改或发布资源；生成后必须等待用户在页面显式确认。",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"namespace":      map[string]any{"type": "string", "description": "配置文件命名空间"},
				"group":          map[string]any{"type": "string", "description": "配置分组"},
				"name":           map[string]any{"type": "string", "description": "配置文件名"},
				"desiredContent": map[string]any{"type": "string", "description": "完整目标配置正文"},
				"comment":        map[string]any{"type": "string", "description": "可选变更说明"},
			},
			"required": []string{"namespace", "group", "name", "desiredContent"},
		},
	})
	return tools, nil
}

func findTool(tools []ToolDefinition, name string) (ToolDefinition, bool) {
	for _, tool := range tools {
		if tool.Name == name {
			return tool, true
		}
	}
	return ToolDefinition{}, false
}

func isWriteLikeRemoteTool(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, prefix := range []string{
		"create_", "update_", "delete_", "publish_", "release_", "confirm_",
		"write_", "save_", "apply_", "rollback_", "remove_", "set_", "patch_",
		"upsert_", "mutate_", "enable_", "disable_",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return lower == "confirm" || lower == "publish" || lower == "release"
}

func decodeToolArguments(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || !json.Valid(raw) {
		return nil, runtimeError(CategoryValidationFailed, 422003,
			"tool arguments are not valid JSON", false, nil)
	}
	var arguments map[string]any
	if err := json.Unmarshal(raw, &arguments); err != nil || arguments == nil {
		return nil, runtimeError(CategoryValidationFailed, 422004,
			"tool arguments must be a JSON object", false, err)
	}
	return arguments, nil
}

func (a *Agent) prepareConfigFile(
	ctx context.Context,
	actor agentworkbench.Actor,
	raw json.RawMessage,
) (*agentworkbench.Proposal, error) {
	var input agentworkbench.PrepareConfigFileRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, runtimeError(CategoryValidationFailed, 422005,
			"invalid config file proposal arguments", false, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, runtimeError(CategoryValidationFailed, 422006,
			"config file proposal arguments contain trailing data", false, err)
	}
	if len(input.DesiredContent) > maxDesiredConfigBytes {
		return nil, runtimeError(CategoryValidationFailed, 422007,
			"desired config content exceeds its size limit", false, nil)
	}
	return a.approval.PrepareConfigFile(ctx, actor, input)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values")
	}
	return err
}

func proposalModelResult(proposal *agentworkbench.Proposal) string {
	content, _ := json.Marshal(map[string]any{
		"proposalId": proposal.ID,
		"status":     proposal.Status,
		"resource":   proposal.Resource,
		"expiresAt":  proposal.ExpiresAt,
		"nextAction": "wait_for_user_confirmation",
	})
	return string(content)
}

func mapApprovalError(err error) error {
	if err == nil {
		return nil
	}
	var runtimeErr *RuntimeError
	if errors.As(err, &runtimeErr) {
		return runtimeErr
	}
	switch {
	case errors.Is(err, agentworkbench.ErrSensitiveResource):
		return runtimeError(CategoryNotDraftable, 422011,
			"encrypted config files cannot be changed by Pole Agent", false, err)
	case errors.Is(err, agentworkbench.ErrInvalidRequest),
		errors.Is(err, agentworkbench.ErrNoChanges):
		return runtimeError(CategoryValidationFailed, 422012,
			err.Error(), false, err)
	default:
		var upstream *agentworkbench.UpstreamError
		if errors.As(err, &upstream) {
			runtimeErr := runtimeError(CategoryDownstreamFailed, upstream.Code,
				upstream.Info, upstream.Status >= 500, err)
			runtimeErr.RequestID = upstream.RequestID
			return runtimeErr
		}
		return runtimeError(CategoryDownstreamFailed, 502011,
			"prepare config file proposal failed", false, err)
	}
}
