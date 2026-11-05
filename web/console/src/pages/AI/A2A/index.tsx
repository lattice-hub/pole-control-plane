import React, { memo, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Col,
  Drawer,
  Form,
  Input,
  Link,
  PrimaryTableProps,
  Row,
  Select,
  Space,
  Switch,
  Table,
  TableRowData,
  Tabs,
  Tag,
  TagInput,
  Textarea,
  Tooltip,
} from 'components/Fluent';
import EditorDrawer, { EditorDrawerActions } from 'components/EditorDrawer';
import type { FormProps, PageInfo } from 'components/Fluent';
import {
  AddIcon,
  DeleteIcon,
  RefreshIcon,
  ServerIcon,
} from 'components/Fluent/icons';
import { useNavigate, useSearchParams } from 'components/Router';

import Text from 'components/Text';
import AuthorizeInput from 'components/Authorize';
import { ConfirmOperationButton, OperationButton } from 'components/OperationButton';
import { ResourceHeader, ResourceToolbar } from 'components/ResourceLayout';
import QueryComposer, { QuerySnapshot } from 'components/QueryComposer';
import AILogicalResourceDirectory, {
  AIResourceView,
  AIResourceViewSwitch,
} from 'components/AILogicalResourceDirectory';
import {
  cleanA2ADetails,
  cleanA2APage,
  editorA2AAgent,
  getA2AAgentCard,
  listA2AAgents,
  listA2AAgentSkills,
  removeA2AAgents,
  resetA2AAgent,
  saveA2AAgent,
  selectA2A,
  updateA2AAgent,
} from 'modules/ai/a2a';
import { useAppDispatch, useAppSelector } from 'modules/store';
import {
  A2AAgent,
  A2AAgentInterface,
  A2AAgentSkill,
  bindA2AAgentDefinition,
  createA2AAgentDefinition,
  deleteA2AAgentDefinition,
  describeA2AAgentDefinitionEnvironments,
  describeA2AAgentDefinitions,
  describeAllA2AAgents,
} from 'services/a2a';
import { PolicySourceType } from 'services/auth_policy';
import { Op } from 'services/types';
import ResourceNameLink from 'components/ResourceNameLink';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import style from './index.module.less';

const { FormItem } = Form;
const { TabPanel } = Tabs;

type A2ADetailTab = 'card' | 'skills' | 'edit';

const protocolOptions = [
  { label: 'JSON-RPC', value: 'jsonrpc' },
  { label: 'HTTP+JSON', value: 'http+json' },
  { label: 'gRPC', value: 'grpc' },
];

const backendOptions = [
  { label: 'Pole 注册服务', value: 'service' },
  { label: '自定义地址', value: 'address' },
];

const fetchStatusOptions = [
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '未拉取', value: 'pending' },
];

const defaultInterface = (): A2AAgentInterface => ({
  url: '',
  protocol_binding: 'jsonrpc',
  protocol_version: '0.3.0',
  tenant: '',
});

const defaultSkill = (): A2AAgentSkill => ({
  skill_id: '',
  name: '',
  description: '',
  tags: [],
  examples: [],
  input_modes: ['text/plain'],
  output_modes: ['text/plain'],
  security_requirements_json: '',
});

function tagList(value?: string[]) {
  return Array.isArray(value) ? value.filter(Boolean) : [];
}

export function jsonPreview(value: unknown) {
  if (!value) return '';
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2);
    } catch {
      return value;
    }
  }
  return JSON.stringify(value, null, 2);
}

export function parseJSONValue(value: unknown): Record<string, any> | undefined {
  if (!value) return undefined;
  if (typeof value === 'string') {
    try {
      const parsed = JSON.parse(value);
      return parsed && typeof parsed === 'object' ? parsed : undefined;
    } catch {
      return undefined;
    }
  }
  return typeof value === 'object' ? value as Record<string, any> : undefined;
}

function arrayValue(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.map((item) => String(item)).filter(Boolean);
}

function cardField(card: Record<string, any> | undefined, key: string, fallback = '-') {
  const value = card?.[key];
  return value === undefined || value === null || value === '' ? fallback : String(value);
}

function isHttpURL(value: unknown) {
  return typeof value === 'string' && /^https?:\/\//.test(value);
}

function parseMetadata(raw?: string): Record<string, string> | undefined {
  if (!raw) return undefined;
  return raw.split('\n').reduce<Record<string, string>>((acc, line) => {
    const index = line.indexOf('=');
    if (index <= 0) return acc;
    const key = line.slice(0, index).trim();
    const value = line.slice(index + 1).trim();
    if (key) acc[key] = value;
    return acc;
  }, {});
}

function stringifyMetadata(metadata?: Record<string, string>) {
  return metadata ? Object.entries(metadata).map(([key, value]) => `${key}=${value}`).join('\n') : '';
}

export function protocolLabel(value?: string) {
  return protocolOptions.find((item) => item.value === value)?.label || value || '-';
}

export function protocolTheme(value?: string) {
  if (value === 'jsonrpc') return 'primary';
  if (value === 'http+json') return 'success';
  if (value === 'grpc') return 'warning';
  return 'default';
}

export function backendType(agent?: A2AAgent) {
  if (agent?.backend_type === 'url' || agent?.backend_type === 'external') return 'address';
  if (agent?.backend_type) return agent.backend_type;
  if (agent?.backend_service_namespace || agent?.backend_service_name) return 'service';
  if (agent?.backend_address) return 'address';
  return '';
}

export function backendTypeLabel(value?: string) {
  const normalized = value === 'url' || value === 'external' ? 'address' : value;
  return backendOptions.find((item) => item.value === normalized)?.label || '-';
}

export function backendLabel(agent?: A2AAgent) {
  const type = backendType(agent);
  if (type === 'service') {
    return `${agent?.backend_service_namespace || agent?.namespace || '-'}/${agent?.backend_service_name || agent?.name || '-'}`;
  }
  if (type === 'address') return agent?.backend_address || agent?.preferred_interface_url || '-';
  return agent?.preferred_interface_url || '-';
}

export function backendServiceRef(agent?: A2AAgent) {
  if (backendType(agent) !== 'service') return undefined;
  const namespace = agent?.backend_service_namespace || agent?.namespace || '';
  const name = agent?.backend_service_name || agent?.name || '';
  if (!namespace || !name) return undefined;
  return { namespace, name };
}

function fetchStatusTheme(value?: string) {
  if (value === 'success') return 'success';
  if (value === 'failed') return 'danger';
  return 'default';
}

function fetchStatusLabel(value?: string) {
  if (value === 'success') return '成功';
  if (value === 'failed') return '失败';
  return '未拉取';
}

function sourceTypeLabel(value?: string) {
  if (value === 'manual') return '手动';
  if (value === 'well-known') return 'Well-known';
  if (value === 'service') return 'Pole 服务';
  if (value === 'fetch') return '远程';
  return value || '-';
}

