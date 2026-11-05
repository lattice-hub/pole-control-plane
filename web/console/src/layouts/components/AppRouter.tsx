import React, { Suspense, memo } from 'react';
import { Routes, Route, Navigate, useLocation } from 'components/Router';
import { Layout, Loading } from 'components/Fluent';
import routers, { IRouter } from 'router';
import { useAppSelector } from 'modules/store';
import { resolve } from 'utils/path';
import Page from './Page';
import Style from './AppRouter.module.less';
import RouteLoadBoundary from './RouteLoadBoundary';

const { Content } = Layout;

type TRenderRoutes = (routes: IRouter[], parentPath?: string, breadcrumbs?: string[]) => React.ReactNode[];

const PrivateRoute = ({ children }: { children: React.ReactNode }) => {
  const { isLogin, sessionResolved } = useAppSelector((state) => state.userLogin);
  const location = useLocation();

  if (!sessionResolved) {
    return <Loading />;
  }

  if (!isLogin) {
    return <Navigate to="/login" replace state={{ from: location }} />;
  }

  return children;
};

const AdminRoute = ({ children }: { children: React.ReactNode }) => {
  const role = useAppSelector((state) => state.userLogin.currentUser.role);
  if (role !== 'main' && role !== 'admin') {
    return <Navigate to="/namespace" replace />;
  }
  return children;
};

/**
 * 渲染应用路由
 * @param routes
 * @param parentPath
 * @param breadcrumb
 */
const renderRoutes: TRenderRoutes = (routes, parentPath = '', breadcrumb = []) =>
  routes.map((route, index: number) => {
    const { Component, children, redirect, meta } = route;
    const currentPath = resolve(parentPath, route.path);
    let currentBreadcrumb = breadcrumb;

    if (redirect) {
      // 重定向
      return <Route key={index} path={currentPath} element={<Navigate to={redirect} replace />} />;
    }

    if (Component) {
      if (currentPath === '/login' || currentPath === '/init-admin') {
        // 登录页、初始化 admin 页不需要权限
        return (
          <Route
            key={index}
            path={currentPath}
            element={
              <Page isFullPage={route.isFullPage} breadcrumbs={currentBreadcrumb}>
                <Component />
              </Page>
            }
          />
        );
      } else {
        // 有路由菜单
        return (
          <Route
            key={index}
            path={currentPath}
            element={
              <PrivateRoute>
                {meta?.adminOnly ? (
                  <AdminRoute>
                    <Page isFullPage={route.isFullPage} breadcrumbs={currentBreadcrumb}>
                      <Component />
                    </Page>
                  </AdminRoute>
                ) : (
                  <Page isFullPage={route.isFullPage} breadcrumbs={currentBreadcrumb}>
                    <Component />
                  </Page>
                )}
              </PrivateRoute>
            }
          />
        );
      }
    }
    // 无路由菜单
    return children ? renderRoutes(children, currentPath, currentBreadcrumb) : null;
  });

const AppRouter = () => {
  const location = useLocation();
  return (
  <Content>
    <RouteLoadBoundary resetKey={location.pathname}>
    <Suspense
      fallback={
        <div className={Style.loading}>
          <Loading />
        </div>
      }
    >

      <Routes>{renderRoutes(routers)}</Routes>
    </Suspense>
    </RouteLoadBoundary>
  </Content>
  );
};

export default memo(AppRouter);
