import React, { useEffect, useState } from 'react';
import { Table, Button, PageInfo, Popconfirm, PrimaryTableProps, TableProps, Space, TableRowData, Tag, Tooltip } from 'components/Fluent';
import { AddIcon, RefreshIcon } from 'components/Fluent/icons';
import { useNavigate } from 'components/Router';

import Search from 'components/Search';
import ErrorPage from 'components/ErrorPage';
import Text from 'components/Text';
import { ConfirmOperationButton, OperationButton } from 'components/OperationButton';
import ResourceNameLink from 'components/ResourceNameLink';
import { ResourceToolbar } from 'components/ResourceLayout';
import { useAppDispatch } from 'modules/store';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import style from './index.module.less';
import { describeUserGroups, describeUserGroupToken } from 'services/user_group';
import ShowToken from './ShowToken';
import GroupEditor from './GroupEditor';
import { editorUserGroup, removeUserGroups } from 'modules/user/groups';

interface IUsersProps {
    onTotalChange?: (total: number) => void;
    onTotalsRefresh?: () => void;
}

const ServerError = () => <ErrorPage code={500} />;

const columns = (
    handleEditGroup: (row: TableRowData, op: 'view' | 'create' | 'edit' | 'delete', res: string) => void,
    openGroupDetail: (row: TableRowData) => void,
): PrimaryTableProps['columns'] => [
    {
        colKey: 'id',
        title: 'ID',
        type: 'multiple',
        checkProps: ({ row }) => ({ disabled: row.editable === false || row.deleteable === false }),
    },
    {
        colKey: 'name',
        title: '名称',
        cell: ({ row }) => (
            <ResourceNameLink name={row.name} onClick={() => openGroupDetail(row)} />
        ),
    },
    {
        colKey: 'commnet',
        title: '描述',
        ellipsis: true,
        cell: ({ row: { comment } }: TableRowData) => (<Text>{comment || '-'}</Text>),
    },
    {
        colKey: 'token_enable',
        title: 'Token 状态',
        cell: ({ row: { token_enable } }: TableRowData) => (<Tag theme={token_enable ? 'success' : 'danger'}>{token_enable ? '启用' : '禁用'}</Tag>),
    },
    {
        colKey: 'time',
        title: '操作时间',
        cell: ({ row: { ctime, mtime } }: TableRowData) => <Text>修改: {mtime}<br />创建: {ctime}</Text>,
    },
    {
        colKey: 'action',
        title: '操作',
        cell: ({ row }) => {
            return (
                <Space>
                    <OperationButton action="view" onClick={() => openGroupDetail(row)} />
                    <OperationButton
                        action="edit"
                        disabled={row.editable === false}
                        disabledLabel="无权限操作"
                        onClick={() => handleEditGroup(row, 'edit', 'group')}
                    />
                    <OperationButton
                        action="token"
                        disabled={row.editable === false}
                        disabledLabel="无权限操作"
                        onClick={() => {
                            describeUserGroupToken({ id: row.id }).then((res) => {
                                if (res?.userGroup) {
                                    handleEditGroup({ ...row, auth_token: res.userGroup.auth_token }, 'view', 'group_token');
                                }
                            });
                        }}
                    />
                    <ConfirmOperationButton action="delete" disabled={row.deleteable === false} disabledLabel="无权限操作" confirmContent="确认删除吗" onConfirm={() => handleEditGroup(row, 'delete', 'group')} />
                </Space>
            )
        },
    },
]


