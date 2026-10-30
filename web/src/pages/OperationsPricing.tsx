import type { Envelope } from '@/services/admin';
// A05 计价规则管理：版本化的城市/车型价格配置。
// 页面刻意把「分账样例」放在发布按钮旁边——改价是高风险动作，运营必须先看
// 到 T/Q/H/K/F/P/D/N 对得上，再点发布。
import {
  ModalForm,
  ProCard,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProTable,
  type ProColumns,
} from '@ant-design/pro-components';
import { Alert, Button, Descriptions, Drawer, Space, Table, Tag, message } from 'antd';
import dayjs from 'dayjs';
import { useAccess } from '@umijs/max';
import { useEffect, useRef, useState } from 'react';
import {
  type FareSample,
  type PricingRule,
  createPricingRule,
  disablePricingRule,
  listPricingRules,
  previewPricingFare,
  publishPricingRule,
} from '@/services/operations';
import * as api from '@/services/operations';

const yuan = (fen?: number) => ((fen ?? 0) / 100).toFixed(2);
const fmt = (value?: string) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '-');

const STATE_LABEL: Record<string, string> = {
  draft: '草稿',
  published: '已发布',
  disabled: '已停用',
};
const STATE_COLOR: Record<string, string> = {
  draft: 'default',
  published: 'green',
  disabled: 'red',
};

