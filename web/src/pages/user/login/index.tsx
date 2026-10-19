import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { LoginForm, ProFormText } from '@ant-design/pro-components';
import { Helmet, history, useModel } from '@umijs/max';
import { App } from 'antd';
import { createStyles } from 'antd-style';
import React from 'react';
import { flushSync } from 'react-dom';

import { Footer } from '@/components';
import { login } from '@/services/admin';
import Settings from '../../../../config/defaultSettings';

const useStyles = createStyles(() => ({
  container: {
    minHeight: '100dvh',
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) minmax(420px, 520px)',
    background: '#f5f7fa',
    color: '#1c2431',
    overflow: 'hidden',
    '@media (max-width: 920px)': {
      display: 'block',
      minHeight: '100dvh',
      overflow: 'auto',
    },
  },
  visual: {
    position: 'relative',
    minHeight: '100dvh',
    padding: 48,
    display: 'flex',
    flexDirection: 'column',
    justifyContent: 'space-between',
    backgroundImage: 'linear-gradient(160deg, #0d1f3c 0%, #14315e 100%)',
    '@media (max-width: 920px)': {
      display: 'none',
    },
  },
  visualBrand: {
    display: 'flex',
    alignItems: 'center',
    gap: 14,
    color: '#f7f9fc',
  },
  mark: {
    width: 44,
    height: 44,
    borderRadius: 8,
    display: 'grid',
    placeItems: 'center',
    background: 'rgba(255, 255, 255, 0.12)',
    border: '1px solid rgba(255, 255, 255, 0.28)',
    boxShadow: '0 18px 50px rgba(0, 0, 0, 0.24)',
    fontSize: 17,
    fontWeight: 700,
    letterSpacing: 0,
  },
  brandText: {
    display: 'flex',
    flexDirection: 'column',
    gap: 3,
  },
  brandName: {
    fontSize: 18,
    lineHeight: 1.2,
    fontWeight: 700,
  },
  brandMeta: {
    fontSize: 13,
    lineHeight: 1.35,
    color: 'rgba(247, 249, 252, 0.68)',
    '@media (max-width: 920px)': {
      color: '#6b7683',
    },
  },
  formSide: {
    position: 'relative',
    minHeight: '100dvh',
    padding: '40px 48px',
    display: 'flex',
    flexDirection: 'column',
    justifyContent: 'center',
    background:
      'linear-gradient(180deg, rgba(255,255,255,0.96) 0%, rgba(246,248,251,0.98) 100%)',
    '@media (max-width: 920px)': {
      minHeight: '100dvh',
      padding: '40px 24px 72px',
      justifyContent: 'center',
    },
    '@media (max-width: 520px)': {
      padding: '72px 18px 70px',
      justifyContent: 'flex-start',
    },
  },
  panel: {
    width: '100%',
    maxWidth: 400,
    margin: '0 auto',
    '@media (max-width: 920px)': {
      maxWidth: 420,
    },
  },
  formHeader: {
    marginBottom: 28,
  },
  formTitle: {
    margin: 0,
    fontSize: 30,
    lineHeight: 1.18,
    fontWeight: 760,
    letterSpacing: 0,
    color: '#1c2431',
    '@media (max-width: 920px)': {
      fontSize: 28,
    },
    '@media (max-width: 520px)': {
      fontSize: 26,
    },
  },
  formSubtitle: {
    margin: '10px 0 0',
    fontSize: 14,
    lineHeight: 1.7,
    color: '#6b7683',
  },
  loginForm: {
    '.ant-pro-form-login-container': {
      width: '100%',
      padding: 0,
    },
    '.ant-pro-form-login-main': {
      width: '100%',
      minWidth: 0,
    },
    '.ant-pro-form-login-header': {
      display: 'none',
    },
    '.ant-input-affix-wrapper': {
      minHeight: 46,
      borderRadius: 8,
      borderColor: '#d9e2ee',
      background: '#fff',
      boxShadow: 'none',
    },
    '.ant-input-affix-wrapper:hover': {
      borderColor: '#91caff',
    },
    '.ant-input-affix-wrapper-focused': {
      borderColor: '#1677ff',
      boxShadow: '0 0 0 3px rgba(22, 119, 255, 0.12)',
    },
    '.ant-input-prefix': {
      marginInlineEnd: 10,
      color: '#7a8494',
    },
    '.ant-btn-primary': {
      minHeight: 46,
      borderRadius: 8,
      background: '#1677ff',
      boxShadow: '0 14px 30px rgba(22, 119, 255, 0.22)',
      fontWeight: 650,
    },
    '.ant-btn-primary:not(:disabled):not(.ant-btn-disabled):hover': {
      background: '#3c89ff',
    },
  },
  mobileBrand: {
    display: 'none',
    alignItems: 'center',
    gap: 12,
    marginBottom: 32,
    color: '#1c2431',
    '@media (max-width: 920px)': {
      display: 'flex',
    },
  },
  mobileMark: {
    width: 40,
    height: 40,
    borderRadius: 8,
    display: 'grid',
    placeItems: 'center',
    background: '#1677ff',
    color: '#f7f9fc',
    fontSize: 16,
    fontWeight: 700,
    letterSpacing: 0,
  },
  footer: {
    position: 'absolute',
    right: 48,
    bottom: 22,
    left: 48,
    color: '#7a8494',
    '@media (max-width: 920px)': {
      right: 24,
      left: 24,
      bottom: 18,
    },
  },
}));

