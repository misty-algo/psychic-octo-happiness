import type { RequestOptions } from '@@/plugin-request/request';
import type { RequestConfig } from '@umijs/max';
import { history } from '@umijs/max';
import { message } from 'antd';

import { getCSRFToken } from '@/services/csrf-token';

type ApiEnvelope = {
  code?: number;
  message?: string;
  data?: unknown;
};

type RequestError = Error & {
  code?: string;
  response?: { status?: number; data?: unknown };
  data?: unknown;
  info?: ApiEnvelope;
};

const successCode = 100001;
const systemUninitializedCode = 100410;
const unauthorizedCode = 100401;
const loginPath = '/user/login';
const setupPath = '/setup';
const csrfHeader = 'X-CSRF-Token';
const unsafeMethods = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

// 后端 apperr 注册码的展示文案。fallback 是后端注册表里的默认英文消息，
// 后端返回的消息等于 fallback（或为空）时说明没有更具体的信息，直接展示中文。
const apiErrorTexts: Record<number, { text: string; fallback: string }> = {
  100002: {
    text: '服务器开小差了，请稍后重试',
    fallback: 'Internal server error',
  },
  100003: {
    text: '请求体解析失败，请检查提交内容',
    fallback: 'Error occurred while binding the request body to the struct',
  },
  100004: { text: '请求参数校验失败', fallback: 'Validation failed' },
  100005: { text: '登录凭证无效，请重新登录', fallback: 'Token invalid' },
  100101: {
    text: '数据库暂时不可用，请稍后重试',
    fallback: 'Database error',
  },
  100102: { text: '缓存服务暂时不可用', fallback: 'Redis error' },
  100103: { text: '消息服务暂时不可用', fallback: 'Kafka error' },
  100104: {
    text: '外部服务暂时不可用，请稍后重试',
    fallback: 'External service error',
  },
  100201: {
    text: '密码处理失败，请稍后重试',
    fallback: 'Error occurred while encrypting the user password',
  },
  100202: { text: '签名无效', fallback: 'Signature is invalid' },
  100203: { text: '凭证已过期，请重新登录', fallback: 'Token expired' },
  100204: {
    text: 'Authorization 请求头格式错误',
    fallback: 'Invalid authorization header',
  },
  100205: {
    text: '缺少 Authorization 请求头',
    fallback: 'The `Authorization` header was empty',
  },
  100206: { text: '用户名或密码错误', fallback: 'Password was incorrect' },
  100207: { text: '没有权限执行该操作', fallback: 'Permission denied' },
  100208: { text: '账号已锁定，请联系管理员', fallback: 'Account is locked' },
  100209: { text: '账号已停用，请联系管理员', fallback: 'Account is disabled' },
  100210: {
    text: '尝试次数过多，请稍后再试',
    fallback: 'Too many login attempts',
  },
  100301: {
    text: '数据处理失败，请稍后重试',
    fallback: 'Encoding failed due to an error with the data',
  },
  100302: {
    text: '数据处理失败，请稍后重试',
    fallback: 'Decoding failed due to an error with the data',
  },
  100303: {
    text: '数据处理失败，请稍后重试',
    fallback: 'Data is not valid JSON',
  },
  100304: {
    text: '数据处理失败，请稍后重试',
    fallback: 'JSON data could not be encoded',
  },
  100305: {
    text: '数据处理失败，请稍后重试',
    fallback: 'JSON data could not be decoded',
  },
  100306: {
    text: '数据处理失败，请稍后重试',
    fallback: 'Data is not valid Yaml',
  },
  100307: {
    text: '数据处理失败，请稍后重试',
    fallback: 'Yaml data could not be encoded',
  },
  100308: {
    text: '数据处理失败，请稍后重试',
    fallback: 'Yaml data could not be decoded',
  },
  100400: { text: '请求参数不正确', fallback: 'Bad request' },
  100401: { text: '未登录或登录已过期', fallback: 'Unauthorized' },
  100403: { text: '没有权限执行该操作', fallback: 'Forbidden' },
  100404: { text: '请求的资源不存在', fallback: 'Not found' },
  100405: { text: '请求方法不被允许', fallback: 'Method not allowed' },
  100409: { text: '数据冲突，请刷新后重试', fallback: 'Conflict' },
  100410: { text: '系统尚未初始化', fallback: 'system is not initialized' },
  100500: {
    text: '服务器内部错误，请稍后重试',
    fallback: 'Internal server error',
  },
};

