import React, { useState, useEffect } from 'react';
import { Form, Input, Space, Button, InputNumber, Radio, Select, Transfer, Tag, Tabs } from 'components/Fluent';
import type { FormProps } from 'components/Fluent';
import EditorDrawer, { EditorDrawerActions } from 'components/EditorDrawer';

import { useAppDispatch, useAppSelector } from 'modules/store';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import LabelInput from 'components/LabelInput';
import { saveUsers, updateUsers } from 'modules/user/users';
import { selectUser } from 'modules/user/users';
import { saveUserGroups, selectUserGroup, updateUserGroups } from 'modules/user/groups';
import { describeAllUsers, User } from 'services/users';
import { describeUserGroupDetail, UserGroup } from 'services/user_group';
import style from './index.module.less';
import PrincipalPolicyTable from './PrincipalPolicyTable';


const { FormItem } = Form;
const { TabPanel } = Tabs;

interface IGroupEditorProps {
    modify: boolean;
    op: string;
    closeDrawer: () => void;
    refresh: () => void;
    visible: boolean;
}

const GroupEditor: React.FC<IGroupEditorProps> = ({ visible, op, modify, closeDrawer, refresh }) => {
    const [form] = Form.useForm();
    const [editable, setEditable] = React.useState(op !== 'view');
    const dispatch = useAppDispatch();
    const currentUserGroup = useAppSelector(selectUserGroup);
    const {
        id,
        name,
        comment,
        metadata,
    } = currentUserGroup;

    const [searchState, setSearchState] = React.useState<{
        users: { value: string; label: string }[];
        userLoading: boolean,
        userError: string,
    }>({
        users: [],
        userLoading: false,
        userError: '',
    });
    const [detailGroup, setDetailGroup] = React.useState<UserGroup | null>(null);
    const [submitting, setSubmitting] = React.useState(false);

    React.useEffect(() => {
        if (visible) {
            setEditable(op !== 'view');
            if (op !== 'create' && id) {
                fetchUserGroupDetail();
            } else {
                setDetailGroup(null);
                form.reset();
                form.setFieldsValue({ name: '', comment: '', token_enable: true, users: [], group_labels: [] });
            }
            fetchUserData();
        }
    }, [id, op, visible]);

    const fetchUserGroupDetail = async () => {
        if (!id) {
            return;
        }
        try {
            // 默认只查询简要信息
            const response = await describeUserGroupDetail({
                id: id as string,
            });
            if (!response.userGroup) {
                openErrNotification("获取用户组详情失败", "用户组不存在");
                return;
            }
            const users = response.userGroup.relation.users ? response.userGroup.relation.users.map((user) => user.id) : []
            const group_labels = response.userGroup.metadata ? Object.entries(response.userGroup.metadata).map(([key, value]) => ({ key, value })) : [];

            setDetailGroup(response.userGroup);
            form.setFieldsValue({
                name: response.userGroup.name,
                comment: response.userGroup.comment,
                token_enable: response.userGroup.token_enable,
                users: users,
                group_labels: group_labels,
            })
        } catch (error: Error | any) {
            openErrNotification("获取用户组详情失败", error);
        }
    }

    const fetchUserData = async () => {
        setSearchState(s => ({ ...s, userLoading: true, userError: '' }));
        try {
            // 请求可能存在跨域问题
            const response = await describeAllUsers();
            const users = response.map((user: User) => {
                return {
                    value: user.id,
                    label: user.name,
                    disabled: user.user_type !== 'sub',
                }
            })
            setSearchState(s => ({ ...s, users: users, userLoading: false }));
        } catch (error) {
            setSearchState(s => ({ ...s, userLoading: false, userError: (error as Error).message || '加载成员失败' }));
            openErrNotification('获取用户列表失败', (error as Error).message);
        }
    }

    const onSubmit: FormProps['onSubmit'] = async (e) => {
        if (e.validateResult !== true || submitting) {
            return;
        }
        setSubmitting(true);

        // Fluent Form 不会为未交互的 Transfer/LabelInput 自动写入空数组；
        // 创建空成员、空标签的用户组时必须按空集合处理，避免提交阶段调用
        // undefined.map/reduce 导致整个抽屉卡在 loading 状态。
        const inusers = (form.getFieldValue('users') as string[] | undefined) || []
        const labels = (form.getFieldValue('group_labels') as { key: string, value: string }[] | undefined) || []

        const newData = {
            id: op === 'create' ? undefined : id,
            name: form.getFieldValue('name') as string,
            token_enable: form.getFieldValue('token_enable') as boolean,
            comment: form.getFieldValue('comment') as string,
            metadata: labels.reduce((acc: { [key: string]: string }, label: { key: string, value: string }) => {
                acc[label.key] = label.value;
                return acc;
            }, {}),
            relation: {
                group_id: op === 'create' ? undefined : id,
                users: inusers.map((v) => {
                    return {
                        id: v,
                    }
                }),
            },
        }

        try {
            const result = op === 'create'
                ? await dispatch(saveUserGroups({ state: { ...newData } }))
                : await dispatch(updateUserGroups({ state: { ...newData } }));
            if (result.meta.requestStatus !== 'fulfilled') {
                openErrNotification('请求错误', result?.payload as string);
            } else {
                openInfoNotification('请求成功', op === 'create' ? '创建用户组成功' : '修改用户组信息成功');
                closeDrawer();
                refresh();
            }
        } finally {
            setSubmitting(false);
        }
    };

    const viewGroup = detailGroup || currentUserGroup as UserGroup;
    const labels = viewGroup?.metadata ? Object.entries(viewGroup.metadata) : [];
    const users = viewGroup?.relation?.users || [];
    const resetEditor = () => {
        const baseline = op === 'create' ? null : viewGroup;
        form.setFieldsValue({
            name: baseline?.name || '',
            comment: baseline?.comment || '',
            token_enable: baseline ? !!baseline.token_enable : true,
            users: baseline?.relation?.users?.map((user) => user.id) || [],
            group_labels: baseline?.metadata
                ? Object.entries(baseline.metadata).map(([key, value]) => ({ key, value }))
                : [],
        });
    };

    const groupBaseView = (
        <div className={style.authDetail}>
            <div className={style.detailField}>
                <span>用户组名</span>
                <strong>{viewGroup?.name || '-'}</strong>
            </div>
            <div className={style.detailField}>
                <span>用户组 ID</span>
                <p>{viewGroup?.id || '-'}</p>
            </div>
            <div className={style.detailGrid}>
                <div className={style.detailField}>
                    <span>Token 状态</span>
                    <Tag theme={viewGroup?.token_enable ? 'success' : 'danger'} variant="outline">
                        {viewGroup?.token_enable ? '启用中' : '禁用中'}
                    </Tag>
                </div>
                <div className={style.detailField}>
                    <span>用户数量</span>
                    <p>{viewGroup?.user_count ?? users.length ?? 0}</p>
                </div>
                <div className={style.detailField}>
                    <span>来源</span>
                    <p>{viewGroup?.source || '-'}</p>
                </div>
                <div className={style.detailField}>
                    <span>时间</span>
                    <p>修改: {viewGroup?.mtime || '-'}<br />创建: {viewGroup?.ctime || '-'}</p>
                </div>
            </div>
            <div className={style.detailField}>
                <span>备注</span>
                <p>{viewGroup?.comment || '-'}</p>
            </div>
            <div className={style.detailField}>
                <span>成员用户</span>
                <div className={style.detailLabels}>
                    {users.length > 0 ? users.map((user) => (
                        <em key={user.id}>{user.name || user.id}</em>
                    )) : '-'}
                </div>
            </div>
            <div className={style.detailField}>
                <span>用户组标签</span>
                <div className={style.detailLabels}>
                    {labels.length > 0 ? labels.map(([key, value]) => (
                        <em key={key}>{key}: {value}</em>
                    )) : '-'}
                </div>
            </div>
        </div>
    );

    const groupView = (
        <Tabs defaultValue="base">
            <TabPanel value="base" label="基础信息">
                {groupBaseView}
            </TabPanel>
            <TabPanel value="permission" label="权限信息">
                <PrincipalPolicyTable principalId={viewGroup?.id} principalType={2} />
            </TabPanel>
        </Tabs>
    );

    const userForm = (
        <Form
            form={form}
            layout="vertical"
            labelWidth={120}
            labelAlign={'left'}
            onSubmit={onSubmit}
            onReset={resetEditor}
        >
            <FormItem label={'用户组名'} name={'name'} initialData={op === 'create' ? '' : name}
                rules={[
                    { required: true, message: '用户组名不能为空' },
                    { max: 64, message: '长度不超过64个字符' },
                ]}
            >
                <Input readonly={op !== 'create'} />
            </FormItem>
            <FormItem label={'备注'} name={'comment'} initialData={op === 'create' ? '' : comment}
                rules={[
                    { max: 255, message: '长度不超过255个字符' }
                ]}>
                <Input disabled={!editable} />
            </FormItem>
            {op === 'create' ? (
                <p>创建后默认启用 Token，可在详情中禁用。</p>
            ) : (
                <div className={style.detailField}>
                    <span>Token 状态</span>
                    <p>{viewGroup?.token_enable ? '启用' : '禁用'}</p>
                    <p>Token 状态请在详情中管理。</p>
                </div>
            )}
            <FormItem
                label={'用户'}
                name={'users'}
                help={searchState.userError ? (
                    <span role="alert">
                        成员列表加载失败：{searchState.userError}
                        <Button size="small" variant="text" onClick={fetchUserData}>重新加载</Button>
                    </span>
                ) : undefined}
            >
                <Transfer
                    search={true}
                    data={searchState.users}
                    loading={searchState.userLoading}
                    disabled={!editable || Boolean(searchState.userError)}
                />
            </FormItem>
            <LabelInput form={form} label='用户组标签' name='group_labels' editable={editable} />
        </Form>
    )

    return (
        <div>
            <EditorDrawer
                bodyClassName={style.editorBody}
                width="standard"
                title={op === 'create' ? "创建用户组" : editable ? "编辑用户组" : "用户组详情"}
                description={op === 'create' ? '集中维护成员和权限识别标签。' : undefined}
                footer={op === 'view' && !editable ? (
                    <Space>
                        <Button theme="primary" onClick={() => setEditable(true)}>
                            编辑
                        </Button>
                        <Button theme="default" onClick={closeDrawer}>
                            关闭
                        </Button>
                    </Space>
                ) : editable ? (
                    <EditorDrawerActions
                        onCancel={closeDrawer}
                        onSubmit={() => form.submit({ showErrorMessage: true })}
                        submitText={op === 'create' ? '创建用户组' : '保存修改'}
                        submitting={submitting}
                        submitDisabled={searchState.userLoading || Boolean(searchState.userError)}
                    />
                ) : false}
                visible={visible}
                onClose={closeDrawer}
            >
                {editable ? userForm : groupView}
            </EditorDrawer>
        </div>
    );
}

export default React.memo(GroupEditor);
