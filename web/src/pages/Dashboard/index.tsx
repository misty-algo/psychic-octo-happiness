// 花小猪打车运营看板：指标全部来自真实业务接口（订单/审核/工单/拼车/计价），
// 不是写死的示例数据，用于快速判断后台各业务是否有真实数据支撑。
import { PageContainer } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Card, Col, Row, Space, Statistic } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useState } from 'react';
import {
  listDriverAudits,
  listOrders,
  listPricingRules,
  listTickets,
  readPoolStats,
} from '@/services/operations';

type Overview = {
  todayOrders: number;
  runningOrders: number;
  pendingAudits: number;
  pendingTickets: number;
  sharedRate: number;
  publishedRules: number;
};

const emptyOverview = (): Overview => ({
  todayOrders: 0,
  runningOrders: 0,
  pendingAudits: 0,
  pendingTickets: 0,
  sharedRate: 0,
  publishedRules: 0,
});

const Dashboard: React.FC = () => {
  const [loading, setLoading] = useState(true);
  const [overview, setOverview] = useState<Overview>(emptyOverview);

  useEffect(() => {
    let active = true;
    const load = async () => {
      setLoading(true);
      try {
        // 各项独立取数：任一接口失败只影响对应卡片，不让整页空掉。
        const [today, running, audits, tickets, pool, rules] = await Promise.all([
          listOrders({ page: 1, page_size: 1, from: dayjs().startOf('day').toISOString() }).catch(() => undefined),
          // 行程中 = 已接单到结束前，用乘客域状态枚举（后端支持逗号分隔多状态）。
          listOrders({ page: 1, page_size: 1, state: 'ACCEPTED,ARRIVED,IN_SERVICE' }).catch(() => undefined),
          // 审核状态字典是中文（待审/通过/驳回），与后端 SSOT 保持一致。
          listDriverAudits({ page: 1, page_size: 1, state: '待审' }).catch(() => undefined),
          listTickets({ page: 1, page_size: 1, state: 'PENDING' }).catch(() => undefined),
          readPoolStats().catch(() => undefined),
          listPricingRules({ page: 1, page_size: 1, state: 'published' }).catch(() => undefined),
        ]);
        if (!active) {
          return;
        }
        setOverview({
          todayOrders: today?.page?.total ?? 0,
          runningOrders: running?.page?.total ?? 0,
          pendingAudits: audits?.page?.total ?? 0,
          pendingTickets: tickets?.page?.total ?? 0,
          sharedRate: pool?.shared_rate ?? 0,
          publishedRules: rules?.page?.total ?? 0,
        });
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    };
    void load();
    return () => {
      active = false;
    };
  }, []);

  return (
    <PageContainer
      title="运营看板"
      content="花小猪打车 · 运营总览：订单、审核、工单、拼车与计价规则实时汇总"
      extra={
        <Space>
          <Button onClick={() => history.push('/operations/orders')}>订单管理</Button>
          <Button onClick={() => history.push('/operations/tickets')}>投诉工单</Button>
          <Button type="primary" onClick={() => history.push('/pricing-dispatch/pricing')}>
            计价规则
          </Button>
        </Space>
      }
    >
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} xl={8}>
          <Card loading={loading}>
            <Statistic title="今日订单" value={overview.todayOrders} suffix="单" />
            {/* 带当天时间边界跳转，订单管理页按 URL 参数过滤，避免看到全部历史订单。 */}
            <Button
              type="link"
              style={{ paddingLeft: 0 }}
              onClick={() =>
                history.push(
                  `/operations/orders?from=${dayjs().startOf('day').toISOString()}&to=${dayjs().endOf('day').toISOString()}`,
                )
              }
            >
              查看订单
            </Button>
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={8}>
          <Card loading={loading}>
            <Statistic title="行程中订单" value={overview.runningOrders} suffix="单" />
            <Button
              type="link"
              style={{ paddingLeft: 0 }}
              onClick={() =>
                history.push('/operations/orders?state=ACCEPTED,ARRIVED,IN_SERVICE')
              }
            >
              实时监控
            </Button>
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={8}>
          <Card loading={loading}>
            <Statistic
              title="待审核司机/车辆"
              value={overview.pendingAudits}
              suffix="件"
              valueStyle={{ color: overview.pendingAudits > 0 ? '#fa8c16' : undefined }}
            />
            <Button type="link" style={{ paddingLeft: 0 }} onClick={() => history.push('/operations/driver-audits')}>
              去审核
            </Button>
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={8}>
          <Card loading={loading}>
            <Statistic
              title="待处理工单"
              value={overview.pendingTickets}
              suffix="件"
              valueStyle={{ color: overview.pendingTickets > 0 ? '#cf1322' : undefined }}
            />
            <Button type="link" style={{ paddingLeft: 0 }} onClick={() => history.push('/operations/tickets')}>
              去处理
            </Button>
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={8}>
          <Card loading={loading}>
            <Statistic title="拼车实际共乘率" value={overview.sharedRate} precision={1} suffix="%" />
            <Button type="link" style={{ paddingLeft: 0 }} onClick={() => history.push('/marketing/pool')}>
              拼车与特价
            </Button>
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={8}>
          <Card loading={loading}>
            <Statistic
              title="已发布计价规则"
              value={overview.publishedRules}
              suffix="条"
              valueStyle={{ color: overview.publishedRules === 0 ? '#cf1322' : undefined }}
            />
            <Button type="link" style={{ paddingLeft: 0 }} onClick={() => history.push('/pricing-dispatch/pricing')}>
              计价与派单
            </Button>
          </Card>
        </Col>
      </Row>
    </PageContainer>
  );
};

export default Dashboard;
