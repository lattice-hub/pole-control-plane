import React from 'react';
import { Button, Drawer, Empty, Loading, Select, Tag } from 'components/Fluent';
import {
    DescribeGovernanceServiceContracts as describeGovernanceServiceContracts,
    DescribeGovernanceServiceContractVersions as describeGovernanceServiceContractVersions,
    GovernanceInterfaceDescription,
    GovernanceServiceContract,
} from 'services/service';

import style from './ServiceContractPanel.module.less';

interface ServiceContractPanelProps {
    namespace: string
    serviceName: string
}

type ContractView = 'interfaces' | 'raw'

const PROTOCOLS = [
    { key: 'http', label: 'HTTP / OpenAPI', hint: '路径、方法与请求响应模型' },
    { key: 'dubbo', label: 'Dubbo', hint: '接口、方法与类型定义' },
    { key: 'grpc', label: 'gRPC', hint: 'Service、Method 与流模式' },
    { key: 'thrift', label: 'Thrift', hint: 'Service、Function 与 IDL' },
] as const;

const normalizeProtocol = (protocol?: string) => {
    const value = String(protocol || '').trim().toLocaleLowerCase();
    if (value === 'openapi' || value.startsWith('http')) {
        return 'http';
    }
    return value;
}

const protocolLabel = (protocol?: string) => (
    PROTOCOLS.find((item) => item.key === normalizeProtocol(protocol))?.label || protocol || '未知协议'
)

const protocolOperationLabel = (protocol: string | undefined, operation: GovernanceInterfaceDescription) => {
    const normalized = normalizeProtocol(protocol);
    const path = operation.path || '未命名接口';
    const method = operation.method || '未声明方法';
    switch (normalized) {
        case 'http':
            return `${method.toLocaleUpperCase()} ${path}`;
        case 'dubbo':
            return `${path}#${operation.type && operation.type !== 'dubbo' ? operation.type : method}`;
        case 'grpc':
            return `${path} · ${method}`;
        case 'thrift':
            return `${path}.${method}`;
        default:
            return `${path} / ${method}`;
    }
}

const operationFieldLabels = (protocol?: string) => {
    switch (normalizeProtocol(protocol)) {
        case 'http':
            return { path: 'HTTP 路径', method: 'HTTP 方法' };
        case 'grpc':
            return { path: 'gRPC Service', method: 'RPC Method' };
        case 'dubbo':
            return { path: 'Dubbo 接口', method: 'RPC 方法' };
        case 'thrift':
            return { path: 'Thrift Service', method: 'Function' };
        default:
            return { path: '接口', method: '方法' };
    }
}

const contractIdentity = (contract: GovernanceServiceContract, index: number) => (
    contract.id
    || [contract.protocol, contract.name, contract.version].filter(Boolean).join('/')
    || `contract-${index}`
)

const sourceLabel = (source?: string) => {
    if (source === '2' || String(source).toLocaleLowerCase() === 'client') {
        return 'SDK 上报';
    }
    if (source === '1' || String(source).toLocaleLowerCase() === 'manual') {
        return '人工维护';
    }
    return source || '未标注来源';
}

const metadataValue = (metadata: Record<string, string> | undefined, ...keys: string[]) => {
    for (const key of keys) {
        const value = metadata?.[key];
        if (String(value || '').trim()) {
            return value;
        }
    }
    return '-';
}

const operationNativeMetadata = (operation?: GovernanceInterfaceDescription) => {
    if (!operation?.content) {
        return {};
    }
    try {
        const definition = JSON.parse(operation.content) as Record<string, unknown>;
        const raw = definition.params ?? definition.parameters;
        if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
            return {};
        }
        return Object.fromEntries(
            Object.entries(raw as Record<string, unknown>)
                .filter(([, value]) => ['string', 'number', 'boolean'].includes(typeof value))
                .map(([key, value]) => [key, String(value)]),
        );
    } catch {
        return {};
    }
}

const dubboMetadataFields = (
    contract?: GovernanceServiceContract,
    operation?: GovernanceInterfaceDescription,
) => {
    const metadata = contract?.metadata;
    const native = operationNativeMetadata(operation);
    const value = (...keys: string[]) => {
        const operationValue = metadataValue(native, ...keys);
        return operationValue !== '-' ? operationValue : metadataValue(metadata, ...keys);
    }
    const interfaceName = operation?.path || value('dubbo.interface', 'interface');
    const group = value('dubbo.group', 'group');
    const version = value('dubbo.version', 'version');
    const projectedServiceKey = [
        group === '-' ? '' : `${group}/`,
        interfaceName === '-' ? '' : interfaceName,
        version === '-' ? '' : `:${version}`,
    ].join('') || '-';
    return [
        { label: '应用名', value: value('dubbo.application', 'application', 'app') },
        { label: '接口名', value: interfaceName },
        { label: '服务分组', value: group },
        { label: '接口版本', value: version },
        { label: '服务端角色', value: value('dubbo.side', 'side') },
        { label: '元数据模式', value: value('dubbo.metadata-type', 'metadata-type') },
        { label: '应用修订', value: value('dubbo.metadata-revision', 'metadata-revision') },
        { label: '序列化', value: value('dubbo.serialization', 'serialization') },
        { label: 'Service Key', value: value('dubbo.service-key', 'service-key') === '-'
            ? projectedServiceKey : value('dubbo.service-key', 'service-key') },
        {
            label: '映射应用',
            value: value('dubbo.mapping-applications', 'mapping-applications', 'applications'),
        },
    ];
}

