import React from 'react';
import { useNavigate, useSearchParams } from 'components/Router';
import { Button } from 'components/Fluent';

import { ResourceHeader } from 'components/ResourceLayout';
import ServicesTable from './services';
import SystemNamespaceServices from './SystemNamespaceServices';
import style from './index.module.less';

export default React.memo(() => {
    const [searchParams] = useSearchParams();
    const navigate = useNavigate();
    const systemNamespace = searchParams.get('scope') === 'system'
        ? searchParams.get('namespace') || 'pole-system'
        : '';
    const businessNamespace = systemNamespace ? '' : searchParams.get('namespace') || '';
    const initialTab = systemNamespace
        ? 'system'
        : searchParams.get('view') === 'unbound' ? 'unbound' : 'logical';

    return (
        <div className={style.page}>
            <ResourceHeader
                density="compact"
                placement="app-header"
                eyebrow={systemNamespace ? '注册发现 / 系统空间' : businessNamespace ? '注册发现 / 业务环境' : '注册发现 / 逻辑服务'}
                title={systemNamespace ? `${systemNamespace} 系统服务` : businessNamespace ? `${businessNamespace} 环境服务` : '服务'}
                description={systemNamespace
                    ? '显式维护当前控制面系统空间中的运行时服务；这些服务不参与业务逻辑服务聚合。'
                    : businessNamespace
                    ? '当前列表严格限定在所选业务环境；返回服务主页可继续按逻辑服务跨环境浏览。'
                    : '逻辑服务用于跨环境聚合；每个环境继续使用自己的 Namespace 与运行时服务名。'}
            />
            {!systemNamespace && <section className={style.onboarding} aria-label="首次接入服务">
                <div><strong>首次接入：选或创建环境 → 接入环境服务 → 注册实例 → 查看健康状态</strong><p>服务创建后仍需注册实例，待实例健康后即可完成接入。逻辑服务可在之后用于跨环境聚合。</p></div>
                {!businessNamespace && <Button variant="outline" onClick={() => navigate('/namespace')}>选择或创建环境</Button>}
            </section>}
            <section className={style.serviceWorkspace}>
                {businessNamespace
                    ? <SystemNamespaceServices namespace={businessNamespace} system={false} />
                    : <ServicesTable initialTab={initialTab} systemNamespace={systemNamespace || 'pole-system'} />}
            </section>
        </div>
    )
});
