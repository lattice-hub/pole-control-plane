import React, { useState } from 'react';
import { Table, Button, PrimaryTableProps, Space, TableRowData, Tag, Tooltip } from 'components/Fluent';
import { AddIcon, RefreshIcon } from 'components/Fluent/icons';
import { useNavigate } from 'components/Router';

import Search from 'components/Search';
import ErrorPage from 'components/ErrorPage';
import Text from 'components/Text';
import { ConfirmOperationButton, OperationButton } from 'components/OperationButton';
import ResourceNameLink from 'components/ResourceNameLink';
import { ResourceToolbar } from 'components/ResourceLayout';
import { useAppDispatch, useAppSelector } from 'modules/store';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import style from './index.module.less';
import { describeUserToken, User, USER_ROLE_MAP } from 'services/users';
import ShowToken from './ShowToken';
import UserEditor from './UserEditor';
import { editorUser, listUsers, removeUsers, resetUser, selectUser } from 'modules/user/users';
import { Op } from 'services/types';

interface IUsersProps {
    onTotalChange?: (total: number) => void;
    onTotalsRefresh?: () => void;
}

const ServerError = () => <ErrorPage code={500} />;

const columns = (
    operateUser: (op: Op, res: string, row?: TableRowData) => void,
    openUserDetail: (row: TableRowData) => void,
): PrimaryTableProps['columns'] => [
    {
        colKey: 'id',
        title: 'ID',
        type: 'multiple',
        checkProps: ({ row }) => ({ disabled: row.editable === false || row.deleteable === false || row.user_type === 'main' }),
    },
    {
        colKey: 'name',
        title: '用户名',
        cell: ({ row }) => (
            <ResourceNameLink name={row.name} onClick={() => openUserDetail(row)} />
        ),
    },
    {
        colKey: 'user_type',
        title: '用户类型',
        cell: ({ row: { user_type } }: TableRowData) => (
            <Tag theme={USER_ROLE_MAP?.[user_type as keyof typeof USER_ROLE_MAP]?.theme as "success" | "danger" | "default" | "primary" | "warning"} variant="outline">{USER_ROLE_MAP?.[user_type as keyof typeof USER_ROLE_MAP]?.text ?? '-'}</Tag>
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
        title: 'Token状态',
        cell: ({ row: { token_enable } }: TableRowData) => (<Tag theme={token_enable ? 'success' : 'danger'} variant="outline">{token_enable ? '启用中' : '禁用中'}</Tag>),
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
                    <OperationButton action="view" onClick={() => openUserDetail(row)} />
                    <OperationButton
                        action="edit"
                        disabled={row.editable === false}
                        disabledLabel="无权限操作"
                        onClick={() => operateUser('edit', 'user', row)}
                    />
                    <OperationButton
                        action="token"
                        disabled={row.editable === false}
                        disabledLabel="无权限操作"
                        onClick={() => {
                            describeUserToken({ id: row.id }).then((res) => {
                                if (res) {
                                    operateUser('view', 'user_token', { ...row, auth_token: res.user.auth_token });
                                }
                            });
                        }}
                    />
                    {row.user_type !== 'main' && (
                        <ConfirmOperationButton action="delete" disabled={row.deleteable === false} disabledLabel="无权限操作" confirmContent="确认删除吗" onConfirm={() => operateUser('delete', 'user', row)} />
                    )}
                </Space>
            )
        },
    },
]

