import React from 'react';
import { AddIcon, CheckCircleFilledIcon, DeleteIcon, FilterIcon, RocketIcon, SendIcon, TagIcon, UsergroupIcon } from 'components/Fluent/icons';
import { Button, Form, Input, Radio, RadioGroup, RangeInput, Select, Space, TagInput } from 'components/Fluent';
import type { CustomValidator, FormProps } from 'components/Fluent';
import EditorDrawer from 'components/EditorDrawer';

import { openErrNotification, openInfoNotification } from 'utils/notifition';
import { useAppDispatch } from 'modules/store';
import { ClientLabelType, ClientLabelTypeOption, MatcheLabel, MatchType, MatchTypeOption, MatchValueType, MatchValueTypeOption, RuleRelease } from 'services/types';
import { PolicySourceType } from 'services/auth_policy';
import { releaseCustomRoutes } from 'modules/governance/route';
import { releaseRateLimitRule } from 'modules/governance/ratelimit';
import { releaseLaneGroups } from 'modules/governance/lane_group';
import { releaseLosslessRule } from 'modules/governance/lossless';
import { releaseCircuitBreaker } from 'modules/governance/circuitbreaker';
import { releaseFaultDetect } from 'modules/governance/faultdetect';
import { inferTrafficGovernanceKindByResource, publishTrafficGovernanceRule } from 'services/traffic_governance';

import style from './PublishForm.module.less';

const { FormItem, FormList } = Form;

export interface IPublishFormProps {
    ruleId: string;
    ruleName: string;
    resource: PolicySourceType;
    releaseResource?: string;
    visible: boolean;
    close: () => void;
}

