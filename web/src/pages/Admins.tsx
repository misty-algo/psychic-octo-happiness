import { PlusOutlined } from '@ant-design/icons';
import {
  type ActionType,
  DrawerForm,
  PageContainer,
  type ProColumns,
  ProForm,
  ProFormDependency,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess, useModel } from '@umijs/max';
import { Button, message, Popconfirm, Space, Tag } from 'antd';
import React, { useEffect, useRef, useState } from 'react';

import {
  type AdminUser,
  createAdmin,
  deleteAdmin,
  listAdmins,
  listRoles,
  pageParams,
  type Role,
  toTableResult,
  updateAdmin,
} from '@/services/admin';

type AdminFormValues = {
  username: string;
  display_name: string;
  email?: string;
  password?: string;
  role_ids: number[];
  active_role_id?: number;
  active: boolean;
};

const formatDate = (value?: string) =>
  value ? new Date(value).toLocaleString() : '-';

const Admins: React.FC = () => {
  const access = useAccess();
  const { initialState } = useModel('@@initialState');
  const currentAdminID = initialState?.currentUser?.id;
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [roles, setRoles] = useState<Role[]>([]);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<AdminUser>();

  useEffect(() => {
    void listRoles({ page_size: 100 }).then((response) => {
      setRoles(response.data);
    });
  }, []);

  const roleName = (roleID: number) =>
    roles.find((role) => role.id === roleID)?.name ?? `#${roleID}`;

  const roleOptions = roles.map((role) => ({
    label: role.name,
    value: role.id,
  }));

  const columns: ProColumns<AdminUser>[] = [
    { title: '用户名', dataIndex: 'username', width: 140 },
    { title: '显示名', dataIndex: 'display_name', width: 140 },
    {
      title: '邮箱',
      dataIndex: 'email',
      ellipsis: true,
      render: (_, record) => record.email || '-',
    },
    {
      title: '角色',
      dataIndex: 'role_ids',
      render: (_, record) => (
        <Space wrap size={[4, 4]}>
          {record.role_ids.map((roleID) =>
            roleID === record.active_role_id ? (
              <Tag key={roleID} color="blue">
                {roleName(roleID)}
              </Tag>
            ) : (
              <Tag key={roleID}>{roleName(roleID)}</Tag>
            ),
          )}
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'active',
      width: 80,
      render: (_, record) => (
        <Tag color={record.active ? 'green' : 'default'}>
          {record.active ? '启用' : '停用'}
        </Tag>
      ),
    },
    {
      title: '更新时间',
      dataIndex: 'updated_at',
      width: 170,
      render: (_, record) => formatDate(record.updated_at),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 120,
      render: (_, record) => {
        const actions: React.ReactNode[] = [];
        const isSelf = record.id === currentAdminID;
        if (access.canAdminUpdate) {
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
        if (access.canAdminDelete && !isSelf) {
          actions.push(
            <Popconfirm
              key="delete"
              title="删除管理员"
              description={`确认删除 ${record.username}？`}
              okText="删除"
              okButtonProps={{ danger: true }}
              onConfirm={async () => {
                await deleteAdmin(record.id);
                message.success('管理员已删除');
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

  return (
    <PageContainer title="管理员管理">
      <ProTable<AdminUser>
        headerTitle="管理员列表"
        rowKey="id"
        actionRef={actionRef}
        search={false}
        columns={columns}
        request={async (params) =>
          toTableResult(await listAdmins(pageParams(params)))
        }
        toolBarRender={() =>
          access.canAdminCreate
            ? [
                <Button
                  key="create"
                  type="primary"
                  icon={<PlusOutlined />}
                  onClick={() => {
                    setEditing(undefined);
                    setDrawerOpen(true);
                  }}
                >
                  新增管理员
                </Button>,
              ]
            : []
        }
      />
      <DrawerForm<AdminFormValues>
        key={editing?.id ?? 'create'}
        title={editing ? '编辑管理员' : '新增管理员'}
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        width="min(720px, 100%)"
        grid
        drawerProps={{ destroyOnHidden: true }}
        initialValues={
          editing
            ? {
                username: editing.username,
                display_name: editing.display_name,
                email: editing.email,
                role_ids: editing.role_ids,
                active_role_id: editing.active_role_id,
                active: editing.active,
              }
            : { active: true, role_ids: [] }
        }
        onFinish={async (values) => {
          if (editing) {
            const password = values.password?.trim();
            await updateAdmin(editing.id, {
              display_name: values.display_name,
              email: values.email,
              role_ids: values.role_ids,
              active_role_id: values.active_role_id,
              active: values.active,
              ...(password ? { password } : {}),
            });
            message.success('管理员已更新');
          } else {
            await createAdmin({
              username: values.username,
              display_name: values.display_name,
              email: values.email,
              password: values.password ?? '',
              role_ids: values.role_ids,
              active_role_id: values.active_role_id,
              active: values.active,
            });
            message.success('管理员已创建');
          }
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProForm.Group title="基本信息" grid>
          <ProFormText
            name="username"
            label="用户名"
            colProps={{ xs: 24, md: 12 }}
            disabled={Boolean(editing)}
            fieldProps={{ maxLength: 64 }}
            rules={editing ? [] : [{ required: true, message: '请输入用户名' }]}
          />
          <ProFormText
            name="display_name"
            label="显示名"
            colProps={{ xs: 24, md: 12 }}
            fieldProps={{ maxLength: 80 }}
            rules={[{ required: true, message: '请输入显示名' }]}
          />
          <ProFormText
            name="email"
            label="邮箱"
            colProps={{ xs: 24, md: 12 }}
            fieldProps={{ maxLength: 160 }}
            rules={[{ type: 'email', message: '请输入有效邮箱' }]}
          />
          <ProFormText.Password
            name="password"
            label={editing ? '新密码' : '密码'}
            colProps={{ xs: 24, md: 12 }}
            fieldProps={{ maxLength: 72 }}
            rules={[{ required: !editing, min: 8, message: '密码至少 8 位' }]}
          />
          <ProFormSwitch
            name="active"
            label="启用"
            colProps={{ xs: 24, md: 12 }}
            // 编辑自己时禁止停用：当前会话需要可用的账号所有者。
            disabled={editing?.id === currentAdminID}
          />
        </ProForm.Group>
        <ProForm.Group title="角色分配" grid>
          <ProFormSelect
            name="role_ids"
            label="角色"
            colProps={{ span: 24 }}
            mode="multiple"
            options={roleOptions}
            fieldProps={{ maxTagCount: 'responsive' }}
            rules={[{ required: true, message: '请选择角色' }]}
          />
          <ProFormDependency name={['role_ids']}>
            {({ role_ids }) => (
              <ProFormSelect
                name="active_role_id"
                label="登录后生效的当前角色"
                colProps={{ span: 24 }}
                tooltip="管理员可持有多个角色，登录会话使用当前角色进行授权"
                options={roleOptions.filter((option) =>
                  role_ids?.includes(option.value),
                )}
                disabled={(role_ids ?? []).length === 0}
                rules={[{ required: true, message: '请选择当前角色' }]}
              />
            )}
          </ProFormDependency>
        </ProForm.Group>
      </DrawerForm>
    </PageContainer>
  );
};

export default Admins;
