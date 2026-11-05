import React from "react";
import {
    Button,
    Form,
    Input,
    InputNumber,
    Popup,
    Radio,
    RadioGroup,
    Select,
    StickyTool,
    Switch,
    Tag,
    TagInput,
    Textarea,
} from 'components/Fluent';
import {
    AddIcon,
    ChevronRightIcon,
    CloseIcon,
    Edit1Icon,
    RocketIcon,
    RollbackIcon,
    SaveIcon,
} from 'components/Fluent/icons';

import RuleLabelField from "../shared/RuleLabelField";
import CollapsibleSection from "../shared/CollapsibleSection";
import TrafficMatchConditionEditor, { TrafficMatchConditionRow } from "../shared/TrafficMatchConditionEditor";
import { GovernanceServiceContext } from "../shared/serviceContext";
import { useRuleNamespace } from "../shared/ruleNamespace";
import shared from "../shared/governance.module.less";
import { useAppDispatch, useAppSelector } from 'modules/store';
import { ServiceView } from "services/service";
import { NamespaceView } from "services/namespace";
import { MatchLogic, MatchType, MatchTypeMap, MatchTypeOption, MatchValueType, Op } from "services/types";
import { openErrNotification, openInfoNotification } from "utils/notifition";
import {
    listOneRateLimitRule,
    saveRateLimitRule,
    selectRateLimitRule,
    updateRateLimitRule,
} from 'modules/governance/ratelimit';
import {
    defaultLimitTriggerView,
    LimitAction,
    LimitActionMap,
    LimitAmountsValidationUnit,
    LimitArgumentsConfig,
    LimitArgumentsType,
    LimitArgumentsTypeMap,
    LimitArgumentsTypeOptions,
    LimitConfigView,
    LimitFailover,
    LimitFailoverMap,
    LimitType,
    LimitTypeMap,
    RateLimitResource,
    RateLimitResourceMap,
    RateLimitView,
} from "services/ratelimit";
import PublishForm from "../RuleRelease/PublishForm";
import RuleStickyAction from "../RuleRelease/RuleStickyAction";
import { PolicySourceType } from "services/auth_policy";
import { cleanNamespacePage, listAllNamespaces, selectNamespace } from "modules/namespace";
import { cleanServicePage, listAllServices, selectService } from "modules/discovery/service";
import cloneDeep from "lodash/cloneDeep";

import {
    defaultRateLimitAPI,
    describeRuleSummary,
    describeRuleThreshold,
    normalizeRateLimitApis,
    RATE_LIMIT_HTTP_METHOD_OPTIONS,
    RateLimitAPI,
    RATE_LIMIT_PROTOCOL_OPTIONS,
    validateRateLimitDraft,
} from "./rateLimitEditorUtils";
import { commaStringToTags, isTagInputMatchType, tagsToCommaString } from "../Router/routeEditorUtils";
import styles from './RateLimitEditor.module.less';
import QuotaLeaseRuntime from './QuotaLeaseRuntime';

const { FormItem } = Form;
const { StickyItem } = StickyTool;

const defaultMatchArgs: () => LimitArgumentsConfig = () => ({
    type: LimitArgumentsType.HEADER,
    key: '',
    value: {
        type: MatchType.EXACT,
        value: '',
        value_type: MatchValueType.TEXT
    }
});

type QuotaScenario = 'QPS' | 'TPM' | 'CONCURRENCY';

const quotaScenarios: Array<{ value: QuotaScenario; title: string; description: string }> = [
    { value: 'QPS', title: 'QPS', description: '请求数 · 默认 1 秒' },
    { value: 'TPM', title: 'TPM', description: 'Token 数 · 默认 60 秒' },
    { value: 'CONCURRENCY', title: '并发', description: '同时占用槽位' },
];

const detectQuotaScenario = (trigger: RateLimitView['rules'][number]): QuotaScenario | '' => {
    if (trigger.resource === RateLimitResource.Concurrency) return 'CONCURRENCY';
    if (trigger.resource === RateLimitResource.Token) return 'TPM';
    if (trigger.resource === RateLimitResource.QPS) return 'QPS';
    return '';
};

export const defaultRateLimitView = (): RateLimitView => ({
    id: '',
    name: '',
    namespace: '',
    callee: { namespace: '', service: '' },
    type: LimitType.LOCAL,
    priority: 0,
    disable: false,
    rules: [defaultLimitTriggerView()],
});

interface IRateLimitEditorProps {
    limitType: LimitType;
    op: Op;
    refresh: (close: boolean) => void;
    visible: boolean;
    serviceContext?: GovernanceServiceContext;
}