const loadAllContracts = async (namespace: string, serviceName: string) => {
    const all: GovernanceServiceContract[] = [];
    const limit = 100;
    for (let offset = 0; ; offset += limit) {
        const response = await describeGovernanceServiceContracts({
            namespace,
            service: serviceName,
            offset,
            limit,
        });
        const page = response.data ?? [];
        all.push(...page);
        if (page.length < limit || all.length >= (response.amount ?? all.length)) {
            return all;
        }
    }
}

const ServiceContractPanel: React.FC<ServiceContractPanelProps> = ({ namespace, serviceName }) => {
    const [contracts, setContracts] = React.useState<GovernanceServiceContract[]>([]);
    const [versions, setVersions] = React.useState<string[]>([]);
    const [selectedVersion, setSelectedVersion] = React.useState('');
    const [selectedContractId, setSelectedContractId] = React.useState('');
    const [selectedOperation, setSelectedOperation] = React.useState<GovernanceInterfaceDescription | null>(null);
    const [activeView, setActiveView] = React.useState<ContractView>('interfaces');
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState('');
    const [reloadRevision, setReloadRevision] = React.useState(0);

    React.useEffect(() => {
        let disposed = false;

        const load = async () => {
            if (!namespace || !serviceName) {
                setContracts([]);
                setVersions([]);
                setSelectedVersion('');
                setSelectedContractId('');
                setSelectedOperation(null);
                setLoading(false);
                return;
            }

            setLoading(true);
            setError('');
            try {
                const [versionResponse, nextContracts] = await Promise.all([
                    describeGovernanceServiceContractVersions({ namespace, service: serviceName }),
                    loadAllContracts(namespace, serviceName),
                ]);
                if (disposed) {
                    return;
                }

                const nextVersions = Array.from(new Set([
                    ...(versionResponse.data ?? []).map((item) => item.version || ''),
                    ...nextContracts.map((item) => item.version || ''),
                ].filter(Boolean)));
                const nextVersion = nextVersions[0] || '';
                const firstContractIndex = nextContracts.findIndex((item) => !nextVersion || item.version === nextVersion);

                setContracts(nextContracts);
                setVersions(nextVersions);
                setSelectedVersion(nextVersion);
                setSelectedContractId(firstContractIndex >= 0 ? contractIdentity(nextContracts[firstContractIndex], firstContractIndex) : '');
                setSelectedOperation(null);
                setActiveView('interfaces');
            } catch (reason) {
                if (!disposed) {
                    setContracts([]);
                    setVersions([]);
                    setSelectedVersion('');
                    setSelectedContractId('');
                    setSelectedOperation(null);
                    setError(String((reason as Error)?.message || reason || '未知错误'));
                }
            } finally {
                if (!disposed) {
                    setLoading(false);
                }
            }
        }

        load();
        return () => {
            disposed = true;
        }
    }, [namespace, reloadRevision, serviceName]);

    const visibleContracts = React.useMemo(
        () => contracts.filter((item) => !selectedVersion || item.version === selectedVersion),
        [contracts, selectedVersion],
    );

    const selectedContract = React.useMemo(
        () => {
            const exactIndex = contracts.findIndex((item, index) => contractIdentity(item, index) === selectedContractId);
            if (exactIndex >= 0) {
                return contracts[exactIndex];
            }
            return visibleContracts[0];
        },
        [contracts, selectedContractId, visibleContracts],
    );

    const protocolCounts = React.useMemo(() => contracts.reduce<Record<string, number>>((result, contract) => {
        const protocol = normalizeProtocol(contract.protocol);
        result[protocol] = (result[protocol] || 0) + 1;
        return result;
    }, {}), [contracts]);

    const selectVersion = (value: string) => {
        const nextVisible = contracts
            .map((contract, index) => ({ contract, id: contractIdentity(contract, index) }))
            .filter((item) => !value || item.contract.version === value);
        setSelectedVersion(value);
        setSelectedContractId(nextVisible[0]?.id || '');
        setSelectedOperation(null);
        setActiveView('interfaces');
    }

    if (loading) {
        return (
            <section className={style.state} aria-live="polite">
                <Loading text="加载服务契约中..." />
            </section>
        );
    }

    if (error) {
        return (
            <section className={style.state} role="alert">
                <strong>服务契约加载失败</strong>
                <span>{error}</span>
                <Button variant="outline" onClick={() => setReloadRevision((value) => value + 1)}>重新加载</Button>
            </section>
        );
    }

    if (contracts.length === 0) {
        return (
            <section className={style.state}>
                <Empty
                    title="暂无服务契约"
                    description="可通过 CI 上传或 SDK 上报 OpenAPI、Dubbo、gRPC、Thrift 契约。"
                />
            </section>
        );
    }

    const fields = operationFieldLabels(selectedContract?.protocol);
    const interfaces = selectedContract?.interfaces ?? [];

    return (
        <section className={style.panel} aria-label={`${namespace}/${serviceName} 服务契约`}>
            <header className={style.header}>
                <div>
                    <span className={style.eyebrow}>SERVICE CONTRACT</span>
                    <h2>服务契约</h2>
                    <p>统一查看 HTTP、Dubbo、gRPC 与 Thrift 的接口定义和原始描述。</p>
                </div>
                <div className={style.headerMeta}>
                    <span>契约</span>
                    <strong>{contracts.length}</strong>
                </div>
            </header>

            <div className={style.protocolGrid} aria-label="服务契约协议能力">
                {PROTOCOLS.map((protocol) => {
                    const count = protocolCounts[protocol.key] || 0;
                    return (
                        <div className={style.protocolCard} key={protocol.key}>
                            <div className={style.protocolCardHeader}>
                                <Tag variant={count ? 'outline' : 'light'}>{protocol.label}</Tag>
                                <strong>{count}</strong>
                            </div>
                            <span>{protocol.hint}</span>
                        </div>
                    )
                })}
            </div>

            <div className={style.selectorBar}>
                <label>
                    <span>契约版本</span>
                    <Select
                        label="契约版本"
                        value={selectedVersion}
                        options={versions.map((version) => ({ label: version, value: version }))}
                        onChange={(value: string) => selectVersion(value)}
                        placeholder="选择版本"
                    />
                </label>
                <label>
                    <span>契约定义</span>
                    <Select
                        label="契约定义"
                        value={selectedContract ? contracts
                            .map((contract, index) => ({ contract, id: contractIdentity(contract, index) }))
                            .find((item) => item.contract === selectedContract)?.id : ''}
                        options={visibleContracts.map((contract) => {
                            const originalIndex = contracts.indexOf(contract);
                            return {
                                label: `${protocolLabel(contract.protocol)} · ${contract.type || contract.name || '未命名契约'}`,
                                value: contractIdentity(contract, originalIndex),
                            }
                        })}
                        onChange={(value: string) => {
                            setSelectedContractId(value);
                            setSelectedOperation(null);
                            setActiveView('interfaces');
                        }}
                        placeholder="选择契约"
                    />
                </label>
                <dl className={style.contractMeta}>
                    <div>
                        <dt>协议</dt>
                        <dd>{protocolLabel(selectedContract?.protocol)}</dd>
                    </div>
                    <div>
                        <dt>修订</dt>
                        <dd title={selectedContract?.revision}>{selectedContract?.revision || '-'}</dd>
                    </div>
                    <div>
                        <dt>状态</dt>
                        <dd>{selectedContract?.status === 'Online' ? '在线' : '离线'}</dd>
                    </div>
                </dl>
            </div>

            <div className={style.viewTabs} role="tablist" aria-label="契约展示方式">
                <Button
                    role="tab"
                    aria-selected={activeView === 'interfaces'}
                    theme={activeView === 'interfaces' ? 'primary' : 'default'}
                    variant={activeView === 'interfaces' ? 'base' : 'text'}
                    onClick={() => setActiveView('interfaces')}
                >
                    接口清单
                </Button>
                <Button
                    role="tab"
                    aria-selected={activeView === 'raw'}
                    theme={activeView === 'raw' ? 'primary' : 'default'}
                    variant={activeView === 'raw' ? 'base' : 'text'}
                    onClick={() => setActiveView('raw')}
                >
                    原始契约
                </Button>
            </div>

            {activeView === 'interfaces' ? (
                <div className={style.interfaceList} role="tabpanel" aria-label="接口清单">
                    {interfaces.length === 0 ? (
                        <Empty title="暂无接口定义" description="当前契约没有可展示的归一化接口。" />
                    ) : interfaces.map((operation, index) => (
                        <Button
                            variant="text"
                            type="button"
                            className={style.interfaceRow}
                            key={operation.id || `${operation.path}-${operation.method}-${index}`}
                            aria-label={`查看接口详情：${protocolOperationLabel(selectedContract?.protocol, operation)}`}
                            onClick={() => setSelectedOperation(operation)}
                        >
                            <span className={style.operationIdentity}>
                                <strong>{protocolOperationLabel(selectedContract?.protocol, operation)}</strong>
                                <span>{operation.content || '暂无接口说明'}</span>
                            </span>
                            <span className={style.operationFields}>
                                <span className={style.operationField}>
                                    <span>{fields.path}</span>
                                    <strong>{operation.path || '-'}</strong>
                                </span>
                                <span className={style.operationField}>
                                    <span>{fields.method}</span>
                                    <strong>{operation.method || '-'}</strong>
                                </span>
                                <span className={style.operationField}>
                                    <span>来源</span>
                                    <strong>{sourceLabel(operation.source)}</strong>
                                </span>
                            </span>
                            <span className={style.detailAction}>查看详情</span>
                        </Button>
                    ))}
                </div>
            ) : (
                <div className={style.rawPanel} role="tabpanel" aria-label="原始契约">
                    <div className={style.rawHeader}>
                        <span>原始内容</span>
                        <Tag variant="outline">{protocolLabel(selectedContract?.protocol)}</Tag>
                    </div>
                    <pre className={style.rawContent}>{selectedContract?.content || '当前契约未保存原始内容。'}</pre>
                </div>
            )}
            <Drawer
                size="large"
                header="接口详情"
                footer={false}
                visible={Boolean(selectedOperation)}
                onClose={() => setSelectedOperation(null)}
            >
                {selectedOperation && (
                    <div className={style.detailPanel}>
                        <header className={style.detailHeader}>
                            <span>{protocolLabel(selectedContract?.protocol)}</span>
                            <h3>{protocolOperationLabel(selectedContract?.protocol, selectedOperation)}</h3>
                            <p>来自 {namespace}/{serviceName} · 契约版本 {selectedContract?.version || '-'}</p>
                        </header>

                        <section className={style.detailSection} aria-label="接口标识">
                            <h4>接口标识</h4>
                            <dl className={style.detailGrid}>
                                <div>
                                    <dt>{fields.path}</dt>
                                    <dd>{selectedOperation.path || '-'}</dd>
                                </div>
                                <div>
                                    <dt>{fields.method}</dt>
                                    <dd>{selectedOperation.method || '-'}</dd>
                                </div>
                                <div>
                                    <dt>方法签名</dt>
                                    <dd>{selectedOperation.type || '-'}</dd>
                                </div>
                                <div>
                                    <dt>来源</dt>
                                    <dd>{sourceLabel(selectedOperation.source)}</dd>
                                </div>
                                <div>
                                    <dt>接口修订</dt>
                                    <dd>{selectedOperation.revision || selectedContract?.revision || '-'}</dd>
                                </div>
                                <div>
                                    <dt>契约状态</dt>
                                    <dd>{selectedContract?.status === 'Online' ? '在线' : '离线'}</dd>
                                </div>
                            </dl>
                        </section>

                        {normalizeProtocol(selectedContract?.protocol) === 'dubbo' && (
                            <section className={style.detailSection} aria-label="Dubbo 元数据">
                                <div className={style.detailSectionHeading}>
                                    <h4>Dubbo 元数据</h4>
                                    <Tag variant="outline">Metadata Center 投影</Tag>
                                </div>
                                <p className={style.sectionHint}>
                                    保留 Dubbo 原生应用、接口映射与修订语义；当前区域是统一契约上的可查询投影。
                                </p>
                                <dl className={style.detailGrid}>
                                    {dubboMetadataFields(selectedContract, selectedOperation).map((field) => (
                                        <div key={field.label}>
                                            <dt>{field.label}</dt>
                                            <dd>{field.value}</dd>
                                        </div>
                                    ))}
                                </dl>
                            </section>
                        )}

                        <section className={style.detailSection} aria-label="接口定义">
                            <h4>接口定义</h4>
                            <pre className={style.detailContent}>
                                {selectedOperation.content || selectedOperation.type || '当前接口未保存独立定义。'}
                            </pre>
                        </section>

                        {Object.keys(selectedContract?.metadata ?? {}).length > 0 && (
                            <section className={style.detailSection} aria-label="全部契约元数据">
                                <h4>全部契约元数据</h4>
                                <dl className={style.metadataList}>
                                    {Object.entries(selectedContract?.metadata ?? {})
                                        .sort(([left], [right]) => left.localeCompare(right))
                                        .map(([key, value]) => (
                                            <div key={key}>
                                                <dt>{key}</dt>
                                                <dd>{value}</dd>
                                            </div>
                                        ))}
                                </dl>
                            </section>
                        )}
                    </div>
                )}
            </Drawer>
        </section>
    )
}

export default React.memo(ServiceContractPanel);
