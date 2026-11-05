import React from 'react';
import { useNavigate } from 'components/Router';
import { Button, Empty, Loading, MessagePlugin } from 'components/Fluent';
import { CopyIcon, LinkIcon, ViewListIcon } from 'components/Fluent/icons';

import { ConfirmOperationButton } from 'components/OperationButton';
import { useAppDispatch, useAppSelector } from 'modules/store';
import { listOneService, selectService } from 'modules/discovery/service';
import { describeServiceAlias } from 'services/alias';
import { deleteServices, type ServiceView } from 'services/service';
import {
    describeLogicalServiceEnvironments,
    resolveServiceEnvironmentBinding,
    ServiceEnvironmentBindingView,
} from 'services/logical_service';
import EnvironmentResourceSwitcher from 'components/EnvironmentResourceSwitcher';
import { copyToClipboard } from 'utils/sys';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import ServiceForm, { ServiceFormHandle } from '../ServiceForm';
import style from './index.module.less';

interface IServiceDetailProps {
    namespace: string;
    serviceName: string;
    onTabChange: (tab: '1' | '2') => void;
    initialEdit?: boolean;
    logicalServiceId?: string;
}

const emptyText = '—';

const isEmpty = (value?: string | null) => value === undefined || value === null || value === '';

const parseCount = (value?: string) => {
    const parsed = Number.parseInt(value || '0', 10);
    return Number.isNaN(parsed) ? 0 : parsed;
};

