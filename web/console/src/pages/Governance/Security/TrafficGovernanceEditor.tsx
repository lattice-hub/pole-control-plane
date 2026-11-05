import React from 'react';
import { Button, Dialog, Form, Input, InputNumber, Radio, RadioGroup, Select, Space, StickyTool, Switch, Tag, Textarea } from 'components/Fluent';
import type { FormProps } from 'components/Fluent';
import { AddIcon, CheckIcon, ChevronRightIcon, CloseIcon, Edit1Icon, RocketIcon, RollbackIcon, SaveIcon } from 'components/Fluent/icons';

import { cleanNamespacePage, listAllNamespaces, selectNamespace } from 'modules/namespace';
import { cleanServicePage, listAllServices, selectService } from 'modules/discovery/service';
import { useAppDispatch, useAppSelector } from 'modules/store';
import { NamespaceView } from 'services/namespace';
import { ServiceView } from 'services/service';
import { PolicySourceType } from 'services/auth_policy';
import {
    InterfaceProtocol,
    MatchLogic,
    MatchType,
    MatchTypeOption,
    MatchValueType,
    Op,
} from 'services/types';
import {
    buildSecurityAuthenticationForMode,
    buildSecurityPoliciesFromView,
    defaultProtectedInterface,
    defaultSecurityArgument,
    defaultSecurityMatchRule,
    defaultManagedCaller,
    listTypeText,
    getApiProtocolPresentation,
    normalizeSecurityMatchRule,
    normalizeManagedCaller,
    normalizeSecurityViewOrder,
    normalizeSecurityViewRules,
    readSecurityAction,
    readSecurityAuthMode,
    SecurityListType,
    SecuritySubRuleKind,
    SecurityViewRule,
    securityMatchSourceOptions,
    resetApiScopeForProtocol,
    validateSecurityView,
} from './trafficSecurityEditorUtils';
import {
    createTrafficGovernanceRules,
    defaultTrafficGovernanceRule,
    defaultTrafficSecurityRule,
    describeOneTrafficGovernanceRule,
    MirrorRule,
    MockRule,
    modifyTrafficGovernanceRules,
    TrafficApiScope,
    TrafficGovernanceAuthResource,
    TrafficGovernanceKind,
    TrafficGovernanceKindLabel,
    TrafficGovernanceReleaseResource,
    TrafficMatchRule,
    TrafficMirror,
    TrafficMock,
    TrafficGovernanceRule,
    TrafficSecurityAction,
    TrafficSecurityActionMap,
    TrafficSecurityAuthMode,
    TrafficSecurityPolicy,
    TrafficSecurityRule,
} from 'services/traffic_governance';
import {
    buildMockRulesForSubmit,
    defaultMockArgument,
    defaultMockCaller,
    defaultMockMatchValue,
    defaultMockSubRule,
    durationToMs,
    extractMockCaller,
    MockCallerScope,
    mockCallerScopeText,
    mockMatchSourceOptions,
    msToDuration,
    normalizeMockMatchRule,
    normalizeMockRule,
    normalizeMockRules,
    validateMockView,
} from './trafficMockEditorUtils';
import {
    buildMirrorRulesForSubmit,
    callerScopeText,
    defaultMirrorSubRule,
    defaultMirrorCaller,
    extractMirrorCaller,
    MirrorCallerScope,
    MirrorViewRule,
    mirrorScopeLabel,
    normalizeMirrorCallee,
    normalizeMirrorRules,
    removeCallerServiceArguments,
    validateMirrorView,
} from './trafficMirrorEditorUtils';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import PublishForm from '../RuleRelease/PublishForm';
import RuleStickyAction from '../RuleRelease/RuleStickyAction';
import RuleLabelField from '../shared/RuleLabelField';
import CollapsibleSection from '../shared/CollapsibleSection';
import ServiceScopeSection from '../shared/ServiceScopeSection';
import { GovernanceServiceContext } from '../shared/serviceContext';
import { useRuleNamespace } from '../shared/ruleNamespace';
import TrafficMatchConditionEditor, { TrafficMatchConditionRow } from '../shared/TrafficMatchConditionEditor';
import MockHeaderEditor from './MockHeaderEditor';
import shared from '../shared/governance.module.less';
import style from './index.module.less';

const { FormItem } = Form;
const { StickyItem } = StickyTool;

const methodOptions = ['GET', 'POST', 'PUT', 'DELETE', '*', 'PATCH', 'HEAD', 'OPTIONS', 'ANY'].map((item) => ({ label: item, value: item }));
const protocolOptions = [
    { label: 'HTTP', value: InterfaceProtocol.HTTP },
    { label: 'GRPC', value: InterfaceProtocol.GRPC },
    { label: 'DUBBO', value: InterfaceProtocol.DUBBO },
];
interface TrafficGovernanceEditorProps {
    kind: TrafficGovernanceKind;
    op: Op;
    data?: TrafficGovernanceRule;
    visible: boolean;
    refresh: (close: boolean) => void;
    serviceContext?: GovernanceServiceContext;
}

const readonlyItem = (label: string, value: React.ReactNode, full = false) => (
    <div className={`${shared.field}${full ? ` ${shared.full}` : ''}`}>
        <div className={shared.fieldLabel}>{label}</div>
        <div className={shared.fieldValue}>{value || '-'}</div>
    </div>
);

const text = (value?: string | number | boolean) => {
    if (value === undefined || value === null || value === '') return '-';
    return String(value);
};

const matchText = (value?: { type?: string; value?: string; value_type?: string }) => {
    if (!value) return '-';
    return [value.type, value.value, value.value_type].filter(Boolean).join(' / ') || '-';
};

const apiText = (api?: TrafficApiScope) => {
    if (!api) return '-';
    return `${text(api.protocol)} ${text(api.method)} ${matchText(api.path)}`;
};

const durationText = (value?: string | { seconds?: number | string; nanos?: number }) => {
    if (!value) return '-';
    if (typeof value === 'string') return value;
    return `${value.seconds ?? 0}s`;
};

const renderMirrorMatchRule = (match?: TrafficMatchRule) => {
    const current = removeCallerServiceArguments(match);
    const args = current.arguments || [];
    return (
        <div className={style.conditionStack}>
            <div className={style.conditionMeta}>
                <Tag variant="light-outline">{text(current.matchMode || 'AND')}</Tag>
                <span className={style.repeatMetaHint}>{current.matchMode === MatchLogic.OR ? '满足任一条件' : '需同时满足全部条件'}</span>
            </div>
            {args.length ? args.map((arg, index) => (
                <div className={shared.tagRow} key={`${arg.type}-${arg.key}-${index}`}>
                    <span className={`${shared.tagPlain} ${shared.tagMethod}`}>{text(arg.type)}</span>
                    <span className={shared.chip}>
                        <span className={shared.chipKey}>{text(arg.key)}</span>
                        <span className={shared.chipValue}>{matchText(arg.value)}</span>
                    </span>
                </div>
            )) : <div className={shared.emptyLine}>全部请求条件</div>}
        </div>
    );
};

const renderSecurityMatchRule = (match?: TrafficMatchRule) => {
    const current = normalizeSecurityMatchRule(match);
    const args = current.arguments || [];
    return (
        <div className={style.conditionStack}>
            <div className={style.conditionMeta}>
                <Tag variant="light-outline">{text(current.matchMode || 'AND')}</Tag>
                <span className={style.repeatMetaHint}>
                    {current.matchMode === MatchLogic.OR ? '满足任一条件即命中名单' : '需同时满足全部条件'}
                </span>
            </div>
            {args.map((arg, index) => (
                <div className={shared.tagRow} key={`${arg.type}-${arg.key}-${index}`}>
                    <span className={`${shared.tagPlain} ${shared.tagMethod}`}>{text(arg.type)}</span>
                    <span className={shared.chip}>
                        <span className={shared.chipKey}>{text(arg.key)}</span>
                        <span className={shared.chipValue}>{matchText(arg.value)}</span>
                    </span>
                </div>
            ))}
        </div>
    );
};

const renderMockMatchRule = (match?: TrafficMatchRule) => {
    const current = normalizeMockMatchRule(match);
    const args = current.arguments || [];
    return (
        <div className={style.conditionStack}>
            <div className={style.conditionMeta}>
                <Tag variant="light-outline">{text(current.matchMode || 'AND')}</Tag>
                <span className={style.repeatMetaHint}>
                    {current.matchMode === MatchLogic.OR ? '满足任一条件即 Mock' : '需同时满足全部条件'}
                </span>
            </div>
            {args.map((arg, index) => (
                <div className={shared.tagRow} key={`${arg.type}-${arg.key}-${index}`}>
                    <span className={`${shared.tagPlain} ${shared.tagMethod}`}>{text(arg.type)}</span>
                    <span className={shared.chip}>
                        <span className={shared.chipKey}>{text(arg.key)}</span>
                        <span className={shared.chipValue}>{matchText(arg.value)}</span>
                    </span>
                </div>
            ))}
        </div>
    );
};

const renderRecord = (record?: Record<string, string | { type?: string; value?: string; value_type?: string }>) => {
    const entries = Object.entries(record || {});
    if (!entries.length) return <span>-</span>;
    return (
        <div className={shared.labelDisplay}>
            {entries.map(([key, value]) => (
                <span className={shared.chip} key={key}>
                    <span className={shared.chipKey}>{key}</span>
                    <span className={shared.chipValue}>{typeof value === 'string' ? value : matchText(value)}</span>
                </span>
            ))}
        </div>
    );
};

const rejectEffectText = (policy: TrafficSecurityPolicy) => {
    const effect = policy.reject_effect;
    if (!effect) return '-';
    return [effect.status_code, effect.code, effect.message].filter(Boolean).join(' / ') || '-';
};

const securityPolicySummary = (policy: TrafficSecurityPolicy) => {
    const matchCount = policy.traffic_match_rule?.arguments?.length || 0;
    const action = TrafficSecurityActionMap[String(policy.action)] || '-';
    const apis = policy.apis?.length ? policy.apis : (policy.api ? [policy.api] : []);
    const api = apis.length ? `${apis.length} 个接口` : apiText(undefined);
    const rejectEffect = rejectEffectText(policy);
    return `${matchCount} 个匹配条件 / ${api} / ${action}${readSecurityAction(policy.action) === TrafficSecurityAction.DENY ? ` / ${rejectEffect}` : ''}`;
};

const renderApiScope = (api?: TrafficApiScope) => {
    if (!api) return <span className={shared.emptyLine}>全部接口</span>;
    return (
        <div className={shared.tagRow}>
            {api.protocol && <span className={shared.tagPlain}>{api.protocol}</span>}
            {api.method && <span className={`${shared.tagPlain} ${shared.tagMethod}`}>{api.method}</span>}
            {api.path?.type && <span className={shared.tagPlain}>{api.path.type}</span>}
            <span className={shared.pathTag}>{api.path?.value || '/'}</span>
        </div>
    );
};

const renderApiScopeList = (apis?: TrafficApiScope[]) => {
    const rows = apis?.length ? apis : [];
    if (!rows.length) return renderApiScope(undefined);
    return (
        <div className={style.scopeTagList}>
            {rows.map((api, index) => (
                <React.Fragment key={`api-scope-${index}`}>
                    {renderApiScope(api)}
                </React.Fragment>
            ))}
        </div>
    );
};

const renderVerdictBlock = (policy: TrafficSecurityPolicy) => {
    const isDeny = readSecurityAction(policy.action) === TrafficSecurityAction.DENY;
    const effect = policy.reject_effect;
    return (
        <div className={`${shared.verdict} ${isDeny ? shared.verdictDeny : shared.verdictAllow}`}>
            <div className={shared.verdictIcon}>{isDeny ? <CloseIcon /> : <CheckIcon />}</div>
            <div>
                <div className={shared.verdictHead}>{TrafficSecurityActionMap[String(policy.action)] || (isDeny ? '拒绝' : '放通')}</div>
                <div className={shared.verdictDesc}>
                    {isDeny ? '命中该策略的请求将被拦截，并返回下方响应信息' : '命中该策略的请求允许继续访问目标接口'}
                </div>
            </div>
            {isDeny && (
                <div className={shared.verdictMeta}>
                    <div>
                        <div className={shared.verdictMetaLabel}>状态码</div>
                        <div className={`${shared.verdictMetaValue} ${shared.mono}`}>{text(effect?.status_code ?? 403)}</div>
                    </div>
                    <div>
                        <div className={shared.verdictMetaLabel}>Code</div>
                        <div className={`${shared.verdictMetaValue} ${shared.mono}`}>{text(effect?.code)}</div>
                    </div>
                    <div>
                        <div className={shared.verdictMetaLabel}>文案</div>
                        <div className={shared.verdictMetaValue}>{text(effect?.message)}</div>
                    </div>
                </div>
            )}
        </div>
    );
};

