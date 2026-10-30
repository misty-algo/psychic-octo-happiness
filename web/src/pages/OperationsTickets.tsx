import {
  ModalForm,
  type ActionType,
  PageContainer,
  type ProColumns,
  ProFormDependency,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import { Button, Descriptions, Drawer, message, Space, Table, Tag } from 'antd';
import React, { useRef, useState } from 'react';

import {
  appealTicket,
  type Ticket,
  type TicketDetailData,
  type TicketRow,
  executeTicketCommand,
  listTickets,
  readTicket,
} from '@/services/operations';

// 五态与后端 SSOT 完全一致，前端不自定义第六种状态。
const stateLabels: Record<string, string> = {
  PENDING: '待处理',
  PROCESSING: '处理中',
  WAITING_USER: '等待用户',
  RESOLVED: '已完结',
  CLOSED: '已关闭',
};

const stateColors: Record<string, string> = {
  PENDING: 'orange',
  PROCESSING: 'blue',
  WAITING_USER: 'cyan',
  RESOLVED: 'green',
  CLOSED: 'default',
};

const typeLabels: Record<string, string> = {
  DETOUR: '绕路',
  SERVICE_ATTITUDE: '服务态度',
  DANGEROUS_DRIVING: '危险驾驶',
  FARE_DISPUTE: '费用异议',
  OTHER: '其他',
};

const commandLabels: Record<string, string> = {
  claim: '领单',
  reply: '回复',
  wait_user: '等待用户（暂停计时）',
  resume: '用户回应（续算）',
  resolve: '解决',
  close: '关闭',
  transfer: '转派换人',
  escalate: '升级为紧急',
  revoke: '误报撤销',
  merge: '合并重复投诉',
};

// 命令前置条件与后端 CheckCommand 保持一致：前端先拦一道，避免无效请求。
const commandStates: Record<string, string[]> = {
  claim: ['PENDING'],
  reply: ['PROCESSING'],
  wait_user: ['PROCESSING'],
  resume: ['WAITING_USER'],
  resolve: ['PROCESSING'],
  close: ['RESOLVED', 'PENDING'],
  transfer: ['PENDING', 'PROCESSING', 'WAITING_USER'],
  escalate: ['PENDING', 'PROCESSING', 'WAITING_USER'],
  revoke: ['PENDING', 'PROCESSING'],
  merge: ['PENDING', 'PROCESSING', 'WAITING_USER', 'RESOLVED'],
};

type TicketCommandForm = {
  command: string;
  message: string;
  assignee?: string;
  merge_no?: string;
};

const formatDate = (value?: string) =>
  value ? new Date(value).toLocaleString() : '-';

const hoursText = (hours?: number) => {
  if (hours === undefined || hours === null) {
    return '-';
  }
  if (hours <= 0) {
    return '已超时';
  }
  return `${hours.toFixed(1)} 小时`;
};

const OperationsTickets: React.FC = () => {
  const access = useAccess();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [detail, setDetail] = useState<TicketDetailData>();
  const [commandTarget, setCommandTarget] = useState<TicketRow>();
  const [appealTarget, setAppealTarget] = useState<TicketRow>();

  const reload = () => actionRef.current?.reload();

  const openDetail = async (ticketNo: string) => {
    setDetail(await readTicket(ticketNo));
  };

  const stateFilterOptions = Object.keys(stateLabels).map((value) => ({
    label: stateLabels[value],
    value,
  }));

  const typeFilterOptions = Object.keys(typeLabels).map((value) => ({
    label: typeLabels[value],
    value,
  }));

  const columns: ProColumns<TicketRow>[] = [
    { title: '关键词', dataIndex: 'keyword', hideInTable: true,
      fieldProps: { placeholder: '工单号或订单号' } },
    { title: '状态', dataIndex: 'state', hideInTable: true, valueType: 'select',
      fieldProps: { placeholder: '按状态筛选', allowClear: true, options: stateFilterOptions } },
    { title: '类型', dataIndex: 'type', hideInTable: true, valueType: 'select',
      fieldProps: { placeholder: '按类型筛选', allowClear: true, options: typeFilterOptions } },
    {
      title: '只看超时',
      dataIndex: 'overdue',
      hideInTable: true,
      valueType: 'select',
      fieldProps: {
        placeholder: '是否只看超时',
        allowClear: true,
        options: [
          { label: '只看超时未回复', value: 'true' },
        ],
      },
    },
    {
      title: '工单号',
      dataIndex: 'ticket_no',
      width: 160,
      search: false,
    },
    {
      title: '状态',
      dataIndex: 'state_show',
      width: 130,
      search: false,
      render: (_, record) => (
        <Space size={4} wrap>
          <Tag color={stateColors[record.state] ?? 'default'}>
            {stateLabels[record.state] ?? record.state}
          </Tag>
          {record.overdue && <Tag color="red">超时</Tag>}
          {record.urgent && <Tag color="volcano">紧急</Tag>}
          {record.paused && <Tag color="cyan">计时暂停</Tag>}
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type_show',
      width: 100,
      search: false,
      render: (_, record) => typeLabels[record.type] ?? record.type,
    },
    {
      title: '分组',
      dataIndex: 'group',
      width: 110,
      valueType: 'select',
      fieldProps: {
        placeholder: '按处理分组筛选',
        allowClear: true,
        showSearch: true,
        options: [
          { label: '全部', value: '全部' },
          { label: '安全组', value: '安全组' },
          { label: '协调组', value: '协调组' },
        ],
      },
    },
    { title: '订单号', dataIndex: 'order_no', width: 110, search: false },
    {
      title: '乘客',
      dataIndex: 'passenger_name',
      width: 150,
      search: false,
      render: (_, record) => (
        <span>
          {record.passenger_name || '-'} {record.passenger_phone || ''}
        </span>
      ),
    },
    {
      title: '司机',
      dataIndex: 'driver_name',
      width: 150,
      search: false,
      render: (_, record) => (
        <span>
          {record.driver_name || '-'} {record.plate || ''}
        </span>
      ),
    },
    {
      title: '首次回复剩余',
      dataIndex: 'remaining_hours',
      width: 130,
      search: false,
      render: (_, record) => (
        <span style={{ color: record.overdue ? '#cf1322' : undefined }}>
          {hoursText(record.remaining_hours)}
        </span>
      ),
    },
    { title: '处理人', dataIndex: 'assignee', width: 110, search: false,
      render: (_, record) => record.assignee || '-' },
    { title: '轮次', dataIndex: 'round', width: 60, search: false },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      width: 170,
      search: false,
      render: (_, record) => formatDate(record.created_at),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 150,
      fixed: 'right',
      render: (_, record) => {
        const actions: React.ReactNode[] = [
          <a key="detail" onClick={() => void openDetail(record.ticket_no)}>
            详情与流转
          </a>,
        ];
        if (access.canOpsTicketHandle) {
          actions.push(
            <a key="command" onClick={() => setCommandTarget(record)}>
              命令
            </a>,
          );
        }
        if (access.canOpsTicketSenior && record.state === 'RESOLVED') {
          actions.push(
            <a key="appeal" onClick={() => setAppealTarget(record)}>
              申诉
            </a>,
          );
        }
        return actions;
      },
    },
  ];

  return (
    <PageContainer title="投诉工单处理">
      <ProTable<TicketRow>
        rowKey="ticket_no"
        actionRef={actionRef}
        columns={columns}
        scroll={{ x: 1400 }}
        request={async (params) => {
          const response = await listTickets({
            page: params.current ?? 1,
            page_size: params.pageSize ?? 20,
            keyword: (params as { keyword?: string }).keyword || undefined,
            state: (params as { state?: string }).state || undefined,
            type: (params as { type?: string }).type || undefined,
            group: (params as { group?: string }).group || undefined,
            overdue: (params as { overdue?: string }).overdue === 'true',
          });
          return {
            data: response.data,
            success: true,
            total: response.page?.total ?? 0,
          };
        }}
        pagination={{ defaultPageSize: 20 }}
      />
      <Drawer
        title={`工单详情 ${detail?.ticket?.ticket_no ?? ''}`}
        width={900}
        open={Boolean(detail)}
        onClose={() => setDetail(undefined)}
        extra={
          detail && access.canOpsTicketHandle ? (
            <Button
              type="primary"
              onClick={() =>
                setCommandTarget({
                  ticket_no: detail.ticket.ticket_no,
                  state: detail.ticket.state,
                } as TicketRow)
              }
            >
              执行命令
            </Button>
          ) : undefined
        }
      >
        {detail && (
          <Descriptions column={2} size="small" bordered>
            <Descriptions.Item label="工单号">
              {detail.ticket.ticket_no}
            </Descriptions.Item>
            <Descriptions.Item label="状态">
              <Space size={4}>
                <Tag color={stateColors[detail.ticket.state] ?? 'default'}>
                  {stateLabels[detail.ticket.state] ?? detail.ticket.state}
                </Tag>
                {detail.ticket.urgent && <Tag color="volcano">紧急</Tag>}
                {detail.ticket.escalated && <Tag color="red">已升级</Tag>}
              </Space>
            </Descriptions.Item>
            <Descriptions.Item label="类型">
              {typeLabels[detail.ticket.type] ?? detail.ticket.type}
            </Descriptions.Item>
            <Descriptions.Item label="处理分组">
              {detail.ticket.group}
            </Descriptions.Item>
            <Descriptions.Item label="关联订单">
              {detail.ticket.order_no || '-'}
            </Descriptions.Item>
            <Descriptions.Item label="轮次">
              第 {detail.ticket.round} 轮
            </Descriptions.Item>
            <Descriptions.Item label="乘客">
              {detail.ticket.passenger_name} {detail.ticket.passenger_phone}
            </Descriptions.Item>
            <Descriptions.Item label="司机">
              {detail.ticket.driver_name || '-'} {detail.ticket.plate || ''}
            </Descriptions.Item>
            <Descriptions.Item label="首次回复时限" span={2}>
              {formatDate(detail.ticket.first_response_due_at)}
              {detail.ticket.first_responded_at
                ? `（已于 ${formatDate(detail.ticket.first_responded_at)} 首次回复）`
                : '（未回复）'}
              {detail.ticket.paused_ms > 0
                ? `　累计暂停 ${(detail.ticket.paused_ms / 3600000).toFixed(2)} 小时`
                : ''}
            </Descriptions.Item>
            <Descriptions.Item label="投诉内容" span={2}>
              {detail.ticket.content}
            </Descriptions.Item>
            <Descriptions.Item label="凭证">
              {detail.ticket.evidence_count} 个
            </Descriptions.Item>
            <Descriptions.Item label="误报">
              {detail.ticket.false_report?.revoked
                ? `已核实为误报（${detail.ticket.false_report.by}）`
                : '-'}
            </Descriptions.Item>
          </Descriptions>
        )}

        {detail && (
          <>
            <h4 style={{ margin: '16px 0 8px' }}>处理轮次</h4>
            <Table
              rowKey="round"
              size="small"
              pagination={false}
              dataSource={detail.ticket.rounds ?? []}
              columns={[
                { title: '轮次', dataIndex: 'round', width: 60 },
                { title: '处理人', dataIndex: 'handler' },
                {
                  title: '状态',
                  dataIndex: 'state',
                  render: (_, row) =>
                    stateLabels[row.state] ?? row.state ?? '-',
                },
                { title: '结果', dataIndex: 'result' },
                {
                  title: '开始',
                  dataIndex: 'started_at',
                  render: (_, row) => formatDate(row.started_at),
                },
                {
                  title: '结束',
                  dataIndex: 'ended_at',
                  render: (_, row) => formatDate(row.ended_at),
                },
              ]}
            />

            <h4 style={{ margin: '16px 0 8px' }}>
              流转记录（谁、什么时间、从什么状态到什么状态、说了什么）
            </h4>
            <Table
              rowKey={(row) => `${row.seq}:${row.at}`}
              size="small"
              pagination={false}
              dataSource={detail.ticket.timeline ?? []}
              columns={[
                { title: '操作人', dataIndex: 'actor', width: 120 },
                {
                  title: '动作',
                  dataIndex: 'action',
                  width: 110,
                  render: (_, row) => commandLabels[row.action] ?? row.action,
                },
                {
                  title: '状态变化',
                  width: 170,
                  render: (_, row) =>
                    `${stateLabels[row.from] ?? (row.from || '-')} → ${
                      stateLabels[row.to] ?? (row.to || '-')
                    }`,
                },
                { title: '说明', dataIndex: 'message' },
                {
                  title: '时间',
                  dataIndex: 'at',
                  width: 170,
                  render: (_, row) => formatDate(row.at),
                },
              ]}
            />

            <h4 style={{ margin: '16px 0 8px' }}>重复投诉（24 小时内同订单同类型）</h4>
            {detail.duplicates && detail.duplicates.length > 0 ? (
              <Table
                rowKey="ticket_no"
                size="small"
                pagination={false}
                dataSource={detail.duplicates}
                columns={[
                  { title: '工单号', dataIndex: 'ticket_no' },
                  {
                    title: '状态',
                    dataIndex: 'state',
                    render: (_, row) => stateLabels[row.state] ?? row.state,
                  },
                  { title: '内容', dataIndex: 'content', ellipsis: true },
                  {
                    title: '创建',
                    dataIndex: 'created_at',
                    width: 170,
                    render: (_, row) => formatDate(row.created_at),
                  },
                ]}
              />
            ) : (
              <span style={{ color: '#999' }}>无重复投诉</span>
            )}
          </>
        )}
      </Drawer>
      <ModalForm<TicketCommandForm>
        key={commandTarget?.ticket_no ?? 'command-idle'}
        title={
          commandTarget ? `执行工单命令 ${commandTarget.ticket_no}` : '执行工单命令'
        }
        open={Boolean(commandTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setCommandTarget(undefined);
          }
        }}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          if (!commandTarget) {
            return true;
          }
          const affected = commandTarget;
          await executeTicketCommand(affected.ticket_no, {
            command: values.command,
            message: values.message,
            assignee: values.assignee || undefined,
            merge_no: values.merge_no || undefined,
          });
          message.success('命令已执行');
          setCommandTarget(undefined);
          if (detail?.ticket?.ticket_no === affected.ticket_no) {
            void openDetail(affected.ticket_no);
          }
          reload();
          return true;
        }}
      >
        <ProFormSelect
          name="command"
          label="命令"
          rules={[{ required: true, message: '请选择命令' }]}
          fieldProps={{
            options: Object.entries(commandLabels)
              .filter(([value]) => {
                if (!commandTarget) return true;
                return (commandStates[value] ?? []).includes(commandTarget.state);
              })
              .map(([value, label]) => ({ value, label })),
          }}
        />
        <ProFormTextArea
          name="message"
          label="说明（乘客可见 / 留痕）"
          placeholder="如：已电话联系乘客并致歉"
          rules={[
            { required: true, message: '命令必须填写说明' },
            { min: 5, max: 200, message: '说明长度需在 5-200 字之间' },
          ]}
          fieldProps={{ rows: 3, showCount: true, maxLength: 200 }}
        />
        <ProFormDependency name={['command']}>
          {({ command }: { command?: string }) =>
            command === 'transfer' ? (
              <ProFormText
                name="assignee"
                label="新处理人"
                placeholder="如：admin:2"
                rules={[{ required: true, message: '转派必须指定新处理人' }]}
              />
            ) : null
          }
        </ProFormDependency>
        <ProFormDependency name={['command']}>
          {({ command }: { command?: string }) =>
            command === 'merge' ? (
              <ProFormText
                name="merge_no"
                label="待合并工单号"
                placeholder="如：W20261022-0005"
                rules={[{ required: true, message: '合并必须指定工单号' }]}
              />
            ) : null
          }
        </ProFormDependency>
        {commandTarget && (
          <span style={{ color: '#999', fontSize: 12 }}>
            当前状态 {stateLabels[commandTarget.state] ?? commandTarget.state}
            ，仅列出该状态允许的命令；转派不会刷新首次回复时限
          </span>
        )}
      </ModalForm>

      <ModalForm<{ message: string }>
        key={appealTarget?.ticket_no ?? 'appeal-idle'}
        title={appealTarget ? `申诉处理 ${appealTarget.ticket_no}` : '申诉处理'}
        open={Boolean(appealTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setAppealTarget(undefined);
          }
        }}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          if (!appealTarget) {
            return true;
          }
          const affected = appealTarget;
          await appealTicket(affected.ticket_no, values);
          message.success('已开启新一轮处理');
          setAppealTarget(undefined);
          void openDetail(affected.ticket_no);
          reload();
          return true;
        }}
      >
        <span style={{ color: '#999', fontSize: 12 }}>
          申诉将开启新一轮次，且必须由另一名高级客服接手（不能是上一轮处理人）
        </span>
        <ProFormTextArea
          name="message"
          label="申诉处理说明"
          rules={[
            { required: true, message: '申诉必须填写说明' },
            { min: 5, max: 200, message: '说明长度需在 5-200 字之间' },
          ]}
          fieldProps={{ rows: 3, showCount: true, maxLength: 200 }}
        />
      </ModalForm>
    </PageContainer>
  );
};

export default OperationsTickets;
