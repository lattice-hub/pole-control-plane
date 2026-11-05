import React from 'react';

import QueryComposer, { QuerySnapshot } from 'components/QueryComposer';
import { ConfirmOperationButton, OperationButton, OperationButtonGroup } from 'components/OperationButton';
import { ResourceHeader, ResourceToolbar } from 'components/ResourceLayout';
import ResourceNameLink from 'components/ResourceNameLink';
import { Button, Empty, Loading, Table, Tag, Tooltip } from 'components/Fluent';
import {
    ArrowRightIcon,
    BotIcon,
    ComponentSpaceIcon,
    LayersIcon,
    QueueIcon,
    RefreshIcon,
    RollbackIcon,
    ServerIcon,
} from 'components/Fluent/icons';
import { useNavigate, useSearchParams } from 'components/Router';
import {
    deleteEnvironmentResource,
    describeEnvironmentResources,
    EnvironmentResource,
    EnvironmentResourceDomain,
} from 'services/environment_resources';
import { describeNamespaces, NamespaceView } from 'services/namespace';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import style from './EnvironmentResources.module.less';

type DomainFilter = '' | EnvironmentResourceDomain;

const PAGE_SIZE = 10;

const domainOptions = [
    { label: '注册发现', value: 'discovery' },
    { label: '配置中心', value: 'configuration' },
    { label: '治理规则', value: 'governance' },
    { label: 'AI 工具', value: 'ai' },
];

const domainMeta: Record<EnvironmentResourceDomain, { label: string, Icon: React.FC<any> }> = {
    discovery: { label: '注册发现', Icon: ServerIcon },
    configuration: { label: '配置中心', Icon: QueueIcon },
    governance: { label: '治理规则', Icon: LayersIcon },
    ai: { label: 'AI 工具', Icon: BotIcon },
};

const statusTag = (row: EnvironmentResource) => {
    if (row.statusTone === 'success') return <Tag size="small" theme="success" variant="light">{row.status}</Tag>;
    if (row.statusTone === 'warning') return <Tag size="small" theme="warning" variant="light">{row.status}</Tag>;
    return <Tag size="small" variant="light">{row.status}</Tag>;
};

