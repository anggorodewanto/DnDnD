// Pure navigation/state helpers for the character builder wizard.
// STEPS = ['Basics','Class','Ability Scores','Skills','Equipment','Spells','Class Features','Review']
// Two steps are conditionally skipped:
//   - Spells (index 5) is hidden for non-spellcaster classes.
//   - Class Features (index 6) is hidden unless the character has class-feature
//     choices to make (currently Warlock pact boon + Eldritch Invocations).
// Navigation is driven by a visibility context object so any number of
// skippable steps compose without bespoke per-step skip logic.

/** Index of the Spells step in the wizard. */
export const SPELLS_STEP = 5;

/** Index of the Warlock Class Features (pact boon + invocations) step. */
export const CLASS_FEATURES_STEP = 6;

/**
 * Normalizes a visibility context, null-guarding a missing ctx to "both hidden"
 * (a safe degrade — optional steps stay out of the way).
 * @param {{isCaster?:boolean, hasClassFeatures?:boolean}} ctx
 * @returns {{isCaster:boolean, hasClassFeatures:boolean}}
 */
function toCtx(ctx) {
  return {
    isCaster: !!(ctx && ctx.isCaster),
    hasClassFeatures: !!(ctx && ctx.hasClassFeatures),
  };
}

/**
 * Whether step `i` should be shown / be navigable. The Spells step is hidden
 * for non-casters; the Class Features step is hidden without class-feature
 * choices; every other step is always visible.
 * @param {number} i
 * @param {{isCaster?:boolean, hasClassFeatures?:boolean}} ctx
 * @returns {boolean}
 */
export function isStepVisible(i, ctx) {
  const c = toCtx(ctx);
  if (i === SPELLS_STEP && !c.isCaster) return false;
  if (i === CLASS_FEATURES_STEP && !c.hasClassFeatures) return false;
  return true;
}

/**
 * Next visible step index from `current`, skipping any hidden step. Never
 * exceeds total-1 (the last step — Review — is always visible).
 * @param {number} current  current step index
 * @param {number} total    STEPS.length
 * @param {{isCaster?:boolean, hasClassFeatures?:boolean}} ctx
 * @returns {number}
 */
export function nextStep(current, total, ctx) {
  const last = total - 1;
  let target = current + 1;
  while (target < last && !isStepVisible(target, ctx)) target += 1;
  return Math.min(target, last);
}

/**
 * Previous visible step index from `current`, skipping any hidden step. Never
 * goes below 0 (the first step is always visible).
 * @param {number} current
 * @param {{isCaster?:boolean, hasClassFeatures?:boolean}} ctx
 * @returns {number}
 */
export function prevStep(current, ctx) {
  let target = current - 1;
  while (target > 0 && !isStepVisible(target, ctx)) target -= 1;
  return Math.max(target, 0);
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
