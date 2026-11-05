import React, { } from 'react';
import {
    Button,
    Form,
    Input,
    Select,
    InputNumber,
    Radio,
    Space,
    Textarea,
    StickyTool,
    FormProps,
    Tag,
    Popup
} from 'components/Fluent';
import { AddIcon, ChevronRightIcon, CloseIcon, DeleteIcon, Edit1Icon, RocketIcon, RollbackIcon, SaveIcon } from 'components/Fluent/icons';

import { API, HTTPMethod, HTTPMethodOption, InterfaceProtocol, Label, MatchType, Op } from "services/types";
import RuleLabelField from "../shared/RuleLabelField";
import CollapsibleSection from "../shared/CollapsibleSection";
import { GovernanceServiceContext } from "../shared/serviceContext";
import { useRuleNamespace } from "../shared/ruleNamespace";
import shared from "../shared/governance.module.less";
import styles from './FaultDetectEditor.module.less';
import {
    FaultDetectRule,
    FaultDetectProtocol,
    FaultDetectSubRule
} from 'services/faultdetect';
import {
    buildFaultDetectSubmitPayload,
    describeProbePort,
    describeProbeSummary,
    isCustomProbePort,
    normalizeFaultDetectRulesDraft,
    PayloadMatchOptions,
    receiveToText,
    validateFaultDetectDraft,
} from './faultDetectEditorUtils';
import PublishForm from '../RuleRelease/PublishForm';
import RuleStickyAction from '../RuleRelease/RuleStickyAction';
import { PolicySourceType } from 'services/auth_policy';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import Text from 'components/Text';
import { useAppDispatch, useAppSelector } from 'modules/store';
import { listOneFaultDetect, resetFaultDetect, saveFaultDetects, selectFaultDetect, updateFaultDetects } from 'modules/governance/faultdetect';
import { cleanServicePage, listAllServices, selectService } from 'modules/discovery/service';
import { cleanNamespacePage, listAllNamespaces, selectNamespace } from 'modules/namespace';

const { FormItem } = Form;
const { Group: RadioGroup } = Radio;
const { StickyItem } = StickyTool;

const displayText = (value?: string | number | null) => {
    if (value === undefined || value === null || value === '') {
        return '-';
    }
    return String(value);
};


interface FaultDetectDO {
    id?: string
    name: string
    description: string
    priority?: number
    rules: FaultDetectSubRule[]
    targetService: {
        namespace: string
        service: string
        api?: API
    }
    interval: number
    timeout: number
    port: number
    // 协议，支持HTTP, TCP, UPD
    protocol: string
    httpConfig?: {
        method: string
        url: string
        headers: Label[]
        body: string
    }
    tcpConfig?: {
        send: string
        receive: string[] | string
        match?: string
    }
    udpConfig?: {
        send: string
        receive: string[] | string
        match?: string
    }
    metadata?: Record<string, string>
}

const defaultApi = (): API => ({
    protocol: InterfaceProtocol.HTTP,
    method: '',
    path: {
        type: MatchType.EXACT,
        value: '',
        value_type: 'TEXT'
    }
});

const defaultTargetService = (): FaultDetectDO['targetService'] => ({
    namespace: '',
    service: '',
    api: defaultApi()
});

const normalizeTargetService = (target?: any): FaultDetectDO['targetService'] => ({
    ...defaultTargetService(),
    ...(target || {}),
    api: {
        ...defaultApi(),
        ...(target?.api || {}),
        path: {
            ...defaultApi().path,
            ...(target?.api?.path || {})
        }
    }
});

const defaultFaultDetectSubRule = (): FaultDetectSubRule => ({
    interval: 5,
    timeout: 2,
    port: 0,
    protocol: FaultDetectProtocol.HTTP,
    httpConfig: {
        method: HTTPMethod.GET,
        url: '/healthz',
        headers: [],
        body: ''
    },
    tcpConfig: {
        send: '',
        receive: [],
        match: 'EXACT'
    },
    udpConfig: {
        send: '',
        receive: [],
        match: 'EXACT'
    },
    disable: false
});

export const defaultFaultDetectRule: () => FaultDetectDO = () => ({
    name: '',
    description: '',
    priority: 0,
    rules: [defaultFaultDetectSubRule()],
    targetService: defaultTargetService(),
    interval: defaultFaultDetectSubRule().interval,
    timeout: defaultFaultDetectSubRule().timeout,
    port: defaultFaultDetectSubRule().port,
    protocol: defaultFaultDetectSubRule().protocol,
    httpConfig: defaultFaultDetectSubRule().httpConfig
});

