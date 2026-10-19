import {
  CheckOutlined,
  LockOutlined,
  LogoutOutlined,
  SkinOutlined,
  SwapOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { history, useModel } from '@umijs/max';
import type { MenuProps } from 'antd';
import { Form, Input, message, Modal, Spin } from 'antd';
import React, { startTransition, useState } from 'react';
import {
  changePassword,
  logout,
  switchRole,
  updateCurrentUserProfile,
} from '@/services/admin';
import HeaderDropdown from '../HeaderDropdown';

type GlobalHeaderRightProps = {
  children?: React.ReactNode;
};

type PasswordFormValues = {
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
};

type ProfileFormValues = {
  displayName: string;
  email?: string;
};

export const AvatarDropdown: React.FC<GlobalHeaderRightProps> = ({
  children,
}) => {
  const [profileOpen, setProfileOpen] = useState(false);
  const [profileSaving, setProfileSaving] = useState(false);
  const [passwordOpen, setPasswordOpen] = useState(false);
  const [passwordSaving, setPasswordSaving] = useState(false);
  const [profileForm] = Form.useForm<ProfileFormValues>();
  const [passwordForm] = Form.useForm<PasswordFormValues>();
  const { initialState, setInitialState } = useModel('@@initialState');

  const loginOut = async () => {
    await logout();
    history.replace('/user/login');
  };

  const submitProfile = async () => {
    const values = await profileForm.validateFields();
    setProfileSaving(true);
    try {
      const user = await updateCurrentUserProfile({
        display_name: values.displayName,
        email: values.email,
      });
      startTransition(() => {
        setInitialState((s) => ({ ...s, currentUser: user }));
      });
      message.success('个人资料已更新');
      setProfileOpen(false);
    } finally {
      setProfileSaving(false);
    }
  };

  const submitPassword = async () => {
    const values = await passwordForm.validateFields();
    setPasswordSaving(true);
    try {
      await changePassword({
        current_password: values.currentPassword,
        new_password: values.newPassword,
      });
      message.success('密码已更新');
      setPasswordOpen(false);
      passwordForm.resetFields();
    } finally {
      setPasswordSaving(false);
    }
  };

  const onMenuClick: MenuProps['onClick'] = async (event) => {
    const { key } = event;
    if (key === 'logout') {
      startTransition(() => {
        setInitialState((s) => ({ ...s, currentUser: undefined }));
      });
      await loginOut();
      return;
    }
    if (key === 'theme') {
      setInitialState((s) => ({ ...s, settingDrawerOpen: true }));
      return;
    }
    if (key === 'profile') {
      const user = initialState?.currentUser;
      if (!user) {
        return;
      }
      profileForm.setFieldsValue({
        displayName: user.display_name,
        email: user.email,
      });
      setProfileOpen(true);
      return;
    }
    if (key === 'password') {
      setPasswordOpen(true);
      return;
    }
    if (typeof key === 'string' && key.startsWith('role:')) {
      const roleID = Number(key.slice('role:'.length));
      if (!Number.isFinite(roleID)) return;
      const result = await switchRole(roleID);
      startTransition(() => {
        setInitialState((s) => ({ ...s, currentUser: result.user }));
      });
      message.success('角色已切换');
      history.replace(result.user.default_path || '/dashboard');
    }
  };

  if (!initialState) {
    return <Spin size="small" />;
  }

  const { currentUser } = initialState;

  if (!currentUser) {
    return <Spin size="small" />;
  }

  const roleItems: MenuProps['items'] = currentUser.roles.map((role) => ({
    key: `role:${role.id}`,
    icon:
      role.id === currentUser.active_role_id ? <CheckOutlined /> : undefined,
    label: role.name,
    disabled: role.id === currentUser.active_role_id,
  }));

  const menuItems: MenuProps['items'] = [
    {
      key: 'roles',
      icon: <SwapOutlined />,
      label: currentUser.active_role?.name ?? '切换角色',
      children: roleItems,
    },
    {
      type: 'divider' as const,
    },
    {
      key: 'theme',
      icon: <SkinOutlined />,
      label: '主题设置',
    },
    {
      key: 'profile',
      icon: <UserOutlined />,
      label: '个人资料',
    },
    {
      key: 'password',
      icon: <LockOutlined />,
      label: '修改密码',
    },
    {
      type: 'divider' as const,
    },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
    },
  ];

  return (
    <>
      <HeaderDropdown
        placement="bottomRight"
        menu={{
          selectedKeys: [],
          onClick: onMenuClick,
          items: menuItems,
        }}
        arrow
      >
        {children}
      </HeaderDropdown>
      <Modal
        title="个人资料"
        open={profileOpen}
        confirmLoading={profileSaving}
        onCancel={() => setProfileOpen(false)}
        onOk={() => void submitProfile()}
      >
        <Form<ProfileFormValues> form={profileForm} layout="vertical">
          <Form.Item
            label="展示名"
            name="displayName"
            rules={[
              { required: true, message: '请输入展示名' },
              { max: 80, message: '展示名最多 80 个字符' },
            ]}
          >
            <Input maxLength={80} autoComplete="name" />
          </Form.Item>
          <Form.Item
            label="邮箱"
            name="email"
            rules={[
              { type: 'email', message: '邮箱格式不正确' },
              { max: 160, message: '邮箱最多 160 个字符' },
            ]}
          >
            <Input maxLength={160} autoComplete="email" />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        title="修改密码"
        open={passwordOpen}
        confirmLoading={passwordSaving}
        onCancel={() => {
          setPasswordOpen(false);
          passwordForm.resetFields();
        }}
        onOk={() => void submitPassword()}
      >
        <Form<PasswordFormValues> form={passwordForm} layout="vertical">
          <Form.Item
            label="当前密码"
            name="currentPassword"
            rules={[{ required: true, message: '请输入当前密码' }]}
          >
            <Input.Password maxLength={72} autoComplete="current-password" />
          </Form.Item>
          <Form.Item
            label="新密码"
            name="newPassword"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 8, message: '密码至少 8 位' },
            ]}
          >
            <Input.Password maxLength={72} autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            label="确认新密码"
            name="confirmPassword"
            dependencies={['newPassword']}
            rules={[
              { required: true, message: '请再次输入新密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('newPassword') === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error('两次输入的密码不一致'));
                },
              }),
            ]}
          >
            <Input.Password maxLength={72} autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
};
