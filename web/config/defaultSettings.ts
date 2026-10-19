import type { ProLayoutProps } from '@ant-design/pro-components';

/**
 * @name
 */
const Settings: ProLayoutProps & {
  logo?: string;
} = {
  navTheme: 'light',
  colorPrimary: '#1677ff',
  layout: 'mix',
  contentWidth: 'Fluid',
  fixedHeader: true,
  fixSiderbar: true,
  colorWeak: false,
  title: 'Echo Admin',
  logo: '/logo.svg',
  iconfontUrl: '',
  token: {
    // ProLayout 的菜单选中色不会自动跟随 antd colorPrimary，需显式指定为品牌蓝。
    sider: {
      colorTextMenuSelected: '#1677ff',
      colorTextMenuActive: 'rgba(22, 119, 255, 0.92)',
      colorBgMenuItemSelected: 'rgba(22, 119, 255, 0.09)',
    },
  },
};

export default Settings;
