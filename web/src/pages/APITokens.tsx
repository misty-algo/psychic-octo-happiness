import { PlusOutlined } from '@ant-design/icons';
import {
  type ActionType,
  ModalForm,
  PageContainer,
  type ProColumns,
  ProFormDateTimePicker,
  ProFormDependency,
  ProFormDigit,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess, useModel } from '@umijs/max';
import { Button, Modal, message, Popconfirm, Tag, Typography } from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';

import {
  type AdminUser,
  type APIToken,
  type APITokenInput,
  createAPIToken,
  deleteAPIToken,
  listAdmins,
  listAPITokens,
  listRoles,
  pageParams,
  type Role,
  toTableResult,
  updateAPIToken,
} from '@/services/admin';

type TokenFormValues = {
  admin_id?: number;
  role_id?: number;
  name: string;
  description?: string;
  active: boolean;
  days?: number;
  expires_at?: Dayjs | null;
};

const formatDate = (value?: string | null) =>
  value ? new Date(value).toLocaleString() : '-';

const toInput = (values: TokenFormValues, editing: boolean): APITokenInput => ({
  admin_id: editing ? undefined : values.admin_id,
  role_id: editing ? undefined : values.role_id,
  name: values.name,
  description: values.description,
  active: values.active,
  days: editing ? undefined : values.days,
  expires_at: editing
    ? values.expires_at
      ? values.expires_at.toISOString()
      : null
    : undefined,
});

