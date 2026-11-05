import yaml from 'js-yaml';
import type { ConfigDefinition, Parameter, RenderSnapshot, Scalar } from './unified_config';

const encoder = new TextEncoder();
const decoder = new TextDecoder('utf-8', { fatal: true, ignoreBOM: true });
const namePattern = /^[A-Za-z_][A-Za-z0-9_-]*(\.[A-Za-z_][A-Za-z0-9_-]*)*$/;
const integerPattern = /^-?(?:0|[1-9][0-9]*)$/;
const decimalPattern = /^-?(?:0|[1-9][0-9]*)(?:\.[0-9]*[1-9])?$/;
const MAX_OUTPUT_BYTES = 512 * 1024;
const MAX_PARAMETERS = 128;

export function bytesFromBase64(content: string): Uint8Array {
  if (!/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(content)) {
    throw new Error('配置正文不是有效 base64');
  }
  const decoded = atob(content);
  return Uint8Array.from(decoded, (char) => char.charCodeAt(0));
}

export function textToBase64(content: string): string {
  const bytes = encoder.encode(content);
  let binary = '';
  for (let index = 0; index < bytes.length; index += 8192) {
    binary += String.fromCharCode(...bytes.subarray(index, index + 8192));
  }
  return btoa(binary);
}

export function textFromBase64(content: string): string {
  return decoder.decode(bytesFromBase64(content));
}

function validateScalar(scalar: Scalar, parameter: Parameter) {
  if (!scalar || scalar.type !== parameter.type || (scalar.value !== undefined && typeof scalar.value !== 'string')) {
    throw new Error(`参数 ${parameter.name} 的类型或值无效`);
  }
  const value = scalar.value ?? '';
  if (scalar.type === 'VALUE_TYPE_STRING' && decoder.decode(encoder.encode(value)) !== value) throw new Error(`参数 ${parameter.name} 不是有效 UTF-8`);
  if (scalar.type === 'VALUE_TYPE_BOOLEAN' && value !== 'true' && value !== 'false') throw new Error(`参数 ${parameter.name} 不是规范布尔值`);
  if (scalar.type === 'VALUE_TYPE_INTEGER' && (!integerPattern.test(value) || value === '-0')) throw new Error(`参数 ${parameter.name} 不是规范整数`);
  if (scalar.type === 'VALUE_TYPE_DECIMAL' && (!decimalPattern.test(value) || value === '-0')) throw new Error(`参数 ${parameter.name} 不是规范小数`);
  if (!['VALUE_TYPE_STRING', 'VALUE_TYPE_BOOLEAN', 'VALUE_TYPE_INTEGER', 'VALUE_TYPE_DECIMAL'].includes(scalar.type)) {
    throw new Error(`参数 ${parameter.name} 类型未知`);
  }
}

/** 仅用于旧 labels 共享向量；正式预览由后端选择 values，避免以 JS 正则模拟 RE2。 */
export function selectConfigValues(definition: ConfigDefinition, labels: Record<string, string>): Record<string, Scalar> {
  if (definition.valueRules?.some(rule => rule.audience?.conditions?.length)) {
    throw new Error('结构化条件必须通过服务端 Preview 选值');
  }
  const matches = (definition.valueRules || []).filter((rule) =>
    Object.entries(rule.audience?.labels || {}).every(([key, value]) => labels[key] === value));
  matches.sort((left, right) => left.priority - right.priority);
  if (matches.length > 1 && (matches[0].priority || 0) === (matches[1].priority || 0)) {
    throw new Error('同优先级规则匹配不唯一');
  }
  return { ...(definition.defaultValues || {}), ...(matches[0]?.values || {}) };
}

function rejectDuplicateJSONKeys(source: string) {
  let cursor = 0;
  const space = () => { while (/\s/.test(source[cursor] || '')) cursor += 1; };
  const readString = () => {
    const start = cursor++;
    while (cursor < source.length) {
      if (source[cursor] === '\\') { cursor += 2; continue; }
      if (source[cursor++] === '"') break;
    }
    return JSON.parse(source.slice(start, cursor)) as string;
  };
  const readValue = (): void => {
    space();
    if (source[cursor] === '{') {
      cursor += 1; space();
      const keys = new Set<string>();
      while (source[cursor] !== '}') {
        const key = readString();
        if (keys.has(key)) throw new Error(`JSON 存在重复键 ${key}`);
        keys.add(key); space(); cursor += 1; readValue(); space();
        if (source[cursor] !== ',') break;
        cursor += 1; space();
      }
      cursor += 1;
    } else if (source[cursor] === '[') {
      cursor += 1; space();
      while (source[cursor] !== ']') {
        readValue(); space();
        if (source[cursor] !== ',') break;
        cursor += 1; space();
      }
      cursor += 1;
    } else if (source[cursor] === '"') readString();
    else while (cursor < source.length && !/[\s,}\]]/.test(source[cursor])) cursor += 1;
  };
  readValue();
}

