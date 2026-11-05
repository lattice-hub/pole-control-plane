import { apiRequest, getApiRequest, putApiRequest } from 'utils/request';
import { BaseURL } from './types';
import {
  AIEnvironmentBinding,
  AIResourceDefinition,
  bindAIResourceDefinition,
  createAIResourceDefinition,
  deleteAIResourceDefinition,
  describeAIEnvironmentBinding,
  describeAIEnvironmentBindings,
  describeAIResourceDefinitions,
} from './ai_definition';

const MCPDefinitionURL = '/ai/mcp/v1/definitions';

export interface MCPServer {
  id?: string;
  name: string;
  namespace: string;
  ports?: string;
  business?: string;
  department?: string;
  description?: string;
  revision?: string;
  flag?: number;
  reference?: string;
  protocol?: string;
  ctime?: string;
  mtime?: string;
  export_to?: string;
  backend_type?: string;
  backend_service_namespace?: string;
  backend_service_name?: string;
  backend_address?: string;
}

export interface MCPServerTool {
  id?: string;
  mcp_server_id?: string;
  name: string;
  description?: string;
  input_schema?: string;
  output_schema?: string;
  annotations?: string;
  flag?: number;
  ctime?: string;
  mtime?: string;
}

export type MCPEnvironmentBinding = AIEnvironmentBinding<'mcp_server'>;
export type MCPResourceDefinition = AIResourceDefinition;

export interface DescribeMCPServersRequest {
  offset: number;
  limit: number;
  name?: string;
  namespace?: string;
  business?: string;
  department?: string;
  protocol?: string;
  backend_type?: string;
  backend_service_namespace?: string;
  backend_service_name?: string;
}

export interface DescribeMCPServersResponse {
  amount: number;
  size: number;
  data?: MCPServer[];
}

export interface DescribeMCPServerToolsRequest {
  offset: number;
  limit: number;
  server_id?: string;
  server_name?: string;
  server_namespace?: string;
}

export interface DescribeMCPServerToolsResponse {
  amount: number;
  size: number;
  data?: MCPServerTool[];
}

function normalizeMCPObject<T extends Record<string, any>>(item: T): T {
  if (!item) return item;
  const { '@type': _typeUrl, ...rest } = item;
  return rest as T;
}

export async function describeMCPServers(params: DescribeMCPServersRequest) {
  const res = await getApiRequest<DescribeMCPServersResponse>({
    action: BaseURL.MCP_SERVER,
    data: params,
  });

  const list = (res.data ?? []).map((item) => normalizeMCPObject(item));
  return {
    list,
    totalCount: res.amount ?? list.length,
  };
}

export async function describeAllMCPServers() {
  const { list } = await describeMCPServers({ offset: 0, limit: 10000 });
  return list;
}

export async function createMCPServers(params: MCPServer[]) {
  return apiRequest({
    action: BaseURL.MCP_SERVER,
    data: params,
  });
}

export async function modifyMCPServers(params: MCPServer[]) {
  return putApiRequest({
    action: BaseURL.MCP_SERVER,
    data: params,
  });
}

export async function deleteMCPServers(ids: string[]) {
  return apiRequest({
    action: `${BaseURL.MCP_SERVER}/delete`,
    data: { server_ids: ids },
  });
}

export async function describeMCPServerTools(params: DescribeMCPServerToolsRequest) {
  const res = await getApiRequest<DescribeMCPServerToolsResponse>({
    action: BaseURL.MCP_SERVER_TOOL,
    data: params,
  });

  const list = (res.data ?? []).map((item) => normalizeMCPObject(item));
  return {
    list,
    totalCount: res.amount ?? list.length,
  };
}

export async function describeMCPServerDefinitionBinding(resourceId: string) {
  return describeAIEnvironmentBinding<'mcp_server'>(MCPDefinitionURL, resourceId);
}

export async function describeMCPServerDefinitionEnvironments(definitionId: string) {
  return describeAIEnvironmentBindings<'mcp_server'>(MCPDefinitionURL, definitionId);
}

export async function describeMCPServerDefinitions(name?: string) {
  return describeAIResourceDefinitions(MCPDefinitionURL, name);
}

export async function createMCPServerDefinition(name: string) {
  return createAIResourceDefinition(MCPDefinitionURL, 'MCP Server', name);
}

export async function deleteMCPServerDefinition(definitionId: string) {
  return deleteAIResourceDefinition(MCPDefinitionURL, definitionId);
}

export async function bindMCPServerDefinition(definitionId: string, resourceId: string) {
  return bindAIResourceDefinition(MCPDefinitionURL, definitionId, resourceId);
}
