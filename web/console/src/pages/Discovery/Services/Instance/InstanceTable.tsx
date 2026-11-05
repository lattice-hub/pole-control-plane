import React, { useState } from 'react';
import { Table, Button, PrimaryTableProps, Tooltip, Space, TableRowData, Tag, Link, Popconfirm } from 'components/Fluent';
import { AddIcon, DeleteIcon, EditIcon, RefreshIcon } from 'components/Fluent/icons';
import { BrowserRouterProps } from 'components/Router';

import Search from 'components/Search';
import ErrorPage from 'components/ErrorPage';
import { ResourceToolbar } from 'components/ResourceLayout';
import Text from 'components/Text';
import { useAppDispatch, useAppSelector } from 'modules/store';
import { describeInstances, HEALTH_STATUS_MAP, Instance, ISOLATE_STATUS_MAP } from 'services/instance';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import style from './index.module.less';
import InstanceEditor from './InstanceEditor';
import { cleanInsPage, editorInstance, listInstances, removeInstances, resetInstance, selectInstance } from 'modules/discovery/instance';
import { Op } from 'services/types';

export interface IInstanceListProps {
    namespace: string;
    serviceName: string;
}

const ServerError = () => <ErrorPage code={500} />;

const columns = (operateInstance: (op: Op, row: TableRowData) => void): PrimaryTableProps['columns'] => [
    {
        colKey: 'id',
        title: 'ID',
        type: 'multiple',
        width: 52,
        fixed: 'left',
        checkProps: ({ row }) => ({ disabled: row.editable === false || row.deleteable === false }),
    },
    {
        colKey: 'host',
        title: '主机',
        width: 180,
        cell: ({ row }) => (
            <Link theme='primary' title={String(row.host || '-')} onClick={() => operateInstance('view', row)}>
                <Text>{row.host}</Text>
            </Link>
        ),
    },
    {
        colKey: 'port',
        title: '端口',
        width: 88,
        cell: ({ row: { port } }) => <Text>{port || '-'}</Text>,
    },
    {
        colKey: 'protocol',
        title: '协议',
        width: 104,
        cell: ({ row: { protocol } }) => <Text>{protocol || '-'}</Text>,
    },
    {
        colKey: 'version',
        title: '版本',
        width: 116,
        cell: ({ row: { version } }) => (
            <Text>{version || '-'}</Text>
        ),
    },
    {
        colKey: 'weight',
        title: '权重',
        width: 88,
        cell: ({ row: { weight } }) => (
            <Text>{weight || '-'}</Text>
        ),
    },
    {
        colKey: 'healthy',
        title: '健康状态',
        width: 112,
        cell: ({ row: { healthy } }) => (
            <Tag theme={HEALTH_STATUS_MAP?.[healthy as keyof typeof HEALTH_STATUS_MAP]?.theme as "success" | "danger" | "default" | "primary" | "warning"} variant="outline">{HEALTH_STATUS_MAP?.[healthy as keyof typeof HEALTH_STATUS_MAP]?.text ?? '-'}</Tag>
        ),
    },
    {
        colKey: 'isolate',
        title: '隔离状态',
        width: 112,
        cell: ({ row: { isolate } }) => (
            <Tag theme={ISOLATE_STATUS_MAP?.[isolate as keyof typeof ISOLATE_STATUS_MAP]?.theme as "success" | "danger" | "default" | "primary" | "warning"} variant="outline">{ISOLATE_STATUS_MAP?.[isolate as keyof typeof ISOLATE_STATUS_MAP]?.text ?? '-'}</Tag>
        ),
    },
    {
        colKey: 'location',
        title: '地理位置',
        width: 190,
        cell: ({ row: { location } }) => (
            <Text title={`${location?.region || '-'}/${location?.zone || '-'}/${location?.campus || '-'}`}>
                {location?.region || '-'}/{location?.zone || '-'}/{location?.campus || '-'}
            </Text>
        ),
    },
    {
        colKey: 'time',
        title: '操作时间',
        width: 210,
        cell: ({ row: { ctime, mtime } }: TableRowData) => <Text>修改: {mtime}<br />创建: {ctime}</Text>,
    },
    {
        colKey: 'action',
        title: '操作',
        width: 112,
        fixed: 'right',
        cell: ({ row }) => {
            return (
                <Space>
                    <Tooltip content={row.editable === false ? '无权限操作' : '编辑'}>
                        <Button
                            shape="square"
                            variant="text"
                            aria-label="编辑实例"
                            disabled={row.editable === false}
                            onClick={() => operateInstance('edit', row)}>
                            <EditIcon />
                        </Button>
                    </Tooltip>
                    <Tooltip content={row.deleteable === false ? '无权限操作' : '删除'}>
                        <Popconfirm
                            content="确认删除吗"
                            destroyOnClose
                            placement="top"
                            showArrow
                            theme="default"
                            onConfirm={() => {
                                operateInstance('delete', row);
                            }}
                        >
                            <Button shape="square" variant="text" aria-label="删除实例" disabled={row.deleteable === false}>
                                <DeleteIcon />
                            </Button>
                        </Popconfirm>
                    </Tooltip>
                </Space>
            )
        },
    },
]

