#!/usr/bin/env node

/**
 * update-safe.mjs — wraps `node update-system.mjs apply` with automatic
 * 3-way reconciliation of local dashboard/ customizations.
 *
 * WHY THIS EXISTS: update-system.mjs treats `dashboard/` as pure system
 * layer and does a wholesale `git checkout FETCH_HEAD -- dashboard/`, which
 * overwrites every file that exists on both sides — including any shared
 * file (catalog.go, pipeline.go, career.go, main.go, ...) this fork has
 * customized but not yet contributed upstream. Brand-new local-only files
 * (e.g. a new screen) survive untouched, since upstream doesn't have them
 * to check out over — the risk is specifically files edited on both sides.
 *
 * HOW IT WORKS: every successful `update-system.mjs apply` run creates a
 * commit `chore: auto-update system files to v{X}` whose tree, restricted
 * to the updated paths, is pure upstream content for that version. That
 * commit is a ready-made 3-way-merge base marker — no separate state file
 * needed. This script:
 *
 *   1. Finds the most recent such commit (from BEFORE this run) as `base`.
 *   2. Snapshots the current (pre-update) dashboard/ file contents as `ours`.
 *   3. Runs `node update-system.mjs apply`.
 *   4. Treats the new post-update commit's dashboard/ tree as `theirs`.
 *   5. Runs `git merge-file` per shared file (base/ours/theirs) and writes
 *      the result back.
 *   6. Rebuilds + tests the dashboard. Only if the merge was conflict-free
 *      AND the build+tests pass does it commit the reconciliation, as a
 *      follow-up commit on top of update-system.mjs's own commit. Any
 *      conflict or build/test failure leaves the working tree exactly as
 *      the merge left it (conflict markers and all, same as a normal git
 *      merge conflict) with instructions — nothing is force-committed.
 *
 * This file deliberately lives OUTSIDE update-system.mjs's SYSTEM_PATHS, so
 * future updates never overwrite or delete it.
 *
 * Usage: node update-safe.mjs
 */

