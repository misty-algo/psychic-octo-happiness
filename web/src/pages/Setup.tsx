import {
  HomeOutlined,
  IdcardOutlined,
  LockOutlined,
  MailOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { ProForm, ProFormText } from '@ant-design/pro-components';
import { Helmet, history } from '@umijs/max';
import { App } from 'antd';
import { createStyles } from 'antd-style';
import React from 'react';

import { Footer } from '@/components';
import { submitSetup } from '@/services/admin';
import type { SetupInput } from '@/services/admin';
import Settings from '../../config/defaultSettings';

const loginPath = '/user/login';

const useStyles = createStyles(() => ({
  container: {
    minHeight: '100dvh',
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) minmax(420px, 540px)',
    background: '#f5f7f6',
    color: '#18211f',
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
    backgroundImage:
      'linear-gradient(90deg, rgba(13, 29, 28, 0.88), rgba(13, 29, 28, 0.2) 58%, rgba(245, 247, 246, 0.12)), url("/images/login-operations.png")',
    backgroundSize: 'cover',
    backgroundPosition: 'center',
    '@media (max-width: 920px)': {
      display: 'none',
    },
  },
  visualBrand: {
    display: 'flex',
    alignItems: 'center',
    gap: 14,
    color: '#f7fbf8',
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
    color: 'rgba(247, 251, 248, 0.68)',
    '@media (max-width: 920px)': {
      color: '#66706c',
    },
  },
  visualCopy: {
    maxWidth: 560,
    color: '#f7fbf8',
  },
  visualTitle: {
    margin: 0,
    fontSize: 40,
    lineHeight: 1.12,
    fontWeight: 760,
    letterSpacing: 0,
  },
  visualLine: {
    width: 72,
    height: 3,
    marginTop: 22,
    borderRadius: 999,
    background: 'linear-gradient(90deg, #77e2d4 0%, #f3b56f 58%, #e87c68 100%)',
  },
  formSide: {
    position: 'relative',
    minHeight: '100dvh',
    padding: '36px 48px 76px',
    display: 'flex',
    flexDirection: 'column',
    justifyContent: 'center',
    background:
      'linear-gradient(180deg, rgba(255,255,255,0.96) 0%, rgba(246,248,247,0.98) 100%)',
    '@media (max-width: 920px)': {
      minHeight: '100dvh',
      padding: '40px 24px 76px',
      backgroundImage:
        'linear-gradient(180deg, rgba(246,248,247,0.9), rgba(246,248,247,0.98)), url("/images/login-operations.png")',
      backgroundSize: 'cover',
      backgroundPosition: 'center left',
    },
    '@media (max-width: 520px)': {
      padding: '56px 18px 72px',
      justifyContent: 'flex-start',
    },
  },
  panel: {
    width: '100%',
    maxWidth: 420,
    margin: '0 auto',
  },
  mobileBrand: {
    display: 'none',
    alignItems: 'center',
    gap: 12,
    marginBottom: 30,
    color: '#18211f',
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
    background: '#1d5c58',
    color: '#f7fbf8',
    fontSize: 16,
    fontWeight: 700,
    letterSpacing: 0,
  },
  header: {
    marginBottom: 26,
  },
  title: {
    margin: 0,
    fontSize: 30,
    lineHeight: 1.18,
    fontWeight: 760,
    letterSpacing: 0,
    color: '#18211f',
    '@media (max-width: 520px)': {
      fontSize: 26,
    },
  },
  subtitle: {
    margin: '10px 0 0',
    fontSize: 14,
    lineHeight: 1.7,
    color: '#66706c',
  },
  form: {
    '.ant-pro-form-group-container': {
      gap: 12,
    },
    '.ant-input-affix-wrapper': {
      minHeight: 46,
      borderRadius: 8,
      borderColor: '#d9e1de',
      background: '#fff',
      boxShadow: 'none',
    },
    '.ant-input-affix-wrapper:hover': {
      borderColor: '#7ecfc4',
    },
    '.ant-input-affix-wrapper-focused': {
      borderColor: '#34b7aa',
      boxShadow: '0 0 0 3px rgba(52, 183, 170, 0.14)',
    },
    '.ant-input-prefix': {
      marginInlineEnd: 10,
      color: '#7a8581',
    },
    '.ant-btn-primary': {
      minHeight: 46,
      borderRadius: 8,
      background: '#1d5c58',
      boxShadow: '0 14px 30px rgba(29, 92, 88, 0.22)',
      fontWeight: 650,
    },
    '.ant-btn-primary:not(:disabled):not(.ant-btn-disabled):hover': {
      background: '#174d49',
    },
  },
  footer: {
    position: 'absolute',
    right: 48,
    bottom: 22,
    left: 48,
    color: '#7a8581',
    '@media (max-width: 920px)': {
      right: 24,
      left: 24,
      bottom: 18,
    },
  },
}));

