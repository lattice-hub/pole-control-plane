import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..');

const checks = [
  {
    file: 'src/services/instance.ts',
    required: ['const instances = (res.data ?? res.instances ?? []).map'],
    message: '服务实例列表必须先读标准 data，再兼容 res.instances。',
  },
  {
    file: 'src/services/alias.ts',
    required: ['const aliases = result.data ?? result.aliases ?? []'],
    message: '服务别名列表必须先读标准 data，再兼容 result.aliases。',
  },
  {
    file: 'src/services/users.ts',
    required: ['const users = result.data ?? result.users ?? []'],
    message: '用户列表必须先读标准 data，再兼容 result.users。',
  },
  {
    file: 'src/services/user_group.ts',
    required: ['const userGroups = result.data ?? result.userGroups ?? []'],
    message: '用户组列表必须先读标准 data，再兼容 result.userGroups。',
  },
  {
    file: 'src/services/auth_policy.ts',
    required: ['const authStrategies = result.data ?? result.authStrategies ?? []'],
    message: '鉴权策略列表必须先读标准 data，再兼容 result.authStrategies。',
  },
  {
    file: 'src/services/ratelimit.ts',
    required: ['const rateLimits = res.data ?? res.rateLimits ?? []'],
    message: '限流规则列表必须先读标准 data，再兼容 res.rateLimits。',
  },
  {
    file: 'src/services/service.ts',
    required: ['const services = res.data ?? res.services ?? []', 'const subscribers = res.data ?? res.subscribers ?? []'],
    message: '服务和服务订阅列表必须先读标准 data，再兼容旧字段。',
  },
  {
    file: 'src/pages/Discovery/Services/Instance/ServiceDetail.tsx',
    required: [
      'const ServiceDetail: React.FC<IServiceDetailProps> = ({',
      'serviceName,',
      'initialEdit = false,',
      'dispatch(listOneService({',
      'namespace,',
      'name: serviceName,',
    ],
    message: '服务详情页刷新后 Redux editSvc 为空，必须按 URL 传入的 namespace/serviceName 重新查询。',
  },
  {
    file: 'src/router/modules/governance.ts',
    required: ["path: ''", "Component: lazy(() => import('pages/Governance/Workbench'))"],
    message: '治理根路径 /governance/ 必须有默认工作台入口，避免主区空白。',
  },
  {
    file: 'src/pages/Governance/Router/CustomRoute.tsx',
    required: ['editorState.data?.editable ?? true', 'editorState.data?.deleteable ?? true', 'routing_config?.rules?.[0]', '选择路由规则查看详情'],
    message: '自定义路由详情区必须保留后端 false 权限，并安全读取 routing_config。',
  },
  {
    file: 'src/pages/Governance/Router/LaneGroupTable.tsx',
    required: ['removeLaneGroups({ ids: [row.id] })', 'PolicySourceType.LaneRules', 'editor.data?.editable ?? true', 'editor.data?.deleteable ?? true', '选择泳道组查看详情'],
    message: '泳道组必须接入删除/授权动作，详情区必须保留后端 false 权限并提供空态。',
  },
  {
    file: 'src/pages/Governance/Router/LaneRuleTable.tsx',
    required: ['removeLaneRules({', 'groupName: row.groupName || editGroup?.name ||', '删除泳道规则成功'],
    message: '泳道规则删除按钮必须真正调用删除接口，并刷新当前泳道组规则。',
  },
  {
    file: 'src/pages/Governance/RateLimit/RateLimitTable.tsx',
    required: ['data: { ...row }', 'editor.data?.editable ?? true', 'editor.data?.deleteable ?? true'],
    message: '限流授权和版本/监听区必须绑定当前行，并保留后端 false 权限。',
  },
  {
    file: 'src/pages/Governance/LossLess/LossLessTable.tsx',
    required: ['data: { ...row }', 'editor.data?.editable ?? true', 'editor.data?.deleteable ?? true', 'name: query'],
    message: '无损授权和版本/监听区必须绑定当前行、保留后端 false 权限，并让搜索条件进入请求。',
  },
  {
    file: 'src/pages/Governance/CircuitBreaker/CircuitBreakerTable.tsx',
    required: ['editorState.data?.editable ?? true', 'editorState.data?.deleteable ?? true', 'ruleMatcher?.source', 'refreshData(1, limit);'],
    message: '熔断列表必须安全读取规则匹配字段，删除成功后刷新，并保留后端 false 权限。',
  },
  {
    file: 'src/pages/Governance/CircuitBreaker/FaultDetectTable.tsx',
    required: ['editorState.data?.editable ?? true', 'editorState.data?.deleteable ?? true', 'targetService?.api', 'refreshData(1, limit);'],
    message: '主动探测列表必须安全读取 targetService/api，删除成功后刷新，并保留后端 false 权限。',
  },
  {
    file: 'src/services/lane.ts',
    required: ['totalCount: result.amount ?? result.total ?? result.data?.length ?? 0'],
    message: '泳道版本列表必须优先按标准 amount 解包，再兼容旧 total。',
  },
  {
    file: 'src/services/lossless.ts',
    required: ['totalCount: result.amount ?? result.total ?? result.data?.length ?? 0'],
    message: '无损版本列表必须优先按标准 amount 解包，再兼容旧 total。',
  },
  {
    file: 'src/services/traffic_governance.ts',
    required: ['const keyedList = res[dataKey(kind) as keyof DescribeTrafficGovernanceResponse<T>] as T[] | undefined', 'const list = res.data ?? keyedList ?? []'],
    message: '流量治理规则列表必须先读标准 data，再兼容按类型返回的旧字段。',
  },
];

const failures = checks.flatMap((check) => {
  const content = fs.readFileSync(path.join(root, check.file), 'utf8');
  return check.required
    .filter((pattern) => !content.includes(pattern))
    .map((pattern) => `${check.file}: missing "${pattern}". ${check.message}`);
});

if (failures.length > 0) {
  console.error(failures.join('\n'));
  process.exit(1);
}

console.log(`standard response mapping checks passed (${checks.length} files)`);
