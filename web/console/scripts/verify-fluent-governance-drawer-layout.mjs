import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const read = (file) => fs.readFileSync(path.join(root, file), 'utf8');
const assert = (condition, message) => {
  if (!condition) throw new Error(message);
};

const fluent = read('src/components/Fluent/index.tsx');
const drawer = read('src/pages/Governance/RuleRelease/RuleDetailDrawer.tsx');
const drawerStyle = read('src/pages/Governance/RuleRelease/RuleDetailDrawer.module.less');
const action = read('src/pages/Governance/RuleRelease/RuleStickyAction.tsx');
const publish = read('src/pages/Governance/RuleRelease/PublishForm.tsx');
const publishStyle = read('src/pages/Governance/RuleRelease/PublishForm.module.less');
const laneStyle = read('src/pages/Governance/Router/LaneGroupEditor.module.less');
const routeStyle = read('src/pages/Governance/Router/CustomRouteEditor.module.less');
const securityStyle = read('src/pages/Governance/Security/index.module.less');
const rateLimitTable = read('src/pages/Governance/RateLimit/RateLimitTable.tsx');
const circuitBreakerEditor = read('src/pages/Governance/CircuitBreaker/CircuitBreakerEditor.tsx');

assert(fluent.includes("{ ...style, width: size, maxWidth: 'calc(100vw - 32px)' }"), 'Fluent Drawer must preserve custom governance drawer widths and keep size authoritative');
assert(fluent.includes('headerClassName') && fluent.includes('bodyClassName'), 'Fluent Drawer must expose stable layout classes');
assert(fluent.includes('<Dismiss20Regular />'), 'Fluent Drawer must use the Fluent dismiss icon');

assert(drawer.includes('RuleDetailActionHostContext'), 'Rule detail drawer must expose a shared header action host');
assert(drawer.includes("DEFAULT_RULE_DETAIL_DRAWER_SIZE = 'clamp(720px, 52vw, 960px)'"), 'Default rule details must remain narrower than complex editors');
assert(drawer.includes('headerClassName={style.drawerHeader}'), 'Rule detail drawer must own its Fluent header surface');
assert(drawer.includes('bodyClassName={style.drawerBody}'), 'Rule detail drawer must own its Fluent body surface');
assert(action.includes('createPortal(action, actionHost)'), 'Governance actions must render in the shared drawer header');

assert(!drawerStyle.includes('.t-drawer__'), 'Rule detail drawer must not depend on legacy TDesign drawer classes');
assert(!drawerStyle.includes('.t-tabs__'), 'Rule detail drawer must not depend on legacy TDesign tab classes');
assert(drawerStyle.includes("[role='tablist']") && drawerStyle.includes('.fluent-tab-content'), 'Rule detail tabs must use Fluent-owned layout boundaries');
assert(drawerStyle.includes("[class*='_sectionCard_']"), 'Rule detail drawer must normalize legacy editor surfaces');
assert(publish.includes('bodyClassName={style.drawerBody}') && publish.includes('footerClassName={style.drawerFooter}'), 'Publish drawer must use stable Fluent body and footer classes');
assert(!publishStyle.includes('.t-drawer__'), 'Publish drawer must not depend on legacy TDesign drawer classes');
assert(!laneStyle.includes('.t-tabs') && laneStyle.includes('.fluent-tab-content'), 'Lane editor tabs must use Fluent-owned layout boundaries');
assert(routeStyle.includes('height: calc(100vh - 158px)') && laneStyle.includes('height: calc(100vh - 158px)'), 'Complex route and lane editors must share the bounded editor height');
assert(securityStyle.includes('.ruleWorkspace') && securityStyle.includes('min-width: 0'), 'Direct-page security editors must use a fluid page boundary instead of a drawer-only fixed height');
assert(rateLimitTable.includes('size={WIDE_RULE_DETAIL_DRAWER_SIZE}'), 'Rate-limit details must use the same wide drawer in direct and workbench entrypoints');
assert(circuitBreakerEditor.includes("op === 'create' || viewRule?.editable !== false"), 'Circuit-breaker actions must remain available unless the API explicitly denies editing');

console.log('Fluent governance drawer layout contracts verified');
