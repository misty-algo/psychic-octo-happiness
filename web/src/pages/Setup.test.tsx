import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const { mockMessageSuccess, mockReplace, mockSubmitSetup } = vi.hoisted(() => ({
  mockMessageSuccess: vi.fn(),
  mockReplace: vi.fn(),
  mockSubmitSetup: vi.fn(),
}));

vi.mock('@ant-design/icons', () => ({
  HomeOutlined: () => null,
  IdcardOutlined: () => null,
  LockOutlined: () => null,
  MailOutlined: () => null,
  UserOutlined: () => null,
}));

vi.mock('@ant-design/pro-components', () => ({
  ProForm: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  ProFormText: Object.assign(() => null, { Password: () => null }),
}));

vi.mock('@umijs/max', () => ({
  Helmet: ({ children }: { children?: React.ReactNode }) => children,
  history: {
    replace: mockReplace,
  },
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
      visual: 'visual',
      visualBrand: 'visualBrand',
      mark: 'mark',
      brandText: 'brandText',
      brandName: 'brandName',
      brandMeta: 'brandMeta',
      visualCopy: 'visualCopy',
      visualTitle: 'visualTitle',
      visualLine: 'visualLine',
      formSide: 'formSide',
      panel: 'panel',
      mobileBrand: 'mobileBrand',
      mobileMark: 'mobileMark',
      header: 'header',
      title: 'title',
      subtitle: 'subtitle',
      form: 'form',
      footer: 'footer',
    },
  }),
}));

vi.mock('@/components', () => ({
  Footer: () => null,
}));

vi.mock('@/services/admin', () => ({
  submitSetup: mockSubmitSetup,
}));

vi.mock('../../config/defaultSettings', () => ({
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

describe('Setup page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('submits setup input and returns to login after initialization completes', async () => {
    const { default: Setup } = await import('./Setup');
    mockSubmitSetup.mockResolvedValue({ initialized: true });

    const page = await Promise.resolve(Setup({}));
    const form = findElementWithProp(page, 'onFinish');
    if (!form) {
      throw new Error('ProForm with onFinish was not rendered');
    }
    const onFinish = form.props.onFinish;
    if (typeof onFinish !== 'function') {
      throw new TypeError('ProForm onFinish is not a function');
    }

    await onFinish({
      username: 'root',
      display_name: 'Root Admin',
      email: 'root@example.com',
      password: 'secret-password',
      site_name: 'Control',
    });

    expect(mockSubmitSetup).toHaveBeenCalledWith({
      username: 'root',
      display_name: 'Root Admin',
      email: 'root@example.com',
      password: 'secret-password',
      site_name: 'Control',
    });
    expect(mockMessageSuccess).toHaveBeenCalledWith('初始化完成');
    expect(mockReplace).toHaveBeenCalledWith('/user/login');
  });
});