const Pricing = () => {
  const access = useAccess();
  const canWrite = Boolean(access.canOpsPricingWrite);
  const canPublish = Boolean(access.canOpsPricingPublish);
  const [createOpen, setCreateOpen] = useState(false);
  const [previewTarget, setPreviewTarget] = useState<PricingRule | undefined>();
  const [samples, setSamples] = useState<FareSample[]>([]);
  const [effective, setEffective] = useState<PricingRule | undefined>();
  const tableRef = useRef<any>(undefined);

  const reload = () => tableRef.current?.reload();

  const openPreview = async (row: PricingRule) => {
    setPreviewTarget(row);
    try {
      setSamples(await api.previewPricingFare({ rule_id: row.rule_id }));
    } catch (error: any) {
      message.error(error?.message ?? '预览分账样例失败');
      setSamples([]);
    }
  };

  const queryEffective = async () => {
    try {
      setEffective(await api.readEffectivePricingRule('310100', 'economy'));
    } catch {
      // 无生效价（404）是正常业务态：页面顶部的「无有效价」告警条已表达，不再弹错误 toast。
      setEffective(undefined);
    }
  };

  useEffect(() => {
    queryEffective();
  }, []);

  const columns: ProColumns<PricingRule>[] = [
    { title: '规则号', dataIndex: 'rule_id', width: 180 },
    { title: '版本', dataIndex: 'version', width: 70 },
    { title: '城市', dataIndex: 'city_code', width: 90 },
    { title: '区域', dataIndex: 'area', width: 100, render: (_, row) => row.area || '全城' },
    { title: '车型', dataIndex: 'car_type', width: 100 },
    {
      title: '起步价',
      dataIndex: 'start_fen',
      width: 90,
      render: (_, row) => `¥${yuan(row.start_fen)}`,
    },
    {
      title: '里程价',
      dataIndex: 'per_km_fen',
      width: 90,
      render: (_, row) => `¥${yuan(row.per_km_fen)}/km`,
    },
    {
      title: '时长价',
      dataIndex: 'per_min_fen',
      width: 90,
      render: (_, row) => `¥${yuan(row.per_min_fen)}/min`,
    },
    {
      title: '溢价',
      dataIndex: 'surge_bp',
      width: 80,
      render: (_, row) => `${(row.surge_bp / 100).toFixed(0)}%`,
    },
    {
      title: '固定价',
      dataIndex: 'fixed_fen',
      width: 90,
      render: (_, row) => (row.fixed_fen > 0 ? `¥${yuan(row.fixed_fen)}` : '-'),
    },
    { title: '优先级', dataIndex: 'priority', width: 70 },
    {
      title: '生效区间',
      width: 200,
      render: (_, row) => `${fmt(row.valid_from)} ~ ${fmt(row.valid_to)}`,
    },
    {
      title: '状态',
      dataIndex: 'state',
      width: 90,
      render: (_, row) => <Tag color={STATE_COLOR[row.state]}>{STATE_LABEL[row.state]}</Tag>,
    },
    {
      title: '操作',
      width: 200,
      render: (_, row) => (
        <Space size={4}>
          <a onClick={() => openPreview(row)}>分账样例</a>
          {canPublish && row.state === 'draft' && (
            <a
              onClick={async () => {
                try {
                  await publishPricingRule(row.rule_id);
                  message.success('已发布新计价版本');
                  reload();
                  queryEffective();
                } catch (error: any) {
                  message.error(error?.message ?? '发布失败');
                }
              }}
            >
              发布
            </a>
          )}
          {canPublish && row.state === 'published' && (
            <a
              onClick={async () => {
                try {
                  await disablePricingRule(row.rule_id);
                  message.success('已停用，请发新版本接上');
                  reload();
                } catch (error: any) {
                  message.error(error?.message ?? '停用失败');
                }
              }}
            >
              停用
            </a>
          )}
        </Space>
      ),
    },
  ];

  return (
    <>
      {!effective && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="310100 / economy 当前没有生效中的计价版本"
          description="没有有效价时必须停止该车型新单估价并告警，不能用一个错价接单。请尽快发布计价规则。"
        />
      )}
      <ProTable<PricingRule>
        actionRef={tableRef}
        rowKey="rule_id"
        headerTitle="计价规则"
        columns={columns}
        search={false}
        toolBarRender={() => [
          canWrite && (
            <Button key="new" type="primary" onClick={() => setCreateOpen(true)}>
              新建草稿
            </Button>
          ),
        ]}
        request={async (params) => {
          const page = params.current ?? 1;
          const pageSize = params.pageSize ?? 10;
          const res: Envelope<PricingRule[]> = await listPricingRules({
            page,
            page_size: pageSize,
            city_code: params.city_code as string,
            car_type: params.car_type as string,
            state: params.state as string,
          });
          return { data: res.data ?? [], total: res.page?.total ?? 0, success: true };
        }}
      />

      <ModalForm<{
        city_code: string;
        area?: string;
        car_type: string;
        start_fen: number;
        include_m: number;
        per_km_fen: number;
        per_min_fen: number;
        service_bp: number;
        comm_bp: number;
        surge_bp: number;
        discount_bp: number;
        cancel_fen: number;
        fixed_fen: number;
        priority: number;
        valid_from: string;
        valid_to: string;
      }>
        title="新建计价规则草稿"
        open={createOpen}
        onOpenChange={setCreateOpen}
        modalProps={{ destroyOnHidden: true }}
        width={560}
        initialValues={{
          city_code: '310100',
          car_type: 'economy',
          start_fen: 800,
          include_m: 3000,
          per_km_fen: 200,
          per_min_fen: 50,
          service_bp: 500,
          comm_bp: 2000,
          surge_bp: 10000,
          discount_bp: 10000,
          cancel_fen: 0,
          fixed_fen: 0,
          priority: 0,
        }}
        onFinish={async (values) => {
          try {
            await createPricingRule({
              ...values,
              valid_from: dayjs(values.valid_from).format('YYYY-MM-DD'),
              valid_to: dayjs(values.valid_to).format('YYYY-MM-DD'),
            });
            message.success('草稿已创建，请先看分账样例再发布');
            setCreateOpen(false);
            reload();
            return true;
          } catch (error: any) {
            message.error(error?.message ?? '创建失败');
            return false;
          }
        }}
      >
        <ProFormText name="city_code" label="城市编码" rules={[{ required: true }]} />
        <ProFormText name="area" label="区域（可空表示全城）" />
        <ProFormText name="car_type" label="车型/业务类型" rules={[{ required: true }]} />
        <ProFormDigit
          name="start_fen"
          label="起步价（分）"
          min={0}
          rules={[{ required: true }]}
        />
        <ProFormDigit
          name="include_m"
          label="起步包含里程（米）"
          min={0}
          rules={[{ required: true }]}
        />
        <ProFormDigit
          name="per_km_fen"
          label="每公里价（分）"
          min={0}
          rules={[{ required: true }]}
        />
        <ProFormDigit
          name="per_min_fen"
          label="每分钟价（分）"
          min={0}
          rules={[{ required: true }]}
        />
        <ProFormDigit
          name="service_bp"
          label="乘客服务费（万分比）"
          min={0}
          max={10000}
          tooltip="500 表示 5%"
          rules={[{ required: true }]}
        />
        <ProFormDigit
          name="comm_bp"
          label="司机佣金（万分比）"
          min={0}
          max={10000}
          rules={[{ required: true }]}
        />
        <ProFormDigit
          name="surge_bp"
          label="溢价系数（万分比）"
          min={1}
          max={100000}
          tooltip="10000 为不溢价，12000 为 1.2 倍"
          rules={[{ required: true }]}
        />
        <ProFormDigit
          name="discount_bp"
          label="拼车折扣（万分比）"
          min={0}
          max={10000}
          rules={[{ required: true }]}
        />
        <ProFormDigit name="cancel_fen" label="取消费（分）" min={0} />
        <ProFormDigit
          name="fixed_fen"
          label="固定价（分）"
          min={0}
          tooltip="大于 0 时按固定价计价，不再叠加里程时长（特价方案用）"
        />
        <ProFormDigit name="priority" label="优先级" min={0} tooltip="区域重叠时取优先级高者" />
        <ProFormSelect
          name="valid_from"
          label="生效开始"
          options={[
            { value: dayjs().format('YYYY-MM-DD'), label: '今天' },
            { value: dayjs().add(1, 'day').format('YYYY-MM-DD'), label: '明天' },
          ]}
          rules={[{ required: true }]}
        />
        <ProFormText
          name="valid_to"
          label="生效结束（YYYY-MM-DD）"
          rules={[{ required: true }]}
        />
      </ModalForm>

      <Drawer
        title={previewTarget ? `分账样例 ${previewTarget.rule_id}` : '分账样例'}
        rootStyle={{ width: 860 }}
        size="large"
        open={Boolean(previewTarget)}
        onClose={() => setPreviewTarget(undefined)}
      >
        {previewTarget && (
          <>
            <Descriptions column={3} size="small" bordered>
              <Descriptions.Item label="规则">
                {previewTarget.city_code} / {previewTarget.car_type}
              </Descriptions.Item>
              <Descriptions.Item label="版本">V{previewTarget.version}</Descriptions.Item>
              <Descriptions.Item label="状态">
                {STATE_LABEL[previewTarget.state]}
              </Descriptions.Item>
              <Descriptions.Item label="起步价">
                ¥{yuan(previewTarget.start_fen)}（含 {previewTarget.include_m} 米）
              </Descriptions.Item>
              <Descriptions.Item label="里程价">
                ¥{yuan(previewTarget.per_km_fen)}/km
              </Descriptions.Item>
              <Descriptions.Item label="时长价">
                ¥{yuan(previewTarget.per_min_fen)}/min
              </Descriptions.Item>
              <Descriptions.Item label="服务费">
                {(previewTarget.service_bp / 100).toFixed(2)}%
              </Descriptions.Item>
              <Descriptions.Item label="司机佣金">
                {(previewTarget.comm_bp / 100).toFixed(2)}%
              </Descriptions.Item>
              <Descriptions.Item label="溢价">
                {(previewTarget.surge_bp / 100).toFixed(2)}%
              </Descriptions.Item>
            </Descriptions>

            <div style={{ margin: '16px 0 8px', fontWeight: 500 }}>
              分账样例（发布前必看，资金守恒：P = D + N + G）
            </div>
            <Table
              size="small"
              rowKey="name"
              pagination={false}
              dataSource={samples}
              columns={[
                { title: '样例', dataIndex: 'name', width: 100 },
                {
                  title: '输入',
                  width: 150,
                  render: (_, row) => `${(row.input.dist_m / 1000).toFixed(1)}km / ${Math.round(row.input.dur_s / 60)}min`,
                },
                {
                  title: 'T 里程时长费',
                  dataIndex: ['fare', 't_fen'],
                  width: 110,
                  render: (_, row) => `¥${yuan(row.fare.t_fen)}`,
                },
                {
                  title: 'Q 溢价后',
                  dataIndex: ['fare', 'q_fen'],
                  width: 100,
                  render: (_, row) => `¥${yuan(row.fare.q_fen)}`,
                },
                {
                  title: 'H 服务费',
                  dataIndex: ['fare', 'h_fen'],
                  width: 100,
                  render: (_, row) => `¥${yuan(row.fare.h_fen)}`,
                },
                {
                  title: 'K 佣金',
                  dataIndex: ['fare', 'k_fen'],
                  width: 90,
                  render: (_, row) => `¥${yuan(row.fare.k_fen)}`,
                },
                {
                  title: 'P 乘客应付',
                  dataIndex: ['fare', 'p_fen'],
                  width: 110,
                  render: (_, row) => `¥${yuan(row.fare.p_fen)}`,
                },
                {
                  title: 'D 司机应得',
                  dataIndex: ['fare', 'd_fen'],
                  width: 110,
                  render: (_, row) => `¥${yuan(row.fare.d_fen)}`,
                },
                {
                  title: 'N 平台净额',
                  dataIndex: ['fare', 'n_fen'],
                  width: 110,
                  render: (_, row) => `¥${yuan(row.fare.n_fen)}`,
                },
                {
                  title: 'G 守恒校验',
                  dataIndex: ['fare', 'g_fen'],
                  width: 100,
                  render: (_, row) =>
                    row.fare.g_fen === 0 ? (
                      <Tag color="green">守恒</Tag>
                    ) : (
                      <Tag color="red">{row.fare.g_fen}</Tag>
                    ),
                },
              ]}
            />
            <ProCard style={{ marginTop: 16 }}>
              <Alert
                type={samples.every((s) => s.fare.g_fen === 0) ? 'success' : 'error'}
                showIcon
                message={
                  samples.every((s) => s.fare.g_fen === 0)
                    ? '三个样例资金守恒，可以发布'
                    : '样例分账不守恒，禁止发布'
                }
              />
            </ProCard>
          </>
        )}
      </Drawer>
    </>
  );
};

export default Pricing;
