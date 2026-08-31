#!/usr/bin/env node
/**
 * check-comp-coverage.mjs — Comp-field coverage validator for reports/*.md
 *
 * `advertised_comp` in a report's Machine Summary YAML is supposed to be one
 * of two things per modes/oferta.md: the JD's verbatim comp figure, or the
 * literal `null` when the JD states nothing. Both are legitimate — a JD with
 * no posted range is common and not a bug. What IS a bug is the key being
 * missing entirely: that means whoever generated the report skipped the
 * field, which is indistinguishable from a silent extraction failure unless
 * something checks for it. This script draws that line explicitly instead of
 * collapsing "no value" and "no comp in the JD" into the same blank cell,
 * which is what the dashboard's parser (and salary-gap.mjs's yamlStr) do by
 * necessity for their own purposes.
 *
 * Four categories per report:
 *   - found:        advertised_comp has a real value
 *   - stated-null:  advertised_comp is explicitly null — the JD had no comp,
 *                    and whoever wrote the report said so on purpose
 *   - missing-key:   the Machine Summary block exists but has no
 *                    advertised_comp key at all — schema violation, likely
 *                    an extraction failure, needs a human look
 *   - no-summary:   no Machine Summary block at all (legacy report, or a
 *                    reservation sentinel skipped before classification)
 *
 * Zero LLM, zero network, zero writes. Reservation sentinels (NNN-RESERVED.md)
 * are skipped outright — they are not reports.
 *
 * Run: node check-comp-coverage.mjs             (JSON)
 *      node check-comp-coverage.mjs --summary   (human-readable)
 *      node check-comp-coverage.mjs --self-test
 */