const Setup: React.FC = () => {
  const { styles } = useStyles();
  const { message } = App.useApp();

  return (
    <div className={styles.container}>
      <Helmet>
        <title>{`系统初始化 - ${Settings.title}`}</title>
      </Helmet>
      <section className={styles.visual} aria-hidden="true">
        <div className={styles.visualBrand}>
          <div className={styles.mark}>EA</div>
          <div className={styles.brandText}>
            <span className={styles.brandName}>Echo Admin</span>
            <span className={styles.brandMeta}>后台管理模板</span>
          </div>
        </div>
        <div className={styles.visualCopy}>
          <h1 className={styles.visualTitle}>系统初始化</h1>
          <div className={styles.visualLine} />
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
          <div className={styles.header}>
            <h2 className={styles.title}>创建首个管理员</h2>
            <p className={styles.subtitle}>该账号拥有最高权限。</p>
          </div>
          <ProForm<SetupInput>
            className={styles.form}
            initialValues={{ site_name: 'Echo Admin' }}
            submitter={{
              resetButtonProps: false,
              searchConfig: {
                submitText: '完成初始化',
              },
            }}
            onFinish={async (values) => {
              const result = await submitSetup({
                username: String(values.username ?? ''),
                display_name: String(values.display_name ?? ''),
                email: values.email ? String(values.email) : undefined,
                password: String(values.password ?? ''),
                site_name: values.site_name
                  ? String(values.site_name)
                  : undefined,
              });
              if (result.initialized) {
                message.success('初始化完成');
                history.replace(loginPath);
              }
            }}
          >
            <ProFormText
              name="site_name"
              fieldProps={{
                size: 'large',
                prefix: <HomeOutlined />,
                autoComplete: 'organization',
              }}
              placeholder="站点名称"
              rules={[{ max: 120, message: '站点名称不能超过 120 个字符' }]}
            />
            <ProFormText
              name="username"
              fieldProps={{
                size: 'large',
                prefix: <UserOutlined />,
                autoComplete: 'username',
              }}
              placeholder="用户名"
              rules={[
                { required: true, message: '请输入用户名' },
                { max: 64, message: '用户名不能超过 64 个字符' },
              ]}
            />
            <ProFormText
              name="display_name"
              fieldProps={{
                size: 'large',
                prefix: <IdcardOutlined />,
                autoComplete: 'name',
              }}
              placeholder="显示名称"
              rules={[
                { required: true, message: '请输入显示名称' },
                { max: 80, message: '显示名称不能超过 80 个字符' },
              ]}
            />
            <ProFormText
              name="email"
              fieldProps={{
                size: 'large',
                prefix: <MailOutlined />,
                autoComplete: 'email',
              }}
              placeholder="邮箱"
              rules={[
                { type: 'email', message: '请输入有效邮箱' },
                { max: 160, message: '邮箱不能超过 160 个字符' },
              ]}
            />
            <ProFormText.Password
              name="password"
              fieldProps={{
                size: 'large',
                prefix: <LockOutlined />,
                autoComplete: 'new-password',
              }}
              placeholder="密码"
              rules={[
                { required: true, message: '请输入密码' },
                { min: 8, message: '密码至少 8 个字符' },
                { max: 72, message: '密码不能超过 72 个字符' },
              ]}
            />
          </ProForm>
        </div>
        <div className={styles.footer}>
          <Footer />
        </div>
      </main>
    </div>
  );
};

export default Setup;
