import {
  ModalForm,
  ProCard,
  ProFormDatePicker,
  ProFormDependency,
  ProFormDigit,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
  ProTable,
  type ProColumns,
} from '@ant-design/pro-components';
import {
  Button,
  DatePicker,
  Descriptions,
  Drawer,
  Modal,
  Popconfirm,
  Progress,
  Space,
  Table,
  Tag,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';
import { useAccess } from '@umijs/max';

import {
  type Campaign,
  type CampaignSettlement,
  type CampaignSubsidySummary,
  type CouponBatch,
  type CouponTemplate,
  type IssuedCoupon,
  type IssueResult,
  type SubsidyRecord,
} from '@/services/operations';
import * as api from '@/services/operations';

/** 分转元：金额一律按分存、按元显示。 */
const yuan = (fen?: number) => ((fen ?? 0) / 100).toFixed(2);
const fmt = (value?: string) =>
  value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '-';

/** A08 补贴类活动：发的是平台承担的钱，走补贴台账而不是券。 */
const SUBSIDY_CAMPAIGN_TYPES = ['rush_subsidy', 'period_subsidy', 'driver_bonus'];
const isSubsidyCampaign = (type?: string) => SUBSIDY_CAMPAIGN_TYPES.includes(type ?? '');
const SUBSIDY_TRIGGER_LABEL: Record<string, string> = {
  per_order: '每单固定补贴',
  time_period: '时段出车补贴',
  task: '冲单任务奖',
};
const SUBSIDY_BIZ_LABEL: Record<string, string> = {
  time_subsidy: '时段补贴',
  complete_task: '冲单奖',
  peak_bonus: '高峰奖',
  other: '其他',
};

const STATE_COLOR: Record<string, string> = {
  草稿: 'default',
  已发布: 'green',
  已停售: 'red',
};
const TYPE_LABEL: Record<string, string> = {
  cash: '满减券',
  discount: '折扣券',
  new_user: '新客券',
};
const BEARER_LABEL: Record<string, string> = {
  platform: '平台承担',
  driver: '司机承担',
  share: '共同承担',
};
const CHANNEL_LABEL: Record<string, string> = {
  new_user: '新客',
  activity: '活动',
  manual: '手工',
  invite: '邀请',
  compensate: '补偿',
};
const BATCH_STATUS: Record<string, string> = {
  running: '发放中',
  stopped: '已停止',
  finished: '已结束',
};
const CAMPAIGN_TYPE: Record<string, string> = {
  new_user_gift: '新客礼包',
  invite: '邀请拉新',
  recharge: '充值活动',
  sign_in: '签到活动',
  task_center: '任务中心',
  driver_bonus: '司机冲单奖',
  flash_sale: '限时秒杀',
  rush_subsidy: '高峰补贴',
  period_subsidy: '时段补贴',
};
const CAMPAIGN_STATUS: Record<string, string> = {
  draft: '草稿',
  running: '进行中',
  paused: '已暂停',
  ended: '已结束',
};
const COUPON_STATE: Record<string, string> = {
  AVAILABLE: '可用',
  USED: '已核销',
  EXPIRED: '已过期',
  FROZEN: '已冻结',
};

const Marketing: React.FC = () => {
  const access = useAccess();
  const canWrite = Boolean(access.canOpsMarketingWrite);
  const canSettle = Boolean(access.canOpsMarketingSettle);

  const templateRef = useRef<any>(null);
  const batchRef = useRef<any>(null);
  const couponRef = useRef<any>(null);
  const campaignRef = useRef<any>(null);

  const [templates, setTemplates] = useState<CouponTemplate[]>([]);
  const [createTemplateOpen, setCreateTemplateOpen] = useState(false);
  const [createBatchOpen, setCreateBatchOpen] = useState(false);
  const [createCampaignOpen, setCreateCampaignOpen] = useState(false);
  const [issueTarget, setIssueTarget] = useState<CouponBatch | undefined>();
  const [issueResult, setIssueResult] = useState<IssueResult | undefined>();
  // 公开投放：生成无主券码到领券中心，乘客先到先得。
  const [publishOpen, setPublishOpen] = useState(false);
  // A08 补贴台账抽屉：目标活动 + 汇总 + 台账记录。
  const [subsidyTarget, setSubsidyTarget] = useState<Campaign | undefined>();
  const [subsidySummary, setSubsidySummary] = useState<CampaignSubsidySummary | undefined>();
  const [subsidyRecords, setSubsidyRecords] = useState<SubsidyRecord[]>([]);
  const [settlement, setSettlement] = useState<CampaignSettlement | undefined>();

  const reloadAll = () => {
    templateRef.current?.reload();
    batchRef.current?.reload();
    couponRef.current?.reload();
    campaignRef.current?.reload();
  };

  const templateColumns: ProColumns<CouponTemplate>[] = [
    { title: '模板编号', dataIndex: 'template_no', width: 130 },
    { title: '名称', dataIndex: 'name' },
    {
      title: '类型',
      dataIndex: 'coupon_type',
      width: 90,
      render: (_, row) => TYPE_LABEL[row.coupon_type] ?? row.coupon_type,
    },
    {
      title: '面额/门槛',
      width: 140,
      render: (_, row) =>
        row.coupon_type === 'discount'
          ? `${row.discount_rate / 10} 折`
          : `¥${yuan(row.face_value_fen)} / 满 ¥${yuan(row.threshold_fen)}`,
    },
    {
      title: '承担方',
      dataIndex: 'bearer',
      width: 100,
      render: (_, row) => BEARER_LABEL[row.bearer] ?? row.bearer,
    },
    {
      title: '库存',
      width: 110,
      render: (_, row) => `${row.stock}（已发 ${row.issued_count}）`,
    },
    { title: '每人限领', dataIndex: 'per_limit', width: 90 },
    {
      title: '可叠加',
      dataIndex: 'stackable',
      width: 80,
      render: (_, row) => (row.stackable ? '是' : <Tag color="orange">否</Tag>),
    },
    {
      title: '有效期',
      width: 200,
      render: (_, row) => `${fmt(row.valid_start)} ~ ${fmt(row.valid_end)}`,
    },
    {
      title: '状态',
      dataIndex: 'state',
      width: 90,
      render: (_, row) => (
        <Tag color={STATE_COLOR[row.state] ?? 'default'}>{row.state}</Tag>
      ),
    },
    {
      title: '操作',
      width: 120,
      fixed: 'right',
      render: (_, row) =>
        canWrite ? (
          <Space size={4}>
            {row.state === '草稿' && (
              <Popconfirm
                title="发布后即可发放，确认发布？"
                onConfirm={async () => {
                  await api.executeTemplateCommand(row.template_no, {
                    command: 'publish',
                  });
                  message.success('已发布');
                  reloadAll();
                }}
              >
                <a>发布</a>
              </Popconfirm>
            )}
            {row.state === '已发布' && (
              <Popconfirm
                title="停售后不能再发放，确认停售？"
                onConfirm={async () => {
                  await api.executeTemplateCommand(row.template_no, {
                    command: 'stop',
                  });
                  message.success('已停售');
                  reloadAll();
                }}
              >
                <a>停售</a>
              </Popconfirm>
            )}
          </Space>
        ) : (
          '-'
        ),
    },
  ];

  const batchColumns: ProColumns<CouponBatch>[] = [
    { title: '批次号', dataIndex: 'batch_no', width: 130 },
    { title: '券名称', dataIndex: 'template_name' },
    {
      title: '发放进度',
      width: 170,
      render: (_, row) => (
        <span>
          {row.issued_count} / {row.total_count}
          {row.issued_count >= row.total_count && (
            <Tag color="red" style={{ marginLeft: 6 }}>
              已发完
            </Tag>
          )}
        </span>
      ),
    },
    {
      title: '渠道',
      dataIndex: 'channel',
      width: 90,
      render: (_, row) => CHANNEL_LABEL[row.channel] ?? row.channel,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (_, row) => (
        <Tag color={row.status === 'running' ? 'green' : 'default'}>
          {BATCH_STATUS[row.status] ?? row.status}
        </Tag>
      ),
    },
    { title: '关联活动', dataIndex: 'campaign_code', width: 110 },
    {
      title: '发放窗口',
      width: 200,
      render: (_, row) => `${fmt(row.start_at)} ~ ${fmt(row.end_at)}`,
    },
    {
      title: '操作',
      width: 190,
      fixed: 'right',
      render: (_, row) =>
        canWrite ? (
          <Space size={4}>
            {row.status === 'running' && (
              <>
                <a onClick={() => setIssueTarget(row)}>发放</a>
                <Popconfirm
                  title="停止后不能再发放，确认停止？"
                  onConfirm={async () => {
                    await api.executeBatchCommand(row.batch_no, { command: 'stop' });
                    message.success('已停止');
                    reloadAll();
                  }}
                >
                  <a>停止</a>
                </Popconfirm>
              </>
            )}
            {row.status !== 'finished' && (
              <Popconfirm
                title="结束后进入对账，确认结束？"
                onConfirm={async () => {
                  await api.executeBatchCommand(row.batch_no, { command: 'finish' });
                  message.success('已结束');
                  reloadAll();
                }}
              >
                <a>结束</a>
              </Popconfirm>
            )}
          </Space>
        ) : (
          '-'
        ),
    },
  ];

  const couponColumns: ProColumns<IssuedCoupon>[] = [
    { title: '券码', dataIndex: 'coupon_no', width: 150 },
    { title: '券名称', dataIndex: 'template_name' },
    { title: '批次', dataIndex: 'batch_no', width: 120 },
    {
      title: '乘客',
      width: 170,
      render: (_, row) => `${row.passenger_name ?? '-'} ${row.phone_mask ?? ''}`,
    },
    {
      title: '状态',
      dataIndex: 'state',
      width: 90,
      render: (_, row) => (
        <Tag color={row.state === 'AVAILABLE' ? 'green' : 'default'}>
          {COUPON_STATE[row.state] ?? row.state}
        </Tag>
      ),
    },
    {
      title: '面额',
      dataIndex: 'face_value_fen',
      width: 90,
      render: (_, row) => `¥${yuan(row.face_value_fen)}`,
    },
    {
      title: '发放时间',
      dataIndex: 'issued_at',
      width: 160,
      render: (_, row) => fmt(row.issued_at),
    },
    {
      title: '过期时间',
      dataIndex: 'expire_at',
      width: 160,
      render: (_, row) => fmt(row.expire_at),
    },
  ];

  /** 打开补贴台账抽屉：拉汇总 + 全量台账。 */
  const openSubsidyDrawer = async (row: Campaign) => {
    setSubsidyTarget(row);
    try {
      const [summary, ledger] = await Promise.all([
        api.readSubsidySummary(row.code),
        api.listSubsidyRecords(row.code, { page: 1, page_size: 50 }),
      ]);
      setSubsidySummary(summary);
      setSubsidyRecords(ledger.data ?? []);
    } catch (error: any) {
      message.error(error?.message ?? '读取补贴台账失败');
    }
  };

  const campaignColumns: ProColumns<Campaign>[] = [
    { title: '活动编码', dataIndex: 'code', width: 130 },
    { title: '活动名称', dataIndex: 'name' },
    {
      title: '类型',
      dataIndex: 'type',
      width: 110,
      render: (_, row) => CAMPAIGN_TYPE[row.type] ?? row.type,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (_, row) => (
        <Tag color={row.status === 'running' ? 'green' : 'default'}>
          {CAMPAIGN_STATUS[row.status] ?? row.status}
        </Tag>
      ),
    },
    {
      title: '预算使用',
      width: 150,
      render: (_, row) =>
        `¥${yuan(row.used_budget_fen)} / ¥${yuan(row.budget_fen)}`,
    },
    {
      title: '活动时间',
      width: 200,
      render: (_, row) => `${fmt(row.start_at)} ~ ${fmt(row.end_at)}`,
    },
    {
      title: '操作',
      width: 220,
      fixed: 'right',
      render: (_, row) => (
        <Space size={4}>
          {canWrite && row.status === 'draft' && (
            <Popconfirm
              title="确认开始活动？"
              onConfirm={async () => {
                await api.executeCampaignCommand(row.code, { command: 'start' });
                message.success('活动已开始');
                reloadAll();
              }}
            >
              <a>开始</a>
            </Popconfirm>
          )}
          {canWrite && row.status === 'running' && (
            <Popconfirm
              title="确认暂停活动？"
              onConfirm={async () => {
                await api.executeCampaignCommand(row.code, { command: 'pause' });
                message.success('活动已暂停');
                reloadAll();
              }}
            >
              <a>暂停</a>
            </Popconfirm>
          )}
          {canWrite && row.status !== 'ended' && row.status !== 'draft' && (
            <Popconfirm
              title="结束后才能出最终对账单，确认结束？"
              onConfirm={async () => {
                await api.executeCampaignCommand(row.code, { command: 'end' });
                message.success('活动已结束');
                reloadAll();
              }}
            >
              <a>结束</a>
            </Popconfirm>
          )}
          {isSubsidyCampaign(row.type) && (
            <a onClick={() => openSubsidyDrawer(row)}>补贴台账</a>
          )}
          {canSettle && (
            <a
              onClick={async () => {
                setSettlement(await api.readCampaignSettlement(row.code));
              }}
            >
              对账
            </a>
          )}
        </Space>
      ),
    },
  ];

  return (
    <>
      <ProCard
        tabs={{
        type: 'card',
        items: [
          {
            key: 'templates',
            label: '券模板',
            children: (
              <ProTable<CouponTemplate>
                headerTitle="券模板"
                actionRef={templateRef}
                rowKey="template_no"
                search={{ labelWidth: 'auto' }}
                toolBarRender={() =>
                  canWrite
                    ? [
                        <Button
                          key="new"
                          type="primary"
                          onClick={() => setCreateTemplateOpen(true)}
                        >
                          新建券模板
                        </Button>,
                      ]
                    : []
                }
                request={async (params) => {
                  const res = await api.listCouponTemplates({
                    page: params.current,
                    page_size: params.pageSize,
                    keyword: params.name,
                    state: params.state,
                    type: params.coupon_type,
                  });
                  return {
                    data: res.data ?? [],
                    total: (res as any).page?.total ?? 0,
                    success: true,
                  };
                }}
                columns={templateColumns}
              />
            ),
          },
          {
            key: 'batches',
            label: '券批次',
            children: (
              <ProTable<CouponBatch>
                headerTitle="券批次"
                actionRef={batchRef}
                rowKey="batch_no"
                search={{ labelWidth: 'auto' }}
                toolBarRender={() =>
                  canWrite
                    ? [
                        <Button
                          key="new"
                          type="primary"
                          onClick={async () => {
                            const res = await api.listCouponTemplates({
                              page: 1,
                              page_size: 100,
                              state: '已发布',
                            });
                            setTemplates(res.data ?? []);
                            setCreateBatchOpen(true);
                          }}
                        >
                          新建批次
                        </Button>,
                      ]
                    : []
                }
                request={async (params) => {
                  const res = await api.listCouponBatches({
                    page: params.current,
                    page_size: params.pageSize,
                    status: params.status,
                    channel: params.channel,
                  });
                  return {
                    data: res.data ?? [],
                    total: (res as any).page?.total ?? 0,
                    success: true,
                  };
                }}
                columns={batchColumns}
              />
            ),
          },
          {
            key: 'coupons',
            label: '发放记录',
            children: (
              <ProTable<IssuedCoupon>
                headerTitle="已发放券"
                actionRef={couponRef}
                rowKey={(row) => row.coupon_no || `${row.batch_no}${row.issued_at}`}
                search={{ labelWidth: 'auto' }}
                toolBarRender={() =>
                  canWrite
                    ? [
                        <Button key="publish" type="primary" onClick={() => setPublishOpen(true)}>
                          投放领券中心
                        </Button>,
                      ]
                    : undefined
                }
                request={async (params) => {
                  const res = await api.listIssuedCoupons({
                    page: params.current,
                    page_size: params.pageSize,
                    batch_no: params.batch_no,
                    state: params.state,
                  });
                  return {
                    data: res.data ?? [],
                    total: (res as any).page?.total ?? 0,
                    success: true,
                  };
                }}
                columns={couponColumns}
              />
            ),
          },
          {
            key: 'campaigns',
            label: '营销活动',
            children: (
              <ProTable<Campaign>
                headerTitle="营销活动"
                actionRef={campaignRef}
                rowKey="code"
                search={{ labelWidth: 'auto' }}
                toolBarRender={() =>
                  canWrite
                    ? [
                        <Button
                          key="new"
                          type="primary"
                          onClick={() => setCreateCampaignOpen(true)}
                        >
                          新建活动
                        </Button>,
                      ]
                    : []
                }
                request={async (params) => {
                  const res = await api.listCampaigns({
                    page: params.current,
                    page_size: params.pageSize,
                    keyword: params.name,
                    status: params.status,
                  });
                  return {
                    data: res.data ?? [],
                    total: (res as any).page?.total ?? 0,
                    success: true,
                  };
                }}
                columns={campaignColumns}
              />
            ),
          },
        ],
        }}
      />
      <ModalForm<any>
        title="新建券模板"
        open={createTemplateOpen}
        onOpenChange={(open) => {
          if (!open) setCreateTemplateOpen(false);
        }}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          await api.createCouponTemplate({
            ...values,
            valid_start: dayjs(values.valid_start).format('YYYY-MM-DD'),
            valid_end: dayjs(values.valid_end).format('YYYY-MM-DD'),
          });
          message.success('券模板已创建（草稿，需发布后发放）');
          setCreateTemplateOpen(false);
          reloadAll();
          return true;
        }}
      >
        <ProFormText
          name="template_no"
          label="模板编号"
          placeholder="如 TPL-NEW-001"
          rules={[{ required: true, message: '请填写模板编号' }]}
        />
        <ProFormText
          name="name"
          label="券名称"
          placeholder="如 满20减5"
          rules={[
            { required: true, message: '请填写券名称' },
            { min: 2, max: 60, message: '名称长度需在 2-60 字之间' },
          ]}
        />
        <ProFormSelect
          name="coupon_type"
          label="券类型"
          initialValue="cash"
          options={[
            { value: 'cash', label: '满减券' },
            { value: 'discount', label: '折扣券' },
            { value: 'new_user', label: '新客券' },
          ]}
          rules={[{ required: true }]}
        />
        <ProFormDigit
          name="face_value_fen"
          label="面额（分）"
          placeholder="500 表示 5 元"
          min={0}
          rules={[{ required: true, message: '请填写面额' }]}
        />
        <ProFormDigit
          name="threshold_fen"
          label="使用门槛（分）"
          placeholder="2000 表示满 20 元可用"
          min={0}
          tooltip="面额不能超过门槛，否则等于免单还倒贴，后端会拒绝"
        />
        <ProFormSelect
          name="bearer"
          label="承担方"
          initialValue="platform"
          options={[
            { value: 'platform', label: '平台承担' },
            { value: 'driver', label: '司机承担' },
            { value: 'share', label: '共同承担' },
          ]}
          rules={[{ required: true }]}
        />
        <ProFormDigit
          name="stock"
          label="库存（张）"
          min={1}
          rules={[{ required: true, message: '请填写库存' }]}
        />
        <ProFormDigit
          name="per_limit"
          label="每人限领（张）"
          min={1}
          initialValue={1}
          tooltip="不能超过库存"
          rules={[{ required: true, message: '请填写每人限领张数' }]}
        />
        <ProFormSwitch name="stackable" label="可与其他券叠加" />
        <ProFormDatePicker
          name="valid_start"
          label="生效时间"
          rules={[{ required: true, message: '请选择生效时间' }]}
        />
        <ProFormDatePicker
          name="valid_end"
          label="失效时间"
          rules={[{ required: true, message: '请选择失效时间' }]}
        />
        <ProFormTextArea
          name="description"
          label="说明"
          fieldProps={{ rows: 2, maxLength: 200, showCount: true }}
        />
      </ModalForm>

      <ModalForm<any>
        title="新建券批次"
        open={createBatchOpen}
        onOpenChange={(open) => {
          if (!open) setCreateBatchOpen(false);
        }}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          await api.createCouponBatch({
            ...values,
            start_at: dayjs(values.start_at).format('YYYY-MM-DD'),
            end_at: dayjs(values.end_at).format('YYYY-MM-DD'),
          });
          message.success('券批次已创建');
          setCreateBatchOpen(false);
          reloadAll();
          return true;
        }}
      >
        <ProFormText
          name="batch_no"
          label="批次号"
          placeholder="如 BATCH-20260701"
          rules={[{ required: true, message: '请填写批次号' }]}
        />
        <ProFormSelect
          name="template_no"
          label="券模板"
          rules={[{ required: true, message: '请选择券模板' }]}
          fieldProps={{
            options: templates.map((t) => ({
              value: t.template_no,
              label: `${t.name}（${t.template_no}）`,
            })),
          }}
        />
        <ProFormDigit
          name="total_count"
          label="发行总量（张）"
          min={1}
          tooltip="总量就是防超发上限，创建后不可上调，且不能超过券模板剩余库存"
          rules={[{ required: true, message: '请填写发行总量' }]}
        />
        <ProFormSelect
          name="channel"
          label="发放渠道"
          initialValue="manual"
          options={[
            { value: 'manual', label: '手工发放' },
            { value: 'new_user', label: '新客' },
            { value: 'activity', label: '活动' },
            { value: 'invite', label: '邀请' },
            { value: 'compensate', label: '补偿' },
          ]}
          rules={[{ required: true }]}
        />
        <ProFormDigit
          name="budget_fen"
          label="预算（分）"
          min={0}
          initialValue={0}
        />
        <ProFormText
          name="campaign_code"
          label="关联活动编码"
          placeholder="可留空"
        />
        <ProFormDatePicker
          name="start_at"
          label="发放开始"
          rules={[{ required: true, message: '请选择开始时间' }]}
        />
        <ProFormDatePicker
          name="end_at"
          label="发放结束"
          rules={[{ required: true, message: '请选择结束时间' }]}
        />
      </ModalForm>

      <ModalForm<any>
        title="新建营销活动"
        open={createCampaignOpen}
        onOpenChange={(open) => {
          if (!open) setCreateCampaignOpen(false);
        }}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          const {
            sc_trigger_type,
            sc_amount_fen,
            sc_max_per_driver,
            sc_daily_cap_fen,
            sc_task_type,
            sc_need_enroll,
            ...rest
          } = values;
          // 补贴类活动把表单里的补贴字段组装成结构化配置；其他活动不带。
          if (isSubsidyCampaign(rest.type)) {
            rest.subsidy_config = {
              trigger_type: sc_trigger_type,
              amount_fen: sc_amount_fen,
              max_per_driver: sc_max_per_driver,
              daily_cap_fen: sc_daily_cap_fen ?? 0,
              task_type: sc_task_type,
              need_enroll: sc_need_enroll ?? false,
            };
          }
          await api.createCampaign({
            ...rest,
            start_at: dayjs(values.start_at).format('YYYY-MM-DD'),
            end_at: dayjs(values.end_at).format('YYYY-MM-DD'),
          });
          message.success('活动已创建（草稿，需开始后生效）');
          setCreateCampaignOpen(false);
          reloadAll();
          return true;
        }}
      >
        <ProFormText
          name="code"
          label="活动编码"
          placeholder="如 CAMP-SUMMER-2026"
          rules={[{ required: true, message: '请填写活动编码' }]}
        />
        <ProFormText
          name="name"
          label="活动名称"
          rules={[
            { required: true, message: '请填写活动名称' },
            { min: 2, max: 60, message: '名称长度需在 2-60 字之间' },
          ]}
        />
        <ProFormSelect
          name="type"
          label="活动类型"
          rules={[{ required: true, message: '请选择活动类型' }]}
          options={Object.entries(CAMPAIGN_TYPE).map(([value, label]) => ({
            value,
            label,
          }))}
        />
        <ProFormSelect
          name="target_audience"
          label="面向人群"
          initialValue="passenger"
          options={[
            { value: 'passenger', label: '乘客' },
            { value: 'driver', label: '司机' },
            { value: 'all', label: '全部' },
          ]}
        />
        <ProFormDigit
          name="budget_fen"
          label="活动预算（分）"
          min={0}
          initialValue={0}
        />
        <ProFormDigit
          name="reward_amount_fen"
          label="奖励金额（分）"
          min={0}
          initialValue={0}
          tooltip="不能超过活动预算"
        />
        <ProFormDatePicker
          name="start_at"
          label="开始时间"
          rules={[{ required: true, message: '请选择开始时间' }]}
        />
        <ProFormDatePicker
          name="end_at"
          label="结束时间"
          rules={[{ required: true, message: '请选择结束时间' }]}
        />
        <ProFormTextArea
          name="rules"
          label="规则说明"
          fieldProps={{ rows: 3, maxLength: 500, showCount: true }}
        />
        <ProFormDependency name={['type']}>
          {({ type }) =>
            isSubsidyCampaign(type) ? (
              <>
                <ProFormSelect
                  name="sc_trigger_type"
                  label="补贴触发口径"
                  initialValue="per_order"
                  options={Object.entries(SUBSIDY_TRIGGER_LABEL).map(([value, label]) => ({
                    value,
                    label,
                  }))}
                  rules={[{ required: true, message: '请选择补贴触发口径' }]}
                />
                <ProFormDigit
                  name="sc_amount_fen"
                  label="单次补贴金额（分）"
                  placeholder="300 表示每笔 3 元，发放时金额以此为准"
                  min={1}
                  rules={[{ required: true, message: '请填写单次补贴金额' }]}
                />
                <ProFormDigit
                  name="sc_max_per_driver"
                  label="每司机累计次数上限"
                  min={1}
                  initialValue={1}
                  rules={[{ required: true, message: '请填写每司机累计次数上限' }]}
                />
                <ProFormDigit
                  name="sc_daily_cap_fen"
                  label="活动单日发放封顶（分）"
                  placeholder="0 表示不设日封顶"
                  min={0}
                  initialValue={0}
                  tooltip="单日发放达到封顶后当天不能再登记，次日自动恢复"
                />
                <ProFormDependency name={['sc_trigger_type']}>
                  {({ sc_trigger_type: trigger }) =>
                    trigger === 'task' ? (
                      <ProFormText
                        name="sc_task_type"
                        label="关联任务类型"
                        placeholder="如 complete_3_orders"
                        rules={[{ required: true, message: '任务口径补贴必须填写任务类型' }]}
                      />
                    ) : (
                      <></>
                    )
                  }
                </ProFormDependency>
                <ProFormSwitch name="sc_need_enroll" label="司机需要报名" />
              </>
            ) : (
              <></>
            )
          }
        </ProFormDependency>
      </ModalForm>
      <ModalForm<{ passenger_ids: string }>
        key={issueTarget?.batch_no ?? 'issue-idle'}
        title={issueTarget ? `发放优惠券 ${issueTarget.batch_no}` : '发放优惠券'}
        open={Boolean(issueTarget)}
        onOpenChange={(open) => {
          if (!open) setIssueTarget(undefined);
        }}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          if (!issueTarget) return true;
          const ids = String(values.passenger_ids)
            .split(/[\s,，]+/)
            .map((s) => s.trim())
            .filter(Boolean);
          if (ids.length === 0) {
            message.error('请填写至少一个乘客 ID');
            return false;
          }
          try {
            const result = await api.issueCoupons(issueTarget.batch_no, {
              passenger_ids: ids,
            });
            // 发放结果必须回显：被规则挡下的人要能看到原因，不能静默丢弃。
            setIssueResult(result);
            setIssueTarget(undefined);
            reloadAll();
            return true;
          } catch (error: any) {
            // 后端 409/429 的消息要原样传达给运营。
            message.error(error?.message ?? '发放失败');
            return false;
          }
        }}
      >
        <ProFormTextArea
          name="passenger_ids"
          label="乘客 ID 列表"
          placeholder="每行一个或用逗号分隔，如：&#10;6ad9adcc77e161c8570967b1&#10;6ad9adcc77e161c8570967b2"
          rules={[{ required: true, message: '请填写要发放的乘客 ID' }]}
          fieldProps={{ rows: 6 }}
        />
        {issueTarget && (
          <span style={{ color: '#999', fontSize: 12 }}>
            当前批次剩余 {issueTarget.total_count - issueTarget.issued_count} 张；
            超过总量、超过库存、达到每人限领或券互斥的乘客会被挡下并在结果里列出
          </span>
        )}
      </ModalForm>

      <ModalForm<{
        name: string;
        face_value_fen: number;
        threshold_fen: number;
        count: number;
        expire_at: string;
      }>
        key="publish-claim-center"
        title="投放领券中心"
        width={480}
        open={publishOpen}
        onOpenChange={setPublishOpen}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          try {
            const result = await api.publishCoupons({
              name: values.name,
              face_value_fen: values.face_value_fen,
              threshold_fen: values.threshold_fen ?? 0,
              count: values.count,
              expire_at: dayjs(values.expire_at).format('YYYY-MM-DD HH:mm:ss'),
            });
            message.success(
              `已投放 ${result.count} 张到领券中心，乘客先到先得（券码 ${result.coupon_nos?.[0] ?? ''} 等）`,
            );
            couponRef.current?.reload();
            return true;
          } catch (error: any) {
            message.error(error?.message ?? '投放失败');
            return false;
          }
        }}
      >
        <ProFormText
          name="name"
          label="券名称"
          placeholder="乘客端展示，默认：打车券"
          fieldProps={{ maxLength: 40 }}
        />
        <ProFormDigit
          name="face_value_fen"
          label="面值（分）"
          placeholder="1000 表示 10 元"
          min={1}
          rules={[{ required: true, message: '请填写面值' }]}
        />
        <ProFormDigit
          name="threshold_fen"
          label="使用门槛（分）"
          placeholder="0 表示无门槛"
          min={0}
          initialValue={0}
          tooltip="门槛必须大于面值，例如面值 500 分（5 元）可设门槛 2000 分（满 20 可用）"
        />
        <ProFormDigit
          name="count"
          label="投放张数"
          min={1}
          max={500}
          initialValue={20}
          tooltip="每张一个独立券码，乘客领取时先到先得，领完即止"
        />
        <ProFormDatePicker
          name="expire_at"
          label="券到期时间"
          rules={[{ required: true, message: '请选择到期时间' }]}
        />
      </ModalForm>

      <Modal
        title="发放结果"
        open={Boolean(issueResult)}
        onCancel={() => setIssueResult(undefined)}
        footer={[
          <Button key="ok" type="primary" onClick={() => setIssueResult(undefined)}>
            知道了
          </Button>,
        ]}
        width={680}
      >
        {issueResult && (
          <>
            <Descriptions column={2} size="small" bordered>
              <Descriptions.Item label="批次">{issueResult.batch_no}</Descriptions.Item>
              <Descriptions.Item label="券模板">
                {issueResult.template_no}
              </Descriptions.Item>
              <Descriptions.Item label="成功发出">
                <Tag color="green">{issueResult.issued} 张</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="剩余可发">
                {issueResult.remaining} 张
              </Descriptions.Item>
            </Descriptions>
            {issueResult.skipped && issueResult.skipped.length > 0 && (
              <>
                <p style={{ margin: '16px 0 8px', color: '#d46b08' }}>
                  以下 {issueResult.skipped.length} 人被规则挡下，未发放：
                </p>
                <Table
                  size="small"
                  rowKey={(row) => row.passenger_id}
                  pagination={false}
                  dataSource={issueResult.skipped}
                  columns={[
                    { title: '乘客 ID', dataIndex: 'passenger_id' },
                    { title: '手机号', dataIndex: 'phone', width: 130 },
                    { title: '原因', dataIndex: 'reason' },
                  ]}
                />
              </>
            )}
          </>
        )}
      </Modal>

      <Drawer
        title={
          settlement
            ? `活动对账 ${settlement.campaign_code} ${settlement.campaign_name}`
            : '活动对账'
        }
        rootStyle={{ width: 860 }}
        size="large"
        open={Boolean(settlement)}
        onClose={() => setSettlement(undefined)}
        extra={
          settlement && canSettle ? (
            <Button
              type="primary"
              onClick={async () => {
                try {
                  await api.exportCampaignSettlement(settlement.campaign_code);
                  message.success('对账单已导出');
                } catch {
                  message.error('导出失败');
                }
              }}
            >
              导出 CSV
            </Button>
          ) : undefined
        }
      >
        {settlement && (
          <>
            {!settlement.closed && (
              <Tag color="orange" style={{ marginBottom: 12 }}>
                活动尚未结束，这是实时数据，结束后再导出最终对账单
              </Tag>
            )}
            <Descriptions column={2} size="small" bordered>
              <Descriptions.Item label="活动">
                {settlement.campaign_code} {settlement.campaign_name}
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                {CAMPAIGN_STATUS[settlement.status] ?? settlement.status}
              </Descriptions.Item>
              <Descriptions.Item label="发放张数">
                {settlement.issued_count}
              </Descriptions.Item>
              <Descriptions.Item label="核销张数">
                {settlement.used_count}
              </Descriptions.Item>
              <Descriptions.Item label="未使用">
                {settlement.unused_count}
              </Descriptions.Item>
              <Descriptions.Item label="已过期">
                {settlement.expired_count}
              </Descriptions.Item>
              <Descriptions.Item label="发放金额">
                ¥{yuan(settlement.issue_amount_fen)}
              </Descriptions.Item>
              <Descriptions.Item label="核销金额">
                ¥{yuan(settlement.used_amount_fen)}
              </Descriptions.Item>
              <Descriptions.Item label="预算">
                ¥{yuan(settlement.used_budget_fen)} / ¥{yuan(settlement.budget_fen)}
              </Descriptions.Item>
              <Descriptions.Item label="预算使用率">
                <Progress
                  percent={Number(settlement.budget_usage_pct.toFixed(1))}
                  size="small"
                  style={{ width: 160 }}
                />
              </Descriptions.Item>
              <Descriptions.Item label="承担方拆分" span={2}>
                {Object.entries(settlement.bearer_split ?? {}).length === 0
                  ? '暂无'
                  : Object.entries(settlement.bearer_split).map(
                      ([bearer, amount]) => (
                        <Tag key={bearer} style={{ marginRight: 8 }}>
                          {BEARER_LABEL[bearer] ?? bearer} ¥{yuan(amount)}
                        </Tag>
                      ),
                    )}
              </Descriptions.Item>
            </Descriptions>

            <p style={{ margin: '16px 0 8px', fontWeight: 500 }}>按批次拆分</p>
            <Table
              size="small"
              rowKey="batch_no"
              pagination={false}
              dataSource={settlement.batches ?? []}
              columns={[
                { title: '批次', dataIndex: 'batch_no', width: 130 },
                { title: '券名称', dataIndex: 'template_name' },
                {
                  title: '承担方',
                  dataIndex: 'bearer',
                  width: 100,
                  render: (_, row) => BEARER_LABEL[row.bearer] ?? row.bearer,
                },
                { title: '发行', dataIndex: 'total_count', width: 70 },
                { title: '已发', dataIndex: 'issued_count', width: 70 },
                { title: '核销', dataIndex: 'used_count', width: 70 },
                {
                  title: '发放金额',
                  dataIndex: 'issue_amount_fen',
                  width: 100,
                  render: (_, row) => `¥${yuan(row.issue_amount_fen)}`,
                },
                {
                  title: '核销金额',
                  dataIndex: 'used_amount_fen',
                  width: 100,
                  render: (_, row) => `¥${yuan(row.used_amount_fen)}`,
                },
              ]}
            />
          </>
        )}
      </Drawer>

      <Drawer
        title={subsidyTarget ? `补贴台账 ${subsidyTarget.code} ${subsidyTarget.name}` : '补贴台账'}
        rootStyle={{ width: 900 }}
        size="large"
        open={Boolean(subsidyTarget)}
        onClose={() => {
          setSubsidyTarget(undefined);
          setSubsidySummary(undefined);
          setSubsidyRecords([]);
        }}
      >
        {subsidyTarget && (
          <>
            <Descriptions column={3} size="small" bordered>
              <Descriptions.Item label="触发口径">
                {SUBSIDY_TRIGGER_LABEL[subsidySummary?.trigger_type ?? ''] ?? '-'}
              </Descriptions.Item>
              <Descriptions.Item label="单次补贴">
                ¥{yuan(subsidySummary?.amount_fen)}
              </Descriptions.Item>
              <Descriptions.Item label="每人累计上限">
                {subsidySummary?.max_per_driver ?? '-'} 次
              </Descriptions.Item>
              <Descriptions.Item label="单日封顶">
                {subsidySummary && subsidySummary.daily_cap_fen > 0
                  ? `¥${yuan(subsidySummary.daily_cap_fen)}`
                  : '不封顶'}
              </Descriptions.Item>
              <Descriptions.Item label="累计发放">
                ¥{yuan(subsidySummary?.granted_total_fen)}（{subsidySummary?.record_count ?? 0} 笔）
              </Descriptions.Item>
              <Descriptions.Item label="今日已发">
                ¥{yuan(subsidySummary?.today_granted_fen)}
              </Descriptions.Item>
              <Descriptions.Item label="活动预算">
                ¥{yuan(subsidySummary?.used_budget_fen)} / ¥{yuan(subsidySummary?.budget_fen)}
              </Descriptions.Item>
              <Descriptions.Item label="剩余预算">
                ¥{yuan(subsidySummary?.budget_remain_fen)}
              </Descriptions.Item>
              <Descriptions.Item label="触达司机">
                {subsidySummary?.driver_count ?? 0} 人
              </Descriptions.Item>
            </Descriptions>

            <div
              style={{
                margin: '16px 0 8px',
                fontWeight: 500,
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
              }}
            >
              <span>发放记录</span>
              {canWrite && subsidyTarget.status === 'running' && (
                <ModalForm<any>
                  title={`登记补贴发放 ${subsidyTarget.code}`}
                  modalProps={{ destroyOnHidden: true }}
                  width={480}
                  trigger={<Button type="primary" size="small">登记补贴</Button>}
                  onFinish={async (values) => {
                    try {
                      const result = await api.grantSubsidy(subsidyTarget.code, {
                        driver_id: values.driver_id,
                        order_no: values.order_no,
                        biz_type: values.biz_type,
                        reason: values.reason,
                      });
                      message.success(
                        `已发放 ¥${yuan(result.record.amount_fen)}，剩余预算 ¥${yuan(result.budget_remain_fen)}`,
                      );
                      await openSubsidyDrawer(subsidyTarget);
                      reloadAll();
                      return true;
                    } catch (error: any) {
                      message.error(error?.message ?? '登记失败');
                      return false;
                    }
                  }}
                >
                  <ProFormText
                    name="driver_id"
                    label="司机 ID"
                    placeholder="填司机业务 ID"
                    rules={[{ required: true, message: '请填写司机 ID' }]}
                  />
                  <ProFormText
                    name="order_no"
                    label="关联订单号（可选）"
                    placeholder="因单补贴时填写订单号"
                  />
                  <ProFormSelect
                    name="biz_type"
                    label="补贴类型"
                    initialValue="other"
                    options={Object.entries(SUBSIDY_BIZ_LABEL).map(([value, label]) => ({
                      value,
                      label,
                    }))}
                    rules={[{ required: true }]}
                  />
                  <ProFormTextArea
                    name="reason"
                    label="事由说明"
                    placeholder="发放依据，会随台账与审计留痕"
                    fieldProps={{ rows: 3, maxLength: 200, showCount: true }}
                    rules={[{ required: true, message: '请填写事由' }]}
                  />
                  <span style={{ color: '#999', fontSize: 12 }}>
                    金额取自活动补贴配置；超过每人上限、当日封顶或活动预算会被后端拒绝
                  </span>
                </ModalForm>
              )}
            </div>
            <Table
              size="small"
              rowKey="record_no"
              pagination={false}
              dataSource={subsidyRecords}
              columns={[
                { title: '流水号', dataIndex: 'record_no', width: 170 },
                { title: '司机', dataIndex: 'driver_id', width: 130 },
                {
                  title: '金额',
                  dataIndex: 'amount_fen',
                  width: 90,
                  render: (_, row) => `¥${yuan(row.amount_fen)}`,
                },
                {
                  title: '类型',
                  dataIndex: 'biz_type',
                  width: 90,
                  render: (_, row) => SUBSIDY_BIZ_LABEL[row.biz_type] ?? row.biz_type,
                },
                { title: '事由', dataIndex: 'reason', ellipsis: true },
                { title: '归属日', dataIndex: 'day', width: 100 },
                {
                  title: '发放时间',
                  dataIndex: 'created_at',
                  width: 140,
                  render: (_, row) => fmt(row.created_at),
                },
              ]}
            />
          </>
        )}
      </Drawer>
    </>
  );
};

export default Marketing;
