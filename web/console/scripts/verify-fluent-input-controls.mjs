import fs from 'node:fs';
import path from 'node:path';
import assert from 'node:assert/strict';

const root = process.cwd();
const read = (file) => fs.readFileSync(path.join(root, file), 'utf8');

const fluent = read('src/components/Fluent/index.tsx');
const labelInput = read('src/components/LabelInput/index.tsx');
const trafficSecurityEditor = read('src/pages/Governance/Security/TrafficGovernanceEditor.tsx');
const mockHeaderEditor = read('src/pages/Governance/Security/MockHeaderEditor.tsx');
const laneGroupEditor = read('src/pages/Governance/Router/LaneGroupEdtor.tsx');

assert.doesNotMatch(
  labelInput,
  /key=\{`\$\{index\}-\$\{item\.key\}`\}/,
  'LabelInput 行 key 不能包含正在编辑的标签键，否则输入一个字符就会重挂载并丢焦点',
);
assert.match(
  labelInput,
  /key=\{`label-row-\$\{index\}`\}/,
  'LabelInput 行 key 必须在编辑过程中保持稳定，保证标签键值可以连续输入',
);

assert.match(
  fluent,
  /const canType = Boolean\(filterable \|\| creatable\);[\s\S]*const \[inputValue, setInputValue\] = React\.useState\(displayValue\);/,
  'Fluent Select 必须为 filterable/creatable 场景维护独立输入文本状态',
);
assert.match(
  fluent,
  /value=\{canType \? inputValue : displayValue\}/,
  'Fluent Select 不能把可输入 Combobox 的 value 固定为选中项展示值',
);
assert.match(
  fluent,
  /onChange=\{\(event\) => \{[\s\S]*if \(canType\) \{[\s\S]*setInputValue\(event\.currentTarget\.value\);/,
  'Fluent Select 必须在键盘输入时更新 Combobox 输入文本，支持连续输入过滤',
);
assert.match(
  fluent,
  /const \[searchQuery, setSearchQuery\] = React\.useState\(''\);[\s\S]*const filteredOptions = filterable && searchQuery\.trim\(\)/,
  'Fluent Select 的 filterable 模式必须基于输入关键字过滤候选项，而不是只允许输入',
);
assert.match(
  fluent,
  /\{filteredOptions\.map\(\(option, index\) => \(/,
  'Fluent Select 必须只渲染过滤后的候选项',
);
assert.match(
  trafficSecurityEditor,
  /className=\{style\.managedCallerSelect\}[\s\S]*multiple[\s\S]*filterable[\s\S]*label="搜索来源服务"/,
  '托管身份来源服务必须使用带可访问标签的多选搜索下拉框',
);

assert.doesNotMatch(
  trafficSecurityEditor + mockHeaderEditor,
  /key=\{`\$\{row\.key\}-\$\{index\}`\}|key=\{`\$\{current\.protocol\}-\$\{current\.method\}-\$\{index\}`\}/,
  '治理鉴权/镜像编辑器的 Header 和接口输入行 key 不能包含正在编辑的字段值',
);
assert.match(
  mockHeaderEditor + trafficSecurityEditor,
  /key=\{`mock-header-\$\{index\}`\}[\s\S]*key=\{`protected-interface-\$\{index\}`\}[\s\S]*key=\{`mirror-interface-\$\{index\}`\}/,
  '治理鉴权/镜像编辑器的重复输入行必须使用稳定 key，避免连续输入时重挂载',
);
assert.doesNotMatch(
  laneGroupEditor,
  /key=\{`\$\{lane\.id \|\| lane\.name\}-\$\{laneIndex\}`\}|key=\{`\$\{serviceTag\.service\}-\$\{serviceIndex\}`\}/,
  '泳道编辑器的泳道卡和组内服务选择行 key 不能包含正在编辑的名称或服务值',
);

console.log('fluent input controls verification passed');
