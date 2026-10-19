import type { Settings as LayoutSettings } from '@ant-design/pro-components';
import type { RequestConfig, RunTimeLayoutConfig } from '@umijs/max';
import { history, Link } from '@umijs/max';
import { theme as antdTheme, ConfigProvider } from 'antd';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import React from 'react';

// Initialize dayjs plugins globally
dayjs.extend(relativeTime);

import {
  AvatarDropdown,
  ErrorBoundary,
  Footer,
  LangDropdown,
  OfflineBanner,
  ThemeToggle,
} from '@/components';
import type { CurrentUser, SetupState } from '@/services/admin';
import {
  currentUser as queryCurrentUser,
  setupState as querySetupState,
} from '@/services/admin';
import defaultSettings from '../config/defaultSettings';
import { errorConfig } from './requestErrorConfig';
import { filterMenuDataByGrantedMenus } from './runtime/menu';
import { themeStore } from './runtime/theme';

const loginPath = '/user/login';
const setupPath = '/setup';
const systemUninitializedCode = 100410;

type RuntimeRequestError = Error & {
  info?: { code?: number };
};

/**
 * @see https://umijs.org/docs/api/runtime-config#getinitialstate
 * */
export async function getInitialState(): Promise<{
  settings?: Partial<LayoutSettings>;
  currentUser?: CurrentUser;
  setupState?: SetupState;
  loading?: boolean;
  fetchUserInfo?: () => Promise<CurrentUser | undefined>;
}> {
  const fetchUserInfo = async () => {
    try {
      const msg = await queryCurrentUser();
      return msg;
    } catch (error) {
      if (
        (error as RuntimeRequestError).info?.code === systemUninitializedCode
      ) {
        history.replace(setupPath);
        return undefined;
      }
      const { pathname, search, hash } = history.location;
      if (pathname === loginPath || pathname === setupPath) {
        return undefined;
      }
      history.replace(
        `${loginPath}?redirect=${encodeURIComponent(pathname + search + hash)}`,
      );
    }
    return undefined;
  };

  const { location } = history;
  const installation = await querySetupState();
  // 暗色偏好持久化在 localStorage，与 themeStore 使用同一存储键。
  const settings: Partial<LayoutSettings> = {
    ...(defaultSettings as Partial<LayoutSettings>),
    navTheme: themeStore.get(),
  };
  if (!installation.initialized) {
    if (location.pathname !== setupPath) {
      history.replace(setupPath);
    }
    return {
      fetchUserInfo,
      setupState: installation,
      settings,
    };
  }

  if (location.pathname === setupPath) {
    history.replace(loginPath);
  }

  // 如果不是公开页面，执行当前用户加载。
  if (location.pathname !== loginPath && location.pathname !== setupPath) {
    const currentUser = await fetchUserInfo();
    return {
      fetchUserInfo,
      currentUser,
      setupState: installation,
      settings,
    };
  }
  return {
    fetchUserInfo,
    setupState: installation,
    settings: defaultSettings as Partial<LayoutSettings>,
  };
}

// ProLayout 支持的api https://procomponents.ant.design/components/layout
export const layout: RunTimeLayoutConfig = ({ initialState }) => {
  return {
    menuItemRender: (item, dom) => {
      if (item.path) {
        return (
          <Link to={item.path} prefetch>
            {dom}
          </Link>
        );
      }
      return dom;
    },
    menuDataRender: (menuData) =>
      filterMenuDataByGrantedMenus(
        menuData,
        initialState?.currentUser?.menus ?? [],
      ) as typeof menuData,
    actionsRender: () => {
      // `locale: false` opts out of the language switcher. ProLayout's own
      // `locale` prop is a locale string, so narrow to the boolean toggle here.
      const localeEnabled =
        (initialState?.settings as { locale?: boolean })?.locale !== false;
      return [
        <ThemeToggle key="theme" />,
        localeEnabled && <LangDropdown key="lang" />,
      ].filter(Boolean);
    },
    avatarProps: {
      title: initialState?.currentUser?.display_name ?? 'Admin',
      render: (_, avatarChildren) => (
        <AvatarDropdown>{avatarChildren}</AvatarDropdown>
      ),
    },
    // waterMarkProps: {
    //   content: initialState?.currentUser?.name,
    // },
    footerRender: () => <Footer />,
    onPageChange: () => {
      const { location } = history;
      // 如果没有登录，重定向到 login
      if (
        !initialState?.currentUser &&
        location.pathname !== loginPath &&
        location.pathname !== setupPath
      ) {
        history.replace(
          `${loginPath}?redirect=${encodeURIComponent(location.pathname + location.search + location.hash)}`,
        );
      }
    },
    links: [],
    // Replace ProLayout's default ErrorBoundary with our offline-aware version,
    // so chunk load errors show friendly messages instead of "Something went wrong."
    ErrorBoundary,
    menuHeaderRender: undefined,
    // 自定义 403 页面
    // unAccessible: <div>unAccessible</div>,
    // 增加一个 loading 的状态
    childrenRender: (children) => {
      return children;
    },
    ...initialState?.settings,
  };
};

/**
 * @name request 配置，可以配置错误处理
 * 它基于 axios 提供了一套统一的网络请求和错误处理方案。
 * @doc https://umijs.org/docs/max/request#配置
 */
export const request: RequestConfig = {
  baseURL: '',
  ...errorConfig,
};

/**
 * 最外层主题包装：navTheme 为 realDark 时整个组件树（含 Modal/Drawer 等传送门
 * 和头部操作按钮）都切换到 antd 暗色算法；ProLayout 自身的侧边栏与头部配色
 * 由 initialState.settings.navTheme 驱动，两处由 ThemeToggle 同步更新。
 */
const RootTheme: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const navTheme = React.useSyncExternalStore(
    themeStore.subscribe,
    themeStore.get,
    () => 'light' as const,
  );
  React.useEffect(() => {
    document.body.dataset.theme = navTheme === 'realDark' ? 'dark' : 'light';
  }, [navTheme]);
  return (
    <ConfigProvider
      theme={{
        algorithm:
          navTheme === 'realDark'
            ? antdTheme.darkAlgorithm
            : antdTheme.defaultAlgorithm,
        token: { fontFamily: 'AlibabaSans, sans-serif' },
      }}
    >
      {children}
    </ConfigProvider>
  );
};

export function rootContainer(container: React.ReactNode) {
  return (
    <RootTheme>
      <OfflineBanner />
      <ErrorBoundary>{container}</ErrorBoundary>
    </RootTheme>
  );
}