import { readFileSync, readdirSync, existsSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const CAREER_OPS = dirname(fileURLToPath(import.meta.url));
const REPORTS_DIR = join(CAREER_OPS, 'reports');
const SENTINEL_RE = /^\d+-RESERVED\.md$/;

// Same fence pattern as salary-gap.mjs's FENCE_RE — kept in sync deliberately
// (both need the identical yaml/yml fence, json rejected the same way) rather
// than imported, since salary-gap.mjs doesn't currently export it.
const FENCE_RE = /##\s*Machine Summary\s*\n+```(?:yaml|yml)?\s*\n([\s\S]*?)\n```/i;

// Classifies one report's advertised_comp coverage. Distinguishes "key absent"
// from "key present but null" by matching the raw line, not by reusing a
// helper (like salary-gap.mjs's yamlStr) that collapses both to null.
export function classifyReport(content, filename) {
  const fence = String(content || '').match(FENCE_RE);
  if (!fence) {
    return { category: 'no-summary', detail: 'no Machine Summary YAML block found' };
  }
  const body = fence[1];
  const m = body.match(/^advertised_comp:\s*(.*)$/m);
  if (!m) {
    return { category: 'missing-key', detail: 'Machine Summary present but has no advertised_comp key' };
  }
  const raw = m[1].trim();
  if (raw === '' ) {
    return { category: 'missing-key', detail: 'advertised_comp key present but has no value' };
  }
  if (/^null$/i.test(raw)) {
    return { category: 'stated-null', detail: 'advertised_comp explicitly null — JD stated no comp figure' };
  }
  const value = raw.replace(/^["']|["']$/g, '');
  return { category: 'found', detail: value };
}

function reportNum(filename) {
  const m = filename.match(/^(\d+)-/);
  return m ? m[1] : null;
}

// reports: [{ file, content }]. Pure function so the self-test runs on fixtures.
export function checkCompCoverage(reports) {
  const counts = { found: 0, 'stated-null': 0, 'missing-key': 0, 'no-summary': 0 };
  const findings = [];
  for (const { file, content } of reports) {
    const { category, detail } = classifyReport(content, file);
    counts[category] += 1;
    findings.push({ file, num: reportNum(file), category, detail });
  }
  return { totalReports: reports.length, counts, findings };
}

function loadReports(dir = REPORTS_DIR) {
  if (!existsSync(dir)) return [];
  const out = [];
  for (const file of readdirSync(dir).sort()) {
    if (!file.endsWith('.md')) continue;
    if (SENTINEL_RE.test(file)) continue; // reservation sentinel, not a report
    out.push({ file, content: readFileSync(join(dir, file), 'utf-8') });
  }
  return out;
}

function printSummary(result) {
  const { totalReports, counts, findings } = result;
  console.log(`\n${'='.repeat(70)}`);
  console.log('  Comp Coverage — career-ops reports');
  console.log(`  reports scanned: ${totalReports}`);
  console.log(`${'='.repeat(70)}\n`);
  console.log(`  ✅ found:        ${counts.found}`);
  console.log(`  ⬜ stated-null:  ${counts['stated-null']}  (JD had no comp value — not a bug)`);
  console.log(`  ⚠️  missing-key:  ${counts['missing-key']}  (schema violation — likely extraction failure)`);
  console.log(`  ⬜ no-summary:   ${counts['no-summary']}  (legacy report, no Machine Summary block)`);
  console.log('');

  const flagged = findings.filter(f => f.category === 'missing-key');
  if (flagged.length) {
    console.log(`  ${flagged.length} report(s) need a look (advertised_comp key missing):`);
    for (const f of flagged) {
      console.log(`    #${f.num ?? '?'} ${f.file}: ${f.detail}`);
    }
    console.log('');
  }

  const nullOnes = findings.filter(f => f.category === 'stated-null');
  if (nullOnes.length) {
    console.log(`  ${nullOnes.length} report(s) had no comp in the JD (informational, no action needed):`);
    for (const f of nullOnes) {
      console.log(`    #${f.num ?? '?'} ${f.file}`);
    }
    console.log('');
  }
}

// --- Self-test ---
function runSelfTest() {
  let pass = 0, fail = 0;
  const check = (cond, label) => { if (cond) pass++; else { fail++; console.error(`  FAIL: ${label}`); } };

  const found = classifyReport('# X\n\n## Machine Summary\n\n```yaml\nadvertised_comp: "80-90k EUR"\n```\n', 'x.md');
  check(found.category === 'found' && found.detail === '80-90k EUR', 'value present -> found');

  const stated = classifyReport('# X\n\n## Machine Summary\n\n```yaml\nadvertised_comp: null\n```\n', 'x.md');
  check(stated.category === 'stated-null', 'explicit null -> stated-null');

  const missing = classifyReport('# X\n\n## Machine Summary\n\n```yaml\ncompany: "X"\n```\n', 'x.md');
  check(missing.category === 'missing-key', 'key absent -> missing-key');

  const emptyVal = classifyReport('# X\n\n## Machine Summary\n\n```yaml\nadvertised_comp:\n```\n', 'x.md');
  check(emptyVal.category === 'missing-key', 'key present, empty value -> missing-key');

  const noSummary = classifyReport('# X\n\nsome legacy report with no fence\n', 'x.md');
  check(noSummary.category === 'no-summary', 'no fence -> no-summary');

  const result = checkCompCoverage([
    { file: '001-a.md', content: '## Machine Summary\n\n```yaml\nadvertised_comp: "100k"\n```\n' },
    { file: '002-b.md', content: '## Machine Summary\n\n```yaml\nadvertised_comp: null\n```\n' },
    { file: '003-c.md', content: '## Machine Summary\n\n```yaml\ncompany: "C"\n```\n' },
  ]);
  check(result.totalReports === 3, 'aggregate: totalReports');
  check(result.counts.found === 1 && result.counts['stated-null'] === 1 && result.counts['missing-key'] === 1, 'aggregate: counts per category');

  console.log(`\nSelf-test: ${pass} passed, ${fail} failed`);
  process.exit(fail > 0 ? 1 : 0);
}

// --- CLI entry ---
const args = process.argv.slice(2);
if (args.includes('--self-test')) {
  runSelfTest();
} else if (import.meta.url === `file://${process.argv[1]}`) {
  const reports = loadReports();
  const result = checkCompCoverage(reports);
  if (args.includes('--summary')) {
    printSummary(result);
  } else {
    console.log(JSON.stringify(result, null, 2));
  }
}
