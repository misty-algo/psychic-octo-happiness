import type { Menu } from '@/services/admin';

/** 树形表格或 TreeSelect 使用的菜单节点。 */
export type MenuNode = Menu & { children: MenuNode[] };

/** 把后端的扁平菜单列表按 parent_id 组装成树。parent_id 为 0 的是顶级菜单。 */
export function buildMenuTree(menus: Menu[], parentID = 0): MenuNode[] {
  const childrenOf = (pid: number): MenuNode[] =>
    menus
      .filter((menu) => menu.parent_id === pid)
      .map((menu) => ({ ...menu, children: childrenOf(menu.id) }));
  return childrenOf(parentID);
}

export type MenuTreeNode = {
  title: string;
  value: number;
  children: MenuTreeNode[];
};

/** 组装 TreeSelect 需要的树节点；excludeID 用于编辑时排除自身，避免把自己选成上级。 */
export function toMenuTreeNodes(
  menus: Menu[],
  excludeID?: number,
): MenuTreeNode[] {
  const nodesOf = (pid: number): MenuTreeNode[] =>
    menus
      .filter(
        (menu) =>
          menu.parent_id === pid && menu.id !== excludeID && menu.id !== pid,
      )
      .map((menu) => ({
        title: menu.name,
        value: menu.id,
        children: nodesOf(menu.id),
      }));
  return [{ title: '顶级菜单', value: 0, children: nodesOf(0) }];
}

/**
 * 补齐选中菜单的所有祖先 id。后端按 menu_ids 集合渲染侧边栏，
 * 只授子菜单不授父菜单会导致父级丢失，提交前做闭包处理。
 */
export function withMenuAncestors(ids: number[], menus: Menu[]): number[] {
  const byID = new Map(menus.map((menu) => [menu.id, menu]));
  const result = new Set<number>();
  for (const id of ids) {
    let current = byID.get(id);
    while (current && !result.has(current.id)) {
      result.add(current.id);
      current =
        current.parent_id === 0 ? undefined : byID.get(current.parent_id);
    }
  }
  return [...result];
}

/** antd Tree 直接可用的树节点（title + key）。 */
export type AntdTreeNode = {
  title: string;
  key: number;
  children: AntdTreeNode[];
};

/** 组装 antd Tree 的 treeData，用于只读展示某个角色可见的菜单树。 */
export function toAntdTreeData(menus: Menu[]): AntdTreeNode[] {
  const map = (nodes: MenuNode[]): AntdTreeNode[] =>
    nodes.map((node) => ({
      title: node.name,
      key: node.id,
      children: map(node.children),
    }));
  return map(buildMenuTree(menus));
}
