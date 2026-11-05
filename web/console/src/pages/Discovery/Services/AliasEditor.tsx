import React from 'react';
import { Form, Input, Select, TableRowData } from 'components/Fluent';
import type { FormProps } from 'components/Fluent';
import EditorDrawer, { EditorDrawerActions } from 'components/EditorDrawer';
import { useAppDispatch, useAppSelector } from 'modules/store';
import { NamespaceView } from 'services/namespace';
import { ServiceView } from 'services/service';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import { saveServiceAliass, selectServiceAlias, updateServiceAliass } from 'modules/discovery/alias';
import { listAllNamespaces, selectNamespace } from 'modules/namespace';
import { listAllServices, selectService } from 'modules/discovery/service';
import { Op } from 'services/types';
import { ServiceAliasView } from 'services/alias';
import style from './index.module.less';

const { FormItem } = Form;
const ALIAS_NAME_REG = /^[a-z][a-z0-9-]*$/;

interface IServiceEditorProps {
    op: Op
    closeDrawer: () => void;
    visible: boolean;
    data?: TableRowData;
    existingAliases?: ServiceAliasView[];
    targetService?: {
        namespace: string;
        serviceName: string;
    };
}

const ServiceAliasEditor: React.FC<IServiceEditorProps> = ({ visible, op, closeDrawer, data, existingAliases = [], targetService }) => {
    const [form] = Form.useForm();
    const dispatch = useAppDispatch();

    useAppSelector(selectServiceAlias);

    const namespaceState = useAppSelector(selectNamespace);
    const { datas: namespaceDatas } = namespaceState;

    const serviceState = useAppSelector(selectService);
    const { datas: serviceDatas } = serviceState;

    const getInitialValues = React.useCallback(() => ({
        name: targetService ? `${targetService.namespace}/${targetService.serviceName}` : data?.service && data?.namespace ? `${data.namespace}@${data.service}` : undefined,
        alias_namespace: data?.alias_namespace || targetService?.namespace,
        alias: data?.alias,
        comment: data?.comment,
    }), [data?.alias, data?.alias_namespace, data?.comment, data?.namespace, data?.service, targetService?.namespace, targetService?.serviceName]);

    const resetForm = React.useCallback(() => {
        form.setFieldsValue(getInitialValues());
    }, [form, getInitialValues]);

    React.useEffect(() => {
        if (visible) {
            resetForm();
            dispatch(listAllNamespaces())
                .then(res => {
                    if (res.meta.requestStatus === 'rejected') {
                        openErrNotification('获取命名空间列表失败', res?.payload as string);
                    }
                })
            if (!targetService) {
                dispatch(listAllServices())
                    .then(res => {
                        if (res.meta.requestStatus === 'rejected') {
                            openErrNotification('获取服务列表失败', res?.payload as string);
                        }
                    })
            }
        }
    }, [visible, resetForm, targetService?.namespace, targetService?.serviceName]);

    const onSubmit: FormProps['onSubmit'] = async (e) => {
        console.log('onSubmit', e);
        if (e.validateResult !== true) {
            return;
        }

        const selectedSvc = form.getFieldValue('name') as string;
        const selectedSvcNamespace = targetService?.namespace || selectedSvc.split('@')[0];
        const selectedSvcName = targetService?.serviceName || selectedSvc.split('@')[1];
        const aliasNamespace = String(form.getFieldValue('alias_namespace') || '').trim();
        const alias = String(form.getFieldValue('alias') || '').trim();
        const comment = String(form.getFieldValue('comment') || '').trim();

        if (!selectedSvcNamespace || !selectedSvcName) {
            openErrNotification('校验失败', '目标服务不能为空');
            return;
        }
        if (!aliasNamespace) {
            openErrNotification('校验失败', '别名所在命名空间不能为空');
            return;
        }
        if (!alias) {
            openErrNotification('校验失败', '别名不能为空');
            return;
        }
        if (!ALIAS_NAME_REG.test(alias)) {
            openErrNotification('校验失败', '别名需以小写字母开头，只能包含小写字母、数字和中划线');
            return;
        }
        const duplicated = op === 'create' && existingAliases.some(item => item.alias_namespace === aliasNamespace && item.alias === alias);
        if (duplicated) {
            openErrNotification('校验失败', '该命名空间下别名已存在');
            return;
        }

        const newData = {
            service: selectedSvcName,
            namespace: selectedSvcNamespace,
            alias_namespace: aliasNamespace,
            alias: alias,
            comment: comment || undefined,
        }
        let result;
        if (op === 'edit') {
            result = await dispatch(updateServiceAliass({ param: { ...newData } }))
        } else {
            result = await dispatch(saveServiceAliass({ param: { ...newData } }))
        }
        if (result.meta.requestStatus === 'fulfilled') {
            openInfoNotification('操作成功', op === 'edit' ? '修改服务别名成功' : '创建服务别名成功');
            closeDrawer();
        } else {
            openErrNotification('操作失败', result?.payload as string);
        }
    }

    const aliasForm = (
        <Form
            labelWidth={140}
            labelAlign={'left'}
            form={form}
            onSubmit={onSubmit}
        >
            <FormItem label="目标服务" name="name" rules={targetService ? [] : [{ required: true, message: '服务名称不能为空' }]}>
                {targetService ? (
                    <div className={style.aliasReadonlyValue} aria-readonly="true">
                        {targetService.namespace}/{targetService.serviceName}
                    </div>
                ) : (
                    <Select
                        filterable={true}
                        options={serviceDatas.map((item: ServiceView) => ({
                            label: `${item.name} (${item.namespace})`,
                            value: `${item.namespace}@${item.name}`,
                        }))}
                    />
                )}
            </FormItem>
            <FormItem label="别名所在命名空间" name="alias_namespace" rules={[{ required: true, message: '命名空间不能为空' }]}>
                {op === 'edit' ? <Input readonly /> : <Select
                    filterable={true}
                    options={namespaceDatas.map((item: NamespaceView) => ({
                        label: item.name,
                        value: item.name,
                    }))}
                />}
            </FormItem>
            <FormItem label="别名" name="alias" rules={[{ required: true, message: '别名不能为空' }]}>
                <Input id="inAlias" placeholder="请输入" readonly={op === 'edit'} />
            </FormItem>
            <FormItem label="备注" name="comment">
                <Input id="inRemark" placeholder="请输入" />
            </FormItem>
        </Form>
    );

    return (
        <>
            <div>
                <EditorDrawer
                    width="compact"
                    title={op === 'edit' ? "编辑服务别名" : "创建服务别名"}
                    description="为目标服务提供另一个可发现的名称。"
                    footer={(
                        <EditorDrawerActions
                            onCancel={closeDrawer}
                            onSubmit={() => form.submit({ showErrorMessage: true })}
                            submitText={op === 'edit' ? '保存修改' : '创建别名'}
                        />
                    )}
                    visible={visible}
                    className={style.aliasDrawer}
                    bodyClassName={style.serviceOperationBody}
                    onClose={closeDrawer}
                >
                    {aliasForm}
                </EditorDrawer>
            </div>
        </>
    )
}

export default React.memo(ServiceAliasEditor);
