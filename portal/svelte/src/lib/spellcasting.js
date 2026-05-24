/**
 * Client-side spellcasting helpers. The ability map mirrors the Go
 * `classSpellcasting` table in internal/portal/derive_stats.go so the builder
 * can compute the prepared-spell cap (ability modifier + class level) for live
 * feedback. The authoritative castable spell *level* still comes from the
 * server preview (DerivedStats.max_spell_level); this file only knows which
 * ability a class casts with and the cap formula.
 */

/** @type {Record<string,string>} class slug -> spellcasting ability */
const CASTER_ABILITY = {
  bard: 'cha',
  cleric: 'wis',
  druid: 'wis',
  sorcerer: 'cha',
  wizard: 'int',
  paladin: 'cha',
  ranger: 'wis',
  warlock: 'cha',
};

/**
 * Returns the spellcasting ability slug ('int'|'wis'|'cha') for a class, or
 * null when the class is not a spellcaster.
 * @param {string} className
 * @returns {string|null}
 */
export function spellcastingAbilityForClass(className) {
  if (!className) return null;
  return CASTER_ABILITY[String(className).toLowerCase()] ?? null;
}

/**
 * Whether a class is a spellcaster.
 * @param {string} className
 * @returns {boolean}
 */
export function isSpellcaster(className) {
  return spellcastingAbilityForClass(className) !== null;
}

/**
 * Prepared-spell cap: ability modifier + class level, minimum 1. Mirrors
 * combat.MaxPreparedSpells on the Go side.
 * @param {number} abilityMod
 * @param {number} level
 * @returns {number}
 */
export function spellPrepCap(abilityMod, level) {
  const cap = (Number(abilityMod) || 0) + (Number(level) || 0);
  return cap < 1 ? 1 : cap;
}

/**
 * Returns [1..maxLevel] (the selectable leveled-spell range), or [] when the
 * character has no leveled slots yet. Cantrips (level 0) are always selectable
 * and handled separately by the picker.
 * @param {number} maxLevel
 * @returns {number[]}
 */
export function levelsUpTo(maxLevel) {
  const m = Number(maxLevel) || 0;
  if (m < 1) return [];
  return Array.from({ length: m }, (_, i) => i + 1);
}
