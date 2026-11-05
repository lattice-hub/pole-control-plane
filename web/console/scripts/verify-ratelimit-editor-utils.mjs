import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import ts from 'typescript';

const root = process.cwd();
const sourcePath = path.join(root, 'src/pages/Governance/RateLimit/rateLimitEditorUtils.ts');
const editorPath = path.join(root, 'src/pages/Governance/RateLimit/RateLimitEditor.tsx');
const stylePath = path.join(root, 'src/pages/Governance/RateLimit/RateLimitEditor.module.less');
const runtimePath = path.join(root, 'src/pages/Governance/RateLimit/QuotaLeaseRuntime.tsx');

if (!fs.existsSync(sourcePath)) {
  throw new Error(`missing helper: ${sourcePath}`);
}
if (!fs.existsSync(editorPath)) {
  throw new Error(`missing editor: ${editorPath}`);
}
if (!fs.existsSync(stylePath)) {
  throw new Error(`missing style: ${stylePath}`);
}
if (!fs.existsSync(runtimePath)) {
  throw new Error(`missing quota lease runtime: ${runtimePath}`);
}

const source = fs.readFileSync(sourcePath, 'utf8');
const editorSource = fs.readFileSync(editorPath, 'utf8');
const styleSource = fs.readFileSync(stylePath, 'utf8');
const runtimeSource = fs.readFileSync(runtimePath, 'utf8');
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ES2020,
    target: ts.ScriptTarget.ES2020,
    esModuleInterop: true,
  },
  fileName: sourcePath,
});

const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ratelimit-editor-utils-'));
const modulePath = path.join(tempDir, 'rateLimitEditorUtils.mjs');
fs.writeFileSync(modulePath, compiled.outputText);
const utils = await import(pathToFileURL(modulePath));

const baseDraft = {
  id: 'rl-1',
  name: 'spec-check-ratelimit',
  namespace: 'spec-governance',
  callee: { namespace: 'spec-runtime', service: 'spec-gateway' },
  type: 'GLOBAL',
  priority: 15,
  disable: false,
  metadata: {
    owner: 'codex',
    scenario: 'sample',
  },
  rules: [
    {
      apis: [
        {
          protocol: 'HTTP',
          method: 'POST',
          path: { type: 'EXACT', value: '/api/v1/payments', value_type: 'TEXT' },
        },
      ],
      arguments: [
        {
          type: 'HEADER',
          key: 'x-tenant',
          value: { type: 'EXACT', value: 'vip', value_type: 'TEXT' },
        },
      ],
      resource: 'QPS',
      amounts: [
        { validDuration: 1, validDurationUnit: 's', maxAmount: 120 },
        { validDuration: 60, validDurationUnit: 's', maxAmount: 3000 },
      ],
      regex_combine: false,
      action: 'UNIRATE',
      failover: 'FAILOVER_PASS',
      max_queue_delay: 1,
      customResponse: { body: '{"code":429,"msg":"rate limited"}' },
    },
  ],
};

const payload = utils.buildRateLimitSubmitPayload(baseDraft);
assert.equal(payload.name, 'spec-check-ratelimit');
assert.deepEqual(payload.callee, { namespace: 'spec-runtime', service: 'spec-gateway' });
assert.equal(payload.type, 'GLOBAL');
assert.equal(payload.disable, false);
assert.deepEqual(payload.metadata, { owner: 'codex', scenario: 'sample' });
assert.equal(payload.rules[0].action, 'REJECT', 'global rate limit must force reject action');
assert.deepEqual(payload.rules[0].apis, [
  {
    protocol: 'HTTP',
    method: 'POST',
    path: { type: 'EXACT', value: '/api/v1/payments', value_type: 'TEXT' },
  },
]);
assert.deepEqual(payload.rules[0].amounts, [
  { validDuration: '1s', maxAmount: 120 },
  { validDuration: '60s', maxAmount: 3000 },
]);
assert.equal(payload.rules[0].customResponse.body, '{"code":429,"msg":"rate limited"}');
assert.equal(payload.apiVersion, undefined);
assert.equal(payload.kind, undefined);
assert.equal(payload.spec, undefined);

