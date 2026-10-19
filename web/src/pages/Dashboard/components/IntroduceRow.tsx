import { InfoCircleOutlined } from '@ant-design/icons';
import { Area, Column } from '@ant-design/plots';
import { Col, Progress, Row, Tooltip } from 'antd';
import React from 'react';

import { formatNumber, type AnalysisSummary } from '../analysis';
import useStyles from '../style';
import { ChartCard, Field, Trend } from './ChartKit';
import { chartPrimary } from './chartPalette';

const topColResponsiveProps = {
  xs: 24,
  sm: 12,
  md: 12,
  lg: 12,
  xl: 6,
  style: {
    marginBottom: 24,
  },
};

/** 同比数值无法计算（基期为 0）时展示灰色占位，箭头方向退化为 up。 */
const TrendValue: React.FC<{ label: string; value: number | null }> = ({
  label,
  value,
}) => {
  const { styles } = useStyles();
  return (
    <Trend
      flag={value !== null && value < 0 ? 'down' : 'up'}
      colorful={value !== null}
      style={{ marginRight: 16 }}
    >
      {label}
      <span className={styles.trendText}>
        {value === null ? '—' : `${Math.abs(value)}%`}
      </span>
    </Trend>
  );
};

/** 顶部四联指标卡：操作数、登录次数、操作成功率、能力可用率。 */
const IntroduceRow: React.FC<{
  loading: boolean;
  summary: AnalysisSummary;
  capability: { available: number; total: number };
}> = ({ loading, summary, capability }) => {
  const capabilityRate =
    capability.total > 0
      ? Math.round((capability.available / capability.total) * 100)
      : null;

  return (
    <Row gutter={24}>
      <Col {...topColResponsiveProps}>
        <ChartCard
          variant="borderless"
          title="操作数（近30天）"
          action={
            <Tooltip title="基于最近审计日志聚合">
              <InfoCircleOutlined />
            </Tooltip>
          }
          loading={loading}
          total={formatNumber(summary.totalOperations)}
          footer={
            <Field label="日操作数" value={formatNumber(summary.dayOperations)} />
          }
          contentHeight={46}
        >
          <TrendValue label="周同比" value={summary.operationWeekOverWeek} />
          <TrendValue label="日同比" value={summary.operationDayOverDay} />
        </ChartCard>
      </Col>

      <Col {...topColResponsiveProps}>
        <ChartCard
          variant="borderless"
          loading={loading}
          title="登录次数（近30天）"
          action={
            <Tooltip title="基于最近登录日志聚合">
              <InfoCircleOutlined />
            </Tooltip>
          }
          total={formatNumber(summary.totalLogins)}
          footer={
            <Field label="日登录数" value={formatNumber(summary.dayLogins)} />
          }
          contentHeight={46}
        >
          <Area
            xField="x"
            yField="y"
            shapeField="smooth"
            height={46}
            axis={false}
            style={{
              fill: `linear-gradient(-90deg, white 0%, ${chartPrimary} 100%)`,
              fillOpacity: 0.6,
              width: '100%',
            }}
            padding={-20}
            data={summary.miniLoginArea}
          />
        </ChartCard>
      </Col>

      <Col {...topColResponsiveProps}>
        <ChartCard
          variant="borderless"
          loading={loading}
          title="操作成功率（近30天）"
          action={
            <Tooltip title="成功操作数 / 操作总数">
              <InfoCircleOutlined />
            </Tooltip>
          }
          total={summary.successRate === null ? '—' : `${summary.successRate}%`}
          footer={
            <Field
              label="成功操作"
              value={formatNumber(summary.successCount)}
            />
          }
          contentHeight={46}
        >
          <Column
            xField="x"
            yField="y"
            colorField={() => 'count'}
            legend={false}
            padding={-20}
            axis={false}
            height={46}
            data={summary.miniOperationColumn}
            scale={{
              x: { paddingInner: 0.4 },
              color: { range: [chartPrimary, chartPrimary] },
            }}
          />
        </ChartCard>
      </Col>

      <Col {...topColResponsiveProps}>
        <ChartCard
          loading={loading}
          variant="borderless"
          title="能力可用率"
          action={
            <Tooltip title="可用 Capability / 全部 Capability">
              <InfoCircleOutlined />
            </Tooltip>
          }
          total={capabilityRate === null ? '—' : `${capabilityRate}%`}
          footer={
            <Field
              label="可用能力"
              value={`${capability.available} / ${capability.total}`}
            />
          }
          contentHeight={46}
        >
          <Progress
            percent={capabilityRate ?? 0}
            strokeColor={{ from: '#1677ff', to: '#4096ff' }}
            status="active"
          />
        </ChartCard>
      </Col>
    </Row>
  );
};

export default IntroduceRow;
