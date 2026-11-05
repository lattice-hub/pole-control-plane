import React from "react";
import { Form, Input, Button, Select, Switch, Dialog, InputNumber, FormProps, Tag, Popup, StickyTool, RadioGroup, Radio, InputAdornment, Textarea, Space } from 'components/Fluent';
import { AddIcon, ChevronRightIcon, CloseIcon, Edit1Icon, SaveIcon, RocketIcon, RollbackIcon, RemoveIcon } from 'components/Fluent/icons';

import Text from "components/Text";
import RuleLabelField from "../shared/RuleLabelField";
import CollapsibleSection from "../shared/CollapsibleSection";
import ServiceScopeSection from "../shared/ServiceScopeSection";
import { GovernanceServiceContext } from "../shared/serviceContext";
import { useRuleNamespace } from "../shared/ruleNamespace";
import shared from "../shared/governance.module.less";
import { useAppDispatch, useAppSelector } from 'modules/store';
import { API, HTTPMethodOption, InterfaceProtocolOption, Label, MatchType, MatchTypeMap, MatchTypeOption, MatchValueType, Op } from "services/types";
import { ServiceView } from "services/service";
import { NamespaceView } from "services/namespace";
import styles from './CircuitBreakerEditor.module.less';
import { openErrNotification, openInfoNotification } from "utils/notifition";
import PublishForm from "../RuleRelease/PublishForm";
import RuleStickyAction from "../RuleRelease/RuleStickyAction";
import { PolicySourceType } from "services/auth_policy";
import { BreakLevelMap, BreakLevelType, CircuitBreakerRule, ErrorConditionMap, ErrorConditionOptions, ErrorConditionType, TriggerType, TriggerTypeMap, TriggerTypeOptions } from "services/circuitbreaker";
import { listOneCircuitBreaker, resetCircuitBreaker, saveCircuitBreakers, selectCircuitBreaker, updateCircuitBreakers } from "modules/governance/circuitbreaker";
import { cleanNamespacePage, listAllNamespaces, selectNamespace } from "modules/namespace";
import { cleanServicePage, listAllServices, selectService } from "modules/discovery/service";
import { cloneDeep } from "lodash";
import {
    buildCircuitBreakerSubmitPayload,
    CircuitBreakerAPI,
    CircuitBreakerDraftLike,
    CircuitBreakerErrorCondition,
    CircuitBreakerStrategyDraft,
    CircuitBreakerSubRuleDraft,
    CircuitBreakerTriggerCondition,
    createCircuitBreakerDraftFromRule,
    defaultCircuitBreakerStrategy,
    defaultCircuitBreakerSubRule,
    describeStrategySummary,
    describeSubRuleSummary,
    validateCircuitBreakerDraft,
} from "./circuitBreakerEditorUtils";

const { StickyItem } = StickyTool;

interface CircuitBreakerDO {
    id?: string
    name: string
    level: string
    description: string
    priority: number
    ruleMatcher: {
        source: {
            service: string
            namespace: string
        }
        destination: {
            service: string
            namespace: string
            method: {
                type: string
                value: string
            }
        }
    }
    subrules: CircuitBreakerSubRuleDraft[]
    metadata: Label[]
}

const defaultCircuitBreakerRule = (): CircuitBreakerDO => ({
    name: '',
    level: BreakLevelType.Method,
    priority: 0,
    description: '',
    metadata: [],
    ruleMatcher: {
        source: {
            service: '',
            namespace: '*'
        },
        destination: {
            service: '',
            namespace: '*',
            method: {
                type: MatchType.EXACT,
                value: ''
            }
        }
    },
    subrules: [defaultCircuitBreakerSubRule(1)],
})

interface ICircuitBreakerEditorProps {
    op: Op;
    refresh: (close: boolean) => void;
    serviceContext?: GovernanceServiceContext;
}

const toLabels = (metadata?: CircuitBreakerDraftLike['metadata']): Label[] => {
    if (Array.isArray(metadata)) {
        return metadata;
    }
    return Object.entries(metadata || {}).map(([key, value]) => ({ key, value }));
}

