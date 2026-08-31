#!/usr/bin/env node

/**
 * backup-personal.mjs — encrypted local backup of the gitignored user-layer
 * files (cv.md, config/profile.yml, portals.yml, modes/_profile.md,
 * modes/_custom.md, data/, reports/, interview-prep/, writing-samples/,
 * documents/, jds/, article-digest.md, voice-dna.md, config/cv-facts.json,
 * config/benchmarks.yml).
 *
 * WHY THIS EXISTS: these files are intentionally excluded from git (see
 * .gitignore + DATA_CONTRACT.md) so update-system.mjs can never overwrite
 * them. That protects them from updates, but it also means they have NO
 * backup anywhere — a lost/corrupted disk loses the CV, profile, portal
 * config, and entire application history with no recovery path. This script
 * closes that gap with a local, encrypted, timestamped archive.
 *
 * HOW IT WORKS:
 *   1. Generates (once) a random passphrase at ~/.config/career-ops-backup/
 *      passphrase (chmod 600). YOU are responsible for backing this up
 *      separately (e.g. a password manager) — without it, the encrypted
 *      archives are permanently unrecoverable.
 *   2. Tars whatever PATHS below actually exist in this checkout.
 *   3. Encrypts with `gpg --symmetric --cipher-algo AES256`, using that
 *      passphrase, non-interactively.
 *   4. Writes ~/backups/career-ops-personal-data/{repo-basename}-{ISO}.tar.gz.gpg
 *   5. Prunes to the most recent KEEP archives so the directory doesn't grow
 *      unbounded.
 *
 * This file deliberately lives OUTSIDE update-system.mjs's SYSTEM_PATHS (see
 * config/local-paths.txt) so future updates never overwrite or delete it.
 *
 * Usage: node backup-personal.mjs [--quiet]
 */

import { execFileSync, spawnSync } from 'child_process';
import { existsSync, mkdirSync, writeFileSync, readdirSync, statSync, unlinkSync, chmodSync } from 'fs';
import { join, basename, dirname } from 'path';
import { fileURLToPath } from 'url';
import { homedir } from 'os';
import { randomBytes } from 'crypto';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = __dirname;
const QUIET = process.argv.includes('--quiet');

const PATHS = [
  'cv.md',
  'article-digest.md',
  'voice-dna.md',
  'config/profile.yml',
  'config/cv-facts.json',
  'config/benchmarks.yml',
  'portals.yml',
  'modes/_profile.md',
  'modes/_custom.md',
  'data',
  'reports',
  'interview-prep',
  'writing-samples',
  'documents',
  'jds',
];

const BACKUP_DIR = join(homedir(), 'backups', 'career-ops-personal-data');
const PASSPHRASE_DIR = join(homedir(), '.config', 'career-ops-backup');
const PASSPHRASE_FILE = join(PASSPHRASE_DIR, 'passphrase');
const KEEP = 20;

function log(...args) {
  if (!QUIET) console.log(...args);
}

function ensurePassphrase() {
  if (existsSync(PASSPHRASE_FILE)) return;
  mkdirSync(PASSPHRASE_DIR, { recursive: true, mode: 0o700 });
  const passphrase = randomBytes(32).toString('hex');
  writeFileSync(PASSPHRASE_FILE, passphrase + '\n', { mode: 0o600 });
  chmodSync(PASSPHRASE_FILE, 0o600);
  log(`Generated a new backup passphrase at ${PASSPHRASE_FILE}`);
  log('IMPORTANT: back this up separately (e.g. a password manager). Without it, encrypted backups cannot be decrypted.');
}

function existingPaths() {
  return PATHS.filter((p) => existsSync(join(ROOT, p)));
}

function pruneOldBackups() {
  if (!existsSync(BACKUP_DIR)) return;
  const files = readdirSync(BACKUP_DIR)
    .filter((f) => f.endsWith('.tar.gz.gpg'))
    .map((f) => ({ f, mtime: statSync(join(BACKUP_DIR, f)).mtimeMs }))
    .sort((a, b) => b.mtime - a.mtime);
  for (const { f } of files.slice(KEEP)) {
    unlinkSync(join(BACKUP_DIR, f));
    log(`Pruned old backup: ${f}`);
  }
}

function main() {
  const paths = existingPaths();
  if (paths.length === 0) {
    log('No personal-data paths found to back up. Nothing to do.');
    return;
  }

  mkdirSync(BACKUP_DIR, { recursive: true, mode: 0o700 });
  ensurePassphrase();

  const stamp = new Date().toISOString().replace(/[:.]/g, '-');
  const repoName = basename(ROOT);
  const tarPath = join(BACKUP_DIR, `${repoName}-${stamp}.tar.gz`);
  const encPath = `${tarPath}.gpg`;

  log(`Archiving ${paths.length} path(s): ${paths.join(', ')}`);
  execFileSync('tar', ['-czf', tarPath, '-C', ROOT, ...paths], { stdio: QUIET ? 'ignore' : 'inherit' });

  const gpgArgs = [
    '--batch',
    '--yes',
    '--pinentry-mode', 'loopback',
    '--passphrase-file', PASSPHRASE_FILE,
    '--symmetric',
    '--cipher-algo', 'AES256',
    '-o', encPath,
    tarPath,
  ];
  const res = spawnSync('gpg', gpgArgs, { stdio: QUIET ? 'ignore' : 'inherit' });
  unlinkSync(tarPath); // never leave the unencrypted tar on disk
  if (res.status !== 0) {
    console.error('gpg encryption failed — backup NOT written.');
    process.exit(1);
  }

  pruneOldBackups();
  log(`Backup written: ${encPath}`);
  log(`Decrypt with: gpg --batch --yes --pinentry-mode loopback --passphrase-file ${PASSPHRASE_FILE} -o out.tar.gz -d ${basename(encPath)}`);
}

main();
