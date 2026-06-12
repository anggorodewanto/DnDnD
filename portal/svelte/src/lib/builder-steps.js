// Pure navigation/state helpers for the 7-step character builder wizard.
// STEPS = ['Basics','Class','Ability Scores','Skills','Equipment','Spells','Review']
// Non-spellcaster classes skip the Spells step (index 5) entirely.

/** Index of the Spells step in the wizard. */
export const SPELLS_STEP = 5;

/**
 * Next step index from `current`, skipping the Spells step when the class is
 * not a spellcaster. Never exceeds total-1.
 * @param {number} current  current step index
 * @param {number} total    STEPS.length (7)
 * @param {boolean} isCaster whether the selected class can cast spells
 * @returns {number}
 */
export function nextStep(current, total, isCaster) {
  const last = total - 1;
  let target = current + 1;
  if (target === SPELLS_STEP && !isCaster) target += 1;
  if (target > last) return last;
  return target;
}

/**
 * Previous step index from `current`, skipping the Spells step when the class
 * is not a spellcaster. Never goes below 0.
 * @param {number} current
 * @param {boolean} isCaster
 * @returns {number}
 */
export function prevStep(current, isCaster) {
  let target = current - 1;
  if (target === SPELLS_STEP && !isCaster) target -= 1;
  if (target < 0) return 0;
  return target;
}

/**
 * Whether step `i` should be shown / be navigable. The Spells step is hidden
 * for non-casters; all other steps are always visible.
 * @param {number} i
 * @param {boolean} isCaster
 * @returns {boolean}
 */
export function isStepVisible(i, isCaster) {
  if (i === SPELLS_STEP && !isCaster) return false;
  return true;
}

/**
 * Classify the Spells step content.
 * @param {{ isCaster: boolean, loading: boolean, error: (string|boolean), count: number }} s
 * @returns {'not-caster'|'loading'|'error'|'empty'|'ready'}
 */
export function spellStepState(s) {
  if (!s.isCaster) return 'not-caster';
  if (s.error) return 'error';
  if (s.loading) return 'loading';
  if (s.count === 0) return 'empty';
  return 'ready';
}
