/**
 * Combat logic utilities for the Combat Manager.
 */

/**
 * Standard 5e conditions.
 */
export const STANDARD_CONDITIONS = [
  'Blinded', 'Charmed', 'Deafened', 'Frightened', 'Grappled',
  'Incapacitated', 'Invisible', 'Paralyzed', 'Petrified', 'Poisoned',
  'Prone', 'Restrained', 'Stunned', 'Unconscious',
];

/**
 * Apply damage to a combatant, absorbing temp HP first.
 * Returns new { hp_current, temp_hp, is_alive }.
 */
export function applyDamage(combatant, amount) {
  if (amount <= 0) {
    return {
      hp_current: combatant.hp_current,
      temp_hp: combatant.temp_hp,
      is_alive: combatant.hp_current > 0,
    };
  }

  let remaining = amount;
  let tempHp = combatant.temp_hp || 0;
  let hpCurrent = combatant.hp_current;

  // Absorb from temp HP first
  if (tempHp > 0) {
    const absorbed = Math.min(tempHp, remaining);
    tempHp -= absorbed;
    remaining -= absorbed;
  }

  // Then reduce current HP
  hpCurrent = Math.max(0, hpCurrent - remaining);

  return {
    hp_current: hpCurrent,
    temp_hp: tempHp,
    is_alive: hpCurrent > 0,
  };
}

/**
 * Apply healing to a combatant, capped at hp_max.
 * Returns new { hp_current, is_alive }.
 */
export function applyHealing(combatant, amount) {
  if (amount <= 0) {
    return {
      hp_current: combatant.hp_current,
      is_alive: combatant.hp_current > 0,
    };
  }

  const hpCurrent = Math.min(combatant.hp_max, combatant.hp_current + amount);
  return {
    hp_current: hpCurrent,
    is_alive: hpCurrent > 0,
  };
}

/**
 * Returns a health tier string based on HP percentage.
 * Used for token color coding.
 */
export function healthTier(hpCurrent, hpMax) {
  if (hpMax <= 0) return 'dead';
  if (hpCurrent <= 0) return 'dead';

  const pct = hpCurrent / hpMax;
  if (pct > 0.75) return 'healthy';
  if (pct > 0.5) return 'wounded';
  if (pct > 0.25) return 'bloodied';
  return 'critical';
}

/**
 * Add a condition to a conditions array (no duplicates).
 * Returns new array.
 */
export function addCondition(conditions, condition) {
  if (conditions.includes(condition)) return conditions;
  return [...conditions, condition];
}

/**
 * Remove a condition from a conditions array.
 * Returns new array.
 */
export function removeCondition(conditions, condition) {
  return conditions.filter(c => c !== condition);
}

/**
 * Returns the opacity for a combatant token on the DM map.
 * Invisible combatants are rendered with reduced opacity.
 */
export function tokenOpacity(combatant) {
  if (combatant.is_visible === false) return 0.4;
  return 1.0;
}

/**
 * Convert a column letter (A-Z, AA, etc.) to a 0-based index.
 */
export function colToIndex(col) {
  if (!col) return 0;
  let result = 0;
  for (let i = 0; i < col.length; i++) {
    result = result * 26 + (col.charCodeAt(i) - 64);
  }
  return result - 1;
}
