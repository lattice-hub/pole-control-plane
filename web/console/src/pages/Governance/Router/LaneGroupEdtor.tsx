import React from 'react';
import {
    Button,
    Empty,
    Input,
    InputAdornment,
    InputNumber,
    Select,
    Space,
    StickyTool,
    Switch,
    Tabs,
    Tag,
    Textarea,
} from 'components/Fluent';
import { AddIcon, DeleteIcon, Edit1Icon, RocketIcon, RollbackIcon, SaveIcon } from 'components/Fluent/icons';

import Text from 'components/Text';
import { useAppDispatch, useAppSelector } from 'modules/store';
import { cleanServicePage, listAllServices, selectService } from 'modules/discovery/service';
import { listOneLaneGroup, saveLaneGroups, selectLaneGroup, updateLaneGroups } from 'modules/governance/lane_group';
import { saveLaneRules, updateLaneRules } from 'modules/governance/lane_rule';
import PublishForm from '../RuleRelease/PublishForm';
import RuleStickyAction from '../RuleRelease/RuleStickyAction';
import RuleLabelField from '../shared/RuleLabelField';
import CollapsibleSection from '../shared/CollapsibleSection';
import { GovernanceServiceContext } from '../shared/serviceContext';
import { useRuleNamespace } from '../shared/ruleNamespace';
import TrafficMatchConditionEditor, { TrafficMatchConditionRow } from '../shared/TrafficMatchConditionEditor';
import { PolicySourceType } from 'services/auth_policy';
import { Label, MatchLogic, MatchType, MatchValueType, Op } from 'services/types';
import {
    LaneGatewaySelectorType,
    LaneGroup,
    LaneGroupView,
    LaneMatchLogic,
    LaneRule,
    LaneServiceSelectorType,
    ServiceGatewaySelector,
    ServiceSelector,
    TrafficEntry,
} from 'services/lane';
import { RoutingArgumentsTypeOptions, RoutingRuleDestination, RoutingSourceArgument } from 'services/router';
import { ServiceView } from 'services/service';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import {
    buildLaneTopology,
    LaneDraftCondition,
    LaneDraftEntry,
    LaneDraftRule,
    LaneGroupDraft,
    LANE_TRAFFIC_TAG_KEY,
    laneRuleSummary,
    normalizeLaneGroupDraft,
    validateLaneGroupDraft,
    validateLaneRulesDraft,
} from './laneEditorUtils';
import styles from './LaneGroupEditor.module.less';

const { StickyItem } = StickyTool;
const { TabPanel } = Tabs;

type LanePage = 'lane' | 'group' | 'version' | 'audit';
type ServiceDraftSelector = { ns: string; svc: string };

interface SimpleService {
    label: string;
    value: string;
    service: string;
    namespace: string;
    isGateway?: boolean;
}

interface ILaneGroupEditorProps {
    op: Op;
    refresh: (close: boolean) => void;
    editable?: boolean;
    deleteable?: boolean;
    serviceContext?: GovernanceServiceContext;
}

const defaultLaneGroupDraft = (): LaneGroupDraft => normalizeLaneGroupDraft({
    name: '',
    enabled: true,
    priority: 5,
    desc: '',
    entries: [{ kind: 'gateway', ns: '', svc: '' }],
    selected: [],
    lanes: [],
});

const isGatewayEntry = (entry: TrafficEntry) => {
    const selectorType = (entry.selector as ServiceGatewaySelector & { '@type'?: string })?.['@type'] || '';
    return entry.type === 'gateway' || entry.type?.includes('gateway') || selectorType.includes('ServiceGatewaySelector');
};

const normalizeEntryForDraft = (entry: TrafficEntry): LaneDraftEntry => ({
    kind: isGatewayEntry(entry) ? 'gateway' : 'app',
    ns: entry.selector?.namespace || '',
    svc: entry.selector?.service || '',
});

const matchTypeFromLabel = (label: string) => {
    const map: Record<string, MatchType> = {
        完全匹配: MatchType.EXACT,
        前缀匹配: MatchType.EXACT,
        正则匹配: MatchType.REGEX,
        包含: MatchType.IN,
        不包含: MatchType.NOT_IN,
        不等于: MatchType.NOT_EQUALS,
        不匹配: MatchType.NOT_EQUALS,
        范围匹配: MatchType.RANGE,
    };
    return map[label] || (Object.values(MatchType).includes(label as MatchType) ? label as MatchType : MatchType.EXACT);
};

const labelFromMatchType = (type?: string) => {
    const map: Record<string, string> = {
        [MatchType.EXACT]: '完全匹配',
        [MatchType.REGEX]: '正则匹配',
        [MatchType.IN]: '包含',
        [MatchType.NOT_IN]: '不包含',
        [MatchType.NOT_EQUALS]: '不匹配',
        [MatchType.RANGE]: '范围匹配',
    };
    return map[type || ''] || '完全匹配';
};

const uniqueOptions = (values: string[]) => Array.from(new Set(values.filter(Boolean))).map(value => ({ label: value, value }));

const conditionToArgument = (condition: LaneDraftCondition): RoutingSourceArgument => ({
    type: condition.type,
    key: condition.key,
    value: {
        type: matchTypeFromLabel(condition.match),
        value: condition.value,
        value_type: condition.valueType || MatchValueType.TEXT,
    },
});