function sourceTypeTheme(value?: string) {
  if (value === 'well-known' || value === 'fetch') return 'primary';
  if (value === 'service') return 'success';
  return 'default';
}

function normalizeInterfaces(items: A2AAgentInterface[]) {
  return items
    .filter((item) => item.url || item.protocol_binding || item.protocol_version || item.tenant)
    .map((item) => ({
      ...item,
      protocol_binding: item.protocol_binding || 'jsonrpc',
      protocol_version: item.protocol_version || '0.3.0',
    }));
}

function normalizeSkills(items: A2AAgentSkill[]) {
  return items
    .filter((item) => item.name || item.skill_id)
    .map((item) => ({
      ...item,
      skill_id: item.skill_id || item.name,
      tags: tagList(item.tags),
      examples: tagList(item.examples),
      input_modes: tagList(item.input_modes),
      output_modes: tagList(item.output_modes),
    }));
}

const capabilityTags = (agent: A2AAgent) => (
  <div className={style.capabilityTagList}>
    {agent.streaming && <Tag theme="success" variant="light-outline">Streaming</Tag>}
    {agent.push_notifications && <Tag theme="warning" variant="light-outline">Push</Tag>}
    {agent.extended_agent_card && <Tag theme="primary" variant="light-outline">Extended</Tag>}
    {!agent.streaming && !agent.push_notifications && !agent.extended_agent_card && <Text>-</Text>}
  </div>
);

const agentColumns = (
  operateAgent: (op: Op | 'detail' | 'skills' | 'card' | 'authorize', row?: TableRowData) => void,
  goBackendService: (agent?: A2AAgent) => void,
  goEnvironment: (namespace: string) => void,
): PrimaryTableProps['columns'] => [
  {
    colKey: 'name',
    title: 'A2A Agent',
    fixed: 'left',
    width: 340,
    cell: ({ row }) => (
      <ResourceNameLink
        className={style.agentNameLink}
        name={row.name}
        onClick={() => operateAgent('detail', row)}
      />
    ),
  },
  {
    colKey: 'namespace',
    title: '环境空间',
    width: 160,
    cell: ({ row }) => (
      <Link theme="primary" onClick={() => goEnvironment(String(row.namespace || ''))}>
        {row.namespace || '-'}
      </Link>
    ),
  },
  {
    colKey: 'endpoint',
    title: '接入',
    ellipsis: true,
    width: 240,
    cell: ({ row }) => {
      const agent = row as A2AAgent;
      const serviceRef = backendServiceRef(agent);
      return (
        <div className={style.compactCell}>
          <Text>{backendTypeLabel(backendType(agent))}</Text>
          {serviceRef ? (
            <Link className={style.backendLink} theme="primary" onClick={() => goBackendService(agent)}>
              {backendLabel(agent)}
            </Link>
          ) : (
            <span>{backendLabel(agent)}</span>
          )}
        </div>
      );
    },
  },
  {
    colKey: 'owner',
    title: '归属',
    width: 160,
    cell: ({ row }) => (
      <div className={style.compactCell}>
        <Text>{row.business || '-'}</Text>
        <span>{row.department || row.provider_organization || '-'}</span>
      </div>
    ),
  },
  {
    colKey: 'capabilities',
    title: '能力',
    width: 250,
    cell: ({ row }) => capabilityTags(row as A2AAgent),
  },
  {
    colKey: 'skills',
    title: '技能数',
    width: 88,
    cell: ({ row }) => (
      <Link className={style.skillCountLink} theme="primary" onClick={() => operateAgent('skills', row)}>
        {Array.isArray(row.skills) ? row.skills.length : 0} 个
      </Link>
    ),
  },
  {
    colKey: 'last_fetch_status',
    title: '来源',
    width: 124,
    cell: ({ row }) => {
      const agent = row as A2AAgent;
      const sourceType = agent.source_type || 'manual';
      const fetchStatus = agent.last_fetch_status || 'pending';
      const showFetchStatus = fetchStatus !== 'success' && (sourceType !== 'manual' || fetchStatus === 'failed');
      return (
        <div className={style.sourceCell}>
          <Tag theme={sourceTypeTheme(sourceType) as any} variant="light-outline">
            {sourceTypeLabel(sourceType)}
          </Tag>
          {showFetchStatus && (
            <Tag theme={fetchStatusTheme(fetchStatus) as any} variant="light">
              {fetchStatusLabel(fetchStatus)}
            </Tag>
          )}
        </div>
      );
    },
  },
  {
    colKey: 'time',
    title: '最近修改',
    width: 210,
    cell: ({ row }) => (
      <div className={style.compactCell}>
        <Text>{row.mtime || '-'}</Text>
        <span>创建 {row.ctime || '-'}</span>
      </div>
    ),
  },
  {
    colKey: 'action',
    title: '操作',
    fixed: 'right',
    width: 132,
    cell: ({ row }) => (
      <Space className={style.actionCell} size={4}>
        <OperationButton action="viewEdit" onClick={() => operateAgent('detail', row)} />
        <OperationButton action="authorize" onClick={() => operateAgent('authorize', row)} />
        <ConfirmOperationButton action="delete" confirmContent="确认删除该 A2A Agent 吗" onConfirm={() => operateAgent('delete', row)} />
      </Space>
    ),
  },
];

