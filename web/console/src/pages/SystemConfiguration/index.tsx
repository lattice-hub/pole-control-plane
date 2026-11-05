import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Button, Dialog, Empty, Table, TableColumnData, Tag, Textarea } from 'components/Fluent';
import { ArrowRightIcon, CheckCircleIcon, RefreshIcon, RocketIcon } from 'components/Fluent/icons';
import { ResourceHeader, ResourceToolbar } from 'components/ResourceLayout';
import QueryComposer from 'components/QueryComposer';
import {
  describeEffectiveSystemSettings,
  AgentSystemDomain,
  AgentSystemProfile,
  EffectiveSystemSetting,
  getAgentSystemDomain,
  getManagedSystemDomain,
  ManagedSystemDomain,
  publishAgentSystemDraft,
  publishManagedSystemDraft,
  SettingComponent,
  SettingSourceKind,
} from 'services/system_configuration';
import { toRequestErrorPayload } from 'utils/request';

import style from './index.module.less';
import AgentConfigurationWorkspace from './components/AgentConfigurationWorkspace';
import AgentGatewayDraftPanel from './components/AgentGatewayDraftPanel';
import ManagedDomainDraftPanel from './components/ManagedDomainDraftPanel';

type SettingRow = EffectiveSystemSetting & { rowKey: string };

const COMPONENT_LABEL: Record<SettingComponent, string> = {
  'pole-server': 'Pole Server',
  'pole-console': 'Console',
};

const DOMAIN_META: Record<string, { label: string; description: string }> = {
  bootstrap: { label: '启动与装配', description: '进程模式、日志配置和 API Server 装配入口。' },
  namespace: { label: '命名空间', description: '命名空间创建和隔离相关的服务端行为。' },
  naming: { label: '服务发现', description: '服务注册、健康检查和实例生命周期参数。' },
  compatibility: { label: '协议兼容', description: 'Nacos、Eureka 等兼容协议缺省请求的环境映射。' },
  config: { label: '配置中心', description: '配置文件能力开关和内容约束。' },
  cache: { label: '缓存同步', description: '控制面缓存的增量同步与回溯参数。' },
  storage: { label: '存储', description: '持久化插件、地址和凭证等自举配置。' },
  auth: { label: '认证授权', description: '控制面认证、授权以及 Console 登录安全参数。' },
  'workload-credential': { label: '工作负载凭证', description: '数据面身份凭证的签发和生命周期策略。' },
  logging: { label: '日志', description: 'Console 日志级别、轮转和保留策略。' },
  runtime: { label: '运行时', description: 'Console Web 监听、静态资源和功能开关。' },
  upstream: { label: '上游连接', description: 'Console 访问 Pole Server 和监控服务的连接地址。' },
  observability: { label: '可观测查询', description: '指标、事件和操作审计查询的连接与超时参数。' },
  agent: { label: 'Agent', description: 'Agent 定义、LLM Gateway、Pole MCP 工具与提案生命周期。' },
};

const DOMAIN_ORDER: Record<SettingComponent, string[]> = {
  'pole-server': ['bootstrap', 'namespace', 'naming', 'compatibility', 'config', 'cache', 'storage', 'auth', 'workload-credential'],
  'pole-console': ['runtime', 'logging', 'auth', 'upstream', 'storage', 'observability', 'agent'],
};

const SOURCE_LABEL: Record<SettingSourceKind, string> = {
  compiled_default: '编译默认值',
  static_file: '静态配置',
  environment: '环境变量',
  command_line: '命令行覆盖',
  dynamic_release: 'Pole 动态配置',
};

const APPLY_MODE_LABEL: Record<EffectiveSystemSetting['apply_mode'], string> = {
  BootstrapOnly: '仅启动配置',
  RestartRequired: '重启生效',
  HotReload: '动态生效',
  GuardedHotReload: '受控动态生效',
};

