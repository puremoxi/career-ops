// tests/providers/eightfold.test.mjs — Eightfold AI PCS/PCSX + SmartApply search API.
import { pass, fail, ROOT } from '../helpers.mjs';
import { join } from 'path';
import { pathToFileURL } from 'url';

console.log('\nProvider — eightfold (Eightfold AI PCSX / SmartApply search API)');
try {
  const efModule = await import(pathToFileURL(join(ROOT, 'providers/eightfold.mjs')).href);
  const eightfold = efModule.default;
  const { resolveConfig: efConfig, parseEightfoldTimestamp, isPcsxDisabled, parseSearchResponse, mapPosition } = efModule;

  if (eightfold.id === 'eightfold') pass('eightfold.id is "eightfold"');
  else fail(`eightfold.id is ${JSON.stringify(eightfold.id)}`);

  // resolveConfig — requires an eightfold.domain block; derives origin + both API URLs.
  const efCfg = efConfig({ careers_url: 'https://explore.jobs.netflix.net/careers', eightfold: { domain: 'netflix.com' } });
  if (
    efCfg && efCfg.origin === 'https://explore.jobs.netflix.net' && efCfg.domain === 'netflix.com' &&
    efCfg.pcsxApi === 'https://explore.jobs.netflix.net/api/pcsx/search' &&
    efCfg.smartApplyApi === 'https://explore.jobs.netflix.net/api/apply/v2/jobs'
  ) {
    pass('eightfold.resolveConfig() derives origin and both API URLs from careers_url + eightfold.domain');
  } else {
    fail(`eightfold.resolveConfig() wrong: ${JSON.stringify(efCfg)}`);
  }
  if (efConfig({ careers_url: 'https://explore.jobs.netflix.net/careers' }) === null) {
    pass('eightfold.resolveConfig() returns null when eightfold.domain is missing');
  } else {
    fail('eightfold.resolveConfig() should require eightfold.domain');
  }
  if (efConfig({ careers_url: 'not a url' }) === null) pass('eightfold.resolveConfig() returns null for an invalid URL');
  else fail('eightfold.resolveConfig() should reject an invalid URL');

  // detect — hostname-anchored: only literal *.eightfold.ai. Branded tenants
  // (like Netflix) carry no "eightfold" token and must wire provider: eightfold
  // explicitly.
  if (eightfold.detect({ careers_url: 'https://netflix.eightfold.ai/careers' })) pass('eightfold.detect() matches *.eightfold.ai');
  else fail('eightfold.detect() should match eightfold.ai');
  if (eightfold.detect({ careers_url: 'https://explore.jobs.netflix.net/careers' }) === null) {
    pass('eightfold.detect() returns null for a branded host (wire explicitly)');
  } else {
    fail('eightfold.detect() should not auto-claim a branded host');
  }
  if (eightfold.detect({ careers_url: 'https://evil.com/x?y=eightfold.ai' }) === null) {
    pass('eightfold.detect() rejects a host that merely contains "eightfold.ai" in path/query');
  } else {
    fail('eightfold.detect() should check the hostname, not the raw URL string');
  }
  if (eightfold.detect({ careers_url: 'https://eightfold.ai.evil.com/x' }) === null) {
    pass('eightfold.detect() rejects suffix-spoofed host');
  } else {
    fail('eightfold.detect() should reject suffix-spoofed host');
  }

  // isPcsxDisabled — matches only the specific 403 + body signal.
  if (isPcsxDisabled({ status: 403, body: '{"message": "PCSX is not enabled for this user."}' })) {
    pass('eightfold.isPcsxDisabled() matches the 403 + PCSX-disabled body');
  } else {
    fail('eightfold.isPcsxDisabled() should match the documented 403 signal');
  }
  if (!isPcsxDisabled({ status: 403, body: 'some other forbidden message' })) {
    pass('eightfold.isPcsxDisabled() rejects a differently-worded 403');
  } else {
    fail('eightfold.isPcsxDisabled() should not match an unrelated 403 body');
  }
  if (!isPcsxDisabled({ status: 500, body: 'PCSX is not enabled for this user.' })) {
    pass('eightfold.isPcsxDisabled() rejects a non-403 status even with a matching body');
  } else {
    fail('eightfold.isPcsxDisabled() should require status 403');
  }

  // parseEightfoldTimestamp — scales epoch seconds to ms, passes through ms, rejects junk.
  if (parseEightfoldTimestamp(1783641600) === 1783641600000) pass('eightfold.parseEightfoldTimestamp() scales epoch seconds to ms');
  else fail(`eightfold.parseEightfoldTimestamp() seconds wrong: ${parseEightfoldTimestamp(1783641600)}`);
  if (parseEightfoldTimestamp(1783641600000) === 1783641600000) pass('eightfold.parseEightfoldTimestamp() passes through an already-ms value');
  else fail(`eightfold.parseEightfoldTimestamp() ms wrong: ${parseEightfoldTimestamp(1783641600000)}`);
  if (parseEightfoldTimestamp(0) === undefined && parseEightfoldTimestamp('') === undefined && parseEightfoldTimestamp(null) === undefined) {
    pass('eightfold.parseEightfoldTimestamp() rejects zero/empty/null');
  } else {
    fail('eightfold.parseEightfoldTimestamp() should reject non-positive/non-numeric input');
  }

  // parseSearchResponse — PCSX wraps in {data:{...}}, SmartApply is top-level.
  const pcsxJson = { data: { count: 42, positions: [{ id: 1 }] } };
  const { total: pcsxTotal, rows: pcsxRows } = parseSearchResponse(pcsxJson, 'pcsx');
  if (pcsxTotal === 42 && pcsxRows.length === 1) pass('eightfold.parseSearchResponse() reads the PCSX {data:{...}} envelope');
  else fail(`eightfold.parseSearchResponse() pcsx wrong: total=${pcsxTotal} rows=${pcsxRows.length}`);
  const smartJson = { count: 89, positions: [{ id: 1 }, { id: 2 }] };
  const { total: smartTotal, rows: smartRows } = parseSearchResponse(smartJson, 'smartapply');
  if (smartTotal === 89 && smartRows.length === 2) pass('eightfold.parseSearchResponse() reads the SmartApply top-level envelope');
  else fail(`eightfold.parseSearchResponse() smartapply wrong: total=${smartTotal} rows=${smartRows.length}`);
  const emptyEnvelope = parseSearchResponse({}, 'smartapply');
  if (emptyEnvelope.total === null && emptyEnvelope.rows.length === 0) pass('eightfold.parseSearchResponse() handles a missing/empty envelope');
  else fail('eightfold.parseSearchResponse() should default total to null and rows to []');

  // mapPosition — id/title required; URL from canonicalPositionUrl (absolute or
  // origin-relative) or a synthesized fallback; live-shaped sample fields.
  const cfg = { origin: 'https://explore.jobs.netflix.net' };
  const liveShaped = mapPosition({
    id: 790317054439, name: 'Creative Director, Creative Publishing', location: 'USA - Remote',
    department: 'Netflix Games Studio', canonicalPositionUrl: 'https://explore.jobs.netflix.net/careers/job/790317054439',
    t_create: 1783641600, t_update: 1783641600,
  }, cfg);
  if (
    liveShaped && liveShaped.id === '790317054439' && liveShaped.title === 'Creative Director, Creative Publishing' &&
    liveShaped.url === 'https://explore.jobs.netflix.net/careers/job/790317054439' && liveShaped.location === 'USA - Remote' &&
    liveShaped.postedAt === 1783641600000
  ) {
    pass('eightfold.mapPosition() parses a live-shaped Netflix position');
  } else {
    fail(`eightfold.mapPosition() live-shaped wrong: ${JSON.stringify(liveShaped)}`);
  }
  const relativeUrl = mapPosition({ id: 5, name: 'Relative', canonicalPositionUrl: '/careers/job/5' }, cfg);
  if (relativeUrl?.url === 'https://explore.jobs.netflix.net/careers/job/5') pass('eightfold.mapPosition() resolves an origin-relative canonicalPositionUrl');
  else fail(`eightfold.mapPosition() relative url wrong: ${JSON.stringify(relativeUrl)}`);
  const fallbackUrl = mapPosition({ id: 6, name: 'No URL' }, cfg);
  if (fallbackUrl?.url === 'https://explore.jobs.netflix.net/careers/job/6') pass('eightfold.mapPosition() synthesizes a URL when canonicalPositionUrl is absent');
  else fail(`eightfold.mapPosition() fallback url wrong: ${JSON.stringify(fallbackUrl)}`);
  if (mapPosition({ id: '', name: 'No id' }, cfg) === null) pass('eightfold.mapPosition() drops a record with no id');
  else fail('eightfold.mapPosition() should drop an id-less record');
  if (mapPosition({ id: 7, name: '' }, cfg) === null) pass('eightfold.mapPosition() drops a record with no title');
  else fail('eightfold.mapPosition() should drop a title-less record');

  // fetch — PCSX path: paginates by start/10 until count, dedups, tags company.
  const mkItem = (id) => ({ id, name: `Job ${id}`, location: 'Remote', canonicalPositionUrl: `/careers/job/${id}` });
  const pcsxPage = (ids, count) => ({ data: { count, positions: ids.map(mkItem) } });
  let pcsxCalls = 0;
  const pcsxCtx = {
    sleep: async () => {},
    fetchJson: async (url) => {
      const u = new URL(url);
      if (!u.pathname.endsWith('/api/pcsx/search')) throw new Error(`unexpected endpoint: ${u.pathname}`);
      const start = Number(u.searchParams.get('start'));
      pcsxCalls++;
      if (start === 0) return pcsxPage([1, 2, 3, 4, 5, 6, 7, 8, 9, 10], 13);
      if (start === 10) return pcsxPage([11, 12, 13], 13);
      return pcsxPage([], 13);
    },
  };
  const pcsxJobs = await eightfold.fetch({ name: 'Netflix', careers_url: 'https://explore.jobs.netflix.net/careers', eightfold: { domain: 'netflix.com' } }, pcsxCtx);
  if (pcsxJobs.length === 13 && pcsxCalls === 2 && pcsxJobs.every((j) => j.company === 'Netflix') && new Set(pcsxJobs.map((j) => j.url)).size === 13) {
    pass('eightfold.fetch() paginates the PCSX path by start/10, dedups, and tags company');
  } else {
    fail(`eightfold.fetch() pcsx path wrong: ${pcsxJobs.length} jobs after ${pcsxCalls} calls`);
  }

  // fetch — falls back to SmartApply on the specific "PCSX not enabled" 403,
  // deciding once from the first page (never retries PCSX per-page).
  let sawPcsxCall = false;
  let smartCalls = 0;
  const fallbackCtx = {
    sleep: async () => {},
    fetchJson: async (url) => {
      const u = new URL(url);
      if (u.pathname.endsWith('/api/pcsx/search')) {
        sawPcsxCall = true;
        const err = new Error('HTTP 403');
        err.status = 403;
        err.body = '{"message": "PCSX is not enabled for this user."}';
        throw err;
      }
      smartCalls++;
      const start = Number(u.searchParams.get('start'));
      if (start === 0) return { count: 12, positions: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map(mkItem) };
      return { count: 12, positions: [11, 12].map(mkItem) };
    },
  };
  const fallbackJobs = await eightfold.fetch({ name: 'Netflix', careers_url: 'https://explore.jobs.netflix.net/careers', eightfold: { domain: 'netflix.com' } }, fallbackCtx);
  if (sawPcsxCall && fallbackJobs.length === 12 && smartCalls === 2) {
    pass('eightfold.fetch() falls back to SmartApply on the PCSX-disabled 403 and paginates it');
  } else {
    fail(`eightfold.fetch() fallback wrong: sawPcsx=${sawPcsxCall} jobs=${fallbackJobs.length} smartCalls=${smartCalls}`);
  }

  // fetch — an unrelated error on the first page propagates (not swallowed
  // into a silent empty result) so scan.mjs's per-portal catch can log it.
  const hardFailCtx = { sleep: async () => {}, fetchJson: async () => { const err = new Error('HTTP 500'); err.status = 500; throw err; } };
  try {
    await eightfold.fetch({ name: 'Netflix', careers_url: 'https://explore.jobs.netflix.net/careers', eightfold: { domain: 'netflix.com' } }, hardFailCtx);
    fail('eightfold.fetch() should propagate a non-PCSX-disabled error on the first page');
  } catch (e) {
    if (e.status === 500) pass('eightfold.fetch() propagates an unrelated first-page error instead of silently falling back');
    else fail(`eightfold.fetch() wrong error propagated: ${e.message}`);
  }

  // fetch — a mid-scan failure on a later page preserves jobs already collected.
  let partialCalls = 0;
  const partialCtx = {
    sleep: async () => {},
    fetchJson: async (url) => {
      const u = new URL(url);
      const start = Number(u.searchParams.get('start'));
      partialCalls++;
      if (start === 0) return pcsxPage([1, 2, 3, 4, 5, 6, 7, 8, 9, 10], 25);
      throw new Error('network blip on page 2');
    },
  };
  const partialJobs = await eightfold.fetch({ name: 'Netflix', careers_url: 'https://explore.jobs.netflix.net/careers', eightfold: { domain: 'netflix.com' } }, partialCtx);
  if (partialJobs.length === 10 && partialCalls === 2) {
    pass('eightfold.fetch() preserves jobs from earlier pages when a later page fetch throws');
  } else {
    fail(`eightfold.fetch() partial-failure handling wrong: ${partialJobs.length} jobs after ${partialCalls} calls`);
  }

  // fetch — throws a clear error when eightfold.domain is missing.
  try {
    await eightfold.fetch({ name: 'NoDomain', careers_url: 'https://netflix.eightfold.ai/careers' }, { sleep: async () => {}, fetchJson: async () => ({}) });
    fail('eightfold.fetch() should throw when eightfold.domain is missing');
  } catch (e) {
    if (/eightfold:.*domain/.test(e.message)) pass('eightfold.fetch() throws a clear error when eightfold.domain is missing');
    else fail(`eightfold.fetch() wrong error for missing domain: ${e.message}`);
  }
} catch (e) {
  fail(`eightfold provider tests crashed: ${e.message}`);
}