export const AgentCardView: React.FC<{ card?: Record<string, any>; loading: boolean }> = ({ card, loading }) => {
  if (loading) {
    return <div className={style.cardEmpty}>Agent Card 加载中...</div>;
  }

  if (!card) {
    return <div className={style.cardEmpty}>暂无可视化 Agent Card</div>;
  }

  const provider = card.provider && typeof card.provider === 'object' ? card.provider : {};
  const capabilities = card.capabilities && typeof card.capabilities === 'object' ? card.capabilities : {};
  const skills = Array.isArray(card.skills) ? card.skills : [];
  const url = card.url || card.endpoint || '-';
  const security = Array.isArray(card.security) ? card.security : [];
  const providerName = provider.organization ? String(provider.organization) : '-';
  const providerURL = provider.url ? String(provider.url) : '-';

  return (
    <div className={style.cardVisual}>
      <section className={style.cardHero}>
        <div>
          <h4>{cardField(card, 'name')}</h4>
          <p>{cardField(card, 'description')}</p>
        </div>
        <Space size={8}>
          <Tag theme="primary" variant="light">A2A {cardField(card, 'protocolVersion')}</Tag>
          {card.version && <Tag variant="outline">v{String(card.version)}</Tag>}
        </Space>
      </section>

      <section className={style.cardInfoGrid}>
        <div>
          <span>访问地址</span>
          <strong>
            {isHttpURL(url) ? (
              <Link href={String(url)} target="_blank">
                {String(url)}
              </Link>
            ) : String(url)}
          </strong>
        </div>
        <div>
          <span>Provider</span>
          <strong>{providerName}</strong>
        </div>
        <div>
          <span>Provider URL</span>
          <strong>
            {isHttpURL(providerURL) ? (
              <Link href={providerURL} target="_blank">
                {providerURL}
              </Link>
            ) : providerURL}
          </strong>
        </div>
        <div>
          <span>默认输入/输出</span>
          <strong>{arrayValue(card.defaultInputModes).join(', ') || '-'} / {arrayValue(card.defaultOutputModes).join(', ') || '-'}</strong>
        </div>
      </section>

      <section className={style.cardSection}>
        <div className={style.cardSectionTitle}>能力</div>
        <Space size={8} breakLine>
          {capabilities.streaming && <Tag theme="success" variant="light-outline">Streaming</Tag>}
          {capabilities.pushNotifications && <Tag theme="warning" variant="light-outline">Push Notification</Tag>}
          {capabilities.stateTransitionHistory && <Tag theme="primary" variant="light-outline">State History</Tag>}
          {security.length > 0 && <Tag variant="outline">Security: {security.length}</Tag>}
          {!capabilities.streaming && !capabilities.pushNotifications && !capabilities.stateTransitionHistory && security.length === 0 && <Text>-</Text>}
        </Space>
      </section>

      <section className={style.cardSection}>
        <div className={style.cardSectionTitle}>
          <span>Skills</span>
          <Tag variant="light">{skills.length}</Tag>
        </div>
        {skills.length > 0 ? (
          <div className={style.cardSkillList}>
            {skills.map((skill: any, index: number) => {
              const tags = arrayValue(skill.tags);
              return (
                <div className={style.cardSkillItem} key={skill.id || skill.name || index}>
                  <div className={style.cardSkillMain}>
                    <div>
                      <strong>{skill.name || skill.id || `Skill ${index + 1}`}</strong>
                      <span>{skill.id || '-'}</span>
                    </div>
                    <p>{skill.description || '-'}</p>
                  </div>
                  <Space size={6} breakLine>
                    {tags.slice(0, 5).map((tag) => <Tag key={tag} variant="outline">{tag}</Tag>)}
                    {tags.length > 5 && <Tag variant="outline">+{tags.length - 5}</Tag>}
                  </Space>
                </div>
              );
            })}
          </div>
        ) : (
          <Text>-</Text>
        )}
      </section>
    </div>
  );
};

function skillKey(skill: A2AAgentSkill, index: number) {
  return skill.id || skill.skill_id || skill.name || `skill-${index}`;
}

export const AgentSkillsView: React.FC<{ skills: A2AAgentSkill[]; loading: boolean }> = ({ skills, loading }) => {
  const [activeKey, setActiveKey] = useState('');

  useEffect(() => {
    if (!skills.length) {
      setActiveKey('');
      return;
    }
    setActiveKey((current) => (skills.some((skill, index) => skillKey(skill, index) === current) ? current : skillKey(skills[0], 0)));
  }, [skills]);

  if (loading) {
    return <div className={style.cardEmpty}>技能加载中...</div>;
  }

  if (!skills.length) {
    return <div className={style.cardEmpty}>暂无技能</div>;
  }

  const activeSkill = skills.find((skill, index) => skillKey(skill, index) === activeKey) || skills[0];
  const activeTags = tagList(activeSkill.tags);
  const inputModes = tagList(activeSkill.input_modes);
  const outputModes = tagList(activeSkill.output_modes);
  const examples = tagList(activeSkill.examples);

  return (
    <div className={style.skillBrowser}>
      <aside className={style.skillCatalog}>
        <div className={style.skillCatalogHeader}>
          <strong>技能目录</strong>
          <span>{skills.length} 个技能</span>
        </div>
        <div className={style.skillCatalogList}>
          {skills.map((skill, index) => {
            const key = skillKey(skill, index);
            const tags = tagList(skill.tags);
            return (
              <Button
                variant="text"
                key={key}
                className={key === activeKey ? style.skillCatalogItemActive : style.skillCatalogItem}
                type="button"
                onClick={() => setActiveKey(key)}
              >
                <div className={style.skillCatalogMain}>
                  <strong>{skill.name || skill.skill_id || `Skill ${index + 1}`}</strong>
                  <span>{skill.skill_id || '-'}</span>
                </div>
                <div className={style.skillCatalogTags}>
                  {tags.slice(0, 3).map((tag) => <Tag key={tag} variant="outline">{tag}</Tag>)}
                  {tags.length > 3 && <Tag variant="outline">+{tags.length - 3}</Tag>}
                </div>
              </Button>
            );
          })}
        </div>
      </aside>

      <section className={style.skillDetailPanel}>
        <div className={style.skillDetailHeader}>
          <div>
            <span>Skill</span>
            <h4>{activeSkill.name || activeSkill.skill_id || '-'}</h4>
            <p>{activeSkill.description || '暂无描述'}</p>
          </div>
          {activeSkill.skill_id && <Tag theme="primary" variant="light">{activeSkill.skill_id}</Tag>}
        </div>

        <div className={style.skillDetailGrid}>
          <div>
            <span>标签</span>
            <Space size={6} breakLine>
              {activeTags.length ? activeTags.map((tag) => <Tag key={tag} variant="outline">{tag}</Tag>) : <Text>-</Text>}
            </Space>
          </div>
          <div>
            <span>输入</span>
            <strong>{inputModes.join(', ') || '-'}</strong>
          </div>
          <div>
            <span>输出</span>
            <strong>{outputModes.join(', ') || '-'}</strong>
          </div>
        </div>

        {examples.length > 0 && (
          <div className={style.skillDetailSection}>
            <strong>示例</strong>
            <div className={style.skillExampleList}>
              {examples.map((example) => <span key={example}>{example}</span>)}
            </div>
          </div>
        )}

        {activeSkill.security_requirements_json && (
          <div className={style.skillDetailSection}>
            <strong>安全声明</strong>
            <pre>{activeSkill.security_requirements_json}</pre>
          </div>
        )}
      </section>
    </div>
  );
};

