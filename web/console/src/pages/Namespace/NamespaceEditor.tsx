import React from 'react';
import { Form, Input, Space, Button } from 'components/Fluent';
import type { FormProps } from 'components/Fluent';
import EditorDrawer, { EditorDrawerActions } from 'components/EditorDrawer';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import { useAppDispatch, useAppSelector } from 'modules/store';
import { saveNamespace, updateNamespace, selectNamespace } from 'modules/namespace';
import LabelInput from 'components/LabelInput';
import { Op } from 'services/types';
import style from './index.module.less';

const { FormItem } = Form;

interface NamespaceEditorProps {
    op: Op;
    closeDrawer: () => void;
    visible: boolean;
}

interface NamespaceLabelRow {
    key: string;
    value: string;
}

const NAMESPACE_NAME_REG = /^[a-zA-Z0-9._-]+$/;

const metadataToRows = (metadata?: Record<string, string>): NamespaceLabelRow[] => (
    Object.entries(metadata || {}).map(([key, value]) => ({ key, value }))
);

const ReadonlyField = ({ value }: { value?: string }) => (
    <div className={style.readonlyField}>{value || '-'}</div>
);

const NamespaceEditor: React.FC<NamespaceEditorProps> = ({ visible, op, closeDrawer }) => {
    const [form] = Form.useForm();
    const [editable, setEditable] = React.useState(op !== 'view');
    const [nameError, setNameError] = React.useState('');
    const [submitting, setSubmitting] = React.useState(false);
    const nameHelpId = React.useId();

    const dispatch = useAppDispatch();
    const namespaceState = useAppSelector(selectNamespace);
    const { editNs } = namespaceState;

    const watchedName = (Form.useWatch('name', form) as string | undefined) || '';
    const watchedComment = (Form.useWatch('comment', form) as string | undefined) || '';

    const initialValues = React.useCallback(() => ({
        name: editNs?.name || undefined,
        comment: editNs?.comment || '',
        namespace_labels: metadataToRows(editNs?.metadata),
    }), [editNs?.comment, editNs?.metadata, editNs?.name]);

    const validateNamespaceName = React.useCallback((name: string) => {
        const trimmedName = name.trim();
        if (!trimmedName) {
            return '命名空间不能为空';
        }
        if (!NAMESPACE_NAME_REG.test(trimmedName)) {
            return '只允许数字、英文字母、.、-、_';
        }
        if (trimmedName.length > 128) {
            return '长度不超过128个字符';
        }
        return '';
    }, []);

    const resetForm = React.useCallback(() => {
        form.setFieldsValue(initialValues());
        setNameError('');
    }, [form, initialValues]);

    React.useEffect(() => {
        if (visible) {
            setEditable(op !== 'view');
            resetForm();
        }
    }, [visible, op, resetForm]);

    const validateLabels = () => {
        const seenKeys = new Set<string>();
        const metadata: Record<string, string> = {};
        const labelRows = (form.getFieldValue('namespace_labels') as NamespaceLabelRow[] | undefined) || [];
        let firstError = '';

        labelRows.forEach((row) => {
            const key = row.key.trim();
            const value = row.value.trim();
            if (!key && !value) {
                return;
            }
            if (!key && value) {
                firstError = firstError || '标签键不能为空';
                return;
            }
            if (seenKeys.has(key)) {
                firstError = firstError || `标签键 ${key} 重复`;
                return;
            }
            seenKeys.add(key);
            metadata[key] = value;
        });

        return {
            metadata,
            firstError,
        };
    };

    const onSubmit: FormProps['onSubmit'] = async (e) => {
        if (e.validateResult !== true) {
            if (op === 'create') {
                setNameError(validateNamespaceName(watchedName));
            }
            return;
        }

        if (op === 'create') {
            const nextNameError = validateNamespaceName(watchedName);
            if (nextNameError) {
                setNameError(nextNameError);
                openErrNotification('校验失败', nextNameError);
                return;
            }
        }

        const labels = validateLabels();
        if (labels.firstError) {
            openErrNotification('校验失败', labels.firstError);
            return;
        }

        const data = {
            name: op === 'create' ? watchedName.trim() : editNs?.name || watchedName,
            comment: form.getFieldValue('comment') as string,
            metadata: labels.metadata,
        }
        setSubmitting(true);
        try {
            const result = op === 'create'
                ? await dispatch(saveNamespace({ param: data }))
                : await dispatch(updateNamespace({ param: data }));
            if (result.meta.requestStatus !== 'fulfilled') {
                openErrNotification('请求错误', result?.payload as string);
            } else {
                openInfoNotification('请求成功', op === 'create' ? '创建命名空间成功' : '修改命名空间成功');
                closeDrawer();
            }
        } finally {
            setSubmitting(false);
        }
    };

    const namespaceForm = (
        <Form
            form={form}
            className={style.namespaceForm}
            layout="vertical"
            labelAlign="left"
            onSubmit={onSubmit}
        >
            <div className={style.namespaceDrawerContent}>
                <section className={style.namespaceFormSection}>
                    <h3 className={style.namespaceFormSectionTitle}>基础信息</h3>
                    <div id="inNsName">
                        <FormItem
                            label="名称"
                            name="name"
                            style={{ marginBottom: 0 }}
                            help={editable && op === 'create' && nameError ? nameError : undefined}
                            rules={editable && op === 'create' ? [
                                { required: true, message: '命名空间不能为空' },
                                { pattern: NAMESPACE_NAME_REG, message: '只允许数字、英文字母、.、-、_' },
                                { max: 128, message: '长度不超过128个字符' }
                            ] : []}
                        >
                            {editable && op === 'create' ? (
                                <Input
                                    placeholder="允许数字、英文字母、.、-、_，限制128个字符"
                                    aria-describedby={`${nameHelpId}-hint ${nameHelpId}-count`}
                                    aria-invalid={Boolean(nameError) || undefined}
                                    onChange={(value) => setNameError(validateNamespaceName(String(value)))}
                                />
                            ) : <ReadonlyField value={watchedName} />}
                        </FormItem>
                        {editable && op === 'create' && (
                            <>
                                <div className={style.nameMeta}>
                                    <span id={`${nameHelpId}-hint`}>命名后不可修改，请谨慎填写</span>
                                    <span id={`${nameHelpId}-count`} className={watchedName.length > 128 ? style.nameCountError : undefined}>
                                        {watchedName.length}/128
                                    </span>
                                </div>
                            </>
                        )}
                    </div>
                    <div id="inNsKind">
                        <FormItem label="空间类型" style={{ marginBottom: 0 }}>
                            <ReadonlyField value={
                                editNs?.kind === 'SYSTEM' ? 'Pole 系统空间（当前控制面）' : '业务运行环境'
                            } />
                        </FormItem>
                    </div>
                    <div id="inNsDesc">
                        <FormItem
                            label="描述"
                            name="comment"
                            style={{ marginBottom: 0 }}
                            rules={editable ? [{ max: 1024, message: '长度不超过1024个字符' }] : []}
                        >
                            {editable ? <Input autoFocus={op === 'view'} placeholder="请输入该环境的用途或阶段说明" /> : <ReadonlyField value={watchedComment} />}
                        </FormItem>
                    </div>
                </section>

                <section className={style.namespaceFormSection}>
                    <h3 className={style.namespaceFormSectionTitle}>命名空间标签</h3>
                    <LabelInput
                        form={form}
                        label="命名空间标签"
                        name="namespace_labels"
                        editable={editable}
                        disabled={!editable}
                        hideLabel
                        editorId="namespaceTagRows"
                        emptyId="namespaceTagsEmpty"
                        countId="namespaceTagCount"
                    />
                </section>

            </div>
        </Form>
    )

    return (
        <div>
            <EditorDrawer
                width="standard"
                bodyClassName={style.namespaceEditorBody}
                title={op === 'create' ? "创建命名空间" : editable ? "编辑命名空间" : "命名空间详情"}
                description={op === 'create' ? '定义一个独立的业务环境空间，名称创建后不可修改。' : undefined}
                footer={op === 'view' && !editable ? (
                    <Space className={style.namespaceDrawerActions}>
                        <Button theme="primary" onClick={() => setEditable(true)}>
                            编辑
                        </Button>
                        <Button theme="default" onClick={closeDrawer}>
                            关闭
                        </Button>
                    </Space>
                ) : editable ? (
                    <EditorDrawerActions
                        onCancel={closeDrawer}
                        onSubmit={() => form.submit({ showErrorMessage: true })}
                        submitText={op === 'create' ? '创建命名空间' : '保存修改'}
                        submitting={submitting}
                    />
                ) : false}
                visible={visible}
                onClose={closeDrawer}
            >
                {namespaceForm}
            </EditorDrawer>
        </div>
    );
}

export default React.memo(NamespaceEditor);
