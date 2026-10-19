import {
  type ActionType,
  ModalForm,
  PageContainer,
  type ProColumns,
  ProDescriptions,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import { Button, Drawer, message, Popconfirm, Tag } from 'antd';
import React, { useRef, useState } from 'react';

import {
  batchDeleteLoginLogs,
  batchDeleteOperationLogs,
  batchDeleteSystemErrorLogs,
  deleteLoginLog,
  deleteOperationLog,
  deleteSystemErrorLog,
  type Envelope,
  type ListParams,
  type LoginLog,
  listLoginLogs,
  listOperationLogs,
  listSystemErrorLogs,
  type OperationLog,
  pageParams,
  readLoginLog,
  readOperationLog,
  readSystemErrorLog,
  reopenSystemErrorLog,
  resolveSystemErrorLog,
  type SystemErrorLog,
  toTableResult,
} from '@/services/admin';

const formatDate = (value: string) => new Date(value).toLocaleString();

type EnvelopeRequest<T> = (params?: ListParams) => Promise<Envelope<T[]>>;

// 统一把后端 Envelope 分页响应映射成 ProTable request 需要的结构。
const envelopeRequest =
  <T,>(list: EnvelopeRequest<T>) =>
  async (params: { current?: number; pageSize?: number }) =>
    toTableResult(await list(pageParams(params)));

type DetailColumn = {
  title: string;
  dataIndex: string;
  render?: (dom: React.ReactNode, entity: any) => React.ReactNode;
};

type DetailState = {
  title: string;
  columns: DetailColumn[];
  detail?: Record<string, unknown>;
};

// 三个日志 Tab 共享同一套表格骨架，仅列定义和数据源不同。
type LogTableProps<T> = {
  columns: ProColumns<T>[];
  actionRef: React.RefObject<ActionType | undefined>;
  canDelete: boolean;
  batchDelete: (ids: number[]) => Promise<void>;
  batchDeleteButton: (
    description: string,
    onConfirm: () => Promise<void>,
  ) => React.ReactElement;
  list: EnvelopeRequest<T>;
};

function LogTableShell<T extends { id: number }>({
  columns,
  actionRef,
  canDelete,
  batchDelete,
  batchDeleteButton,
  list,
}: LogTableProps<T>) {
  const [selectedIDs, setSelectedIDs] = useState<React.Key[]>([]);
  return (
    <ProTable<T>
      rowKey="id"
      actionRef={actionRef}
      search={false}
      columns={columns}
      rowSelection={
        canDelete
          ? {
              selectedRowKeys: selectedIDs,
              onChange: (keys) => setSelectedIDs(keys),
            }
          : false
      }
      request={envelopeRequest(list)}
      toolBarRender={() =>
        canDelete && selectedIDs.length > 0
          ? [
              batchDeleteButton(
                `确认删除选中的 ${selectedIDs.length} 条记录？`,
                async () => {
                  await batchDelete(selectedIDs.map(Number));
                  setSelectedIDs([]);
                },
              ),
            ]
          : []
      }
    />
  );
}

const Logs: React.FC = () => {
  const access = useAccess();
  const operationActionRef = useRef<ActionType | undefined>(undefined);
  const loginActionRef = useRef<ActionType | undefined>(undefined);
  const errorActionRef = useRef<ActionType | undefined>(undefined);
  const [activeTab, setActiveTab] = useState('operations');
  const [detailState, setDetailState] = useState<DetailState>({
    title: '',
    columns: [],
  });
  const [resolving, setResolving] = useState<SystemErrorLog>();

  const deleteButton = (
    title: string,
    description: string,
    onConfirm: () => Promise<void>,
  ) => (
    <Popconfirm
      title={title}
      description={description}
      okText="删除"
      okButtonProps={{ danger: true }}
      onConfirm={onConfirm}
    >
      <Button type="link" danger size="small">
        删除
      </Button>
    </Popconfirm>
  );

  const batchDeleteButton = (
    description: string,
    onConfirm: () => Promise<void>,
  ) => (
    <Popconfirm
      title="批量删除"
      description={description}
      okText="删除"
      okButtonProps={{ danger: true }}
      onConfirm={onConfirm}
    >
      <Button danger>批量删除</Button>
    </Popconfirm>
  );

  const operationColumns: ProColumns<OperationLog>[] = [
    { title: '操作者', dataIndex: 'actor_id', width: 96 },
    { title: '动作', dataIndex: 'action', width: 96 },
    { title: '资源', dataIndex: 'resource', width: 120 },
    { title: '资源 ID', dataIndex: 'resource_id' },
    { title: '方法', dataIndex: 'method', width: 88 },
    { title: '路径', dataIndex: 'path' },
    { title: 'IP', dataIndex: 'ip', width: 140 },
    {
      title: '结果',
      dataIndex: 'success',
      render: (_, record) => (
        <Tag color={record.success ? 'green' : 'red'}>
          {record.success ? '成功' : '失败'}
        </Tag>
      ),
    },
    {
      title: '时间',
      dataIndex: 'created_at',
      render: (_, record) => formatDate(record.created_at),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 120,
      render: (_, record) => [
        <a
          key="detail"
          onClick={() => {
            void readOperationLog(record.id).then((detail) => {
              setDetailState({
                title: `操作日志 #${detail.id}`,
                columns: [
                  { title: '操作者', dataIndex: 'actor_id' },
                  { title: '动作', dataIndex: 'action' },
                  { title: '资源', dataIndex: 'resource' },
                  { title: '资源 ID', dataIndex: 'resource_id' },
                  { title: '方法', dataIndex: 'method' },
                  { title: '路径', dataIndex: 'path' },
                  { title: 'IP', dataIndex: 'ip' },
                  { title: 'User-Agent', dataIndex: 'user_agent' },
                  { title: '消息', dataIndex: 'message' },
                  {
                    title: '时间',
                    dataIndex: 'created_at',
                    render: (_, entity) => formatDate(entity.created_at),
                  },
                ],
                detail,
              });
            });
          }}
        >
          详情
        </a>,
        ...(access.canLogDelete
          ? [
              deleteButton(
                '删除操作日志',
                `确认删除 #${record.id}？`,
                async () => {
                  await deleteOperationLog(record.id);
                  message.success('操作日志已删除');
                  operationActionRef.current?.reload();
                },
              ),
            ]
          : []),
      ],
    },
  ];

  const loginColumns: ProColumns<LoginLog>[] = [
    { title: '管理员', dataIndex: 'admin_id', width: 96 },
    { title: '用户名', dataIndex: 'username' },
    { title: 'IP', dataIndex: 'ip' },
    {
      title: '结果',
      dataIndex: 'success',
      render: (_, record) => (
        <Tag color={record.success ? 'green' : 'red'}>
          {record.success ? '成功' : '失败'}
        </Tag>
      ),
    },
    {
      title: '原因',
      dataIndex: 'reason',
      render: (_, record) => record.reason || '-',
    },
    {
      title: '时间',
      dataIndex: 'created_at',
      render: (_, record) => formatDate(record.created_at),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 120,
      render: (_, record) => [
        <a
          key="detail"
          onClick={() => {
            void readLoginLog(record.id).then((detail) => {
              setDetailState({
                title: `登录日志 #${detail.id}`,
                columns: [
                  { title: '管理员', dataIndex: 'admin_id' },
                  { title: '用户名', dataIndex: 'username' },
                  { title: 'IP', dataIndex: 'ip' },
                  { title: 'User-Agent', dataIndex: 'user_agent' },
                  {
                    title: '原因',
                    dataIndex: 'reason',
                    render: (_, entity) => entity.reason || '-',
                  },
                  {
                    title: '时间',
                    dataIndex: 'created_at',
                    render: (_, entity) => formatDate(entity.created_at),
                  },
                ],
                detail,
              });
            });
          }}
        >
          详情
        </a>,
        ...(access.canLogDelete
          ? [
              deleteButton(
                '删除登录日志',
                `确认删除 #${record.id}？`,
                async () => {
                  await deleteLoginLog(record.id);
                  message.success('登录日志已删除');
                  loginActionRef.current?.reload();
                },
              ),
            ]
          : []),
      ],
    },
  ];

  const errorColumns: ProColumns<SystemErrorLog>[] = [
    { title: '代码', dataIndex: 'code', width: 96 },
    { title: '消息', dataIndex: 'message' },
    { title: '方法', dataIndex: 'method', width: 88 },
    { title: '路径', dataIndex: 'path' },
    {
      title: '用户',
      dataIndex: 'user_id',
      width: 96,
      render: (_, record) => record.user_id || '-',
    },
    {
      title: '状态',
      dataIndex: 'resolved',
      width: 96,
      render: (_, record) => (
        <Tag color={record.resolved ? 'green' : 'red'}>
          {record.resolved ? '已处理' : '未处理'}
        </Tag>
      ),
    },
    { title: '请求ID', dataIndex: 'request_id', ellipsis: true },
    {
      title: '时间',
      dataIndex: 'created_at',
      render: (_, record) => formatDate(record.created_at),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 200,
      render: (_, record) => {
        const actions: React.ReactNode[] = [
          <a
            key="detail"
            onClick={() => {
              void readSystemErrorLog(record.id).then((detail) => {
                setDetailState({
                  title: `系统错误 #${detail.id}`,
                  columns: [
                    { title: '代码', dataIndex: 'code' },
                    { title: '消息', dataIndex: 'message' },
                    { title: '方法', dataIndex: 'method' },
                    { title: '路径', dataIndex: 'path' },
                    { title: 'IP', dataIndex: 'ip' },
                    { title: 'User-Agent', dataIndex: 'user_agent' },
                    { title: '请求ID', dataIndex: 'request_id' },
                    {
                      title: '用户',
                      dataIndex: 'user_id',
                      render: (_, entity) => entity.user_id || '-',
                    },
                    {
                      title: '详情',
                      dataIndex: 'detail',
                      render: (_, entity) => entity.detail || '-',
                    },
                    {
                      title: '处理状态',
                      dataIndex: 'resolved',
                      render: (_, entity) =>
                        entity.resolved ? '已处理' : '未处理',
                    },
                    {
                      title: '处理人',
                      dataIndex: 'resolved_by',
                      render: (_, entity) => entity.resolved_by || '-',
                    },
                    {
                      title: '处理备注',
                      dataIndex: 'resolve_note',
                      render: (_, entity) => entity.resolve_note || '-',
                    },
                    {
                      title: '处理时间',
                      dataIndex: 'resolved_at',
                      render: (_, entity) =>
                        entity.resolved_at
                          ? formatDate(entity.resolved_at)
                          : '-',
                    },
                    {
                      title: '时间',
                      dataIndex: 'created_at',
                      render: (_, entity) => formatDate(entity.created_at),
                    },
                  ],
                  detail,
                });
              });
            }}
          >
            详情
          </a>,
        ];
        if (access.canLogResolve) {
          if (record.resolved) {
            actions.push(
              <Popconfirm
                key="reopen"
                title="取消处理状态"
                description={`确认把 #${record.id} 标记为未处理？`}
                okText="确认"
                onConfirm={async () => {
                  await reopenSystemErrorLog(record.id);
                  message.success('系统错误已取消处理');
                  errorActionRef.current?.reload();
                }}
              >
                <a>取消处理</a>
              </Popconfirm>,
            );
          } else {
            actions.push(
              <a key="resolve" onClick={() => setResolving(record)}>
                处理
              </a>,
            );
          }
        }
        if (access.canLogDelete) {
          actions.push(
            deleteButton(
              '删除系统错误日志',
              `确认删除 #${record.id}？`,
              async () => {
                await deleteSystemErrorLog(record.id);
                message.success('系统错误日志已删除');
                errorActionRef.current?.reload();
              },
            ),
          );
        }
        return actions;
      },
    },
  ];

  return (
    <PageContainer
      title="系统日志"
      tabActiveKey={activeTab}
      onTabChange={setActiveTab}
      tabList={[
        { key: 'operations', tab: '操作日志' },
        { key: 'logins', tab: '登录日志' },
        { key: 'errors', tab: '系统错误' },
      ]}
    >
      {activeTab === 'operations' && (
        <LogTableShell<OperationLog>
          columns={operationColumns}
          actionRef={operationActionRef}
          canDelete={access.canLogDelete}
          list={listOperationLogs}
          batchDelete={async (ids) => {
            await batchDeleteOperationLogs(ids);
            message.success('操作日志已批量删除');
            operationActionRef.current?.reload();
          }}
          batchDeleteButton={batchDeleteButton}
        />
      )}
      {activeTab === 'logins' && (
        <LogTableShell<LoginLog>
          columns={loginColumns}
          actionRef={loginActionRef}
          canDelete={access.canLogDelete}
          list={listLoginLogs}
          batchDelete={async (ids) => {
            await batchDeleteLoginLogs(ids);
            message.success('登录日志已批量删除');
            loginActionRef.current?.reload();
          }}
          batchDeleteButton={batchDeleteButton}
        />
      )}
      {activeTab === 'errors' && (
        <LogTableShell<SystemErrorLog>
          columns={errorColumns}
          actionRef={errorActionRef}
          canDelete={access.canLogDelete}
          list={listSystemErrorLogs}
          batchDelete={async (ids) => {
            await batchDeleteSystemErrorLogs(ids);
            message.success('系统错误日志已批量删除');
            errorActionRef.current?.reload();
          }}
          batchDeleteButton={batchDeleteButton}
        />
      )}
      <Drawer
        title={detailState.title}
        width={560}
        open={Boolean(detailState.detail)}
        onClose={() => setDetailState({ title: '', columns: [] })}
      >
        {detailState.detail && (
          <ProDescriptions
            column={1}
            size="small"
            dataSource={detailState.detail}
            columns={detailState.columns}
          />
        )}
      </Drawer>
      <ModalForm<{ note?: string }>
        key={resolving?.id ?? 'idle'}
        title={resolving ? `处理系统错误 #${resolving.id}` : '处理系统错误'}
        open={Boolean(resolving)}
        onOpenChange={(open) => {
          if (!open) {
            setResolving(undefined);
          }
        }}
        modalProps={{ destroyOnHidden: true }}
        initialValues={{ note: resolving?.resolve_note }}
        onFinish={async (values) => {
          if (!resolving) {
            return true;
          }
          await resolveSystemErrorLog(resolving.id, values.note?.trim());
          message.success('系统错误已标记为已处理');
          errorActionRef.current?.reload();
          return true;
        }}
      >
        <ProFormTextArea
          name="note"
          label="处理备注"
          fieldProps={{ maxLength: 1000, rows: 4, showCount: true }}
          rules={[{ max: 1000 }]}
        />
      </ModalForm>
    </PageContainer>
  );
};

export default Logs;
