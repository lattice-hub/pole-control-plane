import React from 'react';
import { Button, Tag } from 'components/Fluent';

import { describeQuotaLeases, QuotaLeaseSnapshot, QuotaLeaseView } from 'services/ratelimit';

import styles from './QuotaLeaseRuntime.module.less';

interface QuotaLeaseRuntimeProps {
    namespace: string
    service: string
}

const statusMeta: Record<string, { label: string; theme: 'success' | 'warning' | 'default' }> = {
    ACTIVE: { label: '使用中', theme: 'success' },
    SETTLED: { label: '已结算', theme: 'default' },
    EXPIRED: { label: 'TTL 回收', theme: 'warning' },
};

const formatNumber = (value?: number) => new Intl.NumberFormat('zh-CN').format(value || 0);

const formatDuration = (durationMs: number) => {
    if (durationMs % 60000 === 0) return `${durationMs / 60000} 分钟`;
    if (durationMs % 1000 === 0) return `${durationMs / 1000} 秒`;
    return `${durationMs} ms`;
};

const leaseTime = (lease: QuotaLeaseView) => {
    if (lease.status === 'ACTIVE') {
        const seconds = Math.max(0, Math.ceil((lease.expiresAt - Date.now()) / 1000));
        return `TTL 剩余 ${seconds}s`;
    }
    return lease.terminalAt ? new Date(lease.terminalAt).toLocaleTimeString() : '-';
};

const QuotaLeaseRuntime: React.FC<QuotaLeaseRuntimeProps> = ({ namespace, service }) => {
    const [snapshot, setSnapshot] = React.useState<QuotaLeaseSnapshot | null>(null);
    const [loading, setLoading] = React.useState(false);
    const [error, setError] = React.useState('');

    const load = React.useCallback(async () => {
        if (!namespace || !service) return;
        setLoading(true);
        try {
            setSnapshot(await describeQuotaLeases({ namespace, service }));
            setError('');
        } catch (cause) {
            setError(cause instanceof Error ? cause.message : '无法读取 Limiter 运行态');
        } finally {
            setLoading(false);
        }
    }, [namespace, service]);

    React.useEffect(() => {
        load();
        const timer = window.setInterval(load, 5000);
        return () => window.clearInterval(timer);
    }, [load]);

    const summary = snapshot?.summary;
    return (
        <section className={styles.runtimeSection}>
            <div className={styles.header}>
                <div>
                    <div className={styles.title}>Quota Lease 运行态</div>
                    <div className={styles.subtitle}>当前服务的预留、流中消费、结算归还与 TTL 回收，最近记录保留 5 分钟。</div>
                </div>
                <Button variant="outline" loading={loading} onClick={load}>刷新</Button>
            </div>
            {error ? (
                <div className={styles.unavailable}>
                    <strong>Limiter 运行态暂不可用</strong>
                    <span>{error}。请确认以 full 模式启动，并配置 Console 的 limiterServer。</span>
                </div>
            ) : (
                <>
                    <div className={styles.summaryGrid}>
                        <div className={styles.summaryItem}><span>活跃租约</span><strong>{summary?.activeCount || 0}</strong><small>并发中的模型请求</small></div>
                        <div className={styles.summaryItem}><span>当前预留</span><strong>{formatNumber(summary?.activeReserved)}</strong><small>尚未转为实际消费</small></div>
                        <div className={styles.summaryItem}><span>流中已使用</span><strong>{formatNumber(summary?.activeConsumed)}</strong><small>活跃租约累计用量</small></div>
                        <div className={styles.summaryItem}><span>最近归还</span><strong>{formatNumber(summary?.recentlyReturned)}</strong><small>结算或 TTL 回收</small></div>
                    </div>
                    <div className={styles.flowHint}>
                        <span>请求前预留</span><i />
                        <span>流中累计更新</span><i />
                        <span>结束结算归还</span>
                        <em>预留成功后，Limiter 不会在业务流中再次触发配额 429。</em>
                    </div>
                    <div className={styles.leaseList}>
                        {(snapshot?.leases || []).slice(0, 10).map((lease) => {
                            const meta = statusMeta[lease.status] || { label: lease.status, theme: 'default' as const };
                            return (
                                <article className={styles.leaseCard} key={lease.id}>
                                    <div className={styles.leaseHead}>
                                        <div><code>{lease.id.slice(0, 8)}</code><span>客户端 {lease.clientKey}</span></div>
                                        <div><Tag theme={meta.theme} variant="light">{meta.label}</Tag><span>{leaseTime(lease)}</span></div>
                                    </div>
                                    {lease.counters.map((counter) => {
                                        const allocated = counter.reserved + counter.consumed;
                                        const ratio = counter.limit > 0 ? Math.min(100, allocated / counter.limit * 100) : 0;
                                        return (
                                            <div className={styles.counterRow} key={counter.counterKey}>
                                                <div className={styles.counterMeta}>
                                                    <strong>{counter.accounting === 'OCCUPANCY' ? '占用型配额' : '消耗型配额'} · {formatDuration(counter.durationMs)}</strong>
                                                    <span>上限 {formatNumber(counter.limit)} · 可用 {formatNumber(counter.available)}</span>
                                                </div>
                                                <div className={styles.meter}><span style={{ width: `${ratio}%` }} /></div>
                                                <div className={styles.counterValues}>
                                                    <span>预留 {formatNumber(counter.reserved)}</span>
                                                    <span>使用 {formatNumber(counter.consumed)}</span>
                                                    <span>归还 {formatNumber(counter.returned)}</span>
                                                </div>
                                            </div>
                                        );
                                    })}
                                </article>
                            );
                        })}
                        {!loading && snapshot && snapshot.leases.length === 0 && (
                            <div className={styles.empty}>暂无租约。SDK 发起一次受控模型请求后，这里会展示预留和结算过程。</div>
                        )}
                    </div>
                </>
            )}
        </section>
    );
};

export default QuotaLeaseRuntime;
