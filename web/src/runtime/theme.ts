// 暗色主题的模块级状态源。
// rootContainer 位于所有 umi provider 之外，无法使用 useModel，
// 用可订阅的模块 store 让最外层 ConfigProvider 也能响应主题切换。
// 唯一写入方是 ThemeToggle，它与 initialState.settings.navTheme 同步更新。

const storageKey = 'echo-admin:nav-theme';

export type NavTheme = 'light' | 'realDark';

const listeners = new Set<() => void>();

let navTheme: NavTheme = readStoredTheme();

function readStoredTheme(): NavTheme {
  try {
    const stored = localStorage.getItem(storageKey);
    return stored === 'realDark' ? 'realDark' : 'light';
  } catch {
    return 'light';
  }
}

function persist(theme: NavTheme) {
  try {
    localStorage.setItem(storageKey, theme);
  } catch {
    // localStorage 不可用时主题仅存活于当前会话。
  }
}

export const themeStore = {
  get(): NavTheme {
    return navTheme;
  },
  set(next: NavTheme) {
    navTheme = next;
    persist(next);
    for (const listener of listeners) {
      listener();
    }
  },
  subscribe(listener: () => void): () => void {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  },
};
