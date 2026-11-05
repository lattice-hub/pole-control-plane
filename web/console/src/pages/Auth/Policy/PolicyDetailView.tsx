import React from 'react';
import { Button, Drawer, Input, Loading, PrimaryTableProps, Space, Table, TableRowData, Tabs, Tag } from 'components/Fluent';
import { CopyIcon, SearchIcon } from 'components/Fluent/icons';

import style from './index.module.less';
import { describeAuthPolicyDetail, PolicyResource, PolicyResourceLabel, PolicyRule } from 'services/auth_policy';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import { copyToClipboard } from 'utils/sys';
import ErrorPage from 'components/ErrorPage';
import ResourceNameLink from 'components/ResourceNameLink';

const { TabPanel } = Tabs;

const resourceTypeOptions = [
    { label: '命名空间', value: 'namespaces' },
    { label: '服务', value: 'services' },
    { label: '路由规则', value: 'route_rules' },
    { label: '泳道规则', value: 'lane_rules' },
    { label: '熔断规则', value: 'circuitbreaker_rules' },
    { label: '主动探测规则', value: 'faultdetect_rules' },
    { label: '无损规则', value: 'lossless_rules' },
    { label: '限流规则', value: 'ratelimit_rules' },
    { label: '调用鉴权规则', value: 'security_rules' },
    { label: '流量镜像规则', value: 'mirror_rules' },
    { label: '流量 Mock 规则', value: 'mock_rules' },
    { label: '用户', value: 'users' },
    { label: '用户组', value: 'user_groups' },
    { label: '资源鉴权规则', value: 'auth_policies' },
    { label: '角色', value: 'roles' },
    { label: 'MCP Server', value: 'mcp_servers' },
    { label: 'A2A Agent', value: 'a2a_agents' },
];

type ResourceFilter = 'all' | 'inherited';

interface ResourceRow extends TableRowData {
    rowKey: string;
    id?: string;
    name?: string;
    namespace?: string;
    type: string;
    mode: ResourceFilter;
    modeLabel: string;
    scopeLabel: string;
}

interface PrincipalRow {
    id: string;
    name: string;
    type: string;
    bindType: string;
    status: string;
}

interface IPolicyDetailViewProps {
    policyId?: string;
}

const emptyRule = {} as PolicyRule;

const formatValue = (value?: string) => value || '-';

const getActionTheme = (action?: string) => (action === 'ALLOW' ? 'success' : 'danger');

const getActionLabel = (action?: string) => (action === 'ALLOW' ? '允许' : '拒绝');

const isInheritedResource = (item: PolicyResource) => item.id === '*' || item.name === '*';

const getResourceTypeLabel = (type: string) => resourceTypeOptions.find(item => item.value === type)?.label || type;

const normalizeResources = (rule: PolicyRule) => {
    const resources = [] as ResourceRow[];
    if (!rule.resources) {
        return resources;
    }
    Object.entries(rule.resources).forEach(([key, value]) => {
        if (!value) return;
        (value as PolicyResource[]).forEach((item, index) => {
            const inherited = isInheritedResource(item);
            const resourceName = item.name === '*' ? '全部（包括新增）' : item.name || item.id || '-';
            resources.push({
                rowKey: `${key}-${item.id || 'empty'}-${index}`,
                id: item.id,
                name: resourceName,
                namespace: item.namespace,
                type: key,
                mode: inherited ? 'inherited' : 'all',
                modeLabel: inherited ? '包括新增' : '显式资源',
                scopeLabel: inherited ? '覆盖新增资源' : '仅显式资源',
            });
        });
    });
    return resources;
};

const normalizePrincipals = (rule: PolicyRule) => {
    const rows = [] as PrincipalRow[];
    const pushRows = (items: Array<{ id: string; name?: string }> | undefined, type: string) => {
        (items || []).forEach(item => rows.push({
            id: item.id,
            name: item.name || item.id,
            type,
            bindType: rule.default_strategy ? '默认策略自动绑定' : '策略显式绑定',
            status: '已绑定',
        }));
    };
    pushRows(rule.principals?.users, '用户');
    pushRows(rule.principals?.groups, '用户组');
    pushRows(rule.principals?.roles, '角色');
    return rows;
};

