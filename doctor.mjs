#!/usr/bin/env node

/**
 * doctor.mjs — Setup validation for career-ops
 * Checks all prerequisites and prints a pass/fail checklist.
 */

import { copyFileSync, existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from 'fs';
import { spawnSync } from 'child_process';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';
import { createRequire } from 'module';
import yaml from 'js-yaml';
import { discoverPlugins, pluginRoots, pluginStatus } from './plugins/_engine.mjs';
import { resolveExtractorMode } from './browser-extract.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const argv = process.argv.slice(2);
const targetIdx = argv.indexOf('--target');
const projectRoot =
  targetIdx !== -1 && argv[targetIdx + 1] ? argv[targetIdx + 1] : __dirname;
const JSON_OUT = argv.includes('--json');
// --strict adds a live ATS-slug probe of portals.yml (network). Opt-in so the
// default `npm run doctor` stays fast and fully offline.
const STRICT = argv.includes('--strict');

// ANSI colors (only on TTY)
const isTTY = process.stdout.isTTY;
const green = (s) => isTTY ? `\x1b[32m${s}\x1b[0m` : s;
const red = (s) => isTTY ? `\x1b[31m${s}\x1b[0m` : s;
const yellow = (s) => isTTY ? `\x1b[33m${s}\x1b[0m` : s;
const dim = (s) => isTTY ? `\x1b[2m${s}\x1b[0m` : s;
const PLAYWRIGHT_DEBIAN_PACKAGES = [
  'fonts-freefont-ttf',
  'fonts-ipafont-gothic',
  'fonts-liberation',
  'fonts-noto-color-emoji',
  'fonts-tlwg-loma-otf',
  'fonts-unifont',
  'fonts-wqy-zenhei',
  'libasound2',
  'libasound2-data',
  'libfontenc1',
  'libice6',
  'libsm6',
  'libxaw7',
  'libxfont2',
  'libxkbfile1',
  'libxmu6',
  'libxt6',
  'x11-xkb-utils',
  'xfonts-cyrillic',
  'xfonts-encodings',
  'xfonts-scalable',
  'xfonts-utils',
  'xserver-common',
  'xvfb',
];

function buildPlaywrightDepsEnv() {
  const env = { ...process.env };
  const pathEntries = (env.PATH || '').split(':').filter(Boolean);
  const extras = [];
  if (existsSync('/usr/bin/apt-get')) extras.push('/usr/bin');
  if (existsSync('/var/lib/snapd/hostfs/usr/bin/apt-get')) {
    extras.push('/var/lib/snapd/hostfs/usr/bin');
  }
  env.PATH = [...extras, ...pathEntries].join(':');
  return env;
}

function isSnapConfinedShell() {
  if (process.env.SNAP) return true;
  try {
    const mountinfo = readFileSync('/proc/self/mountinfo', 'utf8');
    return /\/snap\/|snapd/.test(mountinfo);
  } catch {
    return false;
  }
}

function readInstalledDebianPackages() {
  const statusFiles = [
    '/var/lib/dpkg/status',
    '/var/lib/snapd/hostfs/var/lib/dpkg/status',
  ];
  for (const file of statusFiles) {
    if (!existsSync(file)) continue;
    try {
      const installed = new Set();
      const paragraphs = readFileSync(file, 'utf8').split('\n\n');
      for (const paragraph of paragraphs) {
        const pkg = paragraph.match(/^Package:\s+(.+)$/m)?.[1]?.trim();
        const status = paragraph.match(/^Status:\s+(.+)$/m)?.[1]?.trim();
        if (pkg && status === 'install ok installed') installed.add(pkg);
      }
      if (installed.size > 0) return installed;
    } catch {
      // Try next status file.
    }
  }
  return null;
}

function checkNodeVersion() {
  const major = parseInt(process.versions.node.split('.')[0]);
  if (major >= 18) {
    return { pass: true, label: `Node.js >= 18 (v${process.versions.node})` };
  }
  return {
    pass: false,
    label: `Node.js >= 18 (found v${process.versions.node})`,
    fix: 'Install Node.js 18 or later from https://nodejs.org',
  };
}

function checkDependencies() {
  if (existsSync(join(projectRoot, 'node_modules'))) {
    return { pass: true, label: 'Dependencies installed' };
  }
  return {
    pass: false,
    label: 'Dependencies not installed',
    fix: 'Run: npm install',
  };
}

function checkPlaywrightDockerVersionSync() {
  const pkgPath = join(projectRoot, 'package.json');
  const dockerfilePath = join(projectRoot, 'Dockerfile');
  if (!existsSync(pkgPath) || !existsSync(dockerfilePath)) {
    return { pass: true, label: 'Playwright Docker version sync check skipped' };
  }
  try {
    const pkg = JSON.parse(readFileSync(pkgPath, 'utf8'));
    const pkgVersion = pkg?.dependencies?.playwright || pkg?.devDependencies?.playwright;
    const dockerText = readFileSync(dockerfilePath, 'utf8');
    const imageVersion = dockerText.match(/FROM\s+mcr\.microsoft\.com\/playwright:v([0-9.]+)-/m)?.[1];
    if (!pkgVersion || !imageVersion) {
      return { warn: true, label: 'Playwright Docker version sync check inconclusive' };
    }
    if (pkgVersion === imageVersion) {
      return { pass: true, label: `Playwright package/Docker image versions aligned (${pkgVersion})` };
    }
    return {
      pass: false,
      label: `Playwright package/Docker image versions differ (${pkgVersion} vs ${imageVersion})`,
      fix: [
        `Update package.json or Dockerfile so both use the same Playwright version.`,
        `Current package.json: ${pkgVersion}`,
        `Current Dockerfile base image: ${imageVersion}`,
      ],
    };
  } catch (err) {
    return {
      warn: true,
      label: `Playwright Docker version sync check failed: ${err.message}`,
    };
  }
}

async function loadPlaywright() {
  try {
    return await import('playwright');
  } catch {
    return null;
  }
}

async function checkPlaywrightInstalled() {
  const playwright = await loadPlaywright();
  if (playwright?.chromium) {
    return { pass: true, label: 'Playwright package installed' };
  }
  return {
    pass: false,
    label: 'Playwright package not installed',
    fix: 'Run: npm install',
  };
}

function resolvePlaywrightCliPath() {
  try {
    const pkgPath = require.resolve('playwright/package.json', { paths: [projectRoot] });
    return join(dirname(pkgPath), 'cli.js');
  } catch {
    return null;
  }
}

function parseMissingPlaywrightDeps(output) {
  const lines = output.split(/\r?\n/);
  const idx = lines.findIndex((line) => /^Missing system dependencies \(\d+\):$/.test(line.trim()));
  if (idx === -1) return [];
  const deps = [];
  for (let i = idx + 1; i < lines.length; i++) {
    const line = lines[i];
    const match = line.match(/^\s{2,}(\S+)$/);
    if (match) {
      deps.push(match[1]);
      continue;
    }
    if (line.trim() === '' || line.startsWith('Installing dependencies')) {
      break;
    }
    if (!line.startsWith(' ')) break;
  }
  return deps;
}

async function checkPlaywrightSystemDeps() {
  if (process.platform !== 'linux') {
    return { pass: true, label: 'Playwright system packages check skipped (non-Linux)' };
  }
  const playwright = await loadPlaywright();
  if (!playwright?.chromium) {
    return { warn: true, label: 'Playwright system packages check skipped (Playwright not installed)' };
  }
  const cliPath = resolvePlaywrightCliPath();
  if (!cliPath || !existsSync(cliPath)) {
    return { warn: true, label: 'Playwright system packages check skipped (CLI not found)' };
  }
  const run = spawnSync(process.execPath, [cliPath, 'install-deps', 'chromium', '--dry-run'], {
    cwd: projectRoot,
    encoding: 'utf8',
    env: buildPlaywrightDepsEnv(),
  });
  const output = `${run.stdout || ''}${run.stderr || ''}`;
  const deps = parseMissingPlaywrightDeps(output);
  if (deps.length > 0) {
    return {
      pass: false,
      label: `Playwright system packages missing (${deps.length})`,
      fix: [
        ...deps,
        'Run: npx playwright install-deps chromium',
      ],
    };
  }
  if (run.status === 0) {
    return { pass: true, label: 'Playwright system packages ready' };
  }
  const installedPackages = readInstalledDebianPackages();
  if (installedPackages) {
    const missing = PLAYWRIGHT_DEBIAN_PACKAGES.filter((pkg) => !installedPackages.has(pkg));
    if (missing.length > 0) {
      return {
        pass: false,
        label: `Playwright system packages missing (${missing.length})`,
        fix: [
          ...missing,
          'Run: sudo npx playwright install-deps chromium',
        ],
      };
    }
    return { pass: true, label: 'Playwright system packages ready (Debian/Ubuntu package check)' };
  }
  return {
    warn: true,
    label: 'Playwright system packages check inconclusive',
    fix: output.trim() ? [output.trim().split(/\r?\n/)[0]] : [],
  };
}

async function checkPlaywrightLaunch() {
  const playwright = await loadPlaywright();
  if (!playwright?.chromium) {
    return {
      warn: true,
      label: 'Playwright browser launch check skipped (Playwright not installed)',
    };
  }
  const { chromium } = playwright;
  // Validate by launching — chromium.executablePath() points at Chrome for Testing
  // (full binary) but chromium.launch() may use the headless-shell binary, which
  // lives at a different path and requires a separate install. Launching directly
  // tests the exact binary the runtime uses and catches stub-installs (directory
  // present but no binary — just ABOUT + LICENSE files).
  let browser;
  try {
    browser = await chromium.launch({ headless: true });
    return { pass: true, label: 'Playwright chromium launches successfully' };
  } catch (err) {
    const full = String(err?.stack || err?.message || '');
    const libLine = full.split('\n').find((line) => line.includes('error while loading shared libraries'));
    const message =
      libLine?.trim() ||
      full.split('\n').find(Boolean) ||
      'Unknown Playwright launch failure';
    if (isSnapConfinedShell()) {
      return {
        warn: true,
        label: 'Playwright chromium launch is blocked in this snap-confined shell (expected here; repo is not broken)',
        fix: [
          message,
          'Use Docker as the default browser runtime in this shell: `npm run pdf -- <input.html> <output.pdf> ...`.',
          'The wrapper will start `docker compose` and render inside the repo Docker image so output files still land in your working tree.',
        ],
      };
    }
    return {
      pass: false,
      label: 'Playwright chromium launch failed',
      fix: [
        message,
        'If dependencies are installed but this still fails, the runtime sandbox may be blocking Chromium.',
      ],
    };
  } finally {
    try { await browser?.close(); } catch { /* ignore */ }
  }
}

function checkBrowserRuntimeGuidance() {
  if (!isSnapConfinedShell()) return { pass: true, label: 'Browser runtime: direct host shell' };
  return {
    warn: true,
    label: 'Snap-confined shell detected — prefer Docker for Playwright/PDF work',
    fix: [
      'This shell may not expose host Chromium libraries cleanly. That does not indicate a broken repo.',
      'Default path here: `npm run pdf -- <input.html> <output.pdf> [--format=letter|a4] [--report=NNN]`.',
    ],
  };
}

// The browser tools (`browser_navigate` / `browser_snapshot`) that scan / pipeline /
// apply rely on are provided by the Playwright MCP server, usually registered through a
// project-level MCP config (for example `.mcp.json`, `.claude/settings.json`, or
// `.claude/settings.local.json`). When no common config is detected, SPA job boards can
// silently return empty or stale content (#522), so doctor surfaces a non-fatal warning
// instead of letting it fail invisibly.
const PLAYWRIGHT_MCP_WARNING = 'Playwright MCP tools not detected';

function playwrightMcpConfigured(root) {
  const configFiles = ['.mcp.json', '.claude/settings.json', '.claude/settings.local.json'];
  for (const rel of configFiles) {
    const file = join(root, ...rel.split('/'));
    if (!existsSync(file)) continue;
    try {
      const servers = JSON.parse(readFileSync(file, 'utf8'))?.mcpServers;
      if (servers && typeof servers === 'object') {
        for (const server of Object.values(servers)) {
          if (JSON.stringify(server ?? '').toLowerCase().includes('playwright')) return true;
        }
      }
    } catch {
      // Malformed config — keep scanning the other locations; never crash doctor on it.
    }
  }
  return false;
}

// Report which scan/JD extractor is active (config/profile.yml → scan.extractor).
// `mcp` (default) uses the browser MCP; `cli` uses browser-extract.mjs. When cli
// is selected but the helper is missing, the modes fall back to MCP — surface
// that as a warning, never a failure.
function checkScanExtractor(root) {
  const mode = resolveExtractorMode(join(root, 'config', 'profile.yml'));
  if (mode === 'cli') {
    if (existsSync(join(root, 'browser-extract.mjs'))) {
      return { pass: true, label: 'Scan extractor: cli (browser-extract.mjs)' };
    }
    return {
      warn: true,
      label: 'Scan extractor: cli set, but browser-extract.mjs is missing — falls back to MCP',
      fix: ['Restore browser-extract.mjs, or set `scan.extractor: mcp` in config/profile.yml.'],
    };
  }
  return { pass: true, label: 'Scan extractor: mcp (default)' };
}

function checkPlaywrightMcp(root) {
  if (playwrightMcpConfigured(root)) {
    return { pass: true, label: 'Playwright MCP server configured' };
  }
  return {
    warn: true,
    label: PLAYWRIGHT_MCP_WARNING,
    fix: [
      'Browser-driven JD fetching and liveness checks (scan / pipeline / apply) need the',
      'Playwright MCP server. No project-level MCP config was detected in `.mcp.json`',
      'or `.claude/settings*.json`, so SPA job boards may return empty or stale content.',
      'Tracking: https://github.com/santifer/career-ops/issues/506',
    ],
  };
}

// Single source of truth for the four user-layer prerequisites (the list
// AGENTS.md "First Run" documents). BOTH the human checklist (`checkPrereq`)
// and the machine-readable cold-start state (`onboardingState`) derive from
// THIS array, so they cannot drift. Paths use "/" and are split for join().
const USER_LAYER_PREREQS = [
  {
    path: 'cv.md',
    fix: [
      'Create cv.md in the project root with your CV in markdown',
      'See examples/ for reference CVs',
    ],
  },
  {
    path: 'config/profile.yml',
    fix: [
      'Run: cp config/profile.example.yml config/profile.yml',
      'Then edit it with your details',
    ],
  },
  {
    path: 'modes/_profile.md',
    fix: [
      'Run: cp modes/_profile.template.md modes/_profile.md',
      'Then customize your archetypes / targeting narrative',
    ],
  },
  {
    path: 'portals.yml',
    fix: [
      'Run: cp templates/portals.example.yml portals.yml',
      'Then customize with your target companies',
    ],
  },
];

function prereqPresent(root, path) {
  return existsSync(join(root, ...path.split('/')));
}

function checkPrereq({ path, fix }) {
  if (prereqPresent(projectRoot, path)) {
    return { pass: true, label: `${path} found` };
  }
  return { pass: false, label: `${path} not found`, fix };
}

function checkFonts() {
  const fontsDir = join(projectRoot, 'fonts');
  if (!existsSync(fontsDir)) {
    return {
      pass: false,
      label: 'fonts/ directory not found',
      fix: 'The fonts/ directory is required for PDF generation',
    };
  }
  try {
    const files = readdirSync(fontsDir);
    if (files.length === 0) {
      return {
        pass: false,
        label: 'fonts/ directory is empty',
        fix: 'The fonts/ directory must contain font files for PDF generation',
      };
    }
  } catch {
    return {
      pass: false,
      label: 'fonts/ directory not readable',
      fix: 'Check permissions on the fonts/ directory',
    };
  }
  return { pass: true, label: 'Fonts directory ready' };
}

function checkAutoDir(name) {
  const dirPath = join(projectRoot, name);
  if (existsSync(dirPath)) {
    return { pass: true, label: `${name}/ directory ready` };
  }
  try {
    mkdirSync(dirPath, { recursive: true });
    return { pass: true, label: `${name}/ directory ready (auto-created)` };
  } catch {
    return {
      pass: false,
      label: `${name}/ directory could not be created`,
      fix: `Run: mkdir ${name}`,
    };
  }
}

// --strict only: probe the ATS slug of every tracked company in portals.yml so
// a typo'd slug (which 404s silently on scans) surfaces here. Skipped gracefully
// when portals.yml is absent. Delegates to verify-portals.mjs so there is one
// slug-probing implementation. Network-bound, hence opt-in.
async function checkPortalSlugs(root) {
  const portalsPath = join(root, 'portals.yml');
  if (!existsSync(portalsPath)) {
    return { pass: true, label: 'ATS slugs: no portals.yml yet (skipped)' };
  }
  try {
    const { verifyPortalsFile } = await import('./verify-portals.mjs');
    const { results } = await verifyPortalsFile(portalsPath);
    const unresolved = results.filter((r) => r.status === 'missing');
    if (unresolved.length === 0) {
      return { pass: true, label: 'All ATS slugs in portals.yml resolve' };
    }
    return {
      pass: false,
      label: `${unresolved.length} ATS slug(s) in portals.yml do not resolve`,
      fix: [
        ...unresolved.map((r) => {
          let line = `${r.name}: ${r.ats || '?'}/${r.slug || '?'} — ${r.reason || 'unresolved'}`;
          if (r.suggested) line += ` → try ${r.suggested.ats}/${r.suggested.slug}`;
          return line;
        }),
        'Probe variants with: node verify-portals.mjs --add "<company>"',
      ],
    };
  } catch (err) {
    return { warn: true, label: `ATS slug check skipped: ${err.message}` };
  }
}

const PIPELINE_SKELETON = `# Pipeline — Pending URLs

Paste job URLs below as \`- [ ] {url}\` then run \`/career-ops pipeline\`.

## Pending

## Processed
`;

function checkPipelineFile() {
  const filePath = join(projectRoot, 'data', 'pipeline.md');
  if (existsSync(filePath)) {
    return { pass: true, label: 'data/pipeline.md ready' };
  }
  try {
    writeFileSync(filePath, PIPELINE_SKELETON, 'utf-8');
    return { pass: true, label: 'data/pipeline.md ready (auto-created)' };
  } catch {
    return {
      pass: false,
      label: 'data/pipeline.md could not be created',
      fix: 'Run: mkdir -p data && touch data/pipeline.md',
    };
  }
}

// Discover plugins + their non-secret config block, synchronously. Used by both
// the human check and the --json onboarding state.
function readPluginConfigSync(root) {
  const cfgPath = join(root, 'config', 'plugins.yml');
  if (!existsSync(cfgPath)) return {};
  try { return yaml.load(readFileSync(cfgPath, 'utf8')) || {}; } catch { return {}; }
}

// Plugin layer health: list discovered plugins + whether each enabled one's keys
// are present. WARN-not-FAIL so a half-configured plugin never blocks setup.
function checkPlugins(root) {
  let manifests;
  try { manifests = discoverPlugins(pluginRoots(root)); } catch { return { pass: true, label: 'Plugins: none' }; }
  if (manifests.length === 0) return { pass: true, label: 'Plugins: none installed' };
  const cfg = readPluginConfigSync(root);
  const lines = [];
  const fixes = [];
  for (const m of manifests) {
    const s = pluginStatus(m, cfg);
    lines.push(`${m.id} (${s.enabled ? 'enabled' : s.configured ? `missing ${s.missingEnv.join(', ')}` : 'off'})`);
    if (s.configured && s.missingEnv.length) fixes.push(`${m.id}: add ${s.missingEnv.join(', ')} to .env`);
  }
  const label = `Plugins: ${lines.join(', ')}`;
  return fixes.length ? { warn: true, label, fix: fixes } : { pass: true, label };
}

async function main() {
  console.log('\ncareer-ops doctor');
  console.log('================\n');

  const checks = [
    checkNodeVersion(),
    checkDependencies(),
    checkPlaywrightDockerVersionSync(),
    await checkPlaywrightInstalled(),
    await checkPlaywrightSystemDeps(),
    await checkPlaywrightLaunch(),
    checkBrowserRuntimeGuidance(),
    checkPlaywrightMcp(projectRoot),
    checkScanExtractor(projectRoot),
    ...USER_LAYER_PREREQS.map(checkPrereq),
    checkFonts(),
    checkAutoDir('data'),
    checkPipelineFile(),
    checkAutoDir('output'),
    checkAutoDir('reports'),
    checkPlugins(projectRoot),
  ];

  // Network-bound ATS slug probe — only under --strict.
  if (STRICT) {
    checks.push(await checkPortalSlugs(projectRoot));
  }

  let failures = 0;
  let warnings = 0;

  for (const result of checks) {
    const fixes = Array.isArray(result.fix) ? result.fix : result.fix ? [result.fix] : [];
    if (result.warn) {
      warnings++;
      console.log(`${yellow('⚠')} ${result.label}`);
      for (const hint of fixes) {
        console.log(`  ${dim('→ ' + hint)}`);
      }
    } else if (result.pass) {
      console.log(`${green('✓')} ${result.label}`);
    } else {
      failures++;
      console.log(`${red('✗')} ${result.label}`);
      for (const hint of fixes) {
        console.log(`  ${dim('→ ' + hint)}`);
      }
    }
  }

  console.log('');
  if (failures > 0) {
    console.log(`Result: ${failures} issue${failures === 1 ? '' : 's'} found. Fix them and run \`npm run doctor\` again.`);
    process.exit(1);
  } else {
    const warnNote = warnings > 0 ? ` (${warnings} warning${warnings === 1 ? '' : 's'} — see above)` : '';
    console.log(`Result: All checks passed${warnNote}. You're ready to go! Run \`claude\` (or \`opencode\`) to start.`);
    console.log('');
    console.log('Join the community: https://discord.gg/8pRpHETxa4');
    process.exit(0);
  }
}

// Single source of truth for the cold-start state: the same four user-layer
// prerequisites that AGENTS.md "First Run" lists. `--json` turns the trigger into
// a deterministic mechanism the agent runs (instead of re-deriving it from prose),
// and `--target <dir>` lets the test suite point it at a simulated virgin env.
function onboardingState(root) {
  const autoCopied = [];
  const templates = [
    { target: 'modes/_profile.md', template: 'modes/_profile.template.md' },
    { target: 'modes/_custom.md', template: 'modes/_custom.template.md' },
  ];
  for (const { target, template } of templates) {
    const targetPath = join(root, ...target.split('/'));
    const templatePath = join(root, ...template.split('/'));
    if (!existsSync(targetPath) && existsSync(templatePath)) {
      try {
        copyFileSync(templatePath, targetPath);
        autoCopied.push(target);
      } catch {
        // Gracefully handle read-only filesystems (e.g., CI/CD or containerized environments)
        // by leaving the file uncopied and letting onboardingNeeded/prereq checks handle it.
      }
    }
  }
  const missing = USER_LAYER_PREREQS
    .filter(({ path }) => !prereqPresent(root, path))
    .map(({ path }) => path);
  const warnings = playwrightMcpConfigured(root) ? [] : [PLAYWRIGHT_MCP_WARNING];
  let plugins = [];
  try {
    const cfg = readPluginConfigSync(root);
    plugins = discoverPlugins(pluginRoots(root)).map((m) => {
      const s = pluginStatus(m, cfg);
      return { id: m.id, hooks: m.hooks, enabled: s.enabled, missingEnv: s.missingEnv };
    });
  } catch { plugins = []; }
  return { onboardingNeeded: missing.length > 0, missing, warnings, autoCopied, plugins };
}

if (JSON_OUT) {
  console.log(JSON.stringify(onboardingState(projectRoot)));
  process.exit(0);
} else {
  main().catch((err) => {
    console.error('doctor.mjs failed:', err.message);
    process.exit(1);
  });
}
