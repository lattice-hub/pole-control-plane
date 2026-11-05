import React, {  } from 'react';
import { Form, Input, Space, Button, Tag, Tabs } from 'components/Fluent';
import type { FormProps } from 'components/Fluent';
import EditorDrawer, { EditorDrawerActions } from 'components/EditorDrawer';

import { useAppDispatch, useAppSelector } from 'modules/store';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import LabelInput from 'components/LabelInput';
import { saveUsers, updateUsers } from 'modules/user/users';
import { selectUser } from 'modules/user/users';
import { Label, Op } from 'services/types';
import { USER_ROLE_MAP } from 'services/users';
import style from './index.module.less';
import PrincipalPolicyTable from './PrincipalPolicyTable';

interface UserDO {
    // 用户ID
    id: string
    // 用户名称
    name: string
    // 用户密码
    password: string
    // 用户鉴权Token
    auth_token?: string
    // 该token是否被禁用
    token_enable: boolean
    // 该账户的简单描述
    comment: string
    // 账户对应邮箱
    email: string
    // 账户对应手机号
    mobile: string
    // 用户标签
    metadata: Label[]
}

const { FormItem } = Form;
const { TabPanel } = Tabs;

interface IUserEditorProps {
    op: Op;
    closeDrawer: () => void;
    visible: boolean;
}

