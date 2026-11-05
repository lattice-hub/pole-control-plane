import React from 'react';
import {
    Button,
    Form,
    FormProps,
    Input,
    InputAdornment,
    InputNumber,
    RadioGroup,
    Select,
    Space,
    StickyTool,
    Switch,
    Tag,
    Textarea,
} from 'components/Fluent';
import { Edit1Icon, RocketIcon, RollbackIcon, SaveIcon } from 'components/Fluent/icons';

import RuleLabelField from '../shared/RuleLabelField';
import CollapsibleSection from '../shared/CollapsibleSection';
import { GovernanceServiceContext } from '../shared/serviceContext';
import { useRuleNamespace } from '../shared/ruleNamespace';
import shared from '../shared/governance.module.less';
import styles from './index.module.less';
import { Label, Op } from 'services/types';
import { cleanNamespacePage, listAllNamespaces, selectNamespace } from 'modules/namespace';
import { useAppDispatch, useAppSelector } from 'modules/store';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import { cleanServicePage, listAllServices, selectService } from 'modules/discovery/service';
import { listOneLossLessRule, saveLossLessRule, selectLosslessRule, updateLosslessRule } from 'modules/governance/lossless';
import { LossLessRuleView } from 'services/lossless';
import PublishForm from '../RuleRelease/PublishForm';
import RuleStickyAction from '../RuleRelease/RuleStickyAction';
import { PolicySourceType } from 'services/auth_policy';
import { ServiceView } from 'services/service';
import { NamespaceView } from 'services/namespace';
import {
    buildLosslessSubmitPayload,
    buildWarmupCurvePoints,
    defaultLosslessRuleDraft,
    delayStrategyOptions,
    describeDelaySummary,
    describeOfflineSummary,
    describeWarmupCurve,
    describeWarmupSummary,
    httpMethodOptions,
    LosslessPayloadMatch,
    LosslessProbeProtocol,
    LosslessRuleDraft,
    metadataToRecord,
    normalizeLosslessRuleDraft,
    payloadMatchOptions,
    payloadMatchText,
    protocolOptions,
    validateLosslessDraft,
} from './losslessEditorUtils';

const { FormItem } = Form;
const { StickyItem } = StickyTool;

export interface LossLessRule {
    id?: string;
    service: string;
    namespace: string;
    lossless_online: LosslessRuleDraft['lossless_online'];
    lossless_offline: LosslessRuleDraft['lossless_offline'];
    metadata?: Label[];
}

export interface LossLessEditorProps {
    op: Op;
    visible: boolean;
    refresh: (close: boolean) => void;
    serviceContext?: GovernanceServiceContext;
}

const protocolText: Record<LosslessProbeProtocol, string> = {
    HTTP: 'HTTP',
    TCP: 'TCP',
    UDP: 'UDP',
};

const curveChartBox = {
    width: 360,
    height: 220,
    left: 42,
    right: 18,
    top: 18,
    bottom: 34,
};

