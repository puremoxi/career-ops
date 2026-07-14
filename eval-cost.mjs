#!/usr/bin/env node
/**
 * eval-cost.mjs — Lightweight local spend counter for career-ops evaluations.
 *
 * career-ops does not intercept API responses, so this cannot read real
 * token counts — it counts evaluations per spend_tier and multiplies by an
 * assumed average tokens/evaluation (editable below) and published per-tier
 * pricing. Treat the dollar figures as an estimate, not a bill.
 *
 * Run: node eval-cost.mjs add --company "Acme" --tier standard [--role "Creative Director"] [--mode oferta]
 *      node eval-cost.mjs                     (JSON to stdout)
 *      node eval-cost.mjs --summary            (writes + prints data/eval-cost-summary.md)
 */

import { readFileSync, writeFileSync, existsSync, appendFileSync, mkdirSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath, pathToFileURL } from 'url';

const CAREER_OPS = dirname(fileURLToPath(import.meta.url));
const LOG_FILE = join(CAREER_OPS, 'data/eval-log.tsv');
const SUMMARY_FILE = join(CAREER_OPS, 'data/eval-cost-summary.md');

// Published per-tier pricing ($ / 1M tokens). Update if career-ops's
// batch-runner.sh spend_tier -> model mapping or Anthropic's pricing changes.
const TIER_PRICING = {
  economy: { model: 'claude-haiku-4-5', inputPerM: 1.0, outputPerM: 5.0 },
  standard: { model: 'claude-sonnet-4-6', inputPerM: 3.0, outputPerM: 15.0 },
  premium: { model: 'claude-opus-4-8', inputPerM: 5.0, outputPerM: 25.0 },
};

// Rough assumption for a single career-ops evaluation (JD + CV + shared mode
// instructions in, report + tailored content out). No real measurement is
// available — adjust these two numbers if your evaluations run consistently
// larger or smaller than this.
const ASSUMED_INPUT_TOKENS = 15000;
const ASSUMED_OUTPUT_TOKENS = 3000;

const args = process.argv.slice(2);
const isAdd = args[0] === 'add';
const summaryMode = args.includes('--summary');

function flagValue(name) {
  const idx = args.indexOf(name);
  return idx !== -1 ? args[idx + 1] : null;
}

function addEntry() {
  const company = flagValue('--company');
  const tier = flagValue('--tier') || 'standard';
  const role = flagValue('--role') || '';
  const mode = flagValue('--mode') || '';

  if (!company) {
    console.error('eval-cost.mjs add: --company is required');
    process.exit(1);
  }
  if (!TIER_PRICING[tier]) {
    console.error(`eval-cost.mjs add: unknown --tier "${tier}" (expected economy | standard | premium)`);
    process.exit(1);
  }

  mkdirSync(dirname(LOG_FILE), { recursive: true });
  const date = new Date().toISOString().slice(0, 10);
  const row = [date, company, role, tier, mode].join('\t') + '\n';
  appendFileSync(LOG_FILE, row);
  return { logged: { date, company, role, tier, mode } };
}

function readLog() {
  if (!existsSync(LOG_FILE)) return [];
  return readFileSync(LOG_FILE, 'utf8')
    .split('\n')
    .filter(Boolean)
    .map((line) => {
      const [date, company, role, tier, mode] = line.split('\t');
      return { date, company, role, tier: tier || 'standard', mode };
    });
}

function summarize() {
  const entries = readLog();
  const byTier = { economy: 0, standard: 0, premium: 0 };
  for (const e of entries) {
    if (byTier[e.tier] === undefined) continue; // ignore rows with an unrecognized/legacy tier value
    byTier[e.tier]++;
  }

  const rows = Object.entries(byTier).map(([tier, count]) => {
    const p = TIER_PRICING[tier];
    const inputCost = (count * ASSUMED_INPUT_TOKENS / 1_000_000) * p.inputPerM;
    const outputCost = (count * ASSUMED_OUTPUT_TOKENS / 1_000_000) * p.outputPerM;
    return { tier, model: p.model, count, estimatedCost: inputCost + outputCost };
  });

  const totalEvaluations = rows.reduce((sum, r) => sum + r.count, 0);
  const totalEstimatedCost = rows.reduce((sum, r) => sum + r.estimatedCost, 0);

  return { generatedAt: new Date().toISOString(), totalEvaluations, totalEstimatedCost, byTier: rows, assumptions: { inputTokensPerEval: ASSUMED_INPUT_TOKENS, outputTokensPerEval: ASSUMED_OUTPUT_TOKENS } };
}

function renderMarkdown(summary) {
  const lines = [];
  lines.push('# Evaluation Spend Estimate');
  lines.push('');
  lines.push(`_Generated ${summary.generatedAt.slice(0, 10)} — this is an estimate, not a bill. career-ops doesn't see real token counts; it multiplies an assumed ${summary.assumptions.inputTokensPerEval.toLocaleString()} input / ${summary.assumptions.outputTokensPerEval.toLocaleString()} output tokens per evaluation by published per-tier pricing. Check your Claude usage/billing page for actual spend._`);
  lines.push('');
  lines.push(`**Total evaluations logged:** ${summary.totalEvaluations}`);
  lines.push(`**Total estimated cost:** $${summary.totalEstimatedCost.toFixed(2)}`);
  lines.push('');
  lines.push('| Tier | Model | Evaluations | Est. cost |');
  lines.push('|------|-------|-------------|-----------|');
  for (const r of summary.byTier) {
    lines.push(`| ${r.tier} | ${r.model} | ${r.count} | $${r.estimatedCost.toFixed(2)} |`);
  }
  lines.push('');
  return lines.join('\n');
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  if (isAdd) {
    const result = addEntry();
    console.log(JSON.stringify(result, null, 2));
  } else {
    const summary = summarize();
    if (summaryMode) {
      mkdirSync(dirname(SUMMARY_FILE), { recursive: true });
      const md = renderMarkdown(summary);
      writeFileSync(SUMMARY_FILE, md);
      console.log(md);
      console.log(`\n(written to ${SUMMARY_FILE.replace(CAREER_OPS + '/', '')})`);
    } else {
      console.log(JSON.stringify(summary, null, 2));
    }
  }
}