const loginPath = '/user/login';

const safeRedirect = (redirect: string | null, fallback: string): string => {
  if (!redirect?.startsWith('/') || redirect.startsWith('//')) return fallback;
  try {
    const parsed = new URL(redirect, window.location.origin);
    if (parsed.origin !== window.location.origin) return fallback;
    if (parsed.pathname === loginPath) return fallback;
    return `${parsed.pathname}${parsed.search}${parsed.hash}`;
  } catch {
    return fallback;
  }
};

const Login: React.FC = () => {
  const { styles } = useStyles();
  const { message } = App.useApp();
  const { setInitialState } = useModel('@@initialState');

  return (
    <div className={styles.container}>
      <Helmet>
        <title>{`登录 - ${Settings.title}`}</title>
      </Helmet>
      <section className={styles.visual} aria-hidden="true">
        <div className={styles.visualBrand}>
          <div className={styles.mark}>EA</div>
          <div className={styles.brandText}>
            <span className={styles.brandName}>Echo Admin</span>
            <span className={styles.brandMeta}>后台管理模板</span>
          </div>
        </div>
      </section>
      <main className={styles.formSide}>
        <div className={styles.panel}>
          <div className={styles.mobileBrand}>
            <div className={styles.mobileMark}>EA</div>
            <div className={styles.brandText}>
              <span className={styles.brandName}>Echo Admin</span>
              <span className={styles.brandMeta}>后台管理模板</span>
            </div>
          </div>
          <div className={styles.formHeader}>
            <h2 className={styles.formTitle}>管理员登录</h2>
            <p className={styles.formSubtitle}>请输入账号和密码。</p>
          </div>
          <LoginForm
            className={styles.loginForm}
            submitter={{
              searchConfig: {
                submitText: '登录',
              },
            }}
            onFinish={async (values) => {
              const result = await login({
                username: String(values.username ?? ''),
                password: String(values.password ?? ''),
              });
              const currentUser = result.user;
              flushSync(() => {
                setInitialState((state) => ({ ...state, currentUser }));
              });
              message.success('登录成功');
              const redirect = new URL(window.location.href).searchParams.get(
                'redirect',
              );
              const fallbackPath = currentUser?.default_path || '/dashboard';
              history.replace(
                safeRedirect(
                  redirect === loginPath ? null : redirect,
                  fallbackPath,
                ),
              );
            }}
          >
            <ProFormText
              name="username"
              fieldProps={{
                size: 'large',
                prefix: <UserOutlined />,
                autoComplete: 'username',
              }}
              placeholder="请输入用户名"
              rules={[{ required: true, message: '请输入用户名' }]}
            />
            <ProFormText.Password
              name="password"
              fieldProps={{
                size: 'large',
                prefix: <LockOutlined />,
                autoComplete: 'current-password',
              }}
              placeholder="请输入密码"
              rules={[{ required: true, message: '请输入密码' }]}
            />
          </LoginForm>
        </div>
        <div className={styles.footer}>
          <Footer />
        </div>
      </main>
    </div>
  );
};

export default Login;
