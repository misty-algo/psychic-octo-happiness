import { Column } from '@ant-design/plots';
import { Button, Card, Col, DatePicker, Row, Tabs } from 'antd';
import type { RangePickerProps } from 'antd/es/date-picker';
import React from 'react';

import { formatNumber, type Point, type RankItem } from '../analysis';
import useStyles from '../style';
import { chartPrimary } from './chartPalette';

export type TimeType = 'today' | 'week' | 'month' | 'year';

const { RangePicker } = DatePicker;

/** 柱状趋势图 + 右侧排名列表，复刻数据分析页销售额卡片的双栏结构。 */
const ChartWithRank: React.FC<{
  data: Point[];
  rankTitle: string;
  rank: RankItem[];
  tooltipName: string;
}> = ({ data, rankTitle, rank, tooltipName }) => {
  const { styles } = useStyles();
  return (
    <Row>
      <Col xl={16} lg={12} md={12} sm={24} xs={24}>
        <div className={styles.salesBar}>
          <Column
            height={300}
            data={data}
            xField="x"
            yField="y"
            colorField={() => tooltipName}
            legend={false}
            paddingBottom={12}
            axis={{
              x: {
                title: false,
              },
              y: {
                title: false,
                gridLineDash: null,
                gridStroke: '#ccc',
              },
            }}
            scale={{
              x: { paddingInner: 0.4 },
              // range 提供两个同色值：G2 的 ordinal 色板不会循环单元素 range。
              color: { range: [chartPrimary, chartPrimary] },
            }}
            tooltip={{
              name: tooltipName,
              channel: 'y',
            }}
          />
        </div>
      </Col>
      <Col xl={8} lg={12} md={12} sm={24} xs={24}>
        <div className={styles.salesRank}>
          <h4 className={styles.rankingTitle}>{rankTitle}</h4>
          <ul className={styles.rankingList}>
            {rank.map((item, i) => (
              <li key={item.title}>
                <span
                  className={
                    i < 3
                      ? styles.rankingItemNumberActive
                      : styles.rankingItemNumber
                  }
                >
                  {i + 1}
                </span>
                <span className={styles.rankingItemTitle} title={item.title}>
                  {item.title}
                </span>
                <span>{formatNumber(item.total)}</span>
              </li>
            ))}
          </ul>
        </div>
      </Col>
    </Row>
  );
};

/** 操作/登录趋势卡：Tabs 切换指标，卡头提供日期快捷选择与范围选择器。 */
const TrendCard: React.FC<{
  loading: boolean;
  operationTrend: Point[];
  loginTrend: Point[];
  resourceRank: RankItem[];
  actorRank: RankItem[];
  rangePickerValue: RangePickerProps['value'];
  isActive: (key: TimeType) => string;
  onRangePickerChange: RangePickerProps['onChange'];
  onSelectDate: (key: TimeType) => void;
}> = ({
  loading,
  operationTrend,
  loginTrend,
  resourceRank,
  actorRank,
  rangePickerValue,
  isActive,
  onRangePickerChange,
  onSelectDate,
}) => {
  const { styles } = useStyles();
  return (
    <Card
      loading={loading}
      variant="borderless"
      styles={{
        body: {
          padding: loading ? 24 : 0,
        },
      }}
    >
      <Tabs
        className={styles.salesCard}
        tabBarExtraContent={
          <div className={styles.salesExtraWrap}>
            <div className={styles.salesExtra}>
              <Button
                type="text"
                className={isActive('today')}
                onClick={() => onSelectDate('today')}
              >
                今日
              </Button>
              <Button
                type="text"
                className={isActive('week')}
                onClick={() => onSelectDate('week')}
              >
                本周
              </Button>
              <Button
                type="text"
                className={isActive('month')}
                onClick={() => onSelectDate('month')}
              >
                本月
              </Button>
              <Button
                type="text"
                className={isActive('year')}
                onClick={() => onSelectDate('year')}
              >
                本年
              </Button>
            </div>
            <RangePicker
              value={rangePickerValue}
              onChange={onRangePickerChange}
              variant="filled"
              style={{
                width: 256,
              }}
            />
          </div>
        }
        size="large"
        tabBarStyle={{
          marginBottom: 24,
        }}
        items={[
          {
            key: 'operations',
            label: '操作趋势',
            children: (
              <ChartWithRank
                data={operationTrend}
                rankTitle="资源操作排名"
                rank={resourceRank}
                tooltipName="操作数"
              />
            ),
          },
          {
            key: 'logins',
            label: '登录趋势',
            children: (
              <ChartWithRank
                data={loginTrend}
                rankTitle="活跃账号排名"
                rank={actorRank}
                tooltipName="登录次数"
              />
            ),
          },
        ]}
      />
    </Card>
  );
};

export default TrendCard;
