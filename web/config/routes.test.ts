import { describe, expect, it } from 'vitest';

import routes from './routes';

type Route = {
  name?: string;
  path?: string;
  routes?: Route[];
};

function routeByName(name: string): Route | undefined {
  return (routes as Route[]).find((route) => route.name === name);
}

describe('routes', () => {
  it('keeps admin pages as flat Umi routes', () => {
    const wantPathsByName = new Map([
      ['admins', '/admins'],
      ['roles', '/roles'],
      ['menus', '/menus'],
      ['apis', '/apis'],
      ['apiTokens', '/api-tokens'],
      ['configs', '/configs'],
      ['params', '/params'],
      ['versions', '/versions'],
      ['dictionaries', '/dictionaries'],
      ['files', '/files'],
      ['logs', '/logs'],
    ]);

    for (const [name, wantPath] of wantPathsByName) {
      expect(routeByName(name)?.path, `${name} route path`).toBe(wantPath);
    }

    expect(routeByName('access'), 'menu-only access group').toBeUndefined();
    expect(routeByName('system'), 'menu-only system group').toBeUndefined();
    expect(routeByName('resources'), 'menu-only resources group').toBeUndefined();
    expect(routeByName('audit'), 'menu-only audit group').toBeUndefined();
  });
});