const ensureFaultDetectRules = (rule?: Partial<FaultDetectRule> | null): FaultDetectSubRule[] => {
    const rawRules = rule?.rules || [];
    if (rawRules.length > 0) {
        return normalizeFaultDetectRulesDraft(rawRules);
    }

    if (rule?.targetService || rule?.protocol || rule?.httpConfig || rule?.tcpConfig || rule?.udpConfig) {
        return normalizeFaultDetectRulesDraft([{
            interval: rule.interval ?? 30,
            timeout: rule.timeout ?? 60,
            port: rule.port ?? 0,
            protocol: rule.protocol || FaultDetectProtocol.HTTP,
            httpConfig: {
                ...defaultFaultDetectSubRule().httpConfig,
                ...(rule.httpConfig || {})
            },
            tcpConfig: {
                ...defaultFaultDetectSubRule().tcpConfig,
                ...(rule.tcpConfig || {})
            },
            udpConfig: {
                ...defaultFaultDetectSubRule().udpConfig,
                ...(rule.udpConfig || {})
            },
            disable: false
        }]);
    }

    return [defaultFaultDetectSubRule()];
};

const normalizeFaultDetectFormValue = (rule?: Partial<FaultDetectRule> | null): FaultDetectDO => {
    const rules = ensureFaultDetectRules(rule);
    const primary = rules[0];
    const legacyPrimary = ((rule?.rules || []) as any[])[0] || {};
    const targetService = (rule as any)?.targetService || (rule as any)?.target_service || legacyPrimary.targetService || legacyPrimary.target_service || defaultFaultDetectRule().targetService;
    return {
        ...(defaultFaultDetectRule()),
        ...(rule || {}),
        rules,
        targetService: normalizeTargetService(targetService),
        priority: rule?.priority ?? 0,
        interval: primary.interval,
        timeout: primary.timeout,
        port: primary.port,
        protocol: primary.protocol,
        httpConfig: primary.httpConfig,
        tcpConfig: primary.tcpConfig,
        udpConfig: primary.udpConfig,
        metadata: rule?.metadata || {}
    };
};

interface IFaultDetectEditorProps {
    op: Op;
    refresh: (close: boolean) => void;
    serviceContext?: GovernanceServiceContext;
}