const yaml = utils.stringifyRateLimitSpec(payload, 'yaml');
assert.match(yaml, /name: spec-check-ratelimit/);
assert.match(yaml, /type: GLOBAL/);
assert.match(yaml, /rules:/);
assert.match(yaml, /apis:/);
assert.match(yaml, /validDuration: 1s/);
assert.doesNotMatch(yaml, /apiVersion:/);

const json = utils.stringifyRateLimitSpec(payload, 'json');
assert.match(json, /"name": "spec-check-ratelimit"/);

assert.equal(utils.describeRuleThreshold(baseDraft.rules[0]), '120 次 / 1s 等 2 窗');
assert.equal(utils.describeRuleSummary(baseDraft.rules[0], baseDraft.type), '1 个接口 · 1 个匹配条件 · 请求数 · 快速失败');

assert.deepEqual(utils.parseRateLimitInterfaceValue('POST /api/v1/payments'), {
  protocol: 'HTTP',
  method: 'POST',
  path: '/api/v1/payments',
});
assert.deepEqual(utils.parseRateLimitInterfaceValue('GRPC POST /payment.Pay/Charge'), {
  protocol: 'GRPC',
  method: 'POST',
  path: '/payment.Pay/Charge',
});
assert.equal(utils.buildRateLimitInterfaceValue({ protocol: 'HTTP', method: 'POST', path: '/api/v1/payments' }), 'POST /api/v1/payments');
assert.equal(utils.buildRateLimitInterfaceValue({ protocol: 'GRPC', method: 'POST', path: '/payment.Pay/Charge' }), 'GRPC POST /payment.Pay/Charge');
assert.equal(utils.getRateLimitInterfacePath('POST /api/v1/payments'), '/api/v1/payments');
assert.deepEqual(utils.normalizeRateLimitApis({ method: { type: 'EXACT', value: 'POST /api/v1/payments', value_type: 'TEXT' } }), [
  {
    protocol: 'HTTP',
    method: 'POST',
    path: { type: 'EXACT', value: '/api/v1/payments', value_type: 'TEXT' },
  },
]);

const concurrencyRule = {
  ...baseDraft.rules[0],
  resource: 'CONCURRENCY',
  concurrencyAmount: { maxAmount: 50 },
  amounts: [],
  action: 'REJECT',
};
assert.equal(utils.describeRuleThreshold(concurrencyRule), '50 并发');

const tokenRule = {
  ...baseDraft.rules[0],
  resource: 'TOKEN',
  amounts: [{ validDuration: 60, validDurationUnit: 's', maxAmount: 120000 }],
};
assert.equal(utils.describeRuleThreshold(tokenRule), '120000 Token / 60s');
assert.deepEqual(utils.validateRateLimitDraft({ ...baseDraft, rules: [{ ...tokenRule, amounts: [{ ...tokenRule.amounts[0], maxAmount: 0 }] }] }).map((item) => item.message), [
  '规则[1] 最大 Token 数必须大于 0',
]);

assert.deepEqual(utils.validateRateLimitDraft(baseDraft), []);

const invalidDraft = {
  ...baseDraft,
  name: 'Bad Name',
  rules: [
    {
      ...baseDraft.rules[0],
      apis: [{ protocol: 'HTTP', method: 'POST', path: { type: 'EXACT', value: '', value_type: 'TEXT' } }],
      amounts: [{ validDuration: 1, validDurationUnit: 's', maxAmount: 0 }],
      arguments: [
        {
          type: 'HEADER',
          key: 'x-tenant',
          value: { type: 'EXACT', value: '', value_type: 'TEXT' },
        },
      ],
      customResponse: { body: '{"code":429' },
    },
  ],
};

