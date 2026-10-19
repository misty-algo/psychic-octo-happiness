import { Line, Tiny } from '@ant-design/plots';
import { Card, Col, Row, Tabs } from 'antd';
import React, { useState } from 'react';

import type { ResourceTrend as ResourceTrendData } from '../analysis';
import useStyles from '../style';
import { NumberInfo } from './ChartKit';
import { chartContrast, chartRing } from './chartPalette';

/** Tab 头：资源名 + 成功率数字 + 环形迷你图，选中态高亮主色。 */
const CustomTab: React.FC<{
  data: ResourceTrendData;
  active: boolean;
}> = ({ data, active }) => (
  <Row
    gutter={8}
    style={{
      width: 138,
      margin: '8px 0',
    }}
  >
    <Col span={12}>
      <NumberInfo
        title={data.name}
        subTitle="成功率"
        gap={2}
        total={`${Math.round(data.successRate * 100)}%`}
        theme={active ? undefined : 'light'}
      />
    </Col>
    <Col
      span={12}
      style={{
        paddingTop: 36,
      }}
    >
      <Tiny.Ring
        height={60}
        width={60}
        percent={data.successRate}
        color={chartRing}
      />
    </Col>
  </Row>
);

/** 资源趋势卡：按热门资源切换的成功/失败双序列折线图。 */
const ResourceTrend: React.FC<{
  loading: boolean;
  resourceTrends: ResourceTrendData[];
}> = ({ loading, resourceTrends }) => {
  const { styles } = useStyles();
  const [selectedKey, setSelectedKey] = useState<string>('');
  // 日志异步加载完成后Tabs才拿到资源列表，未手动选择前回退到第一个资源。
  const activeKey = selectedKey || resourceTrends[0]?.name || '';

  return (
    <Card
      loading={loading}
      className={styles.offlineCard}
      variant="borderless"
      style={{
        marginTop: 32,
      }}
    >
      <Tabs
        activeKey={activeKey}
        onChange={setSelectedKey}
        items={resourceTrends.map((resource) => ({
          key: resource.name,
          label: (
            <CustomTab data={resource} active={activeKey === resource.name} />
          ),
          children: (
            <div
              style={{
                padding: '0 24px',
              }}
            >
              <Line
                height={400}
                data={resource.series}
                xField="date"
                yField="value"
                colorField="type"
                scale={{ color: { range: chartContrast } }}
                slider={{
                  x: {
                    style: {
                      selectionFill: 'rgba(22, 119, 255, 0.16)',
                      handleFill: '#1677ff',
                    },
                  },
                }}
                axis={{
                  x: { title: false },
                  y: {
                    title: false,
                    gridLineDash: null,
                    gridStroke: '#ccc',
                    gridStrokeOpacity: 1,
                  },
                }}
                legend={{
                  color: {
                    layout: { justifyContent: 'center' },
                  },
                }}
              />
            </div>
          ),
        }))}
      />
    </Card>
  );
};

export default ResourceTrend;
