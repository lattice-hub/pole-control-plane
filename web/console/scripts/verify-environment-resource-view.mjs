import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const read = (file) => fs.readFileSync(path.join(root, file), 'utf8');
const assertMatch = (source, pattern, message) => {
  if (!pattern.test(source)) throw new Error(message);
};

const namespacePage = read('src/pages/Namespace/index.tsx');
const environmentPage = read('src/pages/Namespace/EnvironmentResources.tsx');
const environmentService = read('src/services/environment_resources.ts');
const routes = read('src/router/modules/namespace.ts');
const configList = read('src/pages/Configuration/Unified/FileList.tsx');
const configService = read('src/services/unified_config.ts');
const configBackend = read('../../pkg/config/unified/file.go');

assertMatch(namespacePage, /\/namespace\/environment\?name=/, '业务环境名称必须进入环境资源视图。');
assertMatch(routes, /path:\s*['"]\/namespace\/environment['"]/, '必须注册独立的环境资源路由。');
assertMatch(environmentPage, /业务环境 \/ 资源视图/, '环境资源页必须明确当前环境上下文。');
assertMatch(environmentPage, /全部资源[\s\S]*注册发现[\s\S]*配置中心[\s\S]*治理规则[\s\S]*AI 工具/, '资源视图必须提供统一资源域筛选。');
assertMatch(environmentPage, /warnings\.length[\s\S]*部分资源暂未加载/, '跨域查询必须保留部分失败状态。');
assertMatch(environmentPage, /ConfirmOperationButton[\s\S]*action="delete"[\s\S]*删除后无法恢复/, '资源目录必须提供带二次确认的删除操作。');
assertMatch(environmentPage, /deleteEnvironmentResource\(resource\)[\s\S]*await refresh\(\)/, '删除成功后必须刷新当前环境资源。');
assertMatch(environmentService, /describeAllServices\(\{ namespace \}\)/, '服务资源必须按 Namespace 查询。');
assertMatch(configList, /listConfigFiles\(environment, group, query, tokens\[page\]\)/, '配置文件目录必须按所选环境查询并传递分页游标。');
assertMatch(configService, /request<ConfigFileList>\('files', \{ environment, group, query, pageSize: 50, pageToken \}/, '配置文件请求必须传递环境、筛选与分页条件。');
assertMatch(configBackend, /func \(s \*Service\) ListFiles[\s\S]*?s\.authorize\(ctx, &configv1\.ConfigKey\{Environment: req\.GetEnvironment\(\), Group: group, Name: "_"\}, ActionInspect\)[\s\S]*?st\.ListConfigFiles\(ctx, req\)/, '配置目录必须先对请求环境执行读取授权，再查询目录。');
assertMatch(environmentService, /describeCustomRoutes[\s\S]*describeLimitRules[\s\S]*describeCircuitBreakers/, '治理资源必须覆盖路由、限流和熔断。');
assertMatch(environmentService, /describeMCPServers[\s\S]*describeA2AAgents/, 'AI 资源必须覆盖 MCP Server 与 A2A Agent。');
assertMatch(environmentService, /Promise\.allSettled/, '环境资源查询必须允许单个资源域失败后继续展示其它结果。');
assertMatch(environmentService, /deleteServices[\s\S]*deleteCustomRoute[\s\S]*deleteRateLimit/, '删除适配器必须覆盖服务及主要治理规则。');
assertMatch(environmentService, /deleteMCPServers[\s\S]*deleteA2AAgents/, '删除适配器必须覆盖 MCP Server 与 A2A Agent。');
assertMatch(environmentService, /deleteable:\s*service\.deleteable !== false/, '资源目录必须保留服务删除权限。');
assertMatch(environmentService, /if \(!resource\.deleteable\) throw new Error/, '删除操作必须检查资源删除权限。');
assertMatch(environmentService, /default:\s*throw new Error\(`暂不支持删除/, '未知资源类型不得被删除。');
if (/case ['"](?:configuration|config-file|config-group)['"]/.test(environmentService) || /deleteConfigFileGroups/.test(environmentService)) throw new Error('环境资源目录不应绕过统一配置生命周期删除配置资源。');
assertMatch(environmentPage, /\/discovery\/service\?namespace=/, '注册发现入口必须保留环境过滤条件。');
assertMatch(environmentPage, /\/configuration\?environment=\$\{encodeURIComponent\(namespaceName\)\}/, '配置中心入口必须传递环境参数。');
assertMatch(environmentPage, /\/governance\/workbench\?namespace=/, '治理工作台入口必须保留环境过滤条件。');
assertMatch(environmentPage, /\/ai\/a2a\?namespace=/, 'AI 工具入口必须保留环境过滤条件。');

console.log('业务环境资源视图契约检查通过。');