const PublishForm: React.FC<IPublishFormProps> = (props) => {
    const [form] = Form.useForm();
    const [releaseType, setReleaseType] = React.useState<'normal' | 'gray'>('normal');
    const dispatch = useAppDispatch();

    React.useEffect(() => {
        if (props.visible) {
            form.reset();
            setReleaseType('normal');
            form.setFieldsValue({ releaseType: 'normal' });
        }
    }, [form, props.visible, props.ruleId, props.ruleName]);

    const selectReleaseType = (nextType: 'normal' | 'gray') => {
        setReleaseType(nextType);
        form.setFieldsValue({ releaseType: nextType });
    };

    const labelsValidator: CustomValidator = () => {
        const releaseType = form.getFieldValue('releaseType') as string;
        if (releaseType !== 'gray') {
            return { result: true, message: '' };
        }
        const labels = (form.getFieldValue('betaLabels') as MatcheLabel[] || []).filter(Boolean);
        if (labels.length === 0) {
            return {
                result: false,
                type: 'error',
                message: '灰度发布至少需要 1 个客户端标签条件',
            };
        }
        const keys = labels.map(label => label?.key).filter(Boolean);
        if (keys.length !== new Set(keys).size) {
            return {
                result: false,
                type: 'error',
                message: '客户端标签 key 不能重复',
            };
        }
        return { result: true, message: '' };
    };

    const defaultClientLabel = (): MatcheLabel => ({
        key: ClientLabelType.CLIENT_IP,
        value: {
            type: MatchType.EXACT,
            value_type: MatchValueType.TEXT,
            value: '',
        },
    });

    const onSubmit: FormProps['onSubmit'] = async (e) => {
        if (e.validateResult !== true) {
            return;
        }
        const releaseType = form.getFieldValue("releaseType") as string;
        // 提交发布
        const pubData: RuleRelease = {
            rule_id: props.ruleId,
            rule_name: props.ruleName,
            release_name: form.getFieldValue("name") as string,
            description: form.getFieldValue("comment") as string,
            release_type: releaseType === 'normal' ? 'normal' as const : 'gray' as const,
            client_label: releaseType === 'gray' ? (form.getFieldValue("betaLabels") as MatcheLabel[] || []) : [],
            resource: props.releaseResource || props.resource,
        }

        let ret;
        const trafficKind = inferTrafficGovernanceKindByResource(props.releaseResource || props.resource);
        switch (props.resource) {
            case PolicySourceType.RouteRules:
                ret = await dispatch(releaseCustomRoutes({ param: [pubData] }));
                break;
            case PolicySourceType.RateLimitRules:
                ret = await dispatch(releaseRateLimitRule({ param: [pubData] }));
                break;
            case PolicySourceType.LaneRules:
                ret = await dispatch(releaseLaneGroups({ param: [pubData] }));
                break;
            case PolicySourceType.CircuitBreakerRules:
                ret = await dispatch(releaseCircuitBreaker({ param: [pubData] }));
                break;
            case PolicySourceType.FaultDetectRules:
                ret = await dispatch(releaseFaultDetect({ param: [pubData] }));
                break;
            case PolicySourceType.LossLessRules:
                ret = await dispatch(releaseLosslessRule({ param: [pubData] }));
                break;
            case PolicySourceType.SecurityRules:
            case PolicySourceType.MirrorRules:
            case PolicySourceType.MockRules:
                if (trafficKind) {
                    try {
                        const payload = await publishTrafficGovernanceRule(trafficKind, [pubData]);
                        ret = { meta: { requestStatus: 'fulfilled' }, payload };
                    } catch (error) {
                        ret = { meta: { requestStatus: 'rejected' }, payload: (error as Error).message };
                    }
                }
                break;
            default:
                openErrNotification('获取规则发布版本记录失败', '不支持的规则类型');
        }

        if (ret?.meta.requestStatus === 'fulfilled') {
            openInfoNotification('请求成功', `发布${props.ruleName}规则成功`);
            props.close();
        } else {
            openErrNotification('请求失败', `发布${props.ruleName}规则失败: ${ret?.payload as string}`);
        }
    }

    const renderStrategyCard = (
        type: 'normal' | 'gray',
        title: string,
        description: string,
        icon: React.ReactNode,
    ) => {
        const active = releaseType === type;
        return (
            <Button
                variant="text"
                type="button"
                className={`${style.strategyCard} ${active ? style.strategyCardActive : ''}`}
                aria-pressed={active}
                onClick={() => selectReleaseType(type)}
            >
                <div className={style.strategyTopline}>
                    <span className={style.strategyIcon}>{icon}</span>
                    {active && <CheckCircleFilledIcon className={style.strategyChecked} />}
                </div>
                <div className={style.strategyDiagram}>
                    <div className={style.diagramSource}>
                        <UsergroupIcon />
                        <span>客户端</span>
                    </div>
                    <span className={style.diagramLine} />
                    {type === 'gray' && (
                        <>
                            <div className={style.diagramFilter}>
                                <TagIcon />
                                <span>标签命中</span>
                            </div>
                            <span className={style.diagramLine} />
                        </>
                    )}
                    <div className={style.diagramTarget}>
                        <RocketIcon />
                        <span>{type === 'gray' ? '灰度版本' : '当前版本'}</span>
                    </div>
                </div>
                <div className={style.strategyTitle}>{title}</div>
                <div className={style.strategyDescription}>{description}</div>
            </Button>
        );
    };

    const renderValueInput = (name: number, restField: Record<string, unknown>) => (
        <FormItem role="cell" shouldUpdate={(prev, next) => {
            const prevType = prev.betaLabels?.[name]?.value?.type;
            const nextType = next.betaLabels?.[name]?.value?.type;
            return prevType !== nextType;
        }}>
            {({ getFieldValue }) => {
                const matchType = (getFieldValue('betaLabels') as MatcheLabel[] || [])[name]?.value?.type;
                const itemProps = {
                    ...restField,
                    name: [name, 'value', 'value'],
                    rules: [
                        { required: true, message: '匹配值不能为空' },
                        { max: 4096, message: '长度不超过4096个字符' },
                    ],
                };
                if (matchType === MatchType.RANGE) {
                    return (
                        <FormItem {...itemProps}>
                            <RangeInput className={style.fullControl} />
                        </FormItem>
                    );
                }
                if (matchType === MatchType.IN || matchType === MatchType.NOT_IN) {
                    return (
                        <FormItem {...itemProps}>
                            <TagInput className={style.fullControl} />
                        </FormItem>
                    );
                }
                return (
                    <FormItem {...itemProps}>
                        <Input className={style.fullControl} placeholder="输入匹配值" />
                    </FormItem>
                );
            }}
        </FormItem>
    );

    const renderGrayLabels = (
        <FormList
            name={['betaLabels']}
            initialData={[defaultClientLabel()]}
            rules={[{ validator: labelsValidator }]}
        >
            {(fields, { add, remove }) => (
                <div className={style.grayEditor} role="table" aria-label="灰度发布客户端标签">
                    <div className={style.grayHeader} role="row">
                        <span role="columnheader">客户端标签</span>
                        <span role="columnheader">匹配类型</span>
                        <span role="columnheader">值类型</span>
                        <span role="columnheader">匹配值</span>
                        <span role="columnheader">操作</span>
                    </div>
                    <div className={style.grayRows} role="rowgroup">
                        {fields.length === 0 && <div className={style.grayEmpty}>暂无客户端标签，请添加后配置灰度范围</div>}
                        {fields.map(({ key, name, ...restField }) => (
                            <div className={style.grayRow} key={key} role="row">
                                <FormItem
                                    {...restField}
                                    role="cell"
                                    name={[name, 'key']}
                                    rules={[
                                        { required: true, message: '标签不能为空' },
                                        { max: 128, message: '长度不超过128个字符' },
                                        { validator: labelsValidator },
                                    ]}
                                >
                                    <Select options={ClientLabelTypeOption} filterable creatable className={style.fullControl} />
                                </FormItem>
                                <FormItem {...restField} role="cell" name={[name, 'value', 'type']}>
                                    <Select options={MatchTypeOption} className={style.fullControl} />
                                </FormItem>
                                <FormItem {...restField} role="cell" name={[name, 'value', 'value_type']}>
                                    <Select options={MatchValueTypeOption} className={style.fullControl} />
                                </FormItem>
                                {renderValueInput(name, restField)}
                                <div className={style.grayActions} role="cell">
                                    <Button
                                        aria-label={`删除第 ${name + 1} 个客户端标签`}
                                        shape="square"
                                        variant="text"
                                        icon={<DeleteIcon />}
                                        onClick={() => remove(name)}
                                    />
                                </div>
                            </div>
                        ))}
                    </div>
                    <Button
                        className={style.addCondition}
                        variant="text"
                        icon={<AddIcon />}
                        onClick={() => add(defaultClientLabel())}
                    >
                        添加客户端标签
                    </Button>
                </div>
            )}
        </FormList>
    );

    return (
        <>
            <EditorDrawer
                className={style.drawer}
                bodyClassName={style.drawerBody}
                title="规则发布"
                description={props.ruleName}
                width="wide"
                visible={props.visible}
                onClose={() => {
                    props.close();
                }}
                footer={
                    <div className={style.footerBar}>
                        <div className={style.footerMeta}>
                            <span>当前策略</span>
                            <strong>{releaseType === 'gray' ? '灰度发布' : '全量发布'}</strong>
                        </div>
                        <Space>
                            <Button
                                theme='default'
                                onClick={() => {
                                    props.close();
                                }}
                            >
                                取消
                            </Button>
                            <Button theme='primary' icon={<SendIcon />} onClick={() => form.submit()}>
                                发布
                            </Button>
                        </Space>
                    </div>
                }
            >
                <Form
                    className={style.form}
                    form={form}
                    layout="vertical"
                    onSubmit={onSubmit}
                >
                    <section className={style.section}>
                        <div className={style.sectionTitle}>规则身份</div>
                        <div className={style.identityGrid}>
                            <div className={style.identityItem}>
                                <span>规则 ID</span>
                                <strong>{props.ruleId}</strong>
                            </div>
                            <div className={style.identityItem}>
                                <span>规则名称</span>
                                <strong>{props.ruleName}</strong>
                            </div>
                        </div>
                    </section>

                    <section className={style.section}>
                        <div className={style.sectionTitle}>版本信息</div>
                        <div className={style.versionGrid}>
                            <FormItem
                                label="版本名称"
                                name="name"
                                rules={[
                                    { required: true, message: '版本名称不能为空' },
                                    { max: 64, message: '长度不超过64个字符' },
                                ]}
                            >
                                <Input placeholder="输入本次发布版本名称" />
                            </FormItem>
                            <FormItem
                                label="版本描述"
                                name="comment"
                                rules={[
                                    { max: 255, message: '长度不超过255个字符' }
                                ]}
                            >
                                <Input placeholder="补充发布说明" />
                            </FormItem>
                        </div>
                    </section>

                    <section className={style.section}>
                        <div className={style.sectionTitle}>发布策略</div>
                        <div className={style.sectionHint}>选择本次版本生效范围，灰度发布会继续要求配置客户端标签条件。</div>
                        <div className={style.strategyGrid}>
                            {renderStrategyCard(
                                'normal',
                                '全量发布',
                                '版本立即面向全部客户端生效，适合确认无风险后的正式发布。',
                                <RocketIcon />,
                            )}
                            {renderStrategyCard(
                                'gray',
                                '灰度发布',
                                '只对命中客户端标签条件的请求生效，适合小范围验证。',
                                <FilterIcon />,
                            )}
                        </div>
                        <FormItem name='releaseType' initialData={'normal'} className={style.releaseTypeItem}>
                            <RadioGroup
                                className={style.hiddenReleaseType}
                                value={releaseType}
                                onChange={(value) => selectReleaseType(value as 'normal' | 'gray')}
                            >
                                <Radio value="normal">全量发布</Radio>
                                <Radio value="gray">灰度发布</Radio>
                            </RadioGroup>
                        </FormItem>
                    </section>

                    <FormItem shouldUpdate={(prev, next) => prev.releaseType !== next.releaseType}>
                        {({ getFieldValue }) => {
                            if (getFieldValue('releaseType') !== 'gray') {
                                return null;
                            }
                            return (
                                <section className={style.section}>
                                    <div className={style.sectionHeader}>
                                        <div>
                                            <div className={style.sectionTitle}>灰度条件</div>
                                            <div className={style.sectionHint}>客户端满足任一标签条件后会命中本次灰度发布。</div>
                                        </div>
                                    </div>
                                    {renderGrayLabels}
                                </section>
                            );
                        }}
                    </FormItem>
                </Form>
            </EditorDrawer>
        </>
    )
}

export default React.memo(PublishForm);