const CircuitBreakerEditor: React.FC<ICircuitBreakerEditorProps> = ({ op, refresh, serviceContext }) => {
    const ruleNamespace = useRuleNamespace();
    const [form] = Form.useForm();
    const dispatch = useAppDispatch();

    const namespaceState = useAppSelector(selectNamespace);
    const { datas: namespaceDatas } = namespaceState;

    const serviceState = useAppSelector(selectService);
    const { datas: serviceDatas } = serviceState;

    const breakerState = useAppSelector(selectCircuitBreaker);
    const { editRule, viewRule } = breakerState;

    const [editorState, setEditorState] = React.useState<{
        visible: boolean
        headerVisible: boolean
        headerSubRuleIndex?: number
        editable?: boolean
        model: Op;
        publishView: boolean;
    }>({ model: 'view', publishView: false, visible: false, headerVisible: false, editable: op === 'create' || false, });

    const [breakerRule, setBreakerRule] = React.useState<CircuitBreakerDO>(defaultCircuitBreakerRule());
    const [basicInfoCollapsed, setBasicInfoCollapsed] = React.useState(false);
    const [serviceCollapsed, setServiceCollapsed] = React.useState(false);
    const [collapsedSubRuleIndexes, setCollapsedSubRuleIndexes] = React.useState<Set<number>>(() => new Set());
    const [collapsedStrategyKeys, setCollapsedStrategyKeys] = React.useState<Set<string>>(() => new Set());
    const canEditLevel = editorState.editable && op === 'create';

    const updateRule = (updater: (draft: CircuitBreakerDO) => void) => {
        const next = cloneDeep(breakerRule);
        updater(next);
        setBreakerRule(next);
    };

    const toggleSubRuleCollapsed = (idx: number) => {
        setCollapsedSubRuleIndexes(prev => {
            const next = new Set(prev);
            next.has(idx) ? next.delete(idx) : next.add(idx);
            return next;
        });
    };

    const toggleStrategyCollapsed = (subRuleIdx: number, strategyIdx: number) => {
        const key = `${subRuleIdx}-${strategyIdx}`;
        setCollapsedStrategyKeys(prev => {
            const next = new Set(prev);
            next.has(key) ? next.delete(key) : next.add(key);
            return next;
        });
    };

    const renderReadonlyValue = (value: React.ReactNode) => (
        <div className={styles.readonlyValue}>{value}</div>
    );

    const renderReadonlySwitch = (enabled?: boolean) => (
        <Tag theme={enabled ? 'success' : 'default'} variant="light">
            {enabled ? '开启' : '关闭'}
        </Tag>
    );

    const resetCurRule = (editRule: CircuitBreakerRule) => {
        if (!editRule) {
            return;
        }
        const normalized = createCircuitBreakerDraftFromRule(cloneDeep(editRule) as CircuitBreakerDraftLike);
        const sourceNamespace = normalized?.ruleMatcher?.source?.namespace || '*';
        let destinationNamespace = normalized?.ruleMatcher?.destination?.namespace || '*';
        let destinationService = normalized?.ruleMatcher?.destination?.service || '';
        const knownNamespaces = new Set((namespaceDatas || []).map((item: NamespaceView) => item.name));
        const swappedByOptions = knownNamespaces.has(destinationService)
            && serviceDatas.some((item: ServiceView) => item.namespace === destinationService && item.name === destinationNamespace);
        const swappedByNameShape = destinationService.includes('governance') && !destinationNamespace.includes('governance');
        if ((destinationService === sourceNamespace && destinationNamespace !== sourceNamespace) || swappedByOptions || swappedByNameShape) {
            [destinationNamespace, destinationService] = [destinationService, destinationNamespace];
        }
        setBreakerRule({
            id: normalized.id,
            name: normalized.name || '',
            description: normalized.description || '',
            priority: normalized.priority || 0,
            level: normalized.level || BreakLevelType.Method,
            ruleMatcher: {
                source: {
                    service: normalized.ruleMatcher?.source?.service || '',
                    namespace: normalized.ruleMatcher?.source?.namespace || '*'
                },
                destination: {
                    service: destinationService,
                    namespace: destinationNamespace,
                    method: {
                        type: normalized.ruleMatcher?.destination?.method?.type || MatchType.EXACT,
                        value: normalized.ruleMatcher?.destination?.method?.value || ''
                    }
                }
            },
            subrules: normalized.subrules?.length ? normalized.subrules : [defaultCircuitBreakerSubRule(1)],
            metadata: toLabels(normalized.metadata),
        });
        setCollapsedSubRuleIndexes(new Set());
        setCollapsedStrategyKeys(new Set());
    }

    React.useEffect(() => {
        setBasicInfoCollapsed(false);
        if (editRule) {
            if (editRule.id !== '') {
                setEditorState(pre => ({ ...pre, visible: false, editable: false }));
                dispatch(listOneCircuitBreaker({ id: editRule.id || '' })).then(res => {
                    if (res.meta.requestStatus === 'rejected') {
                        openErrNotification('请求失败', `获取熔断规则失败: ${res?.payload as string}`);
                    }
                    const { viewRule } = res.payload as { viewRule: CircuitBreakerRule };
                    resetCurRule(viewRule);
                })
            } else {
                resetCurRule(editRule);
            }
        }
    }, [editRule])

    React.useEffect(() => {
        if (op !== 'create' || !serviceContext) return;
        setBreakerRule((prev) => {
            const next = cloneDeep(prev);
            const endpoint = { namespace: serviceContext.namespace, service: serviceContext.service };
            if (serviceContext.role === 'caller') {
                next.ruleMatcher.source = endpoint;
            } else {
                next.ruleMatcher.destination = {
                    ...next.ruleMatcher.destination,
                    ...endpoint,
                };
            }
            return next;
        });
    }, [op, serviceContext?.namespace, serviceContext?.service, serviceContext?.role]);

    React.useEffect(() => {
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
            dispatch(resetCircuitBreaker());
        }
    }, []);

    const metadataRecord = React.useMemo(
        () => (breakerRule.metadata || []).reduce<Record<string, string>>((acc, cur) => {
            if (cur.key) acc[cur.key] = cur.value;
            return acc;
        }, {}),
        [breakerRule.metadata],
    );

    const submitPayload = React.useMemo(() => {
        const payload = buildCircuitBreakerSubmitPayload({ ...breakerRule, metadata: metadataRecord });
        return { ...payload, namespace: (payload as { namespace?: string }).namespace || ruleNamespace };
    }, [breakerRule, metadataRecord, ruleNamespace]);
    const onSubmit: FormProps['onSubmit'] = async () => {
        const errors = validateCircuitBreakerDraft({ ...breakerRule, metadata: metadataRecord });
        if (errors.length > 0) {
            openErrNotification('保存失败', errors[0].message);
            return;
        }

        let res;
        if (op !== 'create') {
            res = await dispatch(updateCircuitBreakers({ param: submitPayload as any }));
        } else {
            res = await dispatch(saveCircuitBreakers({ param: submitPayload as any }));
        }

        if (res.meta.requestStatus !== 'fulfilled') {
            openErrNotification('请求错误', res?.payload as string);
        } else {
            openInfoNotification('请求成功', op !== 'create' ? '修改熔断规则成功' : '创建熔断规则成功');
            if (op !== 'create') {
                setEditorState(prev => ({ ...prev, editable: false }));
            }
            refresh(false);
        }
    }

    const breakerNamespaceOptions = [{ label: '*', value: '*' }, ...namespaceDatas.map((item: NamespaceView) => ({ label: item.name, value: item.name }))];
    const breakerSourceServiceOptions = serviceDatas
        .filter((opt: ServiceView) => breakerRule.ruleMatcher.source.namespace === '*' || opt.namespace === breakerRule.ruleMatcher.source.namespace)
        .map((item: ServiceView) => ({ label: item.name, value: item.name, namespace: item.namespace }));
    const breakerDestServiceOptions = serviceDatas
        .filter((opt: ServiceView) => breakerRule.ruleMatcher.destination.namespace === '*' || opt.namespace === breakerRule.ruleMatcher.destination.namespace)
        .map((item: ServiceView) => ({ label: item.name, value: item.name, namespace: item.namespace }));

    const destinationView = React.useMemo(() => {
        const destination = breakerRule.ruleMatcher.destination;
        if (destination.service.includes('governance') && !destination.namespace.includes('governance')) {
            return {
                namespace: destination.service,
                service: destination.namespace,
            };
        }
        return destination;
    }, [breakerRule.ruleMatcher.destination]);

    const addSubRule = () => {
        updateRule((draft) => {
            draft.subrules.push(defaultCircuitBreakerSubRule(draft.subrules.length + 1));
        });
    };

    const addStrategy = (subRuleIdx: number) => {
        updateRule((draft) => {
            const subrule = draft.subrules[subRuleIdx];
            subrule.strategies.push(defaultCircuitBreakerStrategy(subrule.strategies.length + 1));
        });
    };

    const defaultErrorCondition = (): CircuitBreakerErrorCondition => ({
        inputType: ErrorConditionType.RET_CODE,
        condition: { type: MatchType.RANGE, value: '500-599' },
    });

    const defaultTriggerCondition = (): CircuitBreakerTriggerCondition => ({
        triggerType: TriggerType.ERROR_RATE,
        errorCount: 0,
        errorPercent: 50,
        triggerVal: 50,
        interval: 30,
        minimumRequest: 5,
    });

    const addIface = (subRuleIdx: number, strategyIdx: number) => {
        updateRule((draft) => {
            draft.subrules[subRuleIdx].strategies[strategyIdx].ifaces.push({
                protocol: 'HTTP',
                method: 'GET',
                path: { type: MatchType.EXACT, value: '', value_type: MatchValueType.TEXT },
            });
        });
    };

    const renderBasicInfo = (
        <CollapsibleSection
            collapsed={basicInfoCollapsed}
            onCollapsedChange={setBasicInfoCollapsed}
            header="基础信息"
            summary={`${breakerRule.name || '未命名规则'} · 优先级 ${breakerRule.priority ?? 0}`}
        >
                <div className={shared.infoGrid}>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>规则名称</div>
                        {editorState.editable
                            ? <Input aria-label="熔断规则名称" value={breakerRule.name} disabled={op === 'edit'} maxLength={64} onChange={(value) => setBreakerRule(prev => ({ ...prev, name: value }))} />
                            : <div className={shared.fieldValue}>{breakerRule.name || '-'}</div>}
                    </div>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>优先级</div>
                        {editorState.editable
                            ? <InputNumber aria-label="熔断规则优先级" theme="normal" min={0} value={breakerRule.priority} onChange={(value) => setBreakerRule(prev => ({ ...prev, priority: value as number }))} />
                            : <div className={shared.fieldValue}>{breakerRule.priority ?? 0}</div>}
                    </div>
                    <div className={`${shared.field} ${shared.span6}`}>
                        <div className={shared.fieldLabel}>描述</div>
                        {editorState.editable
                            ? <Input aria-label="熔断规则描述" value={breakerRule.description} maxLength={255} onChange={(value) => setBreakerRule(prev => ({ ...prev, description: value }))} />
                            : <div className={shared.fieldValue}>{breakerRule.description || '-'}</div>}
                    </div>
                    <div className={`${shared.field} ${shared.full}`}>
                        <div className={shared.fieldLabel}>规则标签</div>
                        <RuleLabelField
                            metadata={metadataRecord}
                            editable={editorState.editable}
                            onChange={(next) => setBreakerRule(prev => ({ ...prev, metadata: Object.entries(next).map(([key, value]) => ({ key, value })) }))}
                        />
                    </div>
                </div>
        </CollapsibleSection>
    );

    const renderServiceScope = (
        <ServiceScopeSection
            editable={Boolean(editorState.editable)}
            collapsed={serviceCollapsed}
            caller={breakerRule.ruleMatcher.source}
            callee={editorState.editable ? breakerRule.ruleMatcher.destination : destinationView}
            namespaceOptions={breakerNamespaceOptions}
            callerServiceOptions={breakerSourceServiceOptions}
            calleeServiceOptions={breakerDestServiceOptions}
            onCollapsedChange={setServiceCollapsed}
            onCallerNamespaceChange={(value) => updateRule(draft => { draft.ruleMatcher.source.namespace = value; })}
            onCallerServiceChange={(value) => updateRule(draft => { draft.ruleMatcher.source.service = value; })}
            onCalleeNamespaceChange={(value) => updateRule(draft => { draft.ruleMatcher.destination.namespace = value; })}
            onCalleeServiceChange={(value) => updateRule(draft => { draft.ruleMatcher.destination.service = value; })}
            fixedRole={op === 'create' ? serviceContext?.role : undefined}
            extraContent={(
                <div className={shared.field}>
                    <div className={shared.fieldLabel}>熔断粒度</div>
                    {canEditLevel ? (
                        <RadioGroup aria-label="熔断粒度" theme="button" variant="primary-filled" value={breakerRule.level} onChange={(value) => setBreakerRule(prev => ({ ...prev, level: value as string }))}>
                            <Radio.Button value={BreakLevelType.Service}>{BreakLevelMap[BreakLevelType.Service]}</Radio.Button>
                            <Radio.Button value={BreakLevelType.Instance}>{BreakLevelMap[BreakLevelType.Instance]}</Radio.Button>
                            <Radio.Button value={BreakLevelType.Method}>{BreakLevelMap[BreakLevelType.Method]}</Radio.Button>
                        </RadioGroup>
                    ) : <div className={shared.fieldValue}>{BreakLevelMap[breakerRule.level as BreakLevelType] || '-'}</div>}
                </div>
            )}
        />
    );

    const renderInterfaceRows = (subRuleIdx: number, strategyIdx: number, strategy: CircuitBreakerStrategyDraft) => (
        <div className={styles.matrixScroller} role="region" aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略接口方法配置`}>
        <div className={styles.interfaceGrid} role="table">
            <div className={`${styles.gridHeader} ${styles.firstHeader}`} role="columnheader">协议</div>
            <div className={styles.gridHeader} role="columnheader">接口方法</div>
            <div className={styles.gridHeader} role="columnheader">接口路径</div>
            <div className={`${styles.gridHeader} ${styles.actionHeader}`} role="columnheader">操作</div>
            {(strategy.ifaces || []).map((api: CircuitBreakerAPI, ifaceIdx) => (
                <React.Fragment key={`iface-${subRuleIdx}-${strategyIdx}-${ifaceIdx}`}>
                    <div className={`${styles.gridCell} ${styles.firstCell}`} role="cell">
                        {editorState.editable ? (
                            <Select aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${ifaceIdx + 1} 个接口协议`} options={InterfaceProtocolOption} value={api.protocol || 'HTTP'} onChange={(value) => updateRule(draft => { draft.subrules[subRuleIdx].strategies[strategyIdx].ifaces[ifaceIdx].protocol = value as string; })} />
                        ) : <Text title={api.protocol || '-'}>{api.protocol || '-'}</Text>}
                    </div>
                    <div className={styles.gridCell} role="cell">
                        {editorState.editable ? (
                            <Select aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${ifaceIdx + 1} 个接口方法`} creatable filterable options={api.protocol === 'HTTP' ? HTTPMethodOption : []} value={api.method || 'GET'} onChange={(value) => updateRule(draft => { draft.subrules[subRuleIdx].strategies[strategyIdx].ifaces[ifaceIdx].method = value as string; })} />
                        ) : <Text title={api.method || '-'}>{api.method || '-'}</Text>}
                    </div>
                    <div className={styles.gridCell} role="cell">
                        {editorState.editable ? (
                            <InputAdornment append={(
                                <Select
                                    aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${ifaceIdx + 1} 个接口路径匹配类型`}
                                    autoWidth
                                    options={MatchTypeOption}
                                    value={api.path?.type || MatchType.EXACT}
                                    onChange={(value) => updateRule(draft => {
                                        const path = draft.subrules[subRuleIdx].strategies[strategyIdx].ifaces[ifaceIdx].path || { type: MatchType.EXACT, value: '', value_type: MatchValueType.TEXT };
                                        draft.subrules[subRuleIdx].strategies[strategyIdx].ifaces[ifaceIdx].path = { ...path, type: value as string };
                                    })}
                                />
                            )}>
                                <Input
                                    aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${ifaceIdx + 1} 个接口路径`}
                                    className={styles.monoInput}
                                    value={api.path?.value || ''}
                                    onChange={(value) => updateRule(draft => {
                                        const path = draft.subrules[subRuleIdx].strategies[strategyIdx].ifaces[ifaceIdx].path || { type: MatchType.EXACT, value: '', value_type: MatchValueType.TEXT };
                                        draft.subrules[subRuleIdx].strategies[strategyIdx].ifaces[ifaceIdx].path = { ...path, value, value_type: MatchValueType.TEXT };
                                    })}
                                />
                            </InputAdornment>
                        ) : <span className={shared.pathTag} title={api.path?.value || '-'}>{api.path?.value || '-'}</span>}
                    </div>
                    <div className={`${styles.gridCell} ${styles.actionCell}`} role="cell">
                        {editorState.editable && (
                            <Space size={4}>
                                <Popup trigger="hover" content="添加接口">
                                    <Button aria-label={`在第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${ifaceIdx + 1} 个接口后添加接口`} shape="circle" variant="text" onClick={() => addIface(subRuleIdx, strategyIdx)}><AddIcon /></Button>
                                </Popup>
                                <Popup trigger="hover" content="删除接口">
                                    <Button
                                        aria-label={`删除第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${ifaceIdx + 1} 个接口`}
                                        shape="circle"
                                        variant="text"
                                        disabled={strategy.ifaces.length <= 1}
                                        onClick={() => updateRule(draft => { draft.subrules[subRuleIdx].strategies[strategyIdx].ifaces.splice(ifaceIdx, 1); })}
                                    >
                                        <RemoveIcon />
                                    </Button>
                                </Popup>
                            </Space>
                        )}
                    </div>
                </React.Fragment>
            ))}
        </div>
        </div>
    );

    const renderErrorRows = (subRuleIdx: number, strategyIdx: number, strategy: CircuitBreakerStrategyDraft) => (
        <div className={styles.matrixScroller} role="region" aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略错误条件配置`}>
        <div className={styles.conditionGrid} role="table">
            <div className={`${styles.gridHeader} ${styles.firstHeader}`} role="columnheader">参数类型</div>
            <div className={styles.gridHeader} role="columnheader">匹配类型</div>
            <div className={styles.gridHeader} role="columnheader">匹配值</div>
            <div className={`${styles.gridHeader} ${styles.actionHeader}`} role="columnheader">操作</div>
            {(strategy.error_conditions || []).map((condition, conditionIdx) => (
                <React.Fragment key={`error-${subRuleIdx}-${strategyIdx}-${conditionIdx}`}>
                    <div className={`${styles.gridCell} ${styles.firstCell}`} role="cell">
                        {editorState.editable ? (
                            <Select aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${conditionIdx + 1} 条错误条件参数类型`} options={ErrorConditionOptions} value={condition.inputType} onChange={(value) => updateRule(draft => { draft.subrules[subRuleIdx].strategies[strategyIdx].error_conditions[conditionIdx].inputType = value as string; })} />
                        ) : <Text title={ErrorConditionMap[condition.inputType as ErrorConditionType] || condition.inputType}>{ErrorConditionMap[condition.inputType as ErrorConditionType] || condition.inputType}</Text>}
                    </div>
                    <div className={styles.gridCell} role="cell">
                        {editorState.editable ? (
                            <Select aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${conditionIdx + 1} 条错误条件匹配类型`} options={MatchTypeOption} value={condition.condition?.type || MatchType.RANGE} onChange={(value) => updateRule(draft => { draft.subrules[subRuleIdx].strategies[strategyIdx].error_conditions[conditionIdx].condition.type = value as string; })} />
                        ) : <Text title={MatchTypeMap[condition.condition?.type as MatchType] || condition.condition?.type}>{MatchTypeMap[condition.condition?.type as MatchType] || condition.condition?.type}</Text>}
                    </div>
                    <div className={styles.gridCell} role="cell">
                        {editorState.editable ? (
                            <Input aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${conditionIdx + 1} 条错误条件匹配值`} value={condition.condition?.value || ''} onChange={(value) => updateRule(draft => { draft.subrules[subRuleIdx].strategies[strategyIdx].error_conditions[conditionIdx].condition.value = value; })} />
                        ) : <Text title={condition.condition?.value || '-'}>{condition.condition?.value || '-'}</Text>}
                    </div>
                    <div className={`${styles.gridCell} ${styles.actionCell}`} role="cell">
                        {editorState.editable && (
                            <Space size={4}>
                                <Popup trigger="hover" content="添加错误条件">
                                    <Button aria-label={`在第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${conditionIdx + 1} 个错误条件后添加条件`} shape="circle" variant="text" onClick={() => updateRule(draft => { draft.subrules[subRuleIdx].strategies[strategyIdx].error_conditions.push(defaultErrorCondition()); })}><AddIcon /></Button>
                                </Popup>
                                <Popup trigger="hover" content="删除错误条件">
                                    <Button
                                        aria-label={`删除第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${conditionIdx + 1} 个错误条件`}
                                        shape="circle"
                                        variant="text"
                                        disabled={strategy.error_conditions.length <= 1}
                                        onClick={() => updateRule(draft => { draft.subrules[subRuleIdx].strategies[strategyIdx].error_conditions.splice(conditionIdx, 1); })}
                                    >
                                        <RemoveIcon />
                                    </Button>
                                </Popup>
                            </Space>
                        )}
                    </div>
                </React.Fragment>
            ))}
        </div>
        </div>
    );

    const renderTriggerRows = (subRuleIdx: number, strategyIdx: number, strategy: CircuitBreakerStrategyDraft) => (
        <div className={styles.matrixScroller} role="region" aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略触发条件配置`}>
        <div className={styles.triggerGrid} role="table">
            <div className={`${styles.gridHeader} ${styles.firstHeader}`} role="columnheader">类型</div>
            <div className={styles.gridHeader} role="columnheader">比较</div>
            <div className={styles.gridHeader} role="columnheader">阈值</div>
            <div className={styles.gridHeader} role="columnheader">统计周期</div>
            <div className={styles.gridHeader} role="columnheader">最小请求数</div>
            <div className={`${styles.gridHeader} ${styles.actionHeader}`} role="columnheader">操作</div>
            {(strategy.trigger_conditions || []).map((condition, conditionIdx) => {
                const isRatio = condition.triggerType === TriggerType.ERROR_RATE;
                const thresholdValue = isRatio ? condition.errorPercent ?? condition.triggerVal ?? 0 : condition.errorCount ?? condition.triggerVal ?? 0;
                return (
                    <React.Fragment key={`trigger-${subRuleIdx}-${strategyIdx}-${conditionIdx}`}>
                        <div className={`${styles.gridCell} ${styles.firstCell}`} role="cell">
                            {editorState.editable ? (
                                <Select
                                    aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${conditionIdx + 1} 条触发条件类型`}
                                    options={TriggerTypeOptions}
                                    value={condition.triggerType}
                                    onChange={(value) => updateRule(draft => {
                                        const item = draft.subrules[subRuleIdx].strategies[strategyIdx].trigger_conditions[conditionIdx];
                                        item.triggerType = value as string;
                                        item.triggerVal = value === TriggerType.ERROR_RATE ? item.errorPercent || 50 : item.errorCount || 3;
                                    })}
                                />
                            ) : <Text title={TriggerTypeMap[condition.triggerType as TriggerType]?.text || condition.triggerType}>{TriggerTypeMap[condition.triggerType as TriggerType]?.text || condition.triggerType}</Text>}
                        </div>
                        <div className={styles.gridCell} role="cell"><Text>{'>='}</Text></div>
                        <div className={styles.gridCell} role="cell">
                            {editorState.editable ? (
                                <InputAdornment append={isRatio ? '%' : '次'}>
                                    <InputNumber
                                        aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${conditionIdx + 1} 条触发条件阈值`}
                                        theme="normal"
                                        min={0}
                                        max={isRatio ? 100 : undefined}
                                        value={thresholdValue}
                                        onChange={(value) => updateRule(draft => {
                                            const item = draft.subrules[subRuleIdx].strategies[strategyIdx].trigger_conditions[conditionIdx];
                                            const nextVal = Number(value || 0);
                                            item.triggerVal = nextVal;
                                            if (item.triggerType === TriggerType.ERROR_RATE) {
                                                item.errorPercent = nextVal;
                                                item.errorCount = 0;
                                            } else {
                                                item.errorCount = nextVal;
                                                item.errorPercent = 0;
                                            }
                                        })}
                                    />
                                </InputAdornment>
                            ) : <Text>{`${thresholdValue} ${isRatio ? '%' : '次'}`}</Text>}
                        </div>
                        <div className={styles.gridCell} role="cell">
                            {editorState.editable ? (
                                <InputAdornment append="秒">
                                    <InputNumber aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${conditionIdx + 1} 条触发条件统计周期`} theme="normal" min={0} value={condition.interval || 0} onChange={(value) => updateRule(draft => { draft.subrules[subRuleIdx].strategies[strategyIdx].trigger_conditions[conditionIdx].interval = Number(value || 0); })} />
                                </InputAdornment>
                            ) : <Text>{`${condition.interval || 0} 秒`}</Text>}
                        </div>
                        <div className={styles.gridCell} role="cell">
                            {editorState.editable ? (
                                <InputAdornment append="个">
                                    <InputNumber aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${conditionIdx + 1} 条触发条件最小请求数`} theme="normal" min={0} value={condition.minimumRequest || 0} onChange={(value) => updateRule(draft => { draft.subrules[subRuleIdx].strategies[strategyIdx].trigger_conditions[conditionIdx].minimumRequest = Number(value || 0); })} />
                                </InputAdornment>
                            ) : <Text>{`${condition.minimumRequest || 0} 个`}</Text>}
                        </div>
                        <div className={`${styles.gridCell} ${styles.actionCell}`} role="cell">
                            {editorState.editable && (
                                <Space size={4}>
                                    <Popup trigger="hover" content="添加触发条件">
                                        <Button aria-label={`在第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${conditionIdx + 1} 个触发条件后添加条件`} shape="circle" variant="text" onClick={() => updateRule(draft => { draft.subrules[subRuleIdx].strategies[strategyIdx].trigger_conditions.push(defaultTriggerCondition()); })}><AddIcon /></Button>
                                    </Popup>
                                    <Popup trigger="hover" content="删除触发条件">
                                        <Button
                                            aria-label={`删除第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略第 ${conditionIdx + 1} 个触发条件`}
                                            shape="circle"
                                            variant="text"
                                            disabled={strategy.trigger_conditions.length <= 1}
                                            onClick={() => updateRule(draft => { draft.subrules[subRuleIdx].strategies[strategyIdx].trigger_conditions.splice(conditionIdx, 1); })}
                                        >
                                            <RemoveIcon />
                                        </Button>
                                    </Popup>
                                </Space>
                            )}
                        </div>
                    </React.Fragment>
                );
            })}
        </div>
        </div>
    );

    const renderStrategy = (subRuleIdx: number, strategy: CircuitBreakerStrategyDraft, strategyIdx: number) => {
        const collapsed = collapsedStrategyKeys.has(`${subRuleIdx}-${strategyIdx}`);
        return (
            <div className={`${shared.policy} ${styles.strategyPolicy} ${collapsed ? shared.policyCollapsed : ''}`} key={`${subRuleIdx}-${strategyIdx}`}>
                <div className={shared.policyHead} onClick={() => toggleStrategyCollapsed(subRuleIdx, strategyIdx)}>
                    <div className={shared.policyHeadMain}>
                        <span className={shared.caret}><ChevronRightIcon /></span>
                        <div>
                            <div className={shared.policyIndex}>熔断策略 [{strategyIdx + 1}]{strategy.name ? `：${strategy.name}` : ''}</div>
                            <div className={shared.policySummary}>{describeStrategySummary(strategy)}</div>
                        </div>
                    </div>
                    <div className={shared.policyHeadActions} onClick={(e) => e.stopPropagation()}>
                        {editorState.editable && (
                            <Popup trigger="hover" content="删除策略">
                                <Button
                                    aria-label={`删除第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略`}
                                    shape="circle"
                                    variant="text"
                                    disabled={breakerRule.subrules[subRuleIdx].strategies.length <= 1}
                                    onClick={() => updateRule(draft => { draft.subrules[subRuleIdx].strategies.splice(strategyIdx, 1); })}
                                >
                                    <CloseIcon />
                                </Button>
                            </Popup>
                        )}
                    </div>
                </div>
                <div className={shared.policyBody}>
                    <div className={shared.step} data-step="1">
                        <div className={shared.stepTitle}>策略名称<span className={shared.stepHint}>用于区分同一子规则内不同触发策略</span></div>
                        <div className={shared.stepContent}>
                            {editorState.editable ? (
                                <Input aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${strategyIdx + 1} 个策略名称`} value={strategy.name} onChange={(value) => updateRule(draft => { draft.subrules[subRuleIdx].strategies[strategyIdx].name = value; })} />
                            ) : renderReadonlyValue(strategy.name || '-')}
                        </div>
                    </div>
                    <div className={shared.step} data-step="2">
                        <div className={shared.stepTitle}>接口范围<span className={shared.stepHint}>满足任一接口条件的请求进入该策略判断</span></div>
                        <div className={shared.stepContent}>{renderInterfaceRows(subRuleIdx, strategyIdx, strategy)}</div>
                    </div>
                    <div className={shared.step} data-step="3">
                        <div className={shared.stepTitle}>错误判断条件<span className={shared.stepHint}>定义哪些请求算作错误</span></div>
                        <div className={shared.stepContent}>{renderErrorRows(subRuleIdx, strategyIdx, strategy)}</div>
                    </div>
                    <div className={shared.step} data-step="4">
                        <div className={shared.stepTitle}>熔断触发条件<span className={shared.stepHint}>满足任一统计条件即可触发熔断</span></div>
                        <div className={shared.stepContent}>{renderTriggerRows(subRuleIdx, strategyIdx, strategy)}</div>
                    </div>
                </div>
            </div>
        );
    };

    const renderRecover = (subRuleIdx: number, subrule: CircuitBreakerSubRuleDraft) => (
        <div className={shared.step} data-step="2">
            <div className={shared.stepTitle}>恢复策略<span className={shared.stepHint}>该子规则进入熔断后的恢复与主动探测开关</span></div>
            <div className={shared.stepContent}>
                <div className={styles.recoverGrid}>
                    <div>
                        <div className={shared.fieldLabel}>最大剔除比例</div>
                        {editorState.editable ? (
                            <InputAdornment append="%">
                                <InputNumber aria-label={`第 ${subRuleIdx + 1} 个熔断子规则最大剔除比例`} theme="normal" min={0} max={100} value={subrule.max_ejection_percent} onChange={(value) => updateRule(draft => { draft.subrules[subRuleIdx].max_ejection_percent = Number(value || 0); })} />
                            </InputAdornment>
                        ) : renderReadonlyValue(`${subrule.max_ejection_percent ?? '-'}%`)}
                    </div>
                    <div>
                        <div className={shared.fieldLabel}>熔断时长</div>
                        {editorState.editable ? (
                            <InputAdornment append="秒">
                                <InputNumber aria-label={`第 ${subRuleIdx + 1} 个熔断子规则熔断时长`} theme="normal" min={0} value={subrule.recoverCondition?.sleepWindow} onChange={(value) => updateRule(draft => { draft.subrules[subRuleIdx].recoverCondition.sleepWindow = Number(value || 0); })} />
                            </InputAdornment>
                        ) : renderReadonlyValue(`${subrule.recoverCondition?.sleepWindow ?? '-'} 秒`)}
                    </div>
                    <div>
                        <div className={shared.fieldLabel}>连续成功次数</div>
                        {editorState.editable ? (
                            <InputAdornment append="次">
                                <InputNumber aria-label={`第 ${subRuleIdx + 1} 个熔断子规则连续成功次数`} theme="normal" min={0} value={subrule.recoverCondition?.consecutiveSuccess || 0} onChange={(value) => updateRule(draft => { draft.subrules[subRuleIdx].recoverCondition.consecutiveSuccess = Number(value || 0); })} />
                            </InputAdornment>
                        ) : renderReadonlyValue(`${subrule.recoverCondition?.consecutiveSuccess ?? 0} 次`)}
                    </div>
                    <div>
                        <div className={shared.fieldLabel}>主动探测</div>
                        {editorState.editable ? (
                            <Switch aria-label={`第 ${subRuleIdx + 1} 个熔断子规则主动探测`} value={subrule.faultDetectConfig?.enable} onChange={(checked) => updateRule(draft => { draft.subrules[subRuleIdx].faultDetectConfig = { enable: checked as boolean }; })} />
                        ) : renderReadonlySwitch(subrule.faultDetectConfig?.enable)}
                    </div>
                </div>
            </div>
        </div>
    );

    const renderFallback = (subRuleIdx: number, subrule: CircuitBreakerSubRuleDraft) => (
        <div className={shared.step} data-step="3">
            <div className={shared.stepTitle}>
                熔断后降级
                <span className={shared.stepHint}>该子规则触发时返回的兜底响应</span>
                {editorState.editable ? (
                    <Switch aria-label={`第 ${subRuleIdx + 1} 个熔断子规则熔断后降级`} value={subrule.fallbackConfig?.enable} onChange={(checked) => updateRule(draft => { draft.subrules[subRuleIdx].fallbackConfig.enable = checked as boolean; })} />
                ) : renderReadonlySwitch(subrule.fallbackConfig?.enable)}
            </div>
            {subrule.fallbackConfig?.enable && (
                <div className={shared.stepContent}>
                    <div className={styles.fallbackGrid}>
                        <div>
                            <div className={shared.fieldLabel}>响应码</div>
                            {editorState.editable ? (
                                <InputNumber aria-label={`第 ${subRuleIdx + 1} 个熔断子规则降级响应码`} theme="normal" value={subrule.fallbackConfig?.response?.code} onChange={(value) => updateRule(draft => { draft.subrules[subRuleIdx].fallbackConfig.response.code = Number(value || 0); })} />
                            ) : renderReadonlyValue(subrule.fallbackConfig?.response?.code ?? '-')}
                        </div>
                        <div>
                            <div className={shared.fieldLabel}>响应头</div>
                            <div className={shared.tagRow}>
                                {(subrule.fallbackConfig?.response?.headers || []).length > 0 ? (
                                    subrule.fallbackConfig.response.headers.map((header, idx) => (
                                        <Tag key={`${header.key}-${idx}`} variant="light">{`${header.key}: ${header.value}`}</Tag>
                                    ))
                                ) : (
                                    <Text>暂无响应头</Text>
                                )}
                                {editorState.editable && (
                                    <Button aria-label={`编辑第 ${subRuleIdx + 1} 个子规则的响应头`} shape="circle" variant="text" onClick={() => setEditorState(prev => ({ ...prev, headerVisible: true, headerSubRuleIndex: subRuleIdx }))}>
                                        <Edit1Icon />
                                    </Button>
                                )}
                            </div>
                            {renderRspHeaderDialog(subRuleIdx, subrule)}
                        </div>
                        <div className={styles.fallbackBodyCell}>
                            <div className={shared.fieldLabel}>响应体</div>
                            {editorState.editable ? (
                                <Textarea aria-label={`第 ${subRuleIdx + 1} 个熔断子规则降级响应体`} value={subrule.fallbackConfig?.response?.body || ''} onChange={(value) => updateRule(draft => { draft.subrules[subRuleIdx].fallbackConfig.response.body = value as string; })} />
                            ) : (
                                <pre className={styles.readonlyCodeBlock}>{subrule.fallbackConfig?.response?.body || '-'}</pre>
                            )}
                        </div>
                    </div>
                </div>
            )}
        </div>
    );

    const renderSubRule = (subrule: CircuitBreakerSubRuleDraft, subRuleIdx: number) => {
        const collapsed = collapsedSubRuleIndexes.has(subRuleIdx);
        return (
            <div className={`${shared.policy} ${styles.subRulePolicy} ${collapsed ? shared.policyCollapsed : ''}`} key={subRuleIdx}>
                <div className={`${shared.policyHead} ${styles.subRuleHead}`} onClick={() => toggleSubRuleCollapsed(subRuleIdx)}>
                    <div className={shared.policyHeadMain}>
                        <span className={shared.caret}><ChevronRightIcon /></span>
                        <div>
                            <div className={shared.policyIndex}>子规则 [{subRuleIdx + 1}]</div>
                            <div className={shared.policySummary}>{describeSubRuleSummary(subrule)}</div>
                        </div>
                    </div>
                    <div className={shared.policyHeadActions} onClick={(e) => e.stopPropagation()}>
                        <Tag theme={subrule.fallbackConfig?.enable ? 'warning' : 'default'} variant="light">降级{subrule.fallbackConfig?.enable ? '开' : '关'}</Tag>
                        {editorState.editable && (
                            <Popup trigger="hover" content="删除子规则">
                                <Button
                                    aria-label={`删除第 ${subRuleIdx + 1} 个子规则`}
                                    shape="circle"
                                    variant="text"
                                    disabled={breakerRule.subrules.length <= 1}
                                    onClick={() => updateRule(draft => { draft.subrules.splice(subRuleIdx, 1); })}
                                >
                                    <CloseIcon />
                                </Button>
                            </Popup>
                        )}
                    </div>
                </div>
                <div className={shared.policyBody}>
                    <div className={shared.step} data-step="1">
                        <div className={shared.stepTitle}>
                            熔断策略
                            <span className={shared.countTag}>{subrule.strategies.length} 个</span>
                            <span className={shared.stepHint}>命中任一策略即触发该子规则的恢复/降级配置</span>
                        </div>
                        <div className={shared.stepContent}>
                            <div className={styles.strategyList}>
                                {subrule.strategies.map((strategy, strategyIdx) => renderStrategy(subRuleIdx, strategy, strategyIdx))}
                            </div>
                            {editorState.editable && (
                                <Button aria-label={`第 ${subRuleIdx + 1} 个熔断子规则添加熔断策略`} className={styles.inlineAdd} variant="dashed" icon={<AddIcon />} onClick={() => addStrategy(subRuleIdx)}>
                                    添加熔断策略
                                </Button>
                            )}
                        </div>
                    </div>
                    {renderRecover(subRuleIdx, subrule)}
                    {renderFallback(subRuleIdx, subrule)}
                </div>
            </div>
        );
    };

    const renderRspHeaderDialog = (subRuleIdx: number, subrule: CircuitBreakerSubRuleDraft) => (
        <Dialog
            visible={editorState.headerVisible && editorState.headerSubRuleIndex === subRuleIdx}
            header="编辑响应头"
            width={700}
            onConfirm={() => setEditorState(prev => ({ ...prev, headerVisible: false, headerSubRuleIndex: undefined }))}
            onClose={() => setEditorState(prev => ({ ...prev, headerVisible: false, headerSubRuleIndex: undefined }))}
        >
            <div>
                {(subrule?.fallbackConfig?.response?.headers || []).map((tag, idx) => (
                    <div key={idx} className={styles.headerEditRow}>
                        <Input aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${idx + 1} 个响应头 Key`} value={tag.key} placeholder="响应头 Key" onChange={value => updateRule(draft => { draft.subrules[subRuleIdx].fallbackConfig.response.headers[idx].key = value; })} />
                        <Input aria-label={`第 ${subRuleIdx + 1} 个熔断子规则第 ${idx + 1} 个响应头 Value`} value={tag.value} placeholder="响应头 Value" onChange={value => updateRule(draft => { draft.subrules[subRuleIdx].fallbackConfig.response.headers[idx].value = value; })} />
                        <Button aria-label={`删除第 ${subRuleIdx + 1} 个熔断子规则第 ${idx + 1} 个响应头`} shape="circle" variant="text" onClick={() => updateRule(draft => { draft.subrules[subRuleIdx].fallbackConfig.response.headers.splice(idx, 1); })}>
                            <CloseIcon />
                        </Button>
                    </div>
                ))}
                <Button
                    variant="text"
                    icon={<AddIcon />}
                    onClick={() => updateRule(draft => { draft.subrules[subRuleIdx].fallbackConfig.response.headers.push({ key: '', value: '' }); })}
                >
                    添加响应头
                </Button>
            </div>
        </Dialog>
    );

    const renderSubRules = (
        <div className={shared.section}>
            <div className={shared.sectionHeader}>
                <span>熔断子规则</span>
                <span className={shared.countTag}>{breakerRule.subrules.length} 条</span>
            </div>
            <div className={shared.sectionBody}>
                <div className={shared.ruleList}>
                    {breakerRule.subrules.map((subrule, idx) => renderSubRule(subrule, idx))}
                </div>
                {editorState.editable && (
                    <Button className={styles.inlineAdd} variant="dashed" icon={<AddIcon />} onClick={addSubRule}>
                        添加子规则
                    </Button>
                )}
            </div>
        </div>
    );

    const renderPublishForm = (
        <>
            {editorState.publishView && (
                <PublishForm
                    ruleId={viewRule?.id || ''}
                    ruleName={viewRule?.name || ''}
                    resource={PolicySourceType.CircuitBreakerRules}
                    visible={editorState.publishView}
                    close={() => {
                        setEditorState(prev => ({ ...prev, publishView: false }));
                    }}
                />
            )}
        </>
    )

    const renderStickyTool = (
        <>
            {(op === 'create' || viewRule?.editable !== false) && (
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
                                <RuleStickyAction label="保存" icon={<SaveIcon />} onClick={() => {
                                    form.submit();
                                }} />
                            }
                        />
                        {(editorState.editable) && (
                            <StickyItem label="" icon={
                                <RuleStickyAction label="撤销" icon={<RollbackIcon />} onClick={() => {
                                    if (op === 'create') {
                                        refresh(true);
                                    } else {
                                        resetCurRule(viewRule as CircuitBreakerRule);
                                        setEditorState(prev => ({ ...prev, editable: false }));
                                    }
                                }} />}
                            />
                        )}
                        {(!editorState.editable && op !== 'create') && (
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
            <Form form={form} onSubmit={onSubmit} layout="vertical" colon>
                <div className={styles.circuitBreakerEditorShell}>
                    <div className={styles.formPane}>
                        {renderBasicInfo}
                        {renderServiceScope}
                        {renderSubRules}
                    </div>
                </div>
            </Form>
            {renderPublishForm}
            {renderStickyTool}
        </div>
    );
};

export default React.memo(CircuitBreakerEditor);
