import dayjs from 'dayjs';
import { describe, expect, it } from 'vitest';

import type { LoginLog, OperationLog } from '@/services/admin';

import { formatNumber, summarize } from './analysis';

const operation = (
  overrides: Partial<OperationLog> & Pick<OperationLog, 'created_at'>,
): OperationLog => ({
  id: 1,
  actor_id: 1,
  action: 'update',
  resource: 'role',
  resource_id: '1',
  method: 'PUT',
  path: '/api/roles/1',
  ip: '127.0.0.1',
  user_agent: '',
  success: true,
  message: '',
  ...overrides,
});

const login = (
  overrides: Partial<LoginLog> & Pick<LoginLog, 'created_at'>,
): LoginLog => ({
  id: 1,
  admin_id: 1,
  username: 'admin',
  ip: '127.0.0.1',
  user_agent: '',
  success: true,
  reason: '',
  ...overrides,
});

// 固定“今天”为 2026-09-08，避免测试随真实时钟漂移。
const now = dayjs('2026-09-08T10:00:00');
const at = (date: string): string =>
  dayjs(`${date}T08:00:00`).toISOString();

describe('summarize', () => {
  it('空日志时输出全零指标且不出现 NaN', () => {
    const summary = summarize([], [], now);

    expect(summary.totalOperations).toBe(0);
    expect(summary.dayOperations).toBe(0);
    expect(summary.operationWeekOverWeek).toBeNull();
    expect(summary.operationDayOverDay).toBeNull();
    expect(summary.successRate).toBeNull();
    expect(summary.operationTrend).toHaveLength(30);
    expect(summary.operationTrend.every((point) => point.y === 0)).toBe(true);
    expect(summary.miniLoginArea).toHaveLength(14);
    expect(summary.topActions).toEqual([]);
    expect(summary.resourceTrends).toEqual([]);
  });

  it('按天分桶并计算总量、日量与同比', () => {
    const operations = [
      operation({ created_at: at('2026-09-08') }),
      operation({ created_at: at('2026-09-08'), success: false }),
      operation({ created_at: at('2026-09-07') }),
      operation({ created_at: at('2026-09-06') }),
      operation({ created_at: at('2026-09-01') }),
      operation({ created_at: at('2026-08-05') }), // 30 天窗口外，应丢弃
    ];
    const logins = [
      login({ created_at: at('2026-09-08'), username: 'alice' }),
      login({ created_at: at('2026-09-08'), username: 'bob' }),
      login({ created_at: at('2026-09-01'), username: 'alice' }),
    ];

    const summary = summarize(operations, logins, now);

    expect(summary.totalOperations).toBe(5);
    expect(summary.dayOperations).toBe(2);
    expect(summary.operationTrend.at(-1)).toEqual({ x: '09-08', y: 2 });
    expect(summary.operationTrend.at(-2)).toEqual({ x: '09-07', y: 1 });
    // 近 7 天共 4 条、前 7 天共 1 条 → 同比 +300%。
    expect(summary.operationWeekOverWeek).toBe(300);
    // 昨天 1 条、前天 1 条 → 日同比 0%。
    expect(summary.operationDayOverDay).toBe(0);
    expect(summary.successRate).toBe(80);
    expect(summary.successCount).toBe(4);
    expect(summary.totalLogins).toBe(3);
    expect(summary.dayLogins).toBe(2);
  });

  it('日同比按昨天与前天比较', () => {
    const operations = [
      operation({ created_at: at('2026-09-07') }),
      operation({ created_at: at('2026-09-07') }),
      operation({ created_at: at('2026-09-06') }),
      operation({ created_at: at('2026-09-06') }),
      operation({ created_at: at('2026-09-06') }),
      operation({ created_at: at('2026-09-06') }),
    ];

    const summary = summarize(operations, [], now);

    // 昨天 2 条、前天 4 条 → -50%。
    expect(summary.operationDayOverDay).toBe(-50);
  });

  it('按资源聚合排名、占比与趋势序列', () => {
    const operations = [
      operation({ created_at: at('2026-09-08'), resource: 'role' }),
      operation({ created_at: at('2026-09-08'), resource: 'role' }),
      operation({
        created_at: at('2026-09-08'),
        resource: 'role',
        success: false,
      }),
      operation({ created_at: at('2026-09-07'), resource: 'menu' }),
    ];

    const summary = summarize(operations, [], now);

    expect(summary.resourceRank).toEqual([
      { title: 'role', total: 3 },
      { title: 'menu', total: 1 },
    ]);
    expect(summary.proportionAll).toEqual([
      { x: 'role', y: 3 },
      { x: 'menu', y: 1 },
    ]);
    expect(summary.proportionSuccess).toEqual([
      { x: 'role', y: 2 },
      { x: 'menu', y: 1 },
    ]);
    expect(summary.proportionFailure).toEqual([{ x: 'role', y: 1 }]);

    const trend = summary.resourceTrends[0];
    expect(trend.name).toBe('role');
    expect(trend.successRate).toBeCloseTo(2 / 3);
    expect(trend.series).toHaveLength(60);
    expect(trend.series.at(-2)).toEqual({
      date: '09-08',
      type: '成功操作',
      value: 2,
    });
    expect(trend.series.at(-1)).toEqual({
      date: '09-08',
      type: '失败操作',
      value: 1,
    });
  });

  it('topActions 按 action 聚合并给出周涨幅方向', () => {
    const operations = [
      ...Array.from({ length: 3 }, () =>
        operation({
          created_at: at('2026-09-07'),
          resource: 'role',
          action: 'update',
        }),
      ),
      operation({
        created_at: at('2026-09-01'),
        resource: 'role',
        action: 'update',
      }),
      operation({
        created_at: at('2026-09-05'),
        resource: 'menu',
        action: 'create',
      }),
    ];

    const summary = summarize(operations, [], now);

    expect(summary.topActions).toEqual([
      {
        index: 1,
        keyword: 'role.update',
        count: 4,
        range: 200,
        status: 0,
      },
      {
        index: 2,
        keyword: 'menu.create',
        count: 1,
        range: 0,
        status: 0,
      },
    ]);
  });

  it('登录账号排名按用户名聚合', () => {
    const logins = [
      login({ created_at: at('2026-09-08'), username: 'alice' }),
      login({ created_at: at('2026-09-07'), username: 'alice' }),
      login({ created_at: at('2026-09-07'), username: 'bob' }),
    ];

    const summary = summarize([], logins, now);

    expect(summary.actorRank).toEqual([
      { title: 'alice', total: 2 },
      { title: 'bob', total: 1 },
    ]);
  });
});

describe('formatNumber', () => {
  it('渲染千分位', () => {
    expect(formatNumber(126560)).toBe('126,560');
    expect(formatNumber('1234')).toBe('1,234');
  });
});
