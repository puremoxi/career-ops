import { existsSync, readFileSync } from 'fs';
import { dirname, isAbsolute, relative, resolve, sep } from 'path';
import { fileURLToPath } from 'url';
import { spawnSync } from 'child_process';

/**
 * Shared browser runtime for Playwright-backed CLI flows.
 *
 * Migrated callers:
 * - generate-pdf.mjs
 * - check-liveness.mjs
 * - scan.mjs (--verify)
 * - browser-extract.mjs
 * - scan-ats-full.mjs (--liveness)
 *
 * Remaining direct Playwright callers to migrate later:
 * - archive-posting.mjs
 * - img-to-pdf.mjs
 * - openrouter-runner.mjs
 * - upskill.mjs
 */

const __dirname = dirname(fileURLToPath(import.meta.url));
const PROJECT_ROOT = __dirname;
const PLAYWRIGHT_WRAPPER = resolve(PROJECT_ROOT, 'scripts', 'playwright-chromium-wrapper.sh');
const INSIDE_DOCKER = existsSync('/.dockerenv') || process.env.CAREER_OPS_IN_DOCKER === '1';
const PLAYWRIGHT_RUNTIME_DIR =
  process.env.CAREER_OPS_PLAYWRIGHT_RUNTIME_DIR ||
  resolve(PROJECT_ROOT, '.playwright-runtime', 'chrome-headless-shell-linux64');
const PLAYWRIGHT_RUNTIME_BIN =
  process.env.CAREER_OPS_PLAYWRIGHT_BROWSER_BIN ||
  resolve(PLAYWRIGHT_RUNTIME_DIR, 'chrome-headless-shell');

export function repoRelativeProjectPath(pathValue) {
  if (!pathValue) return '';
  const rel = relative(PROJECT_ROOT, resolve(pathValue));
  if (rel === '' || rel.startsWith('..') || isAbsolute(rel)) return '';
  return rel.split(sep).join('/');
}

export function dockerComposeCmd() {
  const docker = spawnSync('bash', ['-lc', 'command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1'], {
    cwd: PROJECT_ROOT,
    stdio: 'ignore',
  });
  if (docker.status === 0) return ['docker', 'compose'];
  const compose = spawnSync('bash', ['-lc', 'command -v docker-compose >/dev/null 2>&1'], {
    cwd: PROJECT_ROOT,
    stdio: 'ignore',
  });
  if (compose.status === 0) return ['docker-compose'];
  return null;
}

export function isSnapConfinedShell() {
  if (process.env.SNAP) return true;
  try {
    const mountinfo = readFileSync('/proc/self/mountinfo', 'utf8');
    return /\/snap\/|snapd/.test(mountinfo);
  } catch {
    return false;
  }
}

export function shouldPreferDockerRuntime() {
  if (INSIDE_DOCKER) return false;
  if (process.env.CAREER_OPS_BROWSER_FORCE_LOCAL === '1') return false;
  if (process.env.CAREER_OPS_BROWSER_FORCE_DOCKER === '1') return true;
  return isSnapConfinedShell();
}

export function shouldUseLocalPlaywrightWrapper() {
  if (INSIDE_DOCKER) return false;
  return existsSync(PLAYWRIGHT_WRAPPER) && existsSync(PLAYWRIGHT_RUNTIME_BIN);
}

export async function launchChromium(chromium, options = {}) {
  return chromium.launch(
    shouldUseLocalPlaywrightWrapper()
      ? { ...options, executablePath: PLAYWRIGHT_WRAPPER }
      : options
  );
}

export function runNodeScriptInDocker(scriptRelPath, args = [], { env = {}, service = 'career-ops' } = {}) {
  const compose = dockerComposeCmd();
  if (!compose) return null;
  const up = spawnSync(compose[0], [...compose.slice(1), 'up', '-d', service], {
    cwd: PROJECT_ROOT,
    stdio: 'inherit',
    env: process.env,
  });
  if (up.status !== 0) {
    throw new Error('Docker Compose service startup failed');
  }
  const run = spawnSync(
    compose[0],
    [
      ...compose.slice(1),
      'exec',
      '-T',
      '-e',
      'CAREER_OPS_IN_DOCKER=1',
      ...Object.entries(env).flatMap(([key, value]) => ['-e', `${key}=${value}`]),
      service,
      'node',
      scriptRelPath,
      ...args,
    ],
    {
      cwd: PROJECT_ROOT,
      stdio: 'inherit',
      env: process.env,
    },
  );
  return run.status ?? 1;
}

export async function delegateNodeScriptToDocker(scriptRelPath, args = process.argv.slice(2), opts = {}) {
  if (!shouldPreferDockerRuntime()) return null;
  return runNodeScriptInDocker(scriptRelPath, args, opts);
}
