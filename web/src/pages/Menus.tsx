import { PlusOutlined } from '@ant-design/icons';
import {
  type ActionType,
  DrawerForm,
  PageContainer,
  type ProColumns,
  ProDescriptions,
  ProForm,
  ProFormDigit,
  ProFormList,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProFormTreeSelect,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import {
  Button,
  Collapse,
  Drawer,
  Form,
  InputNumber,
  message,
  Popconfirm,
  Tag,
} from 'antd';
import React, { useEffect, useMemo, useRef, useState } from 'react';

import {
  createMenu,
  deleteMenu,
  listMenus,
  listPermissions,
  type Menu,
  type PermissionDefinition,
  readMenu,
  updateMenu,
} from '@/services/admin';
import {
  buildMenuTree,
  type MenuNode,
  toMenuTreeNodes,
} from '@/utils/menu-tree';

type MenuFormValues = {
  parent_id: number;
  name: string;
  path: string;
  icon?: string;
  hidden: boolean;
  component: string;
  meta: {
    active_name?: string;
    keep_alive: boolean;
    default_menu: boolean;
    close_tab: boolean;
    transition_type?: string;
  };
  permission?: string;
  sort: number;
  active: boolean;
  buttons?: {
    id?: number;
    name: string;
    description?: string;
  }[];
};

const Menus: React.FC = () => {
  const access = useAccess();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [menus, setMenus] = useState<Menu[]>([]);
  const [permissions, setPermissions] = useState<PermissionDefinition[]>([]);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<Menu>();
  // 新增子菜单时预置的上级菜单 id；undefined 表示从工具栏新增顶级菜单。
  const [createParentID, setCreateParentID] = useState<number>();
  const [detail, setDetail] = useState<Menu>();
  // 树形表格的展开行受控：defaultExpandAllRows 对异步加载的数据不生效，
  // 数据到达后手动展开全部有子级的容器。
  const [expandedRowKeys, setExpandedRowKeys] = useState<readonly number[]>([]);

  useEffect(() => {
    void listPermissions().then(setPermissions);
  }, []);

  const menuName = (menuID: number) =>
    menus.find((menu) => menu.id === menuID)?.name ?? `#${menuID}`;

  // 权限绑定展示用中文名；角色页勾选的也是同一批操作，两边语言保持一致，
  // 配置者才能把"菜单 ↔ 操作权限"的映射串起来。
  const permissionByToken = useMemo(
    () => new Map(permissions.map((item) => [item.token, item])),
    [permissions],
  );
  const permissionName = (token?: string): string => {
    if (!token) {
      return '-';
    }
    const definition = permissionByToken.get(token);
    return definition && definition.name !== token ? definition.name : token;
  };
  const permissionDetail = (token?: string): string => {
    if (!token) {
      return '-';
    }
    const name = permissionName(token);
    return name === token ? token : `${name} (${token})`;
  };

  const columns: ProColumns<MenuNode>[] = [
    { title: '名称', dataIndex: 'name', width: 200 },
    { title: '路径', dataIndex: 'path', ellipsis: true },
    {
      title: '绑定权限',
      dataIndex: 'permission',
      width: 140,
      ellipsis: true,
      render: (_, record) => permissionName(record.permission),
    },
    { title: '排序', dataIndex: 'sort', width: 64 },
    {
      title: '按钮',
      dataIndex: 'buttons',
      width: 64,
      render: (_, record) =>
        record.buttons.length > 0 ? `${record.buttons.length} 个` : '-',
    },
    {
      title: '状态',
      dataIndex: 'active',
      width: 120,
      render: (_, record) => (
        <>
          <Tag color={record.hidden ? 'default' : 'blue'}>
            {record.hidden ? '隐藏' : '显示'}
          </Tag>
          <Tag color={record.active ? 'green' : 'default'}>
            {record.active ? '启用' : '停用'}
          </Tag>
        </>
      ),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 200,
      render: (_, record) => {
        const actions: React.ReactNode[] = [];
        if (access.canMenuCreate) {
          actions.push(
            <a
              key="create-child"
              onClick={() => {
                setEditing(undefined);
                setCreateParentID(record.id);
                setDrawerOpen(true);
              }}
            >
              新增子菜单
            </a>,
          );
        }
        actions.push(
          <a
            key="detail"
            onClick={() => {
              void readMenu(record.id).then(setDetail);
            }}
          >
            详情
          </a>,
        );
        if (access.canMenuUpdate) {
          actions.push(
            <a
              key="edit"
              onClick={() => {
                setEditing(record);
                setDrawerOpen(true);
              }}
            >
              编辑
            </a>,
          );
        }
        if (access.canMenuDelete) {
          actions.push(
            <Popconfirm
              key="delete"
              title="删除菜单"
              description={`确认删除 ${record.name}？`}
              okText="删除"
              okButtonProps={{ danger: true }}
              onConfirm={async () => {
                await deleteMenu(record.id);
                message.success('菜单已删除');
                actionRef.current?.reload();
              }}
            >
              <Button type="link" danger size="small">
                删除
              </Button>
            </Popconfirm>,
          );
        }
        return actions;
      },
    },
  ];

  const permissionOptions = permissions.map((permission) => ({
    label: `${permission.name} (${permission.token})`,
    value: permission.token,
  }));

  return (
    <PageContainer
      title="菜单管理"
      subTitle="菜单的可见性由角色的功能权限自动派生：绑定权限后，拥有该权限的角色即可看到此菜单"
    >
      <ProTable<MenuNode>
        headerTitle="菜单树"
        rowKey="id"
        actionRef={actionRef}
        search={false}
        pagination={false}
        columns={columns}
        expandable={{
          expandedRowKeys,
          onExpandedRowsChange: (keys) =>
            setExpandedRowKeys(keys.map((key) => Number(key))),
        }}
        request={async () => {
          const data = await listMenus();
          setMenus(data);
          setExpandedRowKeys(
            data
              .filter((menu) =>
                data.some((child) => child.parent_id === menu.id),
              )
              .map((menu) => menu.id),
          );
          const tree = buildMenuTree(data);
          return { data: tree, success: true, total: data.length };
        }}
        toolBarRender={() =>
          access.canMenuCreate
            ? [
                <Button
                  key="create"
                  type="primary"
                  icon={<PlusOutlined />}
                  onClick={() => {
                    setEditing(undefined);
                    setCreateParentID(undefined);
                    setDrawerOpen(true);
                  }}
                >
                  新增菜单
                </Button>,
              ]
            : []
        }
      />
      <DrawerForm<MenuFormValues>
        key={
          editing ? `edit-${editing.id}` : `create-${createParentID ?? 'root'}`
        }
        title={
          editing ? '编辑菜单' : createParentID ? '新增子菜单' : '新增菜单'
        }
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        width="min(640px, 100%)"
        grid
        drawerProps={{ destroyOnHidden: true }}
        initialValues={
          editing
            ? {
                parent_id: editing.parent_id,
                name: editing.name,
                path: editing.path,
                icon: editing.icon,
                hidden: editing.hidden,
                component: editing.component,
                meta: editing.meta,
                permission: editing.permission,
                sort: editing.sort,
                active: editing.active,
                buttons: editing.buttons.map((button) => ({
                  id: button.id,
                  name: button.name,
                  description: button.description,
                })),
              }
            : {
                active: true,
                hidden: false,
                parent_id: createParentID ?? 0,
                sort: 100,
                meta: {
                  keep_alive: false,
                  default_menu: false,
                  close_tab: false,
                },
                buttons: [],
              }
        }
        onFinish={async (values) => {
          const body = {
            parent_id: values.parent_id,
            name: values.name,
            path: values.path,
            icon: values.icon,
            hidden: values.hidden,
            component: values.component,
            meta: {
              active_name: values.meta?.active_name,
              keep_alive: values.meta?.keep_alive ?? false,
              default_menu: values.meta?.default_menu ?? false,
              close_tab: values.meta?.close_tab ?? false,
              transition_type: values.meta?.transition_type,
            },
            permission: values.permission,
            sort: values.sort,
            active: values.active,
            buttons: values.buttons ?? [],
          };
          if (editing) {
            await updateMenu(editing.id, body);
            message.success('菜单已更新');
          } else {
            await createMenu(body);
            message.success('菜单已创建');
          }
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProForm.Group title="基本信息" grid>
          <ProFormText
            name="name"
            label="名称"
            colProps={{ xs: 24, sm: 12 }}
            fieldProps={{ maxLength: 80 }}
            rules={[{ required: true, message: '请输入菜单名称' }]}
          />
          <ProFormText
            name="path"
            label="路径"
            colProps={{ xs: 24, sm: 12 }}
            fieldProps={{ maxLength: 160 }}
            rules={[{ required: true, message: '请输入菜单路径' }]}
          />
          <ProFormText
            name="component"
            label="组件"
            colProps={{ xs: 24, sm: 12 }}
            fieldProps={{ maxLength: 160 }}
            rules={[{ required: true, message: '请输入组件路径' }]}
          />
          <ProFormDigit
            name="sort"
            label="排序"
            colProps={{ xs: 24, sm: 12 }}
            min={0}
          />
          <ProFormTreeSelect
            name="parent_id"
            label="上级菜单"
            colProps={{ span: 24 }}
            fieldProps={{
              treeData: toMenuTreeNodes(menus, editing?.id),
              treeDefaultExpandAll: true,
            }}
          />
        </ProForm.Group>
        <ProForm.Group title="权限绑定" grid>
          <ProFormSelect
            name="permission"
            label="绑定权限"
            colProps={{ span: 24 }}
            tooltip="角色勾选该权限后自动看到此菜单；容器菜单无需绑定"
            options={permissionOptions}
            fieldProps={{ allowClear: true, showSearch: true }}
          />
        </ProForm.Group>
        <ProForm.Group title="显示行为" grid>
          <ProFormSwitch
            name="hidden"
            label="隐藏"
            colProps={{ xs: 12, md: 8 }}
          />
          <ProFormSwitch
            name={['meta', 'keep_alive']}
            label="缓存"
            colProps={{ xs: 12, md: 8 }}
          />
          <ProFormSwitch
            name={['meta', 'default_menu']}
            label="默认菜单"
            colProps={{ xs: 12, md: 8 }}
          />
          <ProFormSwitch
            name={['meta', 'close_tab']}
            label="允许关闭"
            colProps={{ xs: 12, md: 8 }}
          />
          <ProFormSwitch
            name="active"
            label="启用"
            colProps={{ xs: 12, md: 8 }}
          />
        </ProForm.Group>
        <Collapse
          ghost
          items={[
            {
              key: 'advanced',
              label: '高级设置（图标 / 激活菜单名 / 切换动画）',
              children: (
                <ProForm.Group grid>
                  <ProFormText
                    name="icon"
                    label="图标"
                    colProps={{ xs: 24, sm: 12 }}
                    fieldProps={{ maxLength: 80 }}
                  />
                  <ProFormText
                    name={['meta', 'active_name']}
                    label="激活菜单名"
                    colProps={{ xs: 24, sm: 12 }}
                    fieldProps={{ maxLength: 160 }}
                  />
                  <ProFormText
                    name={['meta', 'transition_type']}
                    label="切换动画"
                    colProps={{ xs: 24, sm: 12 }}
                    fieldProps={{ maxLength: 80 }}
                  />
                </ProForm.Group>
              ),
            },
          ]}
        />
        <ProFormList
          name="buttons"
          label="菜单按钮"
          creatorButtonProps={{ creatorButtonText: '添加按钮' }}
        >
          <ProForm.Group key="button-row" grid>
            {/* 保留已有按钮的 id，后端按 id 识别是更新还是新增。 */}
            <Form.Item name="id" hidden key="button-id">
              <InputNumber />
            </Form.Item>
            <ProFormText
              name="name"
              label="按钮 key"
              colProps={{ xs: 24, sm: 12 }}
              fieldProps={{ maxLength: 80 }}
              rules={[{ required: true, message: '请输入按钮 key' }]}
            />
            <ProFormText
              name="description"
              label="按钮说明"
              colProps={{ xs: 24, sm: 12 }}
              fieldProps={{ maxLength: 120 }}
            />
          </ProForm.Group>
        </ProFormList>
      </DrawerForm>
      <Drawer
        title="菜单详情"
        width={520}
        open={Boolean(detail)}
        onClose={() => setDetail(undefined)}
      >
        {detail && (
          <ProDescriptions<Menu>
            column={1}
            size="small"
            dataSource={detail}
            columns={[
              { title: '名称', dataIndex: 'name' },
              { title: '路径', dataIndex: 'path' },
              { title: '组件', dataIndex: 'component' },
              {
                title: '上级',
                dataIndex: 'parent_id',
                render: (_, entity) =>
                  entity.parent_id === 0
                    ? '顶级菜单'
                    : menuName(entity.parent_id),
              },
              {
                title: '绑定权限',
                dataIndex: 'permission',
                render: (_, entity) => permissionDetail(entity.permission),
              },
              { title: '排序', dataIndex: 'sort' },
              {
                title: '状态',
                dataIndex: 'active',
                render: (_, entity) => (
                  <>
                    <Tag color={entity.hidden ? 'default' : 'blue'}>
                      {entity.hidden ? '隐藏' : '显示'}
                    </Tag>
                    <Tag color={entity.active ? 'green' : 'default'}>
                      {entity.active ? '启用' : '停用'}
                    </Tag>
                  </>
                ),
              },
              {
                title: '高级设置',
                dataIndex: ['meta', 'keep_alive'],
                render: (_, entity) =>
                  `缓存 ${entity.meta.keep_alive ? '开' : '关'} · 默认菜单 ${
                    entity.meta.default_menu ? '是' : '否'
                  } · 允许关闭 ${entity.meta.close_tab ? '是' : '否'}${
                    entity.meta.transition_type
                      ? ` · 过渡 ${entity.meta.transition_type}`
                      : ''
                  }${entity.icon ? ` · 图标 ${entity.icon}` : ''}`,
              },
              {
                title: '按钮',
                dataIndex: 'buttons',
                render: (_, entity) =>
                  entity.buttons
                    .map((button) => button.description || button.name)
                    .join('、') || '-',
              },
            ]}
          />
        )}
      </Drawer>
    </PageContainer>
  );
};

export default Menus;
