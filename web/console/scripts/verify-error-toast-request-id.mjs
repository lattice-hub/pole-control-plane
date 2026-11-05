import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const read = (relativePath) => fs.readFileSync(path.join(root, relativePath), 'utf8');
const assert = (condition, message) => {
  if (!condition) throw new Error(message);
};

const notification = read('src/utils/notifition.ts');
const toast = read('src/components/Fluent/toast.tsx');
const toastStyle = read('src/components/Fluent/toast.module.less');

assert(notification.includes('export const splitRequestId'), '错误通知必须兼容解析旧字符串中的 RequestId');
assert(notification.includes('requestError.codeLabel'), '错误通知必须单独格式化展示错误码');
assert(notification.includes('requestError.messageLabel'), '错误通知必须单独格式化展示错误信息');
assert(notification.includes("action: createElement("), '错误通知必须将复制操作放在标题右上角');
assert(notification.includes("i18n.t('requestError.copy')"), '错误通知必须仅保留一个带文案的复制按钮');
assert(notification.includes("'aria-label': i18n.t('requestError.copy')"), '右上复制按钮必须有可访问名称');
assert(notification.includes('JSON.stringify(payload, null, 2)'), '完整错误信息必须按格式化 JSON 复制');
assert(!notification.includes('footer: createElement'), '错误通知不应保留底部复制操作区');
assert(!notification.includes('requestError.copyRequestId'), '错误通知不应保留独立 Request ID 复制按钮');
assert(!notification.includes('`请求失败（${code}）'), '错误消息不得再拼接业务码');
assert(toast.includes('ToastFooter'), 'Toast 容器必须继续兼容可选底部内容');
assert(toast.includes('footer?: React.ReactNode'), 'Toast 消息模型必须继续支持可选底部内容');
assert(toast.includes('action?: React.ReactNode'), 'Toast 消息模型必须支持标题右侧操作');
assert(toast.includes('className={style.toaster}') && /\.toaster:global\(\.fui-Toaster\)\s*\{[^}]*width:\s*420px;[^}]*max-width:\s*calc\(100vw - 32px\)/.test(toastStyle), 'Toast 定位容器必须保持 420px 桌面宽度和窄屏边界');
assert(/\.toaster:global\(\.fui-ToastContainer\)\s*\{[^}]*width:\s*100%/.test(toastStyle), '单条通知必须服从定位容器宽度，不能再次应用定位偏移');
assert(toast.includes('ToastBody style={{ gridColumnEnd: 4 }}'), 'Toast 正文必须跨越 Fluent 右侧空网格列，避免过早换行');
assert(toast.includes('aria-label="关闭通知"'), '每条 Toast 必须提供手动关闭入口');
assert(toast.includes('dismissToast(toastId)'), '关闭入口必须实际 dismiss 当前 Toast');
assert(toast.includes('timeout = 10000'), 'Toast 默认自动关闭时间必须为 10 秒');

const request = read('src/utils/request.ts');
assert(request.includes('export interface RequestErrorPayload'), '请求层必须定义结构化错误载荷');
assert(request.includes('message: detail || \'请求失败\''), '请求层 message 只能保留业务错误信息');
assert(!request.includes('RequestId: ${reqId}'), '请求层 message 不得拼接 Request ID');
assert(notification.includes("intent: 'success', timeout: 10000"), '成功通知必须在 10 秒后自动关闭');

console.log('结构化错误通知静态约束验证通过');
