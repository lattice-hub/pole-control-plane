import React from 'react';
import { Form, Input, Select, Tag } from 'components/Fluent';
import type { FormProps } from 'components/Fluent';
import LabelInput from 'components/LabelInput';
import { useAppDispatch, useAppSelector } from 'modules/store';
import { listAllNamespaces, selectNamespace } from 'modules/namespace';
import { saveServices, updateServices } from 'modules/discovery/service';
import type { NamespaceView } from 'services/namespace';
import type { Service, ServiceView } from 'services/service';
import { describeAllServices } from 'services/service';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import style from './index.module.less';

const { FormItem } = Form;
const SERVICE_NAME_REG = /^[0-9A-Za-z._-]+$/;

export type ServiceFormMode = 'create' | 'edit' | 'view';

interface ServiceFormProps {
    mode: ServiceFormMode;
    service?: Service | ServiceView | null;
    fixedNamespace?: string;
    onSubmitted?: (service: Service) => void;
    onCancel?: () => void;
    onSubmittingChange?: (submitting: boolean) => void;
}

export interface ServiceFormHandle {
    submit: () => void;
    reset: () => void;
    submitting: boolean;
}

interface ServiceLabelRow {
    key: string;
    value: string;
}

const metadataToRows = (metadata?: Record<string, string>): ServiceLabelRow[] => (
    Object.entries(metadata || {}).map(([key, value]) => ({ key, value }))
);

const ReadonlyField = ({ value }: { value?: string }) => (
    <div className={style.readonlyField}>{value || '-'}</div>
);

const ReadonlyItem = ({
    label,
    value,
    wide = false,
}: {
    label: string;
    value?: string;
    wide?: boolean;
}) => (
    <div className={wide ? `${style.serviceReadonlyItem} ${style.serviceReadonlyItemWide}` : style.serviceReadonlyItem}>
        <span>{label}</span>
        <strong title={value || '-'}>{value || '-'}</strong>
    </div>
);

