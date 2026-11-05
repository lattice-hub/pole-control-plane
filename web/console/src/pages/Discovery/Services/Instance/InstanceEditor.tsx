import React from 'react';
import { Form, Input, Space, Button, InputNumber, Switch, Select, Tag } from 'components/Fluent';
import { ServerIcon } from 'components/Fluent/icons';
import type { FormProps } from 'components/Fluent';
import EditorDrawer, { EditorDrawerActions } from 'components/EditorDrawer';

import { useAppDispatch, useAppSelector } from 'modules/store';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import LabelInput from 'components/LabelInput';
import { selectInstance, updateInstances, saveInstances } from 'modules/discovery/instance';
import { HEALTH_CHECK_STRUCT, HEALTH_STATUS_MAP, ISOLATE_STATUS_MAP } from 'services/instance';
import { get } from 'lodash';
import style from './index.module.less';

const { FormItem } = Form;

interface IInstanceEditorProps {
    op: string;
    namespace: string;
    service: string;
    closeDrawer: () => void;
    onEdit?: () => void;
    canEdit?: boolean;
    visible: boolean;
}

const HealthCheckType = {
    Heartbeat: 1,
    TCP: 2,
    HTTP: 3,
} as const;

const DefaultProbeInterval = 5;

const HealthCheckTypeOptions = [
    { label: '心跳上报', value: HealthCheckType.Heartbeat },
    { label: 'TCP 探测', value: HealthCheckType.TCP },
    { label: 'HTTP 探测', value: HealthCheckType.HTTP },
];

const emptyText = '-';

const isPresent = (value?: React.ReactNode) => value !== undefined && value !== null && value !== '';

const Value = ({ children }: { children?: React.ReactNode }) => (
    <div
        className={style.instanceDetailValue}
        title={isPresent(children) ? String(children) : emptyText}
    >
        {isPresent(children) ? children : emptyText}
    </div>
)

const DetailItem = ({ label, children }: { label: string, children?: React.ReactNode }) => (
    <div className={style.instanceDetailItem}>
        <div className={style.instanceDetailLabel}>{label}</div>
        <Value>{children}</Value>
    </div>
)

const getHealthCheckText = (type?: number) => {
    if (type === HealthCheckType.Heartbeat) return '心跳上报';
    if (type === HealthCheckType.TCP) return 'TCP 探测';
    if (type === HealthCheckType.HTTP) return 'HTTP 探测';
    return emptyText;
}

interface InstanceSummaryProps {
    address: string;
    namespace: string;
    service: string;
    protocol?: string;
    healthy?: boolean;
    isolate?: boolean;
    enableHealthCheck?: boolean;
}

const InstanceSummary: React.FC<InstanceSummaryProps> = ({
    address,
    namespace,
    service,
    protocol,
    healthy,
    isolate,
    enableHealthCheck,
}) => {
    const healthStatus = HEALTH_STATUS_MAP[String(healthy) as keyof typeof HEALTH_STATUS_MAP];
    const isolateStatus = ISOLATE_STATUS_MAP[String(isolate) as keyof typeof ISOLATE_STATUS_MAP];

    return (
        <div className={style.instanceDetailHeader}>
            <div className={style.instanceDetailIdentity}>
                <div className={style.instanceDetailIcon}>
                    <ServerIcon />
                </div>
                <div className={style.instanceDetailTitleGroup}>
                    <div className={style.instanceDetailAddress} title={address}>{address}</div>
                    <div className={style.instanceContextList}>
                        <span className={style.instanceContextItem} title={namespace}>{namespace}</span>
                        <span className={style.instanceContextItem} title={service}>{service}</span>
                        {protocol && <Tag theme="primary" variant="light">{protocol}</Tag>}
                    </div>
                </div>
            </div>
            <div className={style.instanceStatusStrip}>
                <div className={style.instanceStatusItem}>
                    <span>健康状态</span>
                    <Tag theme={healthStatus?.theme as "success" | "danger" | "default" | "primary" | "warning"} variant="light">
                        {healthStatus?.text || emptyText}
                    </Tag>
                </div>
                <div className={style.instanceStatusItem}>
                    <span>隔离状态</span>
                    <Tag theme={isolateStatus?.theme as "success" | "danger" | "default" | "primary" | "warning"} variant="light">
                        {isolateStatus?.text || emptyText}
                    </Tag>
                </div>
                <div className={style.instanceStatusItem}>
                    <span>健康检查</span>
                    <Tag theme={enableHealthCheck ? 'success' : 'default'} variant="light">
                        {enableHealthCheck ? '开启' : '关闭'}
                    </Tag>
                </div>
            </div>
        </div>
    )
}

