import { Pie } from '@ant-design/plots';
import { Card, Segmented, Typography } from 'antd';
import React, { useState } from 'react';

import { formatNumber, type AnalysisSummary, type Point } from '../analysis';
import useStyles from '../style';
import { chartPalette } from './chartPalette';

const { Text } = Typography;

type ProportionType = 'all' | 'success' | 'failure';

const proportionData = (
  summary: AnalysisSummary,
  type: ProportionType,
): Point[] => {
  if (type === 'all') {
    return summary.proportionAll;
  }
  return type === 'success'
    ? summary.proportionSuccess
    : summary.proportionFailure;
};

/** 操作资源占比卡：全部/成功/失败三视角切换的环形饼图。 */
const ResourceProportion: React.FC<{
  loading: boolean;
  summary: AnalysisSummary;
}> = ({ loading, summary }) => {
  const { styles } = useStyles();
  const [type, setType] = useState<ProportionType>('all');

  return (
    <Card
      loading={loading}
      className={styles.salesCard}
      variant="borderless"
      title="操作资源占比"
      style={{
        height: '100%',
      }}
      extra={
        <div className={styles.salesCardExtra}>
          <Segmented
            className={styles.salesTypeRadio}
            value={type}
            onChange={(value) => setType(value as ProportionType)}
            options={[
              { label: '全部', value: 'all' },
              { label: '成功', value: 'success' },
              { label: '失败', value: 'failure' },
            ]}
            size="middle"
          />
        </div>
      }
    >
      <Text>操作数</Text>
      <Pie
        height={340}
        radius={0.8}
        innerRadius={0.5}
        angleField="y"
        colorField="x"
        scale={{ color: { range: chartPalette } }}
        data={proportionData(summary, type)}
        legend={false}
        label={{
          position: 'spider',
          text: (item: Point) => `${item.x}: ${formatNumber(item.y)}`,
        }}
      />
    </Card>
  );
};

export default ResourceProportion;