// 响应不是 JSON envelope（网关、代理、静态 404）时按 HTTP 状态码兜底。
const httpStatusTexts: Record<number, string> = {
  400: '请求参数不正确',
  401: '登录已过期，请重新登录',
  403: '没有权限执行该操作',
  404: '请求的接口不存在',
  405: '请求方法不被允许',
  409: '数据冲突，请刷新后重试',
  413: '上传内容过大',
  429: '请求过于频繁，请稍后再试',
  500: '服务器内部错误，请稍后重试',
  502: '服务暂不可用，请稍后重试',
  503: '服务维护中，请稍后重试',
  504: '网关超时，请稍后重试',
};

/**
 * 把后端错误 envelope 转成用户可读文案：注册码有中文映射时优先映射，
 * 后端返回了更具体的业务消息时保留原文，避免丢失关键语义。
 */
export function formatApiError(code?: number, messageText?: string): string {
  const known = code === undefined ? undefined : apiErrorTexts[code];
  if (!known) {
    return messageText || '请求失败，请稍后重试';
  }
  if (!messageText || messageText === known.fallback) {
    return known.text;
  }
  return messageText;
}

// 从 axios 错误里取后端 ErrorResponse envelope；umi 只在 data.success === false
// 时才调用 errorThrower，而本项目 envelope 没有 success 字段，所以非 2xx 的
// 错误信息只能在这里从 response.data 提取。
const readEnvelope = (error: RequestError): ApiEnvelope | undefined => {
  if (error.info) {
    return error.info;
  }
  const data = error.response?.data ?? error.data;
  if (
    data &&
    typeof data === 'object' &&
    ('code' in data || 'message' in data)
  ) {
    return data as ApiEnvelope;
  }
  return undefined;
};

// ProTable 等组件会并发发起多个请求，同时失败时避免重复弹同一提示。
const notifyWindowMs = 2000;
const lastNotifyAt = new Map<string, number>();
const notifyError = (text: string) => {
  const now = Date.now();
  if (now - (lastNotifyAt.get(text) ?? 0) < notifyWindowMs) {
    return;
  }
  lastNotifyAt.set(text, now);
  message.error(text);
};

export const errorConfig: RequestConfig = {
  errorConfig: {
    errorThrower: (response) => {
      const envelope = response as ApiEnvelope;
      if (typeof envelope.code === 'number' && envelope.code !== successCode) {
        const error = new Error(
          envelope.message ?? 'Request failed',
        ) as RequestError;
        error.name = 'BizError';
        error.info = envelope;
        throw error;
      }
    },
    errorHandler: (error: RequestError, opts) => {
      if (opts?.skipErrorHandler) throw error;

      const envelope = readEnvelope(error);
      const status = error.response?.status;

      if (envelope?.code === systemUninitializedCode) {
        if (history.location.pathname !== setupPath) {
          history.replace(setupPath);
        }
        return;
      }

      if (status === 401 || envelope?.code === unauthorizedCode) {
        notifyError('登录已过期，请重新登录');
        if (history.location.pathname !== loginPath) {
          history.replace(
            `${loginPath}?redirect=${encodeURIComponent(
              history.location.pathname +
                history.location.search +
                history.location.hash,
            )}`,
          );
        }
        return;
      }

      if (envelope && (envelope.code !== undefined || envelope.message)) {
        notifyError(formatApiError(envelope.code, envelope.message));
        return;
      }

      if (status !== undefined) {
        notifyError(httpStatusTexts[status] ?? `请求失败（HTTP ${status}）`);
        return;
      }

      if (typeof navigator !== 'undefined' && !navigator.onLine) {
        notifyError('网络不可用，请检查连接后重试');
        return;
      }

      if (error.code === 'ECONNABORTED' || /timeout/i.test(error.message)) {
        notifyError('请求超时，请稍后重试');
        return;
      }

      notifyError(error.message || '请求失败，请稍后重试');
    },
  },
  requestInterceptors: [
    (config: RequestOptions) => {
      const method = String(config.method ?? 'GET').toUpperCase();
      const csrfToken = unsafeMethods.has(method) ? getCSRFToken() : '';
      return {
        ...config,
        withCredentials: true,
        headers: csrfToken
          ? {
              ...config.headers,
              [csrfHeader]: csrfToken,
            }
          : config.headers,
      };
    },
  ],
  responseInterceptors: [],
};
