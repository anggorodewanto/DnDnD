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
 * The two martial third-caster subclasses (Fighter/Eldritch Knight and
 * Rogue/Arcane Trickster). They cast with INT and gain spellcasting only when
 * the subclass is chosen at class level 3. Both spaced and hyphenated slugs are
 * accepted to mirror character.isThirdCasterSubclass on the Go side.
 * @type {Set<string>}
 */
const THIRD_CASTER_SUBCLASSES = new Set([
  'eldritch-knight', 'eldritch knight', 'arcane-trickster', 'arcane trickster',
]);

/** Class level at which an EK/AT subclass first grants spellcasting. */
const THIRD_CASTER_MIN_LEVEL = 3;

/**
 * Whether the given subclass at the given level is a spellcasting third-caster
 * (EK/AT at class level >= 3).
 * @param {string} subclass
 * @param {number} level
 * @returns {boolean}
 */
export function isThirdCaster(subclass, level) {
  if (!subclass) return false;
  if ((Number(level) || 0) < THIRD_CASTER_MIN_LEVEL) return false;
  return THIRD_CASTER_SUBCLASSES.has(String(subclass).toLowerCase());
}

/**
 * Returns the spellcasting ability slug ('int'|'wis'|'cha') for a class, or
 * null when the class is not a spellcaster. A Fighter/Eldritch Knight or
 * Rogue/Arcane Trickster at level >= 3 is an INT caster even though the base
 * class is not; pass the selected subclass + level to detect them.
 * @param {string} className
 * @param {string} [subclass]
 * @param {number} [level]
 * @returns {string|null}
 */
export function spellcastingAbilityForClass(className, subclass, level) {
  const base = className ? CASTER_ABILITY[String(className).toLowerCase()] ?? null : null;
  if (base !== null) return base;
  if (isThirdCaster(subclass, level)) return 'int';
  return null;
}

/**
 * Whether a class is a spellcaster, accounting for the EK/AT third-caster
 * subclasses (which only cast at class level >= 3).
 * @param {string} className
 * @param {string} [subclass]
 * @param {number} [level]
 * @returns {boolean}
 */
export function isSpellcaster(className, subclass, level) {
  return spellcastingAbilityForClass(className, subclass, level) !== null;
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
 * Cantrips known by the EK/AT third-casters: Eldritch Knight knows 2 at L3 and
 * 3 at L10; Arcane Trickster knows 3 at L3 and 4 at L10. Keyed by normalized
 * subclass slug. Mirrors thirdCasterCantrips in internal/portal/spellbudget.go.
 * @type {Record<string,{base:number,atTen:number}>}
 */
const THIRD_CASTER_CANTRIPS = {
  'eldritch-knight': { base: 2, atTen: 3 },
  'eldritch knight': { base: 2, atTen: 3 },
  'arcane-trickster': { base: 3, atTen: 4 },
  'arcane trickster': { base: 3, atTen: 4 },
};

/**
 * Per-class-level leveled (non-cantrip) spells known for EK/AT third-casters
 * (index 0 == class level 1). Both subclasses share the PHB third-caster Spells
 * Known column; levels 1–2 are 0 because the subclass is not yet chosen.
 * Mirrors thirdCasterSpellsKnown in internal/portal/spellbudget.go.
 * @type {number[]}
 */
const THIRD_CASTER_SPELLS_KNOWN = [
  0, 0, 3, 4, 4, 4, 5, 6, 6, 7, 8, 8, 9, 10, 10, 11, 11, 11, 12, 13,
];

/**
 * Number of cantrips a class knows at the given class level. 0 for classes
 * that learn no cantrips. A Fighter/EK or Rogue/AT (level >= 3) uses the
 * third-caster cantrip counts; pass the selected subclass.
 * @param {string} className
 * @param {number} level
 * @param {string} [subclass]
 * @returns {number}
 */
export function cantripsKnown(className, level, subclass) {
  const base = CANTRIP_BASE[String(className || '').toLowerCase()];
  if (base != null) {
    const lvl = Math.max(1, Number(level) || 1);
    let n = base;
    if (lvl >= 4) n += 1;
    if (lvl >= 10) n += 1;
    return n;
  }
  if (!isThirdCaster(subclass, level)) return 0;
  const tc = THIRD_CASTER_CANTRIPS[String(subclass).toLowerCase()];
  return (Number(level) || 0) >= 10 ? tc.atTen : tc.base;
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
 * @param {string} [subclass]
 * @returns {number}
 */
export function leveledSpellCap(className, level, abilityMod, subclass) {
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
      if (!isThirdCaster(subclass, level)) return 0;
      return THIRD_CASTER_SPELLS_KNOWN[Math.min(20, lvl) - 1];
  }
}

/**
 * A multiclass entry: a class slug, its level in that class, and the optional
 * chosen subclass. Mirrors character.ClassEntry on the Go side.
 * @typedef {{class: string, level: number, subclass?: string}} ClassEntry
 */

/**
 * Whether any class entry is a spellcaster. In a multiclass build the Spells
 * step (and every spell budget) must open when ANY entry casts — even when the
 * primary class is a non-caster but a secondary is (e.g. Fighter 1 / Wizard 3),
 * or when an EK/AT subclass turns a martial entry into a caster at level >= 3.
 * @param {ClassEntry[]} [classEntries]
 * @returns {boolean}
 */
export function anyCaster(classEntries) {
  if (!Array.isArray(classEntries)) return false;
  return classEntries.some((c) => isSpellcaster(c.class, c.subclass, c.level));
}

/**
 * Combined cantrips-known cap across a multiclass build: the sum of
 * cantripsKnown over the caster entries, each using that class's own level and
 * subclass. Non-caster entries contribute 0. In 5e, cantrip counts are per-class
 * (only spell *slots* combine), so the budgets add independently.
 * @param {ClassEntry[]} [classEntries]
 * @returns {number}
 */
export function multiclassCantripCap(classEntries) {
  if (!Array.isArray(classEntries)) return 0;
  return classEntries.reduce((total, c) => {
    if (!isSpellcaster(c.class, c.subclass, c.level)) return total;
    return total + cantripsKnown(c.class, c.level, c.subclass);
  }, 0);
}

/**
 * Combined leveled-spell cap across a multiclass build: the sum of
 * leveledSpellCap over the caster entries, each using its own level, subclass,
 * and spellcasting-ability modifier. The ability is resolved per class via
 * spellcastingAbilityForClass ('int'|'wis'|'cha') and looked up in `mods`
 * (default 0 when missing). Non-caster entries contribute 0.
 * @param {ClassEntry[]} [classEntries]
 * @param {Record<string,number>} mods ability-slug -> modifier (e.g. {int:2, wis:0, cha:3})
 * @returns {number}
 */
export function multiclassLeveledCap(classEntries, mods) {
  if (!Array.isArray(classEntries)) return 0;
  return classEntries.reduce((total, c) => {
    const ability = spellcastingAbilityForClass(c.class, c.subclass, c.level);
    if (ability === null) return total;
    const mod = Number(mods?.[ability]) || 0;
    return total + leveledSpellCap(c.class, c.level, mod, c.subclass);
  }, 0);
}
