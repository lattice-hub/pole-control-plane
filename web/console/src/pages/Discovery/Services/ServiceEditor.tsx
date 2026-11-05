import React from 'react';
import EditorDrawer, { EditorDrawerActions } from 'components/EditorDrawer';
import { Op } from 'services/types';
import type { Service } from 'services/service';
import ServiceForm, { ServiceFormHandle } from './ServiceForm';
import style from './index.module.less';

interface IServiceEditorProps {
    op: Op;
    visible: boolean;
    closeDrawer: () => void;
    fixedNamespace?: string;
    onCreated?: (service: Service) => void;
}

const ServiceEditor: React.FC<IServiceEditorProps> = ({ visible, closeDrawer, fixedNamespace, onCreated }) => {
    const formRef = React.useRef<ServiceFormHandle>(null);
    const [submitting, setSubmitting] = React.useState(false);

    return (
        <EditorDrawer
            width="standard"
            bodyClassName={style.serviceEditorBody}
            title="创建服务"
            description="在指定环境空间中登记服务身份，并补充归属和识别标签。"
            visible={visible}
            onClose={closeDrawer}
            footer={(
                <EditorDrawerActions
                    onCancel={closeDrawer}
                    onSubmit={() => formRef.current?.submit()}
                    submitText="创建服务"
                    submitting={submitting}
                />
            )}
        >
            {visible && <ServiceForm
                ref={formRef}
                mode="create"
                service={null}
                fixedNamespace={fixedNamespace}
                onSubmitted={(service) => { onCreated?.(service); closeDrawer(); }}
                onCancel={closeDrawer}
                onSubmittingChange={setSubmitting}
            />}
        </EditorDrawer>
    );
};

export default React.memo(ServiceEditor);
