/**
 * Phase 103 — Optimistic UI merge helper.
 *
 * When a WebSocket snapshot arrives while the DM has a form open, we must
 * update read-only display areas immediately but preserve any input the DM
 * is currently editing. This helper is a pure function that accepts the
 * current view-model state, the incoming snapshot, and a set of "dirty"
 * field names. It returns a new state where every field is taken from the
 * snapshot EXCEPT for dirty fields, which keep their current value and get
 * recorded in a sibling `_pendingFromSnapshot` object so the UI can render
 * an indicator like "HP updated to 3 by player action".
 *
 * No DOM access — trivially testable in vitest.
 *
 * @param {object|null} currentState
 * @param {object} snapshot
 * @param {Set<string>|Array<string>|null} dirtyFields
 * @returns {object} merged state with _pendingFromSnapshot metadata
 */
export function mergeSnapshot(currentState, snapshot, dirtyFields) {
  const base = currentState || {};
  const dirty = toSet(dirtyFields);
  const merged = { ...base };
  const pending = {};

  for (const key of Object.keys(snapshot)) {
    if (key === '_pendingFromSnapshot') continue;
    if (!dirty.has(key)) {
      merged[key] = snapshot[key];
      continue;
    }
    const draftValue = base[key];
    merged[key] = draftValue;
    if (!deepEqual(draftValue, snapshot[key])) {
      pending[key] = snapshot[key];
    }
  }

  merged._pendingFromSnapshot = pending;
  return merged;
}

function toSet(dirtyFields) {
  if (!dirtyFields) return new Set();
  if (dirtyFields instanceof Set) return dirtyFields;
  return new Set(dirtyFields);
}

function deepEqual(a, b) {
  if (a === b) return true;
  if (a === null || b === null) return false;
  if (typeof a !== typeof b) return false;
  if (typeof a !== 'object') return false;
  const ka = Object.keys(a);
  const kb = Object.keys(b);
  if (ka.length !== kb.length) return false;
  for (const k of ka) {
    if (!deepEqual(a[k], b[k])) return false;
  }
  return true;
}
