import net from 'node:net';

/**
 * Local dev server port resolution.
 *
 * The Go API moves to the next free port when `system.port` is already bound,
 * so the dev proxy cannot assume 9322. This module scans the same port window
 * the backend uses and probes `/api/health`, which stays reachable before the
 * first initialization completes. Probing beats writing a port file: nothing
 * is generated, no stale file can outlive a crashed API, and a port held by an
 * unrelated process is skipped instead of being mistaken for the API.
 */

/** Dev proxy target used when no backend is reachable during the scan. */
export const DEFAULT_API_TARGET = 'http://127.0.0.1:9322';

/** Port Umi picks for the dev server when the environment says nothing. */
export const DEFAULT_DEV_PORT = 8000;

/**
 * PORT_SCAN_ATTEMPTS must match the backend `fallbackPortAttempts`: the API
 * walks at most that many consecutive ports, so the frontend has to scan the
 * same window to cover every port the API can land on.
 */
export const PORT_SCAN_ATTEMPTS = 20;

/** MAX_PORT is the highest usable TCP port; beyond it there is nothing to scan. */
export const MAX_PORT = 65535;

/** probeTimeoutMs keeps the scan short: only a local API is expected to answer. */
const probeTimeoutMs = 300;

/** Hosts the dev server may bind; both have to be free for a port to be usable. */
const LOOPBACK_HOST = '127.0.0.1';
const WILDCARD_HOST = '0.0.0.0';

/**
 * resolveApiTarget returns the first address in the scan window that answers
 * `/api/health`. found is false when nothing answers, in which case target
 * stays at base and the caller keeps the configured behaviour.
 */
export async function resolveApiTarget(options = {}) {
  const {
    base = DEFAULT_API_TARGET,
    attempts = PORT_SCAN_ATTEMPTS,
    probe = probeHealthEndpoint,
  } = options;

  const baseURL = new URL(base);
  if (!baseURL.port) {
    return { target: base, found: false };
  }

  const startPort = Number(baseURL.port);
  for (let offset = 0; offset < attempts; offset += 1) {
    const port = startPort + offset;
    if (port > MAX_PORT) {
      break;
    }
    const candidate = withPort(baseURL, port);
    if (await probe(candidate)) {
      return { target: candidate, found: true };
    }
  }
  return { target: base, found: false };
}

/** resolveDevPort returns the first free port in the scan window. */
export async function resolveDevPort(options = {}) {
  const {
    base = DEFAULT_DEV_PORT,
    attempts = PORT_SCAN_ATTEMPTS,
    isFree = isPortFree,
  } = options;

  for (let offset = 0; offset < attempts; offset += 1) {
    const port = base + offset;
    if (port > MAX_PORT) {
      break;
    }
    if (await isFree(port)) {
      return port;
    }
  }
  throw new Error(`no free dev server port in ${base}..${Math.min(base + attempts - 1, MAX_PORT)}`);
}

/**
 * probeHealthEndpoint reports whether target serves the backend health route.
 * The body is checked, not just the status, so an unrelated service holding
 * the port is not mistaken for the API.
 */
export async function probeHealthEndpoint(target) {
  try {
    const response = await fetch(new URL('/api/health', target), {
      signal: AbortSignal.timeout(probeTimeoutMs),
    });
    if (!response.ok) {
      return false;
    }
    const body = await response.json();
    return body?.status === 'ok';
  } catch {
    return false;
  }
}

/**
 * isPortFree probes both the loopback address and the wildcard address the dev
 * server binds. A port is free only when both binds succeed: Windows reports a
 * wildcard bind as free while another process holds the same port on
 * loopback, and a service bound to a single interface only fails the wildcard
 * probe. Any listen error counts as occupied, because a port the dev server
 * cannot take is not free whatever the reason.
 */
export async function isPortFree(port) {
  for (const host of [LOOPBACK_HOST, WILDCARD_HOST]) {
    if (!(await canBind(port, host))) {
      return false;
    }
  }
  return true;
}

function canBind(port, host) {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.once('error', () => resolve(false));
    server.once('listening', () => {
      server.close(() => resolve(true));
    });
    server.listen({ port, host });
  });
}

function withPort(url, port) {
  const next = new URL(url.toString());
  next.port = String(port);
  return next.origin;
}