const defaultMatchValue = () => ({
    type: MatchType.EXACT,
    value: '',
    value_type: MatchValueType.TEXT,
});

const defaultArgument = () => ({
    type: 'HEADER',
    key: '',
    value: defaultMatchValue(),
});

const normalizeMatchRule = (match?: TrafficMatchRule): TrafficMatchRule => ({
    matchMode: match?.matchMode || MatchLogic.AND,
    randomPercent: match?.randomPercent ?? 100,
    arguments: match?.arguments?.length ? match.arguments : [defaultArgument()],
});

export const trafficRuleCount = (kind: TrafficGovernanceKind, rule?: TrafficGovernanceRule) => {
    if (!rule) return 0;
    if (kind === 'security') return ((rule as TrafficSecurityRule).policies || []).length;
    return ((rule as any).rules || []).length;
};

export const trafficRuleSummary = (kind: TrafficGovernanceKind, rule?: TrafficGovernanceRule) => {
    if (!rule) return '-';
    const target = rule.target_service || { namespace: '', service: '' };
    if (kind === 'security') {
        const security = rule as TrafficSecurityRule;
        const first = security.policies?.[0];
        const action = TrafficSecurityActionMap[String(first?.action || '')] || '-';
        return `${target.namespace || '-'}/${target.service || '-'} / 首条策略 ${action}`;
    }
    if (kind === 'mirror') {
        const first = (rule as any).rules?.[0];
        const caller = extractMirrorCaller(rule as TrafficMirror);
        const callee = normalizeMirrorCallee(rule);
        return first ? `${callerScopeText(caller)} -> ${mirrorScopeLabel(callee)} / 镜像到 ${first.destination?.namespace || '-'}/${first.destination?.service || '-'} / ${first.mirror_percent ?? 0}%` : '-';
    }
    const first = normalizeMockRule((rule as TrafficMock).rules?.[0]);
    const caller = extractMockCaller(rule as TrafficMock);
    return first ? `${mockCallerScopeText(caller)} -> ${target.namespace || '-'}/${target.service || '-'} / Code ${first.response?.code || '200'}` : '-';
};

