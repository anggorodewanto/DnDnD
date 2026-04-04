/**
 * D&D 5e Point Buy calculator.
 * 27 points total. All abilities start at 8.
 * Cost: 8-13 = 1pt each step, 13->14 = 2pts, 14->15 = 2pts.
 */

const POINT_BUY_TOTAL = 27;
const MIN_SCORE = 8;
const MAX_SCORE = 15;

/**
 * Returns the point cost for a single ability score.
 * @param {number} score - Ability score (8-15)
 * @returns {number} Cost in points
 */
export function scoreCost(score) {
  if (score < MIN_SCORE || score > MAX_SCORE) return 0;
  if (score <= 13) return score - 8;
  // 14 costs 7, 15 costs 9
  return 5 + (score - 13) * 2;
}

/**
 * Returns the total points spent for a set of scores.
 * @param {object} scores - { str, dex, con, int, wis, cha }
 * @returns {number}
 */
export function totalCost(scores) {
  return Object.values(scores).reduce((sum, v) => sum + scoreCost(v), 0);
}

/**
 * Returns the remaining points available.
 * @param {object} scores
 * @returns {number}
 */
export function remainingPoints(scores) {
  return POINT_BUY_TOTAL - totalCost(scores);
}

/**
 * Returns the ability modifier for a score.
 * @param {number} score
 * @returns {number}
 */
export function abilityModifier(score) {
  return Math.floor((score - 10) / 2);
}

/**
 * Checks if incrementing a score is allowed.
 * @param {object} scores
 * @param {string} ability
 * @returns {boolean}
 */
export function canIncrement(scores, ability) {
  const current = scores[ability];
  if (current >= MAX_SCORE) return false;
  const next = { ...scores, [ability]: current + 1 };
  return totalCost(next) <= POINT_BUY_TOTAL;
}

/**
 * Checks if decrementing a score is allowed.
 * @param {object} scores
 * @param {string} ability
 * @returns {boolean}
 */
export function canDecrement(scores, ability) {
  return scores[ability] > MIN_SCORE;
}

export { POINT_BUY_TOTAL, MIN_SCORE, MAX_SCORE };
