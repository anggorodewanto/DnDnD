// Pure, framework-free helpers for the Warlock Class Features step (pact boon +
// Eldritch Invocations). The rules DATA lives in invocations-catalog.json, which
// is generated from the Go SSOT (internal/refdata.PactBoonCatalog +
// InvocationCatalog) — `make invocations-catalog-check` guards drift. This
// module reimplements the grant table + prerequisite gates in JS, exactly
// mirroring internal/portal/invocations.go so the picker and the backend agree
// (the same split as expertise: skill-selection.js ⇄ expertise.go).
//
// All functions are non-mutating and side-effect-free.

import catalog from './invocations-catalog.json';

/** Spell id of the eldritch blast cantrip (prereq for the *_blast invocations). */
const ELDRITCH_BLAST = 'eldritch-blast';

/** @returns {Array<{id:string,name:string,description:string}>} the pact boons. */
export function pactBoonList() {
  return catalog.pact_boons;
}

/** @returns {Array<object>} the full invocation catalog. */
export function invocationList() {
  return catalog.invocations;
}

/**
 * Number of Eldritch Invocations a warlock knows at the given warlock class
 * level (2014 PHB table). Mirrors refdata.InvocationsKnown.
 * @param {number} warlockLevel
 * @returns {number}
 */
export function invocationsKnown(warlockLevel) {
  if (warlockLevel < 2) return 0;
  if (warlockLevel < 5) return 2;
  if (warlockLevel < 7) return 3;
  if (warlockLevel < 9) return 4;
  if (warlockLevel < 12) return 5;
  if (warlockLevel < 15) return 6;
  if (warlockLevel < 18) return 7;
  return 8;
}

/**
 * Whether the warlock has a Pact Boon choice (2014 PHB: level 3).
 * Mirrors refdata.PactBoonGranted.
 * @param {number} warlockLevel
 * @returns {boolean}
 */
export function pactBoonGranted(warlockLevel) {
  return warlockLevel >= 3;
}

/**
 * The warlock class level within a multiclass entry list (0 if none).
 * @param {Array<{class?:string, level?:number}>} classEntries
 * @returns {number}
 */
export function warlockLevelOf(classEntries) {
  for (const c of classEntries || []) {
    if ((c.class || '').toLowerCase() === 'warlock') return Number(c.level) || 0;
  }
  return 0;
}

/**
 * Whether the character has any Warlock class-feature choices to make, i.e.
 * whether the Class Features step should be shown. True from warlock level 2
 * (first invocations) onward.
 * @param {Array<{class?:string, level?:number}>} classEntries
 * @returns {boolean}
 */
export function hasClassFeatureChoices(classEntries) {
  const wl = warlockLevelOf(classEntries);
  return invocationsKnown(wl) > 0 || pactBoonGranted(wl);
}

/**
 * Evaluates one invocation's prerequisites. Mirrors checkInvocationPrereq in
 * invocations.go.
 * @param {object} inv - a catalog invocation
 * @param {{warlockLevel:number, pactBoon:string, knownSpells:string[]}} ctx
 * @returns {{ok:boolean, reason:string}}
 */
function prereqMet(inv, { warlockLevel, pactBoon, knownSpells }) {
  const p = inv.prereq || {};
  if (p.min_warlock_level && warlockLevel < p.min_warlock_level) {
    return { ok: false, reason: `Requires warlock level ${p.min_warlock_level}` };
  }
  if (p.requires_pact_boon && pactBoon !== p.requires_pact_boon) {
    const boon = catalog.pact_boons.find((b) => b.id === p.requires_pact_boon);
    return { ok: false, reason: `Requires ${boon ? boon.name : p.requires_pact_boon}` };
  }
  if (p.requires_eldritch_blast && !(knownSpells || []).includes(ELDRITCH_BLAST)) {
    return { ok: false, reason: 'Requires the Eldritch Blast cantrip' };
  }
  return { ok: true, reason: '' };
}

/**
 * Per-invocation UI state plus a budget summary, mirroring computeExpertiseState.
 * Each entry is `{id, name, description, checked, disabled, available, reason}`.
 * An unchosen box is disabled once the grant is spent OR its prereq is unmet; a
 * chosen box stays enabled so it can be toggled off.
 * @param {{warlockLevel:number, pactBoon:string, knownSpells:string[], selected:string[]}} args
 * @returns {{options:Array<object>, max:number, chosen:number}}
 */
export function computeInvocationState({ warlockLevel, pactBoon, knownSpells, selected }) {
  const max = invocationsKnown(warlockLevel);
  const selectedSet = new Set(selected || []);
  const chosen = catalog.invocations.filter((inv) => selectedSet.has(inv.id)).length;
  const remaining = Math.max(0, max - chosen);

  const options = catalog.invocations.map((inv) => {
    const checked = selectedSet.has(inv.id);
    const { ok, reason } = prereqMet(inv, { warlockLevel, pactBoon, knownSpells });
    return {
      id: inv.id,
      name: inv.name,
      description: inv.description,
      edition: inv.edition || '', // '' = both PHBs; '2014'/'2024' badge the picker
      checked,
      available: ok,
      reason: ok ? '' : reason,
      disabled: !checked && (!ok || remaining === 0),
    };
  });

  return { options, max, chosen };
}

/**
 * Reduces an arbitrary invocation list to a legal set: keeps only real,
 * prereq-met, de-duplicated ids (in input order), capped at the warlock-level
 * grant. Mirrors classFeatureFeaturesForSubmission's safety-net filter.
 * Idempotent.
 * @param {{warlockLevel:number, pactBoon:string, knownSpells:string[], selected:string[]}} args
 * @returns {string[]}
 */
export function reconcileInvocations({ warlockLevel, pactBoon, knownSpells, selected }) {
  const max = invocationsKnown(warlockLevel);
  if (max === 0) return [];

  const byId = new Map(catalog.invocations.map((i) => [i.id, i]));
  const seen = new Set();
  const kept = [];
  for (const id of selected || []) {
    if (kept.length >= max) break;
    const inv = byId.get(id);
    if (!inv || seen.has(id)) continue;
    if (!prereqMet(inv, { warlockLevel, pactBoon, knownSpells }).ok) continue;
    seen.add(id);
    kept.push(id);
  }
  return kept;
}

/**
 * Reduces a pact-boon pick to a legal value: cleared below warlock level 3 or
 * when the id is unknown. Mirrors validateSubmittedClassFeatures's boon gate.
 * @param {{warlockLevel:number, pactBoon:string}} args
 * @returns {string}
 */
export function reconcilePactBoon({ warlockLevel, pactBoon }) {
  if (!pactBoon || !pactBoonGranted(warlockLevel)) return '';
  return catalog.pact_boons.some((b) => b.id === pactBoon) ? pactBoon : '';
}
