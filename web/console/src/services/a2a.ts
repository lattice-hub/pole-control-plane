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

const A2ADefinitionURL = '/ai/a2a/v1/definitions';

export interface A2AAgentInterface {
  id?: string;
  agent_id?: string;
  url: string;
  protocol_binding?: string;
  protocol_version?: string;
  tenant?: string;
  flag?: number;
  ctime?: string;
  mtime?: string;
}

export interface A2AAgentSkill {
  id?: string;
  agent_id?: string;
  skill_id?: string;
  name: string;
  description?: string;
  tags?: string[];
  examples?: string[];
  input_modes?: string[];
  output_modes?: string[];
  security_requirements_json?: string;
  flag?: number;
  ctime?: string;
  mtime?: string;
}

export interface A2AAgent {
  id?: string;
  name: string;
  namespace: string;
  visibility?: string;
  description?: string;
  version?: string;
  protocol_version?: string;
  provider_organization?: string;
  provider_url?: string;
  documentation_url?: string;
  icon_url?: string;
  business?: string;
  department?: string;
  backend_type?: string;
  backend_service_namespace?: string;
  backend_service_name?: string;
  backend_address?: string;
  preferred_interface_url?: string;
  preferred_protocol_binding?: string;
  preferred_protocol_version?: string;
  streaming?: boolean;
  push_notifications?: boolean;
  extended_agent_card?: boolean;
  raw_card_json?: string;
  source_type?: string;
  source_url?: string;
  last_fetch_status?: string;
  last_fetch_time?: string;
  metadata?: Record<string, string>;
  interfaces?: A2AAgentInterface[];
  skills?: A2AAgentSkill[];
  flag?: number;
  ctime?: string;
  mtime?: string;
}

export type A2AEnvironmentBinding = AIEnvironmentBinding<'a2a_agent'>;
export type A2AResourceDefinition = AIResourceDefinition;

export interface DescribeA2AAgentsRequest {
  offset: number;
  limit: number;
  name?: string;
  namespace?: string;
  business?: string;
  department?: string;
  protocol_binding?: string;
  skill_tag?: string;
  backend_type?: string;
  backend_service_namespace?: string;
  backend_service_name?: string;
  streaming?: boolean;
  push_notifications?: boolean;
}

export interface DescribeA2AAgentsResponse {
  amount: number;
  size: number;
  data?: A2AAgent[];
}

export interface DescribeA2AAgentSkillsRequest {
  agent_id?: string;
  agent_name?: string;
  agent_namespace?: string;
}

export interface DescribeA2AAgentSkillsResponse {
  amount?: number;
  size?: number;
  data?: A2AAgentSkill[];
}

function normalizeA2AObject<T extends Record<string, any>>(item: T): T {
  if (!item) return item;
  const { '@type': _typeUrl, ...rest } = item;
  return rest as T;
}

export async function describeA2AAgents(params: DescribeA2AAgentsRequest) {
  const res = await getApiRequest<DescribeA2AAgentsResponse>({
    action: BaseURL.A2A_AGENT,
    data: params,
  });

  const list = (res.data ?? []).map((item) => normalizeA2AObject(item));
  return {
    list,
    totalCount: res.amount ?? list.length,
  };
}

export async function describeAllA2AAgents() {
  const { list } = await describeA2AAgents({ offset: 0, limit: 10000 });
  return list;
}

export async function createA2AAgents(params: A2AAgent[]) {
  return apiRequest({
    action: BaseURL.A2A_AGENT,
    data: params,
  });
}

export async function modifyA2AAgents(params: A2AAgent[]) {
  return putApiRequest({
    action: BaseURL.A2A_AGENT,
    data: params,
  });
}

export async function deleteA2AAgents(ids: string[]) {
  return apiRequest({
    action: `${BaseURL.A2A_AGENT}/delete`,
    data: { agent_ids: ids },
  });
}

export async function describeA2AAgentSkills(params: DescribeA2AAgentSkillsRequest) {
  const res = await getApiRequest<DescribeA2AAgentSkillsResponse>({
    action: BaseURL.A2A_AGENT_SKILL,
    data: params,
  });

  const list = (res.data ?? []).map((item) => normalizeA2AObject(item));
  return {
    list,
    totalCount: res.amount ?? res.size ?? list.length,
  };
}

export async function describeA2AAgentCard(id: string) {
  return getApiRequest<Record<string, any>>({
    action: `${BaseURL.A2A_AGENT}/${id}/card`,
  });
}

export async function describeA2AAgentDefinitionBinding(resourceId: string) {
  return describeAIEnvironmentBinding<'a2a_agent'>(A2ADefinitionURL, resourceId);
}

export async function describeA2AAgentDefinitionEnvironments(definitionId: string) {
  return describeAIEnvironmentBindings<'a2a_agent'>(A2ADefinitionURL, definitionId);
}

export async function describeA2AAgentDefinitions(name?: string) {
  return describeAIResourceDefinitions(A2ADefinitionURL, name);
}

export async function createA2AAgentDefinition(name: string) {
  return createAIResourceDefinition(A2ADefinitionURL, 'A2A Agent', name);
}

export async function deleteA2AAgentDefinition(definitionId: string) {
  return deleteAIResourceDefinition(A2ADefinitionURL, definitionId);
}

export async function bindA2AAgentDefinition(definitionId: string, resourceId: string) {
  return bindAIResourceDefinition(A2ADefinitionURL, definitionId, resourceId);
}
