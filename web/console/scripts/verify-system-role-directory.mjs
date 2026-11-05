import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const read = (path) => readFileSync(new URL(`../${path}`, import.meta.url), 'utf8');

const table = read('src/pages/Auth/Principal/RoleTable.tsx');
const editor = read('src/pages/Auth/Principal/RoleEditor.tsx');
const principalPage = read('src/pages/Auth/Principal/index.tsx');
const roleModule = read('src/modules/auth/role.ts');
const policyEditor = read('src/pages/Auth/Policy/PolicyEditor.tsx');

assert.match(table, /新建角色/);
assert.match(table, /type:\s*['"]multiple['"]/);
assert.match(table, /removeRoles/);
assert.match(table, /aria-label=["']管理成员["']/);
assert.match(table, /系统内置/);
assert.match(table, /default_role === true/);
assert.match(table, /disabled: isBuiltInRole/);
assert.match(table, /isBuiltInRole\(role\) \?/);

assert.match(editor, /title=\{op === ['"]create['"] \? ['"]创建角色['"] : editable \? \(builtIn \? ['"]管理内置角色成员['"]/);
assert.match(editor, /users:\s*users\.map/);
assert.match(editor, /user_groups:\s*groups\.map/);
assert.match(editor, /name:\s*form\.getFieldValue/);
assert.match(editor, /comment:\s*\(form\.getFieldValue/);
assert.match(editor, /!builtIn && <LabelInput/);
assert.match(editor, /if \(builtIn\)/);

assert.match(principalPage, /可创建并维护自定义角色/);
assert.match(principalPage, /内置角色，仅允许调整用户与用户组成员/);
assert.match(roleModule, /Pick<RoleState, 'id'> & Partial<Omit<RoleState, 'id'>>/);
assert.match(policyEditor, /rolesRes\.filter\(\(r: any\) => !r\.default_role\)/);

console.log('custom role CRUD and built-in role protection contract verified');
