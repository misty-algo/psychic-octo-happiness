import { message } from 'antd';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { errorConfig, formatApiError } from './requestErrorConfig';

const { mockGetCSRFToken, mockReplace } = vi.hoisted(() => ({
  mockGetCSRFToken: vi.fn(),
  mockReplace: vi.fn(),
}));

vi.mock('antd', () => ({
  message: {
    error: vi.fn(),
  },
}));

vi.mock('@umijs/max', () => ({
  history: {
    location: {
      pathname: '/admins',
      search: '?page=1',
      hash: '#top',
    },
    replace: mockReplace,
  },
}));

vi.mock('@/services/csrf-token', () => ({
  getCSRFToken: mockGetCSRFToken,
}));

type TestError = Error & {
  code?: string;
  response?: { status?: number; data?: unknown };
  info?: { code?: number; message?: string };
};

const axiosError = (status: number, data?: unknown): TestError => {
  const error = new Error(
    `Request failed with status code ${status}`,
  ) as TestError;
  error.response = { status, data };
  return error;
};

describe('requestErrorConfig', () => {
  const errorThrower = errorConfig.errorConfig?.errorThrower;
  const errorHandler = errorConfig.errorConfig?.errorHandler;
  const interceptor = errorConfig.requestInterceptors?.[0] as (config: {
    method?: string;
    headers?: Record<string, string>;
  }) => { headers?: Record<string, string>; withCredentials?: boolean };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-08-28T10:00:00Z'));
    mockGetCSRFToken.mockReturnValue('');
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('throws BizError for non-success API envelopes', () => {
    expect(() =>
      errorThrower?.({ code: 400001, message: 'invalid input' }),
    ).toThrow('invalid input');
  });

  it('accepts success API envelopes', () => {
    expect(() =>
      errorThrower?.({ code: 100001, message: 'OK', data: {} }),
    ).not.toThrow();
  });

  it('extracts the error envelope from axios responses and shows the mapped Chinese text', () => {
    errorHandler?.(
      axiosError(500, { code: 100500, message: 'Internal server error' }),
      {},
    );

    expect(message.error).toHaveBeenCalledWith('服务器内部错误，请稍后重试');
  });

  it('keeps the specific backend message when it carries more detail', () => {
    errorHandler?.(
      axiosError(400, { code: 100400, message: 'cannot delete current admin' }),
      {},
    );

    expect(message.error).toHaveBeenCalledWith('cannot delete current admin');
  });

  it('redirects to setup when the backend reports system uninitialized', () => {
    errorHandler?.(
      axiosError(409, {
        code: 100410,
        message: 'system is not initialized',
      }),
      {},
    );

    expect(mockReplace).toHaveBeenCalledWith('/setup');
    expect(message.error).not.toHaveBeenCalled();
  });

  it('notifies and redirects on unauthorized responses', () => {
    errorHandler?.(
      axiosError(401, { code: 100401, message: 'Unauthorized' }),
      {},
    );

    expect(message.error).toHaveBeenCalledWith('登录已过期，请重新登录');
    expect(mockReplace).toHaveBeenCalledWith(
      `/user/login?redirect=${encodeURIComponent('/admins?page=1#top')}`,
    );
  });

  it('redirects to setup on system-uninitialized business errors', () => {
    const error: TestError = new Error('system is not initialized');
    error.name = 'BizError';
    error.info = { code: 100410, message: 'system is not initialized' };

    errorHandler?.(error, {});

    expect(mockReplace).toHaveBeenCalledWith('/setup');
    expect(message.error).not.toHaveBeenCalled();
  });

  it('shows business error messages', () => {
    const error: TestError = new Error('bad request');
    error.name = 'BizError';
    error.info = { message: 'bad request' };

    errorHandler?.(error, {});

    expect(message.error).toHaveBeenCalledWith('bad request');
  });

  it('falls back to a Chinese status text when the response has no envelope', () => {
    errorHandler?.(axiosError(502), {});

    expect(message.error).toHaveBeenCalledWith('服务暂不可用，请稍后重试');
  });

  it('reports request timeouts in Chinese', () => {
    const error = new Error('timeout of 10000ms exceeded') as TestError;
    error.code = 'ECONNABORTED';

    errorHandler?.(error, {});

    expect(message.error).toHaveBeenCalledWith('请求超时，请稍后重试');
  });

  it('deduplicates identical notifications within the window', () => {
    const error = axiosError(404, { code: 100404, message: 'Not found' });

    errorHandler?.(error, {});
    errorHandler?.(error, {});
    expect(message.error).toHaveBeenCalledTimes(1);
    expect(message.error).toHaveBeenCalledWith('请求的资源不存在');

    vi.setSystemTime(new Date('2026-08-28T10:00:03Z'));
    errorHandler?.(error, {});
    expect(message.error).toHaveBeenCalledTimes(2);
  });

  it('rethrows when the request opts out of shared handling', () => {
    const error = new Error('boom');

    expect(() => errorHandler?.(error, { skipErrorHandler: true })).toThrow(
      'boom',
    );
  });

  it('includes credentials and csrf header for unsafe requests', () => {
    mockGetCSRFToken.mockReturnValue('csrf-1');

    const result = interceptor({
      method: 'POST',
      headers: { Accept: 'application/json' },
    });

    expect(result.withCredentials).toBe(true);
    expect(result.headers).toEqual({
      Accept: 'application/json',
      'X-CSRF-Token': 'csrf-1',
    });
  });

  it('includes credentials without csrf header for safe requests', () => {
    const config = { headers: { Accept: 'application/json' } };
    const result = interceptor(config);

    expect(result).not.toBe(config);
    expect(result.withCredentials).toBe(true);
    expect(result.headers).toEqual({ Accept: 'application/json' });
  });
});

describe('formatApiError', () => {
  it('maps registered codes to Chinese text when the message is the default', () => {
    expect(formatApiError(100404, 'Not found')).toBe('请求的资源不存在');
  });

  it('keeps specific backend messages', () => {
    expect(formatApiError(100404, 'role not found')).toBe('role not found');
  });

  it('falls back to a generic text when nothing is known', () => {
    expect(formatApiError(999999, '')).toBe('请求失败，请稍后重试');
  });
});
