import {
  ModalForm,
  type ActionType,
  PageContainer,
  type ProColumns,
  ProFormSelect,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess, useSearchParams } from '@umijs/max';
import { Button, Descriptions, Drawer, message, Space, Table, Tag } from 'antd';
import React, { useMemo, useRef, useState } from 'react';

import {
  executeOrderCommand,
  type OrderDetailData,
  type OrderRow,
  listOrders,
  readOrder,
} from '@/services/operations';

const stateColors: Record<string, string> = {
  PENDING: 'orange',
  NO_DRIVER: 'volcano',
  ACCEPTED: 'blue',
  ARRIVED: 'cyan',
  IN_SERVICE: 'geekblue',
  COMPLETED_UNPAID: 'gold',
  UNPAID: 'gold',
  COMPLETED: 'green',
  CANCELLED: 'default',
  SUSPENDED: 'red',
};

// 订单状态是乘客域枚举，控制台统一展示中文，筛选与命令前置状态沿用英文枚举值。
const stateLabels: Record<string, string> = {
  PENDING: '待接单',
  NO_DRIVER: '无司机接单',
  ACCEPTED: '已接单',
  ARRIVED: '司机已到达',
  IN_SERVICE: '行程中',
  COMPLETED_UNPAID: '待支付',
  UNPAID: '待支付',
  COMPLETED: '已完成',
  CANCELLED: '已取消',
  SUSPENDED: '已挂起',
};

const stateLabel = (state?: string) =>
  (state && stateLabels[state]) || state || '-';

const payLabels: Record<string, string> = {
  UNPAID: '待支付',
  PAID: '已支付',
  CANCELLED: '已取消',
  REFUNDED: '已退款',
};

const payLabel = (state?: string) =>
  (state && payLabels[state]) || state || '-';

const commandLabels: Record<string, string> = {
  reassign: '改派',
  waive_cancel_fee: '免取消费',
  safe_end: '安全提前结束',
  review_pricing: '异常计费复核',
};

// 命令前置条件与后端 domain.CheckCommand 保持一致：前端先拦一道，避免无效请求。
const commandStates: Record<string, string[]> = {
  reassign: ['PENDING', 'NO_DRIVER', 'ACCEPTED', 'ARRIVED'],
  waive_cancel_fee: ['CANCELLED'],
  safe_end: ['IN_SERVICE'],
  review_pricing: ['UNPAID', 'COMPLETED'],
};

const yuan = (fen?: number) => ((fen ?? 0) / 100).toFixed(2);

const formatDate = (value?: string) =>
  value ? new Date(value).toLocaleString() : '-';

