import { MoreOutlined, PlusOutlined } from '@ant-design/icons';
import {
  DrawerForm,
  PageContainer,
  ProCard,
  ProForm,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
} from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import {
  App,
  Avatar,
  Button,
  Checkbox,
  Dropdown,
  Empty,
  Form,
  Input,
  message,
  Pagination,
  Space,
  Spin,
  Tabs,
  Tag,
  Tree,
  Typography,
} from 'antd';
import { createStyles } from 'antd-style';
import type { MenuProps } from 'antd';
import React, { useEffect, useMemo, useState } from 'react';

import {
  type AdminUser,
  type APIResource,
  copyRole,
  createRole,
  deleteRole,
  listAdmins,
  listAPIs,
  listMenus,
  listPermissions,
  listRoleAdmins,
  listRoles,
  type Menu,
  type PermissionDefinition,
  type Role,
  setRoleAdmins,
  updateRole,
} from '@/services/admin';
import {
  deriveAPIIDs,
  deriveButtonIDs,
  deriveMenuIDs,
} from '@/utils/grant-derivation';
import { toAntdTreeData, type AntdTreeNode } from '@/utils/menu-tree';

type RoleFormValues = {
  parent_id: number;
  code: string;
  name: string;
  permissions: string[];
  menu_ids?: number[];
  api_ids?: number[];
  button_ids?: number[];
  data_role_ids?: number[];
  default_path?: string;
  active: boolean;
};

type MemberTarget = {
  role: Role;
  admin_ids: number[];
};

// 角色头像的循环取色，让左侧列表一眼能区分不同角色。
const avatarColors = [
  '#1677ff',
  '#13c2c2',
  '#722ed1',
  '#fa8c16',
  '#52c41a',
  '#eb2f96',
  '#f5222d',
  '#2f54eb',
];

const avatarColor = (roleID: number) =>
  avatarColors[roleID % avatarColors.length];

// 角色页布局样式：左栏是平铺列表 + 强选中态（浅蓝底 + 左侧主色竖条），
// 右栏是"页头 + 平铺统计行 + 页签"，避免卡片套卡片和 StatisticCard 的小卡感。
const useStyles = createStyles(({ token, css }) => ({
  toolbar: css`
    display: flex;
    gap: 8px;
    margin-bottom: 12px;
  `,
  roleList: css`
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-height: 120px;
  `,
  roleItem: css`
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 12px 10px 9px;
    border-left: 3px solid transparent;
    border-radius: 8px;
    cursor: pointer;
    transition: background 0.2s;
    &:hover {
      background: ${token.colorFillQuaternary};
    }
  `,
  roleItemActive: css`
    background: ${token.colorPrimaryBg};
    border-left-color: ${token.colorPrimary};
  `,
  roleItemMain: css`
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  `,
  roleItemTitle: css`
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 15px;
    font-weight: 500;
    color: ${token.colorText};
  `,
  roleItemCode: css`
    font-size: 12px;
    color: ${token.colorTextSecondary};
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  `,
  detailHeader: css`
    display: flex;
    align-items: center;
    gap: 16px;
  `,
  detailMeta: css`
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  `,
  detailTitle: css`
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 20px;
    font-weight: 600;
    line-height: 1.3;
    color: ${token.colorText};
  `,
  statRow: css`
    display: flex;
    margin: 20px 0 4px;
  `,
  statItem: css`
    flex: 1;
    padding: 0 32px;
    border-left: 1px solid ${token.colorSplit};
    &:first-child {
      border-left: none;
      padding-left: 0;
    }
  `,
  statValue: css`
    font-size: 26px;
    font-weight: 600;
    line-height: 1.2;
    color: ${token.colorText};
  `,
  statLabel: css`
    margin-top: 4px;
    font-size: 13px;
    color: ${token.colorTextSecondary};
  `,
  permMatrixWrap: css`
    overflow-x: auto;
  `,
  permMatrix: css`
    width: 100%;
    border-collapse: collapse;
    th,
    td {
      padding: 9px 12px;
      border-bottom: 1px solid ${token.colorSplit};
      font-size: 13px;
      white-space: nowrap;
    }
    thead th {
      color: ${token.colorTextSecondary};
      font-weight: 500;
    }
    tbody tr:hover td {
      background: ${token.colorFillQuaternary};
    }
    tbody tr:last-child td {
      border-bottom: none;
    }
  `,
  permMatrixCheck: css`
    text-align: center;
  `,
  permMatrixEmpty: css`
    text-align: center;
    color: ${token.colorTextQuaternary};
  `,
  permMenuHint: css`
    margin: 2px 0 0 24px;
    font-size: 12px;
    color: ${token.colorTextTertiary};
  `,
}));

