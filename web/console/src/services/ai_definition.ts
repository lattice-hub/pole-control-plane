import { apiRequest, getApiRequest } from 'utils/request';

export interface AIEnvironmentBinding<Kind extends string> {
  kind: Kind;
  definition_id: string;
  resource_id: string;
  namespace: string;
  resource_name: string;
}

export interface AIResourceDefinition {
  id: string;
  name: string;
  description?: string;
  revision?: string;
}

interface ListResponse<T> {
  amount: number;
  size: number;
  data?: T[];
}

export async function describeAIEnvironmentBinding<Kind extends string>(baseURL: string, resourceId: string) {
  return getApiRequest<AIEnvironmentBinding<Kind>>({
    action: `${baseURL}/environment-binding`,
    data: { resource_id: resourceId },
  });
}

export async function describeAIEnvironmentBindings<Kind extends string>(baseURL: string, definitionId: string) {
  const response = await getApiRequest<ListResponse<AIEnvironmentBinding<Kind>>>({
    action: `${baseURL}/environments`,
    data: { definition_id: definitionId },
  });
  return response.data ?? [];
}

export async function describeAIResourceDefinitions(baseURL: string, name?: string) {
  const response = await getApiRequest<ListResponse<AIResourceDefinition>>({
    action: baseURL,
    data: { offset: 0, limit: 10000, name },
  });
  return response.data ?? [];
}

export async function createAIResourceDefinition(baseURL: string, resourceLabel: string, name: string) {
  await apiRequest({
    action: baseURL,
    data: [{ name }],
  });
  const definitions = await describeAIResourceDefinitions(baseURL, name);
  const definition = definitions.find((item) => item.name === name);
  if (!definition) throw new Error(`新建 ${resourceLabel} 逻辑定义后未能解析其 ID`);
  return definition;
}

export async function deleteAIResourceDefinition(baseURL: string, definitionId: string) {
  return apiRequest({
    action: `${baseURL}/delete`,
    data: { definition_ids: [definitionId] },
  });
}

export async function bindAIResourceDefinition(baseURL: string, definitionId: string, resourceId: string) {
  return apiRequest({
    action: `${baseURL}/environment-bindings`,
    data: { definition_id: definitionId, resource_id: resourceId },
  });
}
