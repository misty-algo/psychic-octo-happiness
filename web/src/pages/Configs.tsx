import { PlusOutlined } from '@ant-design/icons';
import {
  type ActionType,
  ModalForm,
  PageContainer,
  type ProColumns,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import { Button, message, Popconfirm, Tag } from 'antd';
import React, { useRef, useState } from 'react';

import {
  deleteConfig,
  listConfigs,
  type SystemConfig,
  upsertConfig,
} from '@/services/admin';

type ConfigFormValues = {
  key: string;
  name: string;
  value: string;
  public: boolean;
};

const Configs: React.FC = () => {
  const access = useAccess();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<SystemConfig>();

  const columns: ProColumns<SystemConfig>[] = [
    { title: '键', dataIndex: 'key' },
    { title: '名称', dataIndex: 'name' },
    { title: '值', dataIndex: 'value', ellipsis: true },
    {
      title: '公开',
      dataIndex: 'public',
      render: (_, record) => (
        <Tag color={record.public ? 'green' : 'default'}>
          {record.public ? '是' : '否'}
        </Tag>
      ),
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_, record) => {
        const actions: React.ReactNode[] = [];
        if (access.canConfigUpdate) {
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
        // site_name 是站点基础配置，禁止删除。
        if (access.canConfigDelete && record.key !== 'site_name') {
          actions.push(
            <Popconfirm
              key="delete"
              title="删除配置"
              description={`确认删除 ${record.key}？`}
              okText="删除"
              okButtonProps={{ danger: true }}
              onConfirm={async () => {
                await deleteConfig(record.key);
                message.success('配置已删除');
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
    <PageContainer title="系统配置">
      <ProTable<SystemConfig>
        headerTitle="配置列表"
        rowKey="key"
        actionRef={actionRef}
        search={false}
        pagination={false}
        columns={columns}
        request={async () => {
          const data = await listConfigs();
          return { data, success: true, total: data.length };
        }}
        toolBarRender={() =>
          access.canConfigUpdate
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
                  新增配置
                </Button>,
              ]
            : []
        }
      />
      <ModalForm<ConfigFormValues>
        key={editing?.key ?? 'create'}
        title={editing ? '编辑配置' : '新增配置'}
        open={modalOpen}
        onOpenChange={setModalOpen}
        modalProps={{ destroyOnHidden: true }}
        initialValues={editing ?? { public: false, value: '' }}
        onFinish={async (values) => {
          await upsertConfig(values.key, {
            name: values.name,
            value: values.value,
            public: values.public,
          });
          message.success(editing ? '配置已更新' : '配置已创建');
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText
          name="key"
          label="键"
          disabled={Boolean(editing)}
          fieldProps={{ maxLength: 120 }}
          rules={[{ required: true, message: '请输入配置键' }]}
        />
        <ProFormText
          name="name"
          label="名称"
          fieldProps={{ maxLength: 120 }}
          rules={[{ required: true, message: '请输入配置名称' }]}
        />
        <ProFormTextArea
          name="value"
          label="值"
          fieldProps={{ maxLength: 4000, rows: 4 }}
        />
        <ProFormSwitch name="public" label="公开" />
      </ModalForm>
    </PageContainer>
  );
};

export default Configs;
