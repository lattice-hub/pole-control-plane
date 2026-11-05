import React from 'react';
import { Button } from 'components/Fluent';
import style from './AppRouter.module.less';

interface Props { children: React.ReactNode; resetKey: string }
interface State { failed: boolean }

/** 路由资源加载失败停在可恢复界面；不自动刷新，避免静默丢弃其它工作区的未保存内容。 */
export default class RouteLoadBoundary extends React.Component<Props, State> {
  state: State = { failed: false };

  static getDerivedStateFromError(): State {
    return { failed: true };
  }

  componentDidCatch(error: Error) {
    console.error('Console route could not be loaded', error);
  }

  componentDidUpdate(previous: Props) {
    if (this.state.failed && previous.resetKey !== this.props.resetKey) {
      this.setState({ failed: false });
    }
  }

  render() {
    if (!this.state.failed) return this.props.children;
    return (
      <section className={style.loadRecovery} role="alert" aria-labelledby="route-load-title">
        <h1 id="route-load-title">页面暂时无法加载</h1>
        <p>页面资源可能已更新，或网络暂时不可用。恢复连接后，重新加载页面以获取当前版本。</p>
        <p>重新加载会清除当前页面未保存的内容；已保存的草稿和配置不受影响。</p>
        <Button theme="primary" onClick={() => window.location.reload()}>重新加载页面</Button>
        <a href="/namespace">返回环境列表</a>
      </section>
    );
  }
}
