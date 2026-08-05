// @ts-check
// Shared work-model (remote/hybrid/onsite) normalization.
//
// Two entry points, matching the two ways providers learn this:
// - normalizeStructuredWorkModel(): the ATS gave us an explicit field
//   (Lever's categories.workplaceType, Ashby's workplaceType) — canonicalize
//   its casing/spelling. Callers tag workModelSource: 'structured'.
// - classifyWorkModelFromLocation(): no structured field exists (Greenhouse's
//   public boards-api has none), so this is a best-effort read of the
//   free-text location string. Callers tag workModelSource: 'inferred' —
//   never presented as fact.
//
// Canonical values: 'remote' | 'hybrid' | 'onsite' | 'mixed' | 'unknown'.
// 'mixed' means conflicting signals were found (e.g. both "Remote" and
// "Hybrid" in the same string), not a catch-all default — no signal at all
// is 'unknown'.

/** @param {unknown} raw */
export function normalizeStructuredWorkModel(raw) {
  const v = typeof raw === 'string' ? raw.trim().toLowerCase() : '';
  if (!v) return 'unknown';
  if (v.includes('remote')) return 'remote';
  if (v.includes('hybrid')) return 'hybrid';
  if (v.includes('on-site') || v.includes('onsite') || v.includes('on site') || v.includes('office')) return 'onsite';
  return 'unknown';
}

const REMOTE_RE = /\b(remote|distributed|work[-\s]?from[-\s]?home|wfh)\b/i;
const HYBRID_RE = /\b(hybrid|flexible)\b/i;
const ONSITE_RE = /\b(on[-\s]?site|in[-\s]?office|in\s+office)\b/i;

/**
 * Best-effort classification from a free-text location string. Always
 * paired with workModelSource: 'inferred' by callers.
 * @param {unknown} locationName
 */
export function classifyWorkModelFromLocation(locationName) {
  if (typeof locationName !== 'string' || !locationName.trim()) return 'unknown';
  const isRemote = REMOTE_RE.test(locationName);
  const isHybrid = HYBRID_RE.test(locationName);
  const isOnsite = ONSITE_RE.test(locationName);
  const hits = [isRemote, isHybrid, isOnsite].filter(Boolean).length;
  if (hits === 0) return 'unknown';
  if (hits > 1) return 'mixed';
  if (isRemote) return 'remote';
  if (isHybrid) return 'hybrid';
  return 'onsite';
}