const UserEditor: React.FC<IUserEditorProps> = ({ visible, op, closeDrawer }) => {
    const [form] = Form.useForm();
    const [editable, setEditable] = React.useState(op !== 'view');
    const [submitting, setSubmitting] = React.useState(false);
    const dispatch = useAppDispatch();

    const userState = useAppSelector(selectUser);
    const { editUser } = userState;

    React.useEffect(() => {
        if (visible) {
            setEditable(op !== 'view');
        }
    }, [visible, op]);

    React.useEffect(() => {
        if (visible && op === 'create') {
            form.reset();
            form.setFieldsValue({ name: '', password: '', comment: '', token_enable: true, user_labels: [] });
        } else if (visible && editUser) {
            form.setFieldsValue({
                ...editUser,
                user_labels: editUser?.metadata ? Object.entries(editUser.metadata).map(([key, value]) => ({ key, value })) : [],
            });
        }
    }, [editUser, op, visible]);

    const onSubmit: FormProps['onSubmit'] = async (e) => {
        if (e.validateResult !== true) {
            return;
        }

        const labels = (form.getFieldValue('user_labels') as { key: string, value: string }[] | undefined) || []

        const newData = {
            id: op === 'create' ? '' : editUser?.id || '',
            name: form.getFieldValue('name') as string,
            password: form.getFieldValue('password') as string || '',
            token_enable: form.getFieldValue('token_enable') as boolean,
            comment: form.getFieldValue('comment') as string,
            email: form.getFieldValue('email') as string,
            mobile: form.getFieldValue('mobile') as string,
            metadata: labels.reduce((acc: { [key: string]: string }, label: { key: string, value: string }) => {
                acc[label.key] = label.value;
                return acc;
            }, {}),
        }

        setSubmitting(true);
        try {
            const result = op === 'create'
                ? await dispatch(saveUsers({ param: { ...newData } }))
                : await dispatch(updateUsers({ param: { ...newData } }));
            if (result.meta.requestStatus !== 'fulfilled') {
                openErrNotification('请求错误', result?.payload as string);
            } else {
                openInfoNotification('请求成功', op === 'create' ? '创建用户成功' : '修改用户信息成功');
                closeDrawer();
            }
        } finally {
            setSubmitting(false);
        }
    };

    const labels = editUser?.metadata ? Object.entries(editUser.metadata) : [];
    const canEdit = (editUser as (UserDO & { editable?: boolean }) | null)?.editable !== false;
    const resetEditor = () => {
        form.setFieldsValue({
            name: op === 'create' ? '' : editUser?.name || '',
            password: '',
            token_enable: op === 'create' ? true : !!editUser?.token_enable,
            comment: op === 'create' ? '' : editUser?.comment || '',
            user_labels: op !== 'create' && editUser?.metadata
                ? Object.entries(editUser.metadata).map(([key, value]) => ({ key, value }))
                : [],
        });
    };

    const userBaseView = (
        <div className={style.authDetail}>
            <div className={style.detailField}>
                <span>用户名</span>
                <strong>{editUser?.name || '-'}</strong>
            </div>
            <div className={style.detailField}>
                <span>用户 ID</span>
                <p>{editUser?.id || '-'}</p>
            </div>
            <div className={style.detailGrid}>
                <div className={style.detailField}>
                    <span>用户类型</span>
                    <Tag theme="primary" variant="outline">
                        {USER_ROLE_MAP?.[editUser?.user_type as keyof typeof USER_ROLE_MAP]?.text ?? '-'}
                    </Tag>
                </div>
                <div className={style.detailField}>
                    <span>Token 状态</span>
                    <Tag theme={editUser?.token_enable ? 'success' : 'danger'} variant="outline">
                        {editUser?.token_enable ? '启用中' : '禁用中'}
                    </Tag>
                </div>
                <div className={style.detailField}>
                    <span>邮箱</span>
                    <p>{editUser?.email || '-'}</p>
                </div>
                <div className={style.detailField}>
                    <span>手机号</span>
                    <p>{editUser?.mobile || '-'}</p>
                </div>
                <div className={style.detailField}>
                    <span>来源</span>
                    <p>{editUser?.source || '-'}</p>
                </div>
                <div className={style.detailField}>
                    <span>时间</span>
                    <p>修改: {editUser?.mtime || '-'}<br />创建: {editUser?.ctime || '-'}</p>
                </div>
            </div>
            <div className={style.detailField}>
                <span>备注</span>
                <p>{editUser?.comment || '-'}</p>
            </div>
            <div className={style.detailField}>
                <span>用户标签</span>
                <div className={style.detailLabels}>
                    {labels.length > 0 ? labels.map(([key, value]) => (
                        <em key={key}>{key}: {value}</em>
                    )) : '-'}
                </div>
            </div>
        </div>
    );

    const userView = (
        <Tabs defaultValue="base">
            <TabPanel value="base" label="基础信息">
                {userBaseView}
            </TabPanel>
            <TabPanel value="permission" label="权限信息">
                <PrincipalPolicyTable principalId={editUser?.id} principalType={1} />
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
            <FormItem label={'用户名'} name={'name'}
                rules={[
                    { required: true, message: '用户名不能为空' },
                    { max: 64, message: '长度不超过64个字符' },
                ]}
            >
                <Input readonly={op !== 'create'} />
            </FormItem>
            {op === 'create' && (
                <FormItem label={'密码'} name={'password'}
                    rules={[
                        { required: true, message: '密码不能为空' },
                        { min: 6, message: '长度不小于6个字符' },
                        { max: 255, message: '长度不超过255个字符' }
                    ]}>
                    <Input type='password' />
                </FormItem>
            )}
            <FormItem label={'备注'} name={'comment'}
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
                    <p>{editUser?.token_enable ? '启用' : '禁用'}</p>
                    <p>Token 状态请在详情中管理。</p>
                </div>
            )}
            <LabelInput form={form} label='用户标签' name='user_labels' editable={editable} />
        </Form>
    )

    return (
        <div>
            <EditorDrawer
                bodyClassName={style.editorBody}
                width="standard"
                title={op === 'create' ? "创建用户" : editable ? "编辑用户" : "用户详情"}
                description={op === 'create' ? '创建控制台登录主体，并配置识别标签。' : undefined}
                footer={op === 'view' && !editable ? (
                    <Space>
                        <Button theme="primary" disabled={!canEdit} onClick={() => setEditable(true)}>
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
                        submitText={op === 'create' ? '创建用户' : '保存修改'}
                        submitting={submitting}
                    />
                ) : false}
                visible={visible}
                onClose={closeDrawer}
            >
                {editable ? userForm : userView}
            </EditorDrawer>
        </div>
    );
}

export default React.memo(UserEditor);