const InstanceEditor: React.FC<IInstanceEditorProps> = ({
    visible,
    op,
    closeDrawer,
    onEdit,
    canEdit = true,
    namespace,
    service,
}) => {
    const [form] = Form.useForm();
    const dispatch = useAppDispatch();

    const insState = useAppSelector(selectInstance);
    const { editIns } = insState;
    const watchedHost = Form.useWatch('host', form) as string | undefined;
    const watchedPort = Form.useWatch('port', form) as number | undefined;
    const watchedProtocol = Form.useWatch('protocol', form) as string | undefined;
    const watchedHealthy = Form.useWatch('healthy', form) as boolean | undefined;
    const watchedIsolate = Form.useWatch('isolate', form) as boolean | undefined;
    const watchedEnableHealthCheck = Form.useWatch('enableHealthCheck', form) as boolean | undefined;
    const watchedHealthCheckType = Form.useWatch(['healthCheck', 'type'], form) as number | undefined;

    React.useEffect(() => {
        form.setFieldsValue({
            host: editIns?.host || '',
            port: editIns?.port || 0,
            protocol: editIns?.protocol || '',
            version: editIns?.version || '',
            weight: editIns?.weight ?? 100,
            healthy: editIns?.healthy || false,
            isolate: editIns?.isolate || false,
            enableHealthCheck: editIns?.enableHealthCheck || false,
            healthCheck: editIns?.healthCheck || {
                type: HealthCheckType.TCP,
                tcp: { interval: DefaultProbeInterval },
            },
            health_check_probe_interval: editIns?.healthCheck?.tcp?.interval
                || editIns?.healthCheck?.http?.interval
                || DefaultProbeInterval,
            health_check_http_path: editIns?.healthCheck?.http?.path || '/',
            // 健康检查类型默认为心跳上报
            instance_labels: editIns?.metadata ? Object.entries(editIns.metadata).map(([key, value]) => ({ key, value })) : [],
            location: editIns?.location || {},
        })
    }, [editIns])

    const onSubmit: FormProps['onSubmit'] = async (e) => {
        if (e.validateResult !== true) {
            return;
        }

        const labels = form.getFieldValue('instance_labels') as { key: string, value: string }[]

        let newData = {
            id: editIns?.id || '',
            namespace: editIns?.namespace || namespace,
            service: editIns?.service || service,
            host: form.getFieldValue('host') as string,
            port: form.getFieldValue('port') as number,
            protocol: form.getFieldValue('protocol') as string,
            version: form.getFieldValue('version') as string,
            weight: form.getFieldValue('weight') as number,
            healthy: form.getFieldValue('healthy') as boolean,
            isolate: form.getFieldValue('isolate') as boolean,
            location: {
                region: form.getFieldValue(['location', 'region']) as string,
                zone: form.getFieldValue(['location', 'zone']) as string,
                campus: form.getFieldValue(['location', 'campus']) as string,
            },
            healthCheck: {} as HEALTH_CHECK_STRUCT,
            enableHealthCheck: form.getFieldValue('enableHealthCheck') as boolean,
            metadata: labels.filter((label) => label.key.trim() !== '' && label.value.trim() !== '').reduce((acc: { [key: string]: string }, label: { key: string, value: string }) => {
                acc[label.key] = label.value;
                return acc;
            }, {}),
        }
        if (newData.enableHealthCheck) {
            const type = form.getFieldValue(['healthCheck', 'type']) as number;
            if (type === HealthCheckType.Heartbeat) {
                newData.healthCheck = {
                    type,
                    heartbeat: {
                        ttl: form.getFieldValue(['healthCheck', 'heartbeat', 'ttl']) as number,
                    },
                };
            } else if (type === HealthCheckType.HTTP) {
                newData.healthCheck = {
                    type,
                    http: {
                        interval: form.getFieldValue('health_check_probe_interval') as number,
                        path: form.getFieldValue('health_check_http_path') as string,
                    },
                };
            } else {
                newData.healthCheck = {
                    type: HealthCheckType.TCP,
                    tcp: {
                        interval: form.getFieldValue('health_check_probe_interval') as number,
                    },
                };
            }
        }

        let result;
        if (op === 'edit') {
            result = await dispatch(updateInstances({ param: { ...newData } }))
        } else {
            result = await dispatch(saveInstances({ param: { ...newData } }))
        }

        if (result.meta.requestStatus !== 'fulfilled') {
            openErrNotification('请求错误', result?.payload as string);
        } else {
            openInfoNotification('请求成功', op === 'edit' ? '修改服务实例成功' : '创建服务实例成功');
            closeDrawer();
        }
    };

    const metadata = Object.entries(editIns?.metadata || {});
    const location = editIns?.location || {};
    const address = editIns ? `${editIns.host || emptyText}:${isPresent(editIns.port) ? editIns.port : emptyText}` : emptyText;
    const formNamespace = editIns?.namespace || namespace;
    const formService = editIns?.service || service;
    const formHost = watchedHost || editIns?.host || '';
    const formPort = watchedPort ?? editIns?.port;
    const formAddress = formHost ? `${formHost}:${isPresent(formPort) && formPort !== 0 ? formPort : emptyText}` : '待填写实例';
    const formProtocol = watchedProtocol || editIns?.protocol || '';
    const formHealthy = watchedHealthy ?? editIns?.healthy;
    const formIsolate = watchedIsolate ?? editIns?.isolate;
    const formEnableHealthCheck = watchedEnableHealthCheck ?? editIns?.enableHealthCheck;
    const instanceView = (
        <section className={style.instanceDetailPanel}>
            <InstanceSummary
                address={address}
                namespace={editIns?.namespace || namespace}
                service={editIns?.service || service}
                protocol={editIns?.protocol}
                healthy={editIns?.healthy}
                isolate={editIns?.isolate}
                enableHealthCheck={editIns?.enableHealthCheck}
            />

            <div className={style.instanceDetailSections}>
                <section className={style.instanceDetailSection}>
                    <div className={style.instanceDetailSectionTitle}>运行信息</div>
                    <div className={style.instanceDetailGrid}>
                        <DetailItem label="命名空间">{editIns?.namespace}</DetailItem>
                        <DetailItem label="服务">{editIns?.service}</DetailItem>
                        <DetailItem label="主机">{editIns?.host}</DetailItem>
                        <DetailItem label="端口">{editIns?.port}</DetailItem>
                        <DetailItem label="协议">{editIns?.protocol}</DetailItem>
                        <DetailItem label="版本">{editIns?.version}</DetailItem>
                        <DetailItem label="权重">{editIns?.weight}</DetailItem>
                    </div>
                </section>

                <section className={style.instanceDetailSection}>
                    <div className={style.instanceDetailSectionTitle}>健康检查</div>
                    <div className={style.instanceDetailGrid}>
                        <DetailItem label="状态">{editIns?.enableHealthCheck ? '开启' : '关闭'}</DetailItem>
                        <DetailItem label="检查方式">{editIns?.enableHealthCheck ? getHealthCheckText(editIns?.healthCheck?.type) : emptyText}</DetailItem>
                        <DetailItem label={editIns?.healthCheck?.type === HealthCheckType.Heartbeat ? '心跳 TTL' : '探测间隔'}>
                            {editIns?.enableHealthCheck
                                ? `${get(editIns?.healthCheck, editIns?.healthCheck?.type === HealthCheckType.Heartbeat
                                    ? 'heartbeat.ttl'
                                    : editIns?.healthCheck?.type === HealthCheckType.HTTP ? 'http.interval' : 'tcp.interval', DefaultProbeInterval)} 秒`
                                : emptyText}
                        </DetailItem>
                        {editIns?.enableHealthCheck && editIns?.healthCheck?.type === HealthCheckType.HTTP && (
                            <DetailItem label="HTTP 路径">{get(editIns?.healthCheck, 'http.path', '/')}</DetailItem>
                        )}
                    </div>
                </section>

                <section className={style.instanceDetailSection}>
                    <div className={style.instanceDetailSectionTitle}>位置</div>
                    <Space breakLine>
                        <Tag>区域: {location.region || emptyText}</Tag>
                        <Tag>可用区: {location.zone || emptyText}</Tag>
                        <Tag>机房: {location.campus || emptyText}</Tag>
                    </Space>
                </section>

                <section className={style.instanceDetailSection}>
                    <div className={style.instanceDetailSectionTitle}>实例标签</div>
                    {metadata.length > 0 ? (
                        <Space breakLine>
                            {metadata.map(([key, value]) => (
                                <Tag key={key} theme="primary" variant="light">
                                    {key}: {value}
                                </Tag>
                            ))}
                        </Space>
                    ) : (
                        <Value>{emptyText}</Value>
                    )}
                </section>
            </div>
        </section>
    )

    const instanceForm = (
        <>
            <Form
                className={style.instanceForm}
                form={form}
                layout="vertical"
                labelWidth={120}
                labelAlign={'left'}
                onSubmit={onSubmit}
            >
                <section className={style.instanceDetailPanel}>
                    <InstanceSummary
                        address={formAddress}
                        namespace={formNamespace}
                        service={formService}
                        protocol={formProtocol}
                        healthy={formHealthy}
                        isolate={formIsolate}
                        enableHealthCheck={formEnableHealthCheck}
                    />
                    <div className={style.instanceDetailSections}>
                    <section className={style.instanceDetailSection}>
                        <div className={style.instanceDetailSectionTitle}>运行信息</div>
                        <div className={`${style.instanceFormGrid} ${style.instanceRuntimeGrid}`}>
                            <FormItem label={'主机'} name={'host'} rules={[
                                { required: true, message: '请输入服务IP' },
                                { pattern: /^(((\d{1,2}|1\d{2}|2[0-4]\d|25[0-5])\.){3}(\d{1,2}|1\d{2}|2[0-4]\d|25[0-5]))$|^([a-zA-Z0-9][-a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-a-zA-Z0-9]{0,62})+\.?)$/, message: '请输入正确的IP地址或域名' },
                                { max: 15, message: '长度不超过15个字符' }
                            ]}>
                                <Input readonly={op === 'edit'} />
                            </FormItem>
                            <FormItem label={'端口'} name={'port'} rules={[
                                { required: true, message: '请输入服务端口' },
                                { pattern: /^[0-9]{1,5}$/, message: '请输入正确的端口号' },
                            ]}>
                                <InputNumber
                                    readonly={op === 'edit'}
                                    min={1}
                                    max={65535}
                                    step={1}
                                />
                            </FormItem>
                            <FormItem label={'协议'} name={'protocol'} rules={[
                                { required: false, message: '请输入服务协议, TCP/UDP/HTTP/gRPC/DUBBO/...' },
                                { max: 128, message: '长度不超过128个字符' }
                            ]}>
                                <Select
                                    readonly={op === 'view'}
                                    creatable={true}
                                    filterable={true}
                                    options={[
                                        { label: 'TCP', value: 'TCP' },
                                        { label: 'UDP', value: 'UDP' },
                                        { label: 'HTTP', value: 'HTTP' },
                                        { label: 'gRPC', value: 'gRPC' },
                                        { label: 'DUBBO', value: 'DUBBO' },
                                    ]}
                                />
                            </FormItem>
                            <FormItem label={'版本'} name={'version'} rules={[
                                { required: false, message: '请输入服务版本' },
                                { max: 128, message: '长度不超过128个字符' }
                            ]}>
                                <Input readonly={op === 'view'} />
                            </FormItem>
                            <FormItem label={'权重'} name={'weight'} rules={[
                                { required: false, message: '请输入服务权重' },
                                { pattern: /^[0-9]{1,5}$/, message: '请输入正确的权重' },
                            ]}>
                                <InputNumber
                                    min={0}
                                    max={100}
                                    step={1}
                                />
                            </FormItem>
                        </div>
                    </section>

                    <section className={style.instanceDetailSection}>
                        <div className={style.instanceDetailSectionTitle}>健康检查</div>
                        <div className={style.instanceSwitchGrid}>
                            <FormItem label="隔离状态" name="isolate">
                                <Switch size="large" label={['开', '关']} />
                            </FormItem>
                            <FormItem label="健康状态" name="healthy">
                                <Switch size="large" label={['健康', '异常']} />
                            </FormItem>
                            <FormItem label="健康检查" name="enableHealthCheck">
                                <Switch size="large" label={['开', '关']} />
                            </FormItem>
                        </div>
                        <div className={style.instanceFormGrid}>
                            {watchedEnableHealthCheck && (
                                <FormItem label="健康检查类型" name={['healthCheck', 'type']}>
                                    <Select
                                        options={HealthCheckTypeOptions.map((option) => ({
                                            ...option,
                                            disabled: op === 'create' && option.value === HealthCheckType.Heartbeat,
                                        }))}
                                    />
                                </FormItem>
                            )}
                            {watchedEnableHealthCheck && watchedHealthCheckType === HealthCheckType.Heartbeat && (
                                <FormItem label="心跳上报 TTL" name={['healthCheck', 'heartbeat', 'ttl']}>
                                    <InputNumber min={1} max={60} step={1} suffix="秒" />
                                </FormItem>
                            )}
                            {watchedEnableHealthCheck && watchedHealthCheckType === HealthCheckType.HTTP && (
                                <FormItem
                                    label="HTTP 探测路径"
                                    name="health_check_http_path"
                                    rules={[{ required: true, message: '请输入 HTTP 探测路径' }]}
                                >
                                    <Input placeholder="例如：/health" />
                                </FormItem>
                            )}
                            {watchedEnableHealthCheck && (
                                watchedHealthCheckType === HealthCheckType.TCP
                                || watchedHealthCheckType === HealthCheckType.HTTP
                            ) && (
                                <FormItem label="探测间隔" name="health_check_probe_interval">
                                    <InputNumber min={1} max={60} step={1} suffix="秒" />
                                </FormItem>
                            )}
                        </div>
                    </section>

                    <section className={style.instanceDetailSection}>
                        <div className={style.instanceDetailSectionTitle}>位置</div>
                        <div className={style.instanceFormGrid}>
                            <FormItem name={['location', 'region']} label={'区域'}>
                                <Input placeholder='区域' />
                            </FormItem>
                            <FormItem name={['location', 'zone']} label={'可用区'}>
                                <Input placeholder='可用区' />
                            </FormItem>
                            <FormItem name={['location', 'campus']} label={'机房'}>
                                <Input placeholder='机房' />
                            </FormItem>
                        </div>
                    </section>

                    <section className={style.instanceDetailSection}>
                        <div className={style.instanceDetailSectionTitle}>实例标签</div>
                        <LabelInput form={form} label='实例标签' name='instance_labels' editable={true} />
                    </section>
                    </div>
                </section>
            </Form>
        </>
    )

    return (
        <div>
            <EditorDrawer
                className={style.instanceDrawer}
                bodyClassName={style.instanceDrawerBody}
                width="wide"
                title={op === 'view' ? '实例详情' : op === 'edit' ? "编辑实例" : "创建实例"}
                description={op === 'view' ? undefined : '登记可被服务发现和健康检查访问的网络端点。'}
                visible={visible}
                onClose={closeDrawer}
                footer={op === 'view' ? (canEdit && onEdit ? (
                    <div className={style.instanceDrawerActions}>
                        <Button theme="primary" onClick={onEdit}>
                            编辑实例
                        </Button>
                    </div>
                ) : false) : (
                    <EditorDrawerActions
                        onCancel={closeDrawer}
                        onSubmit={() => form.submit({ showErrorMessage: true })}
                        submitText={op === 'edit' ? '保存修改' : '创建实例'}
                    />
                )}
            >
                {op === 'view' ? instanceView : instanceForm}
            </EditorDrawer>
        </div>
    );
}

export default React.memo(InstanceEditor);
