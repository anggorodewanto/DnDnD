/**
 * Pure diff helpers for rendering before/after state changes in the Action Log viewer.
 */

/**
 * Compare before_state and after_state objects and return only the fields that changed.
 * @param {object|null|undefined} before
 * @param {object|null|undefined} after
 * @returns {Array<{field: string, before: unknown, after: unknown}>}
 */
export function diffStates(before, after) {
  const b = before ?? {};
  const a = after ?? {};
  const keys = new Set([...Object.keys(b), ...Object.keys(a)]);
  const out = [];
  for (const key of keys) {
    if (!deepEqual(b[key], a[key])) {
      out.push({ field: key, before: b[key], after: a[key] });
    }
  }
  out.sort((x, y) => x.field.localeCompare(y.field));
  return out;
}

/**
 * Deep equality check suitable for JSON-serializable values.
 */
function deepEqual(x, y) {
  if (x === y) return true;
  if (x === null || y === null) return false;
  if (typeof x !== typeof y) return false;
  if (typeof x !== 'object') return false;
  if (Array.isArray(x) !== Array.isArray(y)) return false;
  if (Array.isArray(x)) {
    if (x.length !== y.length) return false;
    for (let i = 0; i < x.length; i++) {
      if (!deepEqual(x[i], y[i])) return false;
    }
    return true;
  }
  const kx = Object.keys(x);
  const ky = Object.keys(y);
  if (kx.length !== ky.length) return false;
  for (const k of kx) {
    if (!deepEqual(x[k], y[k])) return false;
  }
  return true;
}
