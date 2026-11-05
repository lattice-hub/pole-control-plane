import React, { useMemo, useState } from 'react';
import { Table, Button, PrimaryTableProps, TableRowData, TabPanel, Tabs, Tag, Tooltip } from 'components/Fluent';
import { AddIcon, RefreshIcon } from 'components/Fluent/icons';

import { useAppDispatch, useAppSelector } from 'modules/store';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import Text from 'components/Text';
import { ConfirmOperationButton, OperationButton, OperationButtonGroup } from 'components/OperationButton';
import { ResourceHeader, ResourceToolbar } from 'components/ResourceLayout';
import QueryComposer, { QuerySnapshot } from 'components/QueryComposer';
import NamespaceEditor from './NamespaceEditor';
import style from './index.module.less';
import { cleanNamespacePage, editorNamespace, listNamespaces, removeNamespace, resetNamespace, selectNamespace } from 'modules/namespace';
import AuthorizeInput from 'components/Authorize';
import ResourceNameLink from 'components/ResourceNameLink';
import { PolicySourceType } from 'services/auth_policy';
import { describeSystemNamespaces, Namespace, NamespaceView } from 'services/namespace';
import { Op } from 'services/types';
import { useNavigate } from 'components/Router';

const DEFAULT_NAMESPACE = 'default';
const SYSTEM_NAMESPACE = 'pole-system';
type NamespaceWorkspace = 'business' | 'system';

const protectedNamespaceReason = (namespace?: Pick<NamespaceView, 'name' | 'kind'>) => {
    if (namespace?.kind === 'SYSTEM' || namespace?.name === SYSTEM_NAMESPACE) return 'Pole 内部系统空间不可删除';
    if (namespace?.name === DEFAULT_NAMESPACE) return '默认命名空间不可删除';
    return '';
};

const columns = (
    operateNamespace: (op: Op, row: TableRowData) => void,
    openEnvironment: (row: TableRowData) => void,
): PrimaryTableProps['columns'] => [
    {
        colKey: 'name',
        title: '名称',
        width: 220,
        cell: ({ row }: TableRowData) => {
            const isDefault = row.name === DEFAULT_NAMESPACE;
            const isSystem = row.kind === 'SYSTEM' || row.name === SYSTEM_NAMESPACE;
            return (
                <div className={style.namespaceNameRow}>
                    <ResourceNameLink
                        name={row.name}
                        ariaLabel={isSystem ? `查看系统空间 ${row.name}` : `查看环境 ${row.name} 的全部资源`}
                        onClick={() => isSystem ? operateNamespace('view', row) : openEnvironment(row)}
                    />
                    {isDefault && <Tag size="small" variant="light">默认空间</Tag>}
                    {isSystem && (
                        <Tooltip content="Pole 内部组件使用的系统命名空间">
                            <Tag size="small" theme="primary" variant="light">内部系统空间</Tag>
                        </Tooltip>
                    )}
                </div>
            );
        },
        fixed: 'left',
    },
    {
        colKey: 'commnet',
        title: '描述',
        width: 205,
        ellipsis: true,
        cell: ({ row: { comment } }: TableRowData) => (
            <div className={style.descriptionCell}>
                <Text>{comment || '-'}</Text>
            </div>
        ),
    },
    {
        colKey: 'totalSerivce',
        title: '服务数',
        width: 70,
        cell: ({ row }: TableRowData) => (
            <div className={style.numberCell}>
                <strong>{row.total_service_count ?? 0}</strong>
                <span>服务</span>
            </div>
        ),
    },
    {
        colKey: 'totalConfigFile',
        title: '配置文件',
        width: 90,
        cell: ({ row }: TableRowData) => (
            <div className={style.numberCell}>
                <strong>{row.total_config_file_count ?? 0}</strong>
                <span>文件</span>
            </div>
        ),
    },
    {
        colKey: 'health/total',
        title: '健康/总数',
        width: 120,
        cell: ({ row: { total_instance_count, total_health_instance_count } }: TableRowData) => {
            const totalInstances = total_instance_count ?? 0;
            const healthInstances = total_health_instance_count ?? 0;
            const percent = totalInstances > 0 ? Math.round((healthInstances / totalInstances) * 100) : 0;
            return (
                <div className={style.healthCell}>
                    <div>
                        <strong>{`${healthInstances}/${totalInstances}`}</strong>
                        <span>{totalInstances > 0 ? `${percent}%` : '无实例'}</span>
                    </div>
                    <div className={style.healthTrack}>
                        <i style={{ width: `${percent}%` }} />
                    </div>
                </div>
            )
        },
    },
    {
        colKey: 'time',
        title: '操作时间',
        width: 145,
        cell: ({ row: { ctime, mtime } }: TableRowData) => (
            <div className={style.timeCell}>
                <span>修改：{mtime}</span>
                <span>创建：{ctime}</span>
            </div>
        ),
    },
    {
        colKey: 'action',
        title: '操作',
        width: 108,
        fixed: 'right',
        cell: ({ row }: TableRowData) => {
            const protectedReason = protectedNamespaceReason(row as NamespaceView);
            return (
                <OperationButtonGroup className={style.actionCell}>
                    <OperationButton action="viewEdit" disabled={row.editable === false} disabledLabel="无权限操作" onClick={() => operateNamespace('view', row)} />
                    <OperationButton action="authorize" disabled={row.editable === false} disabledLabel="无权限操作" onClick={() => operateNamespace('authorize', row)} />
                    <ConfirmOperationButton
                        action="delete"
                        disabled={Boolean(protectedReason) || row.deleteable === false}
                        disabledLabel={protectedReason || '无权限操作'}
                        confirmContent="确认删除吗"
                        onConfirm={() => operateNamespace('delete', row)}
                    />
                </OperationButtonGroup>
            )
        },
    },
]

