import React from 'react';
import { Button, Table, Space, Tooltip, PrimaryTableProps, TableRowData, Popconfirm, Tabs, Link, Empty } from 'components/Fluent';
import { AddIcon, CreditcardIcon, DeleteIcon, RefreshIcon } from 'components/Fluent/icons';

import RateLimitEditor, { defaultRateLimitView } from './RateLimitEditor';
import { useAppDispatch, useAppSelector } from 'modules/store';
import style from './index.module.less';
import Search from 'components/Search';
import {
    editorRateLimitRule,
    listRateLimitRules,
    listRateLimitRuleVersions,
    removeRateLimitRule,
    removeRateLimitRuleVersion,
    rollbackRateLimitRuleVersion,
    selectRateLimitRule,
} from 'modules/governance/ratelimit';
import { LimitActionMap, LimitType, RateLimitView } from 'services/ratelimit';
import { Op } from 'services/types';
import AuthorizeInput from 'components/Authorize';
import { ResourceToolbar } from 'components/ResourceLayout';
import { PolicySourceType } from 'services/auth_policy';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import RuleTabs from '../RuleRelease/RuleTabs';
import SubscribeTable from 'components/SubscribeTable';
import RuleDetailDrawer, { WIDE_RULE_DETAIL_DRAWER_SIZE } from '../RuleRelease/RuleDetailDrawer';

const { TabPanel } = Tabs;

export interface IRateLimitTableProps {
    limitType: LimitType;
}

const columns = (handleOpRule: (row: TableRowData, op: Op) => void, redirect: (row: TableRowData) => void): PrimaryTableProps['columns'] => [
    {
        colKey: 'name',
        title: '规则名称',
        cell: ({ row }) => (
            <Link
                theme="primary"
                onClick={() => {
                    redirect(row);
                }}
            >
                {row.name}
            </Link>
        ),
    },
    {
        colKey: 'namespace',
        title: '归属环境',
    },
    {
        colKey: 'callee',
        title: '目标服务',
        cell: ({ row }) => `${(row as RateLimitView).callee?.namespace || '-'}/${(row as RateLimitView).callee?.service || '-'}`,
    },
    {
        colKey: 'rulesCount',
        title: '规则数',
        cell: ({ row }) => (Array.isArray((row as RateLimitView).rules) ? (row as RateLimitView).rules.length : 1),
    },
    {
        colKey: 'resource',
        title: '限流资源',
        cell: ({ row }) => {
            const r = row as RateLimitView;
            const rules = r.rules;
            if (rules?.length === 1) return rules[0].resource;
            if (rules?.length) return `${rules[0].resource} 等`;
            return '-';
        },
    },
    {
        colKey: 'action',
        title: '限流行为',
        cell: ({ row }) => {
            const r = row as RateLimitView;
            const rules = r.rules;
            if (rules?.length === 1) return LimitActionMap[rules[0].action];
            if (rules?.length) return '多条';
            return '-';
        },
    },
    {
        colKey: 'operation',
        title: '操作',
        fixed: 'right',
        cell: ({ row }) => (
            <Space>
                <Tooltip content={row.editable === false ? '无权限操作' : '授权'}>
                    <Button
                        shape="square"
                        variant="text"
                        disabled={row.editable === false}
                        onClick={() => handleOpRule(row, 'authorize')}>
                        <CreditcardIcon />
                    </Button>
                </Tooltip>
                <Tooltip content={row.deleteable === false ? '无权限操作' : '删除'}>
                    <Popconfirm
                        content="确认删除吗"
                        destroyOnClose
                        placement="top"
                        showArrow
                        theme="default"
                        onConfirm={() => { handleOpRule(row, 'delete') }}
                    >
                        <Button shape="square" variant="text" disabled={row.deleteable === false}>
                            <DeleteIcon />
                        </Button>
                    </Popconfirm>
                </Tooltip>
            </Space>
        )
    }
]

