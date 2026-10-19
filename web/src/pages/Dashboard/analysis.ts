import dayjs, { type Dayjs } from 'dayjs';

import type { LoginLog, OperationLog } from '@/services/admin';

/** 图表通用数据点：x 为类目或日期标签，y 为数值。 */
export type Point = { x: string; y: number };

export type RankItem = { title: string; total: number };

export type TopActionItem = {
  index: number;
  keyword: string;
  count: number;
  range: number;
  status: number;
};

export type ResourceTrend = {
  name: string;
  successRate: number;
  series: { date: string; type: string; value: number }[];
};

export type AnalysisSummary = {
  totalOperations: number;
  dayOperations: number;
  operationWeekOverWeek: number | null;
  operationDayOverDay: number | null;
  totalLogins: number;
  dayLogins: number;
  loginWeekOverWeek: number | null;
  weekOperations: number;
  weekPerPerson: number;
  perPersonChange: number | null;
  successRate: number | null;
  successCount: number;
  miniLoginArea: Point[];
  miniOperationColumn: Point[];
  operationTrend: Point[];
  loginTrend: Point[];
  resourceRank: RankItem[];
  actorRank: RankItem[];
  topActions: TopActionItem[];
  proportionAll: Point[];
  proportionSuccess: Point[];
  proportionFailure: Point[];
  resourceTrends: ResourceTrend[];
};

/** 操作与登录日志各取最近 300 条（后端每页上限 100）做聚合，避免工作台打爆日志接口。 */
export const logPageSize = 100;
export const maxLogPages = 3;

const dayWindow = 30;
const miniWindow = 14;
const rankSize = 7;
const topActionSize = 50;
const trendResourceSize = 5;

const numberFormatter = new Intl.NumberFormat('zh-CN');

export const formatNumber = (value: number | string): string =>
  numberFormatter.format(Number(value));

const dayKey = (value: Dayjs): string => value.format('MM-DD');

/** 生成从 now-(days-1) 到 now 的每日桶标签，以及任意时间落到桶下标的函数。 */
const buildBuckets = (
  now: Dayjs,
  days: number,
): { keys: string[]; indexOf: (time: Dayjs) => number } => {
  const start = now.startOf('day').subtract(days - 1, 'day');
  return {
    keys: Array.from({ length: days }, (_, i) => dayKey(start.add(i, 'day'))),
    indexOf: (time: Dayjs) => time.startOf('day').diff(start, 'day'),
  };
};

/** 把日志按天分桶计数，窗口外的记录丢弃。 */
const countByDay = (
  logs: { created_at: string }[],
  now: Dayjs,
  days: number,
): Point[] => {
  const buckets = buildBuckets(now, days);
  const counts = new Array<number>(days).fill(0);
  for (const log of logs) {
    const index = buckets.indexOf(dayjs(log.created_at));
    if (index >= 0 && index < days) {
      counts[index] += 1;
    }
  }
  return buckets.keys.map((key, i) => ({ x: key, y: counts[i] }));
};

/** 同比变化百分比；基期为 0 时无法计算，返回 null 由视图展示占位。 */
const changeRate = (current: number, previous: number): number | null => {
  if (previous <= 0) {
    return null;
  }
  return Math.round(((current - previous) / previous) * 100);
};

const countRange = (points: Point[], from: number, to: number): number =>
  points.slice(from, to).reduce((sum, point) => sum + point.y, 0);

const toRankedPoints = (counts: Map<string, number>): Point[] =>
  [...counts.entries()]
    .map(([x, y]) => ({ x, y }))
    .sort((a, b) => b.y - a.y);

/**
 * 聚合最近的操作与登录日志为工作台指标。
 *
 * 统计窗口统一为「含今天在内的最近 30 个自然日」，周同比比较近 7 天与前 7 天，
 * 日同比比较昨天与前天，因此所有比较区间都落在 30 天窗口内。
 */
