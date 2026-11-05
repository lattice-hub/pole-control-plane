package aimcp

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"google.golang.org/protobuf/encoding/protojson"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
)

type configInspector interface {
	GetState(context.Context, *configv1.GetStateRequest) *apimodel.Response
}

// addToolsConfigState 只暴露统一配置管理视图；不渲染、发布或绕过逐请求权限检查。
func (h *HTTPServer) addToolsConfigState(mcpServer *server.MCPServer) {
	mcpServer.AddTool(mcp.NewTool("get_config_state",
		mcp.WithDescription("读取环境内统一配置的草稿、发布摘要和生效版本，服务端不执行模板渲染"),
		mcp.WithString("environment", mcp.Required()),
		mcp.WithString("group", mcp.Required()),
		mcp.WithString("name", mcp.Required())),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			arguments := request.Params.Arguments
			key := &configv1.ConfigKey{}
			for name, output := range map[string]*string{"environment": &key.Environment, "group": &key.Group, "name": &key.Name} {
				value, ok := arguments[name].(string)
				if !ok || strings.TrimSpace(value) == "" {
					return mcp.NewToolResultError("environment, group and name are required strings"), nil
				}
				*output = value
			}
			response := h.configServer.GetState(ctx, &configv1.GetStateRequest{Key: key})
			if response.GetCode() != uint32(apimodel.Code_ExecuteSuccess) {
				return mcp.NewToolResultError(response.GetInfo()), nil
			}
			payload, err := protojson.Marshal(response)
			if err != nil {
				return mcp.NewToolResultError("configuration response encoding failed"), nil
			}
			return mcp.NewToolResultText(string(payload)), nil
		})
}
