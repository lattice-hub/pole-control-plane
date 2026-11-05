import { lazy } from 'react';
import { ComponentSpaceIcon } from 'components/Fluent/icons';
import { IRouter } from '../index';

const namespace: IRouter[] = [
  {
    path: '/namespace',
    group: 'menu.namespace',
    meta: {
      title: 'menu.namespace',
      Icon: ComponentSpaceIcon,
    },
    Component: lazy(() => import('pages/Namespace'))
  },
  {
    path: '/namespace/promotion-topology',
    Component: lazy(() => import('pages/Namespace/PromotionTopology')),
    meta: { title: '环境晋升拓扑', hidden: true },
  },
  {
    path: '/namespace/environment',
    Component: lazy(() => import('pages/Namespace/EnvironmentResources')),
    meta: {
      title: 'menu.namespace',
      hidden: true,
    },
  },
];

export default namespace;