const UsersTable: React.FC<IUsersProps> = ({ onTotalChange, onTotalsRefresh }) => {
    const dispatch = useAppDispatch();
    const navigate = useNavigate();

    const userState = useAppSelector(selectUser);
    const { datas, total, page, limit, loading } = userState;
    const [query, setQuery] = useState('');

    React.useEffect(() => {
        if (!loading && !query) onTotalChange?.(total);
    }, [loading, onTotalChange, query, total]);

    const openUserDetail = (row: TableRowData) => {
        navigate(`/auth/principals/userdetail?name=${encodeURIComponent(String(row.name || ''))}&id=${encodeURIComponent(String(row.id || ''))}`);
    };

    // 合并编辑相关状态
    const [editorState, setEditorState] = useState<{
        visible: boolean;
        resource: string;
        mode: Op;
        data?: TableRowData;
    }>({ visible: false, resource: '', mode: 'create', data: undefined });

    // 编辑、新建事件
    const operateUser = (mode: Op, res: string, row?: TableRowData) => {
        switch (mode) {
            case 'delete':
                dispatch(removeUsers({ ids: [row?.id as string] }))
                    .then((res) => {
                        if (res.meta.requestStatus === 'fulfilled') {
                            openInfoNotification('请求成功', `用户 ${row?.name} 删除成功`);
                            onTotalsRefresh?.();
                        } else {
                            openErrNotification('请求失败', `删除用户 ${row?.name} 失败, ${res.payload as string}`);
                        }
                    });
                return;
            case 'edit':
                dispatch(editorUser({ ...row as User }));
                setEditorState(prev => ({ ...prev, visible: true, mode: mode, resource: res, data: { ...row } }))
                return;
            case 'create':
                setEditorState(prev => ({ ...prev, visible: true, mode: mode, resource: res, data: undefined }));
                return;
            case 'view':
                if (res === 'user') {
                    dispatch(editorUser({ ...row as User }));
                }
                setEditorState(prev => ({ ...prev, visible: true, mode: mode, resource: res, data: { ...row } }))
        }
    }

    const refreshData = (page = 1, limit = 10, searchParam = query) => {
        setQuery(searchParam);
        dispatch(listUsers({
            param: {
                limit: limit,
                offset: (page - 1) * limit,
                ...(searchParam && { name: searchParam }),
            }
        })).then((res) => {
            if (res.meta.requestStatus === 'rejected') {
                openErrNotification("请求失败", `获取用户列表失败: ${res.payload as string}`);
            }
        })
    }

    React.useEffect(() => {
        refreshData();
    }, []);

    const table = (
        <>
            <ResourceToolbar
                className={style.toolBar}
                title="用户列表"
                count={loading ? '正在同步列表' : `共 ${total} 条`}
                filters={(
                    <>
                        <Search
                            value={query}
                            onChange={(value: string) => { refreshData(1, limit, value); }}
                        />
                        <Tooltip content="刷新">
                            <Button aria-label="刷新用户列表" shape="square" variant="outline" onClick={() => refreshData(1, limit, query)}>
                                <RefreshIcon />
                            </Button>
                        </Tooltip>
                        <Button theme="primary" icon={<AddIcon />} onClick={() => {
                            operateUser('create', 'user');
                        }}>新建用户</Button>
                    </>
                )}
            />
            <ShowToken
                key={editorState.mode + editorState.data?.name + '_showtoken'}
                row={editorState.data || {} as TableRowData}
                visible={editorState.visible && editorState.resource === 'user_token'}
                close={() => {
                    setEditorState(s => ({ ...s, resource: 'user_token', visible: false }))
                }} />
            {editorState.visible && editorState.resource === 'user' && (
                <UserEditor
                    op={editorState.mode}
                    visible={editorState.visible && editorState.resource === 'user'}
                    closeDrawer={() => {
                        // 关闭后重置编辑器状态
                        dispatch(resetUser());
                        setEditorState(s => ({ ...s, visible: false }));
                        refreshData(1, limit, query);
                        onTotalsRefresh?.();
                    }}
                />
            )}
            <Table
                data={datas}
                columns={columns(operateUser, openUserDetail)}
                loading={loading}
                rowKey="id"
                size={"large"}
                tableLayout={'auto'}
                cellEmptyContent={'-'}
                pagination={{
                    current: page,
                    pageSize: limit,
                    total: total,
                    showJumper: true,
                    onChange(pageInfo) {
                        refreshData(pageInfo.current, pageInfo.pageSize, query);
                    },
                }}
                onPageChange={(pageInfo) => {
                    refreshData(pageInfo.current, pageInfo.pageSize, query);
                }}
            />
        </>
    )

    return (
        <>
            {table}
        </>
    )
}

export default React.memo(UsersTable);
