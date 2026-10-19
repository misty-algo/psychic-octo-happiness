import { PageContainer } from '@ant-design/pro-components';
import { useModel } from '@umijs/max';
import { Col, Row } from 'antd';
import type { RangePickerProps } from 'antd/es/date-picker';
import dayjs, { type Dayjs } from 'dayjs';
import React, { useEffect, useState } from 'react';

import {
  capabilities as fetchCapabilities,
  type Envelope,
  listLoginLogs,
  listOperationLogs,
} from '@/services/admin';

import {
  type AnalysisSummary,
  logPageSize,
  maxLogPages,
  summarize,
} from './analysis';
import IntroduceRow from './components/IntroduceRow';
import ResourceProportion from './components/ResourceProportion';
import ResourceTrend from './components/ResourceTrend';
import TopOperations from './components/TopOperations';
import TrendCard, { type TimeType } from './components/TrendCard';
import useStyles from './style';

type RangePickerValue = RangePickerProps['value'];

function fixedZero(val: number) {
  return val * 1 < 10 ? `0${val}` : val;
}

/** 计算今日/本周/本月/本年对应的日期范围，用于卡头快捷选择高亮。 */
function getTimeDistance(type: TimeType): RangePickerValue {
  const now = new Date();
  const oneDay = 1000 * 60 * 60 * 24;

  if (type === 'today') {
    now.setHours(0);
    now.setMinutes(0);
    now.setSeconds(0);
    return [dayjs(now), dayjs(now.getTime() + (oneDay - 1000))];
  }

  if (type === 'week') {
    let day = now.getDay();
    now.setHours(0);
    now.setMinutes(0);
    now.setSeconds(0);

    if (day === 0) {
      day = 6;
    } else {
      day -= 1;
    }

    const beginTime = now.getTime() - day * oneDay;

    return [dayjs(beginTime), dayjs(beginTime + (7 * oneDay - 1000))];
  }
  const year = now.getFullYear();

  if (type === 'month') {
    const month = now.getMonth();
    const nextDate = dayjs(now).add(1, 'months');
    const nextYear = nextDate.year();
    const nextMonth = nextDate.month();

    return [
      dayjs(`${year}-${fixedZero(month + 1)}-01 00:00:00`),
      dayjs(
        dayjs(`${nextYear}-${fixedZero(nextMonth + 1)}-01 00:00:00`).valueOf() -
          1000,
      ),
    ];
  }

  return [dayjs(`${year}-01-01 00:00:00`), dayjs(`${year}-12-31 23:59:59`)];
}

/** 拉取最近若干页日志：先取第一页看总数，再按需补齐剩余页，上限 maxLogPages。 */
const fetchRecentLogs = async <T,>(
  fetchPage: (page: number) => Promise<Envelope<T[]>>,
): Promise<T[]> => {
  const first = await fetchPage(1);
  const total = first.page?.total ?? first.data.length;
  const pages = Math.min(Math.ceil(total / logPageSize), maxLogPages);
  const rest = await Promise.all(
    Array.from({ length: Math.max(pages - 1, 0) }, (_, i) =>
      fetchPage(i + 2),
    ),
  );
  return [first, ...rest].flatMap((response) => response.data);
};

const emptySummary = (): AnalysisSummary => summarize([], [], dayjs());

/** 工作台：数据分析页布局，指标来自最近审计日志与 Capability 状态的实时聚合。 */
const Dashboard: React.FC = () => {
  const { initialState } = useModel('@@initialState');
  const { styles } = useStyles();
  const [loading, setLoading] = useState(true);
  const [summary, setSummary] = useState<AnalysisSummary>(emptySummary);
  const [capability, setCapability] = useState({ available: 0, total: 0 });
  const [logPermissionMissing, setLogPermissionMissing] = useState(false);
  const [rangePickerValue, setRangePickerValue] = useState<RangePickerValue>(
    () => getTimeDistance('year'),
  );

  const permissions = initialState?.currentUser?.permissions;

  useEffect(() => {
    let active = true;
    const canReadLogs = permissions?.includes('log:read') ?? false;
    const load = async () => {
      setLoading(true);
      try {
        if (!canReadLogs) {
          setLogPermissionMissing(true);
          // 无日志权限时仍展示 Capability 指标，日志类指标保持为空。
          const capabilityResult = await fetchCapabilities();
          if (!active) {
            return;
          }
          const rows = capabilityResult.capabilities;
          setCapability({
            available: rows.filter((item) => item.available).length,
            total: rows.length,
          });
          return;
        }
        const [capabilityResult, operations, logins] = await Promise.all([
          fetchCapabilities(),
          fetchRecentLogs((page) =>
            listOperationLogs({ page, page_size: logPageSize }),
          ),
          fetchRecentLogs((page) =>
            listLoginLogs({ page, page_size: logPageSize }),
          ),
        ]);
        if (!active) {
          return;
        }
        const rows = capabilityResult.capabilities;
        setCapability({
          available: rows.filter((item) => item.available).length,
          total: rows.length,
        });
        setSummary(summarize(operations, logins, dayjs()));
      } catch {
        // 请求失败提示由全局响应拦截统一弹出，页面降级为空指标。
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
  }, [permissions]);

  const selectDate = (type: TimeType) => {
    setRangePickerValue(getTimeDistance(type));
  };

  const isActive = (type: TimeType) => {
    if (!rangePickerValue) {
      return '';
    }
    const value = getTimeDistance(type);
    if (!value) {
      return '';
    }
    if (!rangePickerValue[0] || !rangePickerValue[1]) {
      return '';
    }
    if (
      rangePickerValue[0].isSame(value[0] as Dayjs, 'day') &&
      rangePickerValue[1].isSame(value[1] as Dayjs, 'day')
    ) {
      return styles.currentDate;
    }
    return '';
  };

  return (
    <PageContainer
      title="工作台"
      content={
        logPermissionMissing
          ? '当前角色缺少 log:read 权限，日志指标为空；仅展示 Capability 状态'
          : `指标基于最近 ${logPageSize * maxLogPages} 条操作与登录日志聚合`
      }
    >
      <IntroduceRow
        loading={loading}
        summary={summary}
        capability={capability}
      />
      <TrendCard
        loading={loading}
        operationTrend={summary.operationTrend}
        loginTrend={summary.loginTrend}
        resourceRank={summary.resourceRank}
        actorRank={summary.actorRank}
        rangePickerValue={rangePickerValue}
        isActive={isActive}
        onRangePickerChange={setRangePickerValue}
        onSelectDate={selectDate}
      />
      <Row
        gutter={24}
        style={{
          marginTop: 24,
        }}
      >
        <Col xl={12} lg={24} md={24} sm={24} xs={24}>
          <TopOperations loading={loading} summary={summary} />
        </Col>
        <Col xl={12} lg={24} md={24} sm={24} xs={24}>
          <ResourceProportion loading={loading} summary={summary} />
        </Col>
      </Row>
      <ResourceTrend
        loading={loading}
        resourceTrends={summary.resourceTrends}
      />
    </PageContainer>
  );
};

export default Dashboard;
