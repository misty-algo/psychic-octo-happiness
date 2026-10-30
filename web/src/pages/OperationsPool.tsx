import type { Envelope } from '@/services/admin';
// A09 拼车与特价：匹配参数、特价固定价方案与拼车口径统计。
// 拼车的商业承诺是「提交时看到的价就是最终价」，所以固定价方案是承诺价，
// 拼未成由平台承担优惠差额——统计三件套（匹配成功率/共乘率/拼未成数）
// 看的就是这个承诺兑现得怎么样。
import { ModalForm, ProCard, ProFormDigit, ProFormText, ProTable, type ProColumns } from '@ant-design/pro-components';
import { Button, Card, Col, Row, Space, Statistic, Tag, message } from 'antd';
import dayjs from 'dayjs';
import { useAccess } from '@umijs/max';
import { useEffect, useRef, useState } from 'react';
import {
  type PoolConfig,
  type PoolStats,
  type SpecialPricePlan,
  createPoolConfig,
  createSpecialPlan,
  listPoolConfigs,
  listSpecialPlans,
  publishPoolConfig,
  publishSpecialPlan,
  readPoolStats,
} from '@/services/operations';

const yuan = (fen?: number) => ((fen ?? 0) / 100).toFixed(2);
const fmt = (value?: string) => (value ? dayjs(value).format('YYYY-MM-DD') : '-');

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

