import { describe, expect, it } from 'vitest';

import type { Menu } from '@/services/admin';

import {
  buildMenuTree,
  toAntdTreeData,
  toMenuTreeNodes,
  withMenuAncestors,
} from './menu-tree';

const menu = (id: number, parentID: number, name: string): Menu => ({
  id,
  parent_id: parentID,
  name,
  path: `/${name}`,
  icon: '',
  hidden: false,
  component: '',
  meta: {
    active_name: '',
    keep_alive: false,
    default_menu: false,
    close_tab: false,
    transition_type: '',
  },
  permission: '',
  sort: 0,
  active: true,
  buttons: [],
});

const menus = [
  menu(1, 0, '系统'),
  menu(2, 1, '管理员'),
  menu(3, 1, '角色'),
  menu(4, 2, '管理员详情'),
];

describe('buildMenuTree', () => {
  it('nests menus by parent_id under the root level', () => {
    const tree = buildMenuTree(menus);
    expect(tree.map((node) => node.name)).toEqual(['系统']);
    expect(tree[0].children.map((node) => node.name)).toEqual([
      '管理员',
      '角色',
    ]);
    expect(tree[0].children[0].children.map((node) => node.name)).toEqual([
      '管理员详情',
    ]);
  });

  it('can build a subtree from a given parent', () => {
    const tree = buildMenuTree(menus, 2);
    expect(tree.map((node) => node.name)).toEqual(['管理员详情']);
  });
});

describe('toMenuTreeNodes', () => {
  it('prepends a top-level sentinel node', () => {
    const nodes = toMenuTreeNodes(menus);
    expect(nodes[0].title).toBe('顶级菜单');
    expect(nodes[0].value).toBe(0);
    expect(nodes[0].children.map((node) => node.title)).toEqual(['系统']);
  });

  it('excludes the editing menu itself from parent options', () => {
    const nodes = toMenuTreeNodes(menus, 1);
    expect(nodes[0].children).toEqual([]);
  });
});

describe('withMenuAncestors', () => {
  it('adds every ancestor of the selected menus', () => {
    expect(withMenuAncestors([4], menus).sort()).toEqual([1, 2, 4]);
  });

  it('keeps selections without duplicates', () => {
    expect(withMenuAncestors([1, 3], menus).sort()).toEqual([1, 3]);
  });

  it('ignores unknown ids', () => {
    expect(withMenuAncestors([99], menus)).toEqual([]);
  });
});

describe('toAntdTreeData', () => {
  it('produces antd Tree nodes keyed by menu id', () => {
    expect(toAntdTreeData(menus)).toEqual([
      {
        title: '系统',
        key: 1,
        children: [
          {
            title: '管理员',
            key: 2,
            children: [{ title: '管理员详情', key: 4, children: [] }],
          },
          { title: '角色', key: 3, children: [] },
        ],
      },
    ]);
  });
});