export default React.memo((props: IInstanceListProps & BrowserRouterProps) => {
    const dispatch = useAppDispatch();

    const [namespace, serviceName] = [props.namespace, props.serviceName];
    const [selectedRowKeys, setSelectedRowKeys] = useState<Array<string | number>>([]);

    const insState = useAppSelector(selectInstance);
    const { datas, loading, total, page, limit } = insState;

    // 合并编辑相关状态
    const [editState, setEditState] = useState<{
        selectedRow?: TableRowData;
        visible: boolean;
        mode: 'create' | 'edit' | 'view' | 'delete';
    }>({ selectedRow: undefined, visible: false, mode: 'create' });

    React.useEffect(() => {
        refreshTable();
        return () => {
            dispatch(cleanInsPage());
        }
    }, []);

    const refreshTable = (page = 1, limit = 10, query = '') => {
        dispatch(listInstances({
            param: {
                namespace: namespace,
                service: serviceName,
                host: query || undefined,
                offset: (page - 1) * limit,
                limit: limit,
            }
        })).then((res) => {
            if (res.meta.requestStatus === 'rejected') {
                openErrNotification('获取实例列表失败', res.payload as string);
            }
        });
    }

    // 编辑、新建事件
    const operateInstance = (op: Op, row?: TableRowData) => {
        switch (op) {
            case 'create':
                dispatch(resetInstance());
                setEditState(prev => ({ ...prev, visible: true, mode: 'create', selectedRow: undefined }));
                break;
            case 'edit':
                dispatch(editorInstance({ ...row as Instance }));
                setEditState(prev => ({ ...prev, visible: true, mode: op, selectedRow: row }));
                break;
            case 'delete':
                if (!row?.id || !row?.host || !row?.port) return;
                dispatch(removeInstances({
                    instances: [{
                        id: String(row.id),
                        namespace,
                        service: serviceName,
                        host: String(row.host),
                        port: Number(row.port),
                    }],
                })).then((res) => {
                    if (res.meta.requestStatus === 'rejected') {
                        openErrNotification('删除服务实例失败', res.payload as string);
                        return;
                    }
                    openInfoNotification('请求成功', '删除服务实例成功');
                    setSelectedRowKeys(keys => keys.filter(key => String(key) !== String(row.id)));
                    refreshTable(page, limit);
                });
                break;
            case 'view':
                dispatch(editorInstance({ ...row as Instance }));
                setEditState(prev => ({ ...prev, visible: true, mode: 'view', selectedRow: row }));
                break;
            default:
                break;
        }
    }

    const table = (
        <>
            <ResourceToolbar
                density="compact"
                title="实例清单"
                count={loading
                    ? '正在同步列表'
                    : `当前显示 ${datas.length} / ${total} 条${selectedRowKeys.length ? `，已选 ${selectedRowKeys.length} 项` : ''}`}
                filters={(
                    <>
                    <Space>
                        <Search
                            placeholder="主机地址"
                            onChange={(value: string) => {
                                refreshTable(1, limit, value);
                            }} />
                        <Tooltip content="刷新">
                            <Button
                                aria-label="刷新实例列表"
                                shape="square"
                                variant="outline"
                                icon={<RefreshIcon />}
                                onClick={() => refreshTable(1, limit)}
                            />
                        </Tooltip>
                    </Space>
                    <Button theme="primary" icon={<AddIcon />} onClick={() => operateInstance('create')}>新建实例</Button>
                    </>
                )}
            />
            {editState.visible && (
                <InstanceEditor
                    key={editState.mode + (editState.selectedRow?.host || 'new') + (editState.visible ? '1' : '0')}
                    op={editState.mode}
                    namespace={namespace}
                    service={serviceName}
                    visible={editState.visible}
                    canEdit={editState.selectedRow?.editable !== false}
                    onEdit={editState.selectedRow ? () => operateInstance('edit', editState.selectedRow) : undefined}
                    closeDrawer={() => {
                        // 关闭后重置编辑器状态
                        dispatch(resetInstance());
                        setEditState(s => ({ ...s, visible: false }));
                        refreshTable();
                    }} />
            )}
            <section className={style.instanceTableSurface}>
                <Table
                    data={datas}
                    columns={columns(operateInstance)}
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
                            refreshTable(pageInfo.current, pageInfo.pageSize);
                        },
                    }}
                    selectOnRowClick={false}
                    selectedRowKeys={selectedRowKeys}
                    onSelectChange={(selected: Array<string | number>) => {
                        setSelectedRowKeys(selected);
                    }}
                />
            </section>
        </>
    );

    return (
        <div className={style.instanceTablePage}>
            {table}
        </div>
    );
});