const TrafficGovernanceEditor: React.FC<TrafficGovernanceEditorProps> = ({ kind, op, data, visible, refresh, serviceContext }) => {
    const ruleNamespace = useRuleNamespace();
    const [form] = Form.useForm();
    const dispatch = useAppDispatch();
    const namespaceState = useAppSelector(selectNamespace);
    const serviceState = useAppSelector(selectService);
    const [rule, setRule] = React.useState<TrafficGovernanceRule>(() => defaultTrafficGovernanceRule(kind));
    const [editable, setEditable] = React.useState(op === 'create');
    const [publishVisible, setPublishVisible] = React.useState(false);
    const [basicInfoCollapsed, setBasicInfoCollapsed] = React.useState(false);
    const [securityServiceInfoCollapsed, setSecurityServiceInfoCollapsed] = React.useState(false);
    const [securityAuthenticationCollapsed, setSecurityAuthenticationCollapsed] = React.useState(false);
    const [serviceScopeCollapsed, setServiceScopeCollapsed] = React.useState(false);
    const [collapsedSecurityPolicyIndexes, setCollapsedSecurityPolicyIndexes] = React.useState<Set<number>>(() => new Set());
    const [collapsedMirrorRuleIndexes, setCollapsedMirrorRuleIndexes] = React.useState<Set<number>>(() => new Set());
    const [collapsedMockRuleIndexes, setCollapsedMockRuleIndexes] = React.useState<Set<number>>(() => new Set());
    const [securityViewRules, setSecurityViewRules] = React.useState<SecurityViewRule[]>(() => normalizeSecurityViewRules(defaultTrafficSecurityRule()));
    const [reloadVersion, setReloadVersion] = React.useState(0);
    const [mirrorCaller, setMirrorCaller] = React.useState<MirrorCallerScope>(() => defaultMirrorCaller());
    const [mockCaller, setMockCaller] = React.useState<MockCallerScope>(() => defaultMockCaller());
    const [mockBodyDialog, setMockBodyDialog] = React.useState<{ visible: boolean; index: number }>({ visible: false, index: -1 });

    React.useEffect(() => {
        dispatch(listAllNamespaces());
        dispatch(listAllServices());
        return () => {
            dispatch(cleanNamespacePage());
            dispatch(cleanServicePage());
        };
    }, [dispatch]);

    React.useEffect(() => {
        if (!visible) {
            setBasicInfoCollapsed(false);
            setSecurityServiceInfoCollapsed(false);
            setSecurityAuthenticationCollapsed(false);
            return;
        }
        setBasicInfoCollapsed(false);
        setSecurityServiceInfoCollapsed(false);
        setSecurityAuthenticationCollapsed(false);
        setEditable(op === 'create');
        setServiceScopeCollapsed(false);
        setCollapsedSecurityPolicyIndexes(new Set());
        setCollapsedMirrorRuleIndexes(new Set());
        setCollapsedMockRuleIndexes(new Set());
        setMockBodyDialog({ visible: false, index: -1 });
        const load = async () => {
            const loaded = data?.id && op !== 'create'
                ? await describeOneTrafficGovernanceRule(kind, data.id)
                : (data || defaultTrafficGovernanceRule(kind));
            const endpoint = serviceContext
                ? { namespace: serviceContext.namespace, service: serviceContext.service }
                : undefined;
            let next = loaded;
            if (op === 'create' && endpoint) {
                if (kind === 'mirror') {
                    next = {
                        ...loaded,
                        ...(serviceContext?.role === 'caller' ? { caller: endpoint } : { callee: endpoint }),
                    } as TrafficMirror;
                } else if (kind === 'mock') {
                    next = {
                        ...loaded,
                        ...(serviceContext?.role === 'caller' ? { caller: endpoint } : { target_service: endpoint }),
                    } as TrafficMock;
                } else {
                    next = { ...loaded, target_service: endpoint } as TrafficSecurityRule;
                }
            }
            const targetService = kind === 'mirror'
                ? normalizeMirrorCallee(next)
                : next.target_service || { namespace: '', service: '' };
            const normalized = {
                ...next,
                ...(kind === 'mirror' ? { callee: targetService } : { target_service: targetService }),
                ...(kind === 'mirror' ? { rules: normalizeMirrorRules(next as TrafficMirror) } : {}),
                ...(kind === 'mock' ? { rules: normalizeMockRules(next as TrafficMock) } : {}),
            } as TrafficGovernanceRule;
            const consoleRule = normalized;
            setRule(consoleRule);
            if (kind === 'security') {
                const security = consoleRule as TrafficSecurityRule;
                setSecurityViewRules(normalizeSecurityViewRules(security));
            }
            if (kind === 'mirror') {
                setMirrorCaller(extractMirrorCaller(next as TrafficMirror));
            }
            if (kind === 'mock') {
                setMockCaller(extractMockCaller(next as TrafficMock));
            }
            form.setFieldsValue({
                name: next.name,
                targetNamespace: targetService.namespace,
                targetService: targetService.service,
                description: next.description,
                priority: next.priority ?? 0,
                enable: next.enable ?? true,
            });
        };
        load().catch((error) => {
            openErrNotification('请求失败', `获取${TrafficGovernanceKindLabel[kind]}详情失败: ${(error as Error).message}`);
        });
    }, [data, form, kind, op, reloadVersion, visible, serviceContext?.namespace, serviceContext?.service, serviceContext?.role]);

    const namespaceOptions = (namespaceState.datas || []).map((ns: NamespaceView) => ({ label: ns.name, value: ns.name }));
    const currentTarget = rule.target_service || { namespace: '', service: '' };
    const mirrorCallee = normalizeMirrorCallee(rule);
    const serviceOptions = (serviceState.datas || [])
        .filter((svc: ServiceView) => !currentTarget.namespace || currentTarget.namespace === '*' || svc.namespace === currentTarget.namespace)
        .map((svc: ServiceView) => ({ label: svc.name, value: svc.name }));
    const mirrorCallerNamespaceOptions = [{ label: '全部命名空间', value: '*' }, ...namespaceOptions];
    const mirrorCallerServiceOptions = [
        { label: '全部服务', value: '*' },
        ...(serviceState.datas || [])
            .filter((svc: ServiceView) => mirrorCaller.namespace && mirrorCaller.namespace !== '*' && svc.namespace === mirrorCaller.namespace)
            .map((svc: ServiceView) => ({ label: svc.name, value: svc.name })),
    ];
    const mirrorCalleeNamespaceOptions = [{ label: '全部命名空间', value: '*' }, ...namespaceOptions];
    const mirrorCalleeServiceOptions = [
        { label: '全部服务', value: '*' },
        ...(serviceState.datas || [])
            .filter((svc: ServiceView) => mirrorCallee.namespace && mirrorCallee.namespace !== '*' && svc.namespace === mirrorCallee.namespace)
            .map((svc: ServiceView) => ({ label: svc.name, value: svc.name })),
    ];
    const mockCallerNamespaceOptions = [{ label: '全部命名空间', value: '*' }, ...namespaceOptions];
    const mockCallerServiceOptions = [
        { label: '全部服务', value: '*' },
        ...(serviceState.datas || [])
            .filter((svc: ServiceView) => mockCaller.namespace && mockCaller.namespace !== '*' && svc.namespace === mockCaller.namespace)
            .map((svc: ServiceView) => ({ label: svc.name, value: svc.name })),
    ];
    const securityRule = rule as TrafficSecurityRule;
    const storedSecurityAuthMode = readSecurityAuthMode(securityRule);
    // Console no longer creates rule-level CUSTOM_HEADER credentials. Existing
    // records are edited as the original per-policy request-match model.
    const securityAuthMode = storedSecurityAuthMode === TrafficSecurityAuthMode.CUSTOM_HEADER
        ? TrafficSecurityAuthMode.LEGACY_REQUEST_MATCH
        : storedSecurityAuthMode;
    const securityRuleForValidation = storedSecurityAuthMode === securityAuthMode
        ? securityRule
        : { ...securityRule, authentication: buildSecurityAuthenticationForMode(securityAuthMode) };
    const securityValidationErrors = React.useMemo(() => kind === 'security' ? validateSecurityView(securityRuleForValidation, securityViewRules) : [], [kind, securityRuleForValidation, securityViewRules]);
    const managedCallerOptions = React.useMemo(() => [...(serviceState.datas || [])]
        .sort((left, right) => `${left.namespace}/${left.name}`.localeCompare(`${right.namespace}/${right.name}`))
        .map((svc: ServiceView) => ({
            label: `${svc.namespace}/${svc.name}`,
            value: JSON.stringify([svc.namespace, svc.name]),
        })), [serviceState.datas]);
    const mirrorRules = React.useMemo(() => normalizeMirrorRules(rule as TrafficMirror), [rule]);
    const mirrorValidationErrors = React.useMemo(() => kind === 'mirror' ? validateMirrorView(rule, mirrorRules, mirrorCaller) : [], [kind, rule, mirrorRules, mirrorCaller]);
    const mockRules = React.useMemo(() => normalizeMockRules(rule as TrafficMock), [rule]);
    const mockValidationErrors = React.useMemo(() => kind === 'mock' ? validateMockView(rule, mockRules, mockCaller) : [], [kind, rule, mockRules, mockCaller]);

    const setSecurityPolicy = (index: number, updater: (policy: TrafficSecurityPolicy) => TrafficSecurityPolicy) => {
        setRule((prev) => {
            const security = prev as TrafficSecurityRule;
            const policies = [...(security.policies || [])];
            policies[index] = updater(policies[index] || defaultTrafficSecurityRule().policies[0]);
            return { ...security, policies };
        });
    };

    const setSecurityAuthMode = (mode: TrafficSecurityAuthMode) => {
        setRule((prev) => {
            const current = prev as TrafficSecurityRule;
            return {
                ...current,
                authentication: buildSecurityAuthenticationForMode(mode),
            };
        });
        if (mode === TrafficSecurityAuthMode.MANAGED_IDENTITY) {
            setSecurityViewRules((prev) => prev.map((item) => ({ ...item, managedCaller: normalizeManagedCaller(item.managedCaller) })));
        }
    };

    const encodeManagedCallers = (item: SecurityViewRule) => (normalizeManagedCaller(item.managedCaller).callers || [])
        .map((caller) => JSON.stringify([caller.namespace || '', caller.service || '']));

    const decodeManagedCallers = (values: unknown): Array<{ namespace?: string; service?: string }> => (
        Array.isArray(values) ? values : []
    ).flatMap((value) => {
        try {
            const [namespace, service] = JSON.parse(String(value));
            return [{ namespace, service }];
        } catch {
            return [];
        }
    });

    const setSecurityViewRule = (index: number, updater: (item: SecurityViewRule) => SecurityViewRule) => {
        setSecurityViewRules((prev) => {
            const next = [...prev];
            next[index] = updater(next[index]);
            return normalizeSecurityViewOrder(next);
        });
    };

    const removeSecurityViewRule = (index: number) => {
        setSecurityViewRules((prev) => normalizeSecurityViewOrder(prev.filter((_, idx) => idx !== index)));
    };

    const toggleSecurityPolicyCollapsed = (index: number) => {
        setCollapsedSecurityPolicyIndexes((prev) => {
            const next = new Set(prev);
            if (next.has(index)) {
                next.delete(index);
            } else {
                next.add(index);
            }
            return next;
        });
    };

    const toggleMockRuleCollapsed = (index: number) => {
        setCollapsedMockRuleIndexes((prev) => {
            const next = new Set(prev);
            if (next.has(index)) {
                next.delete(index);
            } else {
                next.add(index);
            }
            return next;
        });
    };

    const toggleMirrorRuleCollapsed = (index: number) => {
        setCollapsedMirrorRuleIndexes((prev) => {
            const next = new Set(prev);
            if (next.has(index)) {
                next.delete(index);
            } else {
                next.add(index);
            }
            return next;
        });
    };

    const setMirrorRule = (index: number, updater: (item: MirrorViewRule) => MirrorViewRule) => {
        setRule((prev) => {
            const mirror = prev as TrafficMirror;
            const rules = normalizeMirrorRules(mirror);
            rules[index] = updater(rules[index] || defaultMirrorSubRule());
            return { ...mirror, rules };
        });
    };

    const setMockRule = (index: number, updater: (item: MockRule) => MockRule) => {
        setRule((prev) => {
            const mock = prev as TrafficMock;
            const rules = normalizeMockRules(mock);
            rules[index] = normalizeMockRule(updater(rules[index] || defaultMockSubRule()));
            return { ...mock, rules };
        });
    };

    const renderMirrorMatchRuleEditor = (ruleIndex: number, match: TrafficMatchRule | undefined, onChange: (next: TrafficMatchRule) => void) => {
        const current = removeCallerServiceArguments(normalizeMatchRule(match));
        const args = current.arguments || [];
        const updateArg = (index: number, updater: (arg: NonNullable<TrafficMatchRule['arguments']>[number]) => NonNullable<TrafficMatchRule['arguments']>[number]) => {
            const nextArgs = [...args];
            nextArgs[index] = updater(nextArgs[index] || defaultArgument());
            onChange({ matchMode: current.matchMode || MatchLogic.AND, arguments: nextArgs });
        };
        const rows: TrafficMatchConditionRow[] = args.map(arg => ({
            paramType: arg.type,
            paramKey: arg.key,
            matchType: arg.value?.type || MatchType.EXACT,
            valueType: arg.value?.value_type || MatchValueType.TEXT,
            matchValue: arg.value?.value || '',
        }));
        return (
            <TrafficMatchConditionEditor
                contextLabel={`第 ${ruleIndex + 1} 条镜像子规则`}
                rows={rows}
                editable={editable}
                relation={current.matchMode || MatchLogic.AND}
                hint={current.matchMode === MatchLogic.OR ? '满足任一标签即命中' : '需同时满足全部标签'}
                addText="添加流量标签"
                paramTypeOptions={securityMatchSourceOptions}
                onRelationChange={(next) => onChange({ matchMode: next as string, arguments: args })}
                onRowChange={(index, row) => updateArg(index, (item) => ({
                    ...item,
                    type: row.paramType || 'HEADER',
                    key: row.paramKey || '',
                    value: {
                        ...(item.value || defaultMatchValue()),
                        type: row.matchType || MatchType.EXACT,
                        value: row.valueType === MatchValueType.PARAMETER ? '' : row.matchValue || '',
                        value_type: row.valueType || MatchValueType.TEXT,
                    },
                }))}
                onAdd={() => onChange({ matchMode: current.matchMode || MatchLogic.AND, arguments: [...args, defaultArgument()] })}
                onRemove={(index) => onChange({ matchMode: current.matchMode || MatchLogic.AND, arguments: args.filter((_, idx) => idx !== index) })}
            />
        );
    };

    const renderSecurityMatchRuleEditor = (ruleIndex: number, match: TrafficMatchRule | undefined, onChange: (next: TrafficMatchRule) => void) => {
        const current = normalizeSecurityMatchRule(match);
        const args = current.arguments || [];
        const updateArg = (index: number, updater: (arg: NonNullable<TrafficMatchRule['arguments']>[number]) => NonNullable<TrafficMatchRule['arguments']>[number]) => {
            const nextArgs = [...args];
            nextArgs[index] = updater(nextArgs[index] || defaultSecurityArgument());
            onChange({ ...current, arguments: nextArgs, randomPercent: current.randomPercent ?? 0 });
        };
        const rows: TrafficMatchConditionRow[] = args.map(arg => ({
            paramType: arg.type,
            paramKey: arg.key,
            matchType: arg.value?.type || MatchType.EXACT,
            valueType: arg.value?.value_type || MatchValueType.TEXT,
            matchValue: arg.value?.value || '',
        }));
        return (
            <TrafficMatchConditionEditor
                contextLabel={`第 ${ruleIndex + 1} 条鉴权子规则`}
                rows={rows}
                editable={editable}
                relation={current.matchMode || MatchLogic.AND}
                hint={current.matchMode === MatchLogic.OR ? '满足任一条件即命中名单' : '需同时满足全部条件'}
                paramTypeOptions={securityMatchSourceOptions}
                onRelationChange={(next) => onChange({ ...current, matchMode: next as string, randomPercent: current.randomPercent ?? 0 })}
                onRowChange={(index, row) => updateArg(index, (item) => ({
                    ...item,
                    type: row.paramType || 'HEADER',
                    key: row.paramKey || '',
                    value: {
                        ...(item.value || defaultMatchValue()),
                        type: row.matchType || MatchType.EXACT,
                        value: row.valueType === MatchValueType.PARAMETER ? '' : row.matchValue || '',
                        value_type: row.valueType || MatchValueType.TEXT,
                    },
                }))}
                onAdd={() => onChange({ ...current, arguments: [...args, defaultSecurityArgument()], randomPercent: current.randomPercent ?? 0 })}
                onRemove={(index) => onChange({ ...current, arguments: args.filter((_, idx) => idx !== index), randomPercent: current.randomPercent ?? 0 })}
            />
        );
    };

    const renderMockApiEditor = (api: TrafficApiScope | undefined, onChange: (next: TrafficApiScope) => void) => {
        const current = {
            protocol: InterfaceProtocol.HTTP,
            method: 'GET',
            path: defaultMockMatchValue(),
            ...(api || {}),
        };
        const path = current.path || defaultMockMatchValue();
        const presentation = getApiProtocolPresentation(current.protocol);
        const methodField = presentation.usesHttpMethodSelect ? (
            <Select
                aria-label="Mock 接口方法"
                filterable
                creatable
                options={methodOptions}
                value={current.method || 'GET'}
                onChange={(next) => onChange({ ...current, method: next as string })}
            />
        ) : (
            <Input
                aria-label="Mock 接口方法"
                value={current.method || ''}
                placeholder={presentation.methodPlaceholder}
                onChange={(next) => onChange({ ...current, method: next })}
            />
        );
        const matchField = (
            <Select
                aria-label="Mock 接口路径匹配类型"
                options={MatchTypeOption}
                value={path.type || MatchType.EXACT}
                onChange={(next) => onChange({ ...current, path: { ...path, type: next as string } })}
            />
        );
        const pathField = (
            <Input
                aria-label="Mock 接口路径"
                className={style.monoInput}
                value={path.value || ''}
                placeholder={presentation.pathPlaceholder}
                onChange={(next) => onChange({ ...current, path: { ...path, value: next, value_type: path.value_type || MatchValueType.TEXT } })}
            />
        );
        return (
            <div className={style.mockApiEditor}>
                <Select
                    aria-label="Mock 接口协议"
                    options={protocolOptions}
                    value={current.protocol || InterfaceProtocol.HTTP}
                    onChange={(next) => onChange(resetApiScopeForProtocol(current, next as string))}
                />
                {presentation.usesHttpMethodSelect ? methodField : pathField}
                {matchField}
                {presentation.usesHttpMethodSelect ? pathField : methodField}
            </div>
        );
    };

    const renderMockMatchRuleEditor = (ruleIndex: number, match: TrafficMatchRule | undefined, onChange: (next: TrafficMatchRule) => void) => {
        const current = normalizeMockMatchRule(match);
        const args = current.arguments || [];
        const updateArg = (index: number, updater: (arg: NonNullable<TrafficMatchRule['arguments']>[number]) => NonNullable<TrafficMatchRule['arguments']>[number]) => {
            const nextArgs = [...args];
            nextArgs[index] = updater(nextArgs[index] || defaultMockArgument());
            onChange({ ...current, arguments: nextArgs });
        };
        const rows: TrafficMatchConditionRow[] = args.map(arg => ({
            paramType: arg.type,
            paramKey: arg.key,
            matchType: arg.value?.type || MatchType.EXACT,
            valueType: arg.value?.value_type || MatchValueType.TEXT,
            matchValue: arg.value?.value || '',
        }));
        return (
            <TrafficMatchConditionEditor
                contextLabel={`第 ${ruleIndex + 1} 条Mock子规则`}
                rows={rows}
                editable={editable}
                relation={current.matchMode || MatchLogic.AND}
                hint={current.matchMode === MatchLogic.OR ? '满足任一条件即 Mock' : '需同时满足全部条件'}
                paramTypeOptions={mockMatchSourceOptions}
                onRelationChange={(next) => onChange({ ...current, matchMode: next as string })}
                onRowChange={(index, row) => updateArg(index, (item) => ({
                    ...item,
                    type: row.paramType || 'HEADER',
                    key: row.paramKey || '',
                    value: {
                        ...(item.value || defaultMockMatchValue()),
                        type: row.matchType || MatchType.EXACT,
                        value: row.valueType === MatchValueType.PARAMETER ? '' : row.matchValue || '',
                        value_type: row.valueType || MatchValueType.TEXT,
                    },
                }))}
                onAdd={() => onChange({ ...current, arguments: [...args, defaultMockArgument()] })}
                onRemove={(index) => onChange({ ...current, arguments: args.filter((_, idx) => idx !== index) })}
            />
        );
    };

    const renderProtectedInterfacesEditor = (ruleIndex: number, interfaces: TrafficApiScope[], onChange: (next: TrafficApiScope[]) => void) => {
        const rows = interfaces.length ? interfaces : [defaultProtectedInterface()];
        const usesRpcLayout = rows.every((api) => !getApiProtocolPresentation(api?.protocol).usesHttpMethodSelect);
        const updateRow = (index: number, updater: (api: TrafficApiScope) => TrafficApiScope) => {
            onChange(rows.map((item, idx) => idx === index ? updater(item) : item));
        };
        return (
            <div className={style.repeatEditor}>
                <div className={style.interfaceViewport} role="region" aria-label={`第 ${ruleIndex + 1} 条鉴权子规则受保护接口配置`}>
                <div className={`${style.interfaceHeader}${usesRpcLayout ? ` ${style.rpcInterfaceRow}` : ''}`}>
                    <span>协议</span>
                    <span>HTTP 方法 / RPC 接口</span>
                    <span>匹配类型</span>
                    <span>HTTP 路径 / RPC 方法（可选）</span>
                    <span>操作</span>
                </div>
                {rows.map((api, index) => {
                    const current = api || defaultProtectedInterface();
                    const presentation = getApiProtocolPresentation(current.protocol);
                    const path = current.path || { type: MatchType.EXACT, value: presentation.usesHttpMethodSelect ? '/' : '' };
                    const methodField = presentation.usesHttpMethodSelect ? (
                        <Select
                            aria-label={`第 ${ruleIndex + 1} 条鉴权子规则第 ${index + 1} 个受保护接口${presentation.usesHttpMethodSelect ? "HTTP 方法" : "RPC 接口"}`}
                            filterable
                            creatable
                            options={methodOptions}
                            value={current.method || 'GET'}
                            onChange={(next) => updateRow(index, (item) => ({ ...item, method: next as string }))}
                        />
                    ) : (
                        <Input
                            aria-label={`第 ${ruleIndex + 1} 条鉴权子规则第 ${index + 1} 个受保护接口${presentation.usesHttpMethodSelect ? "HTTP 方法" : "RPC 接口"}`}
                            value={current.method || ''}
                            placeholder={presentation.methodPlaceholder}
                            onChange={(next) => updateRow(index, (item) => ({ ...item, method: next }))}
                        />
                    );
                    const matchField = (
                        <Select
                            aria-label={`第 ${ruleIndex + 1} 条鉴权子规则第 ${index + 1} 个受保护接口${presentation.usesHttpMethodSelect ? "HTTP 路径" : "RPC 方法"}匹配类型`}
                            options={MatchTypeOption}
                            value={path.type || MatchType.EXACT}
                            onChange={(next) => updateRow(index, (item) => ({ ...item, path: { type: next as string, value: item.path?.value || '' } }))}
                        />
                    );
                    const pathField = (
                        <Input
                            aria-label={`第 ${ruleIndex + 1} 条鉴权子规则第 ${index + 1} 个受保护接口${presentation.usesHttpMethodSelect ? "HTTP 路径" : "RPC 方法"}`}
                            value={path.value || ''}
                            placeholder={presentation.pathPlaceholder}
                            onChange={(next) => updateRow(index, (item) => ({ ...item, path: { type: item.path?.type || MatchType.EXACT, value: next } }))}
                        />
                    );
                    return (
                        <div className={`${style.interfaceRow}${presentation.usesHttpMethodSelect ? '' : ` ${style.rpcInterfaceRow}`}`} key={`protected-interface-${index}`}>
                            <Select
                                aria-label={`第 ${ruleIndex + 1} 条鉴权子规则第 ${index + 1} 个受保护接口协议`}
                                options={protocolOptions}
                                value={current.protocol || InterfaceProtocol.HTTP}
                                onChange={(next) => updateRow(index, (item) => resetApiScopeForProtocol(item, next as string))}
                            />
                            {presentation.usesHttpMethodSelect ? methodField : pathField}
                            {matchField}
                            {presentation.usesHttpMethodSelect ? pathField : methodField}
                            <Button
                                aria-label={`删除第 ${ruleIndex + 1} 条鉴权子规则第 ${index + 1} 个受保护接口`}
                                shape="circle"
                                variant="text"
                                disabled={rows.length <= 1}
                                onClick={() => onChange(rows.filter((_, idx) => idx !== index))}
                            >
                                <CloseIcon />
                            </Button>
                        </div>
                    );
                })}
                </div>
                <Button
                    aria-label={`第 ${ruleIndex + 1} 条鉴权子规则添加接口`}
                    className={shared.addRuleButton}
                    variant="dashed"
                    icon={<AddIcon />}
                    onClick={() => onChange([...rows, defaultProtectedInterface()])}
                >
                    添加接口
                </Button>
            </div>
        );
    };

    const renderMirrorInterfacesEditor = (ruleIndex: number, interfaces: TrafficApiScope[], onChange: (next: TrafficApiScope[]) => void, defaultApi = defaultMirrorSubRule().interfaces[0], interfaceKind = '镜像') => {
        const rows = interfaces.length ? interfaces : [defaultApi];
        const usesRpcLayout = rows.every((api) => !getApiProtocolPresentation(api?.protocol).usesHttpMethodSelect);
        const updateRow = (index: number, updater: (api: TrafficApiScope) => TrafficApiScope) => {
            onChange(rows.map((item, idx) => idx === index ? updater(item) : item));
        };
        return (
            <div className={style.repeatEditor}>
                <div className={style.interfaceViewport} role="region" aria-label={`第 ${ruleIndex + 1} 条${interfaceKind}子规则接口配置`}>
                <div className={`${style.interfaceHeader}${usesRpcLayout ? ` ${style.rpcInterfaceRow}` : ''}`}>
                    <span>协议</span>
                    <span>HTTP 方法 / RPC 接口</span>
                    <span>匹配类型</span>
                    <span>HTTP 路径 / RPC 方法（可选）</span>
                    <span>操作</span>
                </div>
                {rows.map((api, index) => {
                    const current = api || defaultApi;
                    const path = current.path || defaultMatchValue();
                    const presentation = getApiProtocolPresentation(current.protocol);
                    const methodField = presentation.usesHttpMethodSelect ? (
                        <Select
                            aria-label={`第 ${ruleIndex + 1} 条${interfaceKind}子规则第 ${index + 1} 个接口${presentation.usesHttpMethodSelect ? "HTTP 方法" : "RPC 接口"}`}
                            filterable
                            creatable
                            options={methodOptions}
                            value={current.method || 'GET'}
                            onChange={(next) => updateRow(index, (item) => ({ ...item, method: next as string }))}
                        />
                    ) : (
                        <Input
                            aria-label={`第 ${ruleIndex + 1} 条${interfaceKind}子规则第 ${index + 1} 个接口${presentation.usesHttpMethodSelect ? "HTTP 方法" : "RPC 接口"}`}
                            value={current.method || ''}
                            placeholder={presentation.methodPlaceholder}
                            onChange={(next) => updateRow(index, (item) => ({ ...item, method: next }))}
                        />
                    );
                    const matchField = (
                        <Select
                            aria-label={`第 ${ruleIndex + 1} 条${interfaceKind}子规则第 ${index + 1} 个接口${presentation.usesHttpMethodSelect ? "HTTP 路径" : "RPC 方法"}匹配类型`}
                            options={MatchTypeOption}
                            value={path.type || MatchType.EXACT}
                            onChange={(next) => updateRow(index, (item) => ({ ...item, path: { ...(item.path || defaultMatchValue()), type: next as string } }))}
                        />
                    );
                    const pathField = (
                        <Input
                            aria-label={`第 ${ruleIndex + 1} 条${interfaceKind}子规则第 ${index + 1} 个接口${presentation.usesHttpMethodSelect ? "HTTP 路径" : "RPC 方法"}`}
                            className={style.monoInput}
                            value={path.value || ''}
                            placeholder={presentation.pathPlaceholder}
                            onChange={(next) => updateRow(index, (item) => ({ ...item, path: { type: item.path?.type || MatchType.EXACT, value: next } }))}
                        />
                    );
                    return (
                        <div className={`${style.interfaceRow}${presentation.usesHttpMethodSelect ? '' : ` ${style.rpcInterfaceRow}`}`} key={`mirror-interface-${index}`}>
                            <Select
                                aria-label={`第 ${ruleIndex + 1} 条${interfaceKind}子规则第 ${index + 1} 个接口协议`}
                                options={protocolOptions}
                                value={current.protocol || InterfaceProtocol.HTTP}
                                onChange={(next) => updateRow(index, (item) => resetApiScopeForProtocol(item, next as string))}
                            />
                            {presentation.usesHttpMethodSelect ? methodField : pathField}
                            {matchField}
                            {presentation.usesHttpMethodSelect ? pathField : methodField}
                            <Button
                                aria-label={`删除第 ${ruleIndex + 1} 条${interfaceKind}子规则第 ${index + 1} 个接口`}
                                shape="circle"
                                variant="text"
                                disabled={rows.length <= 1}
                                onClick={() => onChange(rows.filter((_, idx) => idx !== index))}
                            >
                                <CloseIcon />
                            </Button>
                        </div>
                    );
                })}
                </div>
                <Button
                    aria-label={`第 ${ruleIndex + 1} 条${interfaceKind}子规则添加接口`}
                    className={shared.addRuleButton}
                    variant="dashed"
                    icon={<AddIcon />}
                    onClick={() => onChange([...rows, defaultApi])}
                >
                    添加接口
                </Button>
            </div>
        );
    };

    const renderMockReturnPreview = (item: MockRule) => {
        const current = normalizeMockRule(item);
        const headers = Object.entries(current.response?.headers || {}).filter(([key]) => key.trim());
        return (
            <div className={style.mockReturnPreview}>
                <div className={style.mockReturnHead}>
                    <span>Code {current.response?.code || '-'}</span>
                    <span>delay {durationToMs(current.delay)}ms</span>
                </div>
                <pre className={style.mockReturnCode}>
{`Code: ${current.response?.code || ''}
${headers.map(([key, value]) => `${key}: ${value}`).join('\n')}

${current.response?.body || ''}`}
                </pre>
            </div>
        );
    };

    const renderMockResponseEditor = (item: MockRule, index: number) => {
        const current = normalizeMockRule(item);
        const delayMs = durationToMs(current.delay);
        const code = current.response?.code || '';
        const contentType = Object.entries(current.response?.headers || {}).find(([key]) => key.trim().toLowerCase() === 'content-type')?.[1] || '未声明 Content-Type';
        return (
            <div className={style.mockResponsePanel}>
                <div className={style.mockResponseSection}>
                    <div className={style.mockResponseTitle}>响应 Code / 延迟</div>
                    <div className={style.mockResponseGrid}>
                        <div>
                            <div className={shared.editLabel}>响应 Code</div>
                            <Input
                                aria-label={`第 ${index + 1} 条 Mock 子规则响应 Code`}
                                value={code}
                                placeholder="200 / SUCCESS / MOCK_OK"
                                onChange={(value) => setMockRule(index, (draft) => ({ ...draft, response: { ...(draft.response || {}), code: value } }))}
                            />
                        </div>
                        <div>
                            <div className={shared.editLabel}>响应延迟 ms</div>
                            <InputNumber
                                aria-label={`第 ${index + 1} 条 Mock 子规则响应延迟 ms`}
                                theme="normal"
                                min={0}
                                value={delayMs}
                                onChange={(value) => setMockRule(index, (draft) => ({ ...draft, delay: msToDuration(Number(value ?? 0)) }))}
                            />
                        </div>
                    </div>
                    <div className={style.mockOrderHint}>执行顺序：先等待 {delayMs}ms，再返回响应 Code {code || '-'}。</div>
                </div>
                <div className={style.mockResponseSection}>
                    <div className={style.mockResponseTitle}>响应 Header</div>
                    <MockHeaderEditor key={`${rule.id || 'new'}-${mockRules.length}-${index}`} ruleIndex={index} record={current.response?.headers} onChange={(next) => setMockRule(index, (draft) => ({ ...draft, response: { ...(draft.response || {}), headers: next } }))} />
                </div>
                <div className={style.mockResponseSection}>
                    <div className={style.mockBodyHead}>
                        <div>
                            <div className={style.mockResponseTitle}>响应 Body</div>
                            <div className={style.mockBodyTools}>
                                <Tag variant="light-outline">{contentType}</Tag>
                            </div>
                        </div>
                        <Button aria-label={`第 ${index + 1} 条 Mock 子规则响应 Body 全屏编辑`} size="small" variant="outline" onClick={() => setMockBodyDialog({ visible: true, index })}>全屏编辑</Button>
                    </div>
                    <Textarea
                        aria-label={`第 ${index + 1} 条 Mock 子规则响应 Body`}
                        className={style.mockBodyTextarea}
                        autosize={{ minRows: 1, maxRows: 15 }}
                        value={current.response?.body || ''}
                        onChange={(value) => setMockRule(index, (draft) => ({ ...draft, response: { ...(draft.response || {}), body: value } }))}
                    />
                </div>
                <div className={style.mockResponseSection}>
                    <div className={style.mockResponseTitle}>最终返回预览</div>
                    <div className={style.sectionHint}>命中这条子规则后，客户端会收到下面的响应。</div>
                    {renderMockReturnPreview(current)}
                </div>
            </div>
        );
    };

    const buildRequest = (): TrafficGovernanceRule => {
        const values = form.getFieldsValue(true) as Partial<TrafficGovernanceRule & TrafficSecurityRule> & {
            targetNamespace?: string
            targetService?: string
        };
        const ruleValues = { ...values } as Record<string, unknown>;
        const targetNamespace = ruleValues.targetNamespace as string | undefined;
        const targetService = ruleValues.targetService as string | undefined;
        delete ruleValues.targetNamespace;
        delete ruleValues.targetService;
        delete ruleValues.namespace;
        delete ruleValues.service;
        const base = {
            ...rule,
            ...ruleValues,
            target_service: {
                namespace: targetNamespace || rule.target_service?.namespace || '',
                service: targetService || rule.target_service?.service || '',
            },
            metadata: rule.metadata || {},
            enable: values.enable ?? true,
            priority: Number(values.priority ?? 0),
        } as TrafficGovernanceRule;
        delete (base as Record<string, unknown>).namespace;
        delete (base as Record<string, unknown>).service;
        if (kind === 'security') {
            const securityBase = { ...base } as Record<string, unknown>;
            delete securityBase['default' + '_action'];
            const authentication = buildSecurityAuthenticationForMode(securityAuthMode);
            return {
                ...securityBase,
                authentication,
                policies: buildSecurityPoliciesFromView(securityViewRules, securityAuthMode),
            } as TrafficSecurityRule;
        }
        if (kind === 'mock') {
            return {
                ...base,
                rules: buildMockRulesForSubmit(mockRules, mockCaller),
            } as TrafficMock;
        }
        if (kind === 'mirror') {
            const mirrorBase = { ...base } as Record<string, unknown>;
            const currentCallee = normalizeMirrorCallee(rule);
            delete mirrorBase.target_service;
            return {
                ...mirrorBase,
                caller: mirrorCaller,
                callee: {
                    namespace: targetNamespace || currentCallee.namespace || '',
                    service: targetService || currentCallee.service || '',
                },
                rules: buildMirrorRulesForSubmit(mirrorRules),
            } as TrafficMirror;
        }
        return {
            ...base,
            rules: ((rule as TrafficMirror | TrafficMock).rules || []),
        } as TrafficGovernanceRule;
    };

    const onSubmit: FormProps['onSubmit'] = async (e) => {
        if (e.validateResult !== true) return;
        if (kind === 'security' && securityValidationErrors.length > 0) {
            openErrNotification('保存校验失败', securityValidationErrors[0].message);
            return;
        }
        if (kind === 'mock' && mockValidationErrors.length > 0) {
            openErrNotification('保存校验失败', mockValidationErrors[0].message);
            return;
        }
        if (kind === 'mirror' && mirrorValidationErrors.length > 0) {
            openErrNotification('保存校验失败', mirrorValidationErrors[0].message);
            return;
        }
        const builtRequest = buildRequest();
        const req = {
            ...builtRequest,
            namespace: builtRequest.namespace || ruleNamespace,
        } as TrafficGovernanceRule;
        try {
            if (op === 'create') {
                await createTrafficGovernanceRules(kind, [req]);
                openInfoNotification('请求成功', `创建${TrafficGovernanceKindLabel[kind]}规则成功`);
                refresh(true);
            } else {
                await modifyTrafficGovernanceRules(kind, [req]);
                openInfoNotification('请求成功', `更新${TrafficGovernanceKindLabel[kind]}规则成功`);
                setEditable(false);
                refresh(false);
            }
        } catch (error) {
            openErrNotification('请求失败', `${op === 'create' ? '创建' : '更新'}${TrafficGovernanceKindLabel[kind]}规则失败: ${(error as Error).message}`);
        }
    };

    const renderMockServiceScopeSection = (
        <ServiceScopeSection
            editable={editable}
            collapsed={serviceScopeCollapsed}
            caller={mockCaller}
            callee={currentTarget}
            callerNamespaceOptions={mockCallerNamespaceOptions}
            calleeNamespaceOptions={namespaceOptions}
            callerServiceOptions={mockCallerServiceOptions}
            calleeServiceOptions={serviceOptions}
            onCollapsedChange={setServiceScopeCollapsed}
            onCallerNamespaceChange={(value) => setMockCaller((prev) => {
                if (value === '*') return defaultMockCaller();
                return { namespace: value, service: prev.service === '*' ? '' : prev.service };
            })}
            onCallerServiceChange={(value) => setMockCaller((prev) => (value === '*' ? defaultMockCaller() : { ...prev, service: value }))}
            onCalleeNamespaceChange={(value) => {
                setRule((prev) => ({ ...prev, target_service: { ...(prev.target_service || {}), namespace: value, service: '' } }));
                form.setFieldsValue({ targetNamespace: value, targetService: '' });
            }}
            onCalleeServiceChange={(value) => {
                setRule((prev) => ({ ...prev, target_service: { ...(prev.target_service || {}), service: value } }));
                form.setFieldsValue({ targetService: value });
            }}
            fixedRole={op === 'create' ? serviceContext?.role : undefined}
        />
    );

    const renderMirrorServiceScopeSection = (
        <ServiceScopeSection
            editable={editable}
            collapsed={serviceScopeCollapsed}
            caller={mirrorCaller}
            callee={mirrorCallee}
            callerNamespaceOptions={mirrorCallerNamespaceOptions}
            calleeNamespaceOptions={mirrorCalleeNamespaceOptions}
            callerServiceOptions={mirrorCallerServiceOptions}
            calleeServiceOptions={mirrorCalleeServiceOptions}
            onCollapsedChange={setServiceScopeCollapsed}
            onCallerNamespaceChange={(value) => setMirrorCaller((prev) => {
                if (value === '*') return defaultMirrorCaller();
                return { namespace: value, service: prev.service === '*' ? '' : prev.service };
            })}
            onCallerServiceChange={(value) => setMirrorCaller((prev) => (value === '*' ? defaultMirrorCaller() : { ...prev, service: value }))}
            onCalleeNamespaceChange={(value) => {
                const service = value === '*' ? '*' : '';
                setRule((prev) => ({ ...prev, callee: { namespace: value, service } } as TrafficMirror));
                form.setFieldsValue({ targetNamespace: value, targetService: service });
            }}
            onCalleeServiceChange={(value) => {
                setRule((prev) => ({ ...prev, callee: { ...normalizeMirrorCallee(prev), service: value } } as TrafficMirror));
                form.setFieldsValue({ targetService: value });
            }}
            fixedRole={op === 'create' ? serviceContext?.role : undefined}
        />
    );

    const renderCommonFields = () => {
        if (!editable) {
            return (
                <div className={shared.infoGrid}>
                    {readonlyItem('规则名称', rule.name)}
                    {readonlyItem('启用状态', <span className={`${shared.pill} ${rule.enable ? shared.pillOk : shared.pillOff} ${shared.pillDot}`}>{rule.enable ? '启用' : '禁用'}</span>)}
                    {readonlyItem('优先级', rule.priority ?? 0)}
                    {readonlyItem('Revision', rule.revision)}
                    {readonlyItem('描述', rule.description, true)}
                    {readonlyItem('规则标签', <RuleLabelField metadata={rule.metadata} />, true)}
                </div>
            );
        }
        return (
            <div className={shared.infoGrid}>
                <FormItem className={shared.field} label="规则名称" name="name" rules={[{ required: true, message: '规则名称不能为空' }]}>
                    <Input maxlength={128} />
                </FormItem>
                <FormItem className={shared.field} label="启用状态" name="enable">
                    <Switch label={['启用', '禁用']} />
                </FormItem>
                <FormItem className={shared.field} label="优先级" name="priority">
                    <InputNumber theme="normal" min={0} max={1000000} />
                </FormItem>
                <div className={`${shared.field} ${shared.full}`}>
                    <div className={shared.fieldLabel}>规则标签</div>
                    <RuleLabelField
                        metadata={rule.metadata}
                        editable
                        onChange={(next) => setRule((prev) => ({ ...prev, metadata: next } as TrafficGovernanceRule))}
                    />
                </div>
                <FormItem className={`${shared.field} ${shared.full}`} label="描述" name="description">
                    <Textarea autosize={{ minRows: 2, maxRows: 4 }} maxlength={512} />
                </FormItem>
            </div>
        );
    };

    const renderSecurityServiceInfo = () => {
        const target = rule.target_service || { namespace: '', service: '' };
        const summary = target.namespace || target.service
            ? `${target.namespace || '-'} / ${target.service || '-'}`
            : '未选择被调服务';
        return (
            <CollapsibleSection
                collapsed={securityServiceInfoCollapsed}
                onCollapsedChange={setSecurityServiceInfoCollapsed}
                header="② 服务信息"
                summary={summary}
                collapseLabel="服务信息"
            >
                <div className={style.serviceInfoHint}>
                    调用鉴权绑定被调服务；来源身份由下一步认证方式决定，不再通过普通请求 Header 冒充服务来源。
                </div>
                {!editable || (op === 'create' && Boolean(serviceContext)) ? (
                    <div className={style.securityServiceInfoFields}>
                        <div className={style.securityServiceInfoField}>{readonlyItem('命名空间', target.namespace, false)}</div>
                        <div className={style.securityServiceInfoField}>{readonlyItem('服务名称', target.service, false)}</div>
                    </div>
                ) : (
                    <div className={style.securityServiceInfoFields}>
                        <FormItem className={style.securityServiceInfoField} label="命名空间" name="targetNamespace" rules={[{ required: true, message: '命名空间不能为空' }]}>
                            <Select
                                filterable
                                creatable
                                options={namespaceOptions}
                                onChange={(value) => {
                                    setRule((prev) => ({ ...prev, target_service: { ...(prev.target_service || {}), namespace: value as string, service: '' } }));
                                    form.setFieldsValue({ targetService: '' });
                                }}
                            />
                        </FormItem>
                        <FormItem className={style.securityServiceInfoField} label="服务名称" name="targetService" rules={[{ required: true, message: '服务名称不能为空' }]}>
                            <Select
                                filterable
                                creatable
                                options={serviceOptions}
                                onChange={(value) => setRule((prev) => ({ ...prev, target_service: { ...(prev.target_service || {}), service: value as string } }))}
                            />
                        </FormItem>
                    </div>
                )}
            </CollapsibleSection>
        );
    };

    const renderSecurityAuthentication = () => {
        const modeLabel = securityAuthMode === TrafficSecurityAuthMode.MANAGED_IDENTITY
            ? 'Pole 托管服务身份'
            : '自定义 Header（兼容模式）';
        return (
            <CollapsibleSection
                collapsed={securityAuthenticationCollapsed}
                onCollapsedChange={setSecurityAuthenticationCollapsed}
                header="③ 认证方式"
                summary={modeLabel}
                collapseLabel="认证方式"
            >
                {!editable ? (
                    <div className={style.authenticationSummary}>
                        <div>
                            <div className={style.partitionTitle}>{modeLabel}</div>
                            <div className={style.partitionDesc}>
                                {securityAuthMode === TrafficSecurityAuthMode.MANAGED_IDENTITY
                                    ? 'control-plane 托管服务身份与短期凭证；业务需显式接入 SDK 的出站注入和入站验证适配器。'
                                    : '兼容模式按每条鉴权子规则中的请求 Header 条件匹配，不设置规则级共享 Header 值。'}
                            </div>
                        </div>
                        <Tag theme="warning" variant="light-outline">{modeLabel}</Tag>
                    </div>
                ) : (
                    <div className={style.authenticationEditor}>
                        <RadioGroup aria-label="鉴权模式" value={securityAuthMode} onChange={(value) => setSecurityAuthMode(value as TrafficSecurityAuthMode)}>
                            <Radio value={TrafficSecurityAuthMode.MANAGED_IDENTITY}>Pole 托管服务身份（推荐）</Radio>
                            <Radio value={TrafficSecurityAuthMode.LEGACY_REQUEST_MATCH}>自定义 Header（兼容模式）</Radio>
                        </RadioGroup>
                        <div className={style.authenticationModePanel}>
                            {securityAuthMode === TrafficSecurityAuthMode.MANAGED_IDENTITY ? (
                                <>
                                    <div className={style.partitionTitle}>身份由 control-plane 自动管理</div>
                                    <div className={style.partitionDesc}>无需填写身份 ID 或密钥。SDK 使用 service token 领取和续期 WorkloadCredential；HTTP 或 gRPC 服务需显式挂载 SDK 适配器，入站验签后才会生成可信调用方。</div>
                                </>
                            ) : (
                                <>
                                    <div className={style.partitionTitle}>在每个鉴权子规则中配置 Header 匹配条件</div>
                                    <div className={style.partitionDesc}>先配置受保护接口，再在对应子规则的“自定义 Header 匹配条件”中填写 Header 名、匹配类型和值；每个子规则相互独立。</div>
                                </>
                            )}
                        </div>
                    </div>
                )}
            </CollapsibleSection>
        );
    };

    const renderSecurityRules = () => {
        const rules = normalizeSecurityViewOrder(securityViewRules);
        const renderRuleCard = (item: SecurityViewRule, index: number) => {
            const isCollapsed = collapsedSecurityPolicyIndexes.has(index);
            const isService = item.kind === 'service';
            const title = isService ? '服务级规则' : `子规则 [${index + 1}]`;
            const managedCaller = normalizeManagedCaller(item.managedCaller);
            const callerText = managedCaller.any_authenticated
                ? '任意已认证服务'
                : (managedCaller.callers || []).map((caller) => `${caller.namespace || '-'}/${caller.service || '-'}`).join('、') || '-';
            return (
                <div className={`${shared.policy} ${isCollapsed ? shared.policyCollapsed : ''}`} key={`${item.kind}-${item.listType}-${index}`}>
                    <div className={shared.policyHead} onClick={() => toggleSecurityPolicyCollapsed(index)}>
                        <div className={shared.policyHeadMain}>
                            <span className={shared.caret}><ChevronRightIcon /></span>
                            <div>
                                <div className={shared.policyIndex}>{title}</div>
                                {isCollapsed && (
                                    <div className={shared.policySummary}>
                                        {isService ? '服务级兜底规则 · 1 条名单策略' : `${item.interfaces.length} 个接口 · 1 条名单策略`}
                                    </div>
                                )}
                            </div>
                        </div>
                        <div className={shared.policyHeadActions} onClick={(e) => e.stopPropagation()}>
                            <Tag theme={item.listType === 'ALLOW_LIST' ? 'success' : 'danger'} variant="light-outline">
                                {listTypeText[item.listType]}
                            </Tag>
                        </div>
                    </div>
                    <div className={shared.policyBody}>
                        {isService ? (
                            <div className={shared.step} data-step="1">
                                <div className={shared.stepTitle}>服务级范围</div>
                                <div className={shared.stepContent}>
                                    <div className={style.sectionHint}>受保护接口为空，表示整个服务级别；它始终位于黑名单与白名单接口规则之后。</div>
                                </div>
                            </div>
                        ) : (
                            <div className={shared.step} data-step="1">
                                <div className={shared.stepTitle}>受保护接口</div>
                                <div className={shared.stepContent}>
                                    <div className={style.sectionHint}>
                                        所属{listTypeText[item.listType]}分区，提交时自动按 {item.listType} 映射；子规则内部不再重复选择名单类型。
                                    </div>
                                    <div className={style.interfaceList}>
                                        {item.interfaces.map((api, apiIndex) => (
                                            <div className={shared.tagRow} key={`security-interface-${index}-${apiIndex}`}>
                                                {renderApiScope(api)}
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            </div>
                        )}
                        {isService && (
                            <div className={shared.step} data-step="2">
                                <div className={shared.stepTitle}>服务级名单类型</div>
                                <div className={shared.stepContent}>
                                    <Tag theme={item.listType === 'ALLOW_LIST' ? 'success' : 'danger'} variant="light-outline">{listTypeText[item.listType]}</Tag>
                                    <span className={style.inlineHint}>服务级名单语义由此处选择；受保护接口保持为空</span>
                                </div>
                            </div>
                        )}
                        {securityAuthMode === TrafficSecurityAuthMode.MANAGED_IDENTITY ? (
                            <div className={shared.step} data-step={isService ? '3' : '2'}>
                                <div className={shared.stepTitle}>配置的来源服务</div>
                                <div className={shared.stepContent}>
                                    <div className={style.managedCallerSummary}>{callerText}</div>
                                </div>
                            </div>
                        ) : (
                            <div className={shared.step} data-step={isService ? '3' : '2'}>
                                <div className={shared.stepTitle}>自定义 Header 匹配条件</div>
                                <div className={shared.stepContent}>{renderSecurityMatchRule(item.strategy)}</div>
                            </div>
                        )}
                    </div>
                </div>
            );
        };
        return (
            <div className={style.securitySections}>
                <div className={style.securityPartition}>
                    <div className={style.partitionHead}>
                        <div>
                            <div className={style.partitionTitle}>黑名单接口规则</div>
                            <div className={style.partitionDesc}>黑名单语义由分区自动映射；受保护接口必须写在黑名单或白名单分区里。</div>
                        </div>
                        <Tag theme="danger" variant="light-outline">{rules.filter(item => item.kind === 'deny').length} 条</Tag>
                    </div>
                    <div className={shared.ruleList}>
                        {rules.map((item, index) => item.kind === 'deny' ? renderRuleCard(item, index) : null)}
                        {!rules.some(item => item.kind === 'deny') && <div className={shared.emptyLine}>暂无黑名单接口规则</div>}
                    </div>
                </div>
                <div className={style.securityPartition}>
                    <div className={style.partitionHead}>
                        <div>
                            <div className={style.partitionTitle}>白名单接口规则</div>
                            <div className={style.partitionDesc}>白名单语义由分区自动映射；接口级名单规则统一放在服务级规则之前。</div>
                        </div>
                        <Tag theme="success" variant="light-outline">{rules.filter(item => item.kind === 'allow').length} 条</Tag>
                    </div>
                    <div className={shared.ruleList}>
                        {rules.map((item, index) => item.kind === 'allow' ? renderRuleCard(item, index) : null)}
                        {!rules.some(item => item.kind === 'allow') && <div className={shared.emptyLine}>暂无白名单接口规则</div>}
                    </div>
                </div>
                <div className={style.securityPartition}>
                    <div className={style.partitionHead}>
                        <div>
                            <div className={style.partitionTitle}>服务级规则</div>
                            <div className={style.partitionDesc}>可选；受保护接口留空时表示整个服务级别，存在时始终位于最后。</div>
                        </div>
                    </div>
                    <div className={shared.ruleList}>
                        {rules.map((item, index) => item.kind === 'service' ? renderRuleCard(item, index) : null)}
                        {!rules.some(item => item.kind === 'service') && <div className={shared.emptyLine}>未配置服务级规则，只按上方接口规则生效。</div>}
                    </div>
                </div>
            </div>
        );
    };

    const renderMirrorRules = () => {
        const rules = mirrorRules;
        return (
            <div className={shared.ruleList}>
                {rules.length ? rules.map((item: MirrorViewRule, index) => {
                    const isCollapsed = collapsedMirrorRuleIndexes.has(index);
                    const matchCount = removeCallerServiceArguments(item.traffic_match_rule).arguments?.length || 0;
                    const target = `${text(item.destination?.namespace)}/${text(item.destination?.service)}`;
                    const summary = `${item.interfaces.length} 个接口 · ${matchCount} 个流量标签 · ${item.mirror_percent ?? 0}% -> ${target}`;
                    return (
                        <div className={`${shared.policy} ${isCollapsed ? shared.policyCollapsed : ''}`} key={`mirror-rule-${index}`}>
                            <div className={shared.policyHead} onClick={() => toggleMirrorRuleCollapsed(index)}>
                                <div className={shared.policyHeadMain}>
                                    <span className={shared.caret}><ChevronRightIcon /></span>
                                    <div>
                                        <div className={shared.policyIndex}>镜像子规则 [{index + 1}]</div>
                                        <div className={shared.policySummary}>{summary}</div>
                                    </div>
                                </div>
                                <div className={shared.policyHeadActions} onClick={(e) => e.stopPropagation()}>
                                    <Tag theme="primary" variant="light-outline">{item.mirror_percent ?? 0}%</Tag>
                                    <Tag theme={item.disable ? 'default' : 'success'} variant="light-outline">{item.disable ? '禁用' : '启用'}</Tag>
                                </div>
                            </div>
                            <div className={shared.policyBody}>
                                <div className={shared.step} data-step="1">
                                    <div className={shared.stepTitle}>接口范围</div>
                                    <div className={shared.stepContent}>
                                        <div className={style.interfaceList}>
                                            {item.interfaces.map((api, apiIndex) => (
                                                <div className={shared.tagRow} key={`mirror-interface-view-${index}-${apiIndex}`}>
                                                    {renderApiScope(api)}
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                </div>
                                <div className={shared.step} data-step="2">
                                    <div className={shared.stepTitle}>流量标签</div>
                                    <div className={shared.stepContent}>{renderMirrorMatchRule(item.traffic_match_rule)}</div>
                                </div>
                                <div className={shared.step} data-step="3">
                                    <div className={shared.stepTitle}>镜像执行</div>
                                    <div className={shared.stepContent}>
                                        <div className={shared.infoGrid}>
                                            {readonlyItem('镜像比例', `${item.mirror_percent ?? 0}%`)}
                                            {readonlyItem('镜像目标', target)}
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    );
                }) : <div className={shared.emptyLine}>暂无镜像子规则</div>}
            </div>
        );
    };

    const renderMockRules = () => {
        const rules = mockRules;
        return (
            <div className={shared.ruleList}>
                {rules.length ? rules.map((source: MockRule, index) => {
                    const item = normalizeMockRule(source);
                    const isCollapsed = collapsedMockRuleIndexes.has(index);
                    const match = normalizeMockMatchRule(item.traffic_match_rule);
                    const apis = item.apis?.length ? item.apis : [item.api];
                    return (
                        <div className={`${shared.policy} ${isCollapsed ? shared.policyCollapsed : ''}`} key={`mock-rule-${index}`}>
                            <div className={shared.policyHead} onClick={() => toggleMockRuleCollapsed(index)}>
                                <div className={shared.policyHeadMain}>
                                    <span className={shared.caret}><ChevronRightIcon /></span>
                                    <div>
                                        <div className={shared.policyIndex}>Mock 子规则 [{index + 1}]</div>
                                        <div className={shared.policySummary}>
                                            {apis.length} 个接口 · {match.matchMode === MatchLogic.OR ? '满足任一条件即 Mock' : '需同时满足全部条件'}
                                        </div>
                                    </div>
                                </div>
                                <div className={shared.policyHeadActions} onClick={(e) => e.stopPropagation()}>
                                    <Tag theme={item.disable ? 'default' : 'success'} variant="light-outline">{item.disable ? '禁用' : '启用'}</Tag>
                                </div>
                            </div>
                            <div className={shared.policyBody}>
                                <div className={shared.step} data-step="1">
                                    <div className={shared.stepTitle}>选择接口</div>
                                    <div className={shared.stepContent}>{renderApiScopeList(apis.filter(Boolean) as TrafficApiScope[])}</div>
                                </div>
                                <div className={shared.step} data-step="2">
                                    <div className={shared.stepTitle}>流量匹配</div>
                                    <div className={shared.stepContent}>{renderMockMatchRule(item.traffic_match_rule)}</div>
                                </div>
                                <div className={shared.step} data-step="3">
                                    <div className={shared.stepTitle}>Mock 响应结果</div>
                                    <div className={shared.stepContent}>
                                        <div className={shared.infoGrid}>
                                            {readonlyItem('响应 Code', item.response?.code)}
                                            {readonlyItem('响应延迟', `${durationToMs(item.delay)}ms`)}
                                            {readonlyItem('响应 Header', renderRecord(item.response?.headers), true)}
                                            {readonlyItem('响应 Body', <pre className={style.bodyView}>{text(item.response?.body)}</pre>, true)}
                                        </div>
                                        {renderMockReturnPreview(item)}
                                    </div>
                                </div>
                            </div>
                        </div>
                    );
                }) : <div className={shared.emptyLine}>暂无 Mock 子规则</div>}
            </div>
        );
    };

    const renderEditSecurityRules = () => {
        const rules = normalizeSecurityViewOrder(securityViewRules);
        const addRule = (kindValue: SecuritySubRuleKind, listType: SecurityListType) => {
            const next: SecurityViewRule = {
                kind: kindValue,
                listType,
                interfaces: kindValue === 'service' ? [] : [defaultProtectedInterface()],
                strategy: defaultSecurityMatchRule(),
                managedCaller: defaultManagedCaller(),
            };
            setSecurityViewRules((prev) => normalizeSecurityViewOrder([
                ...prev.filter(item => kindValue !== 'service' || item.kind !== 'service'),
                next,
            ]));
        };
        const renderRuleCard = (item: SecurityViewRule, index: number) => {
            const isCollapsed = collapsedSecurityPolicyIndexes.has(index);
            const isService = item.kind === 'service';
            return (
                <div className={`${shared.policy} ${isCollapsed ? shared.policyCollapsed : ''}`} key={`${item.kind}-${item.listType}-${index}`}>
                    <div className={shared.policyHead} onClick={() => toggleSecurityPolicyCollapsed(index)}>
                        <div className={shared.policyHeadMain}>
                            <span className={shared.caret}><ChevronRightIcon /></span>
                            <div>
                                <div className={shared.policyIndex}>{isService ? '服务级规则' : `子规则 [${index + 1}]`}</div>
                                {isCollapsed && (
                                    <div className={shared.policySummary}>
                                        {isService ? '服务级兜底规则 · 1 条名单策略' : `${item.interfaces.length} 个接口 · 1 条名单策略`}
                                    </div>
                                )}
                            </div>
                        </div>
                        <div className={shared.policyHeadActions} onClick={(e) => e.stopPropagation()}>
                            <Tag theme={item.listType === 'ALLOW_LIST' ? 'success' : 'danger'} variant="light-outline">{listTypeText[item.listType]}</Tag>
                            <Button aria-label={`删除第 ${index + 1} 条鉴权子规则`} shape="circle" variant="text" onClick={() => removeSecurityViewRule(index)}><CloseIcon /></Button>
                        </div>
                    </div>
                    <div className={shared.policyBody}>
                        {isService ? (
                            <>
                                <div className={shared.step} data-step="1">
                                    <div className={shared.stepTitle}>服务级范围</div>
                                    <div className={shared.stepContent}>
                                        <div className={style.sectionHint}>受保护接口为空，表示服务级兜底规则；它只能存在一条，并且始终排在接口级规则之后。</div>
                                    </div>
                                </div>
                                <div className={shared.step} data-step="2">
                                    <div className={shared.stepTitle}>服务级名单类型</div>
                                    <div className={shared.stepContent}>
                                        <Select
                                            aria-label="服务级鉴权名单类型"
                                            className={style.serviceListTypeSelect}
                                            options={[
                                                { label: '白名单', value: 'ALLOW_LIST' },
                                                { label: '黑名单', value: 'DENY_LIST' },
                                            ]}
                                            value={item.listType}
                                            onChange={(value) => setSecurityViewRule(index, (current) => ({ ...current, listType: value as SecurityListType }))}
                                        />
                                        <span className={style.inlineHint}>服务级名单语义由此处选择；受保护接口保持为空</span>
                                    </div>
                                </div>
                            </>
                        ) : (
                            <div className={shared.step} data-step="1">
                                <div className={shared.stepTitle}>受保护接口<span className={shared.stepHint}>接口必须归属黑名单或白名单分区，满足任一接口后进入名单判断</span></div>
                                <div className={shared.stepContent}>
                                    <div className={style.sectionHint}>
                                        所属{listTypeText[item.listType]}分区，提交时自动按 {item.listType} 映射；子规则内部不再重复选择名单类型。
                                    </div>
                                    {renderProtectedInterfacesEditor(index, item.interfaces, (next) => setSecurityViewRule(index, (current) => ({ ...current, interfaces: next })))}
                                </div>
                            </div>
                        )}
                        {securityAuthMode === TrafficSecurityAuthMode.MANAGED_IDENTITY ? (
                            <div className={shared.step} data-step={isService ? '3' : '2'}>
                                <div className={shared.stepTitle}>配置的来源服务<span className={shared.stepHint}>后续由被调方凭证验证结果匹配</span></div>
                                <div className={shared.stepContent}>
                                    <div className={style.managedCallerEditor}>
                                        <Switch
                                            aria-label={`第 ${index + 1} 条鉴权子规则${normalizeManagedCaller(item.managedCaller).any_authenticated ? "任意已认证服务" : "选择具体服务"}`}
                                            label={['任意已认证服务', '选择具体服务']}
                                            value={normalizeManagedCaller(item.managedCaller).any_authenticated}
                                            onChange={(value) => setSecurityViewRule(index, (current) => ({
                                                ...current,
                                                managedCaller: { any_authenticated: value, callers: value ? [] : (current.managedCaller?.callers || []) },
                                            }))}
                                        />
                                        {!normalizeManagedCaller(item.managedCaller).any_authenticated && (
                                            <Select
                                                aria-label={`第 ${index + 1} 条鉴权子规则搜索来源服务`}
                                                className={style.managedCallerSelect}
                                                multiple
                                                filterable
                                                clearable
                                                label="搜索来源服务"
                                                placeholder="选择一个或多个来源服务"
                                                options={managedCallerOptions}
                                                value={encodeManagedCallers(item)}
                                                onChange={(value) => setSecurityViewRule(index, (current) => ({
                                                    ...current,
                                                    managedCaller: { any_authenticated: false, callers: decodeManagedCallers(value) },
                                                }))}
                                            />
                                        )}
                                    </div>
                                </div>
                            </div>
                        ) : (
                            <div className={shared.step} data-step={isService ? '3' : '2'}>
                                <div className={shared.stepTitle}>自定义 Header 匹配条件<span className={shared.stepHint}>每个子规则独立匹配调用请求</span></div>
                                <div className={shared.stepContent}>
                                    {renderSecurityMatchRuleEditor(index, item.strategy, (next) => setSecurityViewRule(index, (current) => ({ ...current, strategy: next })))}
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            );
        };
        return (
            <div className={style.securitySections}>
                <div className={style.securityPartition}>
                    <div className={style.partitionHead}>
                        <div>
                            <div className={style.partitionTitle}>黑名单接口规则</div>
                            <div className={style.partitionDesc}>黑名单语义由分区自动映射；受保护接口必须写在黑名单或白名单分区里。</div>
                        </div>
                        <Button variant="outline" icon={<AddIcon />} onClick={() => addRule('deny', 'DENY_LIST')}>添加黑名单接口</Button>
                    </div>
                    <div className={shared.ruleList}>
                        {rules.map((item, index) => item.kind === 'deny' ? renderRuleCard(item, index) : null)}
                        {!rules.some(item => item.kind === 'deny') && <div className={shared.emptyLine}>暂无黑名单接口规则</div>}
                    </div>
                </div>
                <div className={style.securityPartition}>
                    <div className={style.partitionHead}>
                        <div>
                            <div className={style.partitionTitle}>白名单接口规则</div>
                            <div className={style.partitionDesc}>白名单语义由分区自动映射；接口级名单规则统一放在服务级规则之前。</div>
                        </div>
                        <Button variant="outline" icon={<AddIcon />} onClick={() => addRule('allow', 'ALLOW_LIST')}>添加白名单接口</Button>
                    </div>
                    <div className={shared.ruleList}>
                        {rules.map((item, index) => item.kind === 'allow' ? renderRuleCard(item, index) : null)}
                        {!rules.some(item => item.kind === 'allow') && <div className={shared.emptyLine}>暂无白名单接口规则</div>}
                    </div>
                </div>
                <div className={style.securityPartition}>
                    <div className={style.partitionHead}>
                        <div>
                            <div className={style.partitionTitle}>服务级规则</div>
                            <div className={style.partitionDesc}>可选；没有服务级规则时，只按上方黑名单和白名单接口规则生效。</div>
                        </div>
                        {!rules.some(item => item.kind === 'service') && (
                            <Button variant="outline" icon={<AddIcon />} onClick={() => addRule('service', 'ALLOW_LIST')}>添加服务级规则</Button>
                        )}
                    </div>
                    <div className={shared.ruleList}>
                        {rules.map((item, index) => item.kind === 'service' ? renderRuleCard(item, index) : null)}
                        {!rules.some(item => item.kind === 'service') && <div className={shared.emptyLine}>服务级规则可以不配置。</div>}
                    </div>
                </div>
            </div>
        );
    };

    const renderEditMirrorRules = () => {
        const rules = mirrorRules;
        const serviceOptionsForNamespace = (namespace?: string) => (serviceState.datas || [])
            .filter((svc: ServiceView) => !namespace || namespace === '*' || svc.namespace === namespace)
            .map((svc: ServiceView) => ({ label: svc.name, value: svc.name }));
        return (
            <div className={shared.ruleList}>
                <div className={style.mirrorRuleFooter}>
                    <div>
                        <div className={style.partitionTitle}>镜像规则</div>
                        <div className={style.partitionDesc}>在服务范围内配置多条接口 / 标签镜像子规则</div>
                    </div>
                    <Space size={8}>
                        <Tag theme="primary" variant="light-outline">{rules.length} 条</Tag>
                        <Tag theme="success" variant="light-outline">{rules.filter((item) => !item.disable).length} 条启用</Tag>
                    </Space>
                </div>
                {rules.map((item, index) => {
                    const isCollapsed = collapsedMirrorRuleIndexes.has(index);
                    const matchCount = removeCallerServiceArguments(item.traffic_match_rule).arguments?.length || 0;
                    const target = `${item.destination?.namespace || '-'}/${item.destination?.service || '-'}`;
                    const summary = `${item.interfaces.length} 个接口 · ${matchCount} 个流量标签 · ${item.mirror_percent ?? 0}% -> ${target}`;
                    return (
                        <div className={`${shared.policy} ${isCollapsed ? shared.policyCollapsed : ''}`} key={`mirror-rule-edit-${index}`}>
                            <div className={shared.policyHead} onClick={() => toggleMirrorRuleCollapsed(index)}>
                                <div className={shared.policyHeadMain}>
                                    <span className={shared.caret}><ChevronRightIcon /></span>
                                    <div>
                                        <div className={shared.policyIndex}>镜像子规则 [{index + 1}]</div>
                                        <div className={shared.policySummary}>{summary}</div>
                                    </div>
                                </div>
                                <div className={shared.policyHeadActions} onClick={(e) => e.stopPropagation()}>
                                    <Tag theme="primary" variant="light-outline">{item.mirror_percent ?? 0}%</Tag>
                                    <Switch
                                        aria-label={`第 ${index + 1} 条镜像子规则${!item.disable ? "启用" : "禁用"}`}
                                        label={['启用', '禁用']}
                                        value={!item.disable}
                                        onChange={(value) => setMirrorRule(index, (current) => ({ ...current, disable: !value }))}
                                    />
                                    <Button
                                        aria-label={`删除第 ${index + 1} 条镜像子规则`}
                                        shape="circle"
                                        variant="text"
                                        disabled={rules.length <= 1}
                                        onClick={() => setRule((prev) => ({ ...(prev as TrafficMirror), rules: rules.filter((_, idx) => idx !== index) }))}
                                    >
                                        <CloseIcon />
                                    </Button>
                                </div>
                            </div>
                            <div className={shared.policyBody}>
                                <div className={shared.step} data-step="1">
                                    <div className={shared.stepTitle}>接口范围<span className={shared.stepHint}>支持为一条子规则配置多个接口</span></div>
                                    <div className={shared.stepContent}>
                                        {renderMirrorInterfacesEditor(index, item.interfaces, (next) => setMirrorRule(index, (current) => ({ ...current, interfaces: next })))}
                                    </div>
                                </div>
                                <div className={shared.step} data-step="2">
                                    <div className={shared.stepTitle}>流量标签<span className={shared.stepHint}>按标签关系命中需要镜像的流量</span></div>
                                    <div className={shared.stepContent}>
                                        {renderMirrorMatchRuleEditor(index, item.traffic_match_rule, (next) => setMirrorRule(index, (current) => ({ ...current, traffic_match_rule: next })))}
                                    </div>
                                </div>
                                <div className={shared.step} data-step="3">
                                    <div className={shared.stepTitle}>镜像执行</div>
                                    <div className={shared.stepContent}>
                                        <div className={style.mirrorExecuteGrid}>
                                            <div>
                                                <div className={shared.editLabel}>镜像比例</div>
                                                <InputNumber
                                                    aria-label={`第 ${index + 1} 条镜像子规则镜像比例`}
                                                    theme="normal"
                                                    min={0}
                                                    max={100}
                                                    suffix="%"
                                                    value={item.mirror_percent ?? 0}
                                                    onChange={(value) => setMirrorRule(index, (current) => ({ ...current, mirror_percent: Number(value ?? 0) }))}
                                                />
                                            </div>
                                            <div>
                                                <div className={shared.editLabel}>目标命名空间</div>
                                                <Select
                                                    aria-label={`第 ${index + 1} 条镜像子规则目标命名空间`}
                                                    filterable
                                                    creatable
                                                    options={namespaceOptions}
                                                    value={item.destination?.namespace || ''}
                                                    onChange={(value) => setMirrorRule(index, (current) => ({ ...current, destination: { ...(current.destination || {}), namespace: value as string, service: '' } }))}
                                                />
                                            </div>
                                            <div>
                                                <div className={shared.editLabel}>目标服务</div>
                                                <Select
                                                    aria-label={`第 ${index + 1} 条镜像子规则目标服务`}
                                                    filterable
                                                    creatable
                                                    options={serviceOptionsForNamespace(item.destination?.namespace)}
                                                    value={item.destination?.service || ''}
                                                    onChange={(value) => setMirrorRule(index, (current) => ({ ...current, destination: { ...(current.destination || {}), service: value as string } }))}
                                                />
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    );
                })}
                <Button
                    className={shared.addRuleButton}
                    variant="dashed"
                    icon={<AddIcon />}
                    onClick={() => setRule((prev) => ({ ...(prev as TrafficMirror), rules: [...rules, defaultMirrorSubRule()] }))}
                >
                    添加镜像子规则
                </Button>
            </div>
        );
    };

    const renderEditMockRules = () => {
        const rules = mockRules;
        return (
            <div className={shared.ruleList}>
                {rules.map((source, index) => {
                    const item = normalizeMockRule(source);
                    const isCollapsed = collapsedMockRuleIndexes.has(index);
                    const match = normalizeMockMatchRule(item.traffic_match_rule);
                    const mockDefaultApi = defaultMockSubRule().apis?.[0] || defaultMirrorSubRule().interfaces[0];
                    const apis = item.apis?.length ? item.apis : [item.api || mockDefaultApi].filter(Boolean) as TrafficApiScope[];
                    return (
                        <div className={`${shared.policy} ${isCollapsed ? shared.policyCollapsed : ''}`} key={`mock-rule-edit-${index}`}>
                            <div className={shared.policyHead} onClick={() => toggleMockRuleCollapsed(index)}>
                                <div className={shared.policyHeadMain}>
                                    <span className={shared.caret}><ChevronRightIcon /></span>
                                    <div>
                                        <div className={shared.policyIndex}>Mock 子规则 [{index + 1}]</div>
                                        <div className={shared.policySummary}>
                                            {apis.length} 个接口 · {match.matchMode === MatchLogic.OR ? '满足任一条件即 Mock' : '需同时满足全部条件'}
                                        </div>
                                    </div>
                                </div>
                                <div className={shared.policyHeadActions} onClick={(e) => e.stopPropagation()}>
                                    <Switch
                                        aria-label={`第 ${index + 1} 条 Mock 子规则${!item.disable ? "启用" : "禁用"}`}
                                        label={['启用', '禁用']}
                                        value={!item.disable}
                                        onChange={(value) => setMockRule(index, (current) => ({ ...current, disable: !value }))}
                                    />
                                    <Button
                                        aria-label={`删除第 ${index + 1} 条 Mock 子规则`}
                                        shape="circle"
                                        variant="text"
                                        disabled={rules.length <= 1}
                                        onClick={() => setRule((prev) => ({ ...(prev as TrafficMock), rules: rules.filter((_, idx) => idx !== index) }))}
                                    >
                                        <CloseIcon />
                                    </Button>
                                </div>
                            </div>
                            <div className={shared.policyBody}>
                                <div className={shared.step} data-step="1">
                                    <div className={shared.stepTitle}>选择接口<span className={shared.stepHint}>确定这条 Mock 子规则接管哪些接口</span></div>
                                    <div className={shared.stepContent}>
                                        {renderMirrorInterfacesEditor(index, apis, (next) => setMockRule(index, (current) => ({ ...current, apis: next, api: next[0] })), mockDefaultApi, 'Mock')}
                                    </div>
                                </div>
                                <div className={shared.step} data-step="2">
                                    <div className={shared.stepTitle}>流量匹配<span className={shared.stepHint}>命中接口和匹配条件后直接返回 Mock 响应</span></div>
                                    <div className={shared.stepContent}>
                                        {renderMockMatchRuleEditor(index, item.traffic_match_rule, (next) => setMockRule(index, (current) => ({ ...current, traffic_match_rule: next })))}
                                    </div>
                                </div>
                                <div className={shared.step} data-step="3">
                                    <div className={shared.stepTitle}>Mock 响应结果</div>
                                    <div className={shared.stepContent}>{renderMockResponseEditor(item, index)}</div>
                                </div>
                            </div>
                        </div>
                    );
                })}
                <Button
                    className={shared.addRuleButton}
                    variant="dashed"
                    icon={<AddIcon />}
                    onClick={() => setRule((prev) => ({ ...(prev as TrafficMock), rules: [...rules, defaultMockSubRule()] }))}
                >
                    添加 Mock 子规则
                </Button>
            </div>
        );
    };

    const renderReadonlyPayload = () => {
        if (kind === 'security') return renderSecurityRules();
        if (kind === 'mirror') return renderMirrorRules();
        return renderMockRules();
    };

    const renderEditablePayload = () => {
        if (kind === 'security') return renderEditSecurityRules();
        if (kind === 'mirror') return renderEditMirrorRules();
        return renderEditMockRules();
    };

    const renderPayload = () => (
        <div className={shared.section}>
            <div className={shared.sectionHeader}>
                <span>{kind === 'security' ? '④ 鉴权子规则' : kind === 'mirror' ? '镜像规则' : 'Mock 子规则'}</span>
                <span className={shared.countTag}>{kind === 'security' ? securityViewRules.length : trafficRuleCount(kind, rule)} 条</span>
            </div>
            <div className={shared.sectionBody}>
                {kind === 'mock' && (
                    <div className={style.mockScopeHint}>
                        <div>
                            <div className={style.partitionTitle}>服务范围</div>
                            <div className={style.partitionDesc}>当前 Mock 规则按 {mockCallerScopeText(mockCaller)} → {currentTarget.namespace || '-'}/{currentTarget.service || '-'} 判断流量方向。</div>
                        </div>
                        <div>
                            <div className={style.partitionTitle}>子规则结构</div>
                            <div className={style.partitionDesc}>每条子规则固定按 选择接口 → 流量匹配 → Mock 响应结果 配置。</div>
                        </div>
                    </div>
                )}
                {editable ? renderEditablePayload() : renderReadonlyPayload()}
            </div>
        </div>
    );

    const renderStickyTool = (
        <StickyTool
            style={{ zIndex: 1000 }}
            placement="right-bottom"
            offset={[-10, 200]}
        >
            <StickyItem
                label=""
                icon={editable ? (
                    <RuleStickyAction label="保存" icon={<SaveIcon />} onClick={() => form.submit()} />
                ) : (
                    <span style={rule.editable === false ? { opacity: 0.45, cursor: 'not-allowed' } : undefined}>
                        <RuleStickyAction
                            label="编辑"
                            icon={<Edit1Icon />}
                            onClick={() => {
                                if (rule.editable === false) return;
                                setEditable(true);
                            }}
                        />
                    </span>
                )}
            />
            {editable && (
                <StickyItem
                    label=""
                    icon={(
                        <RuleStickyAction
                            label="撤销"
                            icon={<RollbackIcon />}
                            onClick={() => {
                                if (op === 'create') {
                                    refresh(true);
                                    return;
                                }
                                setEditable(false);
                                setReloadVersion((value) => value + 1);
                            }}
                        />
                    )}
                />
            )}
            {!editable && op !== 'create' && (
                <StickyItem
                    label=""
                    icon={<RuleStickyAction label="发布" icon={<RocketIcon />} onClick={() => setPublishVisible(true)} />}
                />
            )}
        </StickyTool>
    );

    const syncRuleFromForm = (_changed: Record<string, unknown>, values: Record<string, unknown>) => {
        setRule((prev) => {
            const nextBase = {
                ...prev,
                name: String(values.name || prev.name || ''),
                description: String(values.description || ''),
                priority: Number(values.priority ?? prev.priority ?? 0),
                enable: values.enable === undefined ? prev.enable : Boolean(values.enable),
            };
            if (kind === 'mirror') {
                const currentCallee = normalizeMirrorCallee(prev);
                return {
                    ...nextBase,
                    callee: {
                        namespace: String(values.targetNamespace || currentCallee.namespace || ''),
                        service: String(values.targetService || currentCallee.service || ''),
                    },
                } as TrafficMirror;
            }
            return {
                ...nextBase,
                target_service: {
                    namespace: String(values.targetNamespace || prev.target_service?.namespace || ''),
                    service: String(values.targetService || prev.target_service?.service || ''),
                },
            } as TrafficGovernanceRule;
        });
    };

    const formSections = (
        <>
            <CollapsibleSection
                collapsed={basicInfoCollapsed}
                onCollapsedChange={setBasicInfoCollapsed}
                header={kind === 'security' ? '① 基础信息' : '基础信息'}
                summary={`${rule.name || '未命名规则'} · ${rule.enable === false ? '停用' : '启用'} · 优先级 ${rule.priority ?? 0}`}
            >
                {renderCommonFields()}
            </CollapsibleSection>
            {kind === 'security' && renderSecurityServiceInfo()}
            {kind === 'security' && renderSecurityAuthentication()}
            {kind === 'mock' && renderMockServiceScopeSection}
            {kind === 'mirror' && renderMirrorServiceScopeSection}
            {renderPayload()}
        </>
    );

    return (
        <div className={style.editor}>
            <Form form={form} layout="vertical" onSubmit={onSubmit} onValuesChange={syncRuleFromForm}>
                {kind === 'security' ? (
                    <div className={style.securityEditorShell}>
                        <div className={style.formPane}>{formSections}</div>
                    </div>
                ) : kind === 'mirror' ? (
                    <div className={style.securityEditorShell}>
                        <div className={style.formPane}>{formSections}</div>
                    </div>
                ) : kind === 'mock' ? (
                    <div className={style.securityEditorShell}>
                        <div className={style.formPane}>{formSections}</div>
                    </div>
                ) : formSections}
                {renderStickyTool}
            </Form>
            <Dialog
                visible={mockBodyDialog.visible}
                header={(
                    <div>
                        <div>响应 Body 全屏编辑</div>
                        <div className={style.dialogSubTitle}>Mock 子规则 [{mockBodyDialog.index + 1}]</div>
                    </div>
                )}
                onClose={() => setMockBodyDialog({ visible: false, index: -1 })}
                onConfirm={() => setMockBodyDialog({ visible: false, index: -1 })}
                confirmBtn="完成"
                cancelBtn={null}
                width="82vw"
            >
                <div className={style.fullscreenBodyEditor}>
                    <div className={style.sectionHint}>编辑内容会实时同步到当前 Mock 子规则。</div>
                    <Textarea
                        aria-label={`第 ${mockBodyDialog.index + 1} 条 Mock 子规则响应 Body 全屏编辑`}
                        autosize={{ minRows: 20, maxRows: 28 }}
                        value={mockRules[mockBodyDialog.index]?.response?.body || ''}
                        onChange={(value) => setMockRule(mockBodyDialog.index, (current) => ({ ...current, response: { ...(current.response || {}), body: value } }))}
                    />
                </div>
            </Dialog>
            <PublishForm
                ruleId={rule.id || ''}
                ruleName={rule.name || ''}
                resource={TrafficGovernanceAuthResource[kind] as PolicySourceType}
                releaseResource={TrafficGovernanceReleaseResource[kind]}
                visible={publishVisible}
                close={() => setPublishVisible(false)}
            />
        </div>
    );
};

export default React.memo(TrafficGovernanceEditor);
