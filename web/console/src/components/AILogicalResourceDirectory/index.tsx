import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Dialog,
  Input,
  Link,
  Select,
  Table,
  Tag,
  Tooltip,
} from 'components/Fluent';
import type { PrimaryTableProps } from 'components/Fluent';
import { AddIcon, RefreshIcon } from 'components/Fluent/icons';
import { ConfirmOperationButton, OperationButtonGroup } from 'components/OperationButton';
import { ResourceToolbar } from 'components/ResourceLayout';
import type { AIEnvironmentBinding, AIResourceDefinition } from 'services/ai_definition';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import style from './index.module.less';

export type AIResourceView = 'logical' | 'environment';

export interface AIEnvironmentInstance {
  id?: string;
  name: string;
  namespace: string;
}

interface LogicalResourceRow extends AIResourceDefinition {
  bindings: AIEnvironmentBinding<string>[];
}

interface AILogicalResourceDirectoryProps {
  resourceLabel: string;
  instanceLabel: string;
  loadDefinitions: (name?: string) => Promise<AIResourceDefinition[]>;
  loadBindings: (definitionId: string) => Promise<AIEnvironmentBinding<string>[]>;
  loadInstances: () => Promise<AIEnvironmentInstance[]>;
  createDefinition: (name: string) => Promise<AIResourceDefinition>;
  deleteDefinition: (definitionId: string) => Promise<unknown>;
  bindDefinition: (definitionId: string, resourceId: string) => Promise<unknown>;
  onOpenEnvironment: (binding: AIEnvironmentBinding<string>) => void;
  onCreateEnvironment: () => void;
}

export const AIResourceViewSwitch: React.FC<{
  value: AIResourceView;
  environment?: string;
  onChange: (value: AIResourceView) => void;
}> = ({ value, environment, onChange }) => (
  <div className={style.viewBar} aria-label="AI 资源视图">
    <div className={style.viewSwitch}>
      <Button
        variant="text"
        type="button"
        className={value === 'logical' ? style.viewButtonActive : style.viewButton}
        onClick={() => onChange('logical')}
      >
        逻辑资源
      </Button>
      <Button
        variant="text"
        type="button"
        className={value === 'environment' ? style.viewButtonActive : style.viewButton}
        onClick={() => onChange('environment')}
      >
        环境实例
      </Button>
    </div>
    <span className={style.viewHint}>
      {value === 'logical'
        ? '全局定义，用于关联不同环境中的同一项能力'
        : environment ? `当前环境：${environment}` : '按环境空间查询实际运行实例'}
    </span>
  </div>
);

