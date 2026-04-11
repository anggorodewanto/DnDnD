// Phase 105c — pure helpers powering DisplayNameEditor.svelte.
//
// The component is kept as a thin wrapper so its logic can be unit-tested
// without a Svelte mount harness. EncounterBuilder and CombatManager both
// use the same component; only the `onCommit` callback differs (template PUT
// vs combat PATCH). Empty strings are preserved so the backend can clear the
// display_name override back to NULL (the encounter then falls back to its
// internal name).

/**
 * Normalize a raw editor value into its canonical form.
 * Trims whitespace and treats null/undefined/blank as "" (clear override).
 * @param {string|null|undefined} raw
 * @returns {string}
 */
export function normalizeDisplayName(raw) {
  if (raw === null || raw === undefined) return '';
  return String(raw).trim();
}

/**
 * Commit a display-name edit.
 * - Normalizes `next` and compares to the normalized `current`.
 * - If unchanged, returns { status: 'unchanged', value } without invoking onCommit.
 * - Otherwise calls onCommit(value) (may be async). On success returns
 *   { status: 'saved', value }. On throw returns { status: 'error', value, error }.
 *
 * @param {{ current: string|null, next: string, onCommit?: (v: string) => any }} opts
 * @returns {Promise<{ status: 'saved' | 'unchanged' | 'error', value: string, error?: string }>}
 */
export async function commitDisplayName({ current, next, onCommit }) {
  const normalized = normalizeDisplayName(next);
  const currentNormalized = normalizeDisplayName(current);

  if (normalized === currentNormalized) {
    return { status: 'unchanged', value: normalized };
  }

  if (!onCommit) {
    return { status: 'saved', value: normalized };
  }

  try {
    await onCommit(normalized);
    return { status: 'saved', value: normalized };
  } catch (err) {
    return { status: 'error', value: normalized, error: err?.message || String(err) };
  }
}
