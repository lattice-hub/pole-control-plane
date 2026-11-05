import React from 'react';
import { Form, Input, Space, Button, Radio, Steps, Tree, Transfer, RadioGroup, Row, Select, Textarea } from 'components/Fluent';
import type { FormProps } from 'components/Fluent';
import EditorDrawer from 'components/EditorDrawer';

import style from './index.module.less';
import { useAppDispatch, useAppSelector } from 'modules/store';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import LabelInput from 'components/LabelInput';
import { savePolicyRules, selectPolicyRule, updatePolicyRules } from 'modules/auth/policy';
import { describeAllUsers } from 'services/users';
import { describeAllUserGroups } from 'services/user_group';
import { describeAllRoles } from 'services/role';
import { describeAllAuthPolicies, describeAuthPolicyDetail, describeServerFunctions, getServerFunctionDesc, ServerFunction, ServerFunctionGroup } from 'services/auth_policy';
import { describeAllNamespaces } from 'services/namespace';
import { describeAllServices } from 'services/service';
import { describeAllMCPServers } from 'services/mcp';
import { describeAllA2AAgents } from 'services/a2a';
import PolicyDetailView from './PolicyDetailView';

const { FormItem } = Form;
const { StepItem } = Steps;

type StepType = 'base' | 'principal' | 'resource' | 'interface';
type StepInfo = {
    [key: number]: {
        Cur: StepType;
        Prev: StepType;
        Next: StepType;
    };
}

const customStepNext: StepInfo = {
    1: {
        Cur: 'base',
        Prev: 'base',
        Next: 'principal'
    },
    2: {
        Cur: 'principal',
        Prev: 'base',
        Next: 'resource'
    },
    3: {
        Cur: 'resource',
        Prev: 'principal',
        Next: 'interface'
    },
    4: {
        Cur: 'interface',
        Prev: 'resource',
        Next: 'interface'
    }
}

const defaultStepNext: StepInfo = {
    1: {
        Cur: 'base',
        Prev: 'base',
        Next: 'resource'
    },
    2: {
        Cur: 'resource',
        Prev: 'base',
        Next: 'interface'
    },
    3: {
        Cur: 'interface',
        Prev: 'resource',
        Next: 'interface'
    },
}

const resourceTree = [
    { label: '命名空间', value: 'Namespace' },
    { label: '服务', value: 'Service' },
    { label: '路由规则', value: 'RouteRule' },
    { label: '限流规则', value: 'RateLimitRule' },
    { label: '熔断规则', value: 'CircuitBreakerRule' },
    { label: '探测规则', value: 'FaultDetectRule' },
    { label: '用户', value: 'User' },
    { label: '用户组', value: 'UserGroup' },
    { label: '角色', value: 'Role' },
    { label: '鉴权策略', value: 'AuthPolicy' },
    { label: 'MCP Server', value: 'MCPServer' },
    { label: 'A2A Agent', value: 'A2AAgent' },
];

const principalTree = [
    { label: '用户', value: 'User' },
    { label: '用户组', value: 'UserGroup' },
    { label: '角色', value: 'Role' },
]

const serverFunctionTree = [
    { label: "客户端", value: 'Client' },
    { label: "命名空间", value: 'Namespace' },
    { label: "服务", value: 'Service|ServiceContract' },
    { label: "实例", value: 'Instance' },
    { label: "治理规则", value: 'RouteRule|RateLimitRule|CircuitBreakerRule|FaultDetectRule' },
    { label: "AI 工具", value: 'AINative' },
    { label: "用户", value: 'User' },
    { label: "用户组", value: 'UserGroup' },
    { label: "鉴权策略", value: "AuthPolicy" },
];

interface IPolicyEditorProps {
    op: 'create' | 'edit' | 'view' | 'delete';
    closeDrawer: () => void;
    visible: boolean;
}

