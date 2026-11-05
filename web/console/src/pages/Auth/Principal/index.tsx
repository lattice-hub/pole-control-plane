import React from 'react';
import { Button, TabPanel, Tabs } from 'components/Fluent';
import { ResourceHeader } from 'components/ResourceLayout';
import { useSearchParams } from 'components/Router';
import { describeRoles } from 'services/role';
import { describeUserGroups } from 'services/user_group';
import { describeUsers } from 'services/users';

import UserTable from './UserTable';
import GroupsTable from './GroupTable';
import RoleTable from './RoleTable';
import style from './index.module.less';

type PrincipalTotals = {
    users: number | null;
    groups: number | null;
    roles: number | null;
};

export default React.memo(() => {
    const [searchParams, setSearchParams] = useSearchParams();
    const activeTab = searchParams.get('tab') || '1';
    const [totals, setTotals] = React.useState<PrincipalTotals>({
        users: null,
        groups: null,
        roles: null,
    });
    const [totalsLoading, setTotalsLoading] = React.useState(true);

    const loadTotals = React.useCallback(async () => {
        setTotalsLoading(true);
        const [users, groups, roles] = await Promise.allSettled([
            describeUsers({ offset: 0, limit: 1 }),
            describeUserGroups({ offset: 0, limit: 1 }),
            describeRoles({ offset: 0, limit: 1 }),
        ]);
        setTotals((current) => ({
            users: users.status === 'fulfilled' ? users.value.totalCount : current.users,
            groups: groups.status === 'fulfilled' ? groups.value.totalCount : current.groups,
            roles: roles.status === 'fulfilled' ? roles.value.totalCount : current.roles,
        }));
        setTotalsLoading(false);
    }, []);

    React.useEffect(() => {
        void loadTotals();
    }, [loadTotals]);

    const updateUsersTotal = React.useCallback((users: number) => {
        setTotals((current) => ({ ...current, users }));
    }, []);
    const updateGroupsTotal = React.useCallback((groups: number) => {
        setTotals((current) => ({ ...current, groups }));
    }, []);
    const updateRolesTotal = React.useCallback((roles: number) => {
        setTotals((current) => ({ ...current, roles }));
    }, []);

    const summaryItems = [
        { key: 'users', label: '总用户', value: totals.users, hint: '可登录身份' },
        { key: 'groups', label: '用户组', value: totals.groups, hint: '团队与职责集合' },
        { key: 'roles', label: '角色数', value: totals.roles, hint: '授权角色目录' },
    ] as const;

    return (
        <main className={style.authPage}>
            <ResourceHeader
                density="compact"
                placement="app-header"
                eyebrow="Access Control / Principals"
                title="身份主体"
                description="集中管理可登录或被授权的用户、用户组与角色，并从统一入口维护凭据和权限关系。"
            />

            <section className={style.principalSummary} aria-label="身份主体统计" aria-busy={totalsLoading}>
                {summaryItems.map((item) => (
                    <div className={style.principalSummaryItem} key={item.key}>
                        <span>{item.label}</span>
                        <strong>{item.value ?? '—'}</strong>
                        <small>{item.hint}</small>
                    </div>
                ))}
            </section>

            <section className={style.authWorkspace}>
                <p className={style.panelHint}>日常授权可直接使用内置角色：选择适合的角色，再管理其用户或用户组成员；无需先创建鉴权策略。 <Button variant="text" onClick={() => { const next = new URLSearchParams(searchParams); next.set('tab', '3'); setSearchParams(next); }}>查看角色与成员</Button></p>
                <Tabs
                    className={style.authTabs}
                    value={activeTab}
                    onChange={(value: string | number) => {
                        const next = new URLSearchParams(searchParams);
                        next.set('tab', String(value));
                        if (String(value) !== '3') {
                            next.delete('roleMembers');
                        }
                        setSearchParams(next);
                    }}
                >
                    <TabPanel label="用户" value="1">
                        <section className={style.authPanel}>
                            <p className={style.panelHint}>维护登录身份、用户类型与访问 Token，查看用户当前关联的角色和策略。</p>
                            <div className={style.panelBody}>
                                <UserTable onTotalChange={updateUsersTotal} onTotalsRefresh={loadTotals} />
                            </div>
                        </section>
                    </TabPanel>
                    <TabPanel label="用户组" value="2">
                        <section className={style.authPanel}>
                            <p className={style.panelHint}>按团队或职责聚合用户，通过用户组统一分配策略和管理访问 Token。</p>
                            <div className={style.panelBody}>
                                <GroupsTable onTotalChange={updateGroupsTotal} onTotalsRefresh={loadTotals} />
                            </div>
                        </section>
                    </TabPanel>
                    <TabPanel label="角色" value="3">
                        <section className={style.authPanel}>
                            <p className={style.panelHint}>可创建并维护自定义角色；管理员、资源全读和资源全写是内置角色，仅允许调整用户与用户组成员，角色定义和资源/API 权限固定。</p>
                            <div className={style.panelBody}>
                                <RoleTable onTotalChange={updateRolesTotal} onTotalsRefresh={loadTotals} />
                            </div>
                        </section>
                    </TabPanel>
                </Tabs>
            </section>
        </main>
    );
});
