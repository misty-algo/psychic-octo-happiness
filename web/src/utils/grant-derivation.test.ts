import { describe, expect, it } from 'vitest';

import type { APIResource, Menu } from '@/services/admin';

import {
  deriveAPIIDs,
  deriveButtonIDs,
  deriveMenuIDs,
} from './grant-derivation';

const menu = (
  id: number,
  parentID: number,
  name: string,
  permission: string,
  buttons: string[] = [],
): Menu => ({
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
  permission,
  sort: 0,
  active: true,
  buttons: buttons.map((button, index) => ({
    id: id * 100 + index,
    menu_id: id,
    name: button,
    description: button,
    created_at: '',
    updated_at: '',
  })),
});

const menus = [
  menu(1, 0, 'dashboard', ''),
  menu(2, 0, 'access', ''),
  menu(3, 2, 'admins', 'admin:read', ['create', 'update', 'delete']),
  menu(4, 2, 'roles', 'role:read', ['create', 'update']),
  menu(5, 0, 'files', 'file:read', ['upload']),
];

const apis: APIResource[] = [
  {
    id: 11,
    method: 'GET',
    path: '/api/admins',
    description: '',
    group: 'admin',
    permission: 'admin:read',
    created_at: '',
    updated_at: '',
  },
  {
    id: 12,
    method: 'POST',
    path: '/api/admins',
    description: '',
    group: 'admin',
    permission: 'admin:create',
    created_at: '',
    updated_at: '',
  },
  {
    id: 13,
    method: 'GET',
    path: '/api/roles',
    description: '',
    group: 'role',
    permission: 'role:read',
    created_at: '',
    updated_at: '',
  },
  {
    id: 14,
    method: 'GET',
    path: '/api/auth/me',
    description: '',
    group: 'auth',
    permission: '',
    created_at: '',
    updated_at: '',
  },
];

describe('deriveMenuIDs', () => {
  it('grants permission-matched leaves plus ancestors and the default entry', () => {
    const ids = deriveMenuIDs(['admin:read'], menus).sort();
    // admins(3) + 祖先 access(2) + 默认入口 dashboard(1)
    expect(ids).toEqual([1, 2, 3]);
  });

  it('excludes menus whose token is not granted', () => {
    const ids = deriveMenuIDs(['admin:read'], menus);
    expect(ids).not.toContain(4);
    expect(ids).not.toContain(5);
  });

  it('derives the full navigation when all leaf tokens are granted', () => {
    const ids = deriveMenuIDs(['admin:read', 'role:read', 'file:read'], menus).sort();
    expect(ids).toEqual([1, 2, 3, 4, 5]);
  });

  it('keeps only the default entry when no token is granted', () => {
    expect(deriveMenuIDs([], menus)).toEqual([1]);
  });
});

describe('deriveAPIIDs', () => {
  it('selects APIs whose permission token is granted', () => {
    expect(deriveAPIIDs(['admin:read'], apis).sort()).toEqual([11, 14]);
  });

  it('selects multiple routes across groups', () => {
    expect(deriveAPIIDs(['admin:read', 'role:read'], apis).sort()).toEqual([
      11, 13, 14,
    ]);
  });

  it('always grants self-service routes with empty permission', () => {
    // 无 token 的会话/自助路由（auth/me、logout 等）对所有角色默认放行。
    expect(deriveAPIIDs([], apis)).toEqual([14]);
  });
});

describe('deriveButtonIDs', () => {
  it('returns every button of the granted menus', () => {
    expect(deriveButtonIDs([2, 3], menus).sort()).toEqual([300, 301, 302]);
  });

  it('returns empty when no menu is granted', () => {
    expect(deriveButtonIDs([], menus)).toEqual([]);
  });
});