const ServiceDetail: React.FC<IServiceDetailProps> = ({
    namespace,
    serviceName,
    onTabChange,
    initialEdit = false,
    logicalServiceId = '',
}) => {
    const dispatch = useAppDispatch();
    const navigate = useNavigate();
    const { editSvc, viewSvc } = useAppSelector(selectService);
    const [loading, setLoading] = React.useState(false);
    const [aliasCount, setAliasCount] = React.useState<number | null>(null);
    const [editing, setEditing] = React.useState(initialEdit);
    const previousEditing = React.useRef(editing);
    const editButtonRef = React.useRef<HTMLButtonElement>(null);
    React.useEffect(() => {
        if (previousEditing.current && !editing) editButtonRef.current?.focus();
        previousEditing.current = editing;
    }, [editing]);
    const [serviceSubmitting, setServiceSubmitting] = React.useState(false);
    const serviceFormRef = React.useRef<ServiceFormHandle>(null);
    const [environmentServices, setEnvironmentServices] = React.useState<ServiceEnvironmentBindingView[]>([]);
    const [resolvedLogicalId, setResolvedLogicalId] = React.useState(logicalServiceId);

    const reloadService = React.useCallback(() => {
        let active = true;

        if (namespace && serviceName) {
            const id = editSvc?.namespace === namespace && editSvc?.name === serviceName && editSvc?.id !== `${namespace}/${serviceName}`
                ? editSvc?.id
                : undefined;
            setLoading(true);
            setAliasCount(null);

            dispatch(listOneService({
                id,
                namespace,
                name: serviceName,
            })).then((res) => {
                if (res.meta.requestStatus === 'rejected') {
                    openErrNotification('请求错误', `获取服务详情失败: ${res.payload as string}`);
                }
            }).finally(() => {
                if (active) {
                    setLoading(false);
                }
            });

            describeServiceAlias({
                namespace,
                service: serviceName,
                offset: 0,
                limit: 1,
            }).then((res) => {
                if (active) {
                    setAliasCount(res.totalCount);
                }
            }).catch(() => {
                if (active) {
                    setAliasCount(null);
                }
            });
        }

        return () => {
            active = false;
        };
    }, [dispatch, editSvc?.id, editSvc?.name, editSvc?.namespace, namespace, serviceName]);

    React.useEffect(() => reloadService(), [reloadService]);

    React.useEffect(() => {
        setEditing(initialEdit);
    }, [initialEdit, namespace, serviceName]);

    const service = viewSvc?.namespace === namespace && viewSvc?.name === serviceName
        ? viewSvc as ServiceView
        : null;

    React.useEffect(() => {
        let active = true;
        const loadBindings = async () => {
            if (!service?.id) {
                setResolvedLogicalId(logicalServiceId);
                setEnvironmentServices([]);
                return;
            }
            // 从逻辑服务详情进入时，URL 已携带权威 logicalServiceId。
            // 服务查询的兼容响应可能暂时没有数据库 ID，不能再用 namespace/name
            // 组合值查询绑定并把已知逻辑服务上下文覆盖为空。
            let nextLogicalId = logicalServiceId;
            if (!nextLogicalId) {
                const binding = await resolveServiceEnvironmentBinding(service.id);
                nextLogicalId = 'logical_service_id' in binding ? binding.logical_service_id : '';
            }
            if (!active) return;
            setResolvedLogicalId(nextLogicalId);
            if (!nextLogicalId) {
                setEnvironmentServices([]);
                return;
            }
            const result = await describeLogicalServiceEnvironments(nextLogicalId);
            if (active) setEnvironmentServices(result.list);
        };
        loadBindings().catch(() => {
            if (active) {
                setResolvedLogicalId('');
                setEnvironmentServices([]);
            }
        });
        return () => { active = false; };
    }, [logicalServiceId, service?.id]);
    const metadata = Object.entries(service?.metadata || {});
    const totalInstances = parseCount(service?.total_instance_count);
    const healthyInstances = parseCount(service?.healthy_instance_count);
    const available = healthyInstances > 0;

    const switchTab = (tab: '1' | '2', label: string) => {
        onTabChange(tab);
        MessagePlugin.success(`已切换到${label}`);
    };

    const removeService = async () => {
        try {
            await deleteServices([{ id: service?.id, namespace, name: serviceName }]);
            openInfoNotification('删除成功', `环境服务 ${namespace}/${serviceName} 已删除`);
            if (resolvedLogicalId) {
                navigate(`/discovery/service/detail?id=${encodeURIComponent(resolvedLogicalId)}`);
            } else if (namespace === 'pole-system') {
                navigate(`/discovery/service?scope=system&namespace=${encodeURIComponent(namespace)}`);
            } else {
                navigate('/discovery/service');
            }
        } catch (error) {
            openErrNotification('删除环境服务失败', error instanceof Error ? error.message : String(error));
        }
    };

    if (loading && !service) {
        return (
            <div className={style.detailPanel}>
                <Loading text="加载服务详情中..." />
            </div>
        )
    }

    if (!service) {
        return (
            <div className={style.detailPanel}>
                <Empty title="暂无服务详情" />
            </div>
        )
    }

    return (
        <section className={style.detailPanel}>
            <EnvironmentResourceSwitcher
                currentNamespace={namespace}
                resourceLabel="服务"
                items={environmentServices.map((item) => ({
                    namespace: item.namespace,
                    summary: `${item.service_name} · ${parseCount(item.service?.healthy_instance_count)}/${parseCount(item.service?.total_instance_count)} 健康实例`,
                }))}
                onSelect={(nextNamespace) => {
                    const binding = environmentServices.find(item => item.namespace === nextNamespace);
                    if (!binding) return;
                    const params = new URLSearchParams(window.location.search);
                    params.set('namespace', nextNamespace);
                    params.set('service', binding.service_name);
                    if (resolvedLogicalId) params.set('logicalServiceId', resolvedLogicalId);
                    params.delete('mode');
                    navigate(`${window.location.pathname}?${params.toString()}`);
                }}
            />
            {!resolvedLogicalId && (
                <div className={style.unboundNotice}>
                    当前环境服务尚未关联逻辑服务，因此不参与跨环境聚合。
                </div>
            )}
            <section className={style.detailHeader}>
                <div className={style.detailIdentity}>
                    <div className={style.detailIcon} aria-hidden="true">svc</div>
                    <div className={style.detailTitleGroup}>
                        <h2 className={style.detailName}>{service.name}</h2>
                        <div className={style.detailDescription}>
                            {isEmpty(service.comment) ? '暂无服务描述' : service.comment}
                        </div>
                    </div>
                </div>
                <div className={style.detailActions}>
                    <Button
                        variant="outline"
                        icon={<CopyIcon />}
                        onClick={() => copyToClipboard(service.id, '已复制 服务 ID')}
                    >
                        复制 ID
                    </Button>
                    <Button variant="outline" icon={<ViewListIcon />} onClick={() => switchTab('1', '服务实例')}>
                        查看实例
                    </Button>
                    <Button variant="outline" icon={<LinkIcon />} onClick={() => switchTab('2', '服务别名')}>
                        管理别名
                    </Button>
                    <ConfirmOperationButton
                        action="delete"
                        label="删除环境服务"
                        shape="rectangle"
                        variant="outline"
                        theme="danger"
                        disabled={service.deleteable === false}
                        disabledLabel="无权限操作"
                        confirmContent={`确认删除环境服务 ${namespace}/${serviceName} 吗？`}
                        onConfirm={() => void removeService()}
                    />
                </div>
            </section>

            <section className={style.detailStats} aria-label="服务摘要">
                <div className={style.detailStat}>
                    <span className={style.detailStatLabel}>可用实例</span>
                    <strong className={style.detailStatValue}>{healthyInstances}</strong>
                    <span className={style.detailStatDescription}>
                        {available ? '发现结果可返回健康节点' : '发现结果暂无节点'}
                    </span>
                </div>
                <div className={style.detailStat}>
                    <span className={style.detailStatLabel}>注册实例</span>
                    <strong className={style.detailStatValue}>{totalInstances}</strong>
                    <span className={style.detailStatDescription}>
                        {totalInstances > 0 ? '已有客户端实例注册' : '等待客户端注册'}
                    </span>
                </div>
                <div className={style.detailStat}>
                    <span className={style.detailStatLabel}>服务别名</span>
                    <strong className={style.detailStatValue}>{aliasCount ?? emptyText}</strong>
                    <span className={style.detailStatDescription}>
                        {aliasCount === null ? '别名数量暂不可用' : aliasCount > 0 ? '可跨命名空间访问' : '尚未配置服务别名'}
                    </span>
                </div>
                <div className={style.detailStat}>
                    <span className={style.detailStatLabel}>服务标签</span>
                    <strong className={style.detailStatValue}>{metadata.length || emptyText}</strong>
                    <span className={style.detailStatDescription}>
                        {metadata.length > 0 ? '已设置服务标签' : '未设置服务标签'}
                    </span>
                </div>
            </section>

            <section className={style.detailSections}>
                <section className={`${style.detailSection} ${style.detailFormSection}`}>
                    <header className={style.detailSectionHeader}>
                        <div className={style.detailSectionHeading}>
                            <h3>{editing ? '编辑服务信息' : '服务信息'}</h3>
                            <span>{editing ? '在当前详情页修改服务描述、归属和标签' : '注册发现服务元数据'}</span>
                        </div>
                        {!editing ? (
                            <Button
                                ref={editButtonRef}
                                theme="primary"
                                disabled={service.editable === false}
                                onClick={() => setEditing(true)}
                            >
                                编辑服务
                            </Button>
                        ) : (
                            <div className={style.detailActions}>
                                <Button variant="outline" disabled={serviceSubmitting} onClick={() => setEditing(false)}>
                                    取消
                                </Button>
                                <Button theme="primary" loading={serviceSubmitting} onClick={() => serviceFormRef.current?.submit()}>
                                    保存修改
                                </Button>
                            </div>
                        )}
                    </header>
                    <ServiceForm
                        ref={serviceFormRef}
                        mode={editing ? 'edit' : 'view'}
                        service={service}
                        onSubmitted={() => {
                            setEditing(false);
                            reloadService();
                        }}
                        onCancel={() => setEditing(false)}
                        onSubmittingChange={setServiceSubmitting}
                    />
                </section>
            </section>
        </section>
    )
}

export default React.memo(ServiceDetail);