const PolicyEditor: React.FC<IPolicyEditorProps> = ({ visible, op, closeDrawer }) => {
    const [form] = Form.useForm();
    const [editable, setEditable] = React.useState(op !== 'view');
    const dispatch = useAppDispatch();
    const currentPolicy = useAppSelector(selectPolicyRule);

    // 下拉/Transfer数据
    const [state, setState] = React.useState<{
        principalOptions: { [key: string]: { value: string; label: string }[] };
        resourceOptions: { [key: string]: { value: string; label: string }[] };
        viewFunctionOptions: { value: string; label: string }[];
        functionOptions: ServerFunctionGroup[];
        loading: boolean;
        step: number;
        activePrincipalNode: string;
        activeFuncNode: string;
        activeResNode: string;
        selectResources: { [key: string]: { all: boolean; ids: string[] } };
        selectPrincipals: { [key: string]: { all: boolean; ids: string[] } };
        // selectFunctions 用于小白化的勾选页面操作
        selectFunctions: { [key: string]: { group: string; functions: string[] } };
        // customFunctions 用于主动填写方法接口信息，支持前缀匹配的填写操作
        customFunctions: string[];
        // useAllFunc 是否全部接口（包括新增）
        useAllFunc: boolean;
        // swithCustomFunc 是否让用户自行编辑接口授权的信息
        swithCustomFunc: boolean;
        formValues: any; // 新增字段
    }>({
        functionOptions: [],
        viewFunctionOptions: [],
        principalOptions: {} as { [key: string]: { value: string; label: string }[] },
        resourceOptions: {} as { [key: string]: { value: string; label: string }[] },
        loading: false,
        step: 1,
        activePrincipalNode: 'User',
        activeFuncNode: 'Namespace',
        activeResNode: 'Namespace',
        selectResources: {},
        selectPrincipals: {} as { [key: string]: { all: boolean; ids: string[] } },
        selectFunctions: {} as { [key: string]: { group: string; functions: string[] } },
        customFunctions: [],
        swithCustomFunc: false,
        useAllFunc: false,
        formValues: {}, // 新增字段
    });

    const [resourceStatus, setResourceStatus] = React.useState<Record<string, 'loading' | 'error' | 'ready'>>({});
    const resourceSession = React.useRef(0);
    const resourceRequests = React.useRef<Record<string, number>>({});
    React.useEffect(() => {
        resourceSession.current += 1;
        resourceRequests.current = {};
        setResourceStatus({});
        setState(prev => ({ ...prev, resourceOptions: {} }));
        return () => { resourceSession.current += 1; };
    }, [visible, currentPolicy, op]);

    // Helper functions to update individual state properties
    const setLoading = (loading: boolean) => setState(prev => ({ ...prev, loading }));
    // Destructure state for easier access in the component
    const { loading } = state;

    // 拉取所有下拉选项的异步函数
    async function loadOptions() {
        try {
            const users = await describeAllUsers();
            const groups = await describeAllUserGroups();
            const rolesRes = await describeAllRoles();
            const serverFnRes = await describeServerFunctions();
            const functionList: { value: string; label: string }[] = filterFunctionOptions(state.activeFuncNode, serverFnRes.list)?.map((item) => ({
                value: item.id,
                label: item.desc,
            }));

            if (op !== 'create') {
                await loadCurRule(serverFnRes.list);
            }
            setState(prev => ({
                ...prev,
                principalOptions: {
                    User: users.map((u: any) => ({ value: u.id, label: u.name })),
                    UserGroup: groups.map((g: any) => ({ value: g.id, label: g.name })),
                    Role: rolesRes.filter((r: any) => !r.default_role).map((r: any) => ({ value: r.id, label: r.name })),
                },
                functionOptions: serverFnRes.list,
                viewFunctionOptions: functionList,
            }));
        } catch (err) {
            setLoading(false);
            openErrNotification('请求错误', String(err));
        }
    }

    const transferFunctions = (functions: string[], functionOptions: ServerFunctionGroup[]): { [key: string]: { group: string; functions: string[] } } => {
        return functions?.reduce((result: { [key: string]: { group: string; functions: string[] } }, func: string) => {
            // If it's a wildcard function, don't process further
            if (func === '*') {
                setState(prev => ({ ...prev, useAllFunc: true }));
                return result;
            }

            // Find which group this function belongs to
            for (const group of functionOptions) {
                for (const f of group.functions) {
                    // Remove any * suffix from the function name
                    const cleanFunc = func.endsWith('*') ? func.slice(0, -1) : func;
                    if (f === cleanFunc || f.startsWith(`${cleanFunc}`)) {
                        let groupKey = group.name;
                        if (groupKey === 'Service' || groupKey === 'ServiceContract') {
                            groupKey = 'Service|ServiceContract';
                        }
                        if (groupKey === 'RouteRule' || groupKey === 'RateLimitRule' || groupKey === 'CircuitBreakerRule' || groupKey === 'FaultDetectRule') {
                            groupKey = 'RouteRule|RateLimitRule|CircuitBreakerRule|FaultDetectRule';
                        }
                        // Initialize the group if it doesn't exist
                        if (!result[groupKey]) {
                            result[groupKey] = {
                                group: groupKey,
                                functions: []
                            };
                        }

                        // Add function to its group
                        if (!result[groupKey].functions.includes(f)) {
                            result[groupKey].functions.push(f);
                        }
                    }
                }
            }
            return result;
        }, {}) || {}
    }

    async function loadCurRule(functionOptions: ServerFunctionGroup[]) {
        try {
            // 查询当前鉴权策略的详细数据
            const ret = await describeAuthPolicyDetail({ id: currentPolicy.id as string });
            if (ret.strategy) {
                const { name, action, comment, principals, resources, functions, metadata } = ret.strategy;
                const selectFuncs: { [key: string]: { group: string; functions: string[] } } = transferFunctions(functions || [], functionOptions);

                setState(prev => ({
                    ...prev,
                    selectPrincipals: {
                        ...prev.selectPrincipals,
                        User: {
                            all: principals?.users?.length === 1 && principals?.users?.[0].id === '*',
                            ids: principals?.users?.map((item: any) => item.id) || []
                        },
                        UserGroup: {
                            all: principals?.groups?.length === 1 && principals?.groups?.[0].id === '*',
                            ids: principals?.groups?.map((item: any) => item.id) || []
                        },
                        Role: {
                            all: principals?.roles?.length === 1 && principals?.roles?.[0].id === '*',
                            ids: principals?.roles?.map((item: any) => item.id) || []
                        },
                    },
                    selectFunctions: selectFuncs,
                    selectResources: {
                        ...prev.selectResources,
                        Namespace: {
                            all: resources?.namespaces?.length === 1 && resources?.namespaces?.[0].id === '*',
                            ids: resources?.namespaces?.map((item: any) => item.id) || []
                        },
                        Service: {
                            all: resources?.services?.length === 1 && resources?.services?.[0].id === '*',
                            ids: resources?.services?.map((item: any) => item.id) || []
                        },
                        User: {
                            all: resources?.users?.length === 1 && resources?.users?.[0].id === '*',
                            ids: resources?.users?.map((item: any) => item.id) || []
                        },
                        UserGroup: {
                            all: resources?.user_groups?.length === 1 && resources?.user_groups?.[0].id === '*',
                            ids: resources?.user_groups?.map((item: any) => item.id) || []
                        },
                        Role: {
                            all: resources?.roles?.length === 1 && resources?.roles?.[0].id === '*',
                            ids: resources?.roles?.map((item: any) => item.id) || []
                        },
                        RouteRule: {
                            all: resources?.route_rules?.length === 1 && resources?.route_rules?.[0].id === '*',
                            ids: resources?.route_rules?.map((item: any) => item.id) || []
                        },
                        RateLimitRule: {
                            all: resources?.ratelimit_rules?.length === 1 && resources?.ratelimit_rules?.[0].id === '*',
                            ids: resources?.ratelimit_rules?.map((item: any) => item.id) || []
                        },
                        CircuitBreakerRule: {
                            all: resources?.circuitbreaker_rules?.length === 1 && resources?.circuitbreaker_rules?.[0].id === '*',
                            ids: resources?.circuitbreaker_rules?.map((item: any) => item.id) || []
                        },
                        FaultDetectRule: {
                            all: resources?.faultdetect_rules?.length === 1 && resources?.faultdetect_rules?.[0].id === '*',
                            ids: resources?.faultdetect_rules?.map((item: any) => item.id) || []
                        },
                        MCPServer: {
                            all: resources?.mcp_servers?.length === 1 && resources?.mcp_servers?.[0].id === '*',
                            ids: resources?.mcp_servers?.map((item: any) => item.id) || []
                        },
                        A2AAgent: {
                            all: resources?.a2a_agents?.length === 1 && resources?.a2a_agents?.[0].id === '*',
                            ids: resources?.a2a_agents?.map((item: any) => item.id) || []
                        },
                    }
                }));
                form.setFieldsValue({
                    name: name,
                    action: (action || '').toUpperCase(),
                    comment: comment,
                    policy_labels: metadata ? Object.entries(metadata).map(([key, value]) => ({ key, value })) : [],
                });
            }
        } catch (err) {
            console.error(err);
        }
        console.log(state.selectFunctions);
    }

    // 表单初始化（支持create/edit/view）
    React.useEffect(() => {
        if (!visible) return;
        setEditable(op !== 'view');
        loadOptions()
    }, [visible, currentPolicy, op]);

    const filterFunctionOptions = (keyword: string, funcGroup: ServerFunctionGroup[]): ServerFunction[] => {
        const targetGroup = funcGroup.filter(item => {
            for (const searchKey of keyword.split("|")) {
                if (searchKey === "" || searchKey === undefined) {
                    continue
                }
                if (searchKey === item.name) {
                    return true
                }
            }
            return false
        })

        if (targetGroup.length === 0) {
            return [];
        }

        const functionList: ServerFunction[] = [];
        for (const group of targetGroup) {
            functionList.push(...group.functions.map(item => ({
                id: item,
                name: item,
                desc: getServerFunctionDesc(group.name, item)
            })))
        }
        return functionList;
    }

    const handleTreeNodeClick = async (label: 'principal' | 'function' | 'resource', value: string) => {
        switch (label) {
            case 'function':
                const functionList: { value: string; label: string }[] = filterFunctionOptions(value, state.functionOptions)?.map((item) => ({
                    value: item.id,
                    label: item.desc,
                }));
                setState(prev => ({
                    ...prev,
                    activeFuncNode: value,
                    viewFunctionOptions: functionList,
                }));
                break;
            case 'principal':
                setState(prev => ({
                    ...prev,
                    activePrincipalNode: value,
                }));
                break;
            case 'resource':
                setState(prev => ({ ...prev, activeResNode: value }));
                if (state.resourceOptions[value] || resourceStatus[value] === 'loading') return;
                const session = resourceSession.current;
                const request = (resourceRequests.current[value] || 0) + 1;
                resourceRequests.current[value] = request;
                setResourceStatus(prev => ({ ...prev, [value]: 'loading' }));
                let resources: { value: string; label: string }[] = [];
                try {
                    let data;
                    switch (value) {
                        case 'Namespace':
                            data = await describeAllNamespaces();
                            resources = data?.map((item: any) => ({ value: item.id, label: item.name })) || [];
                            break;
                        case 'Service':
                            data = await describeAllServices();
                            resources = data?.map((item: any) => ({ value: item.id, label: item.name })) || [];
                            break;
                        case 'User':
                            data = await describeAllUsers();
                            resources = data?.map((item: any) => ({ value: item.id, label: item.name })) || [];
                            break;
                        case 'UserGroup':
                            data = await describeAllUserGroups();
                            resources = data?.map((item: any) => ({ value: item.id, label: item.name })) || [];
                            break;
                        case 'Role':
                            data = await describeAllRoles();
                            resources = data?.map((item: any) => ({ value: item.id, label: item.name })) || [];
                            break;
                        case 'AuthPolicy':
                            data = await describeAllAuthPolicies();
                            resources = data?.content?.map((item: any) => ({ value: item.id, label: item.name })) || [];
                            break;
                        case 'MCPServer':
                            data = await describeAllMCPServers();
                            resources = data?.map((item: any) => ({ value: item.id, label: `${item.namespace}/${item.name}` })) || [];
                            break;
                        case 'A2AAgent':
                            data = await describeAllA2AAgents();
                            resources = data?.map((item: any) => ({ value: item.id, label: `${item.namespace}/${item.name}` })) || [];
                            break;
                        case 'RouteRule':
                            break;
                        case 'RateLimitRule':
                            break;
                        case 'CircuitBreakerRule':
                            break;
                        case 'FaultDetectRule':
                            break;
                        case 'LaneRule':
                            break;
                        default:
                            break;
                    }

                    if (resourceSession.current !== session || resourceRequests.current[value] !== request) return;
                    setState(prev => ({ ...prev, resourceOptions: { ...prev.resourceOptions, [value]: resources } }));
                    setResourceStatus(prev => ({ ...prev, [value]: 'ready' }));
                } catch (error) {
                    if (resourceSession.current !== session || resourceRequests.current[value] !== request) return;
                    setResourceStatus(prev => ({ ...prev, [value]: 'error' }));
                    openErrNotification('获取资源列表失败', String(error));
                }
                break;
            default:
                break;
        }
    }

    // 步骤切换时保存/恢复表单数据
    const handleStepChange = async (nextStep: number) => {
        // 保存当前表单数据
        if (state.step === 1) {
            const values = form.getFieldsValue(true);
            setState(prev => ({ ...prev, formValues: values, step: nextStep }));
        } else if (nextStep === 1) {
            // 恢复表单数据
            form.setFieldsValue(state.formValues);
            setState(prev => ({ ...prev, step: nextStep }));
        } else {
            // 其他步骤直接切换
            setState(prev => ({ ...prev, step: nextStep }));
        }
    };

    // 表单提交
    const onSubmit: FormProps['onSubmit'] = async (e) => {
        if (e.validateResult !== true) {
            return
        };
        setLoading(true);
        const labels = (form.getFieldValue('policy_labels') || []) as { key: string, value: string }[];

        const newData = {
            id: op === 'create' ? undefined : currentPolicy.id,
            name: String(form.getFieldValue('name') || ''),
            action: String(form.getFieldValue('action') || '').toUpperCase(),
            comment: String(form.getFieldValue('comment') || ''),
            principals: {
                users: state.selectPrincipals['User']?.all ? [{ id: "*" }] : state.selectPrincipals['User']?.ids.map((id) => ({ id })) || [],
                groups: state.selectPrincipals['UserGroup']?.all ? [{ id: "*" }] : state.selectPrincipals['UserGroup']?.ids.map((id) => ({ id })) || [],
                roles: state.selectPrincipals['Role']?.all ? [{ id: "*" }] : state.selectPrincipals['Role']?.ids.map((id) => ({ id })) || [],
            },
            resources: {
                namespaces: state.selectResources['Namespace']?.all ? [{ id: "*" }] : state.selectResources['Namespace']?.ids.map((id) => ({ id })) || [],
                services: state.selectResources['Service']?.all ? [{ id: "*" }] : state.selectResources['Service']?.ids.map((id) => ({ id })) || [],
                users: state.selectResources['User']?.all ? [{ id: "*" }] : state.selectResources['User']?.ids.map((id) => ({ id })) || [],
                user_groups: state.selectResources['UserGroup']?.all ? [{ id: "*" }] : state.selectResources['UserGroup']?.ids.map((id) => ({ id })) || [],
                roles: state.selectResources['Role']?.all ? [{ id: "*" }] : state.selectResources['Role']?.ids.map((id) => ({ id })) || [],
                mcp_servers: state.selectResources['MCPServer']?.all ? [{ id: "*" }] : state.selectResources['MCPServer']?.ids.map((id) => ({ id })) || [],
                a2a_agents: state.selectResources['A2AAgent']?.all ? [{ id: "*" }] : state.selectResources['A2AAgent']?.ids.map((id) => ({ id })) || [],
            },
            functions: state.useAllFunc ? ['*'] : Object.values(state.selectFunctions).reduce((acc: string[], cur) => {
                if (cur.functions.length > 0) {
                    acc.push(...cur.functions);
                }
                return acc;
            }, []),
            metadata: labels.reduce((acc: any, cur: any) => { acc[cur.key] = cur.value; return acc; }, {}),
        };

        let result;
        try {
            if (op === 'create') {
                result = await dispatch(savePolicyRules({ state: newData }));
            } else {
                result = await dispatch(updatePolicyRules({ state: newData }));
            }
        } catch (err) {
            setLoading(false);
            openErrNotification('请求错误', String(err));
            return;
        }
        setLoading(false);
        if (result.meta.requestStatus === 'fulfilled') {
            openInfoNotification('操作成功', op === 'create' ? '新建策略规则成功' : '修改策略规则成功');
            closeDrawer();
        } else {
            openErrNotification('请求错误', result.payload as string || '未知错误');
        }
    };

    const maxStep = currentPolicy.default_strategy ? 3 : 4;
    const stepInfo = currentPolicy.default_strategy ? defaultStepNext : customStepNext;
    React.useEffect(() => {
        if (visible && stepInfo[state.step]?.Cur === 'resource' && !resourceStatus[state.activeResNode]) {
            void handleTreeNodeClick('resource', state.activeResNode);
        }
    // Load the active category on entry; retries are explicit.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [visible, state.step, state.activeResNode, resourceStatus]);
    const policyViewMode = op === 'view' && !editable;
    const drawerFooter = (
        <Space>
            {state.step > 1 && (
                <Button theme="default" onClick={() => handleStepChange(state.step - 1)}>
                    上一步
                </Button>
            )}
            {state.step < maxStep && (
                <Button theme="default" onClick={() => handleStepChange(state.step + 1)}>
                    下一步
                </Button>
            )}
            {editable && state.step === maxStep && (
                <Button
                    theme="primary"
                    onClick={() => form.submit()}
                    loading={loading}
                >
                    提交
                </Button>
            )}
        </Space>
    );

    return (
        <EditorDrawer
            visible={visible}
            onClose={() => {
                form.reset();
                closeDrawer();
            }}
            title={op === 'create' ? '新建策略规则' : editable ? '编辑策略规则' : '策略详情'}
            description={op === 'create' ? '按主体、资源和接口范围逐步定义访问策略。' : undefined}
            width="wide"
            footer={policyViewMode ? false : drawerFooter}
            className={policyViewMode ? style.policyDetailDrawer : undefined}
        >
            {policyViewMode ? (
                <PolicyDetailView policyId={currentPolicy.id} />
            ) : (
                <>
                    {currentPolicy.default_strategy ?
                        (
                            <Steps current={state.step}>
                                <StepItem value={1} title="基本信息" />
                                <StepItem value={2} title="资源信息" />
                                <StepItem value={3} title="接口信息" />
                            </Steps>
                        )
                        :
                        (
                            <Steps current={state.step}>
                                <StepItem value={1} title="基本信息" />
                                <StepItem value={2} title="成员信息" />
                                <StepItem value={3} title="资源信息" />
                                <StepItem value={4} title="接口信息" />
                            </Steps>
                        )}
                    <Form
                        form={form}
                        labelWidth={120}
                        onSubmit={onSubmit}
                        style={{ padding: '24px 0' }}
                    >
                {/* Step 1: Basic Information */}
                {stepInfo[state.step].Cur === 'base' && (
                    <>
                        <FormItem
                            label="策略名称"
                            name="name"
                            initialData={currentPolicy.name}
                            rules={[{ required: true, message: '策略名称不能为空' }, { max: 64, message: '长度不超过64个字符' }]}
                        >
                            <Input placeholder="请输入策略名称" disabled={op !== 'create'} />
                        </FormItem>
                        <FormItem
                            label="策略动作"
                            name="action"
                            initialData={currentPolicy.action === '' ? 'ALLOW' : currentPolicy.action}
                            rules={[{ required: true, message: '策略动作不能为空' }]}
                        >
                            <RadioGroup theme='button' variant='primary-filled' disabled={!editable || currentPolicy.default_strategy}>
                                <Radio.Button value="ALLOW">允许</Radio.Button>
                                <Radio.Button value="DENY">拒绝</Radio.Button>
                            </RadioGroup>
                        </FormItem>
                        <FormItem
                            label="描述"
                            name="comment"
                            initialData={currentPolicy.comment}
                            rules={[{ max: 255, message: '长度不超过255个字符' }]}
                        >
                            <Input placeholder="请输入策略描述" disabled={!editable} />
                        </FormItem>
                        <LabelInput
                            form={form}
                            label='策略标签'
                            name='policy_labels'
                            editable={editable}
                        />
                    </>
                )}

                {/* Step 2: Member Information */}
                {/* 默认策略不能修改默认成员数据信息 */}
                {stepInfo[state.step].Cur === 'principal' && (
                    <>
                        <Space className={style.policySelectionLayout}>
                            <div className={style.treeContent}>
                                <Tree
                                    data={principalTree}
                                    activable
                                    hover
                                    lazy
                                    transition
                                    valueMode={"onlyLeaf"}
                                    defaultActived={[state.activePrincipalNode]}
                                    actived={[state.activePrincipalNode]}
                                    onClick={({ node }: { node: { value: string } }) => handleTreeNodeClick('principal', node.value as string)}
                                />
                            </div>
                            <Space className={style.policySelectionMain} direction='vertical'>
                                <RadioGroup
                                    theme='button'
                                    variant='primary-filled'
                                    disabled={!editable}
                                    value={state.selectPrincipals[state.activePrincipalNode]?.all ? 'all' : 'custom'}
                                    onChange={(value: string) => {
                                        setState(prev => ({
                                            ...prev,
                                            selectPrincipals: {
                                                ...prev.selectPrincipals,
                                                [prev.activePrincipalNode]: {
                                                    ...prev.selectPrincipals[prev.activePrincipalNode],
                                                    all: value === 'all',
                                                },
                                            },
                                        }));
                                    }}
                                >
                                    <Radio.Button value="custom">部分（自定义）</Radio.Button>
                                    <Radio.Button value="all">全部（包括新增）</Radio.Button>
                                </RadioGroup>
                                {!state.selectPrincipals[state.activePrincipalNode]?.all && (
                                    <Transfer
                                        key={state.activePrincipalNode}
                                        title={['可选主体', '已选主体']}
                                        search={true}
                                        data={state.principalOptions[state.activePrincipalNode]}
                                        value={state.selectPrincipals[state.activePrincipalNode]?.ids}
                                        disabled={!editable}
                                        onChange={(target: string[]) => {
                                            const category = state.activePrincipalNode;
                                            setState(prev => ({ ...prev, selectPrincipals: {
                                                ...prev.selectPrincipals,
                                                [category]: { all: false, ids: target },
                                            } }));
                                        }}
                                    />
                                )}
                            </Space>
                        </Space>
                    </>
                )}
                {/* Step 3: Resource Information */}
                {stepInfo[state.step].Cur === 'resource' && (
                    <>
                        <Space className={style.policySelectionLayout}>
                            <div className={style.treeContent}>
                                <Tree
                                    data={resourceTree}
                                    activable
                                    hover
                                    lazy
                                    transition
                                    valueMode={"onlyLeaf"}
                                    defaultActived={[state.activeResNode]}
                                    actived={[state.activeResNode]}
                                    onClick={({ node }: { node: { value: string } }) => handleTreeNodeClick('resource', node.value as string)}
                                />
                            </div>
                            <Space className={style.policySelectionMain} direction='vertical'>
                                <RadioGroup
                                    theme='button'
                                    variant='primary-filled'
                                    disabled={!editable}
                                    value={state.selectResources[state.activeResNode]?.all ? 'all' : 'custom'}
                                    onChange={(value: string) => {
                                        setState(prev => ({
                                            ...prev,
                                            selectResources: {
                                                ...prev.selectResources,
                                                [prev.activeResNode]: {
                                                    ...prev.selectResources[prev.activeResNode],
                                                    all: value === 'all',
                                                },
                                            },
                                        }));
                                    }}
                                >
                                    <Radio.Button value="custom">部分（自定义）</Radio.Button>
                                    <Radio.Button value="all">全部（包括新增）</Radio.Button>
                                </RadioGroup>
                                {resourceStatus[state.activeResNode] === 'error' && <p role="alert">资源加载失败。<Button variant="text" onClick={() => { setResourceStatus(prev => { const next = { ...prev }; delete next[state.activeResNode]; return next; }); }}>重试</Button></p>}
                                {!state.selectResources[state.activeResNode]?.all && resourceStatus[state.activeResNode] !== 'error' && (
                                    <Transfer
                                        key={state.activeResNode}
                                        title={['可选资源', '已选资源']}
                                        loading={!resourceStatus[state.activeResNode] || resourceStatus[state.activeResNode] === 'loading'}
                                        search={true}
                                        data={state.resourceOptions[state.activeResNode]}
                                        value={state.selectResources[state.activeResNode]?.ids}
                                        disabled={!editable}
                                        onChange={(target: string[]) => {
                                            const category = state.activeResNode;
                                            setState(prev => ({ ...prev, selectResources: {
                                                ...prev.selectResources,
                                                [category]: { all: false, ids: target },
                                            } }));
                                        }}
                                    />
                                )}
                            </Space>
                        </Space>
                    </>
                )}
                {/* 需要支持高级参数填写，走 TagSelect 用户可自行填写 */}
                {stepInfo[state.step].Cur === 'interface' && (
                    <>
                        <div className={style.policyModeRow}>
                            <div className={style.policyModeScope}>
                                {!state.swithCustomFunc && (
                                    <RadioGroup
                                        theme='button'
                                        variant='primary-filled'
                                        disabled={!editable}
                                        value={state.useAllFunc ? 'all' : 'custom'}
                                        onChange={(value: string) => {
                                            setState(prev => ({
                                                ...prev,
                                                useAllFunc: value === 'all',
                                            }));
                                        }}
                                    >
                                        <Radio.Button value="custom">部分（自定义）</Radio.Button>
                                        <Radio.Button value="all">全部（包括新增）</Radio.Button>
                                    </RadioGroup>
                                )}
                            </div>
                            <div className={style.policyModeSwitch}>
                                <RadioGroup
                                    layout="horizontal"
                                    value={state.swithCustomFunc ? 'text' : 'visual'}
                                    disabled={!editable}
                                    onChange={(value: string) => {
                                        setState(prev => ({
                                            ...prev,
                                            swithCustomFunc: value === 'text',
                                        }));
                                    }}
                                >
                                    <Radio.Button value="visual">可视化选择</Radio.Button>
                                    <Radio.Button value="text">文本模式</Radio.Button>
                                </RadioGroup>
                            </div>
                        </div>
                        {state.swithCustomFunc ? (
                            <Space style={{ width: '100%' }} direction='vertical'>
                                <Select
                                    filterable={true}
                                    creatable={true}
                                    multiple={true}
                                    disabled={!editable}
                                    options={state.functionOptions.reduce((acc: { value: string; label: string }[], group) => {
                                        acc.push(...group.functions.map(item => ({
                                            value: item,
                                            label: getServerFunctionDesc(group.name, item),
                                        })));
                                        return acc;
                                    }, [])}
                                    onChange={(value: string[]) => {
                                        setState(prev => ({
                                            ...prev,
                                            customFunctions: value,
                                        }));
                                    }}
                                />
                                {/* 预览框 */}
                                <Textarea
                                    // style={{marginTop: 10}}
                                    readOnly={true}
                                    value={JSON.stringify(state.customFunctions, null, 2)}
                                    rows={20}
                                />
                            </Space>
                        ) : (
                            <>
                                {!state.useAllFunc && (
                                    <Row className={style.policySelectionContent}>
                                        <Space className={style.policySelectionLayout} style={{ marginTop: 20 }}>
                                            <div className={style.treeContent}>
                                                <Tree
                                                    data={serverFunctionTree}
                                                    activable
                                                    hover
                                                    lazy
                                                    transition
                                                    valueMode={"onlyLeaf"}
                                                    defaultActived={[state.activeFuncNode]}
                                                    actived={[state.activeFuncNode]}
                                                    onClick={({ node }: { node: { value: string } }) => handleTreeNodeClick("function", node.value as string)}
                                                />
                                            </div>
                                            <div className={style.policySelectionMain}>
                                                <Transfer
                                                    key={state.activeFuncNode}
                                                    title={['可选接口', '已选接口']}
                                                    search={true}
                                                    data={state.viewFunctionOptions}
                                                    value={state.selectFunctions[state.activeFuncNode]?.functions}
                                                    disabled={!editable}
                                                    onChange={(target: string[]) => {
                                                        const category = state.activeFuncNode;
                                                        setState(prev => ({ ...prev, selectFunctions: {
                                                            ...prev.selectFunctions,
                                                            [category]: { group: category, functions: target },
                                                        } }));
                                                    }}
                                                />
                                            </div>
                                        </Space>
                                    </Row>
                                )}
                            </>
                        )}
                    </>
                )}
                    </Form>
                </>
            )}
        </EditorDrawer>
    );
};

export default PolicyEditor;
