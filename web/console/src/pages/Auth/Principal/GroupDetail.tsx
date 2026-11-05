import React from 'react';
import { Breadcrumb, Button, Empty, Link, Loading, Tag } from 'components/Fluent';
import { CopyIcon, RefreshIcon } from 'components/Fluent/icons';
import { useNavigate } from 'components/Router';

import { useAppDispatch } from 'modules/store';
import { enableUserGroupToken, resetUserGroupToken } from 'modules/user/groups';
import { describeUserGroupDetail, describeUserGroupToken, UserGroup } from 'services/user_group';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import { copyToClipboard } from 'utils/sys';
import PrincipalPolicyTable from './PrincipalPolicyTable';
import style from './index.module.less';

const { BreadcrumbItem } = Breadcrumb;
const formatValue = (value?: string) => value || '-';

const GroupDetailPage: React.FC = () => {
    const dispatch = useAppDispatch();
    const navigate = useNavigate();
    const searchParams = new URLSearchParams(window.location.search);
    const groupId = searchParams.get('id') || '';
    const groupName = searchParams.get('name') || '用户组详情';
    const [state, setState] = React.useState<{
        loading: boolean;
        group: UserGroup | null;
        fetchError: boolean;
    }>({ loading: false, group: null, fetchError: false });

    const loadGroup = React.useCallback(async () => {
        if (!groupId) return;
        setState(prev => ({ ...prev, loading: true, fetchError: false }));
        try {
            const [detail, token] = await Promise.all([
                describeUserGroupDetail({ id: groupId }),
                describeUserGroupToken({ id: groupId }),
            ]);
            if (!detail.userGroup) {
                setState({ loading: false, group: null, fetchError: true });
                return;
            }
            setState({
                loading: false,
                group: {
                    ...detail.userGroup,
                    auth_token: token.userGroup?.auth_token || detail.userGroup.auth_token,
                    token_enable: token.userGroup?.token_enable ?? detail.userGroup.token_enable,
                },
                fetchError: false,
            });
        } catch (error) {
            setState(prev => ({ ...prev, loading: false, fetchError: true }));
            openErrNotification('请求错误', `获取用户组详情失败, ${(error as Error).message}`);
        }
    }, [groupId]);

    React.useEffect(() => {
        loadGroup();
    }, [loadGroup]);

    const refreshAfterTokenChange = () => {
        setState(prev => ({ ...prev, loading: true }));
        window.setTimeout(loadGroup, 1000);
    };

    const handleResetToken = async () => {
        if (!groupId) return;
        const result = await dispatch(resetUserGroupToken({ id: groupId }));
        if (result.meta.requestStatus === 'rejected') {
            openErrNotification('请求错误', String(result.payload || '资源访问凭据重置失败'));
            return;
        }
        openInfoNotification('请求成功', '用户组访问凭据已重置');
        refreshAfterTokenChange();
    };

    const handleToggleToken = async () => {
        if (!groupId || !state.group) return;
        const enabled = Boolean(state.group.token_enable);
        const result = await dispatch(enableUserGroupToken({ id: groupId, token_enable: !enabled }));
        if (result.meta.requestStatus === 'rejected') {
            openErrNotification('请求错误', String(result.payload || `资源访问凭据${enabled ? '禁用' : '启用'}失败`));
            return;
        }
        openInfoNotification('请求成功', `用户组访问凭据已${enabled ? '禁用' : '启用'}`);
        setState(prev => prev.group ? ({
            ...prev,
            group: { ...prev.group, token_enable: !enabled },
        }) : prev);
        refreshAfterTokenChange();
    };

    const group = state.group;
    const metadata = Object.entries(group?.metadata || {});
    const members = group?.relation?.users || [];
    const initial = (group?.name || groupName || 'G').slice(0, 1).toUpperCase();

    return (
        <div className={style.userDetailPage}>
            <Breadcrumb className={style.detailBreadcrumb} maxItemWidth="240px">
                <BreadcrumbItem onClick={() => navigate('/auth/principals?tab=2')}>用户组</BreadcrumbItem>
                <BreadcrumbItem>{group?.name || groupName}</BreadcrumbItem>
            </Breadcrumb>

            <Loading indicator loading={state.loading} preventScrollThrough showOverlay>
                {!groupId || state.fetchError || !group ? (
                    <section className={style.userDetailEmpty}>
                        <Empty description={groupId ? '未找到用户组详情' : '缺少用户组标识，无法加载详情'} />
                    </section>
                ) : (
                    <main className={style.userDetailStack}>
                        <section className={style.userDetailSummary}>
                            <div className={style.userDetailAvatar}>{initial}</div>
                            <div className={style.userDetailMain}>
                                <div className={style.userDetailTitleLine}>
                                    <h2>{group.name}</h2>
                                    <Tag theme="primary" variant="light">用户组</Tag>
                                </div>
                                <p>{group.comment || '暂无备注'}</p>
                                <div className={style.userMetaGrid}>
                                    <div className={style.userMetaItem}>
                                        <span>用户组 ID</span>
                                        <strong className={style.userMono} title={group.id}>{formatValue(group.id)}</strong>
                                    </div>
                                    <div className={style.userMetaItem}>
                                        <span>来源</span>
                                        <strong>{formatValue(group.source)}</strong>
                                    </div>
                                    <div className={style.userMetaItem}>
                                        <span>成员数量</span>
                                        <strong>{group.user_count ?? members.length} 个用户</strong>
                                    </div>
                                    <div className={style.userMetaItem}>
                                        <span>创建时间</span>
                                        <strong>{formatValue(group.ctime)}</strong>
                                    </div>
                                    <div className={style.userMetaItem}>
                                        <span>修改时间</span>
                                        <strong>{formatValue(group.mtime)}</strong>
                                    </div>
                                    <div className={style.userMetaItem}>
                                        <span>凭据状态</span>
                                        <strong>{group.token_enable ? '已启用' : '已禁用'}</strong>
                                    </div>
                                </div>
                            </div>
                        </section>

                        <div className={style.userDetailContentGrid}>
                            <section className={style.userCredentialPanel}>
                                <div className={style.userPanelHeader}>
                                    <div>
                                        <strong>访问凭据</strong>
                                        <span>用于以当前用户组身份访问 Console API。</span>
                                    </div>
                                    <Tag theme={group.token_enable ? 'success' : 'danger'} variant="light">
                                        {group.token_enable ? '已启用' : '已禁用'}
                                    </Tag>
                                </div>
                                <div className={style.userTokenRow}>
                                    <div>
                                        <span>访问 Token</span>
                                        <code>{group.auth_token ? '••••••••••••••••' : '暂无 Token'}</code>
                                    </div>
                                    <div className={style.userTokenActions}>
                                        <Button
                                            variant="outline"
                                            icon={<CopyIcon />}
                                            disabled={!group.auth_token}
                                            onClick={() => copyToClipboard(group.auth_token, '用户组访问凭据已复制到剪贴板')}
                                        >
                                            复制
                                        </Button>
                                        <Button variant="outline" icon={<RefreshIcon />} onClick={handleResetToken}>重置</Button>
                                        <Button theme={group.token_enable ? 'danger' : 'primary'} variant="outline" onClick={handleToggleToken}>
                                            {group.token_enable ? '禁用' : '启用'}
                                        </Button>
                                    </div>
                                </div>
                            </section>

                            <section className={style.userTagPanel}>
                                <div className={style.userPanelHeader}>
                                    <div>
                                        <strong>用户组标签</strong>
                                        <span>用于主体检索和权限策略匹配。</span>
                                    </div>
                                </div>
                                {metadata.length > 0 ? (
                                    <div className={style.userLabelList}>
                                        {metadata.map(([key, value]) => <Tag key={key} variant="outline">{key}: {value}</Tag>)}
                                    </div>
                                ) : <p className={style.userPanelEmpty}>暂无标签</p>}
                            </section>
                        </div>

                        <section className={style.userPolicySurface}>
                            <div className={style.userTableHeader}>
                                <div>
                                    <strong>成员用户</strong>
                                    <span>当前用户组关联 {members.length} 个用户</span>
                                </div>
                            </div>
                            {members.length > 0 ? (
                                <div className={style.userLabelList} style={{ padding: '0 16px 16px' }}>
                                    {members.map(member => (
                                        <Link
                                            key={member.id}
                                            theme="primary"
                                            onClick={() => navigate(`/auth/principals/userdetail?name=${encodeURIComponent(member.name || member.id)}&id=${encodeURIComponent(member.id)}`)}
                                        >
                                            {member.name || member.id}
                                        </Link>
                                    ))}
                                </div>
                            ) : <p className={style.userPanelEmpty} style={{ padding: '0 16px 16px' }}>暂无成员用户</p>}
                        </section>

                        <section className={style.userPolicySurface}>
                            <div className={style.userTableHeader}>
                                <div>
                                    <strong>关联策略</strong>
                                    <span>直接关联到当前用户组的访问策略</span>
                                </div>
                            </div>
                            <PrincipalPolicyTable principalId={group.id} principalType={2} />
                        </section>
                    </main>
                )}
            </Loading>
        </div>
    );
};

export default React.memo(GroupDetailPage);
