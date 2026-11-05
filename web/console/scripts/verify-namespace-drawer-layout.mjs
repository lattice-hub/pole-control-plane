import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();

const read = (relativePath) => fs.readFileSync(path.join(root, relativePath), 'utf8');

const assert = (condition, message) => {
  if (!condition) {
    throw new Error(message);
  }
};

const editor = read('src/pages/Namespace/NamespaceEditor.tsx');
const page = read('src/pages/Namespace/index.tsx');
const styles = read('src/pages/Namespace/index.module.less');

assert(editor.includes('const ReadonlyField'), '命名空间查看态必须使用无边框只读字段');
assert(editor.includes('const namespaceForm ='), '命名空间查看和编辑必须复用同一个表单布局');
assert(editor.includes('{namespaceForm}'), '命名空间抽屉必须始终渲染统一表单布局');
assert(editor.includes('layout="vertical"'), '命名空间字段应使用纵向布局，保持窄屏标签、输入和提示对齐');
assert(!editor.includes('namespaceView'), '命名空间抽屉不能保留独立详情展示结构');
assert(editor.includes('width="standard"'), '命名空间抽屉必须使用共享的标准宽度档位');
assert(editor.includes('editable ? "编辑命名空间" : "命名空间详情"'), '命名空间标题必须由 editable 状态驱动');
assert(editor.includes('<Button theme="primary" onClick={() => setEditable(true)}>'), '命名空间查看态必须支持原地切换编辑');
assert(!editor.includes('onAuthorize'), '命名空间详情抽屉底部不能提供授权入口');
assert(editor.includes('editable={editable}') && editor.includes('disabled={!editable}'), '命名空间标签必须支持查看/编辑双态');
assert(page.includes('OperationButton action="authorize"'), '命名空间授权入口必须保留在列表操作列');
assert(!page.includes('onAuthorize={()'), '命名空间页面不应再向详情抽屉传递授权回调');
assert(styles.includes('.namespaceFormSectionTitle'), '命名空间表单必须有和服务一致的分段标题样式');
assert(styles.includes('.readonlyField'), '命名空间必须定义只读字段样式');
assert(!styles.includes('.namespaceDetail'), '命名空间样式不能保留独立详情结构');
assert(!/\.readonlyField\s*\{[\s\S]*?(border|background):/.test(styles), '命名空间只读字段不能呈现 disabled 输入框样式');

console.log('命名空间抽屉布局静态约束验证通过');