export const A2AEditor: React.FC<{
  op: Op;
  visible: boolean;
  closeDrawer: () => void;
  embedded?: boolean;
  environment?: string;
}> = ({ op, visible, closeDrawer, embedded = false, environment = '' }) => {
  const [form] = Form.useForm();
  const dispatch = useAppDispatch();
  const { editAgent } = useAppSelector(selectA2A);
  const [interfaces, setInterfaces] = useState<A2AAgentInterface[]>([defaultInterface()]);
  const [skills, setSkills] = useState<A2AAgentSkill[]>([defaultSkill()]);
  const initialValues = useMemo(() => ({
    name: editAgent?.name || '',
    namespace: editAgent?.namespace || environment,
    visibility: editAgent?.visibility || 'public',
    description: editAgent?.description || '',
    version: editAgent?.version || '',
    protocol_version: editAgent?.protocol_version || '0.3.0',
    provider_organization: editAgent?.provider_organization || '',
    provider_url: editAgent?.provider_url || '',
    documentation_url: editAgent?.documentation_url || '',
    icon_url: editAgent?.icon_url || '',
    business: editAgent?.business || '',
    department: editAgent?.department || '',
    backend_type: editAgent?.backend_type || 'service',
    backend_service_namespace: editAgent?.backend_service_namespace || environment,
    backend_service_name: editAgent?.backend_service_name || '',
    backend_address: editAgent?.backend_address || '',
    preferred_interface_url: editAgent?.preferred_interface_url || '',
    preferred_protocol_binding: editAgent?.preferred_protocol_binding || 'jsonrpc',
    preferred_protocol_version: editAgent?.preferred_protocol_version || '0.3.0',
    streaming: !!editAgent?.streaming,
    push_notifications: !!editAgent?.push_notifications,
    extended_agent_card: !!editAgent?.extended_agent_card,
    source_type: editAgent?.source_type || 'manual',
    source_url: editAgent?.source_url || '',
    last_fetch_status: editAgent?.last_fetch_status || 'pending',
    raw_card_json: jsonPreview(editAgent?.raw_card_json),
    metadata_text: stringifyMetadata(editAgent?.metadata),
  }), [editAgent, environment]);
  const initialInterfaces = useMemo(
    () => editAgent?.interfaces?.length ? editAgent.interfaces : [defaultInterface()],
    [editAgent],
  );
  const initialSkills = useMemo(
    () => editAgent?.skills?.length ? editAgent.skills : [defaultSkill()],
    [editAgent],
  );

  useEffect(() => {
    if (!visible) return;
    form.setFieldsValue(initialValues);
    setInterfaces(initialInterfaces);
    setSkills(initialSkills);
  }, [visible, initialValues, initialInterfaces, initialSkills]);

  const resetEditor = () => {
    form.setFieldsValue(initialValues);
    setInterfaces(initialInterfaces);
    setSkills(initialSkills);
  };

  const updateInterface = (index: number, patch: Partial<A2AAgentInterface>) => {
    setInterfaces((prev) => prev.map((item, idx) => (idx === index ? { ...item, ...patch } : item)));
  };

  const updateSkill = (index: number, patch: Partial<A2AAgentSkill>) => {
    setSkills((prev) => prev.map((item, idx) => (idx === index ? { ...item, ...patch } : item)));
  };

  const onSubmit: FormProps['onSubmit'] = async (e) => {
    if (e.validateResult !== true) return;

    const data: A2AAgent = {
      id: editAgent?.id,
      name: form.getFieldValue('name') as string,
      namespace: environment || form.getFieldValue('namespace') as string,
      visibility: form.getFieldValue('visibility') as string,
      description: form.getFieldValue('description') as string,
      version: form.getFieldValue('version') as string,
      protocol_version: form.getFieldValue('protocol_version') as string,
      provider_organization: form.getFieldValue('provider_organization') as string,
      provider_url: form.getFieldValue('provider_url') as string,
      documentation_url: form.getFieldValue('documentation_url') as string,
      icon_url: form.getFieldValue('icon_url') as string,
      business: form.getFieldValue('business') as string,
      department: form.getFieldValue('department') as string,
      backend_type: form.getFieldValue('backend_type') as string,
      backend_service_namespace: environment || form.getFieldValue('backend_service_namespace') as string,
      backend_service_name: form.getFieldValue('backend_service_name') as string,
      backend_address: form.getFieldValue('backend_address') as string,
      preferred_interface_url: form.getFieldValue('preferred_interface_url') as string,
      preferred_protocol_binding: form.getFieldValue('preferred_protocol_binding') as string,
      preferred_protocol_version: form.getFieldValue('preferred_protocol_version') as string,
      streaming: !!form.getFieldValue('streaming'),
      push_notifications: !!form.getFieldValue('push_notifications'),
      extended_agent_card: !!form.getFieldValue('extended_agent_card'),
      source_type: form.getFieldValue('source_type') as string,
      source_url: form.getFieldValue('source_url') as string,
      last_fetch_status: form.getFieldValue('last_fetch_status') as string,
      raw_card_json: form.getFieldValue('raw_card_json') as string,
      metadata: parseMetadata(form.getFieldValue('metadata_text') as string),
      interfaces: normalizeInterfaces(interfaces),
      skills: normalizeSkills(skills),
      flag: editAgent?.flag,
    };

    const result = op === 'edit'
      ? await dispatch(updateA2AAgent({ param: data }))
      : await dispatch(saveA2AAgent({ param: data }));

    if (result.meta.requestStatus !== 'fulfilled') {
      openErrNotification('请求错误', result.payload as string);
      return;
    }
    openInfoNotification('请求成功', op === 'edit' ? '修改 A2A Agent 成功' : '创建 A2A Agent 成功');
    closeDrawer();
  };

  const editorContent = (
      <Form form={form} layout="vertical" onSubmit={onSubmit} onReset={resetEditor}>
        <Tabs defaultValue="base">
          <TabPanel value="base" label="基础信息">
            <Row gutter={16}>
              <Col span={6}>
                <FormItem label="名称" name="name" rules={[
                  { required: true, message: '请输入 A2A Agent 名称' },
                  { pattern: /^[a-zA-Z0-9._-]+$/, message: '只允许数字、英文字母、.、-、_' },
                ]}
                >
                  <Input readonly={op === 'edit'} placeholder="例如 order-agent" />
                </FormItem>
              </Col>
              <Col span={6}>
                <FormItem label="环境空间" name="namespace" rules={[{ required: true, message: '请输入环境空间' }]}>
                  <Input readonly={op === 'edit' || Boolean(environment)} placeholder="例如 default" />
                </FormItem>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={6}>
                <FormItem label="版本" name="version">
                  <Input placeholder="例如 1.0.0" />
                </FormItem>
              </Col>
              <Col span={6}>
                <FormItem label="A2A 协议版本" name="protocol_version">
                  <Input placeholder="例如 0.3.0" />
                </FormItem>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={6}>
                <FormItem label="业务" name="business">
                  <Input />
                </FormItem>
              </Col>
              <Col span={6}>
                <FormItem label="部门" name="department">
                  <Input />
                </FormItem>
              </Col>
            </Row>
            <FormItem label="描述" name="description">
              <Textarea autosize={{ minRows: 2, maxRows: 4 }} />
            </FormItem>
            <Row gutter={16}>
              <Col span={6}>
                <FormItem label="Provider 组织" name="provider_organization">
                  <Input />
                </FormItem>
              </Col>
              <Col span={6}>
                <FormItem label="Provider URL" name="provider_url">
                  <Input />
                </FormItem>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={6}>
                <FormItem label="文档地址" name="documentation_url">
                  <Input />
                </FormItem>
              </Col>
              <Col span={6}>
                <FormItem label="图标地址" name="icon_url">
                  <Input />
                </FormItem>
              </Col>
            </Row>
          </TabPanel>

          <TabPanel value="access" label="接入与能力">
            <Row gutter={16}>
              <Col span={4}>
                <FormItem label="首选协议" name="preferred_protocol_binding">
                  <Select options={protocolOptions} />
                </FormItem>
              </Col>
              <Col span={4}>
                <FormItem label="首选协议版本" name="preferred_protocol_version">
                  <Input />
                </FormItem>
              </Col>
              <Col span={4}>
                <FormItem label="首选访问地址" name="preferred_interface_url">
                  <Input />
                </FormItem>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={4}>
                <FormItem label="后端类型" name="backend_type">
                  <Select options={backendOptions} />
                </FormItem>
              </Col>
              <Col span={4}>
                <FormItem label="后端服务环境空间" name="backend_service_namespace">
                  <Input readonly={Boolean(environment)} placeholder="service 模式使用" />
                </FormItem>
              </Col>
              <Col span={4}>
                <FormItem label="后端服务名" name="backend_service_name">
                  <Input placeholder="service 模式使用" />
                </FormItem>
              </Col>
            </Row>
            <FormItem label="后端地址" name="backend_address">
              <Input placeholder="URL 或外部系统地址" />
            </FormItem>
            <Row gutter={16}>
              <Col span={4}>
                <FormItem label="Streaming 能力" name="streaming">
                  <Switch size="large" label={['支持', '不支持']} />
                </FormItem>
              </Col>
              <Col span={4}>
                <FormItem label="Push 能力" name="push_notifications">
                  <Switch size="large" label={['支持', '不支持']} />
                </FormItem>
              </Col>
              <Col span={4}>
                <FormItem label="Extended Card" name="extended_agent_card">
                  <Switch size="large" label={['开启', '关闭']} />
                </FormItem>
              </Col>
            </Row>
            <div className={style.sectionHeader}>
              <Text>访问接口</Text>
              <Button size="small" variant="outline" icon={<AddIcon />} onClick={() => setInterfaces((prev) => [...prev, defaultInterface()])}>
                添加接口
              </Button>
            </div>
            {interfaces.map((item, index) => (
              <div className={style.inlineBlock} key={`${item.id || 'new'}-${index}`}>
                <Row gutter={12}>
                  <Col span={5}>
                    <Input aria-label={`第 ${index + 1} 个接口 URL`} value={item.url} placeholder="接口 URL" onChange={(value) => updateInterface(index, { url: value as string })} />
                  </Col>
                  <Col span={3}>
                    <Select aria-label={`第 ${index + 1} 个接口协议`} value={item.protocol_binding} options={protocolOptions} onChange={(value) => updateInterface(index, { protocol_binding: value as string })} />
                  </Col>
                  <Col span={2}>
                    <Input aria-label={`第 ${index + 1} 个接口版本`} value={item.protocol_version} placeholder="版本" onChange={(value) => updateInterface(index, { protocol_version: value as string })} />
                  </Col>
                  <Col span={1}>
                    <Input aria-label={`第 ${index + 1} 个接口租户`} value={item.tenant} placeholder="租户" onChange={(value) => updateInterface(index, { tenant: value as string })} />
                  </Col>
                  <Col span={1}>
                    <Button
                      aria-label={`删除第 ${index + 1} 个接口`}
                      shape="square"
                      variant="text"
                      disabled={interfaces.length === 1}
                      onClick={() => setInterfaces((prev) => prev.filter((_, idx) => idx !== index))}
                    >
                      <DeleteIcon />
                    </Button>
                  </Col>
                </Row>
              </div>
            ))}
          </TabPanel>

          <TabPanel value="skills" label="技能">
            <div className={style.sectionHeader}>
              <Text>技能列表</Text>
              <Button size="small" variant="outline" icon={<AddIcon />} onClick={() => setSkills((prev) => [...prev, defaultSkill()])}>
                添加技能
              </Button>
            </div>
            {skills.map((skill, index) => (
              <div className={style.skillBlock} key={`${skill.id || 'new'}-${index}`}>
                <Row gutter={12}>
                  <Col span={4}>
                    <Input aria-label={`第 ${index + 1} 个技能 ID`} value={skill.skill_id} placeholder="Skill ID" onChange={(value) => updateSkill(index, { skill_id: value as string })} />
                  </Col>
                  <Col span={4}>
                    <Input aria-label={`第 ${index + 1} 个技能名称`} value={skill.name} placeholder="技能名" onChange={(value) => updateSkill(index, { name: value as string })} />
                  </Col>
                  <Col span={3}>
                    <TagInput aria-label={`第 ${index + 1} 个技能标签`} value={tagList(skill.tags)} placeholder="标签" onChange={(value) => updateSkill(index, { tags: value as string[] })} />
                  </Col>
                  <Col span={1}>
                    <Button
                      aria-label={`删除第 ${index + 1} 个技能`}
                      shape="square"
                      variant="text"
                      disabled={skills.length === 1}
                      onClick={() => setSkills((prev) => prev.filter((_, idx) => idx !== index))}
                    >
                      <DeleteIcon />
                    </Button>
                  </Col>
                </Row>
                <Row gutter={12}>
                  <Col span={4}>
                    <TagInput aria-label={`第 ${index + 1} 个技能输入模式`} value={tagList(skill.input_modes)} placeholder="输入模式" onChange={(value) => updateSkill(index, { input_modes: value as string[] })} />
                  </Col>
                  <Col span={4}>
                    <TagInput aria-label={`第 ${index + 1} 个技能输出模式`} value={tagList(skill.output_modes)} placeholder="输出模式" onChange={(value) => updateSkill(index, { output_modes: value as string[] })} />
                  </Col>
                  <Col span={4}>
                    <TagInput aria-label={`第 ${index + 1} 个技能示例`} value={tagList(skill.examples)} placeholder="示例" onChange={(value) => updateSkill(index, { examples: value as string[] })} />
                  </Col>
                </Row>
                <Textarea aria-label={`第 ${index + 1} 个技能描述`} value={skill.description} autosize={{ minRows: 1, maxRows: 3 }} placeholder="技能描述" onChange={(value) => updateSkill(index, { description: value as string })} />
              </div>
            ))}
          </TabPanel>

          <TabPanel value="source" label="来源与 Card">
            <Row gutter={16}>
              <Col span={4}>
                <FormItem label="来源类型" name="source_type">
                  <Input placeholder="manual / fetch" />
                </FormItem>
              </Col>
              <Col span={4}>
                <FormItem label="拉取状态" name="last_fetch_status">
                  <Select options={fetchStatusOptions} />
                </FormItem>
              </Col>
              <Col span={4}>
                <FormItem label="来源地址" name="source_url">
                  <Input />
                </FormItem>
              </Col>
            </Row>
            <FormItem label="元数据" name="metadata_text">
              <Textarea placeholder="每行一个 key=value" autosize={{ minRows: 3, maxRows: 6 }} />
            </FormItem>
            <FormItem label="Agent Card JSON" name="raw_card_json">
              <Textarea className={style.codeText} autosize={{ minRows: 8, maxRows: 14 }} placeholder="可选，保存官方 Agent Card 原文" />
            </FormItem>
          </TabPanel>
        </Tabs>
        {embedded && (
          <FormItem className={style.formAction}>
            <Space>
              <Button type="submit" theme="primary">提交</Button>
              <Button type="reset" theme="default">重置</Button>
            </Space>
          </FormItem>
        )}
      </Form>
  );

  if (embedded) return editorContent;

  return (
    <EditorDrawer
      bodyClassName={style.editorBody}
      width="workspace"
      title={op === 'edit' ? '编辑 A2A Agent' : '创建 A2A Agent'}
      description="按能力、协议、接口与来源分区维护 Agent Card。"
      footer={(
        <EditorDrawerActions
          onCancel={closeDrawer}
          onSubmit={() => form.submit({ showErrorMessage: true })}
          submitText={op === 'edit' ? '保存修改' : '创建 Agent'}
        />
      )}
      visible={visible}
      onClose={closeDrawer}
    >
      {editorContent}
    </EditorDrawer>
  );
};