const sourceOptions = [
  { label: '全部来源', value: '' },
  { label: '编译默认值', value: 'compiled_default' },
  { label: '静态配置', value: 'static_file' },
  { label: '环境变量', value: 'environment' },
  { label: '命令行覆盖', value: 'command_line' },
  { label: 'Pole 动态配置', value: 'dynamic_release' },
];

const columns: TableColumnData<SettingRow>[] = [
  {
    colKey: 'setting',
    title: '配置项',
    width: 260,
    cell: ({ row }) => (
      <div className={style.settingCell}>
        <strong>{row.label}</strong>
        <code>{row.key}</code>
      </div>
    ),
  },
  {
    colKey: 'effective',
    title: '当前有效值',
    width: 230,
    cell: ({ row }) => (
      <div className={style.valueCell}>
        <span className={row.redacted ? style.redacted : undefined}>{row.display_value || '未配置'}</span>
        {row.source.reference && <small>{row.source.reference}</small>}
      </div>
    ),
  },
  {
    colKey: 'source',
    title: '来源',
    width: 130,
    cell: ({ row }) => <Tag variant="outline">{SOURCE_LABEL[row.source.kind]}</Tag>,
  },
  {
    colKey: 'desired',
    title: '发布目标',
    width: 210,
    cell: ({ row }) => row.desired_revision ? (
      <div className={style.valueCell}>
        <span>{row.desired_display_value || '未配置'}</span>
        <small>目标 r{row.desired_revision}{row.drifted ? ' · 与当前实例不同' : ''}</small>
      </div>
    ) : <span className={style.noDesired}>尚未发布目标值</span>,
  },
  {
    colKey: 'apply_mode',
    title: '生效方式',
    width: 150,
    cell: ({ row }) => <Tag variant="outline">{APPLY_MODE_LABEL[row.apply_mode]}</Tag>,
  },
  {
    colKey: 'management',
    title: '管理方式',
    width: 150,
    cell: ({ row }) => (
      <div className={style.managementCell}>
        <Tag variant="outline">{row.editable ? '页面可编辑' : '部署配置'}</Tag>
        <small>{row.editable ? APPLY_MODE_LABEL[row.apply_mode] : row.edit_reason}</small>
      </div>
    ),
  },
];