export default React.memo(() => {
    const navigate = useNavigate();
    const [searchParams] = useSearchParams();
    const namespaceName = searchParams.get('name') || '';
    const [namespace, setNamespace] = React.useState<NamespaceView>();
    const [resources, setResources] = React.useState<EnvironmentResource[]>([]);
    const [warnings, setWarnings] = React.useState<string[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState('');
    const [keyword, setKeyword] = React.useState('');
    const [domain, setDomain] = React.useState<DomainFilter>('');
    const [page, setPage] = React.useState(1);
    const [deletingKey, setDeletingKey] = React.useState('');

    const refresh = React.useCallback(async () => {
        if (!namespaceName) {
            setError('缺少业务环境名称');
            setLoading(false);
            return;
        }
        setLoading(true);
        setError('');
        try {
            const [namespaceResult, resourceResult] = await Promise.all([
                describeNamespaces({ offset: 0, limit: 1, name: namespaceName, kind: 'business' }),
                describeEnvironmentResources(namespaceName),
            ]);
            const current = namespaceResult.namespaces.find((item) => item.name === namespaceName);
            if (!current) throw new Error(`业务环境 ${namespaceName} 不存在或当前账号无权访问`);
            setNamespace(current);
            setResources(resourceResult.resources);
            setWarnings(resourceResult.warnings);
        } catch (reason) {
            const message = reason instanceof Error ? reason.message : String(reason);
            setError(message);
            openErrNotification('加载环境资源失败', message);
        } finally {
            setLoading(false);
        }
    }, [namespaceName]);

    React.useEffect(() => {
        void refresh();
    }, [refresh]);

    const summary = React.useMemo(() => {
        const services = resources.filter((item) => item.domain === 'discovery');
        const governance = resources.filter((item) => item.domain === 'governance');
        const ai = resources.filter((item) => item.domain === 'ai');
        const totalInstances = Number(namespace?.total_instance_count || 0);
        const healthyInstances = Number(namespace?.total_health_instance_count || 0);
        return {
            services: services.length,
            governance: governance.length,
            ai: ai.length,
            totalInstances,
            healthyInstances,
            healthRate: totalInstances > 0 ? Math.round((healthyInstances / totalInstances) * 100) : 0,
        };
    }, [namespace, resources]);

    const filtered = React.useMemo(() => resources.filter((item) => {
        if (domain && item.domain !== domain) return false;
        if (!keyword) return true;
        const normalized = keyword.toLowerCase();
        return [item.name, item.typeLabel, item.description, item.metric]
            .some((value) => value.toLowerCase().includes(normalized));
    }), [domain, keyword, resources]);

    const current = filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);

    const selectDomain = (next: DomainFilter) => {
        setDomain(next);
        setPage(1);
    };

    const submitFilter = ({ keyword: nextKeyword, values }: QuerySnapshot) => {
        setKeyword(nextKeyword);
        selectDomain((values.domain as DomainFilter) || '');
    };

    const openDomain = (target: EnvironmentResourceDomain) => {
        if (target === 'discovery') {
            navigate(`/discovery/service?namespace=${encodeURIComponent(namespaceName)}`);
            return;
        }
        if (target === 'configuration') {
            navigate(`/configuration?environment=${encodeURIComponent(namespaceName)}`);
            return;
        }
        if (target === 'ai') {
            navigate(`/ai/a2a?namespace=${encodeURIComponent(namespaceName)}`);
            return;
        }
        navigate(`/governance/workbench?namespace=${encodeURIComponent(namespaceName)}`);
    };

    const removeResource = async (resource: EnvironmentResource) => {
        setDeletingKey(resource.key);
        try {
            await deleteEnvironmentResource(resource);
            openInfoNotification('删除成功', `${resource.typeLabel}“${resource.name}”已删除`);
            await refresh();
        } catch (reason) {
            openErrNotification(`删除${resource.typeLabel}失败`, reason instanceof Error ? reason : String(reason));
        } finally {
            setDeletingKey('');
        }
    };

    const columns = [
        {
            colKey: 'name',
            title: '资源',
            width: 240,
            cell: ({ row }: { row: EnvironmentResource }) => (
                <div className={style.resourceIdentity}>
                    <ResourceNameLink name={row.name} onClick={() => navigate(row.target)} />
                    <span>{row.description}</span>
                </div>
            ),
        },
        {
            colKey: 'domain',
            title: '资源域',
            width: 120,
            cell: ({ row }: { row: EnvironmentResource }) => {
                const meta = domainMeta[row.domain];
                return (
                    <div className={style.domainCell}>
                        <meta.Icon />
                        <span>{meta.label}</span>
                    </div>
                );
            },
        },
        { colKey: 'typeLabel', title: '类型', width: 120 },
        { colKey: 'metric', title: '资源摘要', minWidth: 150 },
        {
            colKey: 'status',
            title: '状态',
            width: 90,
            cell: ({ row }: { row: EnvironmentResource }) => statusTag(row),
        },
        {
            colKey: 'mtime',
            title: '更新时间',
            width: 150,
            cell: ({ row }: { row: EnvironmentResource }) => row.mtime || '-',
        },
        {
            colKey: 'action',
            title: '操作',
            width: 104,
            fixed: 'right' as const,
            cell: ({ row }: { row: EnvironmentResource }) => (
                <OperationButtonGroup>
                    <OperationButton action="view" onClick={() => navigate(row.target)} />
                    <ConfirmOperationButton
                        action="delete"
                        disabled={!row.deleteable || Boolean(deletingKey)}
                        disabledLabel={!row.deleteable ? row.deleteDisabledReason || '无权限操作' : '正在删除'}
                        confirmContent={`确认删除${row.typeLabel}“${row.name}”？删除后无法恢复。`}
                        onConfirm={() => void removeResource(row)}
                    />
                </OperationButtonGroup>
            ),
        },
    ];

    return (
        <main className={style.page}>
            <ResourceHeader
                density="compact"
                placement="app-header"
                eyebrow="业务环境 / 资源视图"
                title={namespaceName || '环境资源'}
                description="以 Namespace 为范围查看注册发现、配置中心与治理资源。"
                actions={(
                    <div className={style.headerActions}>
                        <Button variant="outline" icon={<RollbackIcon />} onClick={() => navigate('/namespace')}>返回环境列表</Button>
                        <Tooltip content="重新查询当前环境资源">
                            <Button shape="square" variant="outline" icon={<RefreshIcon />} aria-label="刷新环境资源" onClick={() => void refresh()} />
                        </Tooltip>
                    </div>
                )}
            />

            <Loading loading={loading}>
                {error ? (
                    <section className={style.errorState}>
                        <Empty title="无法加载环境资源" description={error} action={<Button onClick={() => void refresh()}>重新加载</Button>} />
                    </section>
                ) : (
                    <div className={style.workspace}>
                        <section className={style.environmentSummary}>
                            <div className={style.environmentIdentity}>
                                <div className={style.environmentIcon}><ComponentSpaceIcon /></div>
                                <div>
                                    <span>业务环境</span>
                                    <strong>{namespaceName}</strong>
                                    <p>{namespace?.comment || '未填写环境说明'}</p>
                                </div>
                            </div>
                            <div className={style.healthSummary}>
                                <span>实例健康</span>
                                <strong>{summary.healthyInstances}/{summary.totalInstances}</strong>
                                <div className={style.healthTrack}><i style={{ width: `${summary.healthRate}%` }} /></div>
                                <small>{summary.totalInstances > 0 ? `${summary.healthRate}% 健康` : '当前没有服务实例'}</small>
                            </div>
                        </section>

                        <section className={style.domainRail} aria-label="环境资源域">
                            <Button variant="text" type="button" aria-pressed={!domain} className={!domain ? style.activeDomain : ''} onClick={() => selectDomain('')}>
                                <span>全部资源</span>
                                <strong>{resources.length}</strong>
                                <small>当前账号可见</small>
                            </Button>
                            <Button variant="text" type="button" aria-pressed={domain === 'discovery'} className={domain === 'discovery' ? style.activeDomain : ''} onClick={() => selectDomain('discovery')}>
                                <span>注册发现</span>
                                <strong>{summary.services}</strong>
                                <small>{summary.totalInstances} 个实例</small>
                            </Button>
                            <Button variant="text" type="button" aria-pressed={domain === 'configuration'} className={domain === 'configuration' ? style.activeDomain : ''} onClick={() => selectDomain('configuration')}>
                                <span>配置中心</span>
                                <strong className={style.lookupValue}>按键打开</strong>
                                <small>输入分组和名称定位配置</small>
                            </Button>
                            <Button variant="text" type="button" aria-pressed={domain === 'governance'} className={domain === 'governance' ? style.activeDomain : ''} onClick={() => selectDomain('governance')}>
                                <span>治理规则</span>
                                <strong>{summary.governance}</strong>
                                <small>跨类型统一查询</small>
                            </Button>
                            <Button variant="text" type="button" aria-pressed={domain === 'ai'} className={domain === 'ai' ? style.activeDomain : ''} onClick={() => selectDomain('ai')}>
                                <span>AI 工具</span>
                                <strong>{summary.ai}</strong>
                                <small>MCP 与 A2A</small>
                            </Button>
                        </section>

                        {warnings.length > 0 && (
                            <section className={style.partialWarning} role="status">
                                <strong>部分资源暂未加载</strong>
                                <span>{warnings.join('、')}，其余查询结果仍可使用。</span>
                            </section>
                        )}

                        {!loading && !error && namespace?.name === namespaceName && !keyword && !domain && summary.services === 0 && (
                            <section className={style.firstService}>
                                <div><strong>开始接入服务</strong><p>创建环境服务后注册实例，并在实例列表查看健康状态；逻辑服务可在后续按需聚合多个环境。</p></div>
                                <Button theme="primary" onClick={() => openDomain('discovery')}>接入环境服务</Button>
                            </section>
                        )}
                        <ResourceToolbar
                            density="compact"
                            title="资源目录"
                            count={`共 ${filtered.length} 项`}
                            description="可直接查看或删除资源，资源编辑继续在对应领域工作区完成。"
                            filters={(
                                <QueryComposer
                                    keyword={keyword}
                                    keywordPlaceholder="搜索资源名称、类型或摘要"
                                    suggestions={resources.map((item) => item.name)}
                                    fields={[{ key: 'domain', label: '资源域', type: 'select', options: domainOptions }]}
                                    values={{ domain }}
                                    onKeywordChange={(value) => { setKeyword(value); setPage(1); }}
                                    onValuesChange={(values) => selectDomain((values.domain as DomainFilter) || '')}
                                    onSubmit={submitFilter}
                                    onReset={() => { setKeyword(''); selectDomain(''); }}
                                    actions={domain ? (
                                        <Button
                                            variant="outline"
                                            icon={<ArrowRightIcon />}
                                            aria-label={`进入${domainMeta[domain].label}`}
                                            onClick={() => openDomain(domain)}
                                        >
                                            进入{domainMeta[domain].label}
                                        </Button>
                                    ) : undefined}
                                />
                            )}
                        />

                        <section className={style.tableSurface}>
                            <Table
                                data={current}
                                columns={columns}
                                loading={loading}
                                rowKey="key"
                                tableLayout="fixed"
                                cellEmptyContent="-"
                                pagination={{
                                    current: page,
                                    pageSize: PAGE_SIZE,
                                    total: filtered.length,
                                    showJumper: true,
                                    onChange: (pageInfo: { current?: number }) => setPage(pageInfo.current || 1),
                                }}
                            />
                        </section>
                    </div>
                )}
            </Loading>
        </main>
    );
});
