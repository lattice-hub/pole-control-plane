import React from 'react';
import { Breadcrumb, Empty, Loading } from 'components/Fluent';
import { useNavigate, useSearchParams } from 'components/Router';

import { useAppDispatch } from 'modules/store';
import { resetCustomRoute } from 'modules/governance/route';
import { resetRateLimitRule } from 'modules/governance/ratelimit';
import { resetCircuitBreaker } from 'modules/governance/circuitbreaker';
import { resetFaultDetect } from 'modules/governance/faultdetect';
import { resetLosslessRule } from 'modules/governance/lossless';
import { resetLaneGroup } from 'modules/governance/lane_group';
import CustomRouteEditor from './Router/CustomRouteEditor';
import RateLimitEditor from './RateLimit/RateLimitEditor';
import CircuitBreakerEditor from './CircuitBreaker/CircuitBreakerEditor';
import FaultDetectEditor from './CircuitBreaker/FaultDetectEditor';
import LossLessEditor from './LossLess/LossLessEditor';
import LaneGroupEdtor from './Router/LaneGroupEdtor';
import TrafficGovernanceEditor from './Security/TrafficGovernanceEditor';
import RuleDetailFrame from './RuleRelease/RuleDetailFrame';
import { LimitType } from 'services/ratelimit';
import { TrafficGovernanceKind, TrafficGovernanceKindLabel } from 'services/traffic_governance';
import { GovernanceServiceContext, GovernanceServiceRole } from './shared/serviceContext';
import { RuleNamespaceProvider } from './shared/ruleNamespace';
import style from './RuleRelease/RuleDetailDrawer.module.less';
import guidanceStyle from './RuleCreatePage.module.less';

const { BreadcrumbItem } = Breadcrumb;

type CreateRuleKind = 'route' | 'ratelimit-local' | 'ratelimit-global' | 'circuitbreaker' | 'faultdetect' | 'lossless' | 'lane' | 'traffic-security' | 'traffic-mirror' | 'traffic-mock';

interface CreateTarget {
    label: string;
    trafficKind?: TrafficGovernanceKind;
    limitType?: LimitType;
}

const createTargets: Record<CreateRuleKind, CreateTarget> = {
    route: { label: '路由' },
    'ratelimit-local': { label: '限流', limitType: LimitType.LOCAL },
    'ratelimit-global': { label: '限流', limitType: LimitType.GLOBAL },
    circuitbreaker: { label: '熔断' },
    faultdetect: { label: '探测' },
    lossless: { label: '无损' },
    lane: { label: '泳道' },
    'traffic-security': { label: TrafficGovernanceKindLabel.security, trafficKind: 'security' },
    'traffic-mirror': { label: TrafficGovernanceKindLabel.mirror, trafficKind: 'mirror' },
    'traffic-mock': { label: TrafficGovernanceKindLabel.mock, trafficKind: 'mock' },
};

const isCreateRuleKind = (value?: string | null): value is CreateRuleKind => !!value && value in createTargets;

const taskGuidance: Record<CreateRuleKind, string> = {
    route: '先确定主调与被调服务，再设置匹配条件和路由目标；确认目标权重合计后保存。',
    'ratelimit-local': '先选择服务与接口，再选择限流模式并配置对应阈值；窗口配额与并发上限使用不同的参数。',
    'ratelimit-global': '先选择服务与接口，再选择限流模式并配置对应阈值；窗口配额与并发上限使用不同的参数。',
    circuitbreaker: '先选择熔断对象，再配置触发条件和恢复条件；降级响应仅在启用相应策略时使用。',
    faultdetect: '先选择被探测服务与协议，再填写实际可访问的探测端口、间隔和超时。',
    lossless: '先选择服务，再按上线保护、健康检查、预热和下线保护逐项配置；关闭的策略不参与执行。',
    lane: '先创建泳道组并选择入口与组内服务，再在组中配置泳道和版本。',
    'traffic-security': '先选择受保护服务，再设置认证方式和接口；需要放行的接口使用白名单。',
    'traffic-mirror': '先选择主调与被调服务，再选择镜像接口、匹配条件、目标和镜像比例。',
    'traffic-mock': '先选择服务与接口，再配置匹配条件和响应 Code、延迟及 Body。',
};

