// @ts-check
/** @typedef {import('./_types.js').Provider} Provider */

// Eightfold AI provider — powers the "PCS"/"PCSX" career sites of many large
// enterprises (Netflix, Nvidia, Cisco, Booking, Salesforce, and others), either
// on a *.eightfold.ai subdomain or proxied behind a branded custom domain
// (Netflix: explore.jobs.netflix.net). Two zero-auth JSON search endpoints
// exist per tenant — a given tenant supports one or the other, not always both:
//
//   GET {origin}/api/pcsx/search?domain={company}&query=&location=&start=N&sort_by=timestamp
//   GET {origin}/api/apply/v2/jobs?domain={company}&query=&location=&start=N&sort_by=timestamp
//
// Both return 10 positions per page (server-fixed) plus a total count, but in
// different envelopes: PCSX wraps the payload in {"data": {...}}, SmartApply
// (`/api/apply/v2/jobs`) returns "positions"/"count" at the top level alongside
// a large block of unrelated page-config JSON. `start` is a 0-based offset,
// stepping by 10.
//
// A tenant that doesn't have PCSX enabled 403s with a body of
// `{"message": "PCSX is not enabled for this user."}` — that specific
// status+body pair is the ONLY condition that triggers a fallback from PCSX to
// SmartApply, decided once from the first page before pagination starts (never
// per-page). Any other error just propagates, same as every other provider —
// scan.mjs's per-portal try/catch means a thrown error skips this one company
// without aborting the run.
//
// Verified live against Netflix's tenant (2026-07-14): PCSX 403s with the
// message above; SmartApply returns 200 with real postings on both
// netflix.eightfold.ai and the branded explore.jobs.netflix.net host directly.
//
// `domain` (Eightfold's internal per-tenant company-domain key, e.g.
// "netflix.com") cannot be reliably derived from the tenant subdomain or
// careers_url, so it must be configured explicitly via an `eightfold:` block:
//   eightfold:
//     domain: netflix.com
//
// Detection: branded hosts carry no "eightfold" token, so detect() only
// auto-claims literal *.eightfold.ai URLs; branded tenants are wired with an
// explicit `provider: eightfold` (which bypasses detect()) — same convention
// as providers/phenom.mjs for branded Phenom tenants.
//
// t_create/t_update are epoch SECONDS (confirmed live), unlike most providers'
// millisecond timestamps — parseEightfoldTimestamp() scales accordingly.
//
// No per-job description request is made (Eightfold's `job_description` field
// on the list payload is empty; hydrating it requires an extra per-job
// `/api/pcsx/position_details` call) — consistent with this scanner's
// zero-token design, which already omits description for every provider
// except Lever (whose list payload includes it for free).

const PAGE_SIZE = 10; // Eightfold's server-fixed page size (confirmed live).
const DEFAULT_MAX_PAGES = 150; // 150*10 = 1500 postings; override via entry.max_pages
const HARD_MAX_PAGES = 300;
const MAX_JOBS = 1500;
// Polite pacing between page requests. Eightfold's documented (unofficial)
// rate limit is ~100 requests/minute; 200ms keeps a single tenant's pagination
// well under that even for a large board.
const PAGE_DELAY_MS = 200;

/** @param {import('./_types.js').PortalEntry} entry */
export function resolveConfig(entry) {
  const raw = entry.api || entry.careers_url || '';
  let u;
  try {
    u = new URL(raw);
  } catch {
    return null;
  }
  if (u.protocol !== 'https:' && u.protocol !== 'http:') return null;
  const block = entry.eightfold && typeof entry.eightfold === 'object' ? entry.eightfold : {};
  const domain = typeof block.domain === 'string' ? block.domain.trim() : '';
  if (!domain) return null;
  const origin = u.origin;
  return {
    origin,
    domain,
    pcsxApi: `${origin}/api/pcsx/search`,
    smartApplyApi: `${origin}/api/apply/v2/jobs`,
  };
}

/** Resolve the page cap: positive integer `max_pages`, else default. */
function resolveMaxPages(entry) {
  const v = entry?.max_pages;
  if (Number.isInteger(v) && v > 0) return Math.min(v, HARD_MAX_PAGES);
  return DEFAULT_MAX_PAGES;
}

// Eightfold timestamps observed live (t_create/t_update) are epoch seconds.
// Guard against a future deployment already sending epoch ms (13 digits) by
// only scaling values that look like seconds.
/** @param {unknown} raw @returns {number | undefined} */
export function parseEightfoldTimestamp(raw) {
  const n = Number(raw);
  if (!Number.isFinite(n) || n <= 0) return undefined;
  return n < 10_000_000_000 ? n * 1000 : n;
}

/**
 * True only for the exact "PCSX not enabled for this tenant" signal — the
 * one condition that should switch the whole fetch over to SmartApply.
 * @param {any} err
 */
export function isPcsxDisabled(err) {
  return err?.status === 403 && typeof err.body === 'string' && err.body.includes('PCSX is not enabled');
}