function validateFormat(content: string, format: string) {
  switch (format.toLowerCase()) {
    case '': case 'text': return;
    case 'json': JSON.parse(content); rejectDuplicateJSONKeys(content); return;
    case 'yaml': case 'yml': yaml.load(content, { json: false }); return;
    case 'xml': {
      const document = new DOMParser().parseFromString(content, 'application/xml');
      if (!document.documentElement || document.getElementsByTagName('parsererror').length) throw new Error('XML 格式无效');
      return;
    }
    case 'html': {
      const document = new DOMParser().parseFromString(content, 'text/html');
      if (!document.documentElement) throw new Error('HTML 格式无效');
      return;
    }
    case 'properties': {
      // Java properties accepts bare keys, escaped separators and odd-backslash continuations.
      const physicalLines = content.split(/\r\n|\n|\r/);
      for (let index = 0; index < physicalLines.length; index += 1) {
        let line = physicalLines[index];
        while (/\\+$/.test(line) && (line.match(/\\+$/)?.[0].length || 0) % 2 === 1 && index + 1 < physicalLines.length) {
          line = line.slice(0, -1) + physicalLines[++index].replace(/^\s+/, '');
        }
        if (!line.trim() || /^[#!]/.test(line.trimStart())) continue;
        for (let cursor = 0; cursor < line.length; cursor += 1) {
          if (line[cursor] !== '\\') continue;
          if (cursor + 1 >= line.length) throw new Error('properties 转义未结束');
          if (line[cursor + 1] === 'u') {
            if (!/^[0-9a-fA-F]{4}$/.test(line.slice(cursor + 2, cursor + 6))) throw new Error('properties Unicode 转义无效');
            cursor += 5;
          } else cursor += 1;
        }
      }
      return;
    }
    default: throw new Error(`未知模板格式 ${format}`);
  }
}

export function renderConfigSnapshot(snapshot: RenderSnapshot): { bytes: Uint8Array; text?: string } {
  const bytes = bytesFromBase64(snapshot.content || '');
  if (bytes.length > MAX_OUTPUT_BYTES) throw new Error('配置正文超过 512 KiB');
  if (snapshot.engine === 'literal') {
    if ((snapshot.parameters || []).length || Object.keys(snapshot.values || {}).length) {
      throw new Error('literal 不允许参数或 values');
    }
    return { bytes };
  }
  if (snapshot.engine !== 'pole-mustache-v1') throw new Error(`未知模板引擎 ${snapshot.engine}`);
  const template = decoder.decode(bytes);
  if ((snapshot.parameters || []).length > MAX_PARAMETERS) throw new Error('参数声明超过 128 个');
  let inputBytes = bytes.length;
  const parameters = new Map<string, Parameter>();
  for (const parameter of snapshot.parameters || []) {
    if (!namePattern.test(parameter.name) || parameters.has(parameter.name)) throw new Error(`参数声明无效或重复：${parameter.name}`);
    if (!['VALUE_TYPE_STRING', 'VALUE_TYPE_BOOLEAN', 'VALUE_TYPE_INTEGER', 'VALUE_TYPE_DECIMAL'].includes(parameter.type)) {
      throw new Error(`参数 ${parameter.name} 类型未知`);
    }
    inputBytes += encoder.encode(parameter.name).length;
    if (inputBytes > MAX_OUTPUT_BYTES) throw new Error('模板输入超过 512 KiB');
    parameters.set(parameter.name, parameter);
  }
  const values = Object.fromEntries(Object.entries(snapshot.values || {}).map(([name, scalar]) => [name, scalar && scalar.value === undefined ? { ...scalar, value: '' } : scalar]));
  for (const [name, scalar] of Object.entries(values)) {
    const parameter = parameters.get(name);
    if (!parameter) throw new Error(`参数 ${name} 未声明`);
    validateScalar(scalar, parameter);
    inputBytes += encoder.encode(scalar.value).length;
    if (inputBytes > MAX_OUTPUT_BYTES) throw new Error('模板输入超过 512 KiB');
  }
  for (const parameter of parameters.values()) {
    if (parameter.required && !Object.hasOwn(values, parameter.name)) throw new Error(`缺少必填参数 ${parameter.name}`);
  }
  const fragments: string[] = [];
  let outputBytes = 0;
  const append = (fragment: string) => {
    outputBytes += encoder.encode(fragment).length;
    if (outputBytes > MAX_OUTPUT_BYTES) throw new Error('渲染结果超过 512 KiB');
    fragments.push(fragment);
  };
  let position = 0;
  while (position < template.length) {
    const open = template.indexOf('{{', position);
    const close = template.indexOf('}}', position);
    if (close !== -1 && (open === -1 || close < open)) throw new Error('模板包含非法占位符语法');
    if (open === -1) { append(template.slice(position)); break; }
    append(template.slice(position, open));
    const match = /^\{\{\{([A-Za-z_][A-Za-z0-9_-]*(?:\.[A-Za-z_][A-Za-z0-9_-]*)*)\}\}\}/.exec(template.slice(open));
    if (!match || template[open + match[0].length] === '}') throw new Error('模板包含非法占位符语法');
    const parameter = parameters.get(match[1]);
    if (!parameter || !Object.hasOwn(values, match[1])) throw new Error(`占位符 ${match[1]} 未声明或没有值`);
    append(values[match[1]].value);
    position = open + match[0].length;
  }
  const rendered = fragments.join('');
  const output = encoder.encode(rendered);
  validateFormat(rendered, snapshot.format || '');
  return { bytes: output, text: rendered };
}