// 详情页签里分组 Tag 列表。
const groupTags = (
  groups: Map<string, { label: string }[]>,
): React.ReactNode[] =>
  [...groups.entries()].map(([group, items]) => (
    <Space
      key={group}
      direction="vertical"
      size={4}
      style={{ display: 'flex' }}
    >
      <Typography.Text type="secondary">{group}</Typography.Text>
      <Space wrap size={[8, 8]}>
        {items.map((item) => (
          <Tag key={item.label}>{item.label}</Tag>
        ))}
      </Space>
    </Space>
  ));

// 操作权限字段：按资源分组的卡片式勾选，是角色授权的单一控制面。
// 菜单可见性、按钮显示、API 放行均由此派生，卡片底部实时预览派生结果。
const PermissionGroupsField = ({
  value = [],
  onChange,
  permissions,
  apis,
  menus,
}: {
  value?: string[];
  onChange?: (value: string[]) => void;
  permissions: PermissionDefinition[];
  apis: APIResource[];
  menus: Menu[];
}) => {
  const { styles } = useStyles();
  // 资源 → (操作中文名 → 权限定义)，驱动"资源 × 操作"勾选矩阵。
  const groups = new Map<string, Map<string, PermissionDefinition>>();
  for (const permission of permissions) {
    const byAction = groups.get(permission.resource) ?? new Map();
    byAction.set(permission.action, permission);
    groups.set(permission.resource, byAction);
  }
  // 操作列按常见顺序排列，目录新增的未知操作排在末尾，避免被静默隐藏。
  const actionOrder = ['查看', '创建', '更新', '删除', '上传', '授权', '处理'];
  const actionColumns = [...new Set(permissions.map((item) => item.action))].sort(
    (a, b) => {
      const rank = (action: string): number => {
        const index = actionOrder.indexOf(action);
        // 未知操作统一排在已知顺序之后，避免被静默隐藏。
        return index === -1 ? actionOrder.length : index;
      };
      return rank(a) - rank(b);
    },
  );
  // 实时预览派生结果：勾选时直接列出会开放的菜单名，让"操作 → 菜单"
  // 的因果关系对配置者可见。
  const grantedMenuIDs = new Set(deriveMenuIDs(value, menus));
  const grantedMenuNames = menus
    .filter((menu) => grantedMenuIDs.has(menu.id))
    .map((menu) => menu.name);
  const menuSummary =
    grantedMenuNames.length > 6
      ? `${grantedMenuNames.slice(0, 6).join('、')} 等 ${grantedMenuNames.length} 个`
      : grantedMenuNames.join('、');
  const apiCount = deriveAPIIDs(value, apis).length;
  // 资源 → 绑定菜单名：菜单绑定的 read token 决定勾"查看"后开放哪个菜单，
  // 直接标注在矩阵行内，配置者不必到预览里反查。
  const definitionByToken = new Map(
    permissions.map((item) => [item.token, item]),
  );
  const menuByResource = new Map<string, string[]>();
  for (const menu of menus) {
    if (!menu.permission) {
      continue;
    }
    const definition = definitionByToken.get(menu.permission);
    if (!definition) {
      continue;
    }
    const list = menuByResource.get(definition.resource) ?? [];
    list.push(menu.name);
    menuByResource.set(definition.resource, list);
  }
  return (
    <Space direction="vertical" size={8} style={{ display: 'flex' }}>
      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        勾选"查看"后开放该行标注的菜单；创建、更新、删除等操作只控制页面按钮与
        API 放行。
      </Typography.Text>
      <div className={styles.permMatrixWrap}>
        <table className={styles.permMatrix}>
          <thead>
            <tr>
              <th>资源</th>
              {actionColumns.map((action) => (
                <th key={action} className={styles.permMatrixCheck}>
                  {action}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {[...groups.entries()].map(([resource, byAction]) => {
              const tokens = [...byAction.values()].map((item) => item.token);
              const checkedCount = tokens.filter((token) =>
                value.includes(token),
              ).length;
              const boundMenus = menuByResource.get(resource);
              return (
                <tr key={resource}>
                  <td>
                    <Checkbox
                      checked={checkedCount === tokens.length}
                      indeterminate={
                        checkedCount > 0 && checkedCount < tokens.length
                      }
                      onChange={(event) => {
                        const next = event.target.checked
                          ? [...new Set([...value, ...tokens])]
                          : value.filter((token) => !tokens.includes(token));
                        onChange?.(next);
                      }}
                    >
                      {resource}
                    </Checkbox>
                    {boundMenus && boundMenus.length > 0 && (
                      <div className={styles.permMenuHint}>
                        菜单：{boundMenus.join('、')}
                      </div>
                    )}
                  </td>
                  {actionColumns.map((action) => {
                    const item = byAction.get(action);
                    if (!item) {
                      return (
                        <td key={action} className={styles.permMatrixEmpty}>
                          —
                        </td>
                      );
                    }
                    return (
                      <td key={action} className={styles.permMatrixCheck}>
                        <Checkbox
                          checked={value.includes(item.token)}
                          onChange={(event) => {
                            onChange?.(
                              event.target.checked
                                ? [...value, item.token]
                                : value.filter((token) => token !== item.token),
                            );
                          }}
                        />
                      </td>
                    );
                  })}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        将开放菜单：{menuSummary || '仅默认工作台'}；放行 {apiCount} 条受管
        API。
      </Typography.Text>
    </Space>
  );
};

const Roles: React.FC = () => {
  const access = useAccess();
  const { styles } = useStyles();
  const { modal } = App.useApp();
  const [roles, setRoles] = useState<Role[]>([]);
  const [rolesLoading, setRolesLoading] = useState(false);
  const [selectedRoleID, setSelectedRoleID] = useState<number>();
  const [keyword, setKeyword] = useState('');
  const [listPage, setListPage] = useState(1);
  const [admins, setAdmins] = useState<AdminUser[]>([]);
  const [menus, setMenus] = useState<Menu[]>([]);
  const [apis, setAPIs] = useState<APIResource[]>([]);
  const [permissions, setPermissions] = useState<PermissionDefinition[]>([]);
  const [memberIDs, setMemberIDs] = useState<number[]>([]);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<Role>();
  const [copying, setCopying] = useState<Role>();
  const [memberTarget, setMemberTarget] = useState<MemberTarget>();

  const loadRoles = async (preferID?: number) => {
    setRolesLoading(true);
    try {
      const response = await listRoles({ page_size: 100 });
      setRoles(response.data);
      setSelectedRoleID((previous) => {
        const wanted = preferID ?? previous;
        if (
          wanted !== undefined &&
          response.data.some((role) => role.id === wanted)
        ) {
          return wanted;
        }
        return response.data[0]?.id;
      });
    } finally {
      setRolesLoading(false);
    }
  };

  useEffect(() => {
    void loadRoles();
    void Promise.all([
      access.canAdminRead
        ? listAdmins({ page_size: 100 })
        : Promise.resolve({ data: [] }),
      access.canMenuRead ? listMenus() : Promise.resolve([]),
      access.canApiRead
        ? listAPIs({ page_size: 100 })
        : Promise.resolve({ data: [] }),
      listPermissions(),
    ]).then(
      ([adminResponse, menuResponse, apiResponse, permissionResponse]) => {
        setAdmins(adminResponse.data);
        setMenus(menuResponse);
        setAPIs(apiResponse.data);
        setPermissions(permissionResponse);
      },
    );
  }, [access]);

  // 成员页签的数据随选中角色切换重新拉取。
  const canManageMembers = access.canRoleUpdate && access.canAdminRead;
  useEffect(() => {
    if (!selectedRoleID || !canManageMembers) {
      setMemberIDs([]);
      return;
    }
    void listRoleAdmins(selectedRoleID).then(setMemberIDs);
  }, [selectedRoleID, canManageMembers, roles]);

  const selectedRole = roles.find((role) => role.id === selectedRoleID);
  const filteredRoles = useMemo(() => {
    const query = keyword.trim().toLowerCase();
    if (!query) {
      return roles;
    }
    return roles.filter(
      (role) =>
        role.name.toLowerCase().includes(query) ||
        role.code.toLowerCase().includes(query),
    );
  }, [roles, keyword]);
  // 左栏列表分页：一页 8 条，超过才显示分页器。
  const listPageSize = 8;
  const pagedRoles = filteredRoles.slice(
    (listPage - 1) * listPageSize,
    listPage * listPageSize,
  );

  const roleName = (roleID: number) =>
    roles.find((role) => role.id === roleID)?.name ?? `#${roleID}`;
  const permissionByToken = useMemo(
    () =>
      new Map(permissions.map((permission) => [permission.token, permission])),
    [permissions],
  );

  // 操作权限页签分组：只展示该角色已持有的 token；name 与 token 相同时
  // 只显示一份，避免出现 "admin:read (admin:read)" 这类重复占位。
  const grantedPermissionGroups = useMemo(() => {
    const groups = new Map<string, { label: string }[]>();
    if (!selectedRole) {
      return groups;
    }
    for (const token of selectedRole.permissions) {
      const permission = permissionByToken.get(token);
      const key = permission?.resource ?? 'other';
      const options = groups.get(key) ?? [];
      options.push({
        label:
          permission && permission.name !== token
            ? `${permission.name} (${token})`
            : (permission?.name ?? token),
      });
      groups.set(key, options);
    }
    return groups;
  }, [selectedRole, permissionByToken]);

  // 菜单树展示：name 即后端中文名；勾选框只用于呈现授权结果，
  // 禁用交互避免"点不动"的假按钮感。
  const menuTreeData = useMemo(() => {
    const decorate = (
      nodes: AntdTreeNode[],
    ): (AntdTreeNode & { disableCheckbox: true })[] =>
      nodes.map((node) => ({
        ...node,
        disableCheckbox: true,
        children: decorate(node.children),
      }));
    return decorate(toAntdTreeData(menus));
  }, [menus]);
  const parentRoleOptions = [
    { label: '顶级角色', value: 0 },
    ...roles
      .filter((role) => role.id !== editing?.id)
      .map((role) => ({ label: role.name, value: role.id })),
  ];
  const adminOptions = admins.map((admin) => ({
    label: `${admin.display_name} (${admin.username})`,
    value: admin.id,
  }));

  const formInitialValues = editing
    ? {
        parent_id: editing.parent_id,
        code: editing.code,
        name: editing.name,
        permissions: editing.permissions,
        menu_ids: editing.menu_ids,
        api_ids: editing.api_ids,
        button_ids: editing.button_ids,
        data_role_ids: editing.data_role_ids,
        default_path: editing.default_path,
        active: editing.active,
      }
    : copying
      ? {
          parent_id: copying.parent_id,
          code: `${copying.code}_copy`,
          name: `${copying.name}副本`,
          permissions: copying.permissions,
          menu_ids: copying.menu_ids,
          api_ids: copying.api_ids,
          button_ids: copying.button_ids,
          data_role_ids: copying.data_role_ids,
          default_path: copying.default_path,
          active: copying.active,
        }
      : {
          active: true,
          parent_id: 0,
          permissions: [],
          menu_ids: [],
          api_ids: [],
          button_ids: [],
          data_role_ids: [],
          default_path: '/dashboard',
        };

  // 页头操作：编辑是高频主操作；复制、删除是低频操作，收进「更多」下拉。
  const moreMenuItems: MenuProps['items'] = [
    access.canRoleCreate ? { key: 'copy', label: '复制角色' } : null,
    access.canRoleDelete
      ? { key: 'delete', label: '删除角色', danger: true }
      : null,
  ].filter(Boolean);

  const headerActions = selectedRole ? (
    <Space>
      {access.canRoleUpdate ? (
        <Button
          type="primary"
          onClick={() => {
            setCopying(undefined);
            setEditing(selectedRole);
            setDrawerOpen(true);
          }}
        >
          编辑
        </Button>
      ) : null}
      {moreMenuItems && moreMenuItems.length > 0 ? (
        <Dropdown
          trigger={['click']}
          menu={{
            items: moreMenuItems,
            onClick: ({ key }) => {
              if (key === 'copy') {
                setEditing(undefined);
                setCopying(selectedRole);
                setDrawerOpen(true);
                return;
              }
              if (key === 'delete') {
                modal.confirm({
                  title: '删除角色',
                  content: `确认删除 ${selectedRole.name}？`,
                  okText: '删除',
                  okButtonProps: { danger: true },
                  onOk: async () => {
                    await deleteRole(selectedRole.id);
                    message.success('角色已删除');
                    await loadRoles();
                  },
                });
              }
            },
          }}
        >
          <Button icon={<MoreOutlined />} />
        </Dropdown>
      ) : null}
    </Space>
  ) : undefined;

  return (
    <PageContainer title="角色权限">
      <ProCard gutter={[24, 24]} wrap>
        <ProCard colSpan={{ xs: 24, lg: 8, xxl: 8 }}>
          <div className={styles.toolbar}>
            <Input.Search
              allowClear
              placeholder="搜索角色名称或编码"
              style={{ flex: 1 }}
              onChange={(event) => {
                setKeyword(event.target.value);
                setListPage(1);
              }}
            />
            {access.canRoleCreate ? (
              <Button
                key="create"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
                  setCopying(undefined);
                  setEditing(undefined);
                  setDrawerOpen(true);
                }}
              >
                新增
              </Button>
            ) : null}
          </div>
          <Spin spinning={rolesLoading}>
            {pagedRoles.length > 0 ? (
              <div className={styles.roleList}>
                {pagedRoles.map((role) => (
                  <div
                    key={role.id}
                    className={
                      role.id === selectedRoleID
                        ? `${styles.roleItem} ${styles.roleItemActive}`
                        : styles.roleItem
                    }
                    onClick={() => setSelectedRoleID(role.id)}
                  >
                    <Avatar
                      size={36}
                      style={{
                        backgroundColor: avatarColor(role.id),
                        flexShrink: 0,
                      }}
                    >
                      {role.name.slice(0, 1)}
                    </Avatar>
                    <div className={styles.roleItemMain}>
                      <div className={styles.roleItemTitle}>
                        <span>{role.name}</span>
                        <Tag
                          color={role.active ? 'green' : 'default'}
                          style={{ marginRight: 0, fontWeight: 400 }}
                        >
                          {role.active ? '启用' : '停用'}
                        </Tag>
                      </div>
                      <div className={styles.roleItemCode}>{role.code}</div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="没有匹配的角色"
              />
            )}
            {filteredRoles.length > listPageSize ? (
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'center',
                  marginTop: 12,
                }}
              >
                <Pagination
                  simple
                  size="small"
                  pageSize={listPageSize}
                  current={listPage}
                  total={filteredRoles.length}
                  onChange={setListPage}
                />
              </div>
            ) : null}
          </Spin>
        </ProCard>
        <ProCard colSpan={{ xs: 24, lg: 16, xxl: 16 }}>
          {selectedRole ? (
            <>
              <div className={styles.detailHeader}>
                <Avatar
                  size={44}
                  style={{
                    backgroundColor: avatarColor(selectedRole.id),
                    flexShrink: 0,
                  }}
                >
                  {selectedRole.name.slice(0, 1)}
                </Avatar>
                <div className={styles.detailMeta}>
                  <div className={styles.detailTitle}>
                    <span>{selectedRole.name}</span>
                    <Tag
                      color={selectedRole.active ? 'green' : 'default'}
                      style={{ marginRight: 0, fontWeight: 400 }}
                    >
                      {selectedRole.active ? '启用' : '停用'}
                    </Tag>
                  </div>
                  <Typography.Text
                    type="secondary"
                    style={{ fontSize: 13 }}
                    ellipsis
                  >
                    {selectedRole.code}
                    {' · 上级角色：'}
                    {selectedRole.parent_id === 0
                      ? '顶级角色'
                      : roleName(selectedRole.parent_id)}
                    {' · 默认入口 '}
                    {selectedRole.default_path}
                  </Typography.Text>
                </div>
                {headerActions}
              </div>
              <div className={styles.statRow}>
                {[
                  {
                    label: '操作权限',
                    value: selectedRole.permissions.length,
                  },
                  { label: '可见菜单', value: selectedRole.menu_ids.length },
                  { label: 'API 放行', value: selectedRole.api_ids.length },
                  {
                    label: '数据角色',
                    value: selectedRole.data_role_ids.length,
                  },
                ].map((item) => (
                  <div key={item.label} className={styles.statItem}>
                    <div className={styles.statValue}>{item.value}</div>
                    <div className={styles.statLabel}>{item.label}</div>
                  </div>
                ))}
              </div>
              <Tabs
                items={[
                  {
                    key: 'permissions',
                    label: '操作权限',
                    children: (
                      <Space
                        direction="vertical"
                        size={12}
                        style={{ display: 'flex' }}
                      >
                        <Typography.Text type="secondary">
                          按资源勾选角色可执行的操作；菜单、按钮与 API
                          放行由此自动生成。
                        </Typography.Text>
                        {grantedPermissionGroups.size > 0 ? (
                          groupTags(grantedPermissionGroups)
                        ) : (
                          <Empty description="该角色没有操作权限" />
                        )}
                      </Space>
                    ),
                  },
                  {
                    key: 'menus',
                    label: '可见菜单',
                    children: (
                      <Space
                        direction="vertical"
                        size={8}
                        style={{ display: 'flex' }}
                      >
                        <Typography.Text type="secondary">
                          以下菜单由操作权限自动开放，点击「编辑」调整操作权限。
                        </Typography.Text>
                        <Tree
                          key={`menu-tree-${menus.length}`}
                          checkable
                          selectable={false}
                          checkedKeys={selectedRole.menu_ids}
                          treeData={menuTreeData}
                          defaultExpandAll
                        />
                      </Space>
                    ),
                  },
                  {
                    key: 'members',
                    label: '成员',
                    children: (
                      <Space
                        direction="vertical"
                        size={12}
                        style={{ display: 'flex' }}
                      >
                        {canManageMembers ? (
                          <Button
                            onClick={() => {
                              setMemberTarget({
                                role: selectedRole,
                                admin_ids: memberIDs,
                              });
                            }}
                          >
                            编辑成员
                          </Button>
                        ) : null}
                        {memberIDs.length > 0 ? (
                          <Space wrap size={[8, 8]}>
                            {memberIDs.map((adminID) => {
                              const admin = admins.find(
                                (candidate) => candidate.id === adminID,
                              );
                              return (
                                <Tag key={adminID} color="blue">
                                  {admin
                                    ? `${admin.display_name} (${admin.username})`
                                    : `#${adminID}`}
                                </Tag>
                              );
                            })}
                          </Space>
                        ) : (
                          <Empty description="该角色暂无成员" />
                        )}
                      </Space>
                    ),
                  },
                ]}
              />
            </>
          ) : (
            <Empty
              description="选择左侧角色查看授权详情"
              style={{ padding: '96px 0' }}
            />
          )}
        </ProCard>
      </ProCard>
      <DrawerForm<RoleFormValues>
        key={
          editing
            ? `edit-${editing.id}`
            : copying
              ? `copy-${copying.id}`
              : 'create'
        }
        title={editing ? '编辑角色' : copying ? '复制角色' : '新增角色'}
        open={drawerOpen}
        onOpenChange={(open) => {
          setDrawerOpen(open);
          if (!open) {
            setCopying(undefined);
          }
        }}
        width="min(720px, 100%)"
        grid
        drawerProps={{ destroyOnHidden: true }}
        initialValues={formInitialValues}
        onFinish={async (values) => {
          // 操作权限是单一控制面：菜单/按钮/API 授权全部由 token 派生，
          // 勾一处即同时决定菜单可见、按钮显示和后端路由放行。
          const menu_ids = deriveMenuIDs(values.permissions, menus);
          const api_ids = deriveAPIIDs(values.permissions, apis);
          const button_ids = deriveButtonIDs(menu_ids, menus);
          if (editing) {
            await updateRole(editing.id, {
              parent_id: values.parent_id,
              name: values.name,
              permissions: values.permissions,
              menu_ids,
              api_ids,
              button_ids,
              data_role_ids: values.data_role_ids,
              default_path: values.default_path,
              active: values.active,
            });
            message.success('角色已更新');
            await loadRoles(editing.id);
          } else if (copying) {
            await copyRole(copying.id, {
              parent_id: values.parent_id,
              code: values.code,
              name: values.name,
              default_path: values.default_path,
              active: values.active,
            });
            message.success('角色已复制');
            await loadRoles();
          } else {
            await createRole({
              parent_id: values.parent_id,
              code: values.code,
              name: values.name,
              permissions: values.permissions,
              menu_ids,
              api_ids,
              button_ids,
              data_role_ids: values.data_role_ids,
              default_path: values.default_path,
              active: values.active,
            });
            message.success('角色已创建');
            await loadRoles();
          }
          return true;
        }}
      >
        <ProForm.Group title="基本信息" grid>
          <ProFormText
            name="name"
            label="名称"
            colProps={{ xs: 24, md: 12 }}
            fieldProps={{ maxLength: 80 }}
            rules={[{ required: true, message: '请输入角色名称' }]}
          />
          <ProFormText
            name="code"
            label="编码"
            colProps={{ xs: 24, md: 12 }}
            disabled={Boolean(editing)}
            fieldProps={{ maxLength: 64 }}
            rules={
              editing ? [] : [{ required: true, message: '请输入角色编码' }]
            }
          />
          <ProFormSelect
            name="parent_id"
            label="上级角色"
            colProps={{ xs: 24, md: 12 }}
            options={parentRoleOptions}
          />
          <ProFormText
            name="default_path"
            label="默认入口"
            colProps={{ xs: 24, md: 12 }}
            fieldProps={{ maxLength: 160 }}
            rules={[{ required: true, message: '请输入默认入口' }]}
          />
          <ProFormSwitch
            name="active"
            label="启用"
            colProps={{ xs: 24, md: 12 }}
          />
        </ProForm.Group>
        {copying ? (
          <Typography.Text type="secondary">
            操作权限及由此生成的菜单、API、按钮和数据角色授权将从源角色「{copying.name}
            」复制。
          </Typography.Text>
        ) : (
          <>
            <Form.Item
              name="permissions"
              label="操作权限"
              required
              tooltip="勾选后自动生成菜单可见性、按钮显示和 API 放行，无需分别配置"
            >
              <PermissionGroupsField
                permissions={permissions}
                apis={apis}
                menus={menus}
              />
            </Form.Item>
            <ProFormSelect
              name="data_role_ids"
              label="数据权限（可见的管理员数据范围）"
              mode="multiple"
              colProps={{ span: 24 }}
              options={roles.map((role) => ({
                label: role.name,
                value: role.id,
              }))}
              fieldProps={{ maxTagCount: 'responsive' }}
            />
          </>
        )}
      </DrawerForm>
      <DrawerForm<{ admin_ids: number[] }>
        key={memberTarget?.role.id ?? 'idle'}
        title={memberTarget ? `${memberTarget.role.name}成员` : '角色成员'}
        open={Boolean(memberTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setMemberTarget(undefined);
          }
        }}
        drawerProps={{ destroyOnHidden: true }}
        initialValues={{ admin_ids: memberTarget?.admin_ids ?? [] }}
        onFinish={async (values) => {
          if (!memberTarget) {
            return true;
          }
          const assignedIDs = await setRoleAdmins(
            memberTarget.role.id,
            values.admin_ids ?? [],
          );
          setMemberIDs(assignedIDs);
          setMemberTarget((previous) =>
            previous ? { ...previous, admin_ids: assignedIDs } : previous,
          );
          message.success('角色成员已更新');
          return true;
        }}
      >
        <ProFormSelect
          name="admin_ids"
          label="成员"
          mode="multiple"
          options={adminOptions}
          fieldProps={{
            showSearch: true,
            optionFilterProp: 'label',
            maxTagCount: 'responsive',
          }}
        />
      </DrawerForm>
    </PageContainer>
  );
};

export default Roles;
