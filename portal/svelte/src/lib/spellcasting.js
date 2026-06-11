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

/**
 * Cantrips known at levels 1–3 per cantrip-casting class. Every such class
 * gains one more cantrip at level 4 and another at level 10 (the PHB "Cantrips
 * Known" columns all step up at those two levels), so cantripsKnown() adds those
 * step-ups to the base. Classes absent here (paladin, ranger, non-casters) know
 * no cantrips. Mirrors cantripBaseKnown in internal/portal/spellbudget.go.
 * @type {Record<string,number>}
 */
const CANTRIP_BASE = { bard: 2, cleric: 3, druid: 2, sorcerer: 4, warlock: 2, wizard: 3 };

/**
 * Number of cantrips a class knows at the given class level. 0 for classes
 * that learn no cantrips.
 * @param {string} className
 * @param {number} level
 * @returns {number}
 */
export function cantripsKnown(className, level) {
  const base = CANTRIP_BASE[String(className || '').toLowerCase()];
  if (base == null) return 0;
  const lvl = Math.max(1, Number(level) || 1);
  let n = base;
  if (lvl >= 4) n += 1;
  if (lvl >= 10) n += 1;
  return n;
}

/**
 * Per-level leveled-spell count (index 0 == class level 1) for "known" casters.
 * These are the PHB Spells Known columns and exclude cantrips. Mirrors
 * spellsKnownTable in internal/portal/spellbudget.go.
 * @type {Record<string,number[]>}
 */
const SPELLS_KNOWN = {
  bard: [4, 5, 6, 7, 8, 9, 10, 11, 12, 14, 15, 15, 16, 18, 19, 19, 20, 22, 22, 22],
  sorcerer: [2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 12, 13, 13, 14, 14, 15, 15, 15, 15],
  ranger: [0, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11],
  warlock: [2, 3, 4, 5, 6, 7, 8, 9, 10, 10, 11, 11, 12, 12, 13, 13, 14, 14, 15, 15],
};

/**
 * Spells known (leveled, excluding cantrips) for a known-caster class at the
 * given level, or null for prepared casters (cleric/druid/wizard/paladin) and
 * non-casters. The level is clamped into [1,20].
 * @param {string} className
 * @param {number} level
 * @returns {number|null}
 */
export function spellsKnown(className, level) {
  const table = SPELLS_KNOWN[String(className || '').toLowerCase()];
  if (!table) return null;
  const lvl = Math.min(20, Math.max(1, Number(level) || 1));
  return table[lvl - 1];
}

/**
 * Number of leveled (non-cantrip) spells a class may have at the given level.
 * Known casters use their Spells Known table (ability modifier ignored); full
 * prepared casters (cleric/druid/wizard) prepare ability modifier + level;
 * paladins (half-caster) prepare ability modifier + half level and have no
 * spellcasting before level 2. Non-casters return 0. Mirrors leveledSpellCap in
 * internal/portal/spellbudget.go.
 * @param {string} className
 * @param {number} level
 * @param {number} abilityMod
 * @returns {number}
 */
export function leveledSpellCap(className, level, abilityMod) {
  const known = spellsKnown(className, level);
  if (known != null) return known;
  const cls = String(className || '').toLowerCase();
  const lvl = Math.max(1, Number(level) || 1);
  switch (cls) {
    case 'cleric':
    case 'druid':
    case 'wizard':
      return spellPrepCap(abilityMod, lvl);
    case 'paladin':
      return lvl < 2 ? 0 : spellPrepCap(abilityMod, Math.floor(lvl / 2));
    default:
      return 0;
  }
}