export default React.memo(() => {
    const dispatch = useAppDispatch();
    const navigate = useNavigate();

    const namespaceState = useAppSelector(selectNamespace);
    const { datas, loading, page, limit, total } = namespaceState;
    const [query, setQuery] = useState('');
    const [activeWorkspace, setActiveWorkspace] = useState<NamespaceWorkspace>('business');
    const [systemNamespaces, setSystemNamespaces] = useState<NamespaceView[]>([]);
    const [systemLoading, setSystemLoading] = useState(false);

    // 合并编辑相关状态
    const [editorState, setEditorState] = useState<{
        visible: boolean;
        mode: 'create' | 'edit' | 'view';
        data?: TableRowData;
        authorizeVisible: boolean;
    }>({ visible: false, mode: 'create', data: undefined, authorizeVisible: false });

    // 编辑、新建事件
    const operateNamespace = (op: Op, row?: TableRowData) => {
        switch (op) {
            case 'edit':
                dispatch(editorNamespace({ ...row as Namespace }));
                setEditorState(prev => ({ ...prev, visible: true, mode: 'edit', data: { ...row } }));
                break;
            case 'view':
                dispatch(editorNamespace({ ...row as Namespace }));
                setEditorState(prev => ({ ...prev, visible: true, mode: 'view', data: { ...row } }));
                break;
            case 'create':
                dispatch(resetNamespace());
                setEditorState(prev => ({ ...prev, visible: true, mode: 'create', data: undefined }));
                break;
            case 'authorize':
                setEditorState(prev => ({ ...prev, authorizeVisible: true, data: { ...row } }));
                break;
            case 'delete':
                if (protectedNamespaceReason(row as NamespaceView)) {
                    openErrNotification('无法删除命名空间', protectedNamespaceReason(row as NamespaceView));
                    break;
                }
                dispatch(removeNamespace({ param: { name: row?.name as string } }))
                    .then((res) => {
                        if (res.meta.requestStatus === 'fulfilled') {
                            openInfoNotification('请求成功', '删除命名空间成功');
                            refreshTable(page, limit, query);
                        } else {
                            openErrNotification('请求失败', res.payload as string);
                        }
                    });
                break;
        }
    }

    const refreshTable = (page = 1, limit = 10, query = '') => {
        dispatch(listNamespaces({
            param: {
                offset: (page - 1) * limit,
                limit: limit,
                name: query || undefined,
                kind: 'business',
            }
        })).then((res) => {
            if (res.meta.requestStatus === 'rejected') {
                openErrNotification("请求失败", res.payload as string);
            }
        });
    }

    const refreshSystemNamespaces = () => {
        setSystemLoading(true);
        describeSystemNamespaces()
            .then(setSystemNamespaces)
            .catch(error => openErrNotification('加载系统空间失败', error))
            .finally(() => setSystemLoading(false));
    };

    React.useEffect(() => {
        refreshTable();
        refreshSystemNamespaces();
        return () => {
            // 清理编辑器状态
            dispatch(cleanNamespacePage());
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const metrics = useMemo(() => {
        const serviceCount = datas.reduce((sum, item) => sum + (item.total_service_count || 0), 0);
        const configFileCount = datas.reduce((sum, item) => sum + (item.total_config_file_count || 0), 0);
        const instanceCount = datas.reduce((sum, item) => sum + (item.total_instance_count || 0), 0);
        const healthCount = datas.reduce((sum, item) => sum + (item.total_health_instance_count || 0), 0);
        const healthRate = instanceCount > 0 ? `${Math.round((healthCount / instanceCount) * 100)}%` : '-';
        return { serviceCount, configFileCount, instanceCount, healthCount, healthRate };
    }, [datas]);

    const systemMetrics = useMemo(() => {
        const serviceCount = systemNamespaces.reduce((sum, item) => sum + (item.total_service_count || 0), 0);
        const configFileCount = systemNamespaces.reduce((sum, item) => sum + (item.total_config_file_count || 0), 0);
        const instanceCount = systemNamespaces.reduce((sum, item) => sum + (item.total_instance_count || 0), 0);
        const healthCount = systemNamespaces.reduce((sum, item) => sum + (item.total_health_instance_count || 0), 0);
        const healthRate = instanceCount > 0 ? `${Math.round((healthCount / instanceCount) * 100)}%` : '-';
        return { serviceCount, configFileCount, instanceCount, healthCount, healthRate };
    }, [systemNamespaces]);

    const submitFilter = ({ keyword }: QuerySnapshot) => {
        refreshTable(1, limit, keyword);
    };

    const openEnvironment = (row: TableRowData) => {
        navigate(`/namespace/environment?name=${encodeURIComponent(String(row.name || ''))}`);
    };

    const resetFilter = () => {
        setQuery('');
        refreshTable(1, limit, '');
    };

    const table = (
        <>
            <ResourceHeader
                density="compact"
                placement="app-header"
                eyebrow="Environment / Namespace"
                title="环境与系统空间"
                description="业务 Namespace 是相互隔离的运行环境；Pole 系统空间属于当前控制面，不参与业务资源的跨环境聚合。"
            />

            <section className={style.namespaceWorkspace}>
                <Tabs
                    className={style.namespaceTabs}
                    value={activeWorkspace}
                    onChange={(value: NamespaceWorkspace) => setActiveWorkspace(value)}
                >
                    <TabPanel value="business" label={`业务环境 (${total})`} className={style.workspacePanel}>
                        <section className={style.metricRail}>
                            <div className={style.metricItem}>
                                <span>业务环境</span>
                                <strong>{total}</strong>
                            </div>
                            <div className={style.metricItem}>
                                <span>当前页服务</span>
                                <strong>{metrics.serviceCount}</strong>
                            </div>
                            <div className={style.metricItem}>
                                <span>当前页配置文件</span>
                                <strong>{metrics.configFileCount}</strong>
                            </div>
                            <div className={style.metricItem}>
                                <span>健康实例</span>
                                <strong>{metrics.healthCount}/{metrics.instanceCount}</strong>
                            </div>
                            <div className={style.metricItem}>
                                <span>实例健康率</span>
                                <strong>{metrics.healthRate}</strong>
                            </div>
                        </section>

                        <ResourceToolbar
                            density="compact"
                            title="业务环境"
                            count={loading ? '正在同步列表' : `当前显示 ${datas.length} 条`}
                            filters={(
                                <>
                                    <QueryComposer
                                        keyword={query}
                                        keywordPlaceholder="搜索命名空间名称"
                                        suggestions={datas.map((item) => String(item.name || '')).filter(Boolean)}
                                        onKeywordChange={setQuery}
                                        onSubmit={submitFilter}
                                        onReset={resetFilter}
                                        actions={(
                                            <>
                                                <Button variant="outline" onClick={() => navigate('/namespace/promotion-topology')}>
                                                    晋升拓扑
                                                </Button>
                                                <Tooltip content="刷新业务环境">
                                                    <Button
                                                        aria-label="刷新业务环境"
                                                        shape="square"
                                                        variant="outline"
                                                        icon={<RefreshIcon />}
                                                        onClick={() => refreshTable(page, limit, query)}
                                                    />
                                                </Tooltip>
                                                <Button theme="primary" icon={<AddIcon />} onClick={() => operateNamespace('create')}>
                                                    新建业务环境
                                                </Button>
                                            </>
                                        )}
                                    />
                                </>
                            )}
                        />

                        <section className={`${style.tableSurface} ${style.namespaceTableSurface}`}>
                            <Table
                                data={datas}
                                columns={columns(operateNamespace, openEnvironment)}
                                loading={loading}
                                rowKey="name"
                                size="large"
                                tableLayout="fixed"
                                cellEmptyContent="-"
                                pagination={{
                                    current: page,
                                    pageSize: limit,
                                    total: total,
                                    showJumper: true,
                                    onChange(pageInfo) {
                                        refreshTable(pageInfo.current, pageInfo.pageSize, query);
                                    },
                                }}
                            />
                        </section>
                    </TabPanel>

                    <TabPanel value="system" label={`系统空间 (${systemNamespaces.length})`} className={style.workspacePanel}>
                        <section className={style.metricRail}>
                            <div className={style.metricItem}>
                                <span>系统空间</span>
                                <strong>{systemNamespaces.length}</strong>
                            </div>
                            <div className={style.metricItem}>
                                <span>系统服务</span>
                                <strong>{systemMetrics.serviceCount}</strong>
                            </div>
                            <div className={style.metricItem}>
                                <span>配置文件</span>
                                <strong>{systemMetrics.configFileCount}</strong>
                            </div>
                            <div className={style.metricItem}>
                                <span>健康实例</span>
                                <strong>{systemMetrics.healthCount}/{systemMetrics.instanceCount}</strong>
                            </div>
                            <div className={style.metricItem}>
                                <span>实例健康率</span>
                                <strong>{systemMetrics.healthRate}</strong>
                            </div>
                        </section>

                        <div className={style.systemNamespaceHeader}>
                            <div>
                                <strong>Pole 系统空间（当前控制面）</strong>
                                <span>承载 Pole 内部服务、MCP、Agent 与系统配置；继承当前部署阶段，不参与业务跨环境聚合。</span>
                            </div>
                            <div className={style.systemNamespaceActions}>
                                <Button size="small" variant="outline" onClick={() => navigate('/discovery/service?scope=system&namespace=pole-system')}>
                                    查看系统服务
                                </Button>
                                <Button size="small" variant="outline" onClick={() => navigate('/configuration?environment=pole-system')}>
                                    查看配置资源
                                </Button>
                                <Button size="small" variant="outline" onClick={() => navigate('/system-configuration')}>
                                    维护系统配置
                                </Button>
                                <Button size="small" variant="outline" onClick={() => navigate('/ai/mcps')}>
                                    查看 MCP
                                </Button>
                                <Button size="small" variant="outline" onClick={() => navigate('/ai/a2a')}>
                                    查看 Agent
                                </Button>
                            </div>
                        </div>

                        <ResourceToolbar
                            density="compact"
                            title="系统空间清单"
                            count={systemLoading ? '正在同步列表' : `共 ${systemNamespaces.length} 条`}
                            filters={(
                                <Tooltip content="刷新系统空间">
                                    <Button
                                        aria-label="刷新系统空间"
                                        shape="square"
                                        variant="outline"
                                        icon={<RefreshIcon />}
                                        onClick={refreshSystemNamespaces}
                                    />
                                </Tooltip>
                            )}
                        />

                        <section className={`${style.tableSurface} ${style.namespaceTableSurface}`}>
                            <Table
                                data={systemNamespaces}
                                columns={columns(operateNamespace, openEnvironment)}
                                loading={systemLoading}
                                rowKey="name"
                                size="large"
                                tableLayout="fixed"
                                cellEmptyContent="-"
                            />
                        </section>
                    </TabPanel>
                </Tabs>

                {editorState.visible && (
                    <NamespaceEditor
                        key={editorState.mode + (editorState.data?.name || 'new') + (editorState.visible ? '1' : '0')}
                        op={editorState.mode}
                        visible={editorState.visible}
                        closeDrawer={() => {
                            dispatch(resetNamespace());
                            setEditorState(s => ({ ...s, visible: false }))
                            refreshTable(page, limit, query);
                            refreshSystemNamespaces();
                        }}
                    />
                )}
                {editorState.authorizeVisible && (
                    <AuthorizeInput
                        resource_type={PolicySourceType.Namespaces}
                        resource_id={editorState.data?.name}
                        resource_name={`${editorState.data?.name}`}
                        visible={editorState.authorizeVisible}
                        onClose={() => {
                            setEditorState(s => ({ ...s, authorizeVisible: false }));
                        }}
                    />
                )}
            </section>
        </>
    );

    return (
        <div className={style.page}>
            {table}
        </div>
    )
});