const FaultDetectEditor: React.FC<IFaultDetectEditorProps> = (props) => {
    const ruleNamespace = useRuleNamespace();
    const [form] = Form.useForm();
    const dispatch = useAppDispatch();

    const detectState = useAppSelector(selectFaultDetect);
    const { editRule, viewRule } = detectState;

    const [editorState, setEditorState] = React.useState<{
        model: Op;
        publishView: boolean;
        loading: boolean;
        editable: boolean;
        content: string;
        visible: boolean;
    }>({
        model: 'view',
        content: '',
        editable: props.op === 'create',
        publishView: false,
        loading: false,
        visible: false
    });
    const [basicInfoCollapsed, setBasicInfoCollapsed] = React.useState(false);
    const [collapsedRuleIndexes, setCollapsedRuleIndexes] = React.useState<Set<number>>(new Set());
    const [formRevision, setFormRevision] = React.useState(0);
    const [ruleDrafts, setRuleDrafts] = React.useState<FaultDetectSubRule[]>(ensureFaultDetectRules(defaultFaultDetectRule()));
    const [protocolOverrides, setProtocolOverrides] = React.useState<Record<number, FaultDetectProtocol>>({});
    const syncingRuleDraftsRef = React.useRef(false);
    const persistedValueRef = React.useRef<ReturnType<typeof normalizeFaultDetectFormValue>>(normalizeFaultDetectFormValue(defaultFaultDetectRule()));
    const editRuleId = editRule?.id || '';

    // 选项数据
    const namespaceState = useAppSelector(selectNamespace);
    const { datas: namespaceDatas } = namespaceState;

    const serviceState = useAppSelector(selectService);
    const { datas: serviceDatas } = serviceState;

    React.useEffect(() => {
        // 创建模式，设置默认值
        const initialValue = normalizeFaultDetectFormValue(defaultFaultDetectRule());
        persistedValueRef.current = initialValue;
        form.setFieldsValue(initialValue);
        setRuleDrafts(initialValue.rules);
        setProtocolOverrides({});
        setFormRevision(prev => prev + 1);
        dispatch(listAllNamespaces())
            .then(res => {
                if (res.meta.requestStatus === 'rejected') {
                    openErrNotification('获取命名空间列表失败', res?.payload as string);
                }
            })
        dispatch(listAllServices())
            .then(res => {
                if (res.meta.requestStatus === 'rejected') {
                    openErrNotification('获取服务列表失败', res?.payload as string);
                }
            })

        return () => {
            dispatch(cleanNamespacePage());
            dispatch(cleanServicePage());
            dispatch(resetFaultDetect());
        }
    }, []);

    React.useEffect(() => {
        setBasicInfoCollapsed(false);
        if (!editRuleId) {
            return;
        }
        dispatch(listOneFaultDetect({ id: editRuleId })).then(res => {
            if (res.meta.requestStatus === 'rejected') {
                openErrNotification('请求失败', `获取主动探测规则详情失败: ${res?.payload as string}`);
                return;
            }
            const { editRule } = res.payload as { editRule: FaultDetectRule };
            const nextValue = normalizeFaultDetectFormValue(editRule);
            persistedValueRef.current = nextValue;
            form.setFieldsValue(nextValue);
            setRuleDrafts(nextValue.rules);
            setProtocolOverrides({});
            setFormRevision(prev => prev + 1);
        })
    }, [editRuleId])

    React.useEffect(() => {
        if (props.op !== 'create' || !props.serviceContext) return;
        form.setFieldsValue({
            targetService: {
                ...normalizeTargetService(form.getFieldValue('targetService')),
                namespace: props.serviceContext.namespace,
                service: props.serviceContext.service,
            },
        });
        setFormRevision((prev) => prev + 1);
    }, [form, props.op, props.serviceContext?.namespace, props.serviceContext?.service]);

    // 表单提交
    const onSubmit: FormProps['onSubmit'] = async (e) => {
        if (e.validateResult !== true || editorState.loading) {
            return;
        }

        setEditorState(prev => ({ ...prev, loading: true }));
        try {
            const fields = e.fields as FaultDetectDO;
            const draft = normalizeFaultDetectFormValue({
                ...(editRule || {}),
                ...fields,
                metadata: fields.metadata || metadata,
                targetService: normalizeTargetService(fields.targetService),
                rules: effectiveRuleDrafts,
            });
            const errors = validateFaultDetectDraft(draft);
            if (errors.length > 0) {
                openErrNotification('校验失败', errors[0].message);
                return;
            }

            const ruleData: FaultDetectRule = {
                ...buildFaultDetectSubmitPayload(draft),
                namespace: (draft as FaultDetectRule & { namespace?: string }).namespace || ruleNamespace,
                id: viewRule?.id || '',
                editable: true,
                deleteable: true
            };

            let res;
            if (props.op === 'create') {
                res = await dispatch(saveFaultDetects({ param: { ...ruleData } }));
            } else {
                res = await dispatch(updateFaultDetects({ param: { ...ruleData } }));
            }
            if (res.meta.requestStatus === 'rejected') {
                openErrNotification('请求错误', res?.payload as string);
            } else {
                openInfoNotification('请求成功', props.op !== 'create' ? '修改主动探测规则成功' : '创建主动探测规则成功');
                const nextValue = normalizeFaultDetectFormValue(ruleData);
                persistedValueRef.current = nextValue;
                form.setFieldsValue(nextValue);
                setRuleDrafts(nextValue.rules);
                setProtocolOverrides({});
                if (props.op === 'create') {
                    props.refresh(false); // 刷新列表
                } else {
                    setEditorState(prev => ({ ...prev, editable: false }));
                }
            }
        } catch (error) {
            console.error('提交失败:', error);
            openErrNotification('操作失败', '提交故障检测规则失败，请检查输入或稍后重试');
        } finally {
            setEditorState(prev => ({ ...prev, loading: false }));
        }
    };

    const formValues = React.useMemo(() => (
        typeof form.getFieldsValue === 'function'
            ? form.getFieldsValue(true) as Partial<FaultDetectDO>
            : {}
    ), [editRule, form, formRevision]);
    const metadata = (formValues.metadata || (editRule?.metadata as Record<string, string>) || {}) as Record<string, string>;
    const targetService = normalizeTargetService(formValues.targetService || editRule?.targetService);
    const effectiveRuleDrafts = ruleDrafts.map((rule, index) => ({
        ...rule,
        protocol: protocolOverrides[index] || rule.protocol,
    }));
    const subRules = ensureFaultDetectRules({ rules: effectiveRuleDrafts });
    const currentDraft = React.useMemo(() => normalizeFaultDetectFormValue({
        ...(editRule || {}),
        ...(formValues || {}),
        name: formValues.name || editRule?.name || '',
        description: formValues.description || editRule?.description || '',
        priority: formValues.priority ?? editRule?.priority ?? 0,
        metadata,
        targetService,
        rules: subRules,
    }), [editRule, formValues, metadata, subRules, targetService]);
    const syncRuleDrafts = (nextRules: FaultDetectSubRule[], writeForm = true) => {
        const normalizedRules = normalizeFaultDetectRulesDraft(nextRules);
        if (writeForm) {
            syncingRuleDraftsRef.current = true;
            form.setFieldsValue({ rules: normalizedRules });
            window.setTimeout(() => {
                syncingRuleDraftsRef.current = false;
            }, 0);
        }
        setRuleDrafts(normalizedRules);
        setFormRevision(prev => prev + 1);
        return normalizedRules;
    };

    const currentFormRules = () => ensureFaultDetectRules({ rules: ruleDrafts });

    const addDetectRule = () => {
        const nextRules = syncRuleDrafts([...currentFormRules(), defaultFaultDetectSubRule()]);
        setCollapsedRuleIndexes(prev => {
            const next = new Set(prev);
            next.delete(nextRules.length - 1);
            return next;
        });
    };

    const removeDetectRule = (index: number) => {
        const rules = currentFormRules();
        if (rules.length <= 1) {
            return;
        }
        syncRuleDrafts(rules.filter((_, idx) => idx !== index));
    };

    const toggleRuleCollapsed = (index: number) => {
        setCollapsedRuleIndexes(prev => {
            const next = new Set(prev);
            if (next.has(index)) {
                next.delete(index);
            } else {
                next.add(index);
            }
            return next;
        });
    };

    const patchRule = (index: number, patch: Partial<FaultDetectSubRule>) => {
        const rules = currentFormRules();
        syncRuleDrafts(rules.map((rule, idx) => idx === index ? { ...rule, ...patch } : rule), false);
    };

    const updateHttpHeaders = (ruleIndex: number, headers: Label[]) => {
        const rules = currentFormRules();
        syncRuleDrafts(rules.map((rule, idx) => {
            if (idx !== ruleIndex) {
                return rule;
            }
            return {
                ...rule,
                httpConfig: {
                    method: rule.httpConfig?.method || HTTPMethod.GET,
                    url: rule.httpConfig?.url || '/healthz',
                    body: rule.httpConfig?.body || '',
                    ...rule.httpConfig,
                    headers,
                },
            };
        }));
    };

    const addHttpHeader = (ruleIndex: number, headers?: Label[]) => {
        updateHttpHeaders(ruleIndex, [...(headers || []), { key: '', value: '' }]);
    };

    const updateHttpHeader = (ruleIndex: number, headerIndex: number, field: keyof Label, value: string, headers?: Label[]) => {
        updateHttpHeaders(ruleIndex, (headers || []).map((item, index) => (
            index === headerIndex ? { ...item, [field]: value } : item
        )));
    };

    const removeHttpHeader = (ruleIndex: number, headerIndex: number, headers?: Label[]) => {
        updateHttpHeaders(ruleIndex, (headers || []).filter((_, index) => index !== headerIndex));
    };

    const readonlyField = (label: string, value: React.ReactNode, full = false) => (
        <div className={full ? shared.full : undefined}>
            <div className={shared.editLabel}>{label}</div>
            <Text>{value || '-'}</Text>
        </div>
    );

    const renderHttpHeaders = (index: number, headers?: Label[]) => {
        const rows = Array.isArray(headers) ? headers : [];
        return (
            <div className={styles.httpHeadersBlock}>
                <div className={styles.httpHeadersTitleRow}>
                    <span className={styles.httpHeadersBadge}>H</span>
                    <span className={styles.httpHeadersTitle}>Headers</span>
                    <span className={styles.httpHeadersHint}>随探测请求发送的请求头</span>
                </div>
                <div className={styles.httpHeadersScroller} role="region" aria-label={`第 ${index + 1} 条探测规则的请求头`}>
                <div className={styles.httpHeadersGrid} role="table">
                    <div className={styles.httpHeadersRow} role="row">
                        <div className={`${styles.httpHeadersHead} ${styles.httpHeadersFirstHead}`} role="columnheader">键</div>
                        <div className={styles.httpHeadersHead} role="columnheader">值</div>
                        <div className={`${styles.httpHeadersHead} ${styles.httpHeadersActionHead}`} role="columnheader">操作</div>
                    </div>
                    {rows.length === 0 ? (
                        <div className={styles.httpHeadersEmpty} role="cell">暂无标签</div>
                    ) : rows.map((item, headerIndex) => (
                        <div className={styles.httpHeadersRow} key={headerIndex} role="row">
                            <div className={`${styles.httpHeadersCell} ${styles.httpHeadersFirstCell}`} role="cell">
                                {editorState.editable ? (
                                    <Input
                                        aria-label={`第 ${index + 1} 条探测规则第 ${headerIndex + 1} 个请求头的键`}
                                        className={styles.monoInput}
                                        value={item.key}
                                        placeholder="x-seed"
                                        onChange={(value) => updateHttpHeader(index, headerIndex, 'key', value, rows)}
                                    />
                                ) : (
                                    <span className={styles.headerReadonlyText} title={displayText(item.key)}>{displayText(item.key)}</span>
                                )}
                            </div>
                            <div className={styles.httpHeadersCell} role="cell">
                                {editorState.editable ? (
                                    <Input
                                        aria-label={`第 ${index + 1} 条探测规则第 ${headerIndex + 1} 个请求头的值`}
                                        className={styles.monoInput}
                                        value={item.value}
                                        placeholder="true"
                                        onChange={(value) => updateHttpHeader(index, headerIndex, 'value', value, rows)}
                                    />
                                ) : (
                                    <span className={styles.headerReadonlyText} title={displayText(item.value)}>{displayText(item.value)}</span>
                                )}
                            </div>
                            <div className={`${styles.httpHeadersCell} ${styles.httpHeadersActionCell}`} role="cell">
                                {editorState.editable ? (
                                    <Popup trigger="hover" content="删除标签">
                                        <Button aria-label={`删除第 ${index + 1} 条探测规则第 ${headerIndex + 1} 个请求头`} shape="square" variant="outline" onClick={() => removeHttpHeader(index, headerIndex, rows)}>
                                            <DeleteIcon />
                                        </Button>
                                    </Popup>
                                ) : (
                                    <span className={styles.headerReadonlyText}>-</span>
                                )}
                            </div>
                        </div>
                    ))}
                </div>
                </div>
                {editorState.editable && (
                    <div className={styles.httpHeadersFooter}>
                        <Button aria-label={`第 ${index + 1} 条探测规则添加标签`} theme="primary" variant="text" icon={<AddIcon />} onClick={() => addHttpHeader(index, rows)}>
                            添加标签
                        </Button>
                        <span>{rows.length} 个标签</span>
                    </div>
                )}
            </div>
        );
    };

    const renderProtocolPayload = (index: number, rule: FaultDetectSubRule) => {
        const protocol = rule.protocol || FaultDetectProtocol.HTTP;
        switch (protocol) {
                    case FaultDetectProtocol.TCP:
                    case FaultDetectProtocol.UDP: {
                        const configKey = protocol === FaultDetectProtocol.TCP ? 'tcpConfig' : 'udpConfig';
                        const config = protocol === FaultDetectProtocol.TCP ? rule.tcpConfig : rule.udpConfig;
                        const hint = protocol === FaultDetectProtocol.TCP
                            ? '建立连接后发送请求内容，再读取响应内容比对。'
                            : '发送 UDP 数据报后读取返回数据报内容比对。';
                        return editorState.editable ? (
                            <div className={shared.kv2}>
                                <div className={shared.full}>
                                    <div className={shared.editLabel}>匹配方式</div>
                                    <FormItem name={['rules', index, configKey, 'match']} style={{ marginBottom: 0 }}>
                                        <Select aria-label={`第 ${index + 1} 条探测规则报文匹配方式`} options={PayloadMatchOptions} />
                                    </FormItem>
                                </div>
                                <div>
                                    <div className={shared.editLabel}>发送内容</div>
                                    <FormItem name={['rules', index, configKey, 'send']} style={{ marginBottom: 0 }}>
                                        <Textarea aria-label={`第 ${index + 1} 条探测规则发送报文`} className={styles.monoTextarea} placeholder="PING\\n" />
                                    </FormItem>
                                </div>
                                <div>
                                    <div className={shared.editLabel}>接收内容</div>
                                    <FormItem
                                        name={['rules', index, configKey, 'receive']}
                                        style={{ marginBottom: 0 }}
                                    >
                                        <Textarea aria-label={`第 ${index + 1} 条探测规则响应报文`} className={styles.monoTextarea} placeholder="PONG" />
                                    </FormItem>
                                </div>
                                <div className={shared.full}>
                                    <div className={styles.strategyNote}>{hint}</div>
                                </div>
                            </div>
                        ) : (
                            <div className={shared.kv2}>
                                {readonlyField('匹配方式', config?.match === 'CONTAINS' ? '包含匹配' : config?.match === 'REGEX' ? '正则匹配' : '完全匹配')}
                                {readonlyField('发送内容', config?.send || '-')}
                                {readonlyField('接收内容', receiveToText(config?.receive) || '-')}
                            </div>
                        );
                    }
                    case FaultDetectProtocol.HTTP:
                    default:
                        return editorState.editable ? (
                            <div className={shared.kv2}>
                                <div>
                                    <div className={shared.editLabel}>方法</div>
                                    <FormItem name={['rules', index, 'httpConfig', 'method']} style={{ marginBottom: 0 }}>
                                        <Select aria-label={`第 ${index + 1} 条探测规则 HTTP 方法`} filterable creatable options={HTTPMethodOption} />
                                    </FormItem>
                                </div>
                                <div>
                                    <div className={shared.editLabel}>URL</div>
                                    <FormItem name={['rules', index, 'httpConfig', 'url']} style={{ marginBottom: 0 }}>
                                        <Input aria-label={`第 ${index + 1} 条探测规则 HTTP 路径`} className={styles.monoTextarea} placeholder="/healthz" />
                                    </FormItem>
                                </div>
                                <div className={shared.full}>
                                    {renderHttpHeaders(index, rule.httpConfig?.headers)}
                                </div>
                            </div>
                        ) : (
                            <div className={shared.kv2}>
                                {readonlyField('方法', rule.httpConfig?.method)}
                                {readonlyField('URL', rule.httpConfig?.url)}
                                <div className={shared.full}>
                                    {renderHttpHeaders(index, rule.httpConfig?.headers)}
                                </div>
                            </div>
                        );
        }
    };

    const renderPortStrategy = (rule: FaultDetectSubRule, index: number) => {
        const customPort = isCustomProbePort(rule);
        return (
            <div className={shared.step} data-step="2">
                <div className={shared.stepTitle}>端口策略<span className={shared.stepHint}>默认按实例注册的协议端口探测，只有特殊探测端口才需要覆盖。</span></div>
                <div className={shared.stepContent}>
                    {editorState.editable ? (
                        <div className={shared.kv2}>
                            <div>
                                <div className={shared.editLabel}>端口来源</div>
                                <RadioGroup
                                    aria-label={`第 ${index + 1} 条探测规则端口来源`}
                                    theme="button"
                                    variant="primary-filled"
                                    value={customPort ? 'CUSTOM' : 'INSTANCE'}
                                    onChange={(value) => {
                                        patchRule(index, { port: value === 'CUSTOM' ? (rule.port || 8080) : 0 });
                                    }}
                                >
                                    <Radio.Button value="INSTANCE">实例协议端口</Radio.Button>
                                    <Radio.Button value="CUSTOM">指定探测端口</Radio.Button>
                                </RadioGroup>
                            </div>
                            {customPort ? (
                                <div>
                                    <div className={shared.editLabel}>探测端口</div>
                                    <FormItem name={['rules', index, 'port']} style={{ marginBottom: 0 }}>
                                        <InputNumber aria-label={`第 ${index + 1} 条探测规则端口`} theme="normal" min={1} max={65535} placeholder="8080" />
                                    </FormItem>
                                </div>
                            ) : (
                                <div className={shared.full}>
                                    <div className={styles.strategyNote}>当前策略：实例协议端口。运行时会选择目标实例中与探测协议匹配的注册端口。</div>
                                </div>
                            )}
                        </div>
                    ) : (
                        <div className={styles.strategyNote}>当前策略：{describeProbePort(rule)}</div>
                    )}
                </div>
            </div>
        );
    };

    const renderDetectRule = (rule: FaultDetectSubRule, index: number) => {
        const collapsed = collapsedRuleIndexes.has(index);
        const changeProtocol = (protocol: FaultDetectProtocol) => {
            setProtocolOverrides(prev => ({ ...prev, [index]: protocol }));
            if (protocol === FaultDetectProtocol.HTTP) {
                patchRule(index, { protocol, httpConfig: { method: rule.httpConfig?.method || HTTPMethod.GET, url: rule.httpConfig?.url || '/healthz', headers: rule.httpConfig?.headers || [], body: rule.httpConfig?.body || '' } });
            } else if (protocol === FaultDetectProtocol.TCP) {
                patchRule(index, { protocol, tcpConfig: { send: rule.tcpConfig?.send || '', match: rule.tcpConfig?.match || 'EXACT', receive: rule.tcpConfig?.receive || [] } });
            } else if (protocol === FaultDetectProtocol.UDP) {
                patchRule(index, { protocol, udpConfig: { send: rule.udpConfig?.send || '', match: rule.udpConfig?.match || 'EXACT', receive: rule.udpConfig?.receive || [] } });
            }
        };
        const protocolOptions = [FaultDetectProtocol.HTTP, FaultDetectProtocol.TCP, FaultDetectProtocol.UDP];

        return (
            <div className={`${shared.policy} ${collapsed ? shared.policyCollapsed : ''}`} key={index}>
                <div className={shared.policyHead} onClick={() => toggleRuleCollapsed(index)}>
                    <div className={shared.policyHeadMain}>
                        <span className={shared.caret}><ChevronRightIcon /></span>
                        <div>
                            <div className={shared.policyIndex}>探测规则 [{index + 1}]</div>
                            <div className={shared.policySummary}>{describeProbeSummary(rule)}</div>
                        </div>
                    </div>
                    <div className={shared.policyHeadActions} onClick={(event) => event.stopPropagation()}>
                        <Tag theme={rule.disable ? 'default' : 'success'} variant="light-outline">{rule.disable ? '禁用' : '启用'}</Tag>
                        <Tag variant="light">{describeProbePort(rule)}</Tag>
                        {editorState.editable && (
                            <Popup trigger="hover" content="删除探测规则">
                                <Button
                                    shape="circle"
                                    variant="text"
                                    disabled={subRules.length <= 1}
                                    onClick={() => removeDetectRule(index)}
                                >
                                    <CloseIcon />
                                </Button>
                            </Popup>
                        )}
                    </div>
                </div>
                <div className={shared.policyBody}>
                    <div className={shared.step} data-step="1">
                        <div className={shared.stepTitle}>基础调度</div>
                        <div className={`${shared.stepContent} ${shared.kv2}`}>
                            <div>
                                <div className={shared.editLabel}>探测协议</div>
                                {editorState.editable ? (
                                    <div className={styles.segmentedControl}>
                                        {protocolOptions.map(protocol => {
                                            const active = (rule.protocol || FaultDetectProtocol.HTTP) === protocol;
                                            return (
                                                <Button
                                                    key={protocol}
                                                    variant="text"
                                                    className={active ? styles.segmentedButtonActive : styles.segmentedButton}
                                                    onClick={() => changeProtocol(protocol)}
                                                >
                                                    {protocol}
                                                </Button>
                                            );
                                        })}
                                    </div>
                                ) : (
                                    <Text>{displayText(rule.protocol)}</Text>
                                )}
                            </div>
                            <div>
                                <div className={shared.editLabel}>状态</div>
                                <FormItem name={['rules', index, 'disable']} initialData={false} style={{ marginBottom: 0 }}>
                                    {editorState.editable ? (
                                        <RadioGroup aria-label={`第 ${index + 1} 条探测规则启用状态`} theme="button" variant="primary-filled">
                                            <Radio.Button value={false}>启用</Radio.Button>
                                            <Radio.Button value={true}>禁用</Radio.Button>
                                        </RadioGroup>
                                    ) : (
                                        <Text>{rule.disable ? '禁用' : '启用'}</Text>
                                    )}
                                </FormItem>
                            </div>
                            <div>
                                <div className={shared.editLabel}>间隔</div>
                                <FormItem name={['rules', index, 'interval']} style={{ marginBottom: 0 }}>
                                    {editorState.editable ? (
                                        <Space><InputNumber aria-label={`第 ${index + 1} 条探测规则探测间隔`} theme="normal" min={1} placeholder="30" /><span>秒</span></Space>
                                    ) : (
                                        <Text>{rule.interval || 30} 秒</Text>
                                    )}
                                </FormItem>
                            </div>
                            <div>
                                <div className={shared.editLabel}>超时</div>
                                <FormItem name={['rules', index, 'timeout']} style={{ marginBottom: 0 }}>
                                    {editorState.editable ? (
                                        <Space><InputNumber aria-label={`第 ${index + 1} 条探测规则超时时间`} theme="normal" min={1} placeholder="60" /><span>秒</span></Space>
                                    ) : (
                                        <Text>{rule.timeout || 60} 秒</Text>
                                    )}
                                </FormItem>
                            </div>
                        </div>
                    </div>
                    {renderPortStrategy(rule, index)}
                    <div className={shared.step} data-step="3">
                        <div className={shared.stepTitle}>{rule.protocol === FaultDetectProtocol.HTTP ? 'HTTP 协议载荷' : '报文匹配'}</div>
                        <div className={shared.stepContent}>
                            {renderProtocolPayload(index, rule)}
                        </div>
                    </div>
                </div>
            </div>
        );
    };

    const ruleBaseInfo = (
        <CollapsibleSection
            collapsed={basicInfoCollapsed}
            onCollapsedChange={setBasicInfoCollapsed}
            header="基础信息"
            summary={`${currentDraft.name || '未命名规则'} · 优先级 ${currentDraft.priority ?? 0}`}
        >
                <div className={shared.infoGrid}>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>规则名称</div>
                        <FormItem
                            name="name"
                            showErrorMessage={editorState.editable}
                            rules={[
                                { required: true, message: '请输入规则名称' },
                                { pattern: /^[a-z][a-z0-9-]*$/, message: '规则名称必须为 kebab-case' },
                                { max: 64, message: '最长64个字符' }
                            ]}
                        >
                            {props.op === 'create'
                                ? <Input aria-label="探测规则名称" placeholder="最长64个字符" />
                                : <Text>{editRule?.name}</Text>}
                        </FormItem>
                    </div>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>优先级</div>
                        <FormItem name="priority">
                            {editorState.editable
                                ? <InputNumber aria-label="探测规则优先级" theme="normal" min={0} placeholder="0" />
                                : <Text>{displayText(currentDraft.priority)}</Text>}
                        </FormItem>
                    </div>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>描述</div>
                        <FormItem name="description">
                            {editorState.editable
                                ? <Input aria-label="探测规则描述" placeholder="补充规则说明" />
                                : <Text>{editRule?.description || '-'}</Text>}
                        </FormItem>
                    </div>
                    <div className={`${shared.field} ${shared.full}`}>
                        <div className={shared.fieldLabel}>规则标签</div>
                        <RuleLabelField
                            metadata={metadata}
                            editable={editorState.editable}
                            onChange={(next) => form.setFieldsValue({ metadata: next })}
                        />
                    </div>
                </div>
        </CollapsibleSection>
    );

    const fixedTarget = props.op === 'create' && Boolean(props.serviceContext);

    const targetInfo = (
        <div className={shared.section}>
            <div className={shared.sectionHeader}>
                <span>被探测对象</span>
                <span className={shared.countTag}>{displayText(targetService.namespace)}/{displayText(targetService.service)}</span>
            </div>
            <div className={shared.sectionBody}>
                <div className={shared.infoGrid}>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>被探测命名空间</div>
                        <FormItem
                            name={['targetService', 'namespace']}
                            rules={[{ required: true, message: '请选择命名空间' }]}
                        >
                            {editorState.editable && !fixedTarget ? (
                                <Select
                                    aria-label="被探测命名空间"
                                    filterable
                                    creatable
                                    options={namespaceDatas.map(ns => ({ label: ns.name, value: ns.name }))}
                                    onChange={() => form.setFieldsValue({ targetService: { ...normalizeTargetService(form.getFieldValue('targetService')), service: '' } })}
                                />
                            ) : (
                                <Text>{targetService.namespace === '*' ? '全部命名空间' : targetService.namespace}</Text>
                            )}
                        </FormItem>
                    </div>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>被探测服务</div>
                        <FormItem shouldUpdate={(prev, next) => prev.targetService?.namespace !== next.targetService?.namespace}>
                            {({ getFieldValue }) => {
                                const selectNs = getFieldValue(['targetService', 'namespace']);
                                return (
                                    <FormItem
                                        name={['targetService', 'service']}
                                        rules={[{ required: true, message: '请选择服务' }]}
                                    >
                                        {editorState.editable && !fixedTarget ? (
                                            <Select
                                                aria-label="被探测服务"
                                                placeholder="请选择服务"
                                                filterable
                                                creatable
                                                options={serviceDatas.filter(opt => selectNs === '*' || opt.namespace === selectNs).map(service => ({
                                                    label: service.name,
                                                    value: service.name
                                                }))}
                                            />
                                        ) : (
                                            <Text>{targetService.service === '*' ? '全部服务' : targetService.service}</Text>
                                        )}
                                    </FormItem>
                                )
                            }}
                        </FormItem>
                    </div>
                </div>
            </div>
        </div>
    );

    const renderEditRule = (
        <Form
            form={form}
            onSubmit={onSubmit}
            onValuesChange={() => {
                if (syncingRuleDraftsRef.current) {
                    return;
                }
                const nextRules = form.getFieldValue('rules') as FaultDetectSubRule[] | undefined;
                if (Array.isArray(nextRules)) {
                    setRuleDrafts(normalizeFaultDetectRulesDraft(nextRules));
                }
                setFormRevision(prev => prev + 1);
            }}
            labelWidth="120px"
            layout="vertical"
        >
            {ruleBaseInfo}
            {targetInfo}

            <div className={shared.section}>
                <div className={shared.sectionHeader}>
                    <span>探测规则</span>
                    <span className={shared.countTag}>{subRules.length} 条</span>
                </div>
                <div className={shared.sectionBody}>
                    <div className={shared.ruleList}>
                        {subRules.map((rule, index) => renderDetectRule(rule, index))}
                    </div>
                    {editorState.editable && (
                        <Button className={shared.addRuleButton} style={{ marginTop: 12 }} variant="dashed" icon={<AddIcon />} onClick={addDetectRule}>
                            添加探测子规则
                        </Button>
                    )}
                </div>
            </div>
        </Form>
    )

    const renderStickyTool = (
        <>
            {viewRule?.editable !== false && editRule?.editable !== false && (
                <div style={{ marginTop: 20 }}>
                    <StickyTool
                        style={{ zIndex: 1000 }}
                        placement='right-bottom'
                        offset={[-10, 200]}
                    >
                        <StickyItem
                            label=""
                            icon={!editorState.editable ?
                                <RuleStickyAction label="编辑" icon={<Edit1Icon />} onClick={() => {
                                    setEditorState(prev => ({ ...prev, editable: true }));
                                }} />
                                :
                                <RuleStickyAction label="保存" icon={<SaveIcon />} loading={editorState.loading} disabled={editorState.loading} onClick={() => {
                                    form.submit();
                                }} />
                            }
                        />
                        {(editorState.editable) && (
                            <StickyItem label="" icon={
                                <RuleStickyAction label="撤销" icon={<RollbackIcon />} onClick={() => {
                                    if (props.op === 'create') {
                                        props.refresh(true);
                                    } else {
                                        const baseline = persistedValueRef.current;
                                        form.setFieldsValue(baseline);
                                        setRuleDrafts(baseline.rules);
                                        setProtocolOverrides({});
                                        setFormRevision(prev => prev + 1);
                                        setEditorState(prev => ({ ...prev, editable: false }));
                                    }
                                }} />}
                            />
                        )}
                        {(!editorState.editable) && (
                            <StickyItem label="" icon={
                                <RuleStickyAction label="发布" icon={<RocketIcon />} onClick={() => {
                                    setEditorState(prev => ({ ...prev, publishView: true }));
                                }} />}
                            />
                        )}
                    </StickyTool>
                </div>
            )}
        </>
    )

    return (
        <div className={styles.editorBody}>
            <div className={styles.editorShell}>
                <div className={styles.formPane}>
                    {renderEditRule}
                </div>
            </div>
            {editorState.publishView && (
                <PublishForm
                    ruleId={viewRule?.id || ''}
                    ruleName={viewRule?.name || ''}
                    resource={PolicySourceType.FaultDetectRules}
                    visible={editorState.publishView}
                    close={() => {
                        setEditorState(prev => ({ ...prev, publishView: false }));
                    }}
                />
            )}
            {renderStickyTool}
        </div>
    );
};

export default React.memo(FaultDetectEditor);