const RuleCreatePage: React.FC = () => {
    const dispatch = useAppDispatch();
    const navigate = useNavigate();
    const [searchParams] = useSearchParams();
    const kindParam = searchParams.get('kind');
    const kind = isCreateRuleKind(kindParam) ? kindParam : undefined;
    const ruleNamespace = searchParams.get('ruleNamespace') || '';
    const [ready, setReady] = React.useState(false);

    const serviceContext = React.useMemo<GovernanceServiceContext | undefined>(() => {
        const namespace = searchParams.get('namespace') || '';
        const service = searchParams.get('service') || '';
        const role = searchParams.get('role');
        if (!namespace || !service || (role !== 'caller' && role !== 'callee')) return undefined;
        return { namespace, service, role: role as GovernanceServiceRole };
    }, [searchParams]);

    React.useEffect(() => {
        setReady(false);
        if (!kind) return;
        if (kind === 'route') dispatch(resetCustomRoute());
        if (kind === 'ratelimit-local' || kind === 'ratelimit-global') dispatch(resetRateLimitRule());
        if (kind === 'circuitbreaker') dispatch(resetCircuitBreaker());
        if (kind === 'faultdetect') dispatch(resetFaultDetect());
        if (kind === 'lossless') dispatch(resetLosslessRule());
        if (kind === 'lane') dispatch(resetLaneGroup());
        setReady(true);
    }, [dispatch, kind]);

    const leaveCreatePage = React.useCallback((_close: boolean) => {
        navigate('/governance/workbench');
    }, [navigate]);

    const renderEditor = () => {
        if (!kind) return null;
        const target = createTargets[kind];
        if (kind === 'route') return <CustomRouteEditor op="create" editable serviceContext={serviceContext} refresh={leaveCreatePage} />;
        if (kind === 'ratelimit-local' || kind === 'ratelimit-global') return <RateLimitEditor limitType={target.limitType || LimitType.LOCAL} visible op="create" serviceContext={serviceContext} refresh={leaveCreatePage} />;
        if (kind === 'circuitbreaker') return <CircuitBreakerEditor op="create" serviceContext={serviceContext} refresh={leaveCreatePage} />;
        if (kind === 'faultdetect') return <FaultDetectEditor op="create" serviceContext={serviceContext} refresh={leaveCreatePage} />;
        if (kind === 'lossless') return <LossLessEditor visible op="create" serviceContext={serviceContext} refresh={leaveCreatePage} />;
        if (kind === 'lane') return <LaneGroupEdtor op="create" serviceContext={serviceContext} refresh={leaveCreatePage} />;
        return <TrafficGovernanceEditor kind={target.trafficKind!} op="create" visible serviceContext={serviceContext} refresh={leaveCreatePage} />;
    };

    if (!kind || !ruleNamespace) {
        return (
            <div className={style.standalonePage}>
                <Breadcrumb className={style.detailBreadcrumb} maxItemWidth="220px">
                    <BreadcrumbItem onClick={() => navigate('/governance/workbench')}>治理工作台</BreadcrumbItem>
                    <BreadcrumbItem>新建规则</BreadcrumbItem>
                </Breadcrumb>
                <div className={style.emptyPane}><Empty description={!kind ? '缺少或不支持的规则类型' : '缺少规则归属环境'} /></div>
            </div>
        );
    }

    const target = createTargets[kind];
    return (
        <div className={`${style.standalonePage} ${style.createPage}`}>
            <Breadcrumb className={style.detailBreadcrumb} maxItemWidth="220px">
                <BreadcrumbItem onClick={() => navigate('/governance/workbench')}>治理工作台</BreadcrumbItem>
                <BreadcrumbItem>新建{target.label}规则</BreadcrumbItem>
            </Breadcrumb>
            <RuleDetailFrame title={`新建${target.label}规则`} subtitle={`${target.label} · 归属环境 ${ruleNamespace}`}>
                <div className={style.createPane}>
                <section className={guidanceStyle.guidance} aria-label="规则配置说明">
                    <p>{taskGuidance[kind]}</p>
                    <details>
                        <summary>作用范围与可选参数说明</summary>
                        <ul>
                            <li>归属环境用于管理这条规则；表单中的服务范围决定规则作用于哪些调用。</li>
                            <li>优先级、标签和描述用于排序、检索和识别；接口、条件及策略参数决定规则如何匹配和执行。</li>
                            <li>先完成必要的服务与策略配置，再调整恢复、降级等可选项。保存前检查启用状态及目标范围。</li>
                            <li>保存后按规则类型确认发布与生效状态；未保存的输入不会生效。限流规则保存后还需完成发布。</li>
                        </ul>
                    </details>
                </section>
                {ready ? (
                    <RuleNamespaceProvider value={ruleNamespace}>
                        {renderEditor()}
                    </RuleNamespaceProvider>
                ) : <div className={style.loadingPane}><Loading text="正在初始化规则表单..." /></div>}
                </div>
            </RuleDetailFrame>
        </div>
    );
};

export default React.memo(RuleCreatePage);
