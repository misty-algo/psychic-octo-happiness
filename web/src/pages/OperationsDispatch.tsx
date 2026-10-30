import type { Envelope } from '@/services/admin';
// A06 派单策略配置：听单排序权重、候选半径、时限与重派边界。
// 权重必须总和 10000，页面实时显示余量；发布前可以预览候选排序，
// 让运营看到「谁会被先叫到」到底变了没有。
import {
  ModalForm,
  ProFormDigit,
  ProFormSwitch,
  ProTable,
  type ProColumns,
} from '@ant-design/pro-components';
import { Alert, Button, Descriptions, Drawer, Progress, Space, Table, Tag, message } from 'antd';
import dayjs from 'dayjs';
import { useAccess } from '@umijs/max';
import { useRef, useState } from 'react';
import {
  type DispatchStrategy,
  type RankedCandidate,
  createDispatchStrategy,
  listDispatchStrategies,
  previewDispatchRanking,
  publishDispatchStrategy,
} from '@/services/operations';

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

const Dispatch = () => {
  const access = useAccess();
  const canWrite = Boolean(access.canOpsDispatchWrite);
  const [createOpen, setCreateOpen] = useState(false);
  const [previewTarget, setPreviewTarget] = useState<DispatchStrategy | undefined>();
  const [ranking, setRanking] = useState<RankedCandidate[]>([]);
  const tableRef = useRef<any>(undefined);

  const reload = () => tableRef.current?.reload();

  const openPreview = async (row: DispatchStrategy) => {
    setPreviewTarget(row);
    try {
      setRanking(await previewDispatchRanking(row.strategy_id));
    } catch (error: any) {
      message.error(error?.message ?? '预览候选排序失败');
      setRanking([]);
    }
  };

  const columns: ProColumns<DispatchStrategy>[] = [
    { title: '策略号', dataIndex: 'strategy_id', width: 120 },
    { title: '版本', dataIndex: 'version', width: 70 },
    {
      title: '权重分布',
      width: 260,
      render: (_, row) => {
        const w = row.weights;
        return (
          <span style={{ fontSize: 12 }}>
            收入 {w.income_bp} / 距离 {w.distance_bp} / 好评 {w.rating_bp} / 在线{' '}
            {w.online_sec_bp} / 成功率 {w.success_rate_bp}
          </span>
        );
      },
    },
    {
      title: '权重总和',
      width: 110,
      render: (_, row) => {
        const sum =
          row.weights.income_bp +
          row.weights.distance_bp +
          row.weights.rating_bp +
          row.weights.online_sec_bp +
          row.weights.success_rate_bp;
        return sum === 10000 ? <Tag color="green">10000</Tag> : <Tag color="red">{sum}</Tag>;
      },
    },
    {
      title: '候选半径',
      dataIndex: 'radius_m',
      width: 90,
      render: (_, row) => `${row.radius_m} m`,
    },
    {
      title: '时限',
      width: 180,
      render: (_, row) =>
        `邀请 ${row.invite_sec}s / 听单 ${row.listen_sec}s + 抢单 ${row.grab_sec}s = ${row.total_sec}s`,
    },
    {
      title: '重派边界',
      width: 140,
      render: (_, row) =>
        `额外窗口 ${row.redispatch.extra_window_sec}s，退回上限 ${row.redispatch.max_accepted_return} 次`,
    },
    { title: '区域开关', dataIndex: 'region_switch', width: 90, render: (_, row) => (row.region_switch ? '开' : '关') },
    {
      title: '状态',
      dataIndex: 'state',
      width: 90,
      render: (_, row) => <Tag color={STATE_COLOR[row.state]}>{STATE_LABEL[row.state]}</Tag>,
    },
    {
      title: '操作',
      width: 160,
      render: (_, row) => (
        <Space size={4}>
          <a onClick={() => openPreview(row)}>排序预览</a>
          {canWrite && row.state === 'draft' && (
            <a
              onClick={async () => {
                try {
                  await publishDispatchStrategy(row.strategy_id);
                  message.success('已发布派单策略');
                  reload();
                } catch (error: any) {
                  message.error(error?.message ?? '发布失败');
                }
              }}
            >
              发布
            </a>
          )}
        </Space>
      ),
    },
  ];

  return (
    <>
      <ProTable<DispatchStrategy>
        actionRef={tableRef}
        rowKey="strategy_id"
        headerTitle="派单策略"
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
          const res: Envelope<DispatchStrategy[]> = await listDispatchStrategies({
            page,
            page_size: pageSize,
            state: params.state as string,
          });
          return { data: res.data ?? [], total: res.page?.total ?? 0, success: true };
        }}
      />

      <ModalForm<{
        income_bp: number;
        distance_bp: number;
        rating_bp: number;
        online_sec_bp: number;
        success_rate_bp: number;
        radius_m: number;
        invite_sec: number;
        total_sec: number;
        listen_sec: number;
        grab_sec: number;
        region_switch: boolean;
        extra_window_sec: number;
        max_accepted_return: number;
      }>
        title="新建派单策略草稿"
        open={createOpen}
        onOpenChange={setCreateOpen}
        modalProps={{ destroyOnHidden: true }}
        width={560}
        initialValues={{
          income_bp: 4000,
          distance_bp: 3000,
          rating_bp: 1500,
          online_sec_bp: 1000,
          success_rate_bp: 500,
          radius_m: 5000,
          invite_sec: 15,
          total_sec: 120,
          listen_sec: 60,
          grab_sec: 60,
          region_switch: true,
          extra_window_sec: 60,
          max_accepted_return: 2,
        }}
        onFinish={async (values) => {
          const sum =
            values.income_bp +
            values.distance_bp +
            values.rating_bp +
            values.online_sec_bp +
            values.success_rate_bp;
          if (sum !== 10000) {
            message.error(`权重总和必须等于 10000，当前 ${sum}`);
            return false;
          }
          try {
            await createDispatchStrategy(values);
            message.success('草稿已创建，可预览排序后发布');
            setCreateOpen(false);
            reload();
            return true;
          } catch (error: any) {
            message.error(error?.message ?? '创建失败');
            return false;
          }
        }}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="五项权重总和必须等于 10000"
          description="0-10000 整数归一，总和不是 10000 后端会直接拒绝；全 0 同样拒绝。"
        />
        <ProFormDigit name="income_bp" label="收入权重（收入少的先派）" min={0} max={10000} />
        <ProFormDigit name="distance_bp" label="距离权重" min={0} max={10000} />
        <ProFormDigit name="rating_bp" label="好评率权重" min={0} max={10000} />
        <ProFormDigit name="online_sec_bp" label="在线时长权重" min={0} max={10000} />
        <ProFormDigit name="success_rate_bp" label="成功率权重" min={0} max={10000} />
        <ProFormDigit name="radius_m" label="候选半径（米）" min={1} />
        <ProFormDigit name="invite_sec" label="邀请应答时限（秒）" min={1} />
        <ProFormDigit name="listen_sec" label="听单时长（秒）" min={1} />
        <ProFormDigit name="grab_sec" label="抢单时长（秒）" min={1} />
        <ProFormDigit name="total_sec" label="总窗口（秒）" min={1} tooltip="必须等于听单 + 抢单" />
        <ProFormDigit name="extra_window_sec" label="重派额外窗口（秒）" min={1} />
        <ProFormDigit name="max_accepted_return" label="已接单退回次数上限" min={1} />
        <ProFormSwitch name="region_switch" label="区域开关（可关停单城派单）" />
      </ModalForm>

      <Drawer
        title={previewTarget ? `候选排序预览 ${previewTarget.strategy_id}` : '候选排序预览'}
        rootStyle={{ width: 760 }}
        size="large"
        open={Boolean(previewTarget)}
        onClose={() => setPreviewTarget(undefined)}
      >
        {previewTarget && (
          <>
            <Descriptions column={2} size="small" bordered>
              <Descriptions.Item label="版本">V{previewTarget.version}</Descriptions.Item>
              <Descriptions.Item label="状态">
                {STATE_LABEL[previewTarget.state]}
              </Descriptions.Item>
              <Descriptions.Item label="候选半径">{previewTarget.radius_m} m</Descriptions.Item>
              <Descriptions.Item label="发布人">{previewTarget.operator || '-'}</Descriptions.Item>
            </Descriptions>
            <div style={{ margin: '16px 0 8px', fontWeight: 500 }}>
              排序规则：先按本月已结算收入升序，收入相同按综合评分降序
            </div>
            <Table
              size="small"
              rowKey={(row) => row.driver.driver_id}
              pagination={false}
              dataSource={ranking}
              columns={[
                { title: '名次', dataIndex: 'rank', width: 70 },
                { title: '司机', dataIndex: ['driver', 'driver_id'], width: 200 },
                {
                  title: '本月收入',
                  dataIndex: ['driver', 'income_fen'],
                  width: 100,
                  render: (_, row) => `¥${((row.driver.income_fen ?? 0) / 100).toFixed(2)}`,
                },
                {
                  title: '距离',
                  dataIndex: ['driver', 'distance_m'],
                  width: 90,
                  render: (_, row) => `${row.driver.distance_m} m`,
                },
                {
                  title: '综合评分',
                  dataIndex: 'score',
                  width: 160,
                  render: (_, row) => (
                    <Progress
                      percent={Math.round(row.score / 100)}
                      size="small"
                      format={() => `${row.score}`}
                    />
                  ),
                },
                {
                  title: '更新时间',
                  width: 140,
                  render: () => dayjs().format('YYYY-MM-DD HH:mm'),
                },
              ]}
            />
            {ranking.length === 0 && (
              <Alert
                style={{ marginTop: 16 }}
                type="info"
                showIcon
                message="暂无候选司机数据"
                description="库里还没有派单尝试记录，可先传候选司机指标做预览。"
              />
            )}
          </>
        )}
      </Drawer>
    </>
  );
};

export default Dispatch;
