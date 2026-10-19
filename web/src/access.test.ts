import { describe, expect, it } from 'vitest';
import access from './access';

const baseUser = {
  id: 1,
  username: 'admin',
  display_name: '系统管理员',
  email: '',
  active_role_id: 1,
  active_role: {} as never,
  default_path: '/dashboard',
  roles: [],
  menus: [],
};

describe('access', () => {
  it('allows admin delete when the permission is granted', () => {
    const result = access({
      currentUser: {
        ...baseUser,
        permissions: ['admin:delete'],
      },
    });

    expect(result.canAdminDelete).toBe(true);
  });

  it('denies admin management without the permission', () => {
    const result = access({
      currentUser: {
        ...baseUser,
        permissions: ['role:delete', 'dict:delete'],
      },
    });

    expect(result.canAdminRead).toBe(false);
    expect(result.canRoleDelete).toBe(true);
    expect(result.canMenuDelete).toBe(false);
    expect(result.canApiRead).toBe(false);
    expect(result.canApiTokenRead).toBe(false);
    expect(result.canDictDelete).toBe(true);
  });

  it('allows api catalog inspection and role grants', () => {
    const result = access({
      currentUser: {
        ...baseUser,
        permissions: ['api:read', 'api:grant'],
      },
    });

    expect(result.canApiRead).toBe(true);
    expect(result.canApiGrant).toBe(true);
  });

  it('allows api token management when token permissions are granted', () => {
    const result = access({
      currentUser: {
        ...baseUser,
        permissions: [
          'api_token:read',
          'api_token:create',
          'api_token:update',
          'api_token:delete',
        ],
      },
    });

    expect(result.canApiTokenRead).toBe(true);
    expect(result.canApiTokenCreate).toBe(true);
    expect(result.canApiTokenUpdate).toBe(true);
    expect(result.canApiTokenDelete).toBe(true);
  });

  it('allows config management when config permissions are granted', () => {
    const result = access({
      currentUser: {
        ...baseUser,
        permissions: ['config:read', 'config:update', 'config:delete'],
      },
    });

    expect(result.canConfigRead).toBe(true);
    expect(result.canConfigUpdate).toBe(true);
    expect(result.canConfigDelete).toBe(true);
  });

  it('allows version management when version permissions are granted', () => {
    const result = access({
      currentUser: {
        ...baseUser,
        permissions: [
          'version:read',
          'version:create',
          'version:update',
          'version:delete',
        ],
      },
    });

    expect(result.canVersionRead).toBe(true);
    expect(result.canVersionCreate).toBe(true);
    expect(result.canVersionUpdate).toBe(true);
    expect(result.canVersionDelete).toBe(true);
  });

  it('allows param management when param permissions are granted', () => {
    const result = access({
      currentUser: {
        ...baseUser,
        permissions: [
          'param:read',
          'param:create',
          'param:update',
          'param:delete',
        ],
      },
    });

    expect(result.canParamRead).toBe(true);
    expect(result.canParamCreate).toBe(true);
    expect(result.canParamUpdate).toBe(true);
    expect(result.canParamDelete).toBe(true);
  });

  it('allows file management when file permissions are granted', () => {
    const result = access({
      currentUser: {
        ...baseUser,
        permissions: [
          'file:read',
          'file:upload',
          'file:update',
          'file:delete',
          'file_category:create',
          'file_category:update',
          'file_category:delete',
        ],
      },
    });

    expect(result.canFileRead).toBe(true);
    expect(result.canFileUpload).toBe(true);
    expect(result.canFileUpdate).toBe(true);
    expect(result.canFileDelete).toBe(true);
    expect(result.canFileCategoryCreate).toBe(true);
    expect(result.canFileCategoryUpdate).toBe(true);
    expect(result.canFileCategoryDelete).toBe(true);
  });

  it('allows log management when log permissions are granted', () => {
    const result = access({
      currentUser: {
        ...baseUser,
        permissions: ['log:read', 'log:delete', 'log:resolve'],
      },
    });

    expect(result.canLogRead).toBe(true);
    expect(result.canLogDelete).toBe(true);
    expect(result.canLogResolve).toBe(true);
  });

  it('denies admin management without a current user', () => {
    expect(access(undefined).canAdminRead).toBe(false);
  });
});
