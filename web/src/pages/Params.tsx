import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import {
  type ActionType,
  ModalForm,
  PageContainer,
  type ProColumns,
  ProDescriptions,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import { Button, Drawer, message, Popconfirm } from 'antd';
import React, { useRef, useState } from 'react';

import {
  batchDeleteParams,
  createParam,
  deleteParam,
  listParams,
  type ParamInput,
  pageParams,
  readParam,
  type SystemParam,
  toTableResult,
  updateParam,
} from '@/services/admin';

const formatDate = (value?: string) =>
  value ? new Date(value).toLocaleString() : '-';

const Params: React.FC = () => {
  const access = useAccess();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<SystemParam>();
  const [detail, setDetail] = useState<SystemParam>();
  const [selectedIDs, setSelectedIDs] = useState<React.Key[]>([]);

  const columns: ProColumns<SystemParam>[] = [
    { title: '名称', dataIndex: 'name' },
    { title: '键', dataIndex: 'key' },
    { title: '值', dataIndex: 'value', ellipsis: true, search: false },
    { title: '说明', dataIndex: 'desc', ellipsis: true, search: false },
    {
      title: '更新时间',
      dataIndex: 'updated_at',
      search: false,
      render: (_, record) => formatDate(record.updated_at),
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_, record) => {
        const actions: React.ReactNode[] = [];
        actions.push(
          <a
            key="detail"
            onClick={() => {
              void readParam(record.id).then(setDetail);
            }}
          >
            详情
          </a>,
        );
        if (access.canParamUpdate) {
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
        if (access.canParamDelete) {
          actions.push(
            <Popconfirm
              key="delete"
              title="删除参数"
              description={`确认删除 ${record.key}？`}
              okText="删除"
              okButtonProps={{ danger: true }}
              onConfirm={async () => {
                await deleteParam(record.id);
                message.success('参数已删除');
                setSelectedIDs((previous) =>
                  previous.filter((id) => id !== record.id),
                );
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
    <PageContainer title="系统参数">
      <ProTable<SystemParam>
        headerTitle="参数列表"
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        rowSelection={
          access.canParamDelete
            ? {
                selectedRowKeys: selectedIDs,
                onChange: (keys) => setSelectedIDs(keys),
              }
            : false
        }
        request={async (params) =>
          toTableResult(
            await listParams({
              ...pageParams(params),
              name: params.name || undefined,
              key: params.key || undefined,
            }),
          )
        }
        toolBarRender={() => {
          const buttons: React.ReactNode[] = [];
          if (access.canParamCreate) {
            buttons.push(
              <Button
                key="create"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
                  setEditing(undefined);
                  setModalOpen(true);
                }}
              >
                新增参数
              </Button>,
            );
          }
          if (access.canParamDelete && selectedIDs.length > 0) {
            buttons.push(
              <Popconfirm
                key="batch-delete"
                title="批量删除参数"
                description={`确认删除选中的 ${selectedIDs.length} 条参数？`}
                okText="删除"
                okButtonProps={{ danger: true }}
                onConfirm={async () => {
                  await batchDeleteParams(selectedIDs.map(Number));
                  message.success('参数已批量删除');
                  setSelectedIDs([]);
                  actionRef.current?.reload();
                }}
              >
                <Button danger icon={<DeleteOutlined />}>
                  批量删除
                </Button>
              </Popconfirm>,
            );
          }
          return buttons;
        }}
      />
      <ModalForm<ParamInput>
        key={editing?.id ?? 'create'}
        title={editing ? '编辑参数' : '新增参数'}
        open={modalOpen}
        onOpenChange={setModalOpen}
        modalProps={{ destroyOnHidden: true }}
        initialValues={editing ?? { name: '', key: '', value: '', desc: '' }}
        onFinish={async (values) => {
          if (editing) {
            await updateParam(editing.id, values);
            message.success('参数已更新');
          } else {
            await createParam(values);
            message.success('参数已创建');
          }
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText
          name="name"
          label="名称"
          fieldProps={{ maxLength: 120 }}
          rules={[{ required: true, message: '请输入参数名称' }]}
        />
        <ProFormText
          name="key"
          label="键"
          fieldProps={{ maxLength: 80 }}
          rules={[{ required: true, message: '请输入参数键' }]}
        />
        <ProFormTextArea
          name="value"
          label="值"
          fieldProps={{ maxLength: 4000, rows: 4 }}
          rules={[{ required: true, message: '请输入参数值' }]}
        />
        <ProFormTextArea
          name="desc"
          label="说明"
          fieldProps={{ maxLength: 4000, rows: 4 }}
        />
      </ModalForm>
      <Drawer
        title="参数详情"
        width={480}
        open={Boolean(detail)}
        onClose={() => setDetail(undefined)}
      >
        {detail && (
          <ProDescriptions<SystemParam>
            column={1}
            size="small"
            dataSource={detail}
            columns={[
              { title: '名称', dataIndex: 'name' },
              { title: '键', dataIndex: 'key' },
              { title: '值', dataIndex: 'value' },
              {
                title: '说明',
                dataIndex: 'desc',
                render: (_, entity) => entity.desc || '-',
              },
              {
                title: '创建时间',
                dataIndex: 'created_at',
                render: (_, entity) => formatDate(entity.created_at),
              },
              {
                title: '更新时间',
                dataIndex: 'updated_at',
                render: (_, entity) => formatDate(entity.updated_at),
              },
            ]}
          />
        )}
      </Drawer>
    </PageContainer>
  );
};

export default Params;