const AILogicalResourceDirectory: React.FC<AILogicalResourceDirectoryProps> = ({
  resourceLabel,
  instanceLabel,
  loadDefinitions,
  loadBindings,
  loadInstances,
  createDefinition,
  deleteDefinition,
  bindDefinition,
  onOpenEnvironment,
  onCreateEnvironment,
}) => {
  const [rows, setRows] = useState<LogicalResourceRow[]>([]);
  const [instances, setInstances] = useState<AIEnvironmentInstance[]>([]);
  const [loading, setLoading] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [createVisible, setCreateVisible] = useState(false);
  const [definitionName, setDefinitionName] = useState('');
  const [bindTarget, setBindTarget] = useState<LogicalResourceRow>();
  const [resourceId, setResourceId] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const refresh = async () => {
    setLoading(true);
    try {
      const [definitions, environmentInstances] = await Promise.all([
        loadDefinitions(keyword.trim() || undefined),
        loadInstances(),
      ]);
      const bindingResults = await Promise.allSettled(
        definitions.map((definition) => loadBindings(definition.id)),
      );
      setRows(definitions.map((definition, index) => {
        const result = bindingResults[index];
        return {
          ...definition,
          bindings: result.status === 'fulfilled'
            ? result.value.filter((binding) => binding.namespace !== 'pole-system')
            : [],
        };
      }));
      setInstances(environmentInstances.filter((instance) => instance.namespace !== 'pole-system'));
      if (bindingResults.some((result) => result.status === 'rejected')) {
        openErrNotification('部分环境关联加载失败', '逻辑资源已显示，请刷新后重试环境覆盖信息。');
      }
    } catch (cause) {
      openErrNotification(`获取 ${resourceLabel} 逻辑资源失败`, (cause as Error).message || String(cause));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    refresh();
  }, []);

  const allBindings = useMemo(() => rows.flatMap((row) => row.bindings), [rows]);
  const environmentCount = useMemo(
    () => new Set(allBindings.map((binding) => binding.namespace)).size,
    [allBindings],
  );
  const boundResourceIds = useMemo(
    () => new Set(allBindings.map((binding) => binding.resource_id)),
    [allBindings],
  );
  const availableInstances = useMemo(
    () => instances.filter((instance) => instance.id && !boundResourceIds.has(instance.id)),
    [instances, boundResourceIds],
  );
  const bindableInstances = useMemo(() => {
    const occupiedEnvironments = new Set(bindTarget?.bindings.map((binding) => binding.namespace) || []);
    return availableInstances.filter((instance) => !occupiedEnvironments.has(instance.namespace));
  }, [availableInstances, bindTarget]);

  const confirmCreate = async () => {
    const name = definitionName.trim();
    if (!name) return;
    setSubmitting(true);
    try {
      await createDefinition(name);
      openInfoNotification('创建成功', `${resourceLabel} 逻辑资源已创建`);
      setCreateVisible(false);
      setDefinitionName('');
      await refresh();
    } catch (cause) {
      openErrNotification('创建逻辑资源失败', (cause as Error).message || String(cause));
    } finally {
      setSubmitting(false);
    }
  };

  const confirmBind = async () => {
    if (!bindTarget?.id || !resourceId) return;
    setSubmitting(true);
    try {
      await bindDefinition(bindTarget.id, resourceId);
      openInfoNotification('关联成功', `环境实例已归入 ${bindTarget.name}`);
      setBindTarget(undefined);
      setResourceId('');
      await refresh();
    } catch (cause) {
      openErrNotification('关联环境实例失败', (cause as Error).message || String(cause));
    } finally {
      setSubmitting(false);
    }
  };

  const confirmDelete = async (definition: LogicalResourceRow) => {
    if (!definition.id || definition.bindings.length > 0) return;
    setSubmitting(true);
    try {
      await deleteDefinition(definition.id);
      openInfoNotification('删除成功', `${resourceLabel} 逻辑资源“${definition.name}”已删除`);
      await refresh();
    } catch (cause) {
      openErrNotification('删除逻辑资源失败', (cause as Error).message || String(cause));
    } finally {
      setSubmitting(false);
    }
  };

  const columns: PrimaryTableProps['columns'] = [
    {
      colKey: 'name',
      title: '逻辑资源',
      width: 300,
      cell: ({ row }) => (
        <div className={style.definitionCell}>
          <strong>{row.name}</strong>
          <span>{row.description || `跨环境稳定识别的 ${resourceLabel}`}</span>
        </div>
      ),
    },
    {
      colKey: 'environments',
      title: '已覆盖环境',
      cell: ({ row }) => {
        const bindings = (row as LogicalResourceRow).bindings;
        if (bindings.length === 0) return <span className={style.muted}>尚未关联环境实例</span>;
        return (
          <div className={style.environmentList}>
            {bindings.slice(0, 5).map((binding) => (
              <Link key={`${binding.namespace}-${binding.resource_id}`} theme="primary" onClick={() => onOpenEnvironment(binding)}>
                <Tag variant="outline">{binding.namespace}</Tag>
              </Link>
            ))}
            {bindings.length > 5 && <Tag variant="outline">+{bindings.length - 5}</Tag>}
          </div>
        );
      },
    },
    {
      colKey: 'instances',
      title: '实例数',
      width: 90,
      cell: ({ row }) => `${(row as LogicalResourceRow).bindings.length} 个`,
    },
    {
      colKey: 'revision',
      title: '版本',
      width: 150,
      cell: ({ row }) => row.revision || '-',
    },
    {
      colKey: 'action',
      title: '操作',
      width: 190,
      cell: ({ row }) => (
        <OperationButtonGroup>
          <Button
            size="small"
            variant="outline"
            onClick={() => {
              setBindTarget(row as LogicalResourceRow);
              setResourceId('');
            }}
          >
            绑定环境实例
          </Button>
          <ConfirmOperationButton
            action="delete"
            label="删除逻辑资源"
            disabled={(row as LogicalResourceRow).bindings.length > 0 || submitting}
            disabledLabel={(row as LogicalResourceRow).bindings.length > 0 ? '请先解绑全部环境实例' : '正在提交'}
            confirmContent={`确认删除逻辑资源“${row.name}”吗？删除后不可恢复。`}
            onConfirm={() => void confirmDelete(row as LogicalResourceRow)}
          />
        </OperationButtonGroup>
      ),
    },
  ];

  return (
    <>
      <section className={style.metricRail}>
        <div><span>逻辑资源</span><strong>{rows.length}</strong></div>
        <div><span>环境实例</span><strong>{allBindings.length}</strong></div>
        <div><span>覆盖环境</span><strong>{environmentCount}</strong></div>
        <div><span>待部署</span><strong>{rows.filter((row) => row.bindings.length === 0).length}</strong></div>
      </section>

      <ResourceToolbar
        density="compact"
        title={`${resourceLabel} 逻辑资源`}
        count={loading ? '正在同步目录' : `共 ${rows.length} 项`}
        filters={(
          <div className={style.toolbarActions}>
            <Input
              value={keyword}
              placeholder={`搜索 ${resourceLabel} 逻辑名称`}
              onChange={(value: unknown) => setKeyword(String(value ?? ''))}
              onKeyDown={(event: React.KeyboardEvent<HTMLInputElement>) => {
                if (event.key === 'Enter') refresh();
              }}
            />
            <Tooltip content="刷新逻辑资源">
              <Button aria-label="刷新逻辑资源" shape="square" variant="outline" icon={<RefreshIcon />} onClick={refresh} />
            </Tooltip>
            <Button variant="outline" onClick={onCreateEnvironment}>查看环境实例</Button>
            <Button theme="primary" icon={<AddIcon />} onClick={() => setCreateVisible(true)}>新建逻辑资源</Button>
          </div>
        )}
      />

      <section className={style.tableSurface}>
        <Table
          data={rows}
          columns={columns}
          loading={loading}
          rowKey="id"
          size="large"
          tableLayout="fixed"
          cellEmptyContent="-"
        />
      </section>

      <Dialog
        visible={createVisible}
        onClose={() => setCreateVisible(false)}
        onConfirm={confirmCreate}
        confirmBtn={submitting ? '创建中...' : '创建'}
        header={`新建 ${resourceLabel} 逻辑资源`}
        width={520}
      >
        <div className={style.dialogBody}>
          <p>逻辑资源是跨环境稳定身份。创建后，再把各环境中的 {instanceLabel} 显式绑定到这里。</p>
          <Input
            value={definitionName}
            aria-label={`${resourceLabel} 逻辑名称`}
            placeholder={`输入 ${resourceLabel} 逻辑名称`}
            onChange={(value: unknown) => setDefinitionName(String(value ?? ''))}
          />
        </div>
      </Dialog>

      <Dialog
        visible={Boolean(bindTarget)}
        onClose={() => setBindTarget(undefined)}
        onConfirm={confirmBind}
        confirmBtn={submitting ? '关联中...' : '确认关联'}
        header={`为 ${bindTarget?.name || resourceLabel} 绑定环境实例`}
        width={560}
      >
        <div className={style.dialogBody}>
          <p>仅显示尚未绑定其它逻辑资源的实例；每个逻辑资源在同一环境最多关联一个实例。</p>
          <Select
            filterable
            value={resourceId || undefined}
            placeholder={bindableInstances.length ? `选择 ${instanceLabel}` : '暂无可绑定的环境实例'}
            options={bindableInstances.map((instance) => ({
              label: `${instance.namespace} / ${instance.name}`,
              value: instance.id!,
            }))}
            onChange={(value: unknown) => setResourceId(String(value || ''))}
          />
        </div>
      </Dialog>
    </>
  );
};

export default React.memo(AILogicalResourceDirectory);