const getMetadataEntries = (metadata?: Record<string, string>) => Object.entries(metadata || {});

const PolicyDetailView: React.FC<IPolicyDetailViewProps> = ({ policyId }) => {
    const [viewState, setViewState] = React.useState<{
        loading: boolean;
        rule: PolicyRule;
        fetchError: boolean;
        allResources: ResourceRow[];
        activeResourceType: string;
        resourceTypeSearch: string;
        resourceFilter: ResourceFilter;
        selectedResourceRowKeys: Array<string | number>;
    }>({
        loading: false,
        rule: emptyRule,
        fetchError: false,
        allResources: [],
        activeResourceType: 'namespaces',
        resourceTypeSearch: '',
        resourceFilter: 'all',
        selectedResourceRowKeys: [],
    });
    const [selectedResource, setSelectedResource] = React.useState<ResourceRow | null>(null);

    React.useEffect(() => {
        if (!policyId) {
            return;
        }
        fetchData();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [policyId]);

    async function fetchData() {
        setViewState(prev => ({ ...prev, loading: true, fetchError: false }));
        try {
            const ret = await describeAuthPolicyDetail({ id: policyId as string });
            if (!ret?.strategy) {
                setViewState(prev => ({ ...prev, loading: false }));
                return;
            }
            const expectRule = ret.strategy;
            const allResources = normalizeResources(expectRule);
            setViewState(prev => {
                const activeResourceType = allResources.some(item => item.type === prev.activeResourceType)
                    ? prev.activeResourceType
                    : allResources[0]?.type || resourceTypeOptions[0].value;
                return {
                    ...prev,
                    loading: false,
                    rule: expectRule,
                    allResources,
                    activeResourceType,
                    resourceFilter: 'all',
                    selectedResourceRowKeys: [],
                    fetchError: false,
                };
            });
        } catch (error) {
            setViewState(prev => ({ ...prev, loading: false, fetchError: true }));
            openErrNotification('获取策略详情失败', error as string);
        }
    }

    const principalRows = normalizePrincipals(viewState.rule);
    const metadataEntries = getMetadataEntries(viewState.rule.metadata);
    const currentTypeRows = viewState.allResources.filter(item => item.type === viewState.activeResourceType);
    const filteredResources = viewState.resourceFilter === 'inherited'
        ? currentTypeRows.filter(item => item.mode === 'inherited')
        : currentTypeRows;
    const selectedResourceCount = viewState.selectedResourceRowKeys.length;
    const activeResourceTypeLabel = getResourceTypeLabel(viewState.activeResourceType);
    const firstResource = filteredResources[0];
    const visibleResourceTypes = resourceTypeOptions.filter(item => item.label.includes(viewState.resourceTypeSearch.trim()));
    const functionRows = (viewState.rule.functions || []).map((item, index) => ({
        rowKey: `${item}-${index}`,
        group: item === '*' ? '全部 Console API' : item.split('/')[1] || 'Console API',
        scope: item === '*' ? '全部接口（包括新增）' : item,
        status: getActionLabel(viewState.rule.action),
    }));

    const copyMember = () => {
        copyToClipboard(principalRows.map(item => `${item.type}:${item.name}`).join('\n') || '-');
    };

    const copyResourceId = (id?: string) => {
        copyToClipboard(id || '');
    };

    const copyResourceRows = () => {
        copyToClipboard(filteredResources.map(item => `${item.id || '-'}\t${item.name || '-'}\t${item.modeLabel}`).join('\n') || '-');
    };

    const setResourceFilter = (resourceFilter: ResourceFilter) => {
        setViewState(prev => ({
            ...prev,
            resourceFilter,
            selectedResourceRowKeys: [],
        }));
        openInfoNotification('筛选已更新', resourceFilter === 'inherited' ? '仅展示新增继承资源' : '展示全部资源');
    };

    const switchResourceType = (value: string) => {
        setViewState(prev => ({
            ...prev,
            activeResourceType: value,
            selectedResourceRowKeys: [],
        }));
    };

    const renderSummary = () => (
        <section className={style.policySummary}>
            <div className={style.policySummaryLabel}>策略概要</div>
            <div className={style.policySummaryTop}>
                <div className={style.policyBadge}>策</div>
                <div className={style.policySummaryTitle}>
                    <div className={style.policySummaryName}>{formatValue(viewState.rule.name)}</div>
                    <div className={style.policySummaryDesc}>{formatValue(viewState.rule.comment)}</div>
                </div>
            </div>
            <div className={style.policyMetaGrid}>
                <div className={style.policyMetaItem}>
                    <span>创建时间</span>
                    <strong>{formatValue(viewState.rule.ctime)}</strong>
                </div>
                <div className={style.policyMetaItem}>
                    <span>更新时间</span>
                    <strong>{formatValue(viewState.rule.mtime)}</strong>
                </div>
                <div className={style.policyMetaItem}>
                    <span>策略标签</span>
                    <strong>{metadataEntries.length ? `${metadataEntries.length} 个` : '无'}</strong>
                </div>
            </div>
        </section>
    );

    const renderPrincipals = () => (
        <div className={style.policyTabBody}>
            <div className={style.policySectionHeader}>
                <div>
                    <h3>成员信息</h3>
                    <p>展示策略绑定到谁，以及策略如何对成员生效。</p>
                </div>
                <Button variant="outline" icon={<CopyIcon />} onClick={copyMember}>
                    复制成员
                </Button>
            </div>
            {principalRows.length ? (
                <div className={style.memberGrid}>
                    {principalRows.map(item => (
                        <div key={`${item.type}-${item.id}`} className={style.memberCard}>
                            <div>
                                <span>{item.type}</span>
                                <strong>{item.name}</strong>
                            </div>
                            <div className={style.memberMeta}>
                                <em>{item.bindType}</em>
                                <Tag theme="success" variant="light">{item.status}</Tag>
                                {viewState.rule.default_strategy && <Tag theme="primary" variant="light">默认成员</Tag>}
                            </div>
                            <div className={style.memberId}>{item.id}</div>
                        </div>
                    ))}
                </div>
            ) : (
                <div className={style.emptyDetail}>暂无关联用户、用户组或角色。</div>
            )}
            <div className={style.policyFlowWrap}>
                <h3>生效路径</h3>
                <div className={style.policyFlow}>
                    <div><span>1</span><strong>匹配成员</strong><p>按用户、用户组或角色命中策略主体。</p></div>
                    <div><span>2</span><strong>读取策略效果</strong><p>当前策略效果为 {getActionLabel(viewState.rule.action)}。</p></div>
                    <div><span>3</span><strong>进入资源范围</strong><p>根据资源类型和授权范围判断访问能力。</p></div>
                </div>
            </div>
            <div className={style.policyDefinition}>
                <div><span>策略效果</span><strong>{getActionLabel(viewState.rule.action)}</strong></div>
                <div><span>默认策略</span><strong>{viewState.rule.default_strategy ? '是' : '否'}</strong></div>
                <div><span>描述</span><strong>{formatValue(viewState.rule.comment)}</strong></div>
            </div>
        </div>
    );

    const resourceColumns: PrimaryTableProps['columns'] = [
        {
            colKey: 'id',
            title: '资源 ID',
            width: 210,
            cell: ({ row }) => <span className={style.mono}>{row.id === '*' ? '全部（包括新增）' : row.id || '-'}</span>,
        },
        {
            colKey: 'name',
            title: '资源名称',
            width: 220,
            minWidth: 180,
            ellipsis: true,
            cell: ({ row }) => (
                <ResourceNameLink
                    className={style.policyResourceNameLink}
                    name={row.name}
                    onClick={() => setSelectedResource(row as ResourceRow)}
                />
            ),
        },
        {
            colKey: 'modeLabel',
            title: '授权方式',
            width: 120,
            cell: ({ row }) => <Tag theme={row.mode === 'inherited' ? 'primary' : 'warning'} variant="light">{row.modeLabel}</Tag>,
        },
        {
            colKey: 'operation',
            title: '操作',
            width: 76,
            cell: ({ row }) => (
                <Button shape="square" variant="text" aria-label="复制资源 ID" icon={<CopyIcon />} onClick={() => copyResourceId(row.id)} />
            ),
        },
    ];

    const renderResources = () => (
        <div className={style.policyResourceShell}>
            <aside className={style.policyResourceNav}>
                <Input
                    className={style.resourceTypeSearch}
                    clearable
                    prefixIcon={<SearchIcon />}
                    placeholder="搜索资源类型"
                    value={viewState.resourceTypeSearch}
                    onChange={(value) => setViewState(prev => ({ ...prev, resourceTypeSearch: String(value) }))}
                />
                <div className={style.resourceFilter}>
                    <Button size="small" theme={viewState.resourceFilter === 'all' ? 'primary' : 'default'} variant={viewState.resourceFilter === 'all' ? 'base' : 'outline'} onClick={() => setResourceFilter('all')}>
                        全部资源
                    </Button>
                    <Button size="small" theme={viewState.resourceFilter === 'inherited' ? 'primary' : 'default'} variant={viewState.resourceFilter === 'inherited' ? 'base' : 'outline'} onClick={() => setResourceFilter('inherited')}>
                        仅看新增继承
                    </Button>
                </div>
                <div className={style.resourceTypeList}>
                    {visibleResourceTypes.map(item => {
                        const rows = viewState.allResources.filter(row => row.type === item.value);
                        const inherited = rows.some(row => row.mode === 'inherited');
                        return (
                            <Button
                                variant="text"
                                key={item.value}
                                type="button"
                                className={viewState.activeResourceType === item.value ? style.activeResourceType : undefined}
                                onClick={() => switchResourceType(item.value)}
                            >
                                <i className={inherited ? style.inheritedDot : style.explicitDot} />
                                <span>
                                    <strong>{item.label}</strong>
                                    <em>{inherited ? '覆盖新增资源' : '仅显式资源'}</em>
                                </span>
                                <b>{rows.length}</b>
                            </Button>
                        );
                    })}
                </div>
            </aside>
            <main className={style.policyResourceMain}>
                <div className={style.policySectionHeader}>
                    <div>
                        <h3>{activeResourceTypeLabel}</h3>
                        <p>当前资源类型下的授权范围与资源清单。</p>
                    </div>
                    <Button variant="outline" icon={<CopyIcon />} onClick={copyResourceRows}>
                        复制清单
                    </Button>
                </div>
                <div className={style.policyResourceBrief}>
                    <div><span>授权范围</span><strong>{firstResource?.scopeLabel || '-'}</strong></div>
                    <div><span>授权方式</span><strong>{firstResource?.modeLabel || '-'}</strong></div>
                    <div><span>当前选择</span><strong>{selectedResourceCount} 项</strong></div>
                </div>
                {filteredResources.length ? (
                    <Table
                        data={filteredResources}
                        className={style.policyResourceTable}
                        columns={resourceColumns}
                        rowKey="rowKey"
                        size="medium"
                        tableLayout="fixed"
                        cellEmptyContent="-"
                        selectedRowKeys={viewState.selectedResourceRowKeys}
                        onSelectChange={(value) => setViewState(prev => ({ ...prev, selectedResourceRowKeys: value }))}
                        pagination={{
                            defaultCurrent: 1,
                            defaultPageSize: 10,
                            showJumper: true,
                            total: filteredResources.length,
                        }}
                    />
                ) : (
                    <div className={style.emptyDetail}>
                        当前资源类型没有“新增继承”范围，可切回全部资源查看显式授权。
                    </div>
                )}
            </main>
        </div>
    );

    const renderResourceLabels = () => {
        const labels = viewState.rule.resource_labels || [];
        if (!labels.length) {
            return (
                <div className={style.emptyDetail}>
                    暂无资源标签。默认策略通过资源类型和资源范围生效。
                </div>
            );
        }
        return (
            <Table
                data={labels}
                columns={[
                    {
                        colKey: 'key',
                        title: '标签 Key',
                        cell: ({ row }: { row: PolicyResourceLabel }) => <span>{row.key}</span>,
                    },
                    {
                        colKey: 'value',
                        title: '标签值',
                        cell: ({ row }: { row: PolicyResourceLabel }) => <span>{row.value}</span>,
                    },
                    {
                        colKey: 'compare_type',
                        title: '匹配方式',
                        cell: ({ row }: { row: PolicyResourceLabel }) => <span>{row.compare_type}</span>,
                    },
                ]}
                rowKey="key"
                size="medium"
                tableLayout="fixed"
                cellEmptyContent="-"
                pagination={{
                    defaultCurrent: 1,
                    defaultPageSize: 10,
                    showJumper: true,
                    total: labels.length,
                }}
            />
        );
    };

    const renderFunctions = () => (
        <Table
            data={functionRows}
            columns={[
                { colKey: 'group', title: '接口分组', width: 30, ellipsis: true },
                { colKey: 'scope', title: '访问范围', width: 50, ellipsis: true },
                {
                    colKey: 'status',
                    title: '状态',
                    width: 20,
                    cell: ({ row }) => <Tag theme={getActionTheme(viewState.rule.action)} variant="light">{row.status}</Tag>,
                },
            ]}
            rowKey="rowKey"
            size="medium"
            tableLayout="fixed"
            cellEmptyContent="-"
        />
    );

    return (
        <>
            <Loading className={style.policyDetailLoading} indicator loading={viewState.loading} preventScrollThrough showOverlay>
                {viewState.fetchError ? (
                    <ErrorPage code={500} />
                ) : (
                    <div className={style.policyDetailView}>
                    <div className={style.policyDetailHeader}>
                        <Tag theme={getActionTheme(viewState.rule.action)} variant="light">{getActionLabel(viewState.rule.action)}</Tag>
                        {viewState.rule.default_strategy && <Tag theme="primary" variant="outline">默认策略</Tag>}
                    </div>
                    {renderSummary()}
                    <Tabs className={style.policyDetailTabs} defaultValue="principal">
                        <TabPanel value="principal" label="成员信息">
                            {renderPrincipals()}
                        </TabPanel>
                        <TabPanel value="resource" label="资源信息">
                            {renderResources()}
                        </TabPanel>
                        <TabPanel value="resource-labels" label="资源标签">
                            {renderResourceLabels()}
                        </TabPanel>
                        <TabPanel value="function" label="可访问接口">
                            {renderFunctions()}
                        </TabPanel>
                    </Tabs>
                    </div>
                )}
            </Loading>
            <Drawer
                visible={Boolean(selectedResource)}
                header={selectedResource?.name || '资源详情'}
                footer={false}
                size="small"
                onClose={() => setSelectedResource(null)}
            >
                {selectedResource && (
                    <div className={style.resourceReferenceDetail}>
                        <div><span>资源名称</span><strong>{selectedResource.name || '-'}</strong></div>
                        <div><span>资源类型</span><strong>{getResourceTypeLabel(selectedResource.type)}</strong></div>
                        <div><span>资源 ID</span><strong>{selectedResource.id === '*' ? '全部（包括新增）' : selectedResource.id || '-'}</strong></div>
                        <div><span>命名空间</span><strong>{selectedResource.namespace || '-'}</strong></div>
                        <div><span>授权方式</span><strong>{selectedResource.modeLabel}</strong></div>
                        <div><span>授权范围</span><strong>{selectedResource.scopeLabel}</strong></div>
                    </div>
                )}
            </Drawer>
        </>
    );
};

export default React.memo(PolicyDetailView);
