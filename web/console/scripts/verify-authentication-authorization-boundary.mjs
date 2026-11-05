import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const request = readFileSync(new URL('../src/utils/request.ts', import.meta.url), 'utf8');

assert.match(request, /TokenDisabledCode\s*=\s*401003/, 'Token 被禁用必须被识别为认证失效');
assert.match(request, /TokenNotExistCode\s*=\s*401004/, 'Token 不存在必须使用当前协议业务码 401004');
assert.match(
  request,
  /action\s*===\s*['"]\/auth\/v1\/user\/session['"]\s*&&\s*status\s*===\s*401/,
  '只有 Console Session 接口的 HTTP 401 可以单独证明登录态失效',
);

const classifier = request.match(/const shouldResetLogin[\s\S]*?\n\}/)?.[0] || '';
assert.ok(classifier, '必须保留集中式登录态失效分类器');
assert.doesNotMatch(classifier, /code\s*===\s*NotAllowedAccessCode/, '业务码 401001 仅表示无授权，不能触发退出登录');
assert.doesNotMatch(classifier, /return\s+status\s*===\s*401\s*\|\|/, '普通 API 的 HTTP 401 不能直接触发退出登录');
assert.match(classifier, /code\s*===\s*TokenDisabledCode/, '明确的 Token 禁用必须触发退出登录');
assert.match(classifier, /code\s*===\s*TokenNotExistCode/, '明确的 Token 缺失必须触发退出登录');

console.log('Authentication and authorization boundary verified.');
