import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const {
  mockLogin,
  mockMessageSuccess,
  mockReplace,
  mockSetInitialState,
  mockFetchUserInfo,
} = vi.hoisted(() => ({
  mockLogin: vi.fn(),
  mockMessageSuccess: vi.fn(),
  mockReplace: vi.fn(),
  mockSetInitialState: vi.fn(),
  mockFetchUserInfo: vi.fn(),
}));

vi.mock('@ant-design/icons', () => ({
  LockOutlined: () => null,
  UserOutlined: () => null,
}));

vi.mock('@ant-design/pro-components', () => ({
  LoginForm: () => null,
  ProFormText: Object.assign(() => null, { Password: () => null }),
}));

vi.mock('@umijs/max', () => ({
  Helmet: ({ children }: { children?: React.ReactNode }) => children,
  history: {
    replace: mockReplace,
  },
  useModel: () => ({
    initialState: {
      fetchUserInfo: mockFetchUserInfo,
    },
    setInitialState: mockSetInitialState,
  }),
}));

vi.mock('antd', () => ({
  App: {
    useApp: () => ({
      message: {
        success: mockMessageSuccess,
      },
    }),
  },
}));

vi.mock('antd-style', () => ({
  createStyles: () => () => ({
    styles: {
      container: 'container',
      main: 'main',
      panel: 'panel',
    },
  }),
}));

vi.mock('@/components', () => ({
  Footer: () => null,
}));

vi.mock('@/services/admin', () => ({
  login: mockLogin,
}));

vi.mock('../../../../config/defaultSettings', () => ({
  default: { title: 'Echo Admin' },
}));

function findElementWithProp(
  node: React.ReactNode,
  propName: string,
): React.ReactElement<Record<string, unknown>> | undefined {
  if (!React.isValidElement(node)) {
    return undefined;
  }
  const props = node.props as { children?: React.ReactNode } & Record<
    string,
    unknown
  >;
  if (propName in props) {
    return node as React.ReactElement<Record<string, unknown>>;
  }
  const children = React.Children.toArray(props.children);
  for (const child of children) {
    const match = findElementWithProp(child, propName);
    if (match) {
      return match;
    }
  }
  return undefined;
}

describe('Login page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.history.replaceState({}, '', '/user/login?redirect=/roles');
    mockSetInitialState.mockImplementation((updater) => {
      if (typeof updater === 'function') {
        updater({});
      }
    });
  });

  it('commits the logged-in user before navigating away from login', async () => {
    const { default: Login } = await import('./index');
    const user = {
      id: 1,
      username: 'admin',
      display_name: '系统管理员',
      email: '',
      active_role_id: 1,
      active_role: {},
      default_path: '/dashboard',
      roles: [],
      permissions: [],
      menus: [],
    };
    mockLogin.mockResolvedValue({ user });

    const page = await Promise.resolve(Login({}));
    const loginForm = findElementWithProp(page, 'onFinish');
    if (!loginForm) {
      throw new Error('LoginForm with onFinish was not rendered');
    }
    const onFinish = loginForm.props.onFinish;
    if (typeof onFinish !== 'function') {
      throw new TypeError('LoginForm onFinish is not a function');
    }

    await onFinish({
      username: 'admin',
      password: 'secret-password',
    });

    expect(mockLogin).toHaveBeenCalledWith({
      username: 'admin',
      password: 'secret-password',
    });
    expect(mockFetchUserInfo).not.toHaveBeenCalled();
    expect(mockSetInitialState.mock.invocationCallOrder[0]).toBeLessThan(
      mockReplace.mock.invocationCallOrder[0],
    );
    expect(mockMessageSuccess).toHaveBeenCalledWith('登录成功');
    expect(mockReplace).toHaveBeenCalledWith('/roles');
  });

  it('falls back to the default path when redirect points back to login', async () => {
    const { default: Login } = await import('./index');
    window.history.replaceState(
      {},
      '',
      `/user/login?redirect=${encodeURIComponent('/user/login?redirect=%2Fdashboard')}`,
    );
    const user = {
      id: 1,
      username: 'admin',
      display_name: '系统管理员',
      email: '',
      active_role_id: 1,
      active_role: {},
      default_path: '/dashboard',
      roles: [],
      permissions: [],
      menus: [],
    };
    mockLogin.mockResolvedValue({ user });

    const page = await Promise.resolve(Login({}));
    const loginForm = findElementWithProp(page, 'onFinish');
    if (!loginForm) {
      throw new Error('LoginForm with onFinish was not rendered');
    }
    const onFinish = loginForm.props.onFinish;
    if (typeof onFinish !== 'function') {
      throw new TypeError('LoginForm onFinish is not a function');
    }

    await onFinish({
      username: 'admin',
      password: 'secret-password',
    });

    expect(mockReplace).toHaveBeenCalledWith('/dashboard');
  });
});
