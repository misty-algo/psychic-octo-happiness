import type { ProLayoutProps } from '@ant-design/pro-components';

/**
 * @name
 */
const Settings: ProLayoutProps & {
  logo?: string;
} = {
  navTheme: 'light',
  // 花小猪打车品牌色（橙黄）。
  colorPrimary: '#ff7d41',
  layout: 'mix',
  contentWidth: 'Fluid',
  fixedHeader: true,
  fixSiderbar: true,
  colorWeak: false,
  title: '花小猪打车',
  logo: '/logo.svg',
  iconfontUrl: '',
  token: {
    // ProLayout 的菜单选中色不会自动跟随 antd colorPrimary，需显式指定为品牌蓝。
    sider: {
      colorTextMenuSelected: '#ff7d41',
      colorTextMenuActive: 'rgba(255, 125, 65, 0.92)',
      colorBgMenuItemSelected: 'rgba(255, 125, 65, 0.12)',
    },
  },
};

export default Settings;