const Pool = () => {
  const access = useAccess();
  const canWrite = Boolean(access.canOpsMarketingWrite);
  const [configOpen, setConfigOpen] = useState(false);
  const [planOpen, setPlanOpen] = useState(false);
  const [stats, setStats] = useState<PoolStats | undefined>();
  const configRef = useRef<any>(undefined);
  const planRef = useRef<any>(undefined);

  const loadStats = async () => {
    try {
      setStats(await readPoolStats());
    } catch (error: any) {
      message.error(error?.message ?? '读取拼车统计失败');
    }
  };

  useEffect(() => {
    loadStats();
  }, []);

  const configColumns: ProColumns<PoolConfig>[] = [
    { title: '配置号', dataIndex: 'config_id', width: 120 },
    { title: '版本', dataIndex: 'version', width: 70 },
    {
      title: '封组等待',
      dataIndex: 'match_sec',
      width: 100,
      render: (_, row) => `${row.match_sec} 秒`,
    },
    {
      title: '每组上限',
      width: 120,
      render: (_, row) => `${row.group_max} 单 / ${row.seat_max} 座`,
    },
    {
      title: '适用城市',
      dataIndex: 'city_codes',
      render: (_, row) => row.city_codes?.join('、') || '全部',
    },
    {
      title: '状态',
      dataIndex: 'state',
      width: 90,
      render: (_, row) => <Tag color={STATE_COLOR[row.state]}>{STATE_LABEL[row.state]}</Tag>,
    },
    {
      title: '操作',
      width: 90,
      render: (_, row) =>
        canWrite && row.state === 'draft' ? (
          <a
            onClick={async () => {
              try {
                await publishPoolConfig(row.config_id);
                message.success('已发布拼车配置');
                configRef.current?.reload();
              } catch (error: any) {
                message.error(error?.message ?? '发布失败');
              }
            }}
          >
            发布
          </a>
        ) : (
          '-'
        ),
    },
  ];

  const planColumns: ProColumns<SpecialPricePlan>[] = [
    { title: '方案号', dataIndex: 'plan_id', width: 180 },
    { title: '名称', dataIndex: 'name', width: 140 },
    {
      title: '城市/车型',
      width: 140,
      render: (_, row) => `${row.city_code} / ${row.car_type}`,
    },
    {
      title: '固定价',
      dataIndex: 'fixed_fen',
      width: 100,
      render: (_, row) => `¥${yuan(row.fixed_fen)}`,
    },
    { title: '座位', dataIndex: 'seats', width: 70 },
    {
      title: '有效期',
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
      width: 90,
      render: (_, row) =>
        canWrite && row.state === 'draft' ? (
          <a
            onClick={async () => {
              try {
                await publishSpecialPlan(row.plan_id);
                message.success('已发布特价方案');
                planRef.current?.reload();
              } catch (error: any) {
                message.error(error?.message ?? '发布失败');
              }
            }}
          >
            发布
          </a>
        ) : (
          '-'
        ),
    },
  ];

  return (
    <>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card>
            <Statistic title="拼车订单数" value={stats?.pool_order_count ?? 0} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="匹配成功率"
              value={stats?.match_success_rate ?? 0}
              precision={1}
              suffix="%"
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="实际共乘率"
              value={stats?.shared_rate ?? 0}
              precision={1}
              suffix="%"
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="拼未成数"
              value={stats?.unmatched_count ?? 0}
              valueStyle={{ color: (stats?.unmatched_count ?? 0) > 0 ? '#cf1322' : undefined }}
            />
          </Card>
        </Col>
      </Row>

      <ProCard
        tabs={{
          items: [
            {
              key: 'configs',
              label: '拼车匹配配置',
              children: (
                <ProTable<PoolConfig>
                  actionRef={configRef}
                  rowKey="config_id"
                  columns={configColumns}
                  search={false}
                  toolBarRender={() => [
                    canWrite && (
                      <Button key="new" type="primary" onClick={() => setConfigOpen(true)}>
                        新建草稿
                      </Button>
                    ),
                  ]}
                  request={async (params) => {
                    const page = params.current ?? 1;
                    const pageSize = params.pageSize ?? 10;
                    const res: Envelope<PoolConfig[]> = await listPoolConfigs({
                      page,
                      page_size: pageSize,
                      state: params.state as string,
                    });
                    return { data: res.data ?? [], total: res.page?.total ?? 0, success: true };
                  }}
                />
              ),
            },
            {
              key: 'plans',
              label: '特价固定价方案',
              children: (
                <ProTable<SpecialPricePlan>
                  actionRef={planRef}
                  rowKey="plan_id"
                  columns={planColumns}
                  search={false}
                  toolBarRender={() => [
                    canWrite && (
                      <Button key="new" type="primary" onClick={() => setPlanOpen(true)}>
                        新建方案
                      </Button>
                    ),
                  ]}
                  request={async (params) => {
                    const page = params.current ?? 1;
                    const pageSize = params.pageSize ?? 10;
                    const res: Envelope<SpecialPricePlan[]> = await listSpecialPlans({
                      page,
                      page_size: pageSize,
                      state: params.state as string,
                    });
                    return { data: res.data ?? [], total: res.page?.total ?? 0, success: true };
                  }}
                />
              ),
            },
          ],
        }}
      />

      <ModalForm<{
        match_sec: number;
        group_max: number;
        seat_max: number;
        city_codes?: string;
      }>
        title="新建拼车匹配配置"
        open={configOpen}
        onOpenChange={setConfigOpen}
        modalProps={{ destroyOnHidden: true }}
        width={480}
        initialValues={{ match_sec: 60, group_max: 2, seat_max: 4 }}
        onFinish={async (values) => {
          try {
            await createPoolConfig({
              match_sec: values.match_sec,
              group_max: values.group_max,
              seat_max: values.seat_max,
              city_codes: values.city_codes ? values.city_codes.split(',').map((s) => s.trim()) : [],
            });
            message.success('草稿已创建，发布后生效');
            setConfigOpen(false);
            configRef.current?.reload();
            return true;
          } catch (error: any) {
            message.error(error?.message ?? '创建失败');
            return false;
          }
        }}
      >
        <ProFormDigit
          name="match_sec"
          label="第一单等待封组时长（秒）"
          min={1}
          max={180}
          tooltip="拼车首单最多等 60 秒封组；匹配加首次派单总共不超过 180 秒"
        />
        <ProFormDigit name="group_max" label="每组最多订单数" min={1} max={2} />
        <ProFormDigit name="seat_max" label="每组最多座位数" min={1} max={4} />
        <ProFormText name="city_codes" label="适用城市（逗号分隔，留空为全部）" />
      </ModalForm>

      <ModalForm<{
        name: string;
        city_code: string;
        car_type: string;
        fixed_fen: number;
        seats: number;
        valid_from: string;
        valid_to: string;
      }>
        title="新建特价固定价方案"
        open={planOpen}
        onOpenChange={setPlanOpen}
        modalProps={{ destroyOnHidden: true }}
        width={480}
        initialValues={{ city_code: '310100', car_type: 'economy', seats: 4 }}
        onFinish={async (values) => {
          try {
            await createSpecialPlan({
              ...values,
              valid_from: dayjs(values.valid_from).format('YYYY-MM-DD'),
              valid_to: dayjs(values.valid_to).format('YYYY-MM-DD'),
            });
            message.success('特价方案已创建，发布后生效');
            setPlanOpen(false);
            planRef.current?.reload();
            return true;
          } catch (error: any) {
            message.error(error?.message ?? '创建失败');
            return false;
          }
        }}
      >
        <ProFormText name="name" label="方案名称" rules={[{ required: true }]} />
        <ProFormText name="city_code" label="城市编码" rules={[{ required: true }]} />
        <ProFormText name="car_type" label="车型" rules={[{ required: true }]} />
        <ProFormDigit
          name="fixed_fen"
          label="固定价（分）"
          min={1}
          tooltip="固定价是承诺价，不再叠加里程时长，也不重复扣活动差价"
          rules={[{ required: true }]}
        />
        <ProFormDigit name="seats" label="座位数" min={0} max={6} />
        <ProFormText name="valid_from" label="生效开始（YYYY-MM-DD）" rules={[{ required: true }]} />
        <ProFormText name="valid_to" label="生效结束（YYYY-MM-DD）" rules={[{ required: true }]} />
      </ModalForm>
    </>
  );
};

export default Pool;