const LossLessEditor: React.FC<LossLessEditorProps> = ({ op, refresh, visible, serviceContext }) => {
    const ruleNamespace = useRuleNamespace();
    const [form] = Form.useForm();
    const dispatch = useAppDispatch();

    const { datas: namespaceDatas } = useAppSelector(selectNamespace);
    const { datas: serviceDatas } = useAppSelector(selectService);
    const { editRule } = useAppSelector(selectLosslessRule);

    const [losslessRule, setLosslessRule] = React.useState<LosslessRuleDraft>(() => defaultLosslessRuleDraft());
    const [basicInfoCollapsed, setBasicInfoCollapsed] = React.useState(false);
    const [editor, setEditor] = React.useState<{
        editable: boolean;
        publishView: boolean;
        saving: boolean;
    }>({ editable: op === 'create', publishView: false, saving: false });
    const persistedRuleRef = React.useRef<LosslessRuleDraft>(defaultLosslessRuleDraft());

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
            setEditor({ editable: op === 'create', publishView: false, saving: false });
            setLosslessRule(defaultLosslessRuleDraft());
            return;
        }
        setEditor({ editable: op === 'create', publishView: false, saving: false });
    }, [visible, op]);

    React.useEffect(() => {
        if (!editRule) return;
        if (editRule.id) {
            dispatch(listOneLossLessRule({ id: editRule.id || '' })).then((res) => {
                if (res.meta.requestStatus === 'fulfilled') {
                    const ret = res.payload as { viewRule: LossLessRuleView } | null;
                    resetCurRule(ret?.viewRule || null);
                } else {
                    openErrNotification('请求错误', `获取无损规则详情失败: ${res.payload as string}`);
                }
            });
        } else {
            resetCurRule(editRule);
        }
    }, [editRule?.id]);

    React.useEffect(() => {
        if (op !== 'create' || !visible) return;
        setLosslessRule((prev) => ({
            ...prev,
            namespace: serviceContext?.namespace || ruleNamespace || prev.namespace,
            service: serviceContext?.service || prev.service,
        }));
    }, [op, visible, ruleNamespace, serviceContext?.namespace, serviceContext?.service]);

    const resetCurRule = (rule: LossLessRuleView | null) => {
        if (!rule) {
            const next = defaultLosslessRuleDraft();
            persistedRuleRef.current = next;
            setLosslessRule(next);
            return;
        }
        const next = normalizeLosslessRuleDraft(rule);
        persistedRuleRef.current = next;
        setLosslessRule(next);
    };

    const namespaceSelectOptions = namespaceDatas.map((ns: NamespaceView) => ({ label: ns.name, value: ns.name }));
    const serviceSelectOptions = serviceDatas
        .filter((opt: ServiceView) => !losslessRule.namespace || losslessRule.namespace === '*' || opt.namespace === losslessRule.namespace)
        .map((service: ServiceView) => ({ label: service.name, value: service.name }));

    const editable = editor.editable;
    const submitPayload = React.useMemo(() => buildLosslessSubmitPayload(losslessRule), [losslessRule]);
    const losslessEnabled = losslessRule.lossless_online.delay_register.enable || losslessRule.lossless_online.warmup.enable || losslessRule.lossless_offline.enable;
    const metadataRecord = React.useMemo(() => metadataToRecord(losslessRule.metadata), [losslessRule.metadata]);
    const warmupCurvePoints = React.useMemo(() => buildWarmupCurvePoints(losslessRule.lossless_online.warmup.curvature), [losslessRule.lossless_online.warmup.curvature]);

    const updateDraft = (updater: (draft: LosslessRuleDraft) => LosslessRuleDraft) => {
        setLosslessRule(prev => updater(prev));
    };

    const updateDelay = (patch: Partial<LosslessRuleDraft['lossless_online']['delay_register']>) => {
        updateDraft(prev => ({
            ...prev,
            lossless_online: {
                ...prev.lossless_online,
                delay_register: {
                    ...prev.lossless_online.delay_register,
                    ...patch,
                },
            },
        }));
    };

    const updateWarmup = (patch: Partial<LosslessRuleDraft['lossless_online']['warmup']>) => {
        updateDraft(prev => ({
            ...prev,
            lossless_online: {
                ...prev.lossless_online,
                warmup: {
                    ...prev.lossless_online.warmup,
                    ...patch,
                },
            },
        }));
    };

    const updateOffline = (patch: Partial<LosslessRuleDraft['lossless_offline']>) => {
        updateDraft(prev => ({
            ...prev,
            lossless_offline: {
                ...prev.lossless_offline,
                ...patch,
            },
        }));
    };

    const onSubmit: FormProps['onSubmit'] = async () => {
        if (editor.saving) return;
        const errors = validateLosslessDraft(losslessRule);
        if (errors.length > 0) {
            openErrNotification('保存校验失败', errors[0].message);
            return;
        }
        const saveAction = op === 'create' ? saveLossLessRule : updateLosslessRule;
        setEditor(prev => ({ ...prev, saving: true }));
        try {
            const res = await dispatch(saveAction({ param: submitPayload as any }));
            if (res.meta.requestStatus === 'fulfilled') {
                openInfoNotification('请求成功', '保存无损规则成功');
                persistedRuleRef.current = losslessRule;
                if (op === 'create') {
                    refresh(true);
                } else {
                    setEditor(prev => ({ ...prev, editable: false }));
                    refresh(false);
                }
            } else {
                openErrNotification('请求失败', `保存无损规则失败: ${res.payload as string || '未知'}`);
            }
        } finally {
            setEditor(prev => ({ ...prev, saving: false }));
        }
    };

    const renderStatusTag = (enabled?: boolean) => (
        <Tag theme={enabled ? 'success' : 'default'} variant="light">
            {enabled ? '开启' : '关闭'}
        </Tag>
    );

    const renderReadonly = (value?: React.ReactNode) => (
        <div className={shared.fieldValue}>{value || '-'}</div>
    );

    const renderSeconds = (value?: number) => renderReadonly(value === undefined || value === null ? '-' : `${value} Second`);

    const renderUnitNumber = (
        label: string,
        value: number | undefined,
        onChange: (value: number | undefined) => void,
        unit: string,
        min?: number,
        max?: number,
        placeholder?: string,
    ) => (
        <InputAdornment append={unit} className={styles.unitNumber}>
            <InputNumber
                aria-label={label}
                theme="normal"
                min={min}
                max={max}
                placeholder={placeholder}
                value={value}
                onChange={(next) => onChange(next === undefined || next === null ? undefined : next as number)}
            />
        </InputAdornment>
    );

    const renderWarmupCurveChart = () => {
        const { width, height, left, right, top, bottom } = curveChartBox;
        const innerWidth = width - left - right;
        const innerHeight = height - top - bottom;
        const xOf = (progress: number) => left + (progress / 100) * innerWidth;
        const yOf = (weight: number) => top + (1 - weight / 100) * innerHeight;
        const linePath = warmupCurvePoints.map((point, index) => `${index === 0 ? 'M' : 'L'} ${xOf(point.progress).toFixed(2)} ${yOf(point.weight).toFixed(2)}`).join(' ');
        const areaPath = `${linePath} L ${xOf(100)} ${yOf(0)} L ${xOf(0)} ${yOf(0)} Z`;
        const ticks = [0, 25, 50, 75, 100];
        const markerPoints = warmupCurvePoints.filter(point => ticks.includes(point.progress));

        return (
            <svg className={styles.curveSvg} viewBox={`0 0 ${width} ${height}`} role="img" aria-label="预热曲线图">
                {ticks.map(tick => (
                    <React.Fragment key={`grid-${tick}`}>
                        <line className={styles.curveGrid} x1={xOf(tick)} y1={top} x2={xOf(tick)} y2={yOf(0)} />
                        <line className={styles.curveGrid} x1={left} y1={yOf(tick)} x2={xOf(100)} y2={yOf(tick)} />
                        <text className={styles.curveTick} x={xOf(tick)} y={height - 12} textAnchor="middle">{tick}%</text>
                        <text className={styles.curveTick} x={left - 10} y={yOf(tick) + 4} textAnchor="end">{tick}%</text>
                    </React.Fragment>
                ))}
                <path className={styles.curveArea} d={areaPath} />
                <path className={styles.curveLine} d={linePath} />
                {markerPoints.map(point => (
                    <g key={`marker-${point.progress}`}>
                        <circle className={styles.curveDot} cx={xOf(point.progress)} cy={yOf(point.weight)} r="3.5" />
                        <text className={styles.curveMarkerLabel} x={xOf(point.progress)} y={yOf(point.weight) - 8} textAnchor="middle">{point.weight}%</text>
                    </g>
                ))}
                <text className={styles.curveAxisLabel} x={(left + xOf(100)) / 2} y={height - 2} textAnchor="middle">预热时间进度</text>
                <text className={styles.curveAxisLabel} x="12" y={(top + yOf(0)) / 2} textAnchor="middle" transform={`rotate(-90 12 ${(top + yOf(0)) / 2})`}>动态权重占比</text>
            </svg>
        );
    };

    const renderBasicInfo = (
        <CollapsibleSection
            collapsed={basicInfoCollapsed}
            onCollapsedChange={setBasicInfoCollapsed}
            header="基础信息"
            summary={`${losslessRule.namespace || '-'}/${losslessRule.service || '-'} · ${losslessEnabled ? '启用' : '停用'} · 优先级 5`}
        >
                <div className={shared.infoGrid}>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>规则标识</div>
                        <div className={`${shared.fieldValue} ${shared.mono}`}>{losslessRule.id || `${losslessRule.namespace || '-'}/${losslessRule.service || '-'}`}</div>
                    </div>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>优先级</div>
                        {renderReadonly('5')}
                    </div>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>启用状态</div>
                        {renderStatusTag(losslessEnabled)}
                    </div>
                    <div className={`${shared.field} ${shared.full}`}>
                        <div className={shared.fieldLabel}>规则标签</div>
                        <RuleLabelField
                            metadata={metadataRecord}
                            editable={editable}
                            onChange={(next) => updateDraft(prev => ({
                                ...prev,
                                metadata: Object.entries(next).map(([key, value]) => ({ key, value })),
                            }))}
                        />
                    </div>
                </div>
        </CollapsibleSection>
    );

    const renderScope = (
        <section className={shared.section}>
            <div className={shared.sectionHeader}>
                <span>治理对象</span>
                <span className={`${shared.fieldValue} ${shared.mono}`}>{losslessRule.namespace || '-'}/{losslessRule.service || '-'}</span>
            </div>
            <div className={shared.sectionBody}>
                <div className={styles.sectionSubtle}>无损上线 / 下线作用的服务实例集合</div>
                <div className={shared.infoGrid}>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>命名空间</div>
                        {editable && !(op === 'create' && serviceContext) ? (
                            <Select
                                aria-label="无损治理命名空间"
                                filterable
                                creatable
                                value={losslessRule.namespace}
                                options={namespaceSelectOptions}
                                onChange={(value) => updateDraft(prev => ({ ...prev, namespace: value as string }))}
                            />
                        ) : renderReadonly(losslessRule.namespace)}
                    </div>
                    <div className={shared.field}>
                        <div className={shared.fieldLabel}>服务名称</div>
                        {editable && !(op === 'create' && serviceContext) ? (
                            <Select
                                aria-label="无损治理服务"
                                filterable
                                creatable
                                value={losslessRule.service}
                                options={serviceSelectOptions}
                                onChange={(value) => updateDraft(prev => ({ ...prev, service: value as string }))}
                            />
                        ) : renderReadonly(losslessRule.service)}
                    </div>
                </div>
            </div>
        </section>
    );

    const renderStrategyCards = (
        <div className={styles.strategyCards}>
            {delayStrategyOptions.map(option => {
                const active = losslessRule.lossless_online.delay_register.strategy === option.value;
                return (
                    <Button
                        variant="text"
                        key={option.value}
                        className={active ? styles.strategyCardActive : styles.strategyCard}
                        disabled={!editable}
                        type="button"
                        onClick={() => updateDelay({ strategy: option.value as any })}
                    >
                        <span>{option.label}</span>
                        <small>{option.value === 'DELAY_BY_TIME' ? '实例启动后等待固定秒数，再注册到服务发现。' : '健康检查成功后才暴露实例，避免未就绪接流。'}</small>
                    </Button>
                );
            })}
        </div>
    );

    const renderHTTPProbe = (
        <div className={shared.kv2}>
            <div>
                <div className={shared.editLabel}>请求方法</div>
                {editable ? (
                    <Select
                        aria-label="健康检查请求方法"
                        value={losslessRule.lossless_online.delay_register.health_check_method}
                        options={httpMethodOptions}
                        onChange={(value) => updateDelay({ health_check_method: value as string })}
                    />
                ) : renderReadonly(losslessRule.lossless_online.delay_register.health_check_method)}
            </div>
            <div>
                <div className={shared.editLabel}>检查路径</div>
                {editable ? (
                    <Input
                        aria-label="健康检查路径"
                        value={losslessRule.lossless_online.delay_register.health_check_path}
                        onChange={(value) => updateDelay({ health_check_path: value as string })}
                    />
                ) : renderReadonly(losslessRule.lossless_online.delay_register.health_check_path)}
            </div>
        </div>
    );

    const renderPayloadProbe = (
        <>
            <div className={styles.inlineNotice}>默认使用实例 {protocolText[losslessRule.lossless_online.delay_register.health_check_protocol]} 协议端口；仅当响应报文匹配成功后注册。</div>
            <div className={shared.kv2}>
                <div>
                    <div className={shared.editLabel}>匹配方式</div>
                    {editable ? (
                        <Select
                            aria-label="健康检查报文匹配方式"
                            value={losslessRule.lossless_online.delay_register.payload.match}
                            options={payloadMatchOptions}
                            onChange={(value) => updateDelay({
                                payload: {
                                    ...losslessRule.lossless_online.delay_register.payload,
                                    match: value as LosslessPayloadMatch,
                                },
                            })}
                        />
                    ) : renderReadonly(payloadMatchText[losslessRule.lossless_online.delay_register.payload.match])}
                </div>
                <div />
                <div>
                    <div className={shared.editLabel}>发送报文</div>
                    {editable ? (
                        <Textarea
                            aria-label="健康检查发送报文"
                            autosize={{ minRows: 3, maxRows: 6 }}
                            value={losslessRule.lossless_online.delay_register.payload.request}
                            onChange={(value) => updateDelay({
                                payload: {
                                    ...losslessRule.lossless_online.delay_register.payload,
                                    request: value as string,
                                },
                            })}
                        />
                    ) : renderReadonly(losslessRule.lossless_online.delay_register.payload.request || '-')}
                </div>
                <div>
                    <div className={shared.editLabel}>响应匹配</div>
                    {editable ? (
                        <Textarea
                            aria-label="健康检查响应匹配报文"
                            autosize={{ minRows: 3, maxRows: 6 }}
                            value={losslessRule.lossless_online.delay_register.payload.response}
                            onChange={(value) => updateDelay({
                                payload: {
                                    ...losslessRule.lossless_online.delay_register.payload,
                                    response: value as string,
                                },
                            })}
                        />
                    ) : renderReadonly(losslessRule.lossless_online.delay_register.payload.response || '-')}
                </div>
            </div>
        </>
    );

    const renderDelayRegister = (
        <section className={shared.section}>
            <div className={shared.sectionHeader}>
                <span>无损上线 · 延迟注册</span>
                {renderStatusTag(losslessRule.lossless_online.delay_register.enable)}
            </div>
            <div className={shared.sectionBody}>
                <div className={styles.switchRow}>
                    <div>
                        <div className={shared.fieldLabel}>上线保护启用</div>
                        <div className={styles.fieldHint}>{losslessRule.lossless_online.delay_register.enable ? '实例就绪前不会进入服务发现列表' : '已关闭，实例启动完成后立即注册到注册中心'}</div>
                    </div>
                    {editable ? (
                        <Switch
                            aria-label="上线保护启用"
                            value={losslessRule.lossless_online.delay_register.enable}
                            onChange={(checked) => updateDelay({ enable: checked as boolean })}
                        />
                    ) : renderStatusTag(losslessRule.lossless_online.delay_register.enable)}
                </div>
                {losslessRule.lossless_online.delay_register.enable && (
                    <div className={styles.lifecycleConfig}>
                        <div className={shared.step} data-step="1">
                            <div className={shared.stepTitle}>延迟注册策略</div>
                            <div className={shared.stepContent}>{renderStrategyCards}</div>
                        </div>
                        {losslessRule.lossless_online.delay_register.strategy === 'DELAY_BY_TIME' ? (
                            <div className={shared.step} data-step="2">
                                <div className={shared.stepTitle}>延迟注册时长</div>
                                <div className={shared.stepContent}>
                                    {editable
                                        ? renderUnitNumber('延迟注册时长', losslessRule.lossless_online.delay_register.interval, value => updateDelay({ interval: value || 0 }), 'Second', 0)
                                        : renderSeconds(losslessRule.lossless_online.delay_register.interval)}
                                </div>
                            </div>
                        ) : (
                            <>
                                <div className={shared.step} data-step="2">
                                    <div className={shared.stepTitle}>检查协议</div>
                                    <div className={shared.stepContent}>
                                        {editable ? (
                                            <RadioGroup
                                                aria-label="健康检查协议"
                                                theme="button"
                                                variant="primary-filled"
                                                options={protocolOptions}
                                                value={losslessRule.lossless_online.delay_register.health_check_protocol}
                                                onChange={(value) => updateDelay({ health_check_protocol: value as LosslessProbeProtocol })}
                                            />
                                        ) : renderReadonly(losslessRule.lossless_online.delay_register.health_check_protocol)}
                                    </div>
                                </div>
                                <div className={shared.step} data-step="3">
                                    <div className={shared.stepTitle}>探测目标</div>
                                    <div className={shared.stepContent}>
                                        {losslessRule.lossless_online.delay_register.health_check_protocol === 'HTTP' ? renderHTTPProbe : renderPayloadProbe}
                                    </div>
                                </div>
                                <div className={shared.step} data-step="4">
                                <div className={shared.stepTitle}>检查间隔</div>
                                <div className={shared.stepContent}>
                                        {editable
                                            ? renderUnitNumber('健康检查间隔', losslessRule.lossless_online.delay_register.health_check_interval, value => updateDelay({ health_check_interval: value || 0 }), 'Second', 1)
                                            : renderSeconds(losslessRule.lossless_online.delay_register.health_check_interval)}
                                    </div>
                                </div>
                            </>
                        )}
                        <div className={styles.summaryLine}>{describeDelaySummary(losslessRule)}</div>
                        {losslessRule.lossless_online.delay_register.strategy === 'DELAY_BY_HEALTH_CHECK' && (
                            <div className={styles.infoTiles}>
                                <div><strong>接口来源</strong><span>由业务实例自身暴露，不复用治理探测规则。</span></div>
                                <div><strong>端口语义</strong><span>默认使用实例协议端口，不要求单独填写。</span></div>
                                <div><strong>失败处理</strong><span>只延迟注册，不摘除已有健康实例。</span></div>
                            </div>
                        )}
                    </div>
                )}
            </div>
        </section>
    );

    const renderWarmup = (
        <section className={shared.section}>
            <div className={shared.sectionHeader}>
                <span>服务预热 · 渐进放量</span>
                {renderStatusTag(losslessRule.lossless_online.warmup.enable)}
            </div>
            <div className={shared.sectionBody}>
                <div className={styles.switchRow}>
                    <div>
                        <div className={shared.fieldLabel}>预热启用</div>
                        <div className={styles.fieldHint}>{losslessRule.lossless_online.warmup.enable ? '新实例按预热曲线逐步放量' : '已关闭，新实例注册后直接按完整权重接流'}</div>
                    </div>
                    {editable ? (
                        <Switch
                            aria-label="预热启用"
                            value={losslessRule.lossless_online.warmup.enable}
                            onChange={(checked) => updateWarmup({ enable: checked as boolean })}
                        />
                    ) : renderStatusTag(losslessRule.lossless_online.warmup.enable)}
                </div>
                {losslessRule.lossless_online.warmup.enable && (
                    <div className={styles.lifecycleConfig}>
                        <div className={shared.step} data-step="1">
                            <div className={shared.stepTitle}>预热窗口 <span className={shared.stepHint}>新实例不会立即按完整权重接流</span></div>
                            <div className={shared.stepContent}>
                                {editable
                                    ? renderUnitNumber('预热窗口', losslessRule.lossless_online.warmup.interval, value => updateWarmup({ interval: value || 0 }), 'Second', 1)
                                    : renderSeconds(losslessRule.lossless_online.warmup.interval)}
                                <div className={styles.fieldHint}>开启预热后，该时间窗内由治理层按预热曲线逐步放量。</div>
                            </div>
                        </div>
                        <div className={shared.step} data-step="2">
                            <div className={shared.stepTitle}>终止保护</div>
                            <div className={shared.stepContent}>
                                <div className={shared.kv2}>
                                    <div>
                                        <div className={shared.editLabel}>预热终止保护</div>
                                        {editable ? (
                                            <Switch
                                                aria-label="预热终止保护"
                                                value={losslessRule.lossless_online.warmup.enable_overload_protection}
                                                onChange={(checked) => updateWarmup({ enable_overload_protection: checked as boolean })}
                                            />
                                        ) : renderStatusTag(losslessRule.lossless_online.warmup.enable_overload_protection)}
                                    </div>
                                    {losslessRule.lossless_online.warmup.enable_overload_protection && (
                                        <div>
                                            <div className={shared.editLabel}>预热终止百分比</div>
                                            {editable
                                                ? renderUnitNumber('预热终止百分比', losslessRule.lossless_online.warmup.overload_protection_threshold, value => updateWarmup({ overload_protection_threshold: value || 0 }), '%', 0, 100)
                                                : renderReadonly(`${losslessRule.lossless_online.warmup.overload_protection_threshold}%`)}
                                        </div>
                                    )}
                                </div>
                            </div>
                        </div>
                        <div className={shared.step} data-step="3">
                            <div className={shared.stepTitle}>预热曲线值 <span className={shared.stepHint}>范围 1～5，值越大末段爬升越陡</span></div>
                            <div className={shared.stepContent}>
                                <div className={styles.curveLayout}>
                                    <div>
                                        {editable ? (
                                            <InputNumber
                                                aria-label="预热曲线值"
                                                theme="normal"
                                                min={1}
                                                max={5}
                                                placeholder="默认"
                                                value={losslessRule.lossless_online.warmup.curvature === '' ? undefined : losslessRule.lossless_online.warmup.curvature}
                                                onChange={(value) => updateWarmup({ curvature: value === undefined || value === null ? '' : value as number })}
                                            />
                                        ) : renderReadonly(losslessRule.lossless_online.warmup.curvature || '默认')}
                                        <div className={styles.fieldHint}>{describeWarmupCurve(losslessRule.lossless_online.warmup.curvature)}</div>
                                    </div>
                                    <div className={styles.curveChart}>
                                        {renderWarmupCurveChart()}
                                    </div>
                                </div>
                            </div>
                        </div>
                        <div className={styles.summaryLine}>{describeWarmupSummary(losslessRule)}</div>
                    </div>
                )}
            </div>
        </section>
    );

    const renderOffline = (
        <section className={shared.section}>
            <div className={shared.sectionHeader}>
                <span>无损下线 · 等待时长</span>
                {renderStatusTag(losslessRule.lossless_offline.enable)}
            </div>
            <div className={shared.sectionBody}>
                <div className={styles.switchRow}>
                    <div>
                        <div className={shared.fieldLabel}>下线保护启用</div>
                        <div className={styles.fieldHint}>{losslessRule.lossless_offline.enable ? '实例注销前等待存量请求完成' : '已关闭，实例注销时不等待存量请求完成'}</div>
                    </div>
                    {editable ? (
                        <Switch
                            aria-label="下线保护启用"
                            value={losslessRule.lossless_offline.enable}
                            onChange={(checked) => updateOffline({ enable: checked as boolean })}
                        />
                    ) : renderStatusTag(losslessRule.lossless_offline.enable)}
                </div>
                {losslessRule.lossless_offline.enable && (
                    <div className={styles.lifecycleConfig}>
                        <div className={shared.step} data-step="1">
                            <div className={shared.stepTitle}>下线等待</div>
                            <div className={shared.stepContent}>
                                {editable
                                    ? renderUnitNumber('下线等待', losslessRule.lossless_offline.interval, value => updateOffline({ interval: value || 0 }), 'Second', 1)
                                    : renderSeconds(losslessRule.lossless_offline.interval)}
                            </div>
                        </div>
                        <div className={styles.summaryLine}>{describeOfflineSummary(losslessRule)}</div>
                        <div className={styles.readonlyBehavior}>
                            <div>默认执行行为：实例进入下线流程后，系统默认先从流量入口摘除并停止接收新请求，然后按上方间隔等待存量连接 / 请求完成。</div>
                            <Space size={8}>
                                <Tag variant="light">默认摘流</Tag>
                                <Tag variant="light">停止新流量</Tag>
                                <Tag variant="light">等待存量请求</Tag>
                            </Space>
                        </div>
                    </div>
                )}
            </div>
        </section>
    );

    const renderStickyTool = (
        <StickyTool style={{ zIndex: 1000 }} placement="right-bottom" offset={[-10, 200]}>
            <StickyItem
                label=""
                icon={!editable ? (
                    <RuleStickyAction label="编辑" icon={<Edit1Icon />} onClick={() => setEditor(prev => ({ ...prev, editable: true }))} />
                ) : (
                    <RuleStickyAction label="保存" icon={<SaveIcon />} loading={editor.saving} disabled={editor.saving} onClick={() => form.submit()} />
                )}
            />
            {editable && (
                <StickyItem
                    label=""
                    icon={<RuleStickyAction label="撤销" icon={<RollbackIcon />} onClick={() => {
                        if (op === 'create') {
                            refresh(true);
                            return;
                        }
                        setLosslessRule(persistedRuleRef.current);
                        setEditor(prev => ({ ...prev, editable: false }));
                    }} />}
                />
            )}
            {!editable && (
                <StickyItem
                    label=""
                    icon={<RuleStickyAction label="发布" icon={<RocketIcon />} onClick={() => setEditor(prev => ({ ...prev, publishView: true }))} />}
                />
            )}
        </StickyTool>
    );

    return (
        <div className={styles.editorBody}>
            <Form form={form} onSubmit={onSubmit} layout="vertical" labelAlign="left">
                <div className={styles.losslessEditorShell}>
                    <div className={styles.formPane}>
                        {renderBasicInfo}
                        {renderScope}
                        {renderDelayRegister}
                        {renderWarmup}
                        {renderOffline}
                    </div>
                </div>
                {editor.publishView && (
                    <PublishForm
                        ruleId={losslessRule.id || ''}
                        ruleName={losslessRule.id || `${losslessRule.namespace}/${losslessRule.service}`}
                        resource={PolicySourceType.LossLessRules}
                        visible={editor.publishView}
                        close={() => setEditor(prev => ({ ...prev, publishView: false }))}
                    />
                )}
                {renderStickyTool}
            </Form>
        </div>
    );
};

export default React.memo(LossLessEditor);
