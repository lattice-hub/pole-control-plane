import React from 'react';
import { Button, Form, Input, Space, Tabs, Tag, Transfer } from 'components/Fluent';
import type { FormProps } from 'components/Fluent';
import EditorDrawer, { EditorDrawerActions } from 'components/EditorDrawer';

import LabelInput from 'components/LabelInput';
import { useAppDispatch } from 'modules/store';
import { saveRoles, updateRoles } from 'modules/auth/role';
import { describeAllUsers, User } from 'services/users';
import { describeAllUserGroups, UserGroup } from 'services/user_group';
import { describeRoles, Role } from 'services/role';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import style from './index.module.less';
import PrincipalPolicyTable from './PrincipalPolicyTable';

const { FormItem } = Form;
const { TabPanel } = Tabs;

interface IRoleEditorProps {
    op: 'create' | 'view' | 'edit';
    closeDrawer: () => void;
    refresh: () => void;
    visible: boolean;
    role: Role | null;
}

const RoleEditor: React.FC<IRoleEditorProps> = ({ visible, op, closeDrawer, refresh, role }) => {
    const [form] = Form.useForm();
    const dispatch = useAppDispatch();
    const [editable, setEditable] = React.useState(op !== 'view');
    const [submitting, setSubmitting] = React.useState(false);
    const [detailRole, setDetailRole] = React.useState<Role | null>(role);
    const [options, setOptions] = React.useState<{
        users: { value: string; label: string; disabled?: boolean }[];
        groups: { value: string; label: string }[];
    }>({ users: [], groups: [] });

    const currentRole = detailRole || role;
    const builtIn = currentRole?.default_role === true;

    React.useEffect(() => {
        if (!visible) return;
        setEditable(op !== 'view');

        const load = async () => {
            try {
                const [users, groups, detail] = await Promise.all([
                    describeAllUsers(),
                    describeAllUserGroups(),
                    role?.id ? describeRoles({ id: role.id, limit: 1, offset: 0, berif: false }) : Promise.resolve(null),
                ]);
                const resolvedRole = detail?.content?.[0] || role;
                setDetailRole(resolvedRole || null);
                setOptions({
                    users: users.map((user: User) => ({ value: user.id, label: user.name, disabled: user.user_type !== 'sub' })),
                    groups: groups.map((group: UserGroup) => ({ value: group.id, label: group.name })),
                });
                form.reset();
                form.setFieldsValue({
                    name: resolvedRole?.name || '',
                    comment: resolvedRole?.comment || '',
                    users: resolvedRole?.users?.map(user => user.id) || [],
                    user_groups: resolvedRole?.user_groups?.map(group => group.id) || [],
                    role_labels: resolvedRole?.metadata ? Object.entries(resolvedRole.metadata).map(([key, value]) => ({ key, value })) : [],
                });
            } catch (error) {
                openErrNotification('加载角色编辑数据失败', (error as Error).message);
            }
        };

        load();
    }, [form, op, role, visible]);

    const onSubmit: FormProps['onSubmit'] = async (event) => {
        if (event.validateResult !== true) return;

        const users = (form.getFieldValue('users') as string[] | undefined) || [];
        const groups = (form.getFieldValue('user_groups') as string[] | undefined) || [];
        const bindings = {
            users: users.map(id => ({ id })),
            user_groups: groups.map(id => ({ id })),
        };

        setSubmitting(true);
        try {
            let result;
            if (builtIn) {
                result = await dispatch(updateRoles({ state: { id: currentRole!.id, ...bindings } }));
            } else {
                const labels = (form.getFieldValue('role_labels') as { key: string; value: string }[] | undefined) || [];
                const state = {
                    id: currentRole?.id || '',
                    name: form.getFieldValue('name') as string,
                    comment: (form.getFieldValue('comment') as string) || '',
                    source: currentRole?.source || 'pole.io',
                    metadata: labels.reduce<Record<string, string>>((metadata, item) => {
                        metadata[item.key] = item.value;
                        return metadata;
                    }, {}),
                    ...bindings,
                };
                result = op === 'create'
                    ? await dispatch(saveRoles({ state }))
                    : await dispatch(updateRoles({ state }));
            }
            if (result.meta.requestStatus !== 'fulfilled') {
                openErrNotification('请求错误', String(result.payload || '保存角色失败'));
                return;
            }
            openInfoNotification('请求成功', builtIn ? '角色成员绑定已更新' : op === 'create' ? '创建角色成功' : '修改角色成功');
            closeDrawer();
            refresh();
        } finally {
            setSubmitting(false);
        }
    };

    const viewRole = currentRole;
    const labels = viewRole?.metadata ? Object.entries(viewRole.metadata) : [];
    const users = viewRole?.users || [];
    const groups = viewRole?.user_groups || [];

    const roleBaseView = (
        <div className={style.authDetail}>
            <div className={style.detailField}>
                <span>角色名</span>
                <strong>{viewRole?.name || '-'}</strong>
            </div>
            <div className={style.detailField}>
                <span>角色 ID</span>
                <p>{viewRole?.id || '-'}</p>
            </div>
            <div className={style.detailGrid}>
                <div className={style.detailField}>
                    <span>角色类型</span>
                    <Tag theme={viewRole?.default_role ? 'primary' : 'success'} variant="outline">
                        {viewRole?.default_role ? '内置角色' : '自定义角色'}
                    </Tag>
                </div>
                <div className={style.detailField}>
                    <span>来源</span>
                    <p>{viewRole?.source || '-'}</p>
                </div>
                <div className={style.detailField}>
                    <span>用户数量</span>
                    <p>{users.length}</p>
                </div>
                <div className={style.detailField}>
                    <span>用户组数量</span>
                    <p>{groups.length}</p>
                </div>
                <div className={style.detailField}>
                    <span>修改时间</span>
                    <p>{viewRole?.mtime || '-'}</p>
                </div>
                <div className={style.detailField}>
                    <span>创建时间</span>
                    <p>{viewRole?.ctime || '-'}</p>
                </div>
            </div>
            <div className={style.detailField}>
                <span>备注</span>
                <p>{viewRole?.comment || '-'}</p>
            </div>
            <div className={style.detailField}>
                <span>成员用户</span>
                <div className={style.detailLabels}>
                    {users.length > 0 ? users.map(user => (
                        <em key={user.id}>{user.name || user.id}</em>
                    )) : '-'}
                </div>
            </div>
            <div className={style.detailField}>
                <span>成员用户组</span>
                <div className={style.detailLabels}>
                    {groups.length > 0 ? groups.map(group => (
                        <em key={group.id}>{group.name || group.id}</em>
                    )) : '-'}
                </div>
            </div>
            <div className={style.detailField}>
                <span>角色标签</span>
                <div className={style.detailLabels}>
                    {labels.length > 0 ? labels.map(([key, value]) => (
                        <em key={key}>{key}: {value}</em>
                    )) : '-'}
                </div>
            </div>
        </div>
    );

    const roleView = (
        <Tabs defaultValue="base">
            <TabPanel value="base" label="基础信息">
                {roleBaseView}
            </TabPanel>
            <TabPanel value="permission" label="权限信息">
                <PrincipalPolicyTable principalId={viewRole?.id} principalType={3} />
            </TabPanel>
        </Tabs>
    );

    const roleForm = (
        <Form form={form} layout="vertical" labelWidth={120} labelAlign="left" onSubmit={onSubmit}>
            {builtIn ? (
                <div>
                    <strong>{currentRole?.name}</strong>
                    <p>{currentRole?.comment}</p>
                    <p>内置角色的名称、描述、标签及资源/API 权限固定，此处只维护成员关系。</p>
                </div>
            ) : (
                <>
                    <FormItem
                        label="角色名"
                        name="name"
                        rules={[
                            { required: true, message: '角色名不能为空' },
                            { max: 64, message: '长度不超过64个字符' },
                        ]}
                    >
                        <Input readonly={op !== 'create'} />
                    </FormItem>
                    <FormItem label="备注" name="comment" rules={[{ max: 255, message: '长度不超过255个字符' }]}>
                        <Input />
                    </FormItem>
                </>
            )}
            <FormItem label="用户" name="users">
                <Transfer search data={options.users} />
            </FormItem>
            <FormItem label="用户组" name="user_groups">
                <Transfer search data={options.groups} />
            </FormItem>
            {!builtIn && <LabelInput form={form} label="角色标签" name="role_labels" editable />}
        </Form>
    );

    return (
        <EditorDrawer
            bodyClassName={style.editorBody}
            width="standard"
            title={op === 'create' ? '创建角色' : editable ? (builtIn ? '管理内置角色成员' : '编辑角色') : '角色详情'}
            description={op === 'create' ? '定义角色属性，并一次完成用户与用户组绑定。' : undefined}
            visible={visible}
            onClose={closeDrawer}
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
                    submitText={op === 'create' ? '创建角色' : '保存修改'}
                    submitting={submitting}
                />
            ) : false}
        >
            {editable ? roleForm : roleView}
        </EditorDrawer>
    );
};

export default React.memo(RoleEditor);
