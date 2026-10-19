import type { APIResource, Menu } from '@/services/admin';

import { withMenuAncestors } from './menu-tree';

// 工作台是无 permission token 的默认入口（domain.DefaultRolePath），
// 所有角色都应可见，派生时显式带上。
const defaultEntryPath = '/dashboard';

/**
 * 由功能权限 token 派生可见菜单：
 * 授权叶子菜单（permission ∈ tokens）+ 祖先容器 + 默认入口。
 */
export function deriveMenuIDs(tokens: string[], menus: Menu[]): number[] {
  const tokenSet = new Set(tokens);
  const granted = menus
    .filter(
      (menu) => menu.permission !== '' && tokenSet.has(menu.permission),
    )
    .map((menu) => menu.id);
  const entry = menus.find((menu) => menu.path === defaultEntryPath);
  if (entry) {
    granted.push(entry.id);
  }
  return withMenuAncestors(granted, menus);
}

/**
 * 由功能权限 token 派生受管 API 授权：
 * - 绑定了 token 的路由随勾选放行；
 * - permission 为空的自助/会话路由（当前用户、登出、改密、切角色、上传文件访问）
 *   不属于任何功能，但同样走 HasAPI 校验，必须对所有角色默认授权，
 *   否则保存后该角色连 /api/auth/me 都无法访问。
 */
export function deriveAPIIDs(tokens: string[], apis: APIResource[]): number[] {
  const tokenSet = new Set(tokens);
  return apis
    .filter(
      (api) =>
        api.permission === '' || tokenSet.has(api.permission),
    )
    .map((api) => api.id);
}

/**
 * 由派生菜单得到按钮授权：菜单可见即授予其全部按钮。
 * 按钮的真实控制面是 token（前端 access）与 API 放行，这里保持投影完整。
 */
export function deriveButtonIDs(menuIDs: number[], menus: Menu[]): number[] {
  const menuIDSet = new Set(menuIDs);
  return menus
    .filter((menu) => menuIDSet.has(menu.id))
    .flatMap((menu) => menu.buttons.map((button) => button.id));
}