/**
 * Normalize one PCSX/SmartApply response envelope into {total, rows}, where
 * `rows` are the raw position objects (mapping happens in mapPosition so it
 * stays independently testable, mirroring providers/phenom.mjs's split).
 * @param {any} json @param {'pcsx'|'smartapply'} variant
 */
export function parseSearchResponse(json, variant) {
  const body = variant === 'pcsx' ? (json?.data ?? {}) : (json ?? {});
  const total = typeof body.count === 'number' ? body.count : null;
  const rows = Array.isArray(body.positions) ? body.positions : [];
  return { total, rows };
}

/**
 * Map one raw Eightfold position object to the normalized row shape.
 * A record without a resolvable id or title is dropped (no stable dedup key
 * / no meaningful listing) — same guard phenom.mjs applies.
 * @param {any} item @param {{origin:string}} cfg
 */
export function mapPosition(item, cfg) {
  if (!item || typeof item !== 'object') return null;
  const id = String(item.display_job_id || item.displayJobId || item.id || item.ats_job_id || item.atsJobId || '');
  const title = String(item.name || item.posting_name || item.title || '').replace(/\s+/g, ' ').trim();
  if (!id || !title) return null;
  const rawUrl = String(item.canonicalPositionUrl || item.canonical_position_url || item.positionUrl || item.position_url || '');
  const url = rawUrl
    ? (rawUrl.startsWith('/') ? `${cfg.origin}${rawUrl}` : rawUrl)
    : `${cfg.origin}/careers/job/${encodeURIComponent(id)}`;
  const location = String(item.location || '').replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim();
  const postedAt = parseEightfoldTimestamp(item.t_create ?? item.creationTs ?? item.postedTs ?? item.t_update);
  const row = { id, title, url, location };
  if (typeof postedAt === 'number') row.postedAt = postedAt;
  return row;
}

/** @type {Provider} */
export default {
  id: 'eightfold',

  detect(entry) {
    const url = entry.api || entry.careers_url || '';
    if (typeof url !== 'string') return null;
    let host;
    try {
      host = new URL(url).hostname.toLowerCase();
    } catch {
      return null;
    }
    if (host === 'eightfold.ai' || host.endsWith('.eightfold.ai')) return { url };
    return null;
  },

  async fetch(entry, ctx) {
    const cfg = resolveConfig(entry);
    if (!cfg) {
      throw new Error(
        `eightfold: cannot resolve origin/domain for ${entry.name} — set eightfold: { domain: "<company>.com" } in portals.yml`,
      );
    }

    const wait = (ms) => (ctx.sleep ? ctx.sleep(ms) : new Promise((r) => setTimeout(r, ms)));
    const maxPages = resolveMaxPages(entry);

    /** @param {'pcsx'|'smartapply'} variant @param {number} start */
    const fetchPage = async (variant, start) => {
      const apiUrl = variant === 'pcsx' ? cfg.pcsxApi : cfg.smartApplyApi;
      const qs = new URLSearchParams({ domain: cfg.domain, query: '', location: '', start: String(start), sort_by: 'timestamp' });
      const json = await ctx.fetchJson(`${apiUrl}?${qs.toString()}`, {
        redirect: 'error',
        headers: { accept: 'application/json, text/plain, */*' },
      });
      return parseSearchResponse(json, variant);
    };

    // Decide the endpoint once, from the first page, before pagination starts.
    let variant = 'pcsx';
    let first;
    try {
      first = await fetchPage('pcsx', 0);
    } catch (err) {
      if (!isPcsxDisabled(err)) throw err;
      variant = 'smartapply';
      first = await fetchPage('smartapply', 0);
    }

    const jobs = [];
    const seen = new Set();
    const collect = (rows) => {
      let fresh = 0;
      for (const item of rows) {
        const row = mapPosition(item, cfg);
        if (!row || seen.has(row.id)) continue;
        seen.add(row.id);
        fresh++;
        const job = { title: row.title, url: row.url, company: entry.name, location: row.location };
        if (typeof row.postedAt === 'number') job.postedAt = row.postedAt;
        jobs.push(job);
        if (jobs.length >= MAX_JOBS) break;
      }
      return fresh;
    };

    let total = first.total;
    collect(first.rows);

    for (let page = 1; page < maxPages; page++) {
      if (jobs.length >= MAX_JOBS) break;
      if (total !== null && page * PAGE_SIZE >= total) break;
      await wait(PAGE_DELAY_MS);
      let pageResult;
      try {
        pageResult = await fetchPage(variant, page * PAGE_SIZE);
      } catch {
        break; // keep jobs collected so far — a transient mid-scan failure shouldn't discard earlier pages
      }
      if (total === null) total = pageResult.total;
      if (pageResult.rows.length === 0) break;
      const fresh = collect(pageResult.rows);
      if (fresh === 0) break; // server ignored `start` (or we've looped)
    }

    return jobs;
  },
};
