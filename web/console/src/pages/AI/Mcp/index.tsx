import React, { memo, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Drawer,
  Form,
  Input,
  Link,
  PrimaryTableProps,
  Select,
  Space,
  Table,
  TableRowData,
  Tag,
  Tooltip,
} from 'components/Fluent';
import EditorDrawer, { EditorDrawerActions } from 'components/EditorDrawer';
import type { FormProps, PageInfo } from 'components/Fluent';
import {
  AddIcon,
  EditIcon,
  InfoCircleIcon,
  RefreshIcon,
  SearchIcon,
  ServerIcon,
  ToolsCircleIcon,
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
import { useAppDispatch, useAppSelector } from 'modules/store';
import {
  cleanMCPPage,
  cleanMCPTools,
  editorMCPServer,
  listMCPServers,
  listMCPServerTools,
  removeMCPServers,
  resetMCPServer,
  saveMCPServer,
  selectMCP,
  updateMCPServer,
} from 'modules/ai/mcp';
import {
  MCPServer,
  MCPServerTool,
  bindMCPServerDefinition,
  createMCPServerDefinition,
  deleteMCPServerDefinition,
  describeAllMCPServers,
  describeMCPServerDefinitionEnvironments,
  describeMCPServerDefinitions,
} from 'services/mcp';
import { PolicySourceType } from 'services/auth_policy';
import { describeServices, ServiceView } from 'services/service';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import { Op } from 'services/types';
import ResourceNameLink from 'components/ResourceNameLink';
import style from './index.module.less';

const { FormItem } = Form;

const protocolOptions = [
  { label: 'HTTP', value: 'http' },
  { label: 'SSE', value: 'sse' },
  { label: 'Streamable HTTP', value: 'streamable-http' },
  { label: 'STDIO', value: 'stdio' },
];

const backendTypeOptions = [
  { label: 'Pole 注册服务', value: 'service' },
  { label: '自定义地址', value: 'address' },
];

export function protocolLabel(value?: string) {
  return protocolOptions.find((item) => item.value === value)?.label || value || '-';
}

export function protocolTheme(value?: string) {
  if (value === 'http') return 'success';
  if (value === 'sse') return 'primary';
  if (value === 'streamable-http') return 'warning';
  if (value === 'stdio') return 'default';
  return 'default';
}

export function backendType(server?: MCPServer) {
  if (server?.backend_type) return server.backend_type;
  if (server?.backend_service_namespace || server?.backend_service_name) return 'service';
  if (server?.backend_address) return 'address';
  return '';
}

export function backendLabel(server?: MCPServer) {
  const type = backendType(server);
  if (type === 'service') {
    const namespace = server?.backend_service_namespace || server?.namespace || '-';
    const name = server?.backend_service_name || server?.reference || server?.name || '-';
    return `${namespace}/${name}`;
  }
  if (type === 'address') return server?.backend_address || '-';
  return server?.reference || '-';
}

export function backendServiceRef(server?: MCPServer) {
  if (backendType(server) !== 'service') return undefined;
  const namespace = server?.backend_service_namespace || server?.namespace || '';
  const name = server?.backend_service_name || server?.reference || server?.name || '';
  if (!namespace || !name) return undefined;
  return { namespace, name };
}

function serviceKey(service?: Pick<ServiceView, 'namespace' | 'name'>) {
  if (!service?.namespace || !service?.name) return '';
  return `${service.namespace}/${service.name}`;
}

function parseServiceKey(value?: string) {
  const [namespace, ...names] = (value || '').split('/');
  return {
    namespace: namespace || '',
    name: names.join('/') || '',
  };
}

export function backendTypeLabel(value?: string) {
  return backendTypeOptions.find((item) => item.value === value)?.label || '-';
}

const serverColumns = (
  operateServer: (op: Op | 'detail' | 'authorize', row?: TableRowData) => void,
  goBackendService: (server?: MCPServer) => void,
  goEnvironment: (namespace: string) => void,
): PrimaryTableProps['columns'] => [
  {
    colKey: 'name',
    title: 'MCP Server',
    fixed: 'left',
    cell: ({ row }) => (
      <ResourceNameLink name={row.name} onClick={() => operateServer('detail', row)} />
    ),
  },
  {
    colKey: 'namespace',
    title: '环境空间',
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
    cell: ({ row }) => {
      const server = row as MCPServer;
      const serviceRef = backendServiceRef(server);
      return (
        <div className={style.compactCell}>
          <Text>{backendType(server) === 'service' ? 'Pole 注册服务' : backendType(server) === 'address' ? '自定义地址' : '-'}</Text>
          {serviceRef ? (
            <Link className={style.backendLink} theme="primary" onClick={() => goBackendService(server)}>
              {backendLabel(server)}
            </Link>
          ) : (
            <span>{backendLabel(server)}</span>
          )}
        </div>
      );
    },
  },
  {
    colKey: 'owner',
    title: '归属',
    cell: ({ row }) => (
      <div className={style.compactCell}>
        <Text>{row.business || '-'}</Text>
        <span>{row.department || '-'}</span>
      </div>
    ),
  },
  {
    colKey: 'visibility',
    title: '可见范围',
    cell: ({ row }) => {
      const values = splitExportTo(row.export_to);
      if (values.length === 0) return <Text>默认</Text>;
      return (
        <Space size={4}>
          {values.slice(0, 2).map((item) => <Tag key={item} variant="outline">{item}</Tag>)}
          {values.length > 2 && <Tag variant="outline">+{values.length - 2}</Tag>}
        </Space>
      );
    },
  },
  {
    colKey: 'time',
    title: '最近修改',
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
    cell: ({ row }) => (
      <Space>
        <OperationButton action="view" onClick={() => operateServer('detail', row)} />
        <OperationButton action="authorize" onClick={() => operateServer('authorize', row)} />
        <ConfirmOperationButton action="delete" confirmContent="确认删除该 MCP Server 吗" onConfirm={() => operateServer('delete', row)} />
      </Space>
    ),
  },
];

function safeParseJSON(value?: string) {
  if (!value) return undefined;
  try {
    return JSON.parse(value);
  } catch (_) {
    return undefined;
  }
}

function schemaType(schema: any): string {
  if (!schema) return '-';
  if (Array.isArray(schema.type)) return schema.type.join(' | ');
  if (schema.type === 'array') return `array<${schemaType(schema.items) || 'any'}>`;
  if (schema.type) return schema.type;
  if (schema.enum) return 'enum';
  if (schema.anyOf) return 'anyOf';
  if (schema.oneOf) return 'oneOf';
  if (schema.properties) return 'object';
  return '-';
}

function schemaDefault(schema: any): string {
  if (!schema || typeof schema !== 'object') return '-';
  if (schema.default !== undefined) return JSON.stringify(schema.default);
  if (Array.isArray(schema.enum) && schema.enum.length > 0) return schema.enum.map(String).join(' | ');
  return '-';
}

function nestedSchema(schema: any) {
  if (!schema || typeof schema !== 'object') return undefined;
  if (schema.type === 'array') return schema.items;
  return schema;
}

function hasNestedSchema(schema: any) {
  const target = nestedSchema(schema);
  return !!target?.properties && Object.keys(target.properties).length > 0;
}

function schemaRaw(value?: string) {
  const parsed = safeParseJSON(value);
  if (parsed) return JSON.stringify(parsed, null, 2);
  return value || '-';
}

function schemaFields(value?: string) {
  const parsed = safeParseJSON(value);
  if (!parsed || typeof parsed !== 'object') return [];
  const properties = parsed.properties || {};
  const required = new Set(Array.isArray(parsed.required) ? parsed.required : []);
  const entries = Object.entries(properties);
  if (entries.length === 0) {
    return [{
      name: '(root)',
      type: schemaType(parsed),
      required: false,
      description: parsed.description || '-',
      defaultValue: schemaDefault(parsed),
      schema: parsed,
    }];
  }
  return entries.map(([name, schema]: [string, any]) => ({
    name,
    type: schemaType(schema),
    required: required.has(name),
    description: schema.description || schema.title || '-',
    defaultValue: schemaDefault(schema),
    schema,
  }));
}

function exampleValue(schema: any, depth = 0): any {
  if (!schema || depth > 3) return null;
  if (Array.isArray(schema.enum) && schema.enum.length > 0) return schema.enum[0];
  const type = Array.isArray(schema.type) ? schema.type[0] : schema.type;
  if (type === 'string') return 'string';
  if (type === 'integer' || type === 'number') return 0;
  if (type === 'boolean') return true;
  if (type === 'array') return [exampleValue(schema.items, depth + 1)];
  if (type === 'object' || schema.properties) {
    return Object.entries(schema.properties || {}).reduce<Record<string, any>>((memo, [key, child]) => {
      memo[key] = exampleValue(child, depth + 1);
      return memo;
    }, {});
  }
  return null;
}

function schemaExample(value?: string) {
  const parsed = safeParseJSON(value);
  if (!parsed) return '-';
  return JSON.stringify(exampleValue(parsed), null, 2);
}

function annotationEntries(value?: string) {
  const parsed = safeParseJSON(value);
  if (!parsed || typeof parsed !== 'object') return [];
  return Object.entries(parsed).map(([key, val]) => ({ key, value: String(val) }));
}

function annotationTags(value?: string) {
  const annotations = safeParseJSON(value);
  if (!annotations || typeof annotations !== 'object') return [];
  const tags: string[] = [];
  if (annotations.title) tags.push(String(annotations.title));
  if (annotations.readOnlyHint === true) tags.push('Read only');
  if (annotations.destructiveHint === true) tags.push('Destructive');
  if (annotations.destructiveHint === false) tags.push('Non destructive');
  if (annotations.idempotentHint === true) tags.push('Idempotent');
  if (annotations.openWorldHint === false) tags.push('Closed world');
  return tags.slice(0, 4);
}

const SchemaSection: React.FC<{ title: string; schema?: string }> = ({ title, schema }) => {
  const fields = schemaFields(schema);
  const nestedFields = (field: any) => {
    const target = nestedSchema(field.schema);
    if (!target?.properties) return [];
    return Object.entries(target.properties).map(([name, child]: [string, any]) => ({
      name,
      type: schemaType(child),
      required: new Set(Array.isArray(target.required) ? target.required : []).has(name),
      description: child.description || child.title || '-',
      defaultValue: schemaDefault(child),
    }));
  };

  return (
    <section className={style.schemaSection}>
      <div className={style.schemaSectionHeader}>
        <strong>{title}</strong>
        <span>{fields.length > 0 ? `${fields.length} 个字段` : '原始 Schema'}</span>
      </div>
      {fields.length > 0 ? (
        <div className={style.schemaTableScroll}>
          <table className={style.schemaFieldTable} aria-label={`${title}字段 Schema`}>
          <thead>
            <tr>
              <th>字段</th>
              <th>类型</th>
              <th>约束</th>
              <th>说明</th>
              <th>默认/枚举</th>
            </tr>
          </thead>
          <tbody>
            {fields.map((field) => (
              <React.Fragment key={field.name}>
                <tr>
                  <td title={field.name}><code>{field.name}</code></td>
                  <td title={field.type}>{field.type}</td>
                  <td>
                    <Tag size="small" theme={field.required ? 'danger' : 'default'} variant="light">
                      {field.required ? 'required' : 'optional'}
                    </Tag>
                  </td>
                  <td title={field.description}>{field.description}</td>
                  <td title={field.defaultValue}>{field.defaultValue}</td>
                </tr>
                {hasNestedSchema(field.schema) && (
                  <tr className={style.schemaNestedRow}>
                    <td colSpan={5}>
                      <details>
                        <summary>展开 {field.name} 子字段</summary>
                        <div className={style.schemaTableScroll}>
                          <table aria-label={`${title}中 ${field.name} 的子字段 Schema`}>
                          <thead>
                            <tr>
                              <th>字段</th>
                              <th>类型</th>
                              <th>约束</th>
                              <th>说明</th>
                              <th>默认/枚举</th>
                            </tr>
                          </thead>
                          <tbody>
                            {nestedFields(field).map((child) => (
                              <tr key={child.name}>
                                <td title={child.name}><code>{child.name}</code></td>
                                <td title={child.type}>{child.type}</td>
                                <td>
                                  <Tag size="small" theme={child.required ? 'danger' : 'default'} variant="light">
                                    {child.required ? 'required' : 'optional'}
                                  </Tag>
                                </td>
                                <td title={child.description}>{child.description}</td>
                                <td title={child.defaultValue}>{child.defaultValue}</td>
                              </tr>
                            ))}
                          </tbody>
                          </table>
                        </div>
                      </details>
                    </td>
                  </tr>
                )}
              </React.Fragment>
            ))}
          </tbody>
          </table>
        </div>
      ) : (
        <div className={style.schemaFallback}>当前 schema 不是标准 JSON Schema object，已保留原始内容。</div>
      )}
      <details className={style.rawSchema}>
        <summary>查看原始 Schema</summary>
        <pre>{schemaRaw(schema)}</pre>
      </details>
    </section>
  );
};

export const ToolExplorer: React.FC<{ tools: MCPServerTool[] }> = ({ tools }) => {
  const [keyword, setKeyword] = useState('');
  const [selectedToolKey, setSelectedToolKey] = useState('');
  const filteredTools = useMemo(() => {
    const text = keyword.trim().toLowerCase();
    if (!text) return tools;
    return tools.filter((tool) => `${tool.name} ${tool.description || ''}`.toLowerCase().includes(text));
  }, [keyword, tools]);

  useEffect(() => {
    if (filteredTools.length === 0) return;
    const exists = filteredTools.some((tool) => (tool.id || tool.name) === selectedToolKey);
    if (!exists) setSelectedToolKey(filteredTools[0].id || filteredTools[0].name);
  }, [filteredTools, selectedToolKey]);

  const selectedTool = filteredTools.find((tool) => (tool.id || tool.name) === selectedToolKey) || filteredTools[0];
  const selectedAnnotations = annotationEntries(selectedTool?.annotations);
  const selectedTags = annotationTags(selectedTool?.annotations);

  return (
    <section className={style.toolExplorer}>
      <aside className={style.toolCatalog}>
        <Input
          clearable
          prefixIcon={<SearchIcon />}
          placeholder="搜索工具"
          value={keyword}
          onChange={(value) => setKeyword(value as string)}
        />
        <div className={style.toolCatalogList}>
          {filteredTools.map((tool) => {
            const key = tool.id || tool.name;
            const active = key === (selectedTool?.id || selectedTool?.name);
            return (
              <Button
                variant="text"
                key={key}
                className={`${style.toolCatalogItem} ${active ? style.toolCatalogItemActive : ''}`}
                type="button"
                onClick={() => setSelectedToolKey(key)}
              >
                <strong>{tool.name}</strong>
                <span>{tool.description || '未提供工具描述'}</span>
                <div>
                  {annotationTags(tool.annotations).map((item) => <Tag key={item} size="small" variant="light">{item}</Tag>)}
                </div>
              </Button>
            );
          })}
          {filteredTools.length === 0 && <div className={style.toolCatalogEmpty}>没有匹配的工具</div>}
        </div>
      </aside>

      {selectedTool && (
        <article className={style.toolDetail}>
          <header className={style.toolDetailHeader}>
            <div>
              <h4>{selectedTool.name}</h4>
              <p>{selectedTool.description || '未提供工具描述。'}</p>
            </div>
            <span>{selectedTool.mtime || '-'}</span>
          </header>

          <div className={style.toolTagRow}>
            {selectedTags.length > 0
              ? selectedTags.map((item) => <Tag key={item} variant="light">{item}</Tag>)
              : <Tag variant="light">No annotations</Tag>}
          </div>

          <div className={style.schemaColumns}>
            <SchemaSection title="输入参数" schema={selectedTool.input_schema} />
            <SchemaSection title="返回结构" schema={selectedTool.output_schema} />
          </div>

          <section className={style.exampleSection}>
            <div className={style.schemaSectionHeader}>
              <strong>示例</strong>
              <span>根据 schema 自动生成</span>
            </div>
            <div className={style.exampleGrid}>
              <div>
                <span>Request</span>
                <pre>{schemaExample(selectedTool.input_schema)}</pre>
              </div>
              <div>
                <span>Response</span>
                <pre>{schemaExample(selectedTool.output_schema)}</pre>
              </div>
            </div>
          </section>

          {selectedAnnotations.length > 0 && (
            <section className={style.annotationPanel}>
              <div className={style.schemaSectionHeader}>
                <strong>Annotations</strong>
                <span>{selectedAnnotations.length} 项</span>
              </div>
              <div className={style.annotationGrid}>
                {selectedAnnotations.map((item) => (
                  <div key={item.key}>
                    <span>{item.key}</span>
                    <strong>{item.value}</strong>
                  </div>
                ))}
              </div>
            </section>
          )}
        </article>
      )}
    </section>
  );
};

function splitExportTo(value?: string) {
  return value ? value.split(',').map((item) => item.trim()).filter(Boolean) : [];
}

function joinExportTo(value?: string[]) {
  return value?.filter(Boolean).join(',') || '';
}

export const MCPEditor: React.FC<{
  op: Op;
  visible: boolean;
  closeDrawer: () => void;
  environment?: string;
}> = ({ op, visible, closeDrawer, environment = '' }) => {
  const [form] = Form.useForm();
  const dispatch = useAppDispatch();
  const { editServer } = useAppSelector(selectMCP);
  const [backendMode, setBackendMode] = useState('service');
  const [serviceLoading, setServiceLoading] = useState(false);
  const [serviceOptions, setServiceOptions] = useState<ServiceView[]>([]);
  const initialBackendMode = backendType(editServer || undefined) || (editServer?.reference ? 'address' : 'service');
  const initialServiceKey = serviceKey({
    namespace: editServer?.backend_service_namespace || editServer?.namespace || '',
    name: editServer?.backend_service_name || editServer?.reference || editServer?.name || '',
  });
  const initialValues = useMemo(() => ({
    name: editServer?.name || '',
    namespace: editServer?.namespace || environment,
    ports: editServer?.ports || '',
    protocol: editServer?.protocol || 'http',
    business: editServer?.business || '',
    department: editServer?.department || '',
    description: editServer?.description || '',
    reference: editServer?.reference || '',
    export_to: splitExportTo(editServer?.export_to),
    backend_type: initialBackendMode,
    backend_service_key: initialBackendMode === 'service' ? initialServiceKey : '',
    backend_address: editServer?.backend_address || (initialBackendMode === 'address' ? editServer?.reference : ''),
  }), [editServer, environment, initialBackendMode, initialServiceKey]);

  const backendServiceOptions = useMemo(() => {
    const options = serviceOptions.map((item) => ({
      label: `${item.namespace}/${item.name}`,
      value: serviceKey(item),
    }));
    const currentKey = serviceKey({
      namespace: editServer?.backend_service_namespace || editServer?.namespace || '',
      name: editServer?.backend_service_name || editServer?.reference || editServer?.name || '',
    });
    if (currentKey && !options.some((item) => item.value === currentKey)) {
      options.unshift({ label: currentKey, value: currentKey });
    }
    return options;
  }, [editServer, serviceOptions]);

  const loadServices = async () => {
    setServiceLoading(true);
    try {
      const res = await describeServices({ offset: 0, limit: 100, namespace: environment || undefined });
      setServiceOptions(res.list);
    } catch (err: any) {
      openErrNotification('获取服务列表失败', err?.message || String(err));
    } finally {
      setServiceLoading(false);
    }
  };

  const applyBackendService = (value?: string) => {
    const selected = parseServiceKey(value);
    form.setFieldsValue({
      backend_service_key: value || '',
      name: selected.name || form.getFieldValue('name'),
      namespace: environment || selected.namespace || form.getFieldValue('namespace'),
      reference: selected.name || form.getFieldValue('reference'),
    });
  };

  useEffect(() => {
    if (!visible) return;
    setBackendMode(initialBackendMode);
    form.setFieldsValue(initialValues);
    loadServices();
  }, [visible, initialBackendMode, initialValues]);

  const resetEditor = () => {
    setBackendMode(initialBackendMode);
    form.setFieldsValue(initialValues);
  };

  const onSubmit: FormProps['onSubmit'] = async (e) => {
    if (e.validateResult !== true) return;
    const mode = (form.getFieldValue('backend_type') as string) || backendMode || 'service';
    const selectedService = parseServiceKey(form.getFieldValue('backend_service_key') as string);
    const backendAddress = (form.getFieldValue('backend_address') as string) || '';

    const data: MCPServer = {
      id: editServer?.id,
      name: (form.getFieldValue('name') as string) || selectedService.name,
      namespace: environment || (form.getFieldValue('namespace') as string) || selectedService.namespace,
      ports: form.getFieldValue('ports') as string,
      protocol: form.getFieldValue('protocol') as string,
      business: form.getFieldValue('business') as string,
      department: form.getFieldValue('department') as string,
      description: form.getFieldValue('description') as string,
      reference: mode === 'service' ? selectedService.name : '',
      export_to: joinExportTo(form.getFieldValue('export_to') as string[]),
      revision: editServer?.revision,
      backend_type: mode,
      backend_service_namespace: mode === 'service' ? environment || selectedService.namespace : '',
      backend_service_name: mode === 'service' ? selectedService.name : '',
      backend_address: mode === 'address' ? backendAddress : '',
    };

    const result = op === 'edit'
      ? await dispatch(updateMCPServer({ param: data }))
      : await dispatch(saveMCPServer({ param: data }));

    if (result.meta.requestStatus !== 'fulfilled') {
      openErrNotification('请求错误', result.payload as string);
      return;
    }
    openInfoNotification('请求成功', op === 'edit' ? '修改 MCP Server 成功' : '创建 MCP Server 成功');
    closeDrawer();
  };

  return (
    <EditorDrawer
      bodyClassName={style.editorBody}
      width="workspace"
      title={op === 'edit' ? '编辑 MCP Server' : '创建 MCP Server'}
      description="配置服务身份、后端关联和 MCP 接入参数。"
      footer={(
        <EditorDrawerActions
          onCancel={closeDrawer}
          onSubmit={() => form.submit({ showErrorMessage: true })}
          submitText={op === 'edit' ? '保存修改' : '创建 MCP Server'}
        />
      )}
      visible={visible}
      onClose={closeDrawer}
    >
      <Form className={style.drawerForm} form={form} layout="vertical" onSubmit={onSubmit} onReset={resetEditor}>
        <div className={style.formSection}>
          <div className={style.formSectionTitle}>基础信息</div>
          <div className={style.formGrid}>
            <FormItem
              label="名称"
              name="name"
              rules={[
                ...(backendMode === 'address' ? [{ required: true, message: '请输入 MCP Server 名称' }] : []),
                { pattern: /^[a-zA-Z0-9._-]+$/, message: '只允许数字、英文字母、.、-、_' },
              ]}
            >
              <Input readonly={op === 'edit'} placeholder={backendMode === 'service' ? '默认使用关联服务名' : '例如 order-query'} />
            </FormItem>
            <FormItem
              label="环境空间"
              name="namespace"
              rules={backendMode === 'address' ? [{ required: true, message: '请输入环境空间' }] : []}
            >
              <Input readonly={op === 'edit' || Boolean(environment)} placeholder={backendMode === 'service' ? '默认使用服务环境空间' : '例如 default'} />
            </FormItem>
            <FormItem label="业务" name="business">
              <Input />
            </FormItem>
            <FormItem label="部门" name="department">
              <Input />
            </FormItem>
          </div>
        </div>

        <div className={style.formSection}>
          <div className={style.formSectionTitle}>后端关联</div>
          <div className={style.backendChoice}>
            {backendTypeOptions.map((item) => (
              <Button
                variant="text"
                key={item.value}
                aria-pressed={backendMode === item.value}
                className={`${style.backendChoiceItem} ${backendMode === item.value ? style.backendChoiceItemActive : ''}`}
                type="button"
                onClick={() => {
                  setBackendMode(item.value);
                  form.setFieldsValue({ backend_type: item.value });
                }}
              >
                <strong>{item.label}</strong>
                <span>{item.value === 'service' ? 'namespace/name' : 'URL'}</span>
              </Button>
            ))}
          </div>
          <FormItem name="backend_type" style={{ display: 'none' }}>
            <Input />
          </FormItem>
          {backendMode === 'service' ? (
            <FormItem
              label="关联服务"
              name="backend_service_key"
              rules={[{ required: true, message: '请选择关联服务' }]}
            >
              <Select
                filterable
                loading={serviceLoading}
                options={backendServiceOptions}
                placeholder="选择已注册服务"
                onChange={(value) => applyBackendService(value as string)}
              />
            </FormItem>
          ) : (
            <FormItem
              label="自定义地址"
              name="backend_address"
              rules={[{ required: true, message: '请输入自定义地址' }]}
            >
              <Input placeholder="例如 http://127.0.0.1:8080/mcp" />
            </FormItem>
          )}
        </div>

        <div className={style.formSection}>
          <div className={style.formSectionTitle}>接入配置</div>
          <div className={style.formGrid}>
            <FormItem label="协议" name="protocol">
              <Select options={protocolOptions} />
            </FormItem>
            <FormItem label="端口" name="ports">
              <Input placeholder="例如 http:8080 或 8080" />
            </FormItem>
            <FormItem label="可见命名空间" name="export_to">
              <Select creatable multiple filterable placeholder="为空表示默认可见范围" />
            </FormItem>
            <FormItem label="后端类型">
              <Input readonly value={backendTypeLabel(backendMode)} />
            </FormItem>
          </div>
        </div>

        <FormItem label="描述" name="description">
          <Input placeholder="补充 server 能力或维护说明" />
        </FormItem>
      </Form>
    </EditorDrawer>
  );
};

export default memo(() => {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const requestedNamespace = searchParams.get('namespace') || '';
  const [resourceView, setResourceView] = useState<AIResourceView>(requestedNamespace ? 'environment' : 'logical');
  const { datas, loading, page, limit, total, editServer, tools, toolsLoading } = useAppSelector(selectMCP);
  const [query, setQuery] = useState({
    name: '',
    namespace: requestedNamespace,
    protocol: '',
  });
  const [editorState, setEditorState] = useState<{ visible: boolean; mode: Op }>({ visible: false, mode: 'create' });
  const [toolsState, setToolsState] = useState<{ visible: boolean; server?: MCPServer }>({ visible: false });
  const [authorizeState, setAuthorizeState] = useState<{ visible: boolean; server?: MCPServer }>({ visible: false });
  const namespaces = new Set(datas.map((item) => item.namespace).filter(Boolean));
  const protocolCount = datas.reduce<Record<string, number>>((memo, item) => {
    const key = item.protocol || 'unknown';
    return { ...memo, [key]: (memo[key] || 0) + 1 };
  }, {});
  const serviceBackendCount = datas.filter((item) => backendType(item) === 'service').length;

  const refreshTable = (current = 1, pageSize = 10, nextQuery = query) => {
    dispatch(listMCPServers({
      param: {
        offset: (current - 1) * pageSize,
        limit: pageSize,
        name: nextQuery.name || undefined,
        namespace: nextQuery.namespace || undefined,
        protocol: nextQuery.protocol || undefined,
      },
    })).then((res) => {
      if (res.meta.requestStatus === 'rejected') {
        openErrNotification('获取 MCP Server 失败', res.payload as string);
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
    dispatch(cleanMCPPage());
  }, []);

  const refreshTools = (server?: MCPServer) => {
    if (!server?.id) return;
    dispatch(listMCPServerTools({
      param: {
        offset: 0,
        limit: 100,
        server_id: server.id,
      },
    })).then((res) => {
      if (res.meta.requestStatus === 'rejected') {
        openErrNotification('获取 MCP 工具失败', res.payload as string);
      }
    });
  };

  const closeToolsDrawer = () => {
    dispatch(cleanMCPTools());
    setToolsState({ visible: false });
  };

  const editCurrentServerFromTools = () => {
    if (!toolsState.server) return;
    const currentServer = toolsState.server;
    closeToolsDrawer();
    dispatch(editorMCPServer(currentServer));
    setEditorState({ visible: true, mode: 'edit' });
  };

  const goBackendService = (server?: MCPServer) => {
    const service = backendServiceRef(server);
    if (!service) return;
    navigate(`/discovery/service/instance?namespace=${encodeURIComponent(service.namespace)}&service=${encodeURIComponent(service.name)}`);
  };

  const operateServer = (op: Op | 'detail' | 'authorize', row?: TableRowData) => {
    switch (op) {
      case 'create':
        dispatch(resetMCPServer());
        setEditorState({ visible: true, mode: 'create' });
        break;
      case 'edit':
        dispatch(editorMCPServer(row as MCPServer));
        setEditorState({ visible: true, mode: 'edit' });
        break;
      case 'delete':
        dispatch(removeMCPServers({ ids: [row?.id as string] })).then((res) => {
          if (res.meta.requestStatus === 'fulfilled') {
            openInfoNotification('请求成功', '删除 MCP Server 成功');
            refreshTable(page, limit);
          } else {
            openErrNotification('请求错误', res.payload as string);
          }
        });
        break;
      case 'detail':
        navigate(`/ai/mcps/detail?id=${encodeURIComponent(String(row?.id || ''))}&namespace=${encodeURIComponent(String(row?.namespace || ''))}&name=${encodeURIComponent(String(row?.name || ''))}`);
        break;
      case 'authorize':
        setAuthorizeState({ visible: true, server: row as MCPServer });
        break;
      default:
        break;
    }
  };

  const submitFilter = ({ keyword, values }: QuerySnapshot) => {
    refreshTable(1, limit, {
      name: keyword,
      namespace: String(values.namespace || ''),
      protocol: String(values.protocol || ''),
    });
  };

  const resetFilter = () => {
    const nextQuery = { name: '', namespace: requestedNamespace, protocol: '' };
    setQuery(nextQuery);
    refreshTable(1, limit, nextQuery);
  };

  const selectedToolServer = toolsState.server;

  const openBoundEnvironment = (binding: { resource_id: string; namespace: string; resource_name: string }) => {
    navigate(`/ai/mcps/detail?id=${encodeURIComponent(binding.resource_id)}&namespace=${encodeURIComponent(binding.namespace)}&name=${encodeURIComponent(binding.resource_name)}`);
  };

  return (
    <div className={style.page}>
      <ResourceHeader
        density="compact"
        placement="app-header"
        eyebrow="AI Native / MCP Registry"
        title="MCP 服务"
        description="以全局逻辑资源统一识别 MCP 能力，并管理各环境空间中的 Server 实例和工具同步。"
      />

      <AIResourceViewSwitch value={resourceView} environment={requestedNamespace} onChange={setResourceView} />

      {resourceView === 'logical' ? (
        <AILogicalResourceDirectory
          resourceLabel="MCP Server"
          instanceLabel="MCP Server 环境实例"
          loadDefinitions={describeMCPServerDefinitions}
          loadBindings={describeMCPServerDefinitionEnvironments}
          loadInstances={describeAllMCPServers}
          createDefinition={createMCPServerDefinition}
          deleteDefinition={deleteMCPServerDefinition}
          bindDefinition={bindMCPServerDefinition}
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
          <span>HTTP</span>
          <strong>{protocolCount.http || 0}</strong>
        </div>
        <div className={style.metricItem}>
          <span>SSE / Stream</span>
          <strong>{(protocolCount.sse || 0) + (protocolCount['streamable-http'] || 0)}</strong>
        </div>
        <div className={style.metricItem}>
          <span>Pole 服务</span>
          <strong>{serviceBackendCount}</strong>
        </div>
      </section>

      <ResourceToolbar
        density="compact"
        title={requestedNamespace ? `${requestedNamespace} / MCP 环境实例` : 'MCP 环境实例'}
        count={loading ? '正在同步列表' : `当前显示 ${datas.length} 条`}
        filters={(
          <>
            <QueryComposer
              keyword={query.name}
              keywordPlaceholder="搜索 MCP 服务名称"
              suggestions={datas.map((item) => String(item.name || '')).filter(Boolean)}
              fields={[
                ...(!requestedNamespace ? [{
                  key: 'namespace',
                  label: '环境空间',
                  type: 'text' as const,
                }] : []),
                {
                  key: 'protocol',
                  label: '协议',
                  type: 'select',
                  options: protocolOptions,
                },
              ]}
              values={{
                namespace: query.namespace,
                protocol: query.protocol,
              }}
              onKeywordChange={(name) => setQuery((prev) => ({ ...prev, name }))}
              onValuesChange={(values) => setQuery((prev) => ({
                ...prev,
                namespace: String(values.namespace || ''),
                protocol: String(values.protocol || ''),
              }))}
              onSubmit={submitFilter}
              onReset={resetFilter}
              actions={(
                <>
                  <Tooltip content="刷新 MCP 服务列表">
                    <Button
                      aria-label="刷新 MCP 服务列表"
                      shape="square"
                      variant="outline"
                      icon={<RefreshIcon />}
                      onClick={() => refreshTable(page, limit)}
                    />
                  </Tooltip>
                  <Button theme="primary" icon={<AddIcon />} onClick={() => operateServer('create')}>
                    新建 MCP 环境实例
                  </Button>
                </>
              )}
            />
          </>
        )}
      />

      <section className={style.tableSurface}>
        <Table
          data={datas}
          columns={serverColumns(operateServer, goBackendService, (namespace) => {
            navigate(`/namespace/environment?name=${encodeURIComponent(namespace)}`);
          })}
          loading={loading}
          rowKey="id"
          size="large"
          tableLayout="auto"
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
        <MCPEditor
          key={`${editorState.mode}-${editServer?.id || 'new'}`}
          op={editorState.mode}
          visible={editorState.visible}
          environment={editorState.mode === 'create' ? requestedNamespace : ''}
          closeDrawer={() => {
            dispatch(resetMCPServer());
            setEditorState((prev) => ({ ...prev, visible: false }));
            refreshTable(page, limit);
          }}
        />
      )}

      {authorizeState.visible && authorizeState.server?.id && (
        <AuthorizeInput
          resource_type={PolicySourceType.MCPServerResources}
          resource_id={authorizeState.server.id}
          resource_name={`${authorizeState.server.namespace}/${authorizeState.server.name}`}
          visible={authorizeState.visible}
          onClose={() => setAuthorizeState({ visible: false })}
        />
      )}

      <Drawer
        size="min(1180px, 92vw)"
        header="MCP 服务详情"
        footer={false}
        visible={toolsState.visible}
        onClose={closeToolsDrawer}
      >
        <div className={style.toolDrawer}>
          <section className={style.toolDrawerSummary}>
            <div className={style.toolDrawerIcon}>
              <ServerIcon />
            </div>
            <div className={style.toolDrawerMain}>
              <div className={style.toolDrawerTitle}>
                <h3>{selectedToolServer ? `${selectedToolServer.namespace}/${selectedToolServer.name}` : '未选择 MCP Server'}</h3>
                <Tag theme={protocolTheme(selectedToolServer?.protocol) as any} variant="light">
                  {protocolLabel(selectedToolServer?.protocol)}
                </Tag>
              </div>
              <div className={style.toolDrawerDesc}>
                {selectedToolServer?.description || selectedToolServer?.reference || '该 MCP Server 暂无描述。'}
              </div>
              <div className={style.toolMetaGrid}>
                <div>
                  <span>工具数</span>
                  <strong>{toolsLoading ? '-' : tools.length}</strong>
                </div>
                <div>
                  <span>接入</span>
                  <strong>{backendTypeLabel(backendType(selectedToolServer))}</strong>
                </div>
                <div>
                  <span>后端</span>
                  <strong>
                    {backendServiceRef(selectedToolServer) ? (
                      <Link theme="primary" onClick={() => goBackendService(selectedToolServer)}>
                        {backendLabel(selectedToolServer)}
                      </Link>
                    ) : backendLabel(selectedToolServer)}
                  </strong>
                </div>
                <div>
                  <span>最近修改</span>
                  <strong>{selectedToolServer?.mtime || '-'}</strong>
                </div>
              </div>
            </div>
            <div className={style.toolDrawerActions}>
              <Tooltip content="刷新工具">
                <Button className={style.drawerActionIconButton} shape="square" variant="outline" onClick={() => refreshTools(selectedToolServer)}>
                  <RefreshIcon />
                </Button>
              </Tooltip>
              <Button variant="outline" icon={<EditIcon />} onClick={editCurrentServerFromTools}>
                编辑 Server
              </Button>
            </div>
          </section>

          {!toolsLoading && tools.length === 0 ? (
            <section className={style.toolEmpty}>
              <ToolsCircleIcon />
              <h4>暂无工具同步</h4>
              <p>当前 MCP Server 没有返回工具定义。确认服务已连通，并检查协议、引用地址和命名空间配置。</p>
              <div className={style.toolCheckList}>
                <div><InfoCircleIcon />后端：{backendLabel(selectedToolServer)}</div>
                <div><InfoCircleIcon />接入协议：{protocolLabel(selectedToolServer?.protocol)}</div>
                <div><InfoCircleIcon />Server ID：{selectedToolServer?.id || '-'}</div>
              </div>
              <Space>
                <Button theme="primary" icon={<RefreshIcon />} onClick={() => refreshTools(selectedToolServer)}>
                  刷新工具
                </Button>
                <Button variant="outline" icon={<EditIcon />} onClick={editCurrentServerFromTools}>
                  编辑 Server
                </Button>
              </Space>
            </section>
          ) : (
            <section className={style.toolTableSurface}>
              <div className={style.tableHeader}>
                <div>
                  <strong>工具浏览</strong>
                  <span>{toolsLoading ? '正在同步工具' : `当前显示 ${tools.length} 个工具`}</span>
                </div>
              </div>
              <ToolExplorer tools={tools} />
            </section>
          )}
        </div>
      </Drawer>
    </div>
  );
});
