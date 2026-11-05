export interface DirtyEntry {
  definition: unknown;
  saved: string;
  needsDraft: boolean;
}

function normalize(definition: unknown): unknown {
  if (!definition || typeof definition !== 'object' || Array.isArray(definition)) return definition;
  const value = definition as Record<string, unknown>;
  return { ...value, content: value.content ?? '', format: value.format ?? '', parameters: value.parameters ?? [], conditionGroups: value.conditionGroups ?? [], parameterBindings: value.parameterBindings ?? [], valueRules: value.valueRules ?? [], defaultValues: value.defaultValues ?? {} };
}

function canonical(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).sort(([a], [b]) => a.localeCompare(b)).map(([key, item]) => [key, canonical(item)]));
  return value;
}

/** 缺少已绑定环境的草稿也需要保存，即使正文和取值没有发生编辑。 */
export function hasPendingEnvironmentDraft(entries: DirtyEntry[], compose: (definition: unknown) => unknown): boolean {
  return entries.some(entry => entry.needsDraft || JSON.stringify(canonical(normalize(compose(entry.definition)))) !== JSON.stringify(canonical(normalize(JSON.parse(entry.saved)))));
}
