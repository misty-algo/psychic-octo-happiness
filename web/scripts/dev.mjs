#!/usr/bin/env node
/**
 * Dev server launcher.
 *
 * `max dev` reads a static proxy target and blocks interactively when its own
 * port is taken. This launcher resolves both first: it scans for the backend
 * port the Go API actually bound (it walks forward on a bind conflict) and for
 * a free dev server port, then starts `max dev` with them injected.
 */
import { spawn } from 'node:child_process';
import { DEFAULT_API_TARGET, DEFAULT_DEV_PORT, resolveApiTarget, resolveDevPort } from './dev-ports.mjs';

const apiBase = process.env.ECHO_ADMIN_WEB_API_TARGET || DEFAULT_API_TARGET;
const devBase = Number(process.env.PORT || DEFAULT_DEV_PORT);

const { target, found } = await resolveApiTarget({ base: apiBase });
if (!found) {
  console.warn(`[dev] no backend answered /api/health on ${apiBase}, proxying there anyway`);
} else if (target !== apiBase) {
  console.log(`[dev] backend moved off ${apiBase}, proxying to ${target}`);
}

const devPort = await resolveDevPort({ base: devBase });
if (devPort !== devBase) {
  console.log(`[dev] dev server port ${devBase} is occupied, using ${devPort}`);
}

// Windows cannot spawn the npm-installed `max.cmd` shim directly, so cmd.exe
// resolves it. The command stays as separate argv entries instead of using the
// shell option: nothing user-supplied is concatenated into a command line.
const isWindows = process.platform === 'win32';
const command = isWindows ? process.env.ComSpec || 'cmd.exe' : 'max';
const args = isWindows
  ? ['/d', '/s', '/c', 'max.cmd', 'dev', '--port', String(devPort)]
  : ['dev', '--port', String(devPort)];

const child = spawn(command, args, {
  stdio: 'inherit',
  env: {
    ...process.env,
    UMI_ENV: 'dev',
    MOCK: 'none',
    ECHO_ADMIN_WEB_API_TARGET: target,
    PORT: String(devPort),
  },
});

for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => child.kill(signal));
}

child.on('exit', (code, signal) => {
  if (signal) {
    process.exit(1);
  }
  process.exit(code ?? 0);
});
