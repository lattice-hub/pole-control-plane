import React, { useState } from 'react';
import {
    Form,
    Input,
    Button,
    Space,
    Radio,
    Switch,
    Row,
    Divider,
    FormProps,
    Steps,
    InputNumber,
    Table,
} from 'components/Fluent';
import EditorDrawer from 'components/EditorDrawer';

import { useAppDispatch, useAppSelector } from 'modules/store';
import Text from 'components/Text';
import MatchInput from 'components/MatchInput';
import { MatchLogic, Op } from 'services/types';
import { LaneMatchLogic, LaneRule } from 'services/lane';
import { saveLaneRules, selectLaneRule, updateLaneRules } from 'modules/governance/lane_rule';
import { selectLaneGroup } from 'modules/governance/lane_group';
import { RoutingSourceArgument } from 'services/router';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import { cloneDeep } from 'lodash';
import { on } from 'events';

const { FormItem } = Form;
const { StepItem } = Steps;

interface LaneRuleEditorProps {
    op: Op; // 操作类型，'create' 或 'edit'
    visible: boolean;
    onClose: () => void;
}

const LaneRuleEditor: React.FC<LaneRuleEditorProps> = ({ op, visible, onClose }) => {
    const [form] = Form.useForm();
    const dispatch = useAppDispatch();

    const laneRuleState = useAppSelector(selectLaneRule);
    const { viewRule, editRule } = laneRuleState;
    const laneGroupState = useAppSelector(selectLaneGroup);
    const { editGroup } = laneGroupState;

    const [editable, setEditable] = React.useState<boolean>(op !== 'view');

    React.useEffect(() => {
        if (viewRule) {
            form.setFieldsValue({
                ...cloneDeep(viewRule),
                matchMode: viewRule.trafficMatchRule.matchMode || MatchLogic.AND, // 默认松散匹配
                trafficMatchRule: viewRule.trafficMatchRule.arguments,
                fallbackType: viewRule.matchMode || LaneMatchLogic.PERMISSIVE,
            });
            console.log('viewRule', form.getFieldsValue(true));
        }
    }, [viewRule])

    // 表单提交
    const onSubmit: FormProps['onSubmit'] = async (e) => {
        if (e.validateResult !== true) {
            return; // 如果表单验证失败，直接返回
        }

        const trafficMatch = form.getFieldValue('trafficMatchRule') as RoutingSourceArgument[] || [];
        const data: LaneRule = {
            id: viewRule?.id || '',
            groupName: viewRule?.groupName || editGroup?.name || '',
            name: form.getFieldValue('name') as string || '',
            description: form.getFieldValue('description') as string || '',
            priority: form.getFieldValue('priority') as number || 0,
            enable: form.getFieldValue('enabled') as boolean || true,
            matchMode: form.getFieldValue('fallbackType') as LaneMatchLogic || LaneMatchLogic.PERMISSIVE,
            trafficMatchRule: {
                matchMode: form.getFieldValue('matchMode') as MatchLogic || MatchLogic.AND, // 默认松散匹配
                arguments: trafficMatch, // 默认无匹配参数
            },
            defaultLabelValue: form.getFieldValue('defaultLabelValue') as string || '',
            labelKey: 'lane', // 默认标签KEY为 'lane'
        }

        let res;
        if (op === 'create') {
            res = await dispatch(saveLaneRules({ param: data }));
        } else {
            res = await dispatch(updateLaneRules({ param: data }));
        }

        if (res.meta.requestStatus !== 'fulfilled') {
            openErrNotification('请求错误', res?.payload as string);
        } else {
            openInfoNotification('请求成功', op === 'view' ? '修改泳道规则成功' : '创建泳道规则成功');
            onClose(); // 关闭编辑器
        }
    };

    const rendierEditor = () => {
        return (
            <Form form={form} layout="vertical" onSubmit={onSubmit}>
                <Steps readonly={true} layout="vertical" theme="default">
                    <StepItem value={1} title="实例打标" status="process">
                        <div style={{ marginTop: 24 }}>
                            <div style={{ color: 'var(--app-text-secondary)', fontSize: 13 }}>
                                标签 key 为 "lane"，value 作为自定义的“泳道标签值”。
                            </div>
                        </div>
                    </StepItem>
                    <StepItem value={2} title="创建泳道" status="process">
                        <div style={{ marginTop: 24 }}>
                            <Space direction="vertical" style={{ width: '100%' }}>
                                <Row >
                                    <Space>
                                        <FormItem
                                            label="泳道名称"
                                            name="name"
                                            showErrorMessage={editable}
                                            rules={[
                                                { required: true, message: '请输入泳道名称' },
                                                { max: 63, message: '泳道名称不能超过63个字符' },
                                                { pattern: /^[_a-zA-Z0-9-_]+$/, message: '泳道名称只能包含中文、英文字母、数字、-、_' },
                                            ]}
                                            help="允许中文、英文字母、-、_，限制63个字符">
                                            {editable ? (
                                                <Input style={{ width: '100%' }} placeholder="请输入泳道名称" />
                                            ) : (
                                                <Text>{viewRule?.name}</Text>
                                            )}
                                        </FormItem>
                                        <FormItem
                                            label="描述"
                                            name="description"
                                            showErrorMessage={editable}
                                            rules={[
                                                { max: 255, message: '描述不能超过255个字符' },
                                            ]}
                                        >
                                            {editable ? (
                                                <Input placeholder="请输入描述" />
                                            ) : (
                                                <Text>{viewRule?.description}</Text>
                                            )}
                                        </FormItem>
                                    </Space>
                                </Row>
                                <Row >
                                    <FormItem label="所属组" name="groupName">
                                        <Text>{viewRule?.groupName || editGroup?.name}</Text>
                                    </FormItem>
                                </Row>
                                <Row >
                                    <FormItem label="优先级" name="priority" help={'数字越小，优先级越大'}>
                                        {editable ? (
                                            <InputNumber theme="normal" min={0} />
                                        ) : (
                                            <Text>{viewRule?.priority}</Text>
                                        )}
                                    </FormItem>
                                </Row>
                                <Row >
                                    <Space>
                                        <FormItem
                                            label="泳道标签"
                                            name="defaultLabelValue"
                                            showErrorMessage={editable}
                                            rules={[
                                                { required: true, message: '请输入泳道标签' },
                                                { max: 64, message: '泳道标签不能超过64个字符' },
                                                { pattern: /^[_a-zA-Z0-9-]+$/, message: '泳道标签只能包含中文、英文字母、数字、-、_' },
                                            ]}
                                            help={'标签key为"lane"，匹配value值的服务实例将自动添加至泳道。允许中文、英文字母、-、_，限制64个字符'}>
                                            {editable ? (
                                                <Input style={{ width: '100%' }} placeholder="请输入泳道标签" />
                                            ) : (
                                                <Text>{viewRule?.defaultLabelValue}</Text>
                                            )}
                                        </FormItem>
                                    </Space>
                                </Row>
                                <Row>
                                    <FormItem label=" ">
                                        <div>
                                            <Table
                                                columns={[
                                                    { colKey: 'name', title: '服务名' },
                                                    { colKey: 'namespace', title: '命名空间' },
                                                    { colKey: 'label', title: '泳道标签' },
                                                ]}
                                                data={[]}
                                                rowKey="id"
                                            />
                                        </div>
                                    </FormItem>
                                </Row>
                            </Space>
                        </div>
                    </StepItem>
                    <StepItem value={3} title="泳道路由规则" status="process">
                        <div style={{ marginTop: 24 }}>
                            {/* 路由规则条件编辑，复用MatchInput组件 */}
                            <Space direction='vertical'>
                                <MatchInput label="灰度规则" name="trafficMatchRule" form={form} editable={editable} />
                                <FormItem label="规则匹配关系" name="matchMode" initialData={MatchLogic.AND}>
                                    <Radio.Group
                                        readonly={!editable}
                                        options={[
                                            { label: '且（满足全部条件）', value: MatchLogic.AND },
                                            { label: '或（满足以上任意条件）', value: MatchLogic.OR },
                                        ]}
                                    />
                                </FormItem>
                            </Space>
                            <Divider />
                            <Space>
                                <FormItem
                                    label="规则匹配失败"
                                    name={'fallbackType'}
                                    showErrorMessage={editable}
                                    initialData={LaneMatchLogic.PERMISSIVE}
                                    help={<div style={{ color: 'var(--app-text-secondary)', fontSize: 12, marginTop: 4 }}>
                                        松散匹配策略：若无法满足匹配到对应泳道的节点，则流量路由至基础集群节点<br />
                                        严格匹配策略：若无法满足匹配到对应泳道的节点，则返回错误，流量留在指定泳道内
                                    </div>}>
                                    <Radio.Group
                                        readonly={!editable}
                                        theme="button"
                                        variant="primary-filled"
                                    >
                                        <Radio.Button value={LaneMatchLogic.PERMISSIVE}>松散匹配策略</Radio.Button>
                                        <Radio.Button value={LaneMatchLogic.STRICT}>严格匹配策略</Radio.Button>
                                    </Radio.Group>
                                </FormItem>
                            </Space>
                            <FormItem label="是否启用" name="enabled">
                                <Switch disabled={!editable} defaultValue={true} />
                            </FormItem>
                        </div>
                    </StepItem>
                </Steps>
                <Space style={{ marginTop: 24 }}>
                    {editable ? (
                        <Button theme="primary" onClick={() => {
                            form.submit()
                        }}>提交</Button>
                    ) : (
                        <Button theme="primary" onClick={() => {
                            setEditable(true);
                        }}>编辑</Button>
                    )}
                    <Button theme="default" onClick={() => {
                        setEditable(false);
                        form.reset();
                        onClose();
                    }}>取消</Button>
                </Space>
            </Form>
        )
    }

    return (
        <>
            <EditorDrawer
                visible={visible}
                onClose={onClose}
                title="添加泳道规则"
                description="定义流量进入泳道的匹配条件与优先级。"
                width="wide"
                footer={null}
            >
                {rendierEditor()}
            </EditorDrawer>
        </>
    );
};

export default React.memo(LaneRuleEditor);
