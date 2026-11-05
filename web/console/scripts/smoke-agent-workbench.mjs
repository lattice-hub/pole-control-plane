const baseURL = process.env.POLE_BASE_URL || 'http://127.0.0.1:8080';
const user = process.env.POLE_USER || 'admin';
const password = process.env.POLE_PASSWORD || 'admin123';
const keepFixture = process.env.POLE_KEEP_FIXTURE === '1';

const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(4, 14);
const key = { environment: 'default', group: `codex-agent-${stamp}`, name: 'app.yaml' };
const baselineContent = 'feature:\n  enabled: false\n';
const desiredContent = 'feature:\n  enabled: true\n';
let authHeaders = {};
let created = false;

async function request(path, body, type) {
  const response = await fetch(`${baseURL}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders },
    body: JSON.stringify(body),
  });
  const text = await response.text();
  const payload = text ? JSON.parse(text) : {};
  if (!response.ok || ![200000, 299999].includes(payload.code)) {
    throw new Error(`POST ${path} failed: http=${response.status} body=${text}`);
  }
  if (type && payload.data?.['@type'] !== `type.googleapis.com/pole.config.v1.${type}`) {
    throw new Error(`POST ${path} returned an unexpected Any type`);
  }
  return payload.data;
}

async function state(releaseId = '') {
  return request('/config/v1/state', { key, releaseId }, 'ConfigState');
}

async function cleanup() {
  if (keepFixture || !created) return;
  try {
    const current = await state();
    await request('/config/v1/offline', { key, expectedReleaseSetRevision: current.releaseSetRevision || '0' }, 'ConfigState');
  } catch (error) {
    console.error('Fixture offline failed:', error);
  }
}

async function main() {
  const login = await request('/auth/v1/user/login', { name: user, password });
  authHeaders = { Authorization: login.token };

  const draft = await request('/config/v1/draft', { key, expectedDraftRevision: '0', definition: {
    content: Buffer.from(baselineContent).toString('base64'), engine: 'literal', format: 'yaml',
  } }, 'ConfigState');
  created = true;
  const published = await request('/config/v1/publish', {
    key, expectedDraftRevision: draft.draft.revision, expectedReleaseSetRevision: draft.releaseSetRevision || '0',
    kind: 'RELEASE_KIND_NORMAL', audience: { labels: {} }, priority: 0,
  }, 'ConfigState');
  const releaseId = published.releases?.find((release) => release.active && release.kind === 'RELEASE_KIND_NORMAL')?.id;
  if (!releaseId) throw new Error('baseline normal release was not activated');
  const activeBefore = await state(releaseId);
  if (Buffer.from(activeBefore.selectedRelease.definition.content, 'base64').toString('utf8') !== baselineContent) {
    throw new Error('baseline release content mismatch');
  }

  const proposal = await request('/ai/agent/v1/proposals/config-file', {
    namespace: key.environment, group: key.group, name: key.name, desiredContent,
  });
  if (proposal.status !== 'preview_ready') throw new Error(`unexpected proposal status: ${proposal.status}`);

  const before = await state(releaseId);
  if (Buffer.from(before.draft.definition.content, 'base64').toString('utf8') !== baselineContent ||
      Buffer.from(before.selectedRelease.definition.content, 'base64').toString('utf8') !== baselineContent) {
    throw new Error('preview must not mutate draft or active release');
  }

  const confirmation = {
    proposalVersion: proposal.version,
    previewHash: proposal.previewHash,
    idempotencyKey: `agent-smoke-${stamp}`,
  };
  const receipt = await request(`/ai/agent/v1/proposals/${proposal.id}/confirm`, confirmation);
  const repeatedReceipt = await request(`/ai/agent/v1/proposals/${proposal.id}/confirm`, confirmation);
  if (receipt.status !== 'waiting_for_publish' || repeatedReceipt.proposalId !== receipt.proposalId) {
    throw new Error('confirmation must be idempotent and wait for publish');
  }

  const after = await state(releaseId);
  if (Buffer.from(after.draft.definition.content, 'base64').toString('utf8') !== desiredContent) {
    throw new Error('confirmation did not save desired draft');
  }
  if (Buffer.from(after.selectedRelease.definition.content, 'base64').toString('utf8') !== baselineContent) {
    throw new Error('Agent confirmation must not publish');
  }

  console.log(JSON.stringify({ ok: true, key, proposalStatus: proposal.status, receiptStatus: receipt.status,
    draftChanged: true, activeReleaseUnchanged: true, keepFixture }));
}

main().finally(cleanup).catch((error) => { console.error(error); process.exitCode = 1; });