export default memo(() => {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const requestedNamespace = searchParams.get('namespace') || '';
  const [resourceView, setResourceView] = useState<AIResourceView>(requestedNamespace ? 'environment' : 'logical');
  const { datas, loading, page, limit, total, editAgent, skills, skillsLoading, card, cardLoading } = useAppSelector(selectA2A);
  const [query, setQuery] = useState({
    name: '',
    namespace: requestedNamespace,
    protocol_binding: '',
    skill_tag: '',
    backend_type: '',
    streaming: undefined as string | undefined,
    push_notifications: undefined as string | undefined,
  });
  const [editorState, setEditorState] = useState<{ visible: boolean; mode: Op }>({ visible: false, mode: 'create' });
  const [detailState, setDetailState] = useState<{ visible: boolean; agent?: A2AAgent }>({
    visible: false,
  });
  const [authorizeState, setAuthorizeState] = useState<{ visible: boolean; agent?: A2AAgent }>({
    visible: false,
  });
  const [detailActiveTab, setDetailActiveTab] = useState<A2ADetailTab>('card');
  const [cardViewMode, setCardViewMode] = useState<'visual' | 'raw'>('visual');

  const cardText = useMemo(() => jsonPreview(card), [card]);
  const cardObject = useMemo(() => parseJSONValue(card), [card]);
  const namespaces = new Set(datas.map((item) => item.namespace).filter(Boolean));
  const streamingCount = datas.filter((item) => item.streaming).length;
  const pushCount = datas.filter((item) => item.push_notifications).length;
  const serviceBackendCount = datas.filter((item) => backendType(item) === 'service').length;

  const refreshTable = (current = 1, pageSize = 10, nextQuery = query) => {
    dispatch(listA2AAgents({
      param: {
        offset: (current - 1) * pageSize,
        limit: pageSize,
        name: nextQuery.name || undefined,
        namespace: nextQuery.namespace || undefined,
        protocol_binding: nextQuery.protocol_binding || undefined,
        skill_tag: nextQuery.skill_tag || undefined,
        backend_type: nextQuery.backend_type || undefined,
        streaming: nextQuery.streaming === undefined ? undefined : nextQuery.streaming === 'true',
        push_notifications: nextQuery.push_notifications === undefined ? undefined : nextQuery.push_notifications === 'true',
      },
    })).then((res) => {
      if (res.meta.requestStatus === 'rejected') {
        openErrNotification('获取 A2A Agent 失败', res.payload as string);
      }
    });
  };

  useEffect(() => {
    const nextQuery = { ...query, namespace: requestedNamespace };
    setQuery(nextQuery);
    setResourceView(requestedNamespace ? 'environment' : 'logical');
    refreshTable(1, limit, nextQuery);
  }, [requestedNamespace]);

  useEffect(() => () => {
    dispatch(cleanA2APage());
  }, []);

  const openAgentDetail = (agent: A2AAgent, activeTab: A2ADetailTab = 'card') => {
    if (!agent?.id) return;
    dispatch(cleanA2ADetails());
    dispatch(editorA2AAgent(agent));
    setCardViewMode('visual');
    setDetailActiveTab(activeTab);
    setDetailState({ visible: true, agent });
    dispatch(listA2AAgentSkills({
      param: {
        agent_id: agent.id,
      },
    })).then((res) => {
      if (res.meta.requestStatus === 'rejected') {
        openErrNotification('获取 A2A 技能失败', res.payload as string);
      }
    });
    dispatch(getA2AAgentCard({ id: agent.id })).then((res) => {
      if (res.meta.requestStatus === 'rejected') {
        openErrNotification('获取 Agent Card 失败', res.payload as string);
      }
    });
  };

  const closeDetailDrawer = () => {
    dispatch(cleanA2ADetails());
    dispatch(resetA2AAgent());
    setCardViewMode('visual');
    setDetailActiveTab('card');
    setDetailState({ visible: false });
  };

  const operateAgent = (op: Op | 'detail' | 'skills' | 'card' | 'authorize', row?: TableRowData) => {
    const agent = row as A2AAgent;
    switch (op) {
      case 'create':
        dispatch(resetA2AAgent());
        setEditorState({ visible: true, mode: 'create' });
        break;
      case 'edit':
        navigate(`/ai/a2a/detail?id=${encodeURIComponent(String(agent?.id || ''))}&namespace=${encodeURIComponent(String(agent?.namespace || ''))}&name=${encodeURIComponent(String(agent?.name || ''))}&tab=edit`);
        break;
      case 'delete':
        dispatch(removeA2AAgents({ ids: [agent?.id as string] })).then((res) => {
          if (res.meta.requestStatus === 'fulfilled') {
            openInfoNotification('请求成功', '删除 A2A Agent 成功');
            refreshTable(page, limit);
          } else {
            openErrNotification('请求错误', res.payload as string);
          }
        });
        break;
      case 'detail':
        navigate(`/ai/a2a/detail?id=${encodeURIComponent(String(agent?.id || ''))}&namespace=${encodeURIComponent(String(agent?.namespace || ''))}&name=${encodeURIComponent(String(agent?.name || ''))}&tab=card`);
        break;
      case 'skills':
        navigate(`/ai/a2a/detail?id=${encodeURIComponent(String(agent?.id || ''))}&namespace=${encodeURIComponent(String(agent?.namespace || ''))}&name=${encodeURIComponent(String(agent?.name || ''))}&tab=skills`);
        break;
      case 'card':
        navigate(`/ai/a2a/detail?id=${encodeURIComponent(String(agent?.id || ''))}&namespace=${encodeURIComponent(String(agent?.namespace || ''))}&name=${encodeURIComponent(String(agent?.name || ''))}&tab=card`);
        break;
      case 'authorize':
        setAuthorizeState({ visible: true, agent });
        break;
      default:
        break;
    }
  };

  const submitFilter = ({ keyword, values }: QuerySnapshot) => {
    refreshTable(1, limit, {
      name: keyword,
      namespace: String(values.namespace || ''),
      protocol_binding: String(values.protocol_binding || ''),
      skill_tag: String(values.skill_tag || ''),
      backend_type: String(values.backend_type || ''),
      streaming: values.streaming ? String(values.streaming) : undefined,
      push_notifications: values.push_notifications ? String(values.push_notifications) : undefined,
    });
  };

  const resetFilter = () => {
    const nextQuery = {
      name: '',
      namespace: requestedNamespace,
      protocol_binding: '',
      skill_tag: '',
      backend_type: '',
      streaming: undefined,
      push_notifications: undefined,
    };
    setQuery(nextQuery);
    refreshTable(1, limit, nextQuery);
  };

  const refreshDetails = () => {
    const agent = detailState.agent;
    if (!agent?.id) return;
    if (detailActiveTab === 'skills') {
      dispatch(listA2AAgentSkills({ param: { agent_id: agent.id } }));
      return;
    }
    if (detailActiveTab === 'edit') {
      dispatch(editorA2AAgent(agent));
      return;
    }
    dispatch(getA2AAgentCard({ id: agent.id }));
  };

  const goBackendService = (agent?: A2AAgent) => {
    const service = backendServiceRef(agent);
    if (!service) return;
    navigate(`/discovery/service/instance?namespace=${encodeURIComponent(service.namespace)}&service=${encodeURIComponent(service.name)}`);
  };

  const openBoundEnvironment = (binding: { resource_id: string; namespace: string; resource_name: string }) => {
    navigate(`/ai/a2a/detail?id=${encodeURIComponent(binding.resource_id)}&namespace=${encodeURIComponent(binding.namespace)}&name=${encodeURIComponent(binding.resource_name)}&tab=card`);
  };

  return (
    <div className={style.page}>
      <ResourceHeader
        density="compact"
        placement="app-header"
        eyebrow="AI Native / A2A Agent Registry"
        title="A2A Agent"
        description="以全局逻辑资源统一识别 Agent，并管理各环境空间中的 Agent Card、技能能力与实际部署。"
      />

      <AIResourceViewSwitch value={resourceView} environment={requestedNamespace} onChange={setResourceView} />

      {resourceView === 'logical' ? (
        <AILogicalResourceDirectory
          resourceLabel="A2A Agent"
          instanceLabel="A2A Agent 环境实例"
          loadDefinitions={describeA2AAgentDefinitions}
          loadBindings={describeA2AAgentDefinitionEnvironments}
          loadInstances={describeAllA2AAgents}
          createDefinition={createA2AAgentDefinition}
          deleteDefinition={deleteA2AAgentDefinition}
          bindDefinition={bindA2AAgentDefinition}
          onOpenEnvironment={openBoundEnvironment}
          onCreateEnvironment={() => setResourceView('environment')}
        />
      ) : (
        <>

      <section className={style.metricRail}>
        <div className={style.metricItem}>
          <span>环境实例</span>
          <strong>{total}</strong>
        </div>
        <div className={style.metricItem}>
          <span>环境空间</span>
          <strong>{namespaces.size}</strong>
        </div>
        <div className={style.metricItem}>
          <span>Streaming</span>
          <strong>{streamingCount}</strong>
        </div>
        <div className={style.metricItem}>
          <span>Push</span>
          <strong>{pushCount}</strong>
        </div>
        <div className={style.metricItem}>
          <span>Pole 服务</span>
          <strong>{serviceBackendCount}</strong>
        </div>
      </section>

      <ResourceToolbar
        density="compact"
        title={requestedNamespace ? `${requestedNamespace} / Agent 环境实例` : 'Agent 环境实例'}
        count={loading ? '正在同步列表' : `当前显示 ${datas.length} 条`}
        filters={(
          <>
            <QueryComposer
              keyword={query.name}
              keywordPlaceholder="搜索 Agent 名称"
              suggestions={datas.map((item) => String(item.name || '')).filter(Boolean)}
              fields={[
                ...(!requestedNamespace ? [{
                  key: 'namespace',
                  label: '环境空间',
                  type: 'text' as const,
                }] : []),
                {
                  key: 'protocol_binding',
                  label: '协议',
                  type: 'select',
                  options: protocolOptions,
                },
                {
                  key: 'skill_tag',
                  label: 'Skill Tag',
                  type: 'text',
                },
                {
                  key: 'backend_type',
                  label: '后端类型',
                  type: 'select',
                  options: backendOptions,
                },
                {
                  key: 'streaming',
                  label: 'Streaming',
                  type: 'boolean',
                  trueLabel: '支持',
                  falseLabel: '不支持',
                },
                {
                  key: 'push_notifications',
                  label: 'Push',
                  type: 'boolean',
                  trueLabel: '支持',
                  falseLabel: '不支持',
                },
              ]}
              values={{
                namespace: query.namespace,
                protocol_binding: query.protocol_binding,
                skill_tag: query.skill_tag,
                backend_type: query.backend_type,
                streaming: query.streaming,
                push_notifications: query.push_notifications,
              }}
              onKeywordChange={(name) => setQuery((prev) => ({ ...prev, name }))}
              onValuesChange={(values) => setQuery((prev) => ({
                ...prev,
                namespace: String(values.namespace || ''),
                protocol_binding: String(values.protocol_binding || ''),
                skill_tag: String(values.skill_tag || ''),
                backend_type: String(values.backend_type || ''),
                streaming: values.streaming ? String(values.streaming) : undefined,
                push_notifications: values.push_notifications ? String(values.push_notifications) : undefined,
              }))}
              onSubmit={submitFilter}
              onReset={resetFilter}
              actions={(
                <>
                  <Tooltip content="刷新 Agent 列表">
                    <Button
                      aria-label="刷新 Agent 列表"
                      shape="square"
                      variant="outline"
                      icon={<RefreshIcon />}
                      onClick={() => refreshTable(page, limit)}
                    />
                  </Tooltip>
                  <Button theme="primary" icon={<AddIcon />} onClick={() => operateAgent('create')}>
                    新建 A2A 环境实例
                  </Button>
                </>
              )}
            />
          </>
        )}
      />

      <section className={style.tableSurface}>
        <Table
          className={style.agentTable}
          data={datas}
          columns={agentColumns(operateAgent, goBackendService, (namespace) => {
            navigate(`/namespace/environment?name=${encodeURIComponent(namespace)}`);
          })}
          loading={loading}
          rowKey="id"
          size="large"
          tableLayout="fixed"
          cellEmptyContent="-"
          pagination={{
            current: page,
            pageSize: limit,
            total,
            showJumper: true,
            onChange(pageInfo: PageInfo) {
              refreshTable(pageInfo.current, pageInfo.pageSize);
            },
          }}
          onPageChange={(pageInfo) => {
            refreshTable(pageInfo.current, pageInfo.pageSize);
          }}
        />
      </section>
        </>
      )}

      {editorState.visible && (
        <A2AEditor
          key={`${editorState.mode}-${editAgent?.id || 'new'}`}
          op={editorState.mode}
          visible={editorState.visible}
          environment={editorState.mode === 'create' ? requestedNamespace : ''}
          closeDrawer={() => {
            dispatch(resetA2AAgent());
            setEditorState((prev) => ({ ...prev, visible: false }));
            refreshTable(page, limit);
          }}
        />
      )}

      {authorizeState.visible && authorizeState.agent?.id && (
        <AuthorizeInput
          resource_type={PolicySourceType.A2AAgentResources}
          resource_id={authorizeState.agent.id}
          resource_name={`${authorizeState.agent.namespace}/${authorizeState.agent.name}`}
          visible={authorizeState.visible}
          onClose={() => setAuthorizeState({ visible: false })}
        />
      )}

      <Drawer
        size="min(1180px, 92vw)"
        header="A2A Agent 详情"
        footer={false}
        visible={detailState.visible}
        onClose={closeDetailDrawer}
      >
        <div className={style.agentDrawer}>
          <section className={style.agentDrawerSummary}>
            <div className={style.agentDrawerIcon}>
              <ServerIcon />
            </div>
            <div className={style.agentDrawerMain}>
              <div className={style.agentDrawerTitle}>
                <h3 title={detailState.agent ? `${detailState.agent.namespace}/${detailState.agent.name}` : '未选择 A2A Agent'}>
                  {detailState.agent ? `${detailState.agent.namespace}/${detailState.agent.name}` : '未选择 A2A Agent'}
                </h3>
                <Tag theme={protocolTheme(detailState.agent?.preferred_protocol_binding) as any} variant="light">
                  {protocolLabel(detailState.agent?.preferred_protocol_binding)}
                </Tag>
              </div>
              <div className={style.agentDrawerDesc} title={detailState.agent?.description || detailState.agent?.provider_organization || '该 Agent 暂无描述。'}>
                {detailState.agent?.description || detailState.agent?.provider_organization || '该 Agent 暂无描述。'}
              </div>
              <div className={style.agentMetaGrid}>
                <div>
                  <span>技能数</span>
                  <strong>{skillsLoading ? '-' : (detailState.agent?.skills?.length || skills.length || 0)}</strong>
                </div>
                <div>
                  <span>后端</span>
                  <strong>{backendTypeLabel(backendType(detailState.agent))}</strong>
                </div>
                <div>
                  <span>接入地址</span>
                  <strong>
                    {backendServiceRef(detailState.agent) ? (
                      <Link theme="primary" onClick={() => goBackendService(detailState.agent)}>
                        {backendLabel(detailState.agent)}
                      </Link>
                    ) : backendLabel(detailState.agent)}
                  </strong>
                </div>
                <div>
                  <span>最近修改</span>
                  <strong>{detailState.agent?.mtime || '-'}</strong>
                </div>
              </div>
            </div>
            <div className={style.agentDrawerActions}>
              <Tooltip content="刷新">
                <Button aria-label="刷新 Agent 详情" className={style.drawerActionIconButton} shape="square" variant="outline" onClick={refreshDetails}>
                  <RefreshIcon />
                </Button>
              </Tooltip>
            </div>
          </section>

          <Tabs className={style.agentDetailTabs} value={detailActiveTab} onChange={(value) => setDetailActiveTab(value as A2ADetailTab)}>
            <TabPanel value="card" label="Agent Card">
              <section className={style.detailSurface}>
                <div className={style.tableHeader}>
                  <div>
                    <strong>Agent Card</strong>
                    <span>{cardViewMode === 'visual' ? '概览' : '原始 Card JSON'}</span>
                  </div>
                  <Space className={style.cardViewSwitch} size={4}>
                    <Button
                      size="small"
                      theme={cardViewMode === 'visual' ? 'primary' : 'default'}
                      variant={cardViewMode === 'visual' ? 'base' : 'outline'}
                      onClick={() => setCardViewMode('visual')}
                    >
                      概览
                    </Button>
                    <Button
                      size="small"
                      theme={cardViewMode === 'raw' ? 'primary' : 'default'}
                      variant={cardViewMode === 'raw' ? 'base' : 'outline'}
                      onClick={() => setCardViewMode('raw')}
                    >
                      原始 JSON
                    </Button>
                  </Space>
                </div>
                {cardViewMode === 'raw' ? (
                  <Textarea
                    className={style.codeText}
                    readonly
                    value={cardLoading ? '加载中...' : cardText}
                    autosize={{ minRows: 18, maxRows: 26 }}
                  />
                ) : (
                  <AgentCardView card={cardObject} loading={cardLoading} />
                )}
              </section>
            </TabPanel>
            <TabPanel value="skills" label="技能">
              <section className={style.detailSurface}>
                <div className={style.tableHeader}>
                  <div>
                    <strong>技能目录</strong>
                    <span>{skillsLoading ? '正在同步技能' : `当前显示 ${skills.length} 个技能`}</span>
                  </div>
                </div>
                <AgentSkillsView skills={skills} loading={skillsLoading} />
              </section>
            </TabPanel>
            <TabPanel value="edit" label="编辑">
              <section className={style.detailSurface}>
                <div className={style.tableHeader}>
                  <div>
                    <strong>编辑 Agent</strong>
                    <span>修改基础信息、接入方式、技能和 Agent Card 原文</span>
                  </div>
                </div>
                <div className={style.embeddedEditor}>
                  <A2AEditor
                    op="edit"
                    visible={detailState.visible && detailActiveTab === 'edit'}
                    embedded
                    closeDrawer={() => {
                      closeDetailDrawer();
                      refreshTable(page, limit);
                    }}
                  />
                </div>
              </section>
            </TabPanel>
          </Tabs>
        </div>
      </Drawer>
    </div>
  );
});