const OperationsOrders: React.FC = () => {
  const access = useAccess();
  // 看板卡片（今日订单/行程中订单）跳转时带初始过滤条件。
  const [searchParams] = useSearchParams();
  const initialFrom = searchParams.get('from') ?? undefined;
  const initialTo = searchParams.get('to') ?? undefined;
  const initialState = searchParams.get('state') ?? undefined;
  const actionRef = useRef<ActionType | undefined>(undefined);
  // 看板带过来的过滤只作用于首次加载：之后以筛选栏的选择为准，
  // 否则 URL 里的空条件会覆盖操作员手动选的状态/时间。
  const urlSeed = useMemo(() => {
    const seed: Record<string, string> = {};
    if (initialFrom) seed.from = initialFrom;
    if (initialTo) seed.to = initialTo;
    if (initialState) seed.state = initialState;
    return seed;
  }, [initialFrom, initialTo, initialState]);
  const seedUsed = useRef(false);
  const [detail, setDetail] = useState<OrderDetailData>();
  const [commandTarget, setCommandTarget] = useState<OrderRow>();

  const reload = () => actionRef.current?.reload();

  const openDetail = async (orderNo: string) => {
    setDetail(await readOrder(orderNo));
  };

  const stateFilterOptions = Object.keys(stateColors).map((value) => ({
    label: stateLabels[value] ?? value,
    value,
  }));

  const columns: ProColumns<OrderRow>[] = [
    {
      title: '关键词',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '订单号或乘客手机号/姓名' },
    },
    {
      title: '状态筛选',
      dataIndex: 'state',
      hideInTable: true,
      valueType: 'select',
      fieldProps: { placeholder: '按状态筛选', allowClear: true, options: stateFilterOptions },
    },
    {
      title: '业务类型',
      dataIndex: 'biz_type',
      hideInTable: true,
      fieldProps: { placeholder: '如 express / pool / special', allowClear: true },
    },
    {
      title: '城市',
      dataIndex: 'city_code',
      hideInTable: true,
      fieldProps: { placeholder: '城市编码，如 310100', allowClear: true },
    },
    {
      title: '风险等级',
      dataIndex: 'risk_level',
      hideInTable: true,
      fieldProps: { placeholder: '按风险等级筛选', allowClear: true },
    },
    {
      title: '创建时间',
      dataIndex: 'time_range',
      hideInTable: true,
      valueType: 'dateTimeRange',
      search: {
        // 日期选择器给的是 "YYYY-MM-DD HH:mm:ss"，转 ISO 保证后端一定可解析。
        transform: (value: string[]) => ({
          from: value?.[0] ? new Date(value[0]).toISOString() : undefined,
          to: value?.[1] ? new Date(value[1]).toISOString() : undefined,
        }),
      },
      fieldProps: { placeholder: ['开始时间', '结束时间'] },
    },
    {
      title: '订单号',
      dataIndex: 'order_no',
      width: 170,
      search: false,
    },
    {
      title: '状态',
      dataIndex: 'state_show',
      width: 110,
      search: false,
      render: (_, record) => (
        <Tag color={stateColors[record.state] ?? 'default'}>
          {stateLabel(record.state)}
        </Tag>
      ),
    },
    {
      title: '乘客',
      dataIndex: 'passenger_name',
      width: 140,
      search: false,
      render: (_, record) => (
        <span>
          {record.passenger_name || '-'}
          {record.passenger_phone ? ` ${record.passenger_phone}` : ''}
        </span>
      ),
    },
    {
      title: '司机',
      dataIndex: 'driver_name',
      width: 140,
      search: false,
      render: (_, record) => (
        <span>
          {record.driver_name || '-'}
          {record.plate ? ` ${record.plate}` : ''}
        </span>
      ),
    },
    {
      title: '金额',
      dataIndex: 'amount',
      width: 100,
      search: false,
      render: (_, record) => `¥${yuan(record.amount)}`,
    },
    {
      title: '支付',
      dataIndex: 'pay_status',
      width: 90,
      search: false,
      render: (_, record) => payLabel(record.pay_status),
    },
    {
      title: '起点',
      dataIndex: 'start_address',
      search: false,
      ellipsis: true,
    },
    {
      title: '终点',
      dataIndex: 'end_address',
      search: false,
      ellipsis: true,
    },
    {
      title: '下单时间',
      dataIndex: 'created_at',
      width: 170,
      search: false,
      render: (_, record) => formatDate(record.created_at),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 140,
      fixed: 'right',
      render: (_, record) => [
        <a key="detail" onClick={() => void openDetail(record.order_no)}>
          穿透详情
        </a>,
        ...(access.canOpsOrderCommand
          ? [
              <a key="command" onClick={() => setCommandTarget(record)}>
                命令
              </a>,
            ]
          : []),
      ],
    },
  ];

  return (
    <PageContainer title="订单管理">
      <ProTable<OrderRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        scroll={{ x: 1200 }}
        request={async (params) => {
          const seed = seedUsed.current ? {} : urlSeed;
          seedUsed.current = true;
          const query = {
            ...(params as Record<string, unknown>),
            ...seed,
          } as {
            current?: number;
            pageSize?: number;
            keyword?: string;
            state?: string;
            biz_type?: string;
            city_code?: string;
            risk_level?: string;
            from?: string;
            to?: string;
          };
          const response = await listOrders({
            page: query.current ?? 1,
            page_size: query.pageSize ?? 20,
            keyword: query.keyword || undefined,
            state: query.state || undefined,
            biz_type: query.biz_type || undefined,
            city_code: query.city_code || undefined,
            risk_level: query.risk_level || undefined,
            from: query.from || undefined,
            to: query.to || undefined,
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
        title={`订单详情 ${detail?.order?.order_no ?? ''}`}
        width={860}
        open={Boolean(detail)}
        onClose={() => setDetail(undefined)}
        extra={
          detail && access.canOpsOrderCommand ? (
            <Button
              type="primary"
              onClick={() => setCommandTarget(detail.order as OrderRow)}
            >
              执行命令
            </Button>
          ) : undefined
        }
      >
        {detail && (
          <Descriptions column={2} size="small" bordered>
            <Descriptions.Item label="订单号">
              {detail.order.order_no}
            </Descriptions.Item>
            <Descriptions.Item label="状态">
              <Tag color={stateColors[detail.order.state] ?? 'default'}>
                {stateLabel(detail.order.state)}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="乘客">
              {detail.order.passenger_name || '-'}{' '}
              {detail.order.passenger_phone || ''}
            </Descriptions.Item>
            <Descriptions.Item label="司机">
              {detail.order.driver_name || '-'} {detail.order.plate || ''}
            </Descriptions.Item>
            <Descriptions.Item label="金额">
              ¥{yuan(detail.order.amount)}
            </Descriptions.Item>
            <Descriptions.Item label="支付状态">
              {payLabel(detail.order.pay_status)}
            </Descriptions.Item>
            <Descriptions.Item label="起点">
              {detail.order.start_address}
            </Descriptions.Item>
            <Descriptions.Item label="终点">
              {detail.order.end_address}
            </Descriptions.Item>
            <Descriptions.Item label="计价版本">
              {detail.pricing.version || '-'}
            </Descriptions.Item>
            <Descriptions.Item label="计费复核">
              {detail.pricing.reviewed
                ? `已复核 ${detail.pricing.reviewed_by}`
                : '未复核'}
            </Descriptions.Item>
          </Descriptions>
        )}

        {detail && (
          <>
            <h4 style={{ margin: '16px 0 8px' }}>派单任务</h4>
            <Table
              rowKey="task_no"
              size="small"
              pagination={false}
              dataSource={detail.tasks ?? []}
              columns={[
                { title: '任务号', dataIndex: 'task_no' },
                { title: '轮次', dataIndex: 'round', width: 60 },
                { title: '司机', dataIndex: 'driver_name' },
                {
                  title: '状态',
                  dataIndex: 'state',
                  render: (_, row) => row.state || '-',
                },
                {
                  title: '时间',
                  dataIndex: 'created_at',
                  render: (_, row) => formatDate(row.created_at),
                },
              ]}
            />

            <h4 style={{ margin: '16px 0 8px' }}>接单尝试</h4>
            <Table
              rowKey={(row) => `${row.driver_id}:${row.responded_at}`}
              size="small"
              pagination={false}
              dataSource={detail.attempts ?? []}
              columns={[
                { title: '司机', dataIndex: 'driver_name' },
                { title: '结果', dataIndex: 'result' },
                { title: '评分', dataIndex: 'score', width: 80 },
                { title: '距离(米)', dataIndex: 'distance', width: 90 },
                {
                  title: '响应',
                  dataIndex: 'responded_at',
                  render: (_, row) => formatDate(row.responded_at),
                },
              ]}
            />

            <h4 style={{ margin: '16px 0 8px' }}>
              轨迹（{detail.trajectory?.points?.length ?? 0} 点 ·{' '}
              {detail.trajectory?.source || '无'}）
            </h4>
            {(detail.trajectory?.points?.length ?? 0) > 0 ? (
              <pre style={{ maxHeight: 160, overflow: 'auto', fontSize: 12 }}>
                {JSON.stringify(detail.trajectory.points, null, 2)}
              </pre>
            ) : (
              <span style={{ color: '#999' }}>暂无轨迹数据</span>
            )}

            <h4 style={{ margin: '16px 0 8px' }}>支付与退款</h4>
            {detail.payment ? (
              <Descriptions column={2} size="small" bordered>
                <Descriptions.Item label="渠道">
                  {detail.payment.channel || '-'}
                </Descriptions.Item>
                <Descriptions.Item label="流水号">
                  {detail.payment.trade_no || '-'}
                </Descriptions.Item>
                <Descriptions.Item label="状态">
                  {detail.payment.state || '-'}
                </Descriptions.Item>
                <Descriptions.Item label="金额">
                  ¥{yuan(detail.payment.amount)}
                </Descriptions.Item>
                <Descriptions.Item label="已退款">
                  {detail.payment.refunded ? '是' : '否'}
                </Descriptions.Item>
                <Descriptions.Item label="退款金额">
                  ¥{yuan(detail.payment.refund_amount)}
                </Descriptions.Item>
              </Descriptions>
            ) : (
              <span style={{ color: '#999' }}>暂无支付记录</span>
            )}

            <h4 style={{ margin: '16px 0 8px' }}>关联工单</h4>
            <Table
              rowKey="ticket_no"
              size="small"
              pagination={false}
              dataSource={detail.tickets ?? []}
              columns={[
                { title: '工单号', dataIndex: 'ticket_no' },
                { title: '类型', dataIndex: 'type' },
                { title: '状态', dataIndex: 'state' },
                {
                  title: '创建时间',
                  dataIndex: 'created_at',
                  render: (_, row) => formatDate(row.created_at),
                },
              ]}
            />
          </>
        )}
      </Drawer>
      <ModalForm<{ command: string; basis: string }>
        key={commandTarget?.order_no ?? 'command-idle'}
        title={
          commandTarget
            ? `执行订单命令 ${commandTarget.order_no}`
            : '执行订单命令'
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
          await executeOrderCommand(affected.order_no, values);
          message.success('命令已执行');
          setCommandTarget(undefined);
          if (detail?.order?.order_no === affected.order_no) {
            void openDetail(affected.order_no);
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
                return (commandStates[value] ?? []).includes(
                  commandTarget.state,
                );
              })
              .map(([value, label]) => ({ value, label })),
          }}
        />
        <ProFormTextArea
          name="basis"
          label="工单依据"
          placeholder="如：工单 W001 乘客要求更换司机"
          rules={[
            { required: true, message: '命令必须填写依据' },
            { min: 5, max: 200, message: '依据长度需在 5-200 字之间' },
          ]}
          fieldProps={{ rows: 3, showCount: true, maxLength: 200 }}
        />
        {commandTarget && (
          <Space size={4} wrap>
            <span style={{ color: '#999', fontSize: 12 }}>
              当前状态 {stateLabel(commandTarget.state)}，仅当前置条件满足时命令才会被后端接受
            </span>
          </Space>
        )}
      </ModalForm>
    </PageContainer>
  );
};

export default OperationsOrders;
