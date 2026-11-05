import React from 'react';
import { Button, Loading, Tabs } from 'components/Fluent';
import { useNavigate, useSearchParams } from 'components/Router';

import { ResourceHeader } from 'components/ResourceLayout';
import InstanceTable from './InstanceTable';
import ServiceDetail from './ServiceDetail';
import ServiceContractPanel from './ServiceContractPanel';
import SubscribeTable from './SubscribeTable';
import ServiceAliasTable from '../alias';
import style from './index.module.less';

const { TabPanel } = Tabs;
const GovernanceWorkbench = React.lazy(() => import('pages/Governance/Workbench'));

export default React.memo(() => {
    const navigate = useNavigate()
    const [urlParams] = useSearchParams();
    const namespace = urlParams.get('namespace');
    const serviceName = urlParams.get('service');
    const editMode = urlParams.get('mode') === 'edit';
    const logicalServiceId = urlParams.get('logicalServiceId') || '';

    const requestedTab = urlParams.get('tab') === 'instances' ? '1' : '0';
    const [activeTab, setActiveTab] = React.useState(requestedTab);
    React.useEffect(() => { setActiveTab(requestedTab); }, [namespace, serviceName, requestedTab]);
    const returnToService = () => {
        if (logicalServiceId) {
            navigate(`/discovery/service/detail?id=${encodeURIComponent(logicalServiceId)}`);
        } else {
            navigate(-1);
        }
    };

    return (
        <div className={style.serviceDetailPage}>
            <ResourceHeader
                density="compact"
                placement="app-header"
                eyebrow="注册发现 / 环境服务"
                title={serviceName || '服务详情'}
                description={namespace ? `Namespace：${namespace}` : undefined}
            />
            <div className={style.serviceDetailActions}>
                <Button variant="outline" onClick={returnToService}>返回</Button>
            </div>
            <Tabs className={style.serviceDetailTabs} value={activeTab} onChange={(v: string) => setActiveTab(v)}>
                <TabPanel value={"0"} label="服务详情">
                    {activeTab === '0' && (
                        <ServiceDetail
                            namespace={namespace || ''}
                            serviceName={serviceName || ''}
                            onTabChange={setActiveTab}
                            initialEdit={editMode}
                            logicalServiceId={logicalServiceId}
                        />
                    )}
                </TabPanel>
                <TabPanel value={"1"} label="服务实例">
                    {activeTab === '1' && (
                        <InstanceTable
                            namespace={namespace || ''}
                            serviceName={serviceName || ''}
                        />
                    )}
                </TabPanel>
                <TabPanel value={"2"} label="服务别名">
                    {activeTab === '2' && (
                        <ServiceAliasTable
                            namespace={namespace || ''}
                            serviceName={serviceName || ''}
                            embedded
                        />
                    )}
                </TabPanel>
                <TabPanel value={"3"} label="服务订阅">
                    {activeTab === '3' && (
                        <SubscribeTable
                            namespace={namespace || ''}
                            serviceName={serviceName || ''}
                        />
                    )}
                </TabPanel>
                <TabPanel value={"4"} label="服务契约">
                    {activeTab === '4' && (
                        <ServiceContractPanel
                            namespace={namespace || ''}
                            serviceName={serviceName || ''}
                        />
                    )}
                </TabPanel>
                <TabPanel value={"5"} label="流量治理">
                    {activeTab === '5' && (
                        <React.Suspense fallback={<Loading text="加载流量治理中..." />}>
                            <section
                                className={style.serviceGovernance}
                                title={`${namespace || '-'}/${serviceName || '-'}`}
                            >
                                <GovernanceWorkbench
                                    embedded
                                    serviceContext={{ namespace: namespace || '', service: serviceName || '' }}
                                />
                            </section>
                        </React.Suspense>
                    )}
                </TabPanel>
            </Tabs>
        </div>
    )
})
