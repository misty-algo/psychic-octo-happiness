import { BulbOutlined } from '@ant-design/icons';
import { useModel } from '@umijs/max';
import { Button, Tooltip } from 'antd';
import React from 'react';

import { themeStore } from '../runtime/theme';

/**
 * 头部的明暗主题切换按钮。
 * 同时更新两处状态：themeStore 驱动最外层 ConfigProvider 的 darkAlgorithm，
 * initialState.settings.navTheme 驱动 ProLayout 的侧边栏与头部配色。
 */
export const ThemeToggle: React.FC = () => {
  const { initialState, setInitialState } = useModel('@@initialState');
  const dark =
    (initialState?.settings as { navTheme?: string } | undefined)?.navTheme ===
    'realDark';

  return (
    <Tooltip title={dark ? '切换为亮色模式' : '切换为暗色模式'}>
      <Button
        type="text"
        aria-label="主题切换"
        onClick={() => {
          const next = dark ? 'light' : 'realDark';
          themeStore.set(next);
          setInitialState((state) => ({
            ...state,
            settings: {
              ...(state?.settings ?? {}),
              navTheme: next,
            },
          }));
        }}
      >
        <BulbOutlined />
      </Button>
    </Tooltip>
  );
};
