import type { CurrentUser } from '@/services/admin';

/**
 * @see https://umijs.org/docs/max/access#access
 * */
export default function access(
  initialState: { currentUser?: CurrentUser } | undefined,
) {
  const { currentUser } = initialState ?? {};
  const can = (permission: string) =>
    currentUser?.permissions.includes(permission) ?? false;

  return {
    canAdminRead: can('admin:read'),
    canAdminCreate: can('admin:create'),
    canAdminUpdate: can('admin:update'),
    canAdminDelete: can('admin:delete'),
    canRoleRead: can('role:read'),
    canRoleCreate: can('role:create'),
    canRoleUpdate: can('role:update'),
    canRoleDelete: can('role:delete'),
    canMenuRead: can('menu:read'),
    canMenuCreate: can('menu:create'),
    canMenuUpdate: can('menu:update'),
    canMenuDelete: can('menu:delete'),
    canApiRead: can('api:read'),
    canApiGrant: can('api:grant'),
    canApiTokenRead: can('api_token:read'),
    canApiTokenCreate: can('api_token:create'),
    canApiTokenUpdate: can('api_token:update'),
    canApiTokenDelete: can('api_token:delete'),
    canConfigRead: can('config:read'),
    canConfigUpdate: can('config:update'),
    canConfigDelete: can('config:delete'),
    canParamRead: can('param:read'),
    canParamCreate: can('param:create'),
    canParamUpdate: can('param:update'),
    canParamDelete: can('param:delete'),
    canVersionRead: can('version:read'),
    canVersionCreate: can('version:create'),
    canVersionUpdate: can('version:update'),
    canVersionDelete: can('version:delete'),
    canDictRead: can('dict:read'),
    canDictCreate: can('dict:create'),
    canDictUpdate: can('dict:update'),
    canDictDelete: can('dict:delete'),
    canFileRead: can('file:read'),
    canFileUpload: can('file:upload'),
    canFileUpdate: can('file:update'),
    canFileDelete: can('file:delete'),
    canFileCategoryCreate: can('file_category:create'),
    canFileCategoryUpdate: can('file_category:update'),
    canFileCategoryDelete: can('file_category:delete'),
    canLogRead: can('log:read'),
    canLogDelete: can('log:delete'),
    canLogResolve: can('log:resolve'),
  };
}