export default function SystemConfiguration() {
  const initialLocation = useMemo(() => {
    const params = new URLSearchParams(window.location.search);
    const requestedComponent = params.get('component');
    return {
      component: requestedComponent === 'pole-console' ? 'pole-console' : 'pole-server',
      domain: params.get('domain') || '',
    } as { component: SettingComponent; domain: string };
  }, []);
  const [settings, setSettings] = useState<EffectiveSystemSetting[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [component, setComponent] = useState<SettingComponent>(initialLocation.component);
  const [activeDomain, setActiveDomain] = useState(initialLocation.domain);
  const [source, setSource] = useState('');
  const [agentDomain, setAgentDomain] = useState<AgentSystemDomain>();
  const [agentPanelOpen, setAgentPanelOpen] = useState(false);
  const [publishOpen, setPublishOpen] = useState(false);
  const [publishDescription, setPublishDescription] = useState('');
  const [publishing, setPublishing] = useState(false);
  const [publishError, setPublishError] = useState('');
  const [managedDomain, setManagedDomain] = useState<ManagedSystemDomain>();
  const [managedPanelOpen, setManagedPanelOpen] = useState(false);
  const [managedPublishOpen, setManagedPublishOpen] = useState(false);
  const [managedPublishing, setManagedPublishing] = useState(false);
  const [managedPublishDescription, setManagedPublishDescription] = useState('');
  const [managedPublishError, setManagedPublishError] = useState('');
  const activeDomainRef = useRef<HTMLButtonElement | null>(null);
  const domainKey = `${component}/${activeDomain}`;
  const domainVisitRef = useRef({ key: domainKey, generation: 0 });
  if (domainVisitRef.current.key !== domainKey) {
    domainVisitRef.current = { key: domainKey, generation: domainVisitRef.current.generation + 1 };
  }
  const domainVisit = domainVisitRef.current.generation;
  const [domainRefresh, setDomainRefresh] = useState(0);
  const [domainRead, setDomainRead] = useState<{ key: string; status: 'loading' | 'ready' | 'error'; error?: string }>({ key: '', status: 'loading' });
  const domainReady = domainRead.key === domainKey && domainRead.status === 'ready';
  const domainError = domainRead.key === domainKey && domainRead.status === 'error' ? domainRead.error : '';


  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const response = await describeEffectiveSystemSettings();
      setSettings(Array.isArray(response.settings) ? response.settings : []);
    } catch (requestError) {
      const payload = toRequestErrorPayload(requestError);
      setError(typeof payload === 'string' ? payload : payload.message);
    } finally {
      setLoading(false);
      setDomainRefresh(value => value + 1);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const componentSettings = useMemo(
    () => settings.filter((setting) => setting.component === component),
    [component, settings],
  );
  const matchingSettings = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    return componentSettings.filter((setting) => {
      if (source && setting.source.kind !== source) return false;
      if (!normalizedQuery) return true;
      return [setting.key, setting.label, setting.description, setting.display_value]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(normalizedQuery));
    });
  }, [componentSettings, query, source]);

  const domains = useMemo(() => {
    const names = Array.from(new Set(componentSettings.map((setting) => setting.domain)));
    names.sort((left, right) => DOMAIN_ORDER[component].indexOf(left) - DOMAIN_ORDER[component].indexOf(right));
    return names.map((name) => ({
      name,
      total: componentSettings.filter((setting) => setting.domain === name).length,
      matched: matchingSettings.filter((setting) => setting.domain === name).length,
    }));
  }, [component, componentSettings, matchingSettings]);

  useEffect(() => {
    if (!domains.length) {
      if (!loading) {
        setActiveDomain('');
      }
      return;
    }
    const current = domains.find((item) => item.name === activeDomain);
    const firstMatched = domains.find((item) => item.matched > 0);
    if (!current || ((query.trim() || source) && current.matched === 0 && firstMatched)) {
      setActiveDomain(firstMatched?.name || domains[0].name);
    }
  }, [activeDomain, domains, loading, query, source]);

  useEffect(() => {
    const activeItem = activeDomainRef.current;
    const scroller = activeItem?.closest(`.${style.domainNav}`) as HTMLElement | null;
    if (!activeItem || !scroller) return;

    const itemRect = activeItem.getBoundingClientRect();
    const scrollerRect = scroller.getBoundingClientRect();
    if (itemRect.left < scrollerRect.left) {
      scroller.scrollLeft -= scrollerRect.left - itemRect.left;
    } else if (itemRect.right > scrollerRect.right) {
      scroller.scrollLeft += itemRect.right - scrollerRect.right;
    }
    if (itemRect.top < scrollerRect.top) {
      scroller.scrollTop -= scrollerRect.top - itemRect.top;
    } else if (itemRect.bottom > scrollerRect.bottom) {
      scroller.scrollTop += itemRect.bottom - scrollerRect.bottom;
    }
  }, [activeDomain]);

  const rows: SettingRow[] = matchingSettings
    .filter((setting) => setting.domain === activeDomain)
    .map((setting) => ({ ...setting, rowKey: setting.key }));
  const activeDomainSettings = componentSettings.filter((setting) => setting.domain === activeDomain);
  const environmentCount = componentSettings.filter((setting) => setting.source.kind === 'environment').length;
  const staticCount = componentSettings.filter((setting) => setting.source.kind === 'static_file').length;
  const defaultCount = componentSettings.filter((setting) => setting.source.kind === 'compiled_default').length;
  const dynamicCount = componentSettings.filter((setting) => setting.source.kind === 'dynamic_release').length;
  const componentCount = (target: SettingComponent) => settings.filter((setting) => setting.component === target).length;
  const activeDomainMeta = DOMAIN_META[activeDomain] || { label: activeDomain, description: '当前领域的有效配置。' };
  const activeDomainTotal = domains.find((item) => item.name === activeDomain)?.total || 0;
  const valueFor = useCallback((key: string, fallback = '') => {
    const setting = settings.find((item) => item.key === key);
    return typeof setting?.value === 'string' ? setting.value : (setting?.display_value || fallback);
  }, [settings]);
  const fallbackAgentProfile: AgentSystemProfile = {
    runtimeMode: valueFor('console.agent.runtime_mode', 'llm'),
    agentId: valueFor('console.agent.definition_id', 'pole-control-plane'),
    promptVersion: valueFor('console.agent.prompt_builtin_version', 'v1'),
    operatorInstructions: valueFor('console.agent.prompt_operator_instructions'),
    provider: valueFor('console.agent.model_provider', 'openai-compatible'),
    baseURL: valueFor('console.agent.model_base_url'),
    model: valueFor('console.agent.model_name', 'auto'),
    modelTimeout: valueFor('console.agent.model_timeout', '60s'),
    mcpEndpoint: valueFor('console.agent.mcp_endpoint', 'http://127.0.0.1:8090/ai/mcp/v1/sse'),
    mcpToolAllowlist: (() => {
      const setting = settings.find((item) => item.key === 'console.agent.mcp_tool_allowlist');
      return Array.isArray(setting?.value) ? setting.value.map(String) : [];
    })(),
    proposalTTL: valueFor('console.agent.proposal_ttl', '30m0s'),
    upstreamTimeout: valueFor('console.agent.upstream_timeout', '10s'),
  };
  const isAgentDomain = component === 'pole-console' && activeDomain === 'agent';
  const editableRows = activeDomainSettings.filter((setting) => setting.editable);
  const apiKeyConfigured = Boolean(settings.find((item) => item.key === 'console.agent.model_api_key')?.configured);
  const effectiveAgentProfile = agentDomain?.active?.values || fallbackAgentProfile;
  const draftAgentProfile = agentDomain?.draft?.values;
  const publishChanges = useMemo(() => {
    if (!draftAgentProfile) return [];
    const fields = [
      { label: 'LLM Gateway', before: effectiveAgentProfile.baseURL, after: draftAgentProfile.baseURL },
      { label: '模型', before: effectiveAgentProfile.model, after: draftAgentProfile.model },
      { label: '请求超时', before: effectiveAgentProfile.modelTimeout, after: draftAgentProfile.modelTimeout },
      { label: 'MCP Endpoint', before: effectiveAgentProfile.mcpEndpoint, after: draftAgentProfile.mcpEndpoint },
      { label: '工具白名单', before: effectiveAgentProfile.mcpToolAllowlist.join(', '), after: draftAgentProfile.mcpToolAllowlist.join(', ') },
      { label: '提案有效期', before: effectiveAgentProfile.proposalTTL, after: draftAgentProfile.proposalTTL },
      { label: '资源工具超时', before: effectiveAgentProfile.upstreamTimeout, after: draftAgentProfile.upstreamTimeout },
      { label: 'Agent 指令', before: effectiveAgentProfile.operatorInstructions, after: draftAgentProfile.operatorInstructions },
    ];
    return fields.filter((item) => item.before !== item.after);
  }, [draftAgentProfile, effectiveAgentProfile]);

  const publishDraft = async () => {
    if (!domainReady || !isAgentDomain || !agentDomain?.draft || publishing) return;
    const requestVisit = domainVisit;
    setPublishing(true);
    setPublishError('');
    try {
      const result = await publishAgentSystemDraft({
        draftRevision: agentDomain.draft.revision,
        description: publishDescription.trim(),
      });
      if (domainVisitRef.current.generation !== requestVisit) return;
      setAgentDomain(result);
      setPublishOpen(false);
      setPublishDescription('');
      await load();
    } catch (requestError) {
      if (domainVisitRef.current.generation !== requestVisit) return;
      const payload = toRequestErrorPayload(requestError);
      setPublishError(typeof payload === 'string' ? payload : payload.message);
    } finally {
      if (domainVisitRef.current.generation === requestVisit) setPublishing(false);
    }
  };

  useEffect(() => {
    setPublishing(false);
    setManagedPublishing(false);
    setPublishError('');
    setManagedPublishError('');
    setAgentPanelOpen(false);
    setManagedPanelOpen(false);
    setPublishOpen(false);
    setManagedPublishOpen(false);
  }, [domainKey]);

  useEffect(() => {
    let cancelled = false;
    setManagedDomain(undefined);
    setAgentDomain(undefined);
    setDomainRead({ key: domainKey, status: 'loading' });
    if (!activeDomain) return;
    const read = async () => {
      try {
        if (isAgentDomain) {
          const result = await getAgentSystemDomain();
          if (!cancelled) setAgentDomain(result);
        } else {
          const result = await getManagedSystemDomain(component, activeDomain);
          if (!cancelled) setManagedDomain(result);
        }
        if (!cancelled) setDomainRead({ key: domainKey, status: 'ready' });
      } catch (requestError) {
        const payload = toRequestErrorPayload(requestError);
        if (!cancelled) setDomainRead({ key: domainKey, status: 'error', error: typeof payload === 'string' ? payload : payload.message });
      }
    };
    void read();
    return () => { cancelled = true; };
  }, [activeDomain, component, domainKey, domainRefresh, isAgentDomain]);

  const publishManagedDraft = async () => {
    if (!domainReady || isAgentDomain || !managedDomain?.draft || managedPublishing) return;
    const requestVisit = domainVisit;
    setManagedPublishing(true);
    setManagedPublishError('');
    try {
      const result = await publishManagedSystemDraft(component, activeDomain, {
        draftRevision: managedDomain.draft.revision,
        description: managedPublishDescription.trim(),
      });
      if (domainVisitRef.current.generation !== requestVisit) return;
      setManagedDomain(result);
      setManagedPublishOpen(false);
      setManagedPublishDescription('');
      await load();
    } catch (requestError) {
      if (domainVisitRef.current.generation !== requestVisit) return;
      const payload = toRequestErrorPayload(requestError);
      setManagedPublishError(typeof payload === 'string' ? payload : payload.message);
    } finally {
      if (domainVisitRef.current.generation === requestVisit) setManagedPublishing(false);
    }
  };

  const pageHeader = (
    <ResourceHeader
      density="compact"
      placement="app-header"
      eyebrow="SYSTEM CONFIGURATION"
      title="系统配置"
      description="逐字段管理 Pole Server 与 Console 系统配置；自举项保持锁定，可管理项支持草稿、审阅、发布与明确的生效状态。"
      actions={(
        <Button variant="outline" icon={<RefreshIcon />} loading={loading} onClick={load}>
          刷新
        </Button>
      )}
    />
  );

  if (!loading && settings.length === 0 && error) {
    return (
      <div className={style.page}>
        {pageHeader}
        <section className={style.workspace}>
          <div className={style.loadFailure} role="alert">
            <strong>系统配置目录加载失败</strong>
            <span>{error}</span>
            <Button variant="outline" onClick={load}>重新加载</Button>
          </div>
        </section>
      </div>
    );
  }

  return (
    <div className={style.page}>
      {pageHeader}

      <section className={style.workspace}>
        {error && (
          <div className={style.refreshWarning} role="alert">
            <div>
              <strong>刷新失败，继续显示上一次有效配置</strong>
              <span>{error}</span>
            </div>
            <Button variant="outline" onClick={load}>重新加载</Button>
          </div>
        )}
        <div className={style.componentTabs} role="tablist" aria-label="选择配置组件">
          {(['pole-server', 'pole-console'] as SettingComponent[]).map((value) => (
            <Button
              key={value}
              variant="text"
              role="tab"
              aria-selected={component === value}
              className={component === value ? style.componentTabActive : style.componentTab}
              onClick={() => {
                setComponent(value);
                setActiveDomain('');
              }}
            >
              <span>{COMPONENT_LABEL[value]}</span>
              <small>{componentCount(value)} 项</small>
            </Button>
          ))}
        </div>

        {!isAgentDomain && (
          <>
            <div className={style.metricRail} aria-label="配置来源概览">
              <div><span>{COMPONENT_LABEL[component]} 配置项</span><strong>{componentSettings.length}</strong></div>
              <div><span>静态配置</span><strong>{staticCount}</strong></div>
              <div><span>环境变量</span><strong>{environmentCount}</strong></div>
              <div><span>编译默认值</span><strong>{defaultCount}</strong></div>
              <div><span>动态发布</span><strong>{dynamicCount}</strong></div>
            </div>

            <div className={style.readonlyNotice}>
              <strong>字段级管理</strong>
              <span>当前领域 {editableRows.length} 项可在页面编辑；自举、Secret 和部署拓扑字段保持锁定。重启级配置发布后会明确标记为待重启。</span>
            </div>
          </>
        )}

        <div className={`${style.configurationWorkspace} ${isAgentDomain ? style.agentConfigurationWorkspace : ''}`}>
          <nav className={style.domainNav} aria-label={`${COMPONENT_LABEL[component]} 配置领域`}>
            <div className={style.domainNavTitle}>配置领域</div>
            <div className={style.domainNavItems}>
              {domains.map((item) => {
                const meta = DOMAIN_META[item.name] || { label: item.name, description: '' };
                const selected = activeDomain === item.name;
                return (
                  <Button
                    key={item.name}
                    variant="text"
                    ref={selected ? activeDomainRef : undefined}
                    className={selected ? style.domainNavItemActive : style.domainNavItem}
                    aria-current={selected ? 'page' : undefined}
                    onClick={() => setActiveDomain(item.name)}
                  >
                    <span>{meta.label}</span>
                    <small>{query.trim() || source ? `${item.matched}/${item.total}` : item.total}</small>
                  </Button>
                );
              })}
            </div>
          </nav>

          <div className={`${style.domainContent} ${isAgentDomain ? style.agentDomainContent : ''}`}>
            {domainError && (
              <div className={style.refreshWarning} role="alert">
                <div><strong>草稿状态读取失败</strong><span>{domainError}。当前无法确认草稿，编辑与发布暂不可用。</span></div>
                <Button variant="outline" onClick={() => setDomainRefresh(value => value + 1)}>重试读取草稿</Button>
              </div>
            )}
            {isAgentDomain ? (
              <AgentConfigurationWorkspace
                domain={domainReady ? agentDomain : undefined}
                fallback={fallbackAgentProfile}
                apiKeyConfigured={apiKeyConfigured}
                loading={loading || (domainRead.status === 'loading' && domainRead.key === domainKey)}
                onEdit={() => { if (domainReady) setAgentPanelOpen(true); }}
                onPublish={() => {
                  if (!domainReady) return;
                  setPublishError('');
                  setPublishOpen(true);
                }}
              />
            ) : (
              <>
                <ResourceToolbar
                  title={activeDomainMeta.label || '配置领域'}
                  count={`${rows.length} / ${activeDomainTotal}`}
                  description={`${activeDomainMeta.description} 当前组件匹配 ${matchingSettings.length} 项。`}
                  filters={(
                    <QueryComposer
                      keyword={query}
                      keywordPlaceholder={`搜索 ${COMPONENT_LABEL[component]} 的配置键、说明或值`}
                      suggestions={componentSettings.map((item) => ({
                        label: item.label || item.key,
                        value: item.key,
                      }))}
                      fields={[
                        {
                          key: 'source',
                          label: '配置来源',
                          type: 'select',
                          options: sourceOptions,
                        },
                      ]}
                      values={{ source }}
                      onKeywordChange={setQuery}
                      onValuesChange={(values) => setSource(String(values.source || ''))}
                      onReset={() => {
                        setQuery('');
                        setSource('');
                      }}
                      mode="instant"
                      actions={(
                        <>
                          <Button
                            variant="outline"
                            disabled={!domainReady || !editableRows.length}
                            onClick={() => { if (domainReady) setManagedPanelOpen(true); }}
                          >
                            编辑 {editableRows.length} 项
                          </Button>
                          <Button
                            theme="primary"
                            disabled={!domainReady || !managedDomain?.draft}
                            onClick={() => {
                              setManagedPublishError('');
                              setManagedPublishOpen(true);
                            }}
                          >
                            {!domainReady ? (domainError ? '草稿状态未知' : '正在读取草稿') : managedDomain?.draft ? `审阅 r${managedDomain.draft.revision}` : '无待发布草稿'}
                          </Button>
                        </>
                      )}
                    />
                  )}
                />

                {domainReady && managedDomain && (
                  <div className={managedDomain.applyStatus === 'pending_restart' ? style.pendingRestart : style.managedDomainStatus}>
                    <div>
                      <strong>
                        {managedDomain.applyStatus === 'pending_restart'
                          ? `目标版本 r${managedDomain.active?.revision} 等待实例收敛`
                          : '当前使用启动配置'}
                      </strong>
                      <span>{managedDomain.applyMessage}</span>
                    </div>
                    {managedDomain.draft && <Tag variant="outline">草稿 r{managedDomain.draft.revision}</Tag>}
                  </div>
                )}

                <section className={style.tableSurface}>
                  <Table
                    data={rows}
                    columns={columns}
                    loading={loading || (domainRead.status === 'loading' && domainRead.key === domainKey)}
                    rowKey="rowKey"
                    tableLayout="fixed"
                    empty={(
                      <Empty
                        description={error || '当前领域没有匹配的配置项'}
                        action={error ? <Button variant="outline" onClick={load}>重新加载</Button> : undefined}
                      />
                    )}
                  />
                </section>
              </>
            )}
          </div>
        </div>
      </section>
      <AgentGatewayDraftPanel
        key={`agent-${domainVisit}`}
        visible={agentPanelOpen && domainReady && isAgentDomain}
        domain={agentDomain}
        fallback={fallbackAgentProfile}
        onClose={() => { if (domainVisitRef.current.generation === domainVisit) setAgentPanelOpen(false); }}
        onSaved={(next) => { if (domainVisitRef.current.generation === domainVisit) setAgentDomain(next); }}
      />
      <ManagedDomainDraftPanel
        key={`managed-${domainVisit}`}
        visible={managedPanelOpen && domainReady && !isAgentDomain}
        settings={activeDomainSettings}
        domain={managedDomain}
        onClose={() => { if (domainVisitRef.current.generation === domainVisit) setManagedPanelOpen(false); }}
        onSaved={(next) => { if (domainVisitRef.current.generation === domainVisit) setManagedDomain(next); }}
      />
      <Dialog
        visible={publishOpen}
        header="审阅并发布 Agent 配置"
        onClose={() => {
          setPublishError('');
          setPublishOpen(false);
        }}
        footer={(
          <>
            <Button variant="outline" onClick={() => { setPublishOpen(false); setAgentPanelOpen(true); }}>返回编辑</Button>
            <Button theme="primary" icon={<RocketIcon />} loading={publishing} disabled={!domainReady || !agentDomain?.draft} onClick={publishDraft}>
              发布 r{agentDomain?.draft?.revision || ''}
            </Button>
          </>
        )}
      >
        <div className={style.publishReview}>
          {publishError && (
            <div className={style.publishError} role="alert" tabIndex={-1}>
              <strong>Agent 配置发布失败</strong>
              <span>{publishError}</span>
              <Button variant="outline" onClick={publishDraft}>重试发布</Button>
            </div>
          )}
          <div className={style.publishTransition}>
            <div><span>当前生效</span><strong>{agentDomain?.active ? `r${agentDomain.active.revision}` : '静态启动配置'}</strong></div>
            <ArrowRightIcon />
            <div><span>发布目标</span><strong>{agentDomain?.draft ? `r${agentDomain.draft.revision}` : '—'}</strong></div>
          </div>
          <div className={style.publishChecklist}>
            <div><CheckCircleIcon /><span><strong>连接复验</strong><small>发布前重新验证 LLM Gateway 与 Pole MCP</small></span></div>
            <div><CheckCircleIcon /><span><strong>Secret 安全</strong><small>只使用版本引用，页面与审计日志不显示正文</small></span></div>
            <div><CheckCircleIcon /><span><strong>原子热更新</strong><small>失败时继续使用上一次有效配置</small></span></div>
          </div>
          <section className={style.publishDiff}>
            <header><strong>配置变化</strong><span>{publishChanges.length} 项</span></header>
            {publishChanges.length ? publishChanges.map((item) => (
              <div key={item.label}>
                <span>{item.label}</span>
                <del title={item.before || '未配置'}>{item.before || '未配置'}</del>
                <ArrowRightIcon />
                <ins title={item.after || '未配置'}>{item.after || '未配置'}</ins>
              </div>
            )) : <p>普通配置值没有变化，仅 Secret 版本可能更新。</p>}
            <div>
              <span>API Key</span>
              <del title={agentDomain?.active?.secret.configured ? `Pole Secret v${agentDomain.active.secret.version}` : '未配置'}>{agentDomain?.active?.secret.configured ? `Pole Secret v${agentDomain.active.secret.version}` : '未配置'}</del>
              <ArrowRightIcon />
              <ins title={agentDomain?.draft?.secret.configured ? `Pole Secret v${agentDomain.draft.secret.version}` : '未配置'}>{agentDomain?.draft?.secret.configured ? `Pole Secret v${agentDomain.draft.secret.version}` : '未配置'}</ins>
            </div>
          </section>
          <label>
            <span>发布备注 <small>可选</small></span>
            <Textarea value={publishDescription} onChange={setPublishDescription} rows={3} placeholder="说明本次 Agent 配置变更" />
          </label>
        </div>
      </Dialog>
      <Dialog
        visible={managedPublishOpen}
        header={`审阅并发布「${activeDomainMeta.label}」配置`}
        onClose={() => {
          setManagedPublishError('');
          setManagedPublishOpen(false);
        }}
        footer={(
          <>
            <Button variant="outline" onClick={() => setManagedPublishOpen(false)}>取消</Button>
            <Button
              theme="primary"
              loading={managedPublishing}
              disabled={!domainReady || !managedDomain?.draft}
              onClick={publishManagedDraft}
            >
              发布 r{managedDomain?.draft?.revision || ''}
            </Button>
          </>
        )}
      >
        <div className={style.managedPublishReview}>
          {managedPublishError && (
            <div className={style.publishError} role="alert" tabIndex={-1}>
              <strong>系统配置发布失败</strong>
              <span>{managedPublishError}</span>
              <Button variant="outline" onClick={publishManagedDraft}>重试发布</Button>
            </div>
          )}
          <div className={style.managedImpact}>
            <strong>发布不等于当前实例已生效</strong>
            <p>该领域包含重启级设置。发布后 Pole 会保存不可变目标版本并标记为“待重启”；当前有效值会继续单独展示，避免把 desired value 误报为 effective value。</p>
          </div>
          <section>
            <header><strong>字段差异</strong><span>{editableRows.length} 项受管理</span></header>
            {editableRows.map((setting) => {
              const before = setting.display_value || '未配置';
              const after = managedDomain?.draft?.values[setting.key];
              const displayAfter = Array.isArray(after) ? after.join(', ') : String(after ?? '未配置');
              return before !== displayAfter ? (
                <div key={setting.key}>
                  <span>{setting.label}</span>
                  <del title={before}>{before}</del>
                  <ArrowRightIcon />
                  <ins title={displayAfter}>{displayAfter}</ins>
                </div>
              ) : null;
            })}
          </section>
          <label>
            <span>发布备注 <small>可选</small></span>
            <Textarea
              value={managedPublishDescription}
              onChange={setManagedPublishDescription}
              rows={3}
              placeholder="说明本次系统配置变更和重启安排"
            />
          </label>
        </div>
      </Dialog>
    </div>
  );
}