const GroupsTable: React.FC<IUsersProps> = ({ onTotalChange, onTotalsRefresh }) => {
    const dispatch = useAppDispatch();
    const navigate = useNavigate();
    const [selectedRowKeys, setSelectedRowKeys] = useState<Array<string | number>>([]);

    // 合并编辑相关状态
    const [searchState, setSearchState] = useState<{
        groups: TableProps['data'];
        current: number;
        limit: number;
        previous: number;
        total: number;
        query: string;
        fetchError: boolean;
        isLoading: boolean;
    }>({ groups: [], current: 1, limit: 10, previous: 0, total: 0, query: '', fetchError: false, isLoading: false });

    // 合并编辑相关状态
    const [editorState, setEditorState] = useState<{
        visible: boolean;
        mode: 'create' | 'edit' | 'view';
        data?: TableRowData;
        resource: 'group' | 'group_token';
    }>({ visible: false, mode: 'create', data: undefined, resource: 'group' });

    const openGroupDetail = (row: TableRowData) => {
        navigate(`/auth/principals/groupdetail?name=${encodeURIComponent(String(row.name || ''))}&id=${encodeURIComponent(String(row.id || ''))}`);
    };

    const handleBatchDeleteGroups = async (ids: string[]) => {
        if (!ids.length) {
            return;
        }
        setSearchState(s => ({ ...s, isLoading: true }));
        const result = await dispatch(removeUserGroups({ ids }));
        if (result.meta.requestStatus === 'fulfilled') {
            openInfoNotification('请求成功', `已删除 ${ids.length} 个用户组`);
            setSelectedRowKeys([]);
            await fetchData({ current: 1, pageSize: searchState.limit, previous: 0 }, searchState.query);
            onTotalsRefresh?.();
            return;
        }
        setSearchState(s => ({ ...s, isLoading: false }));
        openErrNotification('删除用户组失败', String(result.payload || '未知错误'));
    };

    // 编辑、新建事件
    const handleEditUserGroup = (row: TableRowData, mode: 'view' | 'create' | 'edit' | 'delete', res: string) => {
        if (mode === 'delete') {
            void handleBatchDeleteGroups([row.id as string]);
            return;
        }
        setEditorState({
            visible: true,
            mode: mode === 'create' ? 'create' : mode === 'edit' ? 'edit' : 'view',
            data: { ...row },
            resource: res as 'group' | 'group_token',
        })
        if (res === 'group_token') {
            return;
        }
        dispatch(editorUserGroup({
            id: row.id,
            name: row.name,
            comment: row.comment,
            token_enable: row.token_enable,
            relation: row.relation,
            metadata: row.metadata,
        }))
    }

    const handleCreateUserGroup = () => {
        setEditorState({
            visible: true,
            mode: 'create',
            data: undefined,
            resource: 'group',
        });
    };

    // 模拟远程请求
    async function fetchData(pageInfo: PageInfo, searchParam?: string) {
        const query = searchParam ?? searchState.query;
        setSearchState(s => ({ ...s, current: pageInfo.current, limit: pageInfo.pageSize, previous: pageInfo.previous, query, fetchError: false, isLoading: true }));
        try {
            const { current, pageSize } = pageInfo;
            // 请求可能存在跨域问题
            const response = await describeUserGroups({
                limit: pageSize, offset: (current - 1) * pageSize, ...(query && { name: query })
            });
            setSearchState(s => ({ ...s, groups: response.content, total: response.totalCount, isLoading: false }));
            if (!query) onTotalChange?.(response.totalCount);
        } catch (error: Error | any) {
            setSearchState(s => ({ ...s, fetchError: true, isLoading: false }));
            openErrNotification("获取数据失败", error);
        }
    }

    useEffect(() => {
        fetchData({ current: 1, pageSize: searchState.limit, previous: 0 });
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const refreshTables = () => {
        setSearchState(s => ({ ...s, fetchError: false, isLoading: true }));
        fetchData({ current: 1, pageSize: searchState.limit, previous: 0 }, searchState.query);
        onTotalsRefresh?.();
    }

    const table = (
        <>
            <ResourceToolbar
                className={style.toolBar}
                title="用户组列表"
                count={searchState.isLoading ? '正在同步列表' : `共 ${searchState.total} 条`}
                filters={(
                    <>
                        <Search
                            value={searchState.query}
                            onChange={(value: string) => {
                                fetchData({ current: 1, pageSize: searchState.limit, previous: 0, }, value);
                            }}
                        />
                        <Tooltip content="刷新">
                            <Button aria-label="刷新用户组列表" shape="square" variant="outline" onClick={() => fetchData({ current: 1, pageSize: searchState.limit, previous: 0 }, searchState.query)}>
                                <RefreshIcon />
                            </Button>
                        </Tooltip>
                        {selectedRowKeys.length > 0 && (
                            <>
                                <span className={style.selectionHint}>已选 {selectedRowKeys.length} 项</span>
                                <Popconfirm
                                    content={`确认删除选中的 ${selectedRowKeys.length} 个用户组吗？`}
                                    destroyOnClose
                                    placement="top"
                                    showArrow
                                    onConfirm={() => handleBatchDeleteGroups(selectedRowKeys.map(String))}
                                >
                                    <Button theme="danger">批量删除</Button>
                                </Popconfirm>
                            </>
                        )}
                        <Button theme="primary" icon={<AddIcon />} onClick={handleCreateUserGroup}>新建用户组</Button>
                    </>
                )}
            />
            <ShowToken
                key={editorState.mode + editorState.data?.name + '_showtoken'}
                row={editorState.data || {} as TableRowData}
                visible={editorState.visible && editorState.resource === 'group_token'}
                close={() => {
                    setEditorState(s => ({ ...s, resource: 'group_token', visible: false }))
                }} />
            <GroupEditor
                key={editorState.mode + (editorState.data?.name || 'new') + (editorState.visible ? '1' : '0')}
                modify={editorState.mode !== 'create'}
                visible={editorState.visible && editorState.resource === 'group'}
                refresh={refreshTables}
                closeDrawer={() => {
                    // 关闭后重置编辑器状态
                    setEditorState(s => ({ ...s, visible: false }));
                }} op={editorState.mode} />
            <Table
                data={searchState.groups}
                columns={columns(handleEditUserGroup, openGroupDetail)}
                loading={searchState.isLoading}
                rowKey="id"
                size={"large"}
                tableLayout={'auto'}
                cellEmptyContent={'-'}
                pagination={{
                    current: searchState.current,
                    pageSize: searchState.limit,
                    total: searchState.total,
                    showJumper: true,
                    onChange(pageInfo) {
                        fetchData(pageInfo, searchState.query);
                    },
                }}
                onPageChange={(pageInfo) => {
                    fetchData(pageInfo, searchState.query);
                }}
                selectOnRowClick={false}
                selectedRowKeys={selectedRowKeys}
                onSelectChange={(selected: Array<string | number>) => {
                    setSelectedRowKeys(selected);
                }}
            />
        </>
    )

    return (
        <>
            {searchState.fetchError ? (
                <ServerError />
            ) : (
                table
            )}
        </>
    )
}

export default React.memo(GroupsTable);
