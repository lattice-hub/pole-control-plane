import React from 'react';
import { Button, Drawer } from 'components/Fluent';

import style from './index.module.less';

export type EditorDrawerWidth = 'compact' | 'standard' | 'wide' | 'workspace';

interface EditorDrawerProps {
    visible: boolean;
    title: React.ReactNode;
    description?: React.ReactNode;
    width?: EditorDrawerWidth;
    onClose: () => void;
    children: React.ReactNode;
    footer?: React.ReactNode | false;
    className?: string;
    bodyClassName?: string;
    closeOnOverlayClick?: boolean;
    closeOnEscKeydown?: boolean;
    destroyOnClose?: boolean;
}

interface EditorDrawerActionsProps {
    onCancel: () => void;
    onSubmit: () => void;
    submitText: string;
    submitting?: boolean;
    submitDisabled?: boolean;
    cancelText?: string;
}

export const EditorDrawerActions: React.FC<EditorDrawerActionsProps> = ({
    onCancel,
    onSubmit,
    submitText,
    submitting = false,
    submitDisabled = false,
    cancelText = '取消',
}) => (
    <div className={style.actions}>
        <Button variant="outline" disabled={submitting} onClick={onCancel}>{cancelText}</Button>
        <Button
            theme="primary"
            loading={submitting}
            disabled={submitting || submitDisabled}
            onClick={onSubmit}
        >
            {submitText}
        </Button>
    </div>
);

const EditorDrawer: React.FC<EditorDrawerProps> = ({
    visible,
    title,
    description,
    width = 'standard',
    onClose,
    children,
    footer,
    className,
    bodyClassName,
    closeOnOverlayClick = false,
    closeOnEscKeydown = true,
    destroyOnClose = true,
}) => {
    React.useLayoutEffect(() => {
        if (!visible) return;
        const opener = document.activeElement;
        // Callers may close by unmounting the editor, so restore from cleanup too.
        return () => {
            requestAnimationFrame(() => {
                if (opener instanceof HTMLElement && opener.isConnected && document.activeElement === document.body) {
                    opener.focus({ preventScroll: true });
                }
            });
        };
    }, [visible]);

    return (<Drawer
        visible={visible}
        onClose={onClose}
        size={width}
        className={`${style.drawer} ${className || ''}`}
        headerClassName={style.header}
        bodyClassName={`${style.body} ${bodyClassName || ''}`}
        footerClassName={style.footer}
        header={(
            <div className={style.title}>
                <strong>{title}</strong>
                {description && <span>{description}</span>}
            </div>
        )}
        footer={footer}
        showOverlay
        closeOnOverlayClick={closeOnOverlayClick}
        closeOnEscKeydown={closeOnEscKeydown}
        destroyOnClose={destroyOnClose}
    >
        {children}
    </Drawer>);
};

export default EditorDrawer;
