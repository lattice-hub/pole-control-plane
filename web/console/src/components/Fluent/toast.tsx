import React from 'react';
import {
  Button,
  Toast,
  ToastBody,
  ToastFooter,
  Toaster,
  ToastIntent,
  ToastTitle,
  useToastController,
} from '@fluentui/react-components';
import { Dismiss20Regular } from '@fluentui/react-icons';
import style from './toast.module.less';

const toasterId = 'lattice-hub-toaster';
let toastSequence = 0;

interface ToastMessage {
  title: unknown;
  body?: unknown;
  action?: React.ReactNode;
  footer?: React.ReactNode;
  intent: ToastIntent;
  timeout?: number;
}

const listeners = new Set<(message: ToastMessage) => void>();

const normalizeToastContent = (content: unknown): React.ReactNode => {
  if (React.isValidElement(content) || typeof content === 'string' || typeof content === 'number') return content;
  if (content instanceof Error) return content.message;
  if (content === undefined || content === null) return '';
  if (Array.isArray(content)) return content.map(normalizeToastContent);
  try {
    return JSON.stringify(content);
  } catch {
    return String(content);
  }
};

export const showFluentToast = (message: ToastMessage) => {
  listeners.forEach((listener) => listener(message));
};

export const FluentToastHost: React.FC = () => {
  const { dismissToast, dispatchToast } = useToastController(toasterId);

  React.useEffect(() => {
    const listener = ({ title, body, action, footer, intent, timeout = 10000 }: ToastMessage) => {
      const normalizedTitle = normalizeToastContent(title);
      const normalizedBody = normalizeToastContent(body);
      const toastId = `${toasterId}-${++toastSequence}`;
      dispatchToast(
        <Toast style={{ width: '100%', maxWidth: '100%', boxSizing: 'border-box' }}>
          <ToastTitle
            action={(
              <div style={{ display: 'flex', alignItems: 'center', gap: '2px' }}>
                {action}
                <Button
                  appearance="subtle"
                  aria-label="关闭通知"
                  icon={<Dismiss20Regular />}
                  size="small"
                  onClick={() => dismissToast(toastId)}
                />
              </div>
            )}
          >
            {normalizedTitle}
          </ToastTitle>
          {normalizedBody !== '' && (
            <ToastBody style={{ gridColumnEnd: 4 }}>{normalizedBody}</ToastBody>
          )}
          {footer && <ToastFooter>{footer}</ToastFooter>}
        </Toast>,
        { intent, timeout, position: 'top', toastId },
      );
    };
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  }, [dismissToast, dispatchToast]);

  return (
    <Toaster
      toasterId={toasterId}
      className={style.toaster}
      offset={{ vertical: 56, horizontal: 0 }}
      limit={2}
      pauseOnHover={false}
    />
  );
};

export { normalizeToastContent };
