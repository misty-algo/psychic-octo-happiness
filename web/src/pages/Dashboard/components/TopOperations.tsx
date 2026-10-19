import { InfoCircleOutlined } from '@ant-design/icons';
import { Area } from '@ant-design/plots';
import { Card, Col, Row, Table, Tooltip } from 'antd';
import React from 'react';

import {
  formatNumber,
  type AnalysisSummary,
  type Point,
  type TopActionItem,
} from '../analysis';
import { NumberInfo, Trend } from './ChartKit';

const renderSubTitle = (label: string) => () => (
  <span>
    {label}
    <Tooltip title="基于最近审计日志聚合">
      <InfoCircleOutlined
        style={{
          marginLeft: 8,
        }}
      />
    </Tooltip>
  </span>
);

/** 涨跌状态箭头：负增长显示为下降。 */
const statusOf = (value: number | null): 'up' | 'down' | undefined =>
  value === null ? undefined : value >= 0 ? 'up' : 'down';

/** 热门操作卡：双数字指标 + 迷你趋势 + 操作排行表格。 */
const TopOperations: React.FC<{
  loading: boolean;
  summary: AnalysisSummary;
}> = ({ loading, summary }) => {
  const columns = [
    {
      title: '排名',
      dataIndex: 'index',
      key: 'index',
    },
    {
      title: '操作',
      dataIndex: 'keyword',
      key: 'keyword',
      render: (text: React.ReactNode) => <a href="/">{text}</a>,
    },
    {
      title: '次数',
      dataIndex: 'count',
      key: 'count',
      sorter: (
        a: { count: number },
        b: { count: number },
      ) => a.count - b.count,
    },
    {
      title: '周涨幅',
      dataIndex: 'range',
      key: 'range',
      sorter: (
        a: { range: number },
        b: { range: number },
      ) => a.range - b.range,
      render: (
        text: React.ReactNode,
        record: TopActionItem,
      ) => (
        <Trend flag={record.status === 1 ? 'down' : 'up'}>
          <span
            style={{
              marginRight: 4,
            }}
          >
            {text}%
          </span>
        </Trend>
      ),
    },
  ];

  const miniArea: Point[] = summary.miniOperationColumn;

  return (
    <Card
      loading={loading}
      variant="borderless"
      title="热门操作"
      style={{
        height: '100%',
      }}
    >
      <Row gutter={68}>
        <Col
          sm={12}
          xs={24}
          style={{
            marginBottom: 24,
          }}
        >
          <NumberInfo
            renderSubTitle={renderSubTitle('周操作数')}
            gap={8}
            total={formatNumber(summary.weekOperations)}
            status={statusOf(summary.operationWeekOverWeek)}
            subTotal={
              summary.operationWeekOverWeek === null
                ? undefined
                : Math.abs(summary.operationWeekOverWeek)
            }
          />
          <Area
            xField="x"
            yField="y"
            shapeField="smooth"
            height={45}
            axis={false}
            padding={-12}
            style={{
              fill: 'linear-gradient(-90deg, white 0%, #6294FA 100%)',
              fillOpacity: 0.4,
            }}
            data={miniArea}
          />
        </Col>
        <Col
          sm={12}
          xs={24}
          style={{
            marginBottom: 24,
          }}
        >
          <NumberInfo
            renderSubTitle={renderSubTitle('人均操作次数')}
            total={summary.weekPerPerson.toFixed(1)}
            status={statusOf(summary.perPersonChange)}
            subTotal={
              summary.perPersonChange === null
                ? undefined
                : Math.abs(Math.round(summary.perPersonChange))
            }
            gap={8}
          />
          <Area
            xField="x"
            yField="y"
            shapeField="smooth"
            height={45}
            padding={-12}
            style={{
              fill: 'linear-gradient(-90deg, white 0%, #6294FA 100%)',
              fillOpacity: 0.4,
            }}
            data={miniArea}
            axis={false}
          />
        </Col>
      </Row>
      <Table<TopActionItem>
        rowKey={(record) => record.index}
        size="small"
        columns={columns}
        dataSource={summary.topActions}
        pagination={{
          style: {
            marginBottom: 0,
          },
          pageSize: 5,
        }}
      />
    </Card>
  );
};

export default TopOperations;
