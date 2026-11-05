import { lazy } from 'react';
import { QueueIcon } from 'components/Fluent/icons';
import { IRouter } from '../index';

const UnifiedConfiguration = lazy(() => import('pages/Configuration/Unified'));
const configuration: IRouter[] = [
  // 旧深链保留为独立精确路由；菜单分类本身只展开子路由。
  { path: '/configuration', Component: UnifiedConfiguration, meta: { hidden: true } },
  {
    path: '/configuration',
    meta: { title: 'menu.configuration', Icon: QueueIcon },
    children: [{ path: 'files', Component: UnifiedConfiguration, meta: { title: 'menu.configurationFiles' } }],
  },
];

export default configuration;