const ServiceForm = React.forwardRef<ServiceFormHandle, ServiceFormProps>(({ mode, service, fixedNamespace, onSubmitted, onSubmittingChange }, ref) => {
    const [form] = Form.useForm();
    const dispatch = useAppDispatch();
    const editable = mode !== 'view';
    const identityEditable = mode === 'create';
    const [allServices, setAllServices] = React.useState<ServiceView[]>([]);
    const [nameError, setNameError] = React.useState('');
    const nameHelpId = React.useId();
    const [submitting, setSubmitting] = React.useState(false);

    React.useEffect(() => {
        onSubmittingChange?.(submitting);
    }, [onSubmittingChange, submitting]);

    const { datas: namespaceDatas } = useAppSelector(selectNamespace);

    const watchedName = (Form.useWatch('name', form) as string | undefined) || '';
    const watchedNamespace = (Form.useWatch('namespace', form) as string | undefined) || '';
    const watchedComment = (Form.useWatch('comment', form) as string | undefined) || '';
    const watchedDepartment = (Form.useWatch('department', form) as string | undefined) || '';
    const watchedBusiness = (Form.useWatch('business', form) as string | undefined) || '';

    const initialValues = React.useCallback(() => ({
        name: service?.name || undefined,
        namespace: fixedNamespace || service?.namespace || undefined,
        comment: service?.comment || '',
        department: service?.department || '',
        business: service?.business || '',
        service_labels: metadataToRows(service?.metadata),
    }), [fixedNamespace, service?.business, service?.comment, service?.department, service?.metadata, service?.name, service?.namespace]);

    const validateServiceName = React.useCallback((name: string, namespace: string) => {
        const trimmedName = name.trim();
        if (!trimmedName) {
            return '请输入服务名称';
        }
        if (!SERVICE_NAME_REG.test(trimmedName)) {
            return '只允许数字、英文字母、.、-、_';
        }
        if (trimmedName.length > 128) {
            return '长度不超过128个字符';
        }
        if (mode === 'create' && namespace) {
            const duplicated = allServices.some((item) => item.namespace === namespace && item.name === trimmedName);
            if (duplicated) {
                return '该命名空间下服务名已存在';
            }
        }
        return '';
    }, [allServices, mode]);

    const resetForm = React.useCallback(() => {
        form.setFieldsValue(initialValues());
        setNameError('');
    }, [form, initialValues]);

    React.useImperativeHandle(ref, () => ({
        submit: () => form.submit({ showErrorMessage: true }),
        reset: resetForm,
        submitting,
    }), [form, resetForm, submitting]);

    React.useEffect(() => {
        resetForm();
    }, [mode, resetForm]);

    React.useEffect(() => {
        if (!editable) {
            return;
        }
        dispatch(listAllNamespaces())
            .then(res => {
                if (res.meta.requestStatus === 'rejected') {
                    openErrNotification('获取命名空间列表失败', res?.payload as string);
                }
            });
        if (mode === 'create') {
            describeAllServices()
                .then(setAllServices)
                .catch((error) => {
                    openErrNotification('获取服务列表失败', (error as Error).message);
                });
        }
    }, [dispatch, editable, mode]);

    const validateLabels = () => {
        const seenKeys = new Map<string, number>();
        const metadata: Record<string, string> = {};
        const labelRows = (form.getFieldValue('service_labels') as ServiceLabelRow[] | undefined) || [];
        let firstError = '';

        labelRows.forEach((row, index) => {
            const key = row.key.trim();
            const value = row.value.trim();
            if (!key && !value) {
                return;
            }
            if (!key && value) {
                firstError = firstError || '标签键不能为空';
                return;
            }
            const existedIndex = seenKeys.get(key);
            if (existedIndex !== undefined) {
                const message = `标签键 ${key} 重复`;
                firstError = firstError || message;
                return;
            }
            seenKeys.set(key, index);
            metadata[key] = value;
        });

        return {
            metadata,
            firstError,
        };
    };

    const onSubmit: FormProps['onSubmit'] = async (e) => {
        if (!editable) {
            return;
        }
        if (e.validateResult !== true) {
            const nextNameError = validateServiceName(watchedName, watchedNamespace);
            setNameError(nextNameError);
            return;
        }

        const nextNameError = validateServiceName(watchedName, watchedNamespace);
        if (nextNameError) {
            setNameError(nextNameError);
            openErrNotification('校验失败', nextNameError);
            return;
        }

        const labels = validateLabels();
        if (labels.firstError) {
            openErrNotification('校验失败', labels.firstError);
            return;
        }

        const newData = {
            id: service?.id || '',
            name: watchedName.trim(),
            namespace: fixedNamespace || watchedNamespace,
            comment: form.getFieldValue('comment') as string,
            department: form.getFieldValue('department') as string,
            business: form.getFieldValue('business') as string,
            metadata: labels.metadata,
            revision: service?.revision || '',
            ports: '',
            owners: '',
        };

        setSubmitting(true);
        try {
            const result = mode === 'create'
                ? await dispatch(saveServices({ param: { ...newData } }))
                : await dispatch(updateServices({ param: { ...newData } }));
            if (result.meta.requestStatus !== 'fulfilled') {
                openErrNotification('请求错误', result?.payload as string);
            } else {
                openInfoNotification('请求成功', mode === 'create' ? '服务已创建' : '修改服务成功');
                onSubmitted?.(newData);
            }
        } finally {
            setSubmitting(false);
        }
    };

    if (mode === 'view') {
        const metadata = Object.entries(service?.metadata || {});
        return (
            <div className={style.serviceReadonly}>
                <section className={style.serviceReadonlySection}>
                    <h3 className={style.serviceFormSectionTitle}>身份与描述</h3>
                    <div className={style.serviceReadonlyGrid}>
                        <ReadonlyItem label="命名空间" value={service?.namespace} />
                        <ReadonlyItem label="名称" value={service?.name} />
                        <ReadonlyItem label="描述" value={service?.comment} wide />
                    </div>
                </section>

                <section className={style.serviceReadonlySection}>
                    <h3 className={style.serviceFormSectionTitle}>归属信息</h3>
                    <div className={style.serviceReadonlyGrid}>
                        <ReadonlyItem label="部门" value={service?.department} />
                        <ReadonlyItem label="业务" value={service?.business} />
                    </div>
                </section>

                <section className={style.serviceReadonlySection}>
                    <h3 className={style.serviceFormSectionTitle}>服务标签</h3>
                    {metadata.length > 0 ? (
                        <div className={style.serviceReadonlyTags}>
                            {metadata.map(([key, value]) => (
                                <Tag key={key} theme="primary" variant="light">
                                    {key}: {value}
                                </Tag>
                            ))}
                        </div>
                    ) : (
                        <div className={style.serviceReadonlyEmpty}>暂无标签</div>
                    )}
                </section>
            </div>
        );
    }

    return (
        <Form
            form={form}
            className={style.serviceForm}
            layout="vertical"
            labelWidth={104}
            labelAlign="left"
            onSubmit={onSubmit}
        >
            <div id="drawer" className={style.serviceDrawerContent}>
                <section className={style.serviceFormSection}>
                    <h3 className={style.serviceFormSectionTitle}>身份与描述</h3>
                    <div id="inNs">
                        <FormItem
                            style={{ marginBottom: 0 }}
                            label="命名空间"
                            name="namespace"
                            rules={identityEditable ? [{ required: true, message: '请选择命名空间' }] : []}
                        >
                            {identityEditable && fixedNamespace ? <Input readonly /> : identityEditable ? (
                                <Select
                                    filterable
                                    placeholder="请选择"
                                    options={namespaceDatas.map((item: NamespaceView) => ({
                                        label: item.name,
                                        value: item.name,
                                    }))}
                                    onChange={(value: string) => {
                                        const nextError = validateServiceName(watchedName, String(value || ''));
                                        setNameError(watchedName ? nextError : '');
                                    }}
                                />
                            ) : <ReadonlyField value={watchedNamespace} />}
                        </FormItem>
                    </div>
                    <div id="inName">
                        <FormItem
                            style={{ marginBottom: 0 }}
                            label="名称"
                            name="name"
                            help={identityEditable && nameError ? <span id="errName" className={style.inlineError}>{nameError}</span> : undefined}
                            rules={identityEditable ? [
                                { required: true, message: '请输入服务名称' },
                                { pattern: SERVICE_NAME_REG, message: '只允许数字、英文字母、.、-、_' },
                                { max: 128, message: '长度不超过128个字符' },
                            ] : []}
                        >
                            {identityEditable ? (
                                <Input
                                    aria-describedby={`${nameHelpId}-hint nameCount`}
                                    aria-invalid={Boolean(nameError) || undefined}
                                    placeholder="允许数字、英文字母、.、-、_，限制128个字符"
                                    onChange={(value) => {
                                        setNameError(validateServiceName(String(value), watchedNamespace));
                                    }}
                                />
                            ) : <ReadonlyField value={watchedName} />}
                        </FormItem>
                        {identityEditable && (
                            <>
                                <div className={style.nameMeta}>
                                    <span id={`${nameHelpId}-hint`}>命名后不可修改，请谨慎填写</span>
                                    <span
                                        id="nameCount"
                                        className={watchedName.length > 128 ? style.nameCountError : undefined}
                                    >
                                        {watchedName.length}/128
                                    </span>
                                </div>
                            </>
                        )}
                    </div>
                    <div id="inDesc">
                        <FormItem
                            style={{ marginBottom: 0 }}
                            label="描述"
                            name="comment"
                            rules={editable ? [{ max: 1024, message: '长度不超过1024个字符' }] : []}
                        >
                            {editable ? <Input autoFocus={mode === 'edit'} placeholder="请输入服务描述" /> : <ReadonlyField value={watchedComment} />}
                        </FormItem>
                    </div>
                </section>

                <section className={style.serviceFormSection}>
                    <h3 className={style.serviceFormSectionTitle}>归属信息</h3>
                    <div id="inDept">
                        <FormItem
                            style={{ marginBottom: 0 }}
                            label="部门"
                            name="department"
                            rules={editable ? [{ max: 255, message: '长度不超过255个字符' }] : []}
                        >
                            {editable ? <Input placeholder="请输入部门" /> : <ReadonlyField value={watchedDepartment} />}
                        </FormItem>
                    </div>
                    <div id="inBiz">
                        <FormItem
                            style={{ marginBottom: 0 }}
                            label="业务"
                            name="business"
                            rules={editable ? [{ max: 255, message: '长度不超过255个字符' }] : []}
                        >
                            {editable ? <Input placeholder="请输入业务" /> : <ReadonlyField value={watchedBusiness} />}
                        </FormItem>
                    </div>
                </section>

                <section className={style.serviceFormSection}>
                    <h3 className={style.serviceFormSectionTitle}>服务标签</h3>
                    <LabelInput
                        form={form}
                        label="服务标签"
                        name="service_labels"
                        editable={editable}
                        disabled={!editable}
                        hideLabel
                        editorId="tagRows"
                        emptyId="tagsEmpty"
                        countId="tagCount"
                    />
                </section>

            </div>
        </Form>
    );
});

ServiceForm.displayName = 'ServiceForm';

export default React.memo(ServiceForm);
