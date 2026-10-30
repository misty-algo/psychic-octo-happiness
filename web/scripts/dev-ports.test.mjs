import net from 'node:net';
import { describe, expect, it } from 'vitest';
import {
  DEFAULT_API_TARGET,
  MAX_PORT,
  PORT_SCAN_ATTEMPTS,
  isPortFree,
  resolveApiTarget,
  resolveDevPort,
} from './dev-ports.mjs';

/** probeOnly serves /api/health on the given ports and nowhere else. */
function probeOnly(...ports) {
  const served = new Set(ports.map(String));
  return async (target) => served.has(new URL(target).port);
}

/** portState reports the given ports as free and every other port as taken. */
function portState(...freePorts) {
  const free = new Set(freePorts);
  return async (port) => free.has(port);
}

describe('resolveApiTarget', () => {
  it('follows the backend to the port it actually bound', async () => {
    const result = await resolveApiTarget({
      base: DEFAULT_API_TARGET,
      probe: probeOnly(9324),
    });

    expect(result).toEqual({ target: 'http://127.0.0.1:9324', found: true });
  });

  it('skips a configured port held by another process', async () => {
    const result = await resolveApiTarget({
      base: DEFAULT_API_TARGET,
      probe: probeOnly(9323),
    });

    expect(result.target).toBe('http://127.0.0.1:9323');
    expect(result.found).toBe(true);
  });

  it('keeps the configured target when no port answers', async () => {
    const result = await resolveApiTarget({
      base: DEFAULT_API_TARGET,
      probe: probeOnly(),
    });

    expect(result).toEqual({ target: DEFAULT_API_TARGET, found: false });
  });

  it('stops at the scan window the backend uses', async () => {
    const beyond = 9322 + PORT_SCAN_ATTEMPTS;

    const result = await resolveApiTarget({
      base: DEFAULT_API_TARGET,
      probe: probeOnly(beyond),
    });

    expect(result.found).toBe(false);
    expect(result.target).toBe(DEFAULT_API_TARGET);
  });

  it('does not scan past the last usable port', async () => {
    const base = `http://127.0.0.1:${MAX_PORT}`;

    const result = await resolveApiTarget({ base, probe: probeOnly(MAX_PORT) });

    expect(result).toEqual({ target: base, found: true });
  });

  it('leaves a target without an explicit port untouched', async () => {
    const base = 'http://127.0.0.1';

    const result = await resolveApiTarget({ base, probe: probeOnly(80) });

    expect(result).toEqual({ target: base, found: false });
  });
});

describe('resolveDevPort', () => {
  it('uses the default port when it is free', async () => {
    await expect(resolveDevPort({ isFree: portState(8000) })).resolves.toBe(8000);
  });

  it('moves to the next free port', async () => {
    await expect(resolveDevPort({ isFree: portState(8001) })).resolves.toBe(8001);
  });

  it('fails loudly when every scanned port is occupied', async () => {
    await expect(resolveDevPort({ isFree: portState() })).rejects.toThrow(/no free dev server port/);
  });

  it('does not scan past the last usable port', async () => {
    await expect(
      resolveDevPort({ base: MAX_PORT, isFree: portState(MAX_PORT) }),
    ).resolves.toBe(MAX_PORT);
    await expect(resolveDevPort({ base: MAX_PORT, isFree: portState() })).rejects.toThrow(
      /no free dev server port/,
    );
  });
});

describe('isPortFree', () => {
  it('reports a bound port as taken', async () => {
    const server = net.createServer();
    await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
    const port = server.address().port;

    try {
      await expect(isPortFree(port)).resolves.toBe(false);
    } finally {
      await new Promise((resolve) => server.close(resolve));
    }
  });

  it('reports an unbound port as free', async () => {
    const probe = net.createServer();
    await new Promise((resolve) => probe.listen(0, '127.0.0.1', resolve));
    const port = probe.address().port;
    await new Promise((resolve) => probe.close(resolve));

    await expect(isPortFree(port)).resolves.toBe(true);
  });
});
