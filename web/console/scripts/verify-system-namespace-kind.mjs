import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const read = relative => fs.readFileSync(path.join(root, relative), 'utf8');

const namespaceApi = read('src/services/namespace.ts');
const namespaceModule = read('src/modules/namespace/index.ts');
const namespacePage = read('src/pages/Namespace/index.tsx');
const configPage = read('src/pages/Configuration/Unified/index.tsx');
const logicalService = read('src/services/logical_service.ts');
const servicePage = read('src/pages/Discovery/Services/index.tsx');
const serviceList = read('src/pages/Discovery/Services/services.tsx');
const systemServices = read('src/pages/Discovery/Services/SystemNamespaceServices.tsx');

assert.match(namespaceApi, /type NamespaceKind = 'BUSINESS' \| 'SYSTEM'/,
  '前端必须使用类型化 NamespaceKind');
assert.match(namespaceApi, /item\.name === ['"]pole-system['"]/,
  '滚动升级期间必须按 pole-system 名称兜底识别系统空间');
assert.match(namespaceApi, /describeBusinessNamespaces/,
  '业务资源选择器必须有只返回业务环境的公共 seam');
assert.match(namespaceApi, /kind:\s*['"]business['"]/,
  '业务 Namespace 查询必须在服务端分页前传入 kind filter');
assert.match(namespaceModule, /describeBusinessNamespaces/,
  '共享业务环境 thunk 不得继续加载系统空间');

assert.match(namespacePage, /Pole 系统空间（当前控制面）/,
  'Namespace 页面必须独立解释系统空间');
assert.match(namespacePage, /systemNamespaces/,
  'Namespace 页面必须把系统空间与业务环境分区');
assert.match(namespacePage, /<Tabs[\s\S]*value=\{activeWorkspace\}/,
  '业务环境与系统空间必须在同一工作区通过受控页签切换');
assert.match(namespacePage, /<TabPanel value="business"[\s\S]*<TabPanel value="system"/,
  'Namespace 页面必须提供业务环境和系统空间两个互斥页签');
assert.doesNotMatch(namespacePage, /systemNamespaceSection/,
  '系统空间不得继续作为业务表格下方的纵向第二分区');
assert.match(namespacePage, /新建业务环境/,
  '创建入口必须明确只创建业务环境');
assert.match(namespacePage, /维护系统配置[\s\S]*查看 MCP[\s\S]*查看 Agent/,
  '系统空间必须提供可发现的专用维护入口');
assert.match(namespacePage, /查看系统服务[\s\S]*查看配置资源/,
  '系统空间必须提供原始服务与配置资源入口');
assert.match(servicePage, /scope.*system[\s\S]*SystemNamespaceServices/,
  '服务页必须提供单 Namespace 的系统维护模式');
assert.match(serviceList, /<TabPanel value="logical"[\s\S]*<TabPanel value="unbound"[\s\S]*<TabPanel value="system"/,
  '服务首页必须并列提供逻辑服务、未关联环境服务和系统服务三个一级页签');
assert.match(serviceList, /activeTab === ['"]system['"][\s\S]*SystemNamespaceServices/,
  '系统服务页签必须复用按 SYSTEM Namespace 查询的原始服务列表');
assert.match(serviceList, /scope=system&namespace=/,
  '系统服务页签必须保留可分享的 scope=system 深链');
assert.match(systemServices, /describeServices\([\s\S]*namespace/,
  '系统服务维护模式必须按 pole-system 显式查询原始服务');
assert.match(configPage, /params\.get\('environment'\)/,
  '配置页必须按显式 environment key 定位，支持 pole-system');
assert.match(namespacePage, /refreshTable\(page, limit, query\);[\s\S]*refreshSystemNamespaces\(\)/,
  '编辑 Drawer 关闭后必须同时刷新业务和系统空间');

assert.match(logicalService, /namespace !== ['"]pole-system['"]/,
  '逻辑服务前端必须对旧后端结果做系统空间防御过滤');

console.log('system namespace kind contract verified');
