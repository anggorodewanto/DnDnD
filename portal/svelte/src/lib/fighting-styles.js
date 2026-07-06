// Fighting Style picker helpers (COV-15). Two deliberate duplications of the Go
// side, at different confidence levels:
//   - Grant-level logic (FIGHTING_STYLE_GRANT_LEVELS) is hand-reimplemented, which
//     matches how invocations.js reimplements the warlock grant table.
//   - The style DATA (FIGHTING_STYLES) is hand-copied — UNLIKE invocations, whose
//     catalog is a generated JSON guarded by `make invocations-catalog-check`.
//     Justified only because this list is tiny and capped to combat-wired styles,
//     but it has NO drift guard: keep it in sync with FightingStyleCatalog() by
//     hand (the Go side is pinned by TestFightingStyleCatalog_MatchesWiredCombatSet).

// FIGHTING_STYLES holds only the combat-wired styles — each id is the
// mechanical_effect slug the combat engine reads (e.g. "archery").
export const FIGHTING_STYLES = [
  { id: 'archery', name: 'Archery', description: 'You gain a +2 bonus to attack rolls you make with Ranged weapons.' },
  { id: 'defense', name: 'Defense', description: 'While you wear Light, Medium, or Heavy armor, you gain a +1 bonus to Armor Class.' },
  { id: 'dueling', name: 'Dueling', description: 'When you wield a Melee weapon in one hand and no other weapons, you gain a +2 bonus to damage rolls with that weapon.' },
  { id: 'great_weapon_fighting', name: 'Great Weapon Fighting', description: 'When you roll a 1 or 2 on a damage die for an attack you make with a Melee weapon that you hold with two hands, you can reroll the die, and you must use the new roll.' },
  { id: 'two_weapon_fighting', name: 'Two-Weapon Fighting', description: 'When you make an attack with a weapon in your other hand while Two-Weapon Fighting, you can add your ability modifier to the damage of that attack.' },
];

// FIGHTING_STYLE_GRANT_LEVELS: class id -> level at which it grants a fighting
// style. Mirrors fightingStyleGrantLevels (Go) and the seed.
const FIGHTING_STYLE_GRANT_LEVELS = { fighter: 1, paladin: 2, ranger: 2 };

const STYLE_IDS = new Set(FIGHTING_STYLES.map(s => s.id));

/**
 * Whether any of the character's classes grants the Fighting Style feature (so
 * the Class Features step should show the picker). Mirrors
 * submissionFightingStyleGrant (Go).
 * @param {Array<{class?:string, level?:number}>} classEntries
 * @returns {boolean}
 */
export function fightingStyleEligible(classEntries) {
  for (const c of classEntries || []) {
    const grant = FIGHTING_STYLE_GRANT_LEVELS[(c.class || '').toLowerCase()];
    if (grant !== undefined && (Number(c.level) || 0) >= grant) return true;
  }
  return false;
}

/**
 * Reconciles the chosen fighting style at submit time so a stale draft or a
 * class/level change never submits an illegal style: returns '' unless the class
 * still grants a style AND the id is a known combat-wired style.
 * @param {{classEntries:Array, style:string}} ctx
 * @returns {string}
 */
export function reconcileFightingStyle({ classEntries, style }) {
  if (!style || !STYLE_IDS.has(style) || !fightingStyleEligible(classEntries)) return '';
  return style;
}
