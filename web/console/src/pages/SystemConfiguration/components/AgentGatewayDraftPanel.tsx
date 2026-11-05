import React, { useEffect, useMemo, useState } from 'react';
import { Button, Input, TagInput, Textarea } from 'components/Fluent';
import EditorDrawer from 'components/EditorDrawer';
import { CheckCircleIcon, LockOnIcon, PlugConnectedIcon } from 'components/Fluent/icons';
import {
  AgentSystemDomain,
  AgentSystemProfile,
  SaveAgentDraftRequest,
  saveAgentSystemDraft,
  testAgentSystemConnection,
} from 'services/system_configuration';
import { toRequestErrorPayload } from 'utils/request';

import style from '../index.module.less';

interface Props {
  visible: boolean;
  domain?: AgentSystemDomain;
  fallback: AgentSystemProfile;
  onClose: () => void;
  onSaved: (domain: AgentSystemDomain) => void;
}

const errorText = (error: unknown) => {
  const payload = toRequestErrorPayload(error);
  return typeof payload === 'string' ? payload : payload.message;
};

export default function AgentGatewayDraftPanel({ visible, domain, fallback, onClose, onSaved }: Props) {
  const initial = useMemo(() => domain?.draft?.values || domain?.active?.values || fallback, [domain, fallback]);
  const [profile, setProfile] = useState(initial);
  const [apiKey, setAPIKey] = useState('');
  const [secretOperation, setSecretOperation] = useState<'keep' | 'replace'>('keep');
  const [busy, setBusy] = useState('');
  const [feedback, setFeedback] = useState<{ tone: 'success' | 'error'; text: string }>();
  const [verified, setVerified] = useState(false);
  const hasManagedSecret = Boolean(domain?.draft?.secret.configured || domain?.active?.secret.configured);

  useEffect(() => {
    if (!visible) return;
    setProfile(initial);
    setAPIKey('');
    setSecretOperation(hasManagedSecret ? 'keep' : 'replace');
    setFeedback(undefined);
    setVerified(false);
  }, [hasManagedSecret, initial, visible]);

  const invalidateTest = () => {
    setFeedback(undefined);
    setVerified(false);
  };

  const update = (key: keyof AgentSystemProfile, value: string | string[]) => {
    setProfile((current) => ({ ...current, [key]: value }));
    invalidateTest();
  };
  const request = (): SaveAgentDraftRequest => ({
    expectedDraftRevision: domain?.draft?.revision || 0,
    values: profile,
    apiKey: secretOperation === 'keep'
      ? { operation: 'keep' }
      : { operation: 'replace', value: apiKey },
  });
  const connectionComplete = Boolean(
    profile.baseURL.trim()
    && profile.model.trim()
    && (secretOperation === 'keep' ? hasManagedSecret : apiKey.trim()),
  );

  const test = async () => {
    setBusy('test');
    setFeedback(undefined);
    try {
      const result = await testAgentSystemConnection(request());
      setFeedback({ tone: 'success', text: `${result.message} · ${result.latencyMs} ms${result.model ? ` · ${result.model}` : ''}` });
      setVerified(true);
    } catch (error) {
      setFeedback({ tone: 'error', text: errorText(error) });
      setVerified(false);
    } finally {
      setBusy('');
    }
  };

  const save = async () => {
    setBusy('save');
    setFeedback(undefined);
    try {
      const result = await saveAgentSystemDraft(request());
      onSaved(result);
      onClose();
    } catch (error) {
      setFeedback({ tone: 'error', text: errorText(error) });
    } finally {
      setBusy('');
    }
  };

  return (
    <EditorDrawer
      visible={visible}
      width="standard"
      title="Agent 运行配置"
      description="编辑、预检并保存由 Pole 托管的 Agent 运行期望状态。"
      onClose={onClose}
      footer={(
        <div className={style.agentDrawerFooter}>
          <span className={verified ? style.agentFooterVerified : style.agentFooterPending}>
            {verified ? <><CheckCircleIcon /> 当前配置已通过可选预检</> : '保存后将自动完成连接测试并应用'}
          </span>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button
            theme="primary"
            loading={busy === 'save'}
            disabled={Boolean(busy) || !connectionComplete || !domain?.secretStoreReady}
            onClick={save}
          >
            保存并自动应用
          </Button>
        </div>
      )}
    >
      <div className={style.agentDrawer}>
        <header className={style.agentDrawerIntro}>
          <div className={style.agentEditorSteps} aria-label="Agent 配置流程">
            <span className={style.agentEditorStepActive}><b>1</b> 编辑配置</span>
            <span><b>2</b> 可选预检</span>
            <span><b>3</b> 保存并自动应用</span>
          </div>
          <p>系统管理员保存期望状态后，pole-self-manager 会自动复验 LLM、MCP 与 Prompt 契约；验证通过即原子热更新，失败则保留最近健康版本和待重试草稿。</p>
        </header>

        <section className={style.agentFormSection}>
          <div className={style.agentSectionIntro}>
            <span><PlugConnectedIcon /></span>
            <div><strong>模型与凭证</strong><p>连接 OpenAI-compatible LLM Gateway，API Key 由 Pole 加密托管。</p></div>
          </div>
          <div className={style.agentSectionFields}>
            <label>
              <span>LLM Gateway 地址</span>
              <Input value={profile.baseURL} onChange={(value) => update('baseURL', value)} placeholder="https://gateway.example.com/v1" />
              <small>使用 OpenAI-compatible `/chat/completions` 接口。</small>
            </label>
            <div className={style.agentFormGrid}>
              <label><span>模型</span><Input value={profile.model} onChange={(value) => update('model', value)} placeholder="例如 gpt-5" /></label>
              <label><span>请求超时</span><Input value={profile.modelTimeout} onChange={(value) => update('modelTimeout', value)} placeholder="60s" /></label>
            </div>
            <div className={style.agentProviderNote}>Provider 固定为 <strong>{profile.provider || 'openai-compatible'}</strong></div>
            <div className={style.agentSecretControl}>
              <div>
                <LockOnIcon />
                <span>
                  <strong>{hasManagedSecret ? `Pole Secret 已托管${domain?.draft?.secret.version || domain?.active?.secret.version ? ` · v${domain?.draft?.secret.version || domain?.active?.secret.version}` : ''}` : 'API Key 尚未配置'}</strong>
                  <small>Secret 正文只在本次提交中传输，之后永不回填。</small>
                </span>
              </div>
              {hasManagedSecret && (
                <Button
                  variant="outline"
                  size="small"
                  disabled={!domain?.secretStoreReady}
                  onClick={() => {
                    setSecretOperation(secretOperation === 'keep' ? 'replace' : 'keep');
                    setAPIKey('');
                    invalidateTest();
                  }}
                >
                  {secretOperation === 'replace' ? '取消替换' : '替换密钥'}
                </Button>
              )}
            </div>
            {!domain?.secretStoreReady && (
              <div className={style.agentSecretBlocked}>Secret Store 未就绪。请先配置系统根加密材料，再保存新的 API Key。</div>
            )}
            {secretOperation === 'replace' && (
              <label>
                <span>{hasManagedSecret ? '新 API Key' : 'API Key'}</span>
                <Input
                  type="password"
                  value={apiKey}
                  onChange={(value) => {
                    setAPIKey(value);
                    invalidateTest();
                  }}
                  disabled={!domain?.secretStoreReady}
                  autoComplete="new-password"
                  placeholder="输入后仅用于测试和加密保存"
                />
              </label>
            )}
            <div className={style.agentConnectionTest}>
              <div>
                <strong>连接测试</strong>
                <small>同时验证模型调用与 Pole MCP 可达性，测试结果会在配置改变后失效。</small>
              </div>
              <Button
                variant="outline"
                icon={<PlugConnectedIcon />}
                loading={busy === 'test'}
                disabled={Boolean(busy) || !connectionComplete || !domain?.secretStoreReady}
                onClick={test}
              >
                测试连接
              </Button>
            </div>
            {feedback && (
              <div className={feedback.tone === 'success' ? style.agentFeedbackSuccess : style.agentFeedbackError}>
                {feedback.tone === 'success' && <CheckCircleIcon />}
                <span>{feedback.text}</span>
              </div>
            )}
          </div>
        </section>

        <section className={style.agentFormSection}>
          <div className={style.agentSectionIntro}>
            <span><PlugConnectedIcon /></span>
            <div><strong>提案生命周期</strong><p>控制临时视图保留时间与资源工具的上游等待边界。</p></div>
          </div>
          <div className={style.agentSectionFields}>
            <div className={style.agentFormGrid}>
              <label>
                <span>提案有效期</span>
                <Input value={profile.proposalTTL} onChange={(value) => update('proposalTTL', value)} placeholder="30m" />
                <small>范围 1m ～ 24h；新生成的临时视图立即采用。</small>
              </label>
              <label>
                <span>资源工具上游超时</span>
                <Input value={profile.upstreamTimeout} onChange={(value) => update('upstreamTimeout', value)} placeholder="10s" />
                <small>范围 1s ～ 5m；Pole Server 请求立即采用。</small>
              </label>
            </div>
          </div>
        </section>

        <section className={style.agentFormSection}>
          <div className={style.agentSectionIntro}>
            <span><PlugConnectedIcon /></span>
            <div><strong>MCP 工具</strong><p>只向 Agent 暴露明确允许的控制面工具。</p></div>
          </div>
          <div className={style.agentSectionFields}>
            <label><span>MCP Endpoint</span><Input value={profile.mcpEndpoint} onChange={(value) => update('mcpEndpoint', value)} /></label>
            <label>
              <span>工具白名单</span>
              <TagInput
                value={profile.mcpToolAllowlist}
                onChange={(value: string[]) => update('mcpToolAllowlist', value)}
                placeholder="输入工具名，按 Enter 添加"
              />
              <small>写操作仍只能生成临时视图和草稿，Agent 无法直接发布资源。</small>
            </label>
          </div>
        </section>

        <section className={style.agentFormSection}>
          <div className={style.agentSectionIntro}>
            <span><LockOnIcon /></span>
            <div><strong>Agent 指令</strong><p>在 Pole 内置安全策略之外补充组织级操作约束。</p></div>
          </div>
          <div className={style.agentSectionFields}>
            <label>
              <span>Operator Instructions <small>{profile.promptVersion}</small></span>
              <Textarea value={profile.operatorInstructions} onChange={(value: string) => update('operatorInstructions', value)} rows={5} />
            </label>
          </div>
        </section>
      </div>
    </EditorDrawer>
  );
}