const RateLimitEditor: React.FC<IRateLimitEditorProps> = ({ limitType, op, refresh, visible, serviceContext }) => {
    const ruleNamespace = useRuleNamespace();
    const [form] = Form.useForm();
    const dispatch = useAppDispatch();

    const { datas: namespaceDatas } = useAppSelector(selectNamespace);
    const { datas: serviceDatas } = useAppSelector(selectService);
    const { editRule, viewRule } = useAppSelector(selectRateLimitRule);

    const [rateLimit, setRateLimit] = React.useState<RateLimitView>(() => ({
        ...defaultRateLimitView(),
        type: limitType,
    }));
    const [basicInfoCollapsed, setBasicInfoCollapsed] = React.useState(false);
    const [collapsedRuleIndexes, setCollapsedRuleIndexes] = React.useState<Set<number>>(() => new Set());
    const [editorState, setEditorState] = React.useState<{
        editable: boolean;
        publishView: boolean;
    }>({ editable: op === 'create', publishView: false });

    React.useEffect(() => {
        dispatch(listAllNamespaces()).then((res) => {
            if (res.meta.requestStatus === 'rejected') {
                openErrNotification('请求错误', `获取命名空间列表失败: ${res.payload as string}`);
            }
        });
        dispatch(listAllServices()).then((res) => {
            if (res.meta.requestStatus === 'rejected') {
                openErrNotification('请求错误', `获取服务列表失败: ${res.payload as string}`);
            }
        });

        return () => {
            dispatch(cleanNamespacePage());
            dispatch(cleanServicePage());
        };
    }, []);

    React.useEffect(() => {
        setBasicInfoCollapsed(false);
        if (!visible) {
            setEditorState({ editable: op === 'create', publishView: false });
            setCollapsedRuleIndexes(new Set());
            return;
        }
        setEditorState({ editable: op === 'create', publishView: false });
    }, [visible, op]);

    React.useEffect(() => {
        if (!editRule) return;
        if (editRule.id) {
            dispatch(listOneRateLimitRule({ id: editRule.id })).then((res) => {
                if (res.meta.requestStatus === 'fulfilled') {
                    const ret = res.payload as { viewRule: RateLimitView } | null;
                    resetCurRule(ret?.viewRule ?? null);
                } else {
                    openErrNotification('请求错误', `获取限流规则详情失败: ${res.payload as string}`);
                }
            });
        } else {
            resetCurRule(editRule);
        }
    }, [editRule]);

    React.useEffect(() => {
        if (op !== 'create' || !visible) return;
        setRateLimit((prev) => ({
            ...prev,
            namespace: ruleNamespace || prev.namespace,
            callee: {
                namespace: serviceContext?.namespace || prev.callee.namespace,
                service: serviceContext?.service || prev.callee.service,
            },
        }));
    }, [op, visible, ruleNamespace, serviceContext?.namespace, serviceContext?.service]);

    const normalizeRulesForMode = (rules: RateLimitView['rules'], type: LimitType) => (
        rules.map((rule) => ({
            ...rule,
            action: type === LimitType.GLOBAL ? LimitAction.REJECT : rule.action,
            failover: rule.failover || LimitFailover.FAILOVER_LOCAL,
            regex_combine: rule.regex_combine === true,
            apis: normalizeRateLimitApis(rule),
            amounts: rule.amounts?.length ? rule.amounts : [{ validDuration: 1, validDurationUnit: LimitAmountsValidationUnit.s, maxAmount: 1 }],
        }))
    );

    const resetCurRule = (rule: RateLimitView | null) => {
        if (!rule) return;
        const cloneRule = cloneDeep(rule);
        const type = cloneRule.type || limitType || LimitType.LOCAL;
        const rules = (cloneRule.rules && cloneRule.rules.length > 0) ? cloneRule.rules : [defaultLimitTriggerView()];
        setRateLimit({
            ...cloneRule,
            name: cloneRule.name || '',
            namespace: cloneRule.namespace || '',
            callee: cloneRule.callee || { namespace: '', service: '' },
            type,
            priority: cloneRule.priority ?? 0,
            disable: cloneRule.disable ?? false,
            rules: normalizeRulesForMode(rules, type),
        });
    };

    const namespaceSelectOptions = namespaceDatas.map((ns: NamespaceView) => ({ label: ns.name, value: ns.name }));
    const serviceSelectOptions = serviceDatas
        .filter((opt: ServiceView) => !rateLimit.callee.namespace || rateLimit.callee.namespace === '*' || opt.namespace === rateLimit.callee.namespace)
        .map((s: ServiceView) => ({ label: s.name, value: s.name }));

    const editable = editorState.editable;

    const updateRule = (ruleIdx: number, patch: Partial<RateLimitView['rules'][0]>) => {
        setRateLimit(prev => ({
            ...prev,
            rules: prev.rules.map((rule, index) => {
                if (index !== ruleIdx) return rule;
                const next = { ...rule, ...patch };
                if (prev.type === LimitType.GLOBAL) {
                    next.action = LimitAction.REJECT;
                }
                return next;
            }),
        }));
    };

    const updateMode = (type: LimitType) => {
        setRateLimit(prev => ({
            ...prev,
            type,
            rules: normalizeRulesForMode(prev.rules, type),
        }));
    };

    const addRule = () => {
        setRateLimit(prev => ({
            ...prev,
            rules: normalizeRulesForMode([...prev.rules, defaultLimitTriggerView()], prev.type),
        }));
    };

    const removeRule = (ruleIdx: number) => {
        if (rateLimit.rules.length <= 1) return;
        setRateLimit(prev => ({ ...prev, rules: prev.rules.filter((_, index) => index !== ruleIdx) }));
    };

    const toggleRuleCollapsed = (ruleIdx: number) => {
        setCollapsedRuleIndexes(prev => {
            const next = new Set(prev);
            if (next.has(ruleIdx)) {
                next.delete(ruleIdx);
            } else {
                next.add(ruleIdx);
            }
            return next;
        });
    };

    const addMatch = (ruleIdx: number) => {
        const trigger = rateLimit.rules[ruleIdx];
        updateRule(ruleIdx, { arguments: [...(trigger.arguments || []), defaultMatchArgs()] });
    };

    const updateMatch = (ruleIdx: number, matchIdx: number, patch: Partial<LimitArgumentsConfig>) => {
        const trigger = rateLimit.rules[ruleIdx];
        updateRule(ruleIdx, {
            arguments: (trigger.arguments || []).map((item, index) => index === matchIdx ? { ...item, ...patch } : item),
        });
    };

    const removeMatch = (ruleIdx: number, matchIdx: number) => {
        const trigger = rateLimit.rules[ruleIdx];
        updateRule(ruleIdx, { arguments: (trigger.arguments || []).filter((_, index) => index !== matchIdx) });
    };

    const addLimit = (ruleIdx: number) => {
        const trigger = rateLimit.rules[ruleIdx];
        updateRule(ruleIdx, {
            amounts: [...(trigger.amounts || []), { validDuration: 1, validDurationUnit: LimitAmountsValidationUnit.s, maxAmount: 1 }],
        });
    };

    const updateLimit = (ruleIdx: number, amountIdx: number, patch: Partial<LimitConfigView>) => {
        const trigger = rateLimit.rules[ruleIdx];
        updateRule(ruleIdx, {
            amounts: (trigger.amounts || []).map((item, index) => index === amountIdx ? { ...item, ...patch } : item),
        });
    };

    const removeLimit = (ruleIdx: number, amountIdx: number) => {
        const trigger = rateLimit.rules[ruleIdx];
        if ((trigger.amounts || []).length <= 1) return;
        updateRule(ruleIdx, { amounts: (trigger.amounts || []).filter((_, index) => index !== amountIdx) });
    };

    const sanitizedRateLimitForSubmit = (): RateLimitView => ({
        ...rateLimit,
        id: viewRule?.id || rateLimit.id,
        type: rateLimit.type || limitType,
        priority: rateLimit.priority ?? 0,
        disable: rateLimit.disable ?? false,
        rules: normalizeRulesForMode(rateLimit.rules, rateLimit.type || limitType),
    });

    const onSubmit = async () => {
        const errors = validateRateLimitDraft(rateLimit);
        if (errors.length > 0) {
            openErrNotification('保存失败', errors[0].message);
            return;
        }
        const data = sanitizedRateLimitForSubmit();
        const res = op === 'view'
            ? await dispatch(updateRateLimitRule({ param: data }))
            : await dispatch(saveRateLimitRule({ param: data }));
        if (res.meta.requestStatus === 'rejected') {
            openErrNotification('请求错误', `${op === 'view' ? '修改' : '创建'}限流规则失败: ${res.payload as string}`);
        } else {
            openInfoNotification('请求成功', op === 'view' ? '修改限流规则成功' : '创建限流规则成功');
            if (op === 'view') {
                setEditorState(prev => ({ ...prev, editable: false }));
            }
            refresh(false);
        }
    };

    const ruleBaseInfo = (
        <CollapsibleSection
            collapsed={basicInfoCollapsed}
            onCollapsedChange={setBasicInfoCollapsed}
            header="基础信息"
            summary={`${rateLimit.name || '未命名规则'} · ${rateLimit.disable ? '停用' : '启用'} · 优先级 ${rateLimit.priority ?? 0}`}
        >
                <div className={shared.infoGrid}>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>规则名称</div>
                        {editable
                            ? <Input aria-label="限流规则名称" maxLength={64} value={rateLimit.name} onChange={(value) => setRateLimit(prev => ({ ...prev, name: value }))} />
                            : <div className={shared.fieldValue}>{rateLimit.name || '-'}</div>}
                    </div>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>优先级</div>
                        {editable
                            ? <InputNumber aria-label="限流规则优先级" theme="normal" className={styles.fullControl} min={0} value={rateLimit.priority ?? 0} onChange={(value) => setRateLimit(prev => ({ ...prev, priority: (value as number) ?? 0 }))} />
                            : <div className={shared.fieldValue}>{rateLimit.priority ?? 0}</div>}
                    </div>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>启用状态</div>
                        {editable
                            ? <Switch label={['启用', '停用']} value={!rateLimit.disable} onChange={(value) => setRateLimit(prev => ({ ...prev, disable: !(value as boolean) }))} />
                            : <div className={shared.fieldValue}><span className={`${shared.pill} ${rateLimit.disable ? shared.pillOff : shared.pillOk} ${shared.pillDot}`}>{rateLimit.disable ? '停用' : '启用'}</span></div>}
                    </div>
                    <div className={`${shared.field} ${shared.full}`}>
                        <div className={shared.fieldLabel}>规则标签</div>
                        <RuleLabelField
                            metadata={rateLimit.metadata}
                            editable={editable}
                            onChange={(next) => setRateLimit(prev => ({ ...prev, metadata: next }))}
                        />
                    </div>
                </div>
        </CollapsibleSection>
    );

    const scopeInfo = (
        <section className={shared.section}>
            <div className={shared.sectionHeader}>作用对象</div>
            <div className={shared.sectionBody}>
                <div className={shared.infoGrid}>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>目标命名空间</div>
                        {editable && !(op === 'create' && serviceContext)
                            ? <Select aria-label="限流目标命名空间" filterable creatable options={namespaceSelectOptions} value={rateLimit.callee.namespace} onChange={(value) => setRateLimit(prev => ({ ...prev, callee: { namespace: value as string, service: '' } }))} />
                            : <div className={shared.fieldValue}>{rateLimit.callee.namespace || '-'}</div>}
                    </div>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>服务名称</div>
                        {editable && !(op === 'create' && serviceContext)
                            ? <Select aria-label="限流目标服务" filterable creatable options={serviceSelectOptions} value={rateLimit.callee.service} onChange={(value) => setRateLimit(prev => ({ ...prev, callee: { ...prev.callee, service: value as string } }))} />
                            : <div className={shared.fieldValue}>{rateLimit.callee.service || '-'}</div>}
                    </div>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>限流模式</div>
                        {editable ? (
                            <RadioGroup aria-label="限流模式" theme="button" variant="primary-filled" value={rateLimit.type} onChange={(value) => updateMode(value as LimitType)}>
                                <Radio.Button value={LimitType.LOCAL}>{LimitTypeMap[LimitType.LOCAL]}</Radio.Button>
                                <Radio.Button value={LimitType.GLOBAL}>集群限流</Radio.Button>
                            </RadioGroup>
                        ) : (
                            <div className={shared.fieldValue}>{rateLimit.type === LimitType.GLOBAL ? '集群限流' : LimitTypeMap[LimitType.LOCAL]}</div>
                        )}
                    </div>
                </div>
            </div>
        </section>
    );

    const readonlyInterface = (ruleIdx: number) => {
        const trigger = rateLimit.rules[ruleIdx];
        const apis = normalizeRateLimitApis(trigger);
        return (
            <>
                {apis.map((api, index) => (
                    <div className={shared.tagRow} key={`rate-limit-api-${index}`}>
                        <span className={shared.tagPlain}>{api.protocol || 'HTTP'}</span>
                        <span className={`${shared.tagPlain} ${shared.tagMethod}`}>{api.method || '*'}</span>
                        <span className={shared.tagPlain}>{MatchTypeMap[api.path?.type as MatchType] || api.path?.type || '完全匹配'}</span>
                        <span className={shared.pathTag}>{api.path?.value || '-'}</span>
                    </div>
                ))}
            </>
        );
    };

    const matchInterface = (ruleIdx: number) => {
        const trigger = rateLimit.rules[ruleIdx];
        const apis = normalizeRateLimitApis(trigger);
        const updateApi = (apiIdx: number, patch: Partial<RateLimitAPI>) => {
            updateRule(ruleIdx, {
                apis: apis.map((api, index) => index === apiIdx ? {
                    ...api,
                    ...patch,
                    path: patch.path ? { ...api.path, ...patch.path } : api.path,
                } : api),
            });
        };
        const addInterface = () => {
            updateRule(ruleIdx, {
                apis: [...apis, defaultRateLimitAPI()],
            });
        };
        const removeInterface = (apiIdx: number) => {
            if (apis.length <= 1) return;
            updateRule(ruleIdx, {
                apis: apis.filter((_, index) => index !== apiIdx),
            });
        };
        return (
            <div className={shared.step} data-step="1">
                <div className={shared.stepTitle}>匹配接口<span className={shared.stepHint}>协议、方法和路径保存为当前子规则的 API 列表</span></div>
                <div className={shared.stepContent}>
                    {editable ? (
                        <>
                            <div className={styles.interfaceGrid}>
                                <div className={styles.gridHeader}>协议</div>
                                <div className={styles.gridHeader}>方法</div>
                                <div className={styles.gridHeader}>匹配类型</div>
                                <div className={styles.gridHeader}>接口路径</div>
                                <div className={styles.gridHeader}>操作</div>
                                {apis.map((api, apiIdx) => {
                                    const apiMatchType = api.path?.type || MatchType.EXACT;
                                    return (
                                        <React.Fragment key={`${ruleIdx}-api-${apiIdx}`}>
                                            <div className={styles.gridCell}>
                                                <Select
                                                    aria-label={`第 ${ruleIdx + 1} 条限流子规则第 ${apiIdx + 1} 个接口协议`}
                                                    options={RATE_LIMIT_PROTOCOL_OPTIONS}
                                                    value={api.protocol || 'HTTP'}
                                                    onChange={(value) => updateApi(apiIdx, { protocol: value as string })}
                                                />
                                            </div>
                                            <div className={styles.gridCell}>
                                                <Select
                                                    aria-label={`第 ${ruleIdx + 1} 条限流子规则第 ${apiIdx + 1} 个接口方法`}
                                                    options={RATE_LIMIT_HTTP_METHOD_OPTIONS}
                                                    value={api.method || '*'}
                                                    onChange={(value) => updateApi(apiIdx, { method: value as string })}
                                                />
                                            </div>
                                            <div className={styles.gridCell}>
                                                <Select
                                                    aria-label={`第 ${ruleIdx + 1} 条限流子规则第 ${apiIdx + 1} 个接口路径匹配类型`}
                                                    options={MatchTypeOption}
                                                    value={apiMatchType}
                                                    onChange={(value) => updateApi(apiIdx, { path: { ...api.path, type: value as MatchType } })}
                                                />
                                            </div>
                                            <div className={styles.gridCell}>
                                                {isTagInputMatchType(apiMatchType) ? (
                                                    <TagInput
                                                        aria-label={`第 ${ruleIdx + 1} 条限流子规则第 ${apiIdx + 1} 个接口路径`}
                                                        className={styles.monoInput}
                                                        value={commaStringToTags(api.path?.value || '')}
                                                        placeholder="输入后回车添加"
                                                        clearable
                                                        onChange={(value) => updateApi(apiIdx, { path: { ...api.path, value: tagsToCommaString(value as Array<string | number>), value_type: MatchValueType.TEXT } })}
                                                    />
                                                ) : (
                                                    <Input
                                                        aria-label={`第 ${ruleIdx + 1} 条限流子规则第 ${apiIdx + 1} 个接口路径`}
                                                        className={styles.monoInput}
                                                        value={api.path?.value || ''}
                                                        onChange={(value) => updateApi(apiIdx, { path: { ...api.path, value: value as string, value_type: MatchValueType.TEXT } })}
                                                    />
                                                )}
                                            </div>
                                            <div className={`${styles.gridCell} ${styles.actionCell}`}>
                                                <Popup trigger="hover" content="删除接口">
                                                    <Button shape="circle" variant="text" aria-label={`删除第 ${ruleIdx + 1} 条限流子规则第 ${apiIdx + 1} 个接口`} disabled={apis.length <= 1} onClick={() => removeInterface(apiIdx)}><CloseIcon /></Button>
                                                </Popup>
                                            </div>
                                        </React.Fragment>
                                    );
                                })}
                            </div>
                            <Button aria-label={`第 ${ruleIdx + 1} 条限流子规则新增接口`} className={styles.inlineAdd} variant="text" onClick={addInterface} icon={<AddIcon />}>新增接口</Button>
                        </>
                    ) : readonlyInterface(ruleIdx)}
                </div>
            </div>
        );
    };

    const matchCondition = (ruleIdx: number) => {
        const trigger = rateLimit.rules[ruleIdx];
        const args = trigger.arguments || [];
        const relation = trigger.matchMode || MatchLogic.AND;
        const rows: TrafficMatchConditionRow[] = args.map(arg => ({
            paramType: arg.type,
            paramKey: arg.key,
            matchType: arg.value?.type || MatchType.EXACT,
            valueType: arg.value?.value_type || MatchValueType.TEXT,
            matchValue: arg.value?.value || '',
        }));
        return (
            <div className={shared.step} data-step="2">
                <div className={shared.stepTitle}>
                    匹配条件
                    <span className={shared.stepHint}>
                        {relation === MatchLogic.OR ? '满足任一条件即进入限流规则' : '需同时满足全部条件'}
                    </span>
                </div>
                <div className={shared.stepContent}>
                    <TrafficMatchConditionEditor
                        contextLabel={`第 ${ruleIdx + 1} 条限流子规则`}
                        rows={rows}
                        editable={editable}
                        relation={relation}
                        relationEditable
                        onRelationChange={(value) => updateRule(ruleIdx, { matchMode: value as MatchLogic })}
                        paramTypeOptions={LimitArgumentsTypeOptions}
                        onRowChange={(index, row) => {
                            const current = args[index] || defaultMatchArgs();
                            updateMatch(ruleIdx, index, {
                                ...current,
                                type: row.paramType as LimitArgumentsType,
                                key: row.paramKey || '',
                                value: {
                                    ...current.value,
                                    type: row.matchType as MatchType,
                                    value: row.valueType === MatchValueType.PARAMETER ? '' : row.matchValue || '',
                                    value_type: row.valueType || MatchValueType.TEXT,
                                },
                            });
                        }}
                        onAdd={() => addMatch(ruleIdx)}
                        onRemove={(index) => removeMatch(ruleIdx, index)}
                    />
                </div>
            </div>
        );
    };

    const rateLimitBlock = (ruleIdx: number) => {
        const trigger = rateLimit.rules[ruleIdx];
        const scenario = detectQuotaScenario(trigger);
        const applyScenario = (next: QuotaScenario) => {
            const maxAmount = trigger.amounts?.[0]?.maxAmount || trigger.concurrencyAmount?.maxAmount || 1;
            if (next === 'CONCURRENCY') {
                updateRule(ruleIdx, { resource: RateLimitResource.Concurrency, concurrencyAmount: { maxAmount } });
                return;
            }
            updateRule(ruleIdx, {
                resource: next === 'TPM' ? RateLimitResource.Token : RateLimitResource.QPS,
                amounts: [{
                    validDuration: next === 'QPS' ? 1 : 60,
                    validDurationUnit: LimitAmountsValidationUnit.s,
                    maxAmount,
                }],
            });
        };
        return (
            <div className={shared.step} data-step="3">
                <div className={shared.stepTitle}>限流方式<span className={shared.stepHint}>设置统计指标、阈值窗口和阈值计算方式</span></div>
                <div className={shared.stepContent}>
                    <div className={styles.scenarioLabel}>计量场景</div>
                    <div className={styles.scenarioGrid}>
                        {quotaScenarios.map((item) => (
                            <Button
                                variant="text"
                                className={scenario === item.value ? styles.scenarioCardActive : styles.scenarioCard}
                                disabled={!editable}
                                key={item.value}
                                onClick={() => applyScenario(item.value)}
                                type="button"
                            >
                                <strong>{item.title}</strong>
                                <span>{item.description}</span>
                            </Button>
                        ))}
                    </div>
                    {trigger.resource === RateLimitResource.Token && (
                        <div className={styles.leaseExplanation}>
                            <strong>流式 TPM 使用 Quota Lease</strong>
                            <span>SDK 在模型请求前预留输入 Token + 最大输出 Token，流中只上报累计实际用量，结束时自动归还未生成部分；预留成功后不会在响应流中再次触发配额 429。</span>
                        </div>
                    )}
                    <div className={styles.scenarioLabel}>
                        {trigger.resource === RateLimitResource.Concurrency ? '并发上限' : '阈值窗口'}
                    </div>
                    {trigger.resource === RateLimitResource.Concurrency ? (
                        <div className={styles.singleMetricRow}>
                            <div className={styles.metricLabel}>最大并发数</div>
                            {editable
                                ? <InputNumber aria-label={`第 ${ruleIdx + 1} 条限流子规则最大并发数`} theme="normal" min={1} value={trigger.concurrencyAmount?.maxAmount ?? trigger.amounts?.[0]?.maxAmount ?? 1} onChange={(value) => updateRule(ruleIdx, { concurrencyAmount: { maxAmount: (value as number) ?? 1 } })} />
                                : <div className={shared.fieldValue}>{trigger.concurrencyAmount?.maxAmount ?? '-'}</div>}
                        </div>
                    ) : (
                        <div className={styles.windowGrid}>
                            <div className={styles.gridHeader}>窗口</div>
                            <div className={styles.gridHeader}>{trigger.resource === RateLimitResource.Token ? '最大 Token 数' : '最大请求数'}</div>
                            <div className={styles.gridHeader}>操作</div>
                            {(trigger.amounts || []).map((amount, index) => (
                                <React.Fragment key={`${ruleIdx}-amount-${index}`}>
                                    <div className={styles.gridCell}>
                                        {editable
                                            ? <InputNumber aria-label={`第 ${ruleIdx + 1} 条限流子规则第 ${index + 1} 个窗口时长`} theme="normal" min={1} suffix="秒" value={amount.validDuration} onChange={(value) => updateLimit(ruleIdx, index, { validDuration: (value as number) ?? 1, validDurationUnit: LimitAmountsValidationUnit.s })} />
                                            : <span>{amount.validDuration}{amount.validDurationUnit}</span>}
                                    </div>
                                    <div className={styles.gridCell}>
                                        {editable
                                            ? <InputNumber aria-label={`第 ${ruleIdx + 1} 条限流子规则第 ${index + 1} 个窗口${trigger.resource === RateLimitResource.Token ? "最大 Token 数" : "最大请求数"}`} theme="normal" min={1} suffix={trigger.resource === RateLimitResource.Token ? 'Token' : '次'} value={amount.maxAmount} onChange={(value) => updateLimit(ruleIdx, index, { maxAmount: (value as number) ?? 1 })} />
                                            : <span>{amount.maxAmount}</span>}
                                    </div>
                                    <div className={`${styles.gridCell} ${styles.actionCell}`}>
                                        {editable && (
                                            <Popup trigger="hover" content="删除窗口">
                                                <Button shape="circle" variant="text" aria-label={`删除第 ${ruleIdx + 1} 条限流子规则第 ${index + 1} 个窗口`} disabled={(trigger.amounts || []).length <= 1} onClick={() => removeLimit(ruleIdx, index)}><CloseIcon /></Button>
                                            </Popup>
                                        )}
                                    </div>
                                </React.Fragment>
                            ))}
                        </div>
                    )}
                    {editable && trigger.resource !== RateLimitResource.Concurrency && <Button aria-label={`第 ${ruleIdx + 1} 条限流子规则添加窗口`} className={styles.inlineAdd} variant="text" onClick={() => addLimit(ruleIdx)} icon={<AddIcon />}>添加窗口</Button>}
                    <div className={styles.inlineConfig}>
                        <span className={styles.metricLabel}>阈值计算合并</span>
                        <Switch aria-label={`第 ${ruleIdx + 1} 条限流子规则阈值计算合并`} value={trigger.regex_combine} disabled={!editable} onChange={(value) => updateRule(ruleIdx, { regex_combine: value as boolean })} />
                        <span className={styles.inlineHint}>{trigger.regex_combine ? '按匹配请求合并计算限流' : '按匹配请求单独计算限流'}</span>
                    </div>
                </div>
            </div>
        );
    };

    const limitActionBlock = (ruleIdx: number) => {
        const trigger = rateLimit.rules[ruleIdx];
        const isGlobal = rateLimit.type === LimitType.GLOBAL;
        const action = isGlobal ? LimitAction.REJECT : trigger.action;
        const isReject = action !== LimitAction.UNIRATE;
        const hasCustomResponse = Boolean(trigger.customResponse?.body);
        const isTokenLease = trigger.resource === RateLimitResource.Token;
        return (
            <div className={shared.step} data-step="4">
                <div className={shared.stepTitle}>{isTokenLease ? '准入结果' : '限流效果'}<span className={shared.stepHint}>{isTokenLease ? '仅在模型请求开始前判断配额' : '超过阈值后的处理方式'}</span></div>
                <div className={shared.stepContent}>
                    {editable && !isGlobal && (
                        <div className={styles.resourceSwitch}>
                            <RadioGroup aria-label={`第 ${ruleIdx + 1} 条限流子规则限流效果`} theme="button" variant="primary-filled" value={trigger.action} onChange={(value) => updateRule(ruleIdx, { action: value as LimitAction })}>
                                <Radio.Button value={LimitAction.REJECT}>{LimitActionMap[LimitAction.REJECT]}</Radio.Button>
                                <Radio.Button value={LimitAction.UNIRATE}>排队等待</Radio.Button>
                            </RadioGroup>
                        </div>
                    )}
                    <div className={styles.strategyNote}>
                        <strong>{isTokenLease ? '请求前校验' : (isReject ? '超额请求处理' : '排队策略')}</strong>
                        <span>{isTokenLease ? '预留不足时不发起模型调用；预留成功后允许流式响应完整结束。' : (isGlobal ? '集群配额不足时立即拒绝当前请求。' : (isReject ? '请求超过当前窗口阈值时立即拒绝。' : '超额请求进入队列，等待配额释放后继续。'))}</span>
                    </div>
                    {isGlobal && (
                        <div className={styles.failoverRow}>
                            <span className={styles.metricLabel}>失败处理策略</span>
                            {editable ? (
                                <RadioGroup aria-label={`第 ${ruleIdx + 1} 条限流子规则失败处理策略`} theme="button" variant="primary-filled" value={trigger.failover} onChange={(value) => updateRule(ruleIdx, { failover: value as LimitFailover })}>
                                    <Radio.Button value={LimitFailover.FAILOVER_LOCAL}>{LimitFailoverMap[LimitFailover.FAILOVER_LOCAL]}</Radio.Button>
                                    <Radio.Button value={LimitFailover.FAILOVER_PASS}>{LimitFailoverMap[LimitFailover.FAILOVER_PASS]}</Radio.Button>
                                </RadioGroup>
                            ) : (
                                <span>{LimitFailoverMap[trigger.failover] || trigger.failover}</span>
                            )}
                        </div>
                    )}
                    {!isTokenLease && <div className={styles.responseField}>
                        <div className={styles.responseHeader}>
                            <span className={styles.metricLabel}>自定义响应</span>
                            {editable && (
                                <Switch
                                    aria-label={`第 ${ruleIdx + 1} 条限流子规则自定义响应`}
                                    value={hasCustomResponse}
                                    onChange={(value) => updateRule(ruleIdx, { customResponse: value ? { body: trigger.customResponse?.body || '{"code":429,"msg":"rate limited"}' } : undefined })}
                                />
                            )}
                        </div>
                        {editable && hasCustomResponse ? (
                            <Textarea aria-label={`第 ${ruleIdx + 1} 条限流子规则自定义响应内容`} autosize={{ minRows: 4 }} value={trigger.customResponse?.body} onChange={(value) => updateRule(ruleIdx, { customResponse: { ...trigger.customResponse, body: value as string } })} />
                        ) : (
                            <div className={`${shared.fieldValue} ${hasCustomResponse ? shared.mono : ''}`}>{trigger.customResponse?.body || '无自定义响应 — 返回默认 429'}</div>
                        )}
                    </div>}
                </div>
            </div>
        );
    };

    const renderRateLimitRule = (ruleIdx: number) => {
        const trigger = rateLimit.rules[ruleIdx];
        const collapsed = collapsedRuleIndexes.has(ruleIdx);
        return (
            <div className={`${shared.policy} ${collapsed ? shared.policyCollapsed : ''}`} key={ruleIdx}>
                <div className={shared.policyHead} onClick={() => toggleRuleCollapsed(ruleIdx)}>
                    <div className={shared.policyHeadMain}>
                        <span className={shared.caret}><ChevronRightIcon /></span>
                        <div>
                            <div className={shared.policyIndex}>规则 [{ruleIdx + 1}]</div>
                            <div className={shared.policySummary}>{describeRuleSummary(trigger, rateLimit.type)}</div>
                        </div>
                    </div>
                    <div className={shared.policyHeadActions} onClick={(event) => event.stopPropagation()}>
                        <Tag variant="light">{describeRuleThreshold(trigger)}</Tag>
                        {editable && (
                            <Popup trigger="hover" content="删除规则">
                                <Button shape="circle" variant="text" aria-label={`删除规则 ${ruleIdx + 1}`} disabled={rateLimit.rules.length <= 1} onClick={() => removeRule(ruleIdx)}><CloseIcon /></Button>
                            </Popup>
                        )}
                    </div>
                </div>
                <div className={shared.policyBody}>
                    {matchInterface(ruleIdx)}
                    {matchCondition(ruleIdx)}
                    {rateLimitBlock(ruleIdx)}
                    {limitActionBlock(ruleIdx)}
                </div>
            </div>
        );
    };

    const renderPublishForm = (
        <>
            {editorState.publishView && (
                <PublishForm
                    ruleId={viewRule?.id || ''}
                    ruleName={viewRule?.name || ''}
                    resource={PolicySourceType.RateLimitRules}
                    visible={editorState.publishView}
                    close={() => setEditorState(prev => ({ ...prev, publishView: false }))}
                />
            )}
        </>
    );

    const renderStickyTool = (
        <StickyTool style={{ zIndex: 1000 }} placement='right-bottom' offset={[-10, 200]}>
            <StickyItem
                label=""
                icon={!editable
                    ? <RuleStickyAction label="编辑" icon={<Edit1Icon />} onClick={() => setEditorState(prev => ({ ...prev, editable: true }))} />
                    : <RuleStickyAction label="保存" icon={<SaveIcon />} onClick={() => form.submit()} />}
            />
            {editable && (
                <StickyItem label="" icon={<RuleStickyAction label="撤销" icon={<RollbackIcon />} onClick={() => {
                    if (op === 'create') {
                        refresh(true);
                    }
                    resetCurRule(viewRule ?? editRule ?? null);
                    setEditorState(prev => ({ ...prev, editable: false }));
                }} />} />
            )}
            {!editable && (
                <StickyItem label="" icon={<RuleStickyAction label="发布" icon={<RocketIcon />} onClick={() => setEditorState(prev => ({ ...prev, publishView: true }))} />} />
            )}
        </StickyTool>
    );

    return (
        <div className={styles.editorBody}>
            {visible && (
                <Form form={form} onSubmit={onSubmit} layout="vertical" labelAlign="left" labelWidth={120} colon>
                    <div className={styles.rateLimitEditorShell}>
                        <div className={styles.formPane}>
                            {ruleBaseInfo}
                            {scopeInfo}
                            <section className={shared.section}>
                                <div className={shared.sectionHeader}>
                                    <span>限流规则</span>
                                    <span className={shared.countTag}>{rateLimit.rules.length} 条</span>
                                </div>
                                <div className={shared.sectionBody}>
                                    <div className={shared.ruleList}>
                                        {rateLimit.rules.map((_, ruleIdx) => renderRateLimitRule(ruleIdx))}
                                    </div>
                                    {editable && <Button className={shared.addRuleButton} variant="dashed" icon={<AddIcon />} onClick={addRule}>添加规则</Button>}
                                </div>
                            </section>
                            {op === 'view' && rateLimit.callee.namespace && rateLimit.callee.service && (
                                <QuotaLeaseRuntime namespace={rateLimit.callee.namespace} service={rateLimit.callee.service} />
                            )}
                        </div>
                    </div>
                    {renderPublishForm}
                    {renderStickyTool}
                </Form>
            )}
        </div>
    );
};

export default React.memo(RateLimitEditor);