const APITokens: React.FC = () => {
  const access = useAccess();
  const { initialState } = useModel('@@initialState');
  const currentUser = initialState?.currentUser;
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [admins, setAdmins] = useState<AdminUser[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<APIToken>();

  // 无 admin:read/role:read 权限时，可选项退化为当前登录用户自身及其角色。
  useEffect(() => {
    const fallbackAdmins: AdminUser[] = currentUser
      ? [
          {
            id: currentUser.id,
            username: currentUser.username,
            display_name: currentUser.display_name,
            email: currentUser.email,
            role_ids: currentUser.roles.map((role) => role.id),
            active_role_id:
              currentUser.active_role?.id ?? currentUser.roles[0]?.id ?? 0,
            active: true,
            created_at: '',
            updated_at: '',
          },
        ]
      : [];
    void Promise.all([
      access.canAdminRead
        ? listAdmins({ page_size: 100 })
        : Promise.resolve({ data: fallbackAdmins }),
      access.canRoleRead
        ? listRoles({ page_size: 100 })
        : Promise.resolve({ data: currentUser?.roles ?? [] }),
    ]).then(([adminResponse, roleResponse]) => {
      setAdmins(adminResponse.data);
      setRoles(roleResponse.data);
    });
  }, [access, currentUser]);

  const adminName = (adminID: number) => {
    const admin = admins.find((item) => item.id === adminID);
    return admin ? `${admin.display_name}(${admin.username})` : `#${adminID}`;
  };

  const roleName = (roleID: number) =>
    roles.find((role) => role.id === roleID)?.name ?? `#${roleID}`;

  const showCreatedSecret = (secret: string) => {
    Modal.info({
      title: 'API Token已创建',
      width: 640,
      content: (
        <Typography.Paragraph
          copyable={{ text: secret }}
          style={{ marginTop: 8, marginBottom: 0 }}
        >
          {secret}
        </Typography.Paragraph>
      ),
    });
  };

  const columns: ProColumns<APIToken>[] = [
    { title: '名称', dataIndex: 'name' },
    { title: '前缀', dataIndex: 'prefix', width: 140 },
    {
      title: '管理员',
      dataIndex: 'admin_id',
      width: 180,
      render: (_, record) => adminName(record.admin_id),
    },
    {
      title: '角色',
      dataIndex: 'role_id',
      width: 140,
      render: (_, record) => roleName(record.role_id),
    },
    {
      title: '状态',
      dataIndex: 'active',
      width: 96,
      render: (_, record) => (
        <Tag color={record.active ? 'green' : 'default'}>
          {record.active ? '启用' : '停用'}
        </Tag>
      ),
    },
    {
      title: '过期时间',
      dataIndex: 'expires_at',
      render: (_, record) => formatDate(record.expires_at),
    },
    {
      title: '最近使用',
      dataIndex: 'last_used_at',
      render: (_, record) => formatDate(record.last_used_at),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 140,
      render: (_, record) => {
        const actions: React.ReactNode[] = [];
        if (access.canApiTokenUpdate) {
          actions.push(
            <a
              key="edit"
              onClick={() => {
                setEditing(record);
                setModalOpen(true);
              }}
            >
              编辑
            </a>,
          );
        }
        if (access.canApiTokenDelete) {
          actions.push(
            <Popconfirm
              key="delete"
              title="作废API Token"
              description={`确认作废 ${record.name}？`}
              okText="作废"
              okButtonProps={{ danger: true }}
              onConfirm={async () => {
                await deleteAPIToken(record.id);
                message.success('API Token已作废');
                actionRef.current?.reload();
              }}
            >
              <Button type="link" danger size="small">
                作废
              </Button>
            </Popconfirm>,
          );
        }
        return actions;
      },
    },
  ];

  return (
    <PageContainer title="API Token">
      <ProTable<APIToken>
        headerTitle="Token列表"
        rowKey="id"
        actionRef={actionRef}
        search={false}
        columns={columns}
        request={async (params) =>
          toTableResult(await listAPITokens(pageParams(params)))
        }
        toolBarRender={() =>
          access.canApiTokenCreate
            ? [
                <Button
                  key="create"
                  type="primary"
                  icon={<PlusOutlined />}
                  onClick={() => {
                    setEditing(undefined);
                    setModalOpen(true);
                  }}
                >
                  新增Token
                </Button>,
              ]
            : []
        }
      />
      <ModalForm<TokenFormValues>
        key={editing?.id ?? 'create'}
        title={editing ? '编辑API Token' : '新增API Token'}
        open={modalOpen}
        onOpenChange={setModalOpen}
        modalProps={{ destroyOnHidden: true }}
        initialValues={
          editing
            ? {
                admin_id: editing.admin_id,
                role_id: editing.role_id,
                name: editing.name,
                description: editing.description,
                active: editing.active,
                expires_at: editing.expires_at
                  ? dayjs(editing.expires_at)
                  : null,
              }
            : {
                active: true,
                admin_id: currentUser?.id ?? admins[0]?.id,
                role_id:
                  currentUser?.active_role?.id ??
                  currentUser?.roles[0]?.id ??
                  roles[0]?.id,
                days: 30,
              }
        }
        onFinish={async (values) => {
          if (!editing) {
            // 角色必须属于所选管理员，否则后端会拒绝创建。
            const selected = admins.find(
              (admin) => admin.id === values.admin_id,
            );
            if (
              selected &&
              selected.role_ids.length > 0 &&
              !selected.role_ids.includes(values.role_id ?? 0)
            ) {
              message.error('所选角色不属于该管理员，请重新选择角色');
              return false;
            }
          }
          const input = toInput(values, Boolean(editing));
          if (editing) {
            await updateAPIToken(editing.id, input);
            message.success('API Token已更新');
          } else {
            const created = await createAPIToken(input);
            message.success('API Token已创建');
            showCreatedSecret(created.secret);
          }
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormSelect
          name="admin_id"
          label="管理员"
          disabled={Boolean(editing)}
          options={admins.map((admin) => ({
            value: admin.id,
            label: `${admin.display_name}(${admin.username})`,
          }))}
          rules={editing ? [] : [{ required: true, message: '请选择管理员' }]}
        />
        <ProFormDependency name={['admin_id']}>
          {({ admin_id }) => {
            const selected = admins.find((admin) => admin.id === admin_id);
            const options =
              selected && selected.role_ids.length > 0
                ? roles.filter((role) => selected.role_ids.includes(role.id))
                : roles;
            return (
              <ProFormSelect
                name="role_id"
                label="角色"
                disabled={Boolean(editing)}
                options={options.map((role) => ({
                  value: role.id,
                  label: role.name,
                }))}
                rules={
                  editing ? [] : [{ required: true, message: '请选择角色' }]
                }
              />
            );
          }}
        </ProFormDependency>
        <ProFormText
          name="name"
          label="名称"
          fieldProps={{ maxLength: 80 }}
          rules={[{ required: true, message: '请输入名称' }]}
        />
        <ProFormTextArea
          name="description"
          label="描述"
          fieldProps={{ maxLength: 240, rows: 3 }}
        />
        <ProFormSwitch name="active" label="启用" />
        {editing ? (
          <ProFormDateTimePicker
            name="expires_at"
            label="过期时间"
            fieldProps={{ showTime: true, style: { width: '100%' } }}
          />
        ) : (
          <ProFormDigit
            name="days"
            label="有效天数"
            min={1}
            max={365}
            fieldProps={{ style: { width: '100%' } }}
            rules={[{ required: true, message: '请输入有效天数' }]}
          />
        )}
      </ModalForm>
    </PageContainer>
  );
};

export default APITokens;