assert.deepEqual(utils.validateRateLimitDraft(invalidDraft).map((item) => item.message), [
  '规则名称必须为 kebab-case',
  '规则[1] 存在空的接口路径',
  '规则[1] 最大请求数必须大于 0',
  '规则[1] 存在空匹配值',
  '规则[1] 自定义响应必须是合法 JSON',
]);

assert.match(editorSource, /import \{ commaStringToTags, isTagInputMatchType, tagsToCommaString \} from "\.\.\/Router\/routeEditorUtils";/);
assert.match(editorSource, /TagInput/);
assert.match(editorSource, /RATE_LIMIT_PROTOCOL_OPTIONS/);
assert.match(editorSource, /RATE_LIMIT_HTTP_METHOD_OPTIONS/);
assert.match(editorSource, /normalizeRateLimitApis\(trigger\)/);
assert.match(editorSource, /updateApi\(apiIdx/);
assert.match(editorSource, /isTagInputMatchType\(apiMatchType\)/);
assert.match(editorSource, /const addInterface = \(\) =>/);
assert.match(editorSource, /新增接口/);
assert.match(editorSource, /API 列表/);
assert.doesNotMatch(editorSource, /<Select disabled value="HTTP"/);
assert.doesNotMatch(editorSource, /<Select disabled value="\*"/);
assert.doesNotMatch(editorSource, /复制为新接口规则/);
assert.match(editorSource, /value=\{commaStringToTags\(api\.path\?\.value \|\| ''\)\}/);
assert.match(editorSource, /tagsToCommaString\(value as Array<string \| number>\)/);
assert.match(editorSource, /onChange=\{\(value\) => updateApi\(apiIdx, \{ path: \{ \.\.\.api\.path, type: value as MatchType \} \}\)\}/);
assert.doesNotMatch(editorSource, /relationEditable=\{false\}/);
assert.doesNotMatch(editorSource, /relation=\{MatchLogic\.AND\}/);
assert.match(editorSource, /const relation = trigger\.matchMode \|\| MatchLogic\.AND;/);
assert.match(editorSource, /relation=\{relation\}/);
assert.match(editorSource, /relationEditable/);
assert.match(editorSource, /onRelationChange=\{\(value\) => updateRule\(ruleIdx, \{ matchMode: value as MatchLogic \}\)\}/);
assert.match(editorSource, /请求数 · 默认 1 秒/);
assert.match(editorSource, /Token 数 · 默认 60 秒/);
assert.doesNotMatch(editorSource, /title: 'RPM'/);
assert.doesNotMatch(editorSource, /value=\{RateLimitResource\.QPS\}>\{RateLimitResourceMap/);
assert.match(editorSource, /\? '并发上限' : '阈值窗口'/);
assert.match(editorSource, /流式 TPM 使用 Quota Lease/);
assert.match(editorSource, /预留成功后不会在响应流中再次触发配额 429/);
assert.doesNotMatch(editorSource, /shared\.verdictDeny/);
assert.match(editorSource, /className=\{styles\.strategyNote\}/);
assert.match(editorSource, /请求前校验/);
assert.match(editorSource, /QuotaLeaseRuntime namespace=\{rateLimit\.callee\.namespace\} service=\{rateLimit\.callee\.service\}/);
assert.match(runtimeSource, /当前预留/);
assert.match(runtimeSource, /流中已使用/);
assert.match(runtimeSource, /最近归还/);
assert.match(runtimeSource, /TTL 回收/);
assert.match(runtimeSource, /window\.setInterval\(load, 5000\)/);
assert.match(styleSource, /:global\(\.fluent-tag-input\),/);
assert.match(styleSource, /:global\(\.fui-Combobox\),/);
assert.match(styleSource, /:global\(\.fui-Combobox__input\) \{[\s\S]*?max-width: 100%;[\s\S]*?min-width: 0;[\s\S]*?width: 100%;/);
assert.match(styleSource, /> \* \{[\s\S]*?max-width: 100%;[\s\S]*?min-width: 0;/);
assert.match(styleSource, /\.interfaceGrid \{[\s\S]*?min-width: 720px;/);

console.log('ratelimit editor utils verification passed');
