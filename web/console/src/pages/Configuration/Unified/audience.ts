import type { Audience, MatchCondition, MatchType } from 'services/unified_config';

export const matchTypes: MatchType[] = ['MATCH_TYPE_EXACT', 'MATCH_TYPE_REGEX', 'MATCH_TYPE_NOT_EQUALS', 'MATCH_TYPE_IN', 'MATCH_TYPE_NOT_IN', 'MATCH_TYPE_RANGE'];
const object = (value: unknown): value is Record<string, unknown> => value !== null && typeof value === 'object' && !Array.isArray(value);

/** Reject malformed wire data before rendering or editing it; callers retain the original JSON. */
export function audienceConditions(value: unknown): MatchCondition[] {
  if (value === undefined) return [];
  if (!object(value)) throw new Error('Audience 必须是对象');
  if (value.labels !== undefined && (!object(value.labels) || Object.values(value.labels).some(item => typeof item !== 'string'))) throw new Error('旧 labels 必须是字符串到字符串的对象');
  if (value.conditions !== undefined && !Array.isArray(value.conditions)) throw new Error('conditions 必须是数组');
  const labels = value.labels as Record<string, string> | undefined;
  const rawConditions = value.conditions as unknown[] | undefined;
  if (labels && Object.keys(labels).length && rawConditions && rawConditions.length) throw new Error('条件与旧标签不能同时存在');
  if (rawConditions && rawConditions.length) {
    return rawConditions.map((condition, index) => {
      if (!object(condition) || typeof condition.key !== 'string' || (condition.value !== undefined && typeof condition.value !== 'string') || typeof condition.type !== 'string') throw new Error(`条件 ${index + 1} 的 key、type 和 value 必须是字符串`);
      if (!matchTypes.includes(condition.type as MatchType)) throw new Error(`条件 ${index + 1} 的匹配方式未知：${condition.type}`);
      return { ...condition, value: condition.value === undefined ? '' : condition.value } as MatchCondition;
    });
  }
  if (labels) return Object.entries(labels).map(([key, item]) => ({ key, type: 'MATCH_TYPE_EXACT', value: item }));
  return [];
}
export function validateAudience(audience: Audience, required = false) {
  const conditions = audienceConditions(audience);
  if (required && !conditions.length) throw new Error('至少添加一个匹配条件');
  const seen = new Set<string>();
  for (const condition of conditions) {
    if (!condition.key || seen.has(condition.key)) throw new Error('条件键不能为空或重复');
    seen.add(condition.key);
    if (condition.type === 'MATCH_TYPE_RANGE') {
      const match = /^([+-]?\d+)~([+-]?\d+)$/.exec(condition.value);
      if (!match) throw new Error('范围须为有符号整数 left~right');
      const left = BigInt(match[1]); const right = BigInt(match[2]);
      if (left < -(BigInt(2) ** BigInt(63)) || right > BigInt(2) ** BigInt(63) - BigInt(1) || left > right) throw new Error('范围须为有效的有序 int64 边界');
    }
  }
}