const ruleToDraft = (rule: any, selected: string[]): LaneDraftRule => ({
    id: rule.id,
    name: rule.name || '',
    on: rule.enable ?? rule.enabled ?? true,
    open: false,
    laneValue: rule.defaultLabelValue || rule.default_label_value || '',
    relation: rule.trafficMatchRule?.matchMode || rule.traffic_match_rule?.match_mode || MatchLogic.AND,
    matchRatio: rule.matchRatio ?? 100,
    conditions: (rule.trafficMatchRule?.arguments || rule.traffic_match_rule?.arguments || []).map((item: RoutingSourceArgument) => ({
        type: item.type || 'HEADER',
        key: item.key || '',
        match: labelFromMatchType(item.value?.type),
        valueType: item.value?.value_type || MatchValueType.TEXT,
        value: item.value?.value || '',
    })),
    serviceTags: selected.length ? selected.map(service => ({ service })) : [{ service: '' }],
    note: rule.description || '',
});

const LaneGroupEditor: React.FC<ILaneGroupEditorProps> = ({ op, refresh, editable = true, serviceContext }) => {
    const ruleNamespace = useRuleNamespace();
    const dispatch = useAppDispatch();
    const { datas: serviceDatas } = useAppSelector(selectService);
    const { editGroup } = useAppSelector(selectLaneGroup);

    const [draft, setDraft] = React.useState<LaneGroupDraft>(defaultLaneGroupDraft());
    const [basicInfoCollapsed, setBasicInfoCollapsed] = React.useState(false);
    const [editorState, setEditorState] = React.useState({
        editable: op === 'create',
        publishView: false,
    });
    const [lanePage, setLanePage] = React.useState<LanePage>('group');
    const [services, setServices] = React.useState<SimpleService[]>([]);
    const [gatewayServices, setGatewayServices] = React.useState<SimpleService[]>([]);
    const [allServices, setAllServices] = React.useState<SimpleService[]>([]);
    const [serviceDraft, setServiceDraft] = React.useState<ServiceDraftSelector>({ ns: '', svc: '' });

    const serviceByName = React.useMemo(() => {
        const map = new Map<string, SimpleService>();
        allServices.forEach(item => {
            if (!map.has(item.service)) map.set(item.service, item);
        });
        return map;
    }, [allServices]);

    const loadServices = (items: ServiceView[]) => {
        const next = items.map((service): SimpleService => ({
            label: `${service.name} (${service.namespace})`,
            value: `${service.namespace}/${service.name}`,
            namespace: service.namespace,
            service: service.name,
            isGateway: service.metadata?.service_gateway === 'true',
        }));
        setAllServices(next);
        setServices(next.filter(item => !item.isGateway));
        setGatewayServices(next.filter(item => item.isGateway));
    };

    React.useEffect(() => {
        dispatch(listAllServices()).then((res) => {
            if (res.meta.requestStatus === 'rejected') {
                openErrNotification('请求失败', `获取服务列表失败, ${res?.payload as string}`);
                return;
            }
            const payload = res.payload as { datas?: ServiceView[] } | undefined;
            loadServices(payload?.datas || []);
        });
        return () => {
            dispatch(cleanServicePage());
        };
    }, []);

    React.useEffect(() => {
        loadServices(serviceDatas);
    }, [serviceDatas]);

    const applyGroup = (group: LaneGroupView | null) => {
        if (!group) {
            setDraft(defaultLaneGroupDraft());
            return;
        }
        const selected = (group.destinations || []).map(item => item.service).filter(Boolean);
        setDraft(normalizeLaneGroupDraft({
            id: group.id,
            name: group.name,
            desc: group.description,
            entries: (group.entries || []).map(normalizeEntryForDraft),
            selected,
            lanes: (group.rules || []).map(rule => ruleToDraft(rule, selected)),
            metadata: group.metadata || {},
        }));
    };

    React.useEffect(() => {
        setBasicInfoCollapsed(false);
        setEditorState(prev => ({ ...prev, editable: op === 'create' }));
        setLanePage('group');
        if (op === 'create') {
            setDraft(defaultLaneGroupDraft());
            return;
        }
        if (!editGroup?.id) {
            applyGroup(editGroup || null);
            return;
        }
        dispatch(listOneLaneGroup({ id: editGroup.id })).then((res) => {
            if (res.meta.requestStatus === 'rejected') {
                openErrNotification('请求失败', `获取泳道组详情失败, ${res?.payload as string}`);
                return;
            }
            const payload = res.payload as { viewGroup: LaneGroupView } | null;
            applyGroup(payload?.viewGroup || editGroup);
        });
    }, [editGroup?.id, op]);

    React.useEffect(() => {
        if (op !== 'create' || !serviceContext) return;
        setDraft((prev) => {
            if (serviceContext.role === 'caller') {
                const matched = allServices.find((item) => item.namespace === serviceContext.namespace && item.service === serviceContext.service);
                return normalizeLaneGroupDraft({
                    ...prev,
                    entries: [{
                        kind: matched?.isGateway ? 'gateway' : 'app',
                        ns: serviceContext.namespace,
                        svc: serviceContext.service,
                    }, ...prev.entries.slice(1)],
                });
            }
            return normalizeLaneGroupDraft({
                ...prev,
                selected: prev.selected.includes(serviceContext.service)
                    ? prev.selected
                    : [serviceContext.service, ...prev.selected],
            });
        });
    }, [allServices, op, serviceContext?.namespace, serviceContext?.service, serviceContext?.role]);

    const updateDraft = (updater: (prev: LaneGroupDraft) => LaneGroupDraft) => {
        setDraft(prev => normalizeLaneGroupDraft(updater(prev)));
    };

    const updateLane = (index: number, patch: Partial<LaneDraftRule>) => {
        updateDraft(prev => ({
            ...prev,
            lanes: prev.lanes.map((lane, laneIndex) => laneIndex === index ? { ...lane, ...patch } : lane),
        }));
    };

    const updateCondition = (laneIndex: number, conditionIndex: number, patch: Partial<LaneDraftCondition>) => {
        updateDraft(prev => ({
            ...prev,
            lanes: prev.lanes.map((lane, index) => index === laneIndex ? {
                ...lane,
                conditions: lane.conditions.map((condition, idx) => idx === conditionIndex ? { ...condition, ...patch } : condition),
            } : lane),
        }));
    };

    const metadata = React.useMemo(() => draft.tags.reduce((acc, item) => {
        if (item.key) acc[item.key] = item.value;
        return acc;
    }, {} as Record<string, string>), [draft.tags]);

    const serviceReferenceCount = (service: string) => draft.lanes.reduce((total, lane) => (
        total + lane.serviceTags.filter(tag => tag.service === service).length
    ), 0);

    const getEntryServicePool = (entry: LaneDraftEntry) => (entry.kind === 'gateway' ? gatewayServices : services);

    const getNamespaceOptions = (items: SimpleService[]) => uniqueOptions(items.map(item => item.namespace));

    const getServiceOptions = (items: SimpleService[], namespace: string, excluded: string[] = []) => (
        items
            .filter(item => (!namespace || item.namespace === namespace) && !excluded.includes(item.service))
            .map(item => ({ label: item.service, value: item.service }))
    );

    const getServiceByName = (serviceName: string) => (
        serviceContext && serviceName === serviceContext.service
            ? {
                label: `${serviceContext.service} (${serviceContext.namespace})`,
                value: `${serviceContext.namespace}/${serviceContext.service}`,
                namespace: serviceContext.namespace,
                service: serviceContext.service,
            }
            : serviceByName.get(serviceName)
    ) || {
        label: serviceName,
        value: serviceName,
        namespace: '',
        service: serviceName,
    };

    const buildGroupPayload = (): LaneGroup => ({
        id: draft.id || editGroup?.id || '',
        namespace: (editGroup as (LaneGroup & { namespace?: string }) | undefined)?.namespace || ruleNamespace,
        name: draft.name,
        description: draft.desc,
        entries: draft.entries.map((entry) => ({
            type: entry.kind === 'gateway' ? 'gateway' : 'service',
            selector: {
                '@type': entry.kind === 'gateway' ? LaneGatewaySelectorType : LaneServiceSelectorType,
                namespace: entry.ns,
                service: entry.svc,
            } as (ServiceGatewaySelector | ServiceSelector) & { '@type': string },
        } as TrafficEntry)),
        destinations: draft.selected.map((serviceName) => {
            const service = getServiceByName(serviceName);
            return {
                name: `${service.namespace}/${service.service}`,
                namespace: service.namespace,
                service: service.service,
            } as RoutingRuleDestination;
        }),
        metadata: draft.tags.reduce((acc, item) => {
            if (item.key) acc[item.key] = item.value;
            return acc;
        }, {} as Record<string, string>),
    });

    const buildLanePayload = (lane: LaneDraftRule): LaneRule => ({
        id: lane.id || '',
        groupName: draft.name,
        name: lane.name,
        description: lane.note || '',
        priority: draft.priority,
        enable: lane.on,
        matchMode: LaneMatchLogic.PERMISSIVE,
        trafficMatchRule: {
            matchMode: lane.relation,
            arguments: lane.conditions.map(conditionToArgument),
        },
        defaultLabelValue: lane.laneValue,
        labelKey: LANE_TRAFFIC_TAG_KEY,
    });

    const saveDraft = async () => {
        const issues = [...validateLaneGroupDraft(draft), ...validateLaneRulesDraft(draft)];
        if (issues.length) {
            openErrNotification('配置错误', issues[0].message);
            return;
        }
        const groupAction = op === 'create'
            ? await dispatch(saveLaneGroups({ param: buildGroupPayload() }))
            : await dispatch(updateLaneGroups({ param: buildGroupPayload() }));
        if (groupAction.meta.requestStatus !== 'fulfilled') {
            openErrNotification('请求错误', `${op === 'create' ? '创建' : '修改'}泳道组失败, ${groupAction?.payload as string}`);
            return;
        }
        if (op !== 'create') {
            for (const lane of draft.lanes) {
                const action = lane.id
                    ? await dispatch(updateLaneRules({ param: buildLanePayload(lane) }))
                    : await dispatch(saveLaneRules({ param: buildLanePayload(lane) }));
                if (action.meta.requestStatus !== 'fulfilled') {
                    openErrNotification('请求错误', `保存泳道 ${lane.name || '-'} 失败, ${action?.payload as string}`);
                    return;
                }
            }
        }
        openInfoNotification('请求成功', '已保存并下发到数据面');
        setEditorState(prev => ({ ...prev, editable: false }));
        refresh(op === 'create');
    };

    const renderField = (label: string, value: React.ReactNode, className = '') => (
        <div className={`${styles.infoItem} ${className}`}>
            <div className={styles.infoLabel}>{label}</div>
            <div className={styles.infoValue}>{value || '-'}</div>
        </div>
    );

    const renderEditField = (label: string, control: React.ReactNode, className = '') => (
        <div className={`${styles.editField} ${className}`}>
            <div className={styles.editLabel}>{label}</div>
            {control}
        </div>
    );

    const renderStepTitle = (step: number, title: string, desc: string) => (
        <div className={styles.stepTitle}>
            <span className={styles.designSectionNumber}>{step}</span>
            <span className={styles.stepName}>{title}</span>
            <span className={styles.designSectionDesc}>{desc}</span>
        </div>
    );

    const renderGroupPage = () => (
        <div className={styles.formStack}>
            <CollapsibleSection
                className={styles.designSection}
                headerClassName={styles.designSectionHeader}
                bodyClassName={styles.designSectionBody}
                collapsed={basicInfoCollapsed}
                onCollapsedChange={setBasicInfoCollapsed}
                header={renderStepTitle(1, '基础信息', '规则的标识与基本属性')}
                summary={`${draft.name || '未命名规则'} · ${draft.enabled ? '启用' : '停用'} · 优先级 ${draft.priority}`}
            >
                    <div className={styles.infoGrid}>
                        {editorState.editable ? (
                            <>
                                {renderEditField('规则名称', (
                                    <Input aria-label="泳道组规则名称" value={draft.name} placeholder="spec-check-lane-group" onChange={(value) => updateDraft(prev => ({ ...prev, name: value, laneGroup: { ...prev.laneGroup, name: value } }))} />
                                ), styles.formField)}
                                {renderEditField('优先级', (
                                    <InputNumber aria-label="泳道组规则优先级" theme="normal" min={0} value={draft.priority} onChange={(value) => updateDraft(prev => ({ ...prev, priority: Number(value || 0) }))} />
                                ), styles.formField)}
                                {renderEditField('描述', (
                                    <Textarea aria-label="泳道组规则描述" value={draft.desc} autosize={{ minRows: 2, maxRows: 4 }} onChange={(value) => updateDraft(prev => ({ ...prev, desc: value }))} />
                                ), `${styles.formField} ${styles.fullWidth}`)}
                                {renderEditField('规则标签', (
                                    <RuleLabelField
                                        metadata={metadata}
                                        editable
                                        onChange={(next) => updateDraft(prev => ({ ...prev, tags: Object.entries(next).map(([key, value]) => ({ key, value })) }))}
                                    />
                                ), `${styles.formField} ${styles.fullWidth}`)}
                                <div className={`${styles.formField} ${styles.fullWidth}`}>
                                    <Switch aria-label="泳道组启用状态" value={draft.enabled} onChange={(value) => updateDraft(prev => ({ ...prev, enabled: value }))} />
                                    <span className={styles.inlineHint}>启用状态 - 保存后立即下发到数据面</span>
                                </div>
                            </>
                        ) : (
                            <>
                                {renderField('规则名称', draft.name)}
                                {renderField('优先级', draft.priority)}
                                {renderField('描述', draft.desc || '暂无描述', styles.fullWidth)}
                                <div className={`${styles.infoItem} ${styles.fullWidth}`}>
                                    <div className={styles.infoLabel}>规则标签</div>
                                    <RuleLabelField metadata={metadata} />
                                </div>
                                {renderField('启用状态', draft.enabled ? '启用' : '停用', styles.fullWidth)}
                            </>
                        )}
                    </div>
            </CollapsibleSection>

            <section className={styles.designSection}>
                <div className={styles.designSectionHeader}>
                    {renderStepTitle(2, '泳道组入口', '配置哪些网关或应用入口进入该泳道组判定')}
                    {editorState.editable && <Button size="small" variant="outline" icon={<AddIcon />} onClick={() => updateDraft(prev => ({ ...prev, entries: [...prev.entries, { kind: 'gateway', ns: '', svc: '' }] }))}>新增入口</Button>}
                </div>
                <div className={styles.designSectionBody}>
                    <div className={styles.prdTable}>
                        <div className={`${styles.prdTableRow} ${styles.prdTableHeader}`}>
                            <div>#</div>
                            <div>入口类型</div>
                            <div>命名空间</div>
                            <div>服务</div>
                            <div />
                        </div>
                        {draft.entries.map((entry, index) => {
                            const servicePool = getEntryServicePool(entry);
                            const fixedEntry = op === 'create' && serviceContext?.role === 'caller' && index === 0;
                            return (
                                <div className={styles.prdTableRow} key={`${entry.kind}-${entry.ns}-${entry.svc}-${index}`}>
                                    <div><span className={styles.indexPill}>{index + 1}</span></div>
                                    {editorState.editable && !fixedEntry && !entry.svc ? (
                                        <Select aria-label={`第 ${index + 1} 个泳道组入口类型`} value={entry.kind} options={[{ label: '网关入口', value: 'gateway' }, { label: '应用入口', value: 'app' }]} onChange={(value) => updateDraft(prev => ({ ...prev, entries: prev.entries.map((item, idx) => idx === index ? { ...item, kind: value as 'gateway' | 'app', ns: '', svc: '' } : item) }))} />
                                    ) : (
                                        <span className={entry.kind === 'gateway' ? styles.gatewayPill : styles.appPill}>{entry.kind === 'gateway' ? '网关入口' : '应用入口'}</span>
                                    )}
                                    {editorState.editable && !fixedEntry ? (
                                        <Select
                                            aria-label={`第 ${index + 1} 个泳道组入口命名空间`}
                                            filterable
                                            value={entry.ns || undefined}
                                            options={getNamespaceOptions(servicePool)}
                                            placeholder="选择命名空间"
                                            onChange={(value) => {
                                                const namespace = value as string;
                                                updateDraft(prev => ({ ...prev, entries: prev.entries.map((item, idx) => {
                                                    if (idx !== index) return item;
                                                    const nextPool = item.kind === 'gateway' ? gatewayServices : services;
                                                    const keepService = nextPool.some(service => service.namespace === namespace && service.service === item.svc);
                                                    return { ...item, ns: namespace, svc: keepService ? item.svc : '' };
                                                }) }));
                                            }}
                                        />
                                    ) : <Text title={entry.ns || '-'}>{entry.ns || '-'}</Text>}
                                    {editorState.editable && !fixedEntry ? (
                                        <Select
                                            aria-label={`第 ${index + 1} 个泳道组入口服务`}
                                            filterable
                                            value={entry.svc || undefined}
                                            options={getServiceOptions(servicePool, entry.ns)}
                                            placeholder="选择服务"
                                            disabled={!entry.ns}
                                            onChange={(value) => updateDraft(prev => ({ ...prev, entries: prev.entries.map((item, idx) => idx === index ? { ...item, svc: value as string } : item) }))}
                                        />
                                    ) : <Text title={entry.svc || '-'}>{entry.svc || '-'}</Text>}
                                    {editorState.editable && !fixedEntry && draft.entries.length > 1 && (
                                        <Button shape="square" variant="text" aria-label={`删除入口 ${index + 1}`} icon={<DeleteIcon />} onClick={() => updateDraft(prev => ({ ...prev, entries: prev.entries.filter((_, idx) => idx !== index) }))} />
                                    )}
                                </div>
                            );
                        })}
                    </div>
                </div>
            </section>

            <section className={styles.designSection}>
                <div className={styles.designSectionHeader}>
                    {renderStepTitle(3, '组内服务', '维护泳道组服务集合，已加入服务只允许删除')}
                </div>
                <div className={styles.designSectionBody}>
                    {draft.selected.length === 0 && !editorState.editable ? <Empty title="暂无组内服务" /> : (
                        <div className={`${styles.prdTable} ${styles.serviceTable}`}>
                            <div className={`${styles.prdTableRow} ${styles.prdTableHeader}`}>
                                <div>#</div>
                                <div>命名空间</div>
                                <div>服务</div>
                                <div>引用</div>
                                <div />
                            </div>
                            {draft.selected.map((serviceName, index) => {
                                const service = getServiceByName(serviceName);
                                const referenceCount = serviceReferenceCount(serviceName);
                                const rowNamespace = service.namespace || '';
                                const fixedDestination = op === 'create' && serviceContext?.role === 'callee' && serviceName === serviceContext.service;
                                return (
                                    <div className={styles.prdTableRow} key={serviceName}>
                                        <div><span className={styles.indexPill}>{index + 1}</span></div>
                                        {editorState.editable && !fixedDestination ? (
                                            <Select
                                                aria-label={`第 ${index + 1} 个组内服务命名空间`}
                                                filterable
                                                value={rowNamespace || undefined}
                                                options={getNamespaceOptions(services)}
                                                placeholder="选择命名空间"
                                                onChange={(value) => {
                                                    const namespace = value as string;
                                                    updateDraft(prev => ({ ...prev, selected: prev.selected.map((item, idx) => {
                                                        if (idx !== index) return item;
                                                        const excluded = prev.selected.filter((_, selectedIndex) => selectedIndex !== index);
                                                        const keepService = services.some(serviceItem => serviceItem.namespace === namespace && serviceItem.service === serviceName);
                                                        const fallback = services.find(serviceItem => serviceItem.namespace === namespace && !excluded.includes(serviceItem.service));
                                                        return keepService ? item : fallback?.service || item;
                                                    }) }));
                                                }}
                                            />
                                        ) : <Text title={rowNamespace || '-'}>{rowNamespace || '-'}</Text>}
                                        {editorState.editable && !fixedDestination ? (
                                            <Select
                                                aria-label={`第 ${index + 1} 个组内服务`}
                                                filterable
                                                value={serviceName || undefined}
                                                options={getServiceOptions(services, rowNamespace, draft.selected.filter((_, idx) => idx !== index))}
                                                placeholder="选择服务"
                                                disabled={!rowNamespace}
                                                onChange={(value) => updateDraft(prev => ({ ...prev, selected: prev.selected.map((item, idx) => idx === index ? value as string : item) }))}
                                            />
                                        ) : <Text title={service.service}>{service.service}</Text>}
                                        <Text title={`${referenceCount} 条泳道引用`}>{referenceCount} 条泳道引用</Text>
                                        {editorState.editable && !fixedDestination && (
                                            <Button shape="square" variant="text" aria-label={`删除组内服务 ${serviceName}`} icon={<DeleteIcon />} onClick={() => updateDraft(prev => ({ ...prev, selected: prev.selected.filter(item => item !== serviceName) }))} />
                                        )}
                                    </div>
                                );
                            })}
                            {editorState.editable && (
                                <div className={`${styles.prdTableRow} ${styles.serviceDraftRow}`}>
                                    <div><span className={styles.indexPill}>{draft.selected.length + 1}</span></div>
                                    <Select
                                        aria-label="新增组内服务命名空间"
                                        filterable
                                        value={serviceDraft.ns || undefined}
                                        options={getNamespaceOptions(services)}
                                        placeholder="选择命名空间"
                                        onChange={(value) => setServiceDraft({ ns: value as string, svc: '' })}
                                    />
                                    <Select
                                        aria-label="新增组内服务"
                                        filterable
                                        value={serviceDraft.svc || undefined}
                                        options={getServiceOptions(services, serviceDraft.ns, draft.selected)}
                                        placeholder="选择服务"
                                        disabled={!serviceDraft.ns}
                                        onChange={(value) => {
                                            const service = value as string;
                                            updateDraft(prev => ({ ...prev, selected: [...prev.selected, service] }));
                                            setServiceDraft({ ns: '', svc: '' });
                                        }}
                                    />
                                    <Text>新增后可被泳道引用</Text>
                                    <Button shape="square" variant="text" aria-label="删除待添加服务" icon={<DeleteIcon />} disabled />
                                </div>
                            )}
                        </div>
                    )}
                </div>
            </section>
        </div>
    );

    const renderLaneTopology = (lane: LaneDraftRule) => {
        const topology = buildLaneTopology(draft, lane);
        return (
            <div className={styles.topology}>
                <svg viewBox="0 0 760 230" role="img">
                    <defs>
                        <marker id="lane-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                            <path d="M 0 0 L 10 5 L 0 10 z" fill="#2ba471" />
                        </marker>
                    </defs>
                    <rect x="24" y="88" width="120" height="42" rx="6" className={styles.node} />
                    <text x="84" y="114" textAnchor="middle">{topology.entryLabel}</text>
                    <line x1="144" y1="109" x2="250" y2="109" className={styles.grayLine} />
                    <rect x="250" y="82" width="120" height="54" rx="6" className={styles.matchNode} />
                    <text x="310" y="105" textAnchor="middle">流量匹配</text>
                    <text x="310" y="124" textAnchor="middle" className={styles.smallSvgText}>{topology.matchLabel}</text>
                    <line x1="370" y1="109" x2="500" y2="70" className={styles.greenLine} markerEnd="url(#lane-arrow)" />
                    <text x="440" y="76" textAnchor="middle" className={styles.greenText}>放量 {topology.hitRatio}%</text>
                    <line x1="370" y1="109" x2="500" y2="160" className={styles.fallbackLine} />
                    <text x="440" y="150" textAnchor="middle" className={styles.grayText}>{topology.fallbackLabel} {topology.fallbackRatio}%</text>
                    <polygon points="520,42 610,70 520,98" className={styles.laneShape} />
                    <text x="550" y="73" textAnchor="middle">{topology.laneLabel}</text>
                    <rect x="512" y="142" width="132" height="42" rx="6" className={styles.baseNode} />
                    <text x="578" y="168" textAnchor="middle">{topology.hitRatio === 100 ? '未命中 → 基线版本' : '基线版本'}</text>
                    <g>
                        {topology.visibleServices.map((service, index) => (
                            <React.Fragment key={service}>
                                <line x1="610" y1="70" x2="680" y2={48 + index * 34} className={styles.greenLineThin} />
                                <rect x="680" y={34 + index * 34} width="64" height="26" rx="5" className={styles.serviceNode} />
                                <text x="712" y={51 + index * 34} textAnchor="middle" className={styles.smallSvgText}>{service}</text>
                            </React.Fragment>
                        ))}
                        {topology.omittedServiceCount > 0 && <text x="712" y="150" textAnchor="middle" className={styles.smallSvgText}>省略 {topology.omittedServiceCount} 个</text>}
                    </g>
                </svg>
                <div className={styles.legend}>
                    <span><i className={styles.legendGreen} />命中放量进泳道</span>
                    <span><i className={styles.legendGray} />回落·未命中 → 基线</span>
                    <span><i className={styles.legendService} />本规则选择的服务</span>
                </div>
            </div>
        );
    };

    const renderLanePage = () => (
        <div className={styles.formStack}>
            <section className={styles.designSection}>
                <div className={styles.designSectionHeader}>
                    <div>
                        <div className={styles.designSectionTitle}>泳道定义</div>
                        <div className={styles.designSectionDesc}>每条定义对应泳道组内的一条泳道规则。</div>
                    </div>
                </div>
                <div className={styles.designSectionBody}>
                    {draft.selected.length === 0 && <Empty title="需先创建泳道组服务" />}
                    {draft.selected.length > 0 && draft.lanes.map((lane, laneIndex) => (
                        <div className={styles.laneCard} key={`lane-card-${laneIndex}`}>
                            <div className={styles.laneCardHeader}>
                                <Button variant="text" type="button" className={styles.laneExpandIcon} aria-label={`${lane.open ? '收起' : '展开'}泳道 ${laneIndex + 1}`} onClick={() => updateLane(laneIndex, { open: !lane.open })}>
                                    {lane.open ? '⌄' : '›'}
                                </Button>
                                <span className={styles.laneColorDot} />
                                <div className={styles.laneHeaderText}>
                                    <span className={styles.laneName}>{lane.name || '未命名泳道'}</span>
                                    <span className={styles.laneSummary}>{laneRuleSummary(lane)}</span>
                                </div>
                                <Space>
                                    <Switch aria-label={`第 ${laneIndex + 1} 个泳道启用状态`} size="small" disabled={!editorState.editable} value={lane.on} onChange={(value) => updateLane(laneIndex, { on: value })} />
                                    {editorState.editable && (
                                        <Button
                                            shape="square"
                                            variant="text"
                                            aria-label={`删除泳道 ${laneIndex + 1}`}
                                            disabled={draft.lanes.length <= 1}
                                            icon={<DeleteIcon />}
                                            onClick={() => updateDraft(prev => ({ ...prev, lanes: prev.lanes.filter((_, idx) => idx !== laneIndex) }))}
                                        />
                                    )}
                                </Space>
                            </div>
                            {lane.open && (
                                <div className={styles.laneCardBody}>
                                    <div className={styles.infoGrid}>
                                        {editorState.editable ? (
                                            <>
                                                {renderEditField('泳道名称', (
                                                    <Input aria-label={`第 ${laneIndex + 1} 个泳道名称`} value={lane.name} placeholder="shadow-lane" onChange={(value) => updateLane(laneIndex, { name: value })} />
                                                ), styles.formField)}
                                                {renderEditField('泳道标签 Value', (
                                                    <Input aria-label={`第 ${laneIndex + 1} 个泳道标签值`} value={lane.laneValue} placeholder="shadow" onChange={(value) => updateLane(laneIndex, { laneValue: value })} />
                                                ), styles.formField)}
                                            </>
                                        ) : (
                                            <>
                                                {renderField('泳道名称', lane.name)}
                                                {renderField('泳道标签 Value', lane.laneValue)}
                                            </>
                                        )}
                                    </div>
                                    <div className={styles.fixedTag}>
                                        <span>固定标签 Key</span>
                                        <strong>{LANE_TRAFFIC_TAG_KEY}</strong>
                                        <span>=</span>
                                        <strong>{lane.laneValue || '-'}</strong>
                                    </div>

                                    <div className={styles.conditionPanel}>
                                        {(() => {
                                            const rows: TrafficMatchConditionRow[] = lane.conditions.map(condition => ({
                                                paramType: condition.type,
                                                paramKey: condition.key,
                                                matchType: matchTypeFromLabel(condition.match),
                                                valueType: condition.valueType || MatchValueType.TEXT,
                                                matchValue: condition.value,
                                            }));
                                            return (
                                                <TrafficMatchConditionEditor
                                                    contextLabel={`第 ${laneIndex + 1} 个泳道`}
                                                    rows={rows}
                                                    editable={Boolean(editorState.editable)}
                                                    relation={lane.relation}
                                                    title={(
                                                        <span className={styles.laneSubTitle}>
                                                            <span className={styles.laneStepBadge}>1</span>
                                                            <span className={styles.subPanelTitle}>流量匹配规则</span>
                                                        </span>
                                                    )}
                                                    hint={lane.relation === 'OR' ? '满足任一规则即可进入泳道' : '满足全部规则才可进入泳道'}
                                                    addText="添加匹配规则"
                                                    paramTypeOptions={RoutingArgumentsTypeOptions}
                                                    extraControl={(
                                                        <div className={styles.laneRatioControl}>
                                                            <span className={styles.ratioLabel}>命中后放量</span>
                                                            <InputAdornment append="%">
                                                                <InputNumber aria-label={`第 ${laneIndex + 1} 个泳道命中后放量比例`} theme="normal" disabled={!editorState.editable} min={0} max={100} value={lane.matchRatio} onChange={(value) => updateLane(laneIndex, { matchRatio: Number(value || 0) })} />
                                                            </InputAdornment>
                                                        </div>
                                                    )}
                                                    onRelationChange={(value) => updateLane(laneIndex, { relation: value as 'AND' | 'OR' })}
                                                    onRowChange={(conditionIndex, row) => updateCondition(laneIndex, conditionIndex, {
                                                        type: row.paramType || 'HEADER',
                                                        key: row.paramKey || '',
                                                        match: labelFromMatchType(row.matchType),
                                                        valueType: row.valueType || MatchValueType.TEXT,
                                                        value: row.valueType === MatchValueType.PARAMETER ? '' : row.matchValue || '',
                                                    })}
                                                    onAdd={() => updateDraft(prev => ({ ...prev, lanes: prev.lanes.map((item, idx) => idx === laneIndex ? { ...item, conditions: [...item.conditions, { type: 'HEADER', key: '', match: '完全匹配', valueType: MatchValueType.TEXT, value: '' }] } : item) }))}
                                                    onRemove={(conditionIndex) => updateDraft(prev => ({ ...prev, lanes: prev.lanes.map((item, idx) => idx === laneIndex ? { ...item, conditions: item.conditions.filter((_, cidx) => cidx !== conditionIndex) } : item) }))}
                                                />
                                            );
                                        })()}
                                        <div className={styles.matchRatioHint}>
                                            命中匹配条件的流量中，按 <strong>{lane.matchRatio}%</strong> 放量进入本泳道，其余回落基线版本（100% 即全量进入）。
                                        </div>
                                    </div>

                                    <div className={styles.laneServiceSection}>
                                        <div className={styles.laneSubTitle}>
                                            <span className={styles.laneStepBadge}>2</span>
                                            <span className={styles.subPanelTitle}>进入泳道的组内服务</span>
                                            <span className={styles.subPanelHint}>只能选择当前泳道组已有服务</span>
                                        </div>
                                        <div className={styles.serviceTagList}>
                                            {lane.serviceTags.map((serviceTag, serviceIndex) => (
                                                <div className={styles.serviceTagRow} key={`lane-service-tag-${serviceIndex}`}>
                                                    {editorState.editable ? (
                                                        <Select aria-label={`第 ${laneIndex + 1} 个泳道的第 ${serviceIndex + 1} 个服务`} value={serviceTag.service} options={draft.selected.map(service => ({ label: service, value: service }))} onChange={(value) => updateDraft(prev => ({ ...prev, lanes: prev.lanes.map((item, idx) => idx === laneIndex ? { ...item, serviceTags: item.serviceTags.map((tag, tidx) => tidx === serviceIndex ? { service: value as string } : tag) } : item) }))} />
                                                    ) : (
                                                        <Text title={serviceTag.service}>{serviceTag.service}</Text>
                                                    )}
                                                    <Input aria-label={`第 ${laneIndex + 1} 个泳道的第 ${serviceIndex + 1} 个服务标签`} disabled value={`${LANE_TRAFFIC_TAG_KEY}=${lane.laneValue || '-'}`} />
                                                    {editorState.editable && (
                                                        <Button
                                                            shape="square"
                                                            variant="text"
                                                            aria-label={`删除第 ${laneIndex + 1} 个泳道的第 ${serviceIndex + 1} 个服务`}
                                                            disabled={lane.serviceTags.length <= 1}
                                                            icon={<DeleteIcon />}
                                                            onClick={() => updateDraft(prev => ({ ...prev, lanes: prev.lanes.map((item, idx) => idx === laneIndex ? { ...item, serviceTags: item.serviceTags.filter((_, sidx) => sidx !== serviceIndex) } : item) }))}
                                                        />
                                                    )}
                                                </div>
                                            ))}
                                        </div>
                                        {editorState.editable && <Button className={styles.laneTextAdd} variant="text" icon={<AddIcon />} onClick={() => updateDraft(prev => ({ ...prev, lanes: prev.lanes.map((item, idx) => idx === laneIndex ? { ...item, serviceTags: [...item.serviceTags, { service: prev.selected[0] || '' }] } : item) }))}>添加组内服务</Button>}
                                        <div className={styles.laneResultHint}>命中 <strong>{lane.name || '当前泳道'}</strong> 后，所选组内服务会按固定标签 <strong>{LANE_TRAFFIC_TAG_KEY}={lane.laneValue || '-'}</strong> 进入对应泳道。</div>
                                    </div>

                                    <div className={styles.topologyPanel}>
                                        <div className={styles.laneSubTitle}>
                                            <span className={styles.laneStepBadge}>3</span>
                                            <span className={styles.subPanelTitle}>流量拓扑演示</span>
                                            <span className={styles.subPanelHint}>展示本条泳道规则命中后的流量路径</span>
                                        </div>
                                        {renderLaneTopology(lane)}
                                    </div>
                                </div>
                            )}
                        </div>
                    ))}
                    {editorState.editable && draft.selected.length > 0 && (
                        <Button
                            className={styles.addLaneCardButton}
                            variant="outline"
                            icon={<AddIcon />}
                            onClick={() => updateDraft(prev => ({ ...prev, lanes: [...prev.lanes, { name: '', on: true, open: true, laneValue: '', relation: 'AND', matchRatio: 100, conditions: [{ type: 'HEADER', key: '', match: '完全匹配', valueType: MatchValueType.TEXT, value: '' }], serviceTags: [{ service: prev.selected[0] || '' }] }] }))}
                        >
                            新建泳道
                        </Button>
                    )}
                </div>
            </section>
        </div>
    );

    const renderVersionPage = () => (
        <div className={styles.placeholderPanel}>
            <Empty title="版本快照只读" description="历史版本仍在治理详情的版本数据源中维护；右侧展示当前 LaneGroupVersion 预览。" />
        </div>
    );

    const renderAuditPage = () => (
        <div className={styles.placeholderPanel}>
            <Empty title="审计记录只读" description="操作记录不参与编辑。" />
        </div>
    );

    return (
        <div className={styles.editorBody}>
            <div className={styles.laneEditorShell}>
                <div className={styles.formPane}>
                    <Tabs value={lanePage} onChange={(value) => setLanePage(value as LanePage)}>
                        <TabPanel label="组信息" value="group">
                            {renderGroupPage()}
                        </TabPanel>
                        <TabPanel label="泳道" value="lane" disabled={op === 'create'}>
                            {renderLanePage()}
                        </TabPanel>
                        <TabPanel label="版本" value="version" disabled={op === 'create'}>
                            {renderVersionPage()}
                        </TabPanel>
                        <TabPanel label="审计" value="audit" disabled={op === 'create'}>
                            {renderAuditPage()}
                        </TabPanel>
                    </Tabs>
                </div>
            </div>
            {editorState.publishView && (
                <PublishForm
                    ruleId={editGroup?.id || ''}
                    ruleName={editGroup?.name || draft.name}
                    resource={PolicySourceType.LaneRules}
                    visible={editorState.publishView}
                    close={() => setEditorState(prev => ({ ...prev, publishView: false }))}
                />
            )}
            <StickyTool style={{ zIndex: 1000 }} placement="right-bottom" offset={[-10, 200]}>
                {editorState.editable ? (
                    <StickyItem label="" icon={<RuleStickyAction label="保存" icon={<SaveIcon />} onClick={saveDraft} />} />
                ) : editable !== false && (
                    <StickyItem label="" icon={<RuleStickyAction label="编辑" icon={<Edit1Icon />} onClick={() => setEditorState(prev => ({ ...prev, editable: true }))} />} />
                )}
                {editorState.editable && (
                    <StickyItem label="" icon={<RuleStickyAction label="撤销" icon={<RollbackIcon />} onClick={() => {
                        if (op === 'create') {
                            refresh(true);
                            return;
                        }
                        applyGroup(editGroup);
                        setEditorState(prev => ({ ...prev, editable: false }));
                    }} />} />
                )}
                {!editorState.editable && op !== 'create' && editable !== false && (
                    <StickyItem label="" icon={<RuleStickyAction label="发布" icon={<RocketIcon />} onClick={() => setEditorState(prev => ({ ...prev, publishView: true }))} />} />
                )}
            </StickyTool>
        </div>
    );
};

export default React.memo(LaneGroupEditor);