import { execFileSync, spawnSync } from 'child_process';
import { readFileSync, writeFileSync, mkdtempSync, rmSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';
import { tmpdir } from 'os';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = __dirname;
const DASHBOARD_PREFIX = 'dashboard/';
const AUTO_UPDATE_COMMIT_RE = /^chore: auto-update system files to v(\S+)/;

function git(...args) {
  return execFileSync('git', args, { cwd: ROOT, encoding: 'utf-8' }).trim();
}

function gitLines(...args) {
  const out = git(...args);
  return out ? out.split('\n') : [];
}

function assertCleanWorkingTree() {
  const status = git('status', '--porcelain');
  if (status.trim()) {
    console.error('Working tree is not clean. Commit or stash your changes before running update-safe.mjs.');
    process.exit(1);
  }
}

/** Most recent "chore: auto-update system files to vX" commit, or null. */
function findLastAutoUpdateCommit() {
  for (const line of gitLines('log', '--format=%H %s')) {
    const sp = line.indexOf(' ');
    const subject = line.slice(sp + 1);
    if (AUTO_UPDATE_COMMIT_RE.test(subject)) return line.slice(0, sp);
  }
  return null;
}

function listTrackedDashboardFiles(ref) {
  return gitLines('ls-tree', '-r', '--name-only', ref, '--', DASHBOARD_PREFIX);
}

/** File content at a given ref, or null if the path doesn't exist there. */
function showFile(ref, path) {
  const res = spawnSync('git', ['show', `${ref}:${path}`], { cwd: ROOT, encoding: 'utf-8' });
  if (res.status !== 0) return null;
  return res.stdout;
}

/** 3-way merge via `git merge-file -p`. Returns { content, conflicted }. */
function mergeFile(oursContent, baseContent, theirsContent) {
  const dir = mkdtempSync(join(tmpdir(), 'update-safe-'));
  const oursPath = join(dir, 'ours');
  const basePath = join(dir, 'base');
  const theirsPath = join(dir, 'theirs');
  try {
    writeFileSync(oursPath, oursContent);
    writeFileSync(basePath, baseContent ?? '');
    writeFileSync(theirsPath, theirsContent);
    const res = spawnSync('git', ['merge-file', '-p', oursPath, basePath, theirsPath], {
      cwd: ROOT,
      encoding: 'utf-8',
    });
    // merge-file exit code = number of conflicts (0 = clean); stdout is the
    // merged content either way (with conflict markers when not clean).
    return { content: res.stdout, conflicted: (res.status ?? 0) > 0 };
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

function runDashboardBuildAndTest() {
  console.log('Verifying dashboard build + tests...');
  const build = spawnSync('go', ['build', './...'], { cwd: join(ROOT, 'dashboard'), encoding: 'utf-8' });
  if (build.status !== 0) {
    console.error('go build failed:\n' + build.stdout + build.stderr);
    return false;
  }
  const test = spawnSync('go', ['test', './...'], { cwd: join(ROOT, 'dashboard'), encoding: 'utf-8' });
  if (test.status !== 0) {
    console.error('go test failed:\n' + test.stdout + test.stderr);
    return false;
  }
  const rebuild = spawnSync('go', ['build', '-o', 'career-dashboard', '.'], {
    cwd: join(ROOT, 'dashboard'),
    encoding: 'utf-8',
  });
  if (rebuild.status !== 0) {
    console.error('dashboard binary rebuild failed:\n' + rebuild.stdout + rebuild.stderr);
    return false;
  }
  console.log('Dashboard build + tests OK.');
  return true;
}

function main() {
  assertCleanWorkingTree();

  const baseCommit = findLastAutoUpdateCommit();
  if (!baseCommit) {
    console.log('No prior "chore: auto-update system files" commit found — nothing to reconcile against.');
    console.log('Running a plain update. If you have local dashboard/ customizations, verify them manually afterwards.');
    execFileSync(process.execPath, ['update-system.mjs', 'apply'], { cwd: ROOT, stdio: 'inherit' });
    return;
  }
  console.log(`Reconciliation base: ${baseCommit.slice(0, 12)} (last upstream sync)`);

  // 1. Snapshot current (pre-update) dashboard/ files as "ours".
  const oursFiles = listTrackedDashboardFiles('HEAD');
  const oursSnapshot = new Map(oursFiles.map((f) => [f, readFileSync(join(ROOT, f), 'utf-8')]));

  // 2. Run the real updater.
  console.log('Running node update-system.mjs apply ...');
  try {
    execFileSync(process.execPath, ['update-system.mjs', 'apply'], { cwd: ROOT, stdio: 'inherit' });
  } catch {
    console.error('update-system.mjs apply failed — aborting reconciliation.');
    process.exit(1);
  }

  const theirsCommit = findLastAutoUpdateCommit();
  if (!theirsCommit || theirsCommit === baseCommit) {
    console.log('No new system update was applied. Nothing to reconcile.');
    return;
  }
  const versionMatch = AUTO_UPDATE_COMMIT_RE.exec(git('log', '-1', '--format=%s', theirsCommit));
  const version = versionMatch ? versionMatch[1] : 'unknown';

  // 3. Reconcile every shared dashboard file.
  const changedFiles = [];
  const conflictedFiles = [];
  for (const [file, oursContent] of oursSnapshot) {
    const theirsContent = showFile(theirsCommit, file);
    if (theirsContent === null) continue; // removed/absent upstream — nothing to merge
    if (theirsContent === oursContent) continue; // upstream didn't change this file

    const baseContent = showFile(baseCommit, file); // null if the file is new upstream
    const { content, conflicted } = mergeFile(oursContent, baseContent, theirsContent);
    writeFileSync(join(ROOT, file), content);
    if (conflicted) {
      conflictedFiles.push(file);
    } else {
      changedFiles.push(file);
      git('add', '--', file);
    }
  }

  if (conflictedFiles.length > 0) {
    console.error(`\n${conflictedFiles.length} file(s) have merge conflicts between your local customizations and v${version}:`);
    for (const f of conflictedFiles) console.error(`  ${f}`);
    console.error('\nResolve the conflict markers by hand, `git add` each file, then commit.');
    console.error(`(Reconciliation base: ${baseCommit}, upstream: ${theirsCommit})`);
    process.exitCode = 1;
    return;
  }

  if (changedFiles.length === 0) {
    console.log(`Local dashboard/ customizations were already compatible with v${version} — nothing to reconcile.`);
    return;
  }

  console.log(`Reconciled ${changedFiles.length} file(s):`);
  for (const f of changedFiles) console.log(`  ${f}`);

  if (!runDashboardBuildAndTest()) {
    console.error('\nBuild/tests failed after reconciliation. Changes are staged but NOT committed — fix and commit manually.');
    process.exitCode = 1;
    return;
  }

  git(
    'commit',
    '-m',
    `fix(dashboard): reapply local customizations over v${version} sync\n\nCo-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>`,
    '--',
    ...changedFiles,
  );
  console.log(`\nCommitted reconciliation for v${version}. Working tree is clean.`);
}

main();
