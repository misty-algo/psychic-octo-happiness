import {
  ApiOutlined,
  CloudServerOutlined,
  ControlOutlined,
  DashboardOutlined,
  FileSearchOutlined,
  FolderOutlined,
  KeyOutlined,
  MenuOutlined,
  ProfileOutlined,
  SafetyOutlined,
  SettingOutlined,
  UploadOutlined,
  UserOutlined,
} from '@ant-design/icons';
import React from 'react';

import type { Menu } from '@/services/admin';

type NavigationItem = {
  path?: string;
  children?: NavigationItem[];
  routes?: NavigationItem[];
  [key: string]: unknown;
};

const normalizeMenuPath = (path?: string): string => {
  if (!path) return '';
  return path.length > 1 && path.endsWith('/') ? path.slice(0, -1) : path;
};

const menuIconComponents: Record<string, React.ElementType> = {
  api: ApiOutlined,
  control: ControlOutlined,
  dashboard: DashboardOutlined,
  fileSearch: FileSearchOutlined,
  folder: FolderOutlined,
  key: KeyOutlined,
  menu: MenuOutlined,
  profile: ProfileOutlined,
  safety: SafetyOutlined,
  server: CloudServerOutlined,
  setting: SettingOutlined,
  upload: UploadOutlined,
  user: UserOutlined,
};

// The backend owns menu visibility; the static route table only registers pages.
export function filterMenuDataByGrantedMenus(
  menuData: NavigationItem[],
  menus: Menu[],
): NavigationItem[] {
  const visibleMenus = menus.filter((menu) => menu.active && !menu.hidden);
  if (visibleMenus.length === 0) return [];

  const routeByPath = indexNavigationItems(menuData);
  const childrenByParent = groupMenusByParent(visibleMenus);
  const rendered = new Set<number>();
  const out = buildNavigationItems(
    childrenByParent.get(0) ?? [],
    childrenByParent,
    routeByPath,
    rendered,
  );

  // If a role grants a child menu without its parent group, keep the page
  // reachable in the navigation instead of hiding an otherwise valid grant.
  for (const menu of visibleMenus) {
    if (rendered.has(menu.id)) continue;
    const item = buildNavigationItem(
      menu,
      childrenByParent,
      routeByPath,
      rendered,
    );
    if (item) out.push(item);
  }
  return out;
}

function indexNavigationItems(
  items: NavigationItem[] | undefined,
  out = new Map<string, NavigationItem>(),
): Map<string, NavigationItem> {
  if (!items) return out;
  for (const item of items) {
    const path = normalizeMenuPath(item.path);
    if (path && !out.has(path)) out.set(path, item);
    indexNavigationItems(item.children, out);
    indexNavigationItems(item.routes, out);
  }
  return out;
}

function groupMenusByParent(menus: Menu[]): Map<number, Menu[]> {
  const groups = new Map<number, Menu[]>();
  for (const menu of menus) {
    const children = groups.get(menu.parent_id) ?? [];
    children.push(menu);
    groups.set(menu.parent_id, children);
  }
  return groups;
}

function buildNavigationItems(
  menus: Menu[],
  childrenByParent: Map<number, Menu[]>,
  routeByPath: Map<string, NavigationItem>,
  rendered: Set<number>,
): NavigationItem[] {
  const out: NavigationItem[] = [];
  for (const menu of menus) {
    const item = buildNavigationItem(
      menu,
      childrenByParent,
      routeByPath,
      rendered,
    );
    if (item) out.push(item);
  }
  return out;
}

function buildNavigationItem(
  menu: Menu,
  childrenByParent: Map<number, Menu[]>,
  routeByPath: Map<string, NavigationItem>,
  rendered: Set<number>,
): NavigationItem | undefined {
  const path = normalizeMenuPath(menu.path);
  const route = routeByPath.get(path);
  const routes = buildNavigationItems(
    childrenByParent.get(menu.id) ?? [],
    childrenByParent,
    routeByPath,
    rendered,
  );
  rendered.add(menu.id);

  if (routes.length > 0) {
    return {
      ...(route ?? {}),
      path: route?.path ?? menu.path,
      name: route?.name ?? routeNameFromPath(menu.path),
      icon: normalizeMenuIcon(route?.icon ?? menu.icon),
      routes,
    };
  }

  if (!route) return undefined;
  return {
    ...route,
    icon: normalizeMenuIcon(route.icon ?? menu.icon),
  };
}

/** 按菜单 icon 字段解析对应的 antd 图标元素；无法识别时返回 undefined，由调用方兜底。 */
export function menuIconElement(name?: string): React.ReactNode {
  const Icon = name ? menuIconComponents[name] : undefined;
  return Icon ? React.createElement(Icon) : undefined;
}

function routeNameFromPath(path: string): string {
  const name = normalizeMenuPath(path).replace(/^\//, '');
  return name.replace(/-([a-z])/g, (_, char: string) => char.toUpperCase());
}

function normalizeMenuIcon(icon: unknown): unknown {
  if (React.isValidElement(icon)) return icon;
  if (typeof icon !== 'string') return icon;
  const Icon = menuIconComponents[icon];
  return Icon ? React.createElement(Icon) : undefined;
}
