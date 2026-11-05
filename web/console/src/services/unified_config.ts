import { apiRequest } from 'utils/request';

export interface ConfigKey { environment: string; group: string; name: string }
export type ValueType = 'VALUE_TYPE_STRING' | 'VALUE_TYPE_BOOLEAN' | 'VALUE_TYPE_INTEGER' | 'VALUE_TYPE_DECIMAL';
export interface Scalar { type: ValueType; value: string }
export interface Parameter { name: string; type: ValueType; required?: boolean }
export type MatchType = 'MATCH_TYPE_EXACT' | 'MATCH_TYPE_REGEX' | 'MATCH_TYPE_NOT_EQUALS' | 'MATCH_TYPE_IN' | 'MATCH_TYPE_NOT_IN' | 'MATCH_TYPE_RANGE';
export interface MatchCondition { key: string; type: MatchType; value: string }
export interface Audience { labels?: Record<string, string>; conditions?: MatchCondition[] }
export interface NamedConditionGroup { id: string; name: string; audience: Audience }
export interface ParameterBinding { id: string; groupId: string; value: Scalar }
export interface ParameterBindings { name: string; bindings?: ParameterBinding[]; fallback?: Scalar }
export interface ValueRule { id: string; priority: number; audience: Audience; values: Record<string, Scalar> }
export interface ConfigDefinition {
  content?: string; // protobuf JSON bytes: base64; empty bytes may be omitted
  engine: 'literal' | 'pole-mustache-v1';
  format?: string; // empty string may be omitted
  parameters?: Parameter[];
  defaultValues?: Record<string, Scalar>;
  valueRules?: ValueRule[];
  conditionGroups?: NamedConditionGroup[];
  parameterBindings?: ParameterBindings[];
  compatibility?: { apollo?: boolean; nacos?: boolean; consul?: boolean };
}
export interface ConfigDraft { key: ConfigKey; definition: ConfigDefinition; revision: string }
export type ReleaseKind = 'RELEASE_KIND_NORMAL' | 'RELEASE_KIND_GRAY';
export interface ReleaseSummary {
  id: string; kind: ReleaseKind; audience?: Audience; priority?: number;
  sequence: string; createdAt?: string; createdBy?: string; sourceReleaseId?: string; active?: boolean;
}
export interface ConfigRelease extends ReleaseSummary { key: ConfigKey; definition: ConfigDefinition }
export interface ConfigState {
  key: ConfigKey; draft?: ConfigDraft; releases?: ReleaseSummary[];
  releaseSetRevision: string; incarnation?: string; offline?: boolean;
  nextPageToken?: string; selectedRelease?: ConfigRelease; activeReleaseIds?: string[];
}
export interface RenderSnapshot {
  content: string; engine: string; format: string;
  parameters?: Parameter[]; values?: Record<string, Scalar>;
}

/** proto3 JSON omits empty bytes/string fields; normalize only those defaults before a full-definition CAS write. */
export function parseConfigDefinition(source: string): ConfigDefinition {
  const value = JSON.parse(source) as ConfigDefinition;
  if (!value || typeof value !== 'object' || Array.isArray(value) ||
      (value.content !== undefined && typeof value.content !== 'string') ||
      (value.format !== undefined && typeof value.format !== 'string') ||
      (value.engine !== 'literal' && value.engine !== 'pole-mustache-v1')) {
    throw new Error('Definition 的 engine、content 或 format 无效');
  }
  return { ...value, content: value.content ?? '', format: value.format ?? '' };
}

const TYPE_PREFIX = 'type.googleapis.com/pole.config.v1.';
async function request<T>(action: string, data: object, type: string): Promise<T> {
  const response = await apiRequest<T & { '@type'?: string }>({ action: `/config/v1/${action}`, data });
  if (!response || response['@type'] !== TYPE_PREFIX + type) {
    throw new Error(`配置接口返回了非预期数据类型：${response?.['@type'] || '空'}`);
  }
  // The Any discriminator belongs to the response envelope, not the message.
  // Keeping it on a returned template makes a later nested protobuf JSON write invalid.
  const { '@type': _type, ...message } = response;
  return message as T;
}

export const getConfigState = (key: ConfigKey, pageToken = '', releaseId = '') =>
  request<ConfigState>('state', { key, pageSize: 20, pageToken, releaseId }, 'ConfigState');