const RateLimitTable: React.FC<IRateLimitTableProps> = (props) => {
    const dispatch = useAppDispatch();

    const ratelimitState = useAppSelector(selectRateLimitRule);
    const { datas, total, page, limit, loading } = ratelimitState;
    const { versions, versionTotal, versionPage, versionLimit, versionLoading } = ratelimitState;
    const { subscribers } = ratelimitState;

    // 编辑器状态
    const [editor, setEditor] = React.useState<{
        visible: boolean;
        authorizeVisible: boolean;
        mode: Op;
        data?: TableRowData;
    }>({
        visible: false,
        authorizeVisible: false,
        mode: '',
    });

    // 初始化加载
    React.useEffect(() => {
        refreshData();
    }, []);

    const refreshData = (page = 1, limit = 10, query = '') => {
        dispatch(listRateLimitRules({
            param: {
                limit: limit,
                offset: (page - 1) * limit,
                name: query,
                limit_type: props.limitType,
            }
        }))
    }

    const handleOpRule = (row: TableRowData, op: Op) => {
        switch (op) {
            case 'create':
                setEditor((prev) => ({ ...prev, visible: true, mode: 'create' }));
                break;
            case 'view':
                setEditor((prev) => ({ ...prev, visible: true, mode: 'view', data: { ...row } }));
                dispatch(editorRateLimitRule({ ...row } as RateLimitView));
                break;
            case 'delete':
                dispatch(removeRateLimitRule({ ids: [row.id as string] }))
                    .then((res) => {
                        if (res.meta.requestStatus === 'fulfilled') {
                            openInfoNotification("请求成功", "删除限流规则成功");
                            refreshData(1, limit);
                        } else {
                            openErrNotification("请求失败", `删除限流规则失败: ${res.payload as string || '未知'}`);
                        }
                    });
                break;
            case 'authorize':
                setEditor((prev) => ({ ...prev, authorizeVisible: true, mode: 'authorize', data: { ...row } }));
                break;
            default:
                break;
        }
    }

    const refreshVersions = (page = 1, limit = 10) => {
        dispatch(listRateLimitRuleVersions({
            param: {
                offset: (page - 1) * limit,
                limit: limit,
                rule_name: editor.data?.name as string,
            }
        })).then((res) => {
            if (res.meta.requestStatus === 'rejected') {
                openErrNotification("请求失败", `查询路由版本列表错误: ${res.payload as string}`);
            }
        });
    }

    const operateRelease = async (op: Op, row: TableRowData) => {
        switch (op) {
            case 'delete':
                dispatch(removeRateLimitRuleVersion({ id: row.id })).then((res) => {
                    if (res.meta.requestStatus === 'fulfilled') {
                        openInfoNotification("请求成功", "删除限流规则版本成功");
                        refreshVersions(versionPage, versionLimit);
                    } else {
                        openErrNotification("请求失败", `删除限流规则版本失败: ${res.payload as string || '未知'}`);
                    }
                })
                break;
            case 'rollback':
                dispatch(rollbackRateLimitRuleVersion({ id: row.id })).then((res) => {
                    if (res.meta.requestStatus === 'fulfilled') {
                        openInfoNotification("请求成功", "回滚限流规则版本成功");
                        refreshVersions(versionPage, versionLimit);
                    } else {
                        openErrNotification("请求失败", `回滚限流规则版本失败: ${res.payload as string || '未知'}`);
                    }
                })
                break;
        }
    }

    const table = (
        <>
            <ResourceToolbar
                density="compact"
                title="限流规则清单"
                count={loading ? '正在同步列表' : `共 ${total} 条`}
                filters={(
                    <>
                        <Search
                            onChange={(value: string) => {
                                refreshData(1, limit, value);
                            }}
                        />
                        <Tooltip content="刷新">
                            <Button
                                aria-label="刷新限流规则列表"
                                shape="square"
                                variant="outline"
                                icon={<RefreshIcon />}
                                onClick={() => refreshData(1, limit)}
                            />
                        </Tooltip>
                        <Button theme="primary" icon={<AddIcon />} onClick={() => {
                            handleOpRule({}, 'create')
                        }}>新建限流规则</Button>
                    </>
                )}
            />
            {editor.authorizeVisible && (
                <AuthorizeInput
                    resource_type={PolicySourceType.RateLimitRules}
                    resource_id={editor.data?.id as string}
                    resource_name={`ratelimit_rule/${editor.data?.name}`}
                    visible={editor.authorizeVisible}
                    onClose={() => {
                        setEditor((prev) => ({ ...prev, authorizeVisible: false }));
                    }}
                />
            )}
            <Table
                loading={loading}
                data={datas || []}
                columns={columns(handleOpRule, (row: TableRowData) => {
                    handleOpRule(row, 'view');
                })}
                tableLayout='auto'
                rowKey="id"
                cellEmptyContent={'-'}
                pagination={{
                    current: page,
                    pageSize: limit,
                    total: total,
                    showJumper: true,
                    onChange(pageInfo) {
                        refreshData(pageInfo.current, pageInfo.pageSize);
                    },
                }}
                onPageChange={(pageInfo) => {
                    refreshData(pageInfo.current, pageInfo.pageSize);
                }}
            />
        </>
    )

    return (
        <div className={style.ruleWorkspace}>
            <section className={style.ruleListPane}>
                {table}
            </section>
            <RuleDetailDrawer
                visible={editor.visible}
                title={editor.mode === 'create' ? '新建限流规则' : editor.data?.name || '限流规则详情'}
                subtitle="访问限流"
                size={WIDE_RULE_DETAIL_DRAWER_SIZE}
                onClose={() => setEditor((prev) => ({ ...prev, visible: false }))}
            >
                <RuleTabs
                    op={editor.mode}
                    onVersionView={() => {
                        refreshVersions()
                    }}
                    view={
                        <>
                            <RateLimitEditor
                                limitType={props.limitType}
                                visible={editor.visible}
                                op={editor.mode}
                                refresh={(close: boolean) => {
                                    if (close) {
                                        setEditor((prev) => ({ ...prev, visible: false }));
                                    }
                                    refreshData(1, limit);
                                }}
                            />
                        </>
                    }
                    versions={{
                        datas: versions,
                        action: operateRelease,
                        editable: editor.data?.editable ?? true,
                        deleteable: editor.data?.deleteable ?? true,
                        loading: versionLoading,
                        pagination: {
                            defaultCurrent: versionPage,
                            defaultPageSize: versionLimit,
                            total: versionTotal,
                            showJumper: false,
                            onChange(pageInfo) {
                                refreshVersions(pageInfo.current, pageInfo.pageSize);
                            },
                        },
                        onPageChange: (page) => {
                            refreshVersions(page.current, page.pageSize);
                        }
                    }}
                    subscribe={
                        <>
                            <div style={{ marginLeft: 20, marginTop: 20 }}>
                                <SubscribeTable
                                    title={`${editor.data?.name}`}
                                    editable={editor.data?.editable ?? true}
                                    deleteable={editor.data?.deleteable ?? true}
                                    subscribers={subscribers || []}
                                />
                            </div>
                        </>
                    }
                />
            </RuleDetailDrawer>
        </div>
    );
};

export default React.memo(RateLimitTable);
