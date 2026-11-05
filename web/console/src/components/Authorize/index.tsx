import React from "react";
import { authorizeResources, describeResourcePrincipals, PolicySourceType } from "services/auth_policy";
import { describeRoles } from "services/role";
import { describeUserGroups } from "services/user_group";
import { describeUsers } from "services/users";
import { Form, FormProps, Input, Loading, Select, Space, Tag } from 'components/Fluent';
import EditorDrawer, { EditorDrawerActions } from 'components/EditorDrawer';
import style from './index.module.less';

import { openErrNotification, openInfoNotification } from "utils/notifition";

const { FormItem } = Form;

export interface IAuthorizeInputProps {
    resource_type: PolicySourceType;
    resource_name: string;
    resource_id: string;
    visible: boolean;
    onClose: () => void;
}

const AuthorizeInput: React.FC<IAuthorizeInputProps> = (props) => {
    const [form] = Form.useForm();
    const isNamespaceResource = props.resource_type === PolicySourceType.Namespaces;
    const isMCPServerResource = props.resource_type === PolicySourceType.MCPServerResources;
    const isA2AAgentResource = props.resource_type === PolicySourceType.A2AAgentResources;
    const resourceNameParts = props.resource_name.split('/');
    const [resourceNamespace, resourceName] = resourceNameParts;
    const resourceNameLabel = isMCPServerResource
            ? 'MCP Server'
            : isA2AAgentResource
                ? 'A2A Agent'
                : '资源名称';

    // 下拉/Transfer数据
    const [state, setState] = React.useState<{
        loading: boolean;
        userOpts: { value: string; label: string }[];
        groupOpts: { value: string; label: string }[];
        roleOpts: { value: string; label: string }[];
        checkedUsers: string[];
        checkedGroups: string[];
        checkedRoles: string[];
    }>({
        loading: false,
        userOpts: [],
        groupOpts: [],
        roleOpts: [],
        checkedUsers: [],
        checkedGroups: [],
        checkedRoles: []
    });

    React.useEffect(() => {
        loadOptions();
    }, []);

    // 拉取所有下拉选项的异步函数
    async function loadOptions() {
        setState(pre => ({ ...pre, loading: true }));
        try {
            const [usersResult, groupsResult, rolesResult, principalsResult] = await Promise.allSettled([
                describeUsers({ offset: 0, limit: 100 }),
                describeUserGroups({ offset: 0, limit: 100 }),
                describeRoles({ offset: 0, limit: 100 }),
                describeResourcePrincipals({
                    res_type: props.resource_type,
                    res_id: props.resource_id,
                    action: "ALLOW"
                })
            ]);
            const users = usersResult.status === 'fulfilled' ? usersResult.value.content : [];
            const groups = groupsResult.status === 'fulfilled' ? groupsResult.value.content : [];
            const roles = rolesResult.status === 'fulfilled' ? rolesResult.value.content : [];
            const principals = principalsResult.status === 'fulfilled' ? principalsResult.value : undefined;
            const failures = [usersResult, groupsResult, rolesResult, principalsResult]
                .filter((result): result is PromiseRejectedResult => result.status === 'rejected');
            if (failures.length > 0) {
                openErrNotification("部分授权数据加载失败", failures.map(result => String(result.reason)).join('；'));
            }

            setState(pre => ({
                ...pre,
                loading: false,
                userOpts: users.map((item) => ({
                    value: item.id,
                    label: item.name,
                    // 管理员账户不需要可选
                    disabled: item.user_type === 'main'
                })),
                groupOpts: groups.map((item) => ({
                    value: item.id,
                    label: item.name
                })),
                roleOpts: roles.map((item) => ({
                    value: item.id,
                    label: item.name
                })),
                checkedUsers: principals?.data?.users?.map((item) => item.id) || [],
                checkedGroups: principals?.data?.groups?.map((item) => item.id) || [],
                checkedRoles: principals?.data?.roles?.map((item) => item.id) || [],
            }));
        } catch (err) {
            openErrNotification("获取资源授权失败", err as string);
            setState(pre => ({ ...pre, loading: false }));
        }
    }

    const onSubmit: FormProps['onSubmit'] = async (e) => {
        if (e.validateResult !== true) {
            return;
        }
        const { users, groups, roles } = e.fields;

        const newData = {
            resource_id: props.resource_id,
            resource_type: props.resource_type,
            resource_name: props.resource_name,
            principals: {
                users: (users as string[]).map((item) => ({ id: item })),
                groups: (groups as string[]).map((item) => ({ id: item })),
                roles: (roles as string[]).map((item) => ({ id: item }))
            }
        };

        try {
            const ret = await authorizeResources([newData]);
            openInfoNotification("请求成功", "授权成功");
            // 关闭弹窗
            setTimeout(() => {
                props.onClose();
            }, 2000)
        } catch (err) {
            openErrNotification("授权失败", err as string);
        }
    }

    return (
        <>
            <EditorDrawer
                bodyClassName={style.editorBody}
                title="资源授权"
                description="仅调整成员默认策略中的资源可见范围。"
                visible={props.visible}
                width="standard"
                onClose={() => {
                    props.onClose();
                }}
                footer={(
                    <EditorDrawerActions
                        onCancel={props.onClose}
                        onSubmit={() => form.submit({ showErrorMessage: true })}
                        submitText="确认授权"
                    />
                )}
            >
                <Form
                    form={form}
                    labelWidth={100}
                    onSubmit={onSubmit}
                    layout={'vertical'}
                    style={{ width: '100%' }}
                >
                    <section style={{ marginBottom: 18 }}>
                        <h3 style={{ margin: '0 0 12px' }}>资源摘要</h3>
                        <FormItem label="资源类型" name="resource_type" initialData={props.resource_type}>
                            <Input readonly={true} />
                        </FormItem>
                        <FormItem label="命名空间" name="resource_namespace" initialData={resourceNamespace || props.resource_name}>
                            <Input readonly={true} />
                        </FormItem>
                        {!isNamespaceResource && (
                            <FormItem label={resourceNameLabel} name="resource_group" initialData={resourceName || '-'}>
                                <Input readonly={true} />
                            </FormItem>
                        )}
                        <FormItem label="资源ID" name="resource_id" initialData={props.resource_id}>
                            <Input readonly={true} />
                        </FormItem>
                        <FormItem label="资源名称" name="resource_name" initialData={props.resource_name}>
                            <Input readonly={true} />
                        </FormItem>
                    </section>
                    <section style={{ marginBottom: 18 }}>
                        <h3 style={{ margin: '0 0 12px' }}>权限范围</h3>
                        <Space size={8} breakLine>
                            {['查看', '编辑', '发布', '授权管理'].map((item) => (
                                <Tag key={item} variant="light" theme="primary">{item}</Tag>
                            ))}
                        </Space>
                    </section>
                    <Loading loading={state.loading}>
                        <section>
                            <h3 style={{ margin: '0 0 12px' }}>授权对象</h3>
                            <FormItem label="用户" name="users" initialData={state.checkedUsers}>
                                <Select multiple={true} options={state.userOpts} />
                            </FormItem>
                            <FormItem label="用户组" name="groups" initialData={state.checkedGroups}>
                                <Select multiple={true} options={state.groupOpts} />
                            </FormItem>
                            <FormItem label="角色" name="roles" initialData={state.checkedRoles}>
                                <Select multiple={true} options={state.roleOpts} />
                            </FormItem>
                        </section>
                    </Loading>
                </Form>
            </EditorDrawer>
        </>
    )
}

export default React.memo(AuthorizeInput);