export const saveConfigDraft = (key: ConfigKey, definition: ConfigDefinition, expectedDraftRevision: string) =>
  request<ConfigState>('draft', { key, definition, expectedDraftRevision }, 'ConfigState');
export const previewConfig = (key: ConfigKey, definition: ConfigDefinition, labels: Record<string, string>) =>
  request<RenderSnapshot>('preview', { key, definition, labels }, 'RenderSnapshot');
export const publishConfig = (key: ConfigKey, expectedDraftRevision: string, expectedReleaseSetRevision: string,
  kind: ReleaseKind, audience: Audience, priority: number, replaceReleaseId = '', stopReleaseId = '') =>
  request<ConfigState>('publish', { key, expectedDraftRevision, expectedReleaseSetRevision, kind, audience, priority,
    replaceReleaseId, stopReleaseId }, 'ConfigState');
export const stopConfigGray = (key: ConfigKey, expectedReleaseSetRevision: string, releaseId: string) =>
  request<ConfigState>('stop', { key, expectedReleaseSetRevision, releaseId }, 'ConfigState');
export const stopAllConfigGray = (key: ConfigKey, expectedReleaseSetRevision: string) =>
  request<ConfigState>('stop-all-gray', { key, expectedReleaseSetRevision }, 'ConfigState');
export const copyConfigReleaseToDraft = (key: ConfigKey, releaseId: string, expectedDraftRevision: string,
  expectedReleaseSetRevision: string) =>
  request<ConfigState>('copy-release-to-draft', { key, releaseId, expectedDraftRevision, expectedReleaseSetRevision }, 'ConfigState');
export const rollbackConfig = (key: ConfigKey, sourceReleaseId: string, replaceReleaseId: string,
  expectedReleaseSetRevision: string) =>
  request<ConfigState>('rollback', { key, sourceReleaseId, replaceReleaseId, expectedReleaseSetRevision }, 'ConfigState');
export const offlineConfig = (key: ConfigKey, expectedReleaseSetRevision: string) =>
  request<ConfigState>('offline', { key, expectedReleaseSetRevision }, 'ConfigState');


export interface ConfigFileKey { group: string; name: string }
export interface ConfigFileTemplate { key: ConfigFileKey; definition: ConfigDefinition; environments: string[]; revision: string; description?: string }
export interface ConfigFileSummary { key: ConfigFileKey; environment: string; description?: string; hasSharedTemplate?: boolean; hasDraft?: boolean; hasActiveRelease?: boolean; offline?: boolean; draftRevision?: string }
export interface ConfigFileList { items?: ConfigFileSummary[]; nextPageToken?: string }
export const listConfigFiles = (environment: string, group = '', query = '', pageToken = '') =>
  request<ConfigFileList>('files', { environment, group, query, pageSize: 50, pageToken }, 'ConfigFileList');
export const getConfigFileTemplate = (key: ConfigFileKey, environment: string) =>
  request<ConfigFileTemplate>('file-template', { key, environment }, 'ConfigFileTemplate');
export const saveConfigFileTemplate = (template: ConfigFileTemplate, expectedRevision: string) =>
  request<ConfigFileTemplate>('file-template-draft', { template, expectedRevision }, 'ConfigFileTemplate');
export const isConfigNotFound = (error: unknown) => Boolean(error && typeof error === 'object' && 'code' in error && error.code === 404202);
export const isConfigConflict = (error: unknown) => Boolean(error && typeof error === 'object' && 'code' in error && error.code === 409000);
/** 使用真实 Preview 的 ValueRule 匹配器测试受众；不保存、不发布测试定义。 */
export async function testConfigAudience(key: ConfigKey, audience: Audience, labels: Record<string, string>) {
  const snapshot = await previewConfig(key, { engine: 'pole-mustache-v1', content: '', format: 'text',
    parameters: [{ name: 'audience_match', type: 'VALUE_TYPE_BOOLEAN' }],
    defaultValues: { audience_match: { type: 'VALUE_TYPE_BOOLEAN', value: 'false' } },
    valueRules: [{ id: 'audience-test', priority: 0, audience, values: { audience_match: { type: 'VALUE_TYPE_BOOLEAN', value: 'true' } } }] }, labels);
  return snapshot.values?.audience_match?.value === 'true';
}

export const promoteConfigGray = (key: ConfigKey, releaseId: string, expectedReleaseSetRevision: string) =>
  request<ConfigState>('promote-gray', { key, releaseId, expectedReleaseSetRevision }, 'ConfigState');