export const summarize = (
  operations: OperationLog[],
  logins: LoginLog[],
  now: Dayjs,
): AnalysisSummary => {
  // 所有指标统一基于「含今天的最近 30 个自然日」，窗口外记录在此一次性丢弃。
  const buckets = buildBuckets(now, dayWindow);
  const inWindow = <T extends { created_at: string }>(logs: T[]): T[] =>
    logs.filter((log) => {
      const index = buckets.indexOf(dayjs(log.created_at));
      return index >= 0 && index < dayWindow;
    });
  const recentOperations = inWindow(operations);
  const recentLogins = inWindow(logins);

  const operationPoints = countByDay(recentOperations, now, dayWindow);
  const loginPoints = countByDay(recentLogins, now, dayWindow);

  const totalOperations = recentOperations.length;
  const dayOperations = operationPoints.at(-1)?.y ?? 0;
  const yesterday = operationPoints.at(-2)?.y ?? 0;
  const dayBefore = operationPoints.at(-3)?.y ?? 0;
  const totalLogins = recentLogins.length;
  const dayLogins = loginPoints.at(-1)?.y ?? 0;

  const recentWeek = countRange(operationPoints, dayWindow - 7, dayWindow);
  const previousWeek = countRange(operationPoints, dayWindow - 14, dayWindow - 7);

  // 人均操作 = 近 7 天操作数 / 近 7 天去重操作人（actor_id）。
  const weekActors = new Set<number>();
  const previousWeekActors = new Set<number>();
  for (const log of recentOperations) {
    const index = buckets.indexOf(dayjs(log.created_at));
    if (index >= dayWindow - 7) {
      weekActors.add(log.actor_id);
    } else {
      previousWeekActors.add(log.actor_id);
    }
  }
  const weekPerPerson =
    weekActors.size > 0 ? recentWeek / weekActors.size : 0;
  const previousPerPerson =
    previousWeekActors.size > 0 ? previousWeek / previousWeekActors.size : 0;

  const successCount = recentOperations.filter((log) => log.success).length;
  const successRate =
    totalOperations > 0
      ? Math.round((successCount / totalOperations) * 100)
      : null;

  const resourceCounts = new Map<string, number>();
  const resourceSuccess = new Map<string, number>();
  const resourceFailure = new Map<string, number>();
  for (const log of recentOperations) {
    const total = resourceCounts.get(log.resource) ?? 0;
    resourceCounts.set(log.resource, total + 1);
    const bucket = log.success ? resourceSuccess : resourceFailure;
    bucket.set(log.resource, (bucket.get(log.resource) ?? 0) + 1);
  }

  const actionCounts = new Map<string, number>();
  for (const log of recentOperations) {
    const key = `${log.resource}.${log.action}`;
    actionCounts.set(key, (actionCounts.get(key) ?? 0) + 1);
  }
  // 周涨幅需要按 action 维度再做两次按天计数，直接复用窗口桶。
  const recentWeekActions = new Map<string, number>();
  const previousWeekActions = new Map<string, number>();
  for (const log of recentOperations) {
    const index = buckets.indexOf(dayjs(log.created_at));
    if (index < 0 || index >= dayWindow) {
      continue;
    }
    const key = `${log.resource}.${log.action}`;
    const target =
      index >= dayWindow - 7 ? recentWeekActions : previousWeekActions;
    target.set(key, (target.get(key) ?? 0) + 1);
  }

  const rankedResources = toRankedPoints(resourceCounts);

  const resourceTrends: ResourceTrend[] = rankedResources
    .slice(0, trendResourceSize)
    .map(({ x: resource }) => {
      const successTrend = new Array<number>(dayWindow).fill(0);
      const failureTrend = new Array<number>(dayWindow).fill(0);
      for (const log of recentOperations) {
        if (log.resource !== resource) {
          continue;
        }
        const index = buckets.indexOf(dayjs(log.created_at));
        if (index < 0 || index >= dayWindow) {
          continue;
        }
        if (log.success) {
          successTrend[index] += 1;
        } else {
          failureTrend[index] += 1;
        }
      }
      const success = successTrend.reduce((sum, v) => sum + v, 0);
      const failure = failureTrend.reduce((sum, v) => sum + v, 0);
      return {
        name: resource,
        successRate: success + failure > 0 ? success / (success + failure) : 0,
        series: buckets.keys.flatMap((date, i) => [
          { date, type: '成功操作', value: successTrend[i] },
          { date, type: '失败操作', value: failureTrend[i] },
        ]),
      };
    });

  return {
    totalOperations,
    dayOperations,
    operationWeekOverWeek: changeRate(recentWeek, previousWeek),
    operationDayOverDay: changeRate(yesterday, dayBefore),
    totalLogins,
    dayLogins,
    loginWeekOverWeek: changeRate(
      countRange(loginPoints, dayWindow - 7, dayWindow),
      countRange(loginPoints, dayWindow - 14, dayWindow - 7),
    ),
    weekOperations: recentWeek,
    weekPerPerson,
    perPersonChange: changeRate(weekPerPerson, previousPerPerson),
    successRate,
    successCount,
    miniLoginArea: countByDay(recentLogins, now, miniWindow),
    miniOperationColumn: countByDay(recentOperations, now, miniWindow),
    operationTrend: operationPoints,
    loginTrend: loginPoints,
    resourceRank: rankedResources
      .slice(0, rankSize)
      .map(({ x, y }) => ({ title: x, total: y })),
    actorRank: toRankedPoints(
      recentLogins.reduce((counts, log) => {
        counts.set(log.username, (counts.get(log.username) ?? 0) + 1);
        return counts;
      }, new Map<string, number>()),
    )
      .slice(0, rankSize)
      .map(({ x, y }) => ({ title: x, total: y })),
    topActions: [...actionCounts.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, topActionSize)
      .map(([keyword, count], i) => {
        const recent = recentWeekActions.get(keyword) ?? 0;
        const previous = previousWeekActions.get(keyword) ?? 0;
        const range = changeRate(recent, previous) ?? 0;
        return {
          index: i + 1,
          keyword,
          count,
          range,
          status: range >= 0 ? 0 : 1,
        };
      }),
    proportionAll: rankedResources,
    proportionSuccess: toRankedPoints(resourceSuccess),
    proportionFailure: toRankedPoints(resourceFailure),
    resourceTrends,
  };
};
