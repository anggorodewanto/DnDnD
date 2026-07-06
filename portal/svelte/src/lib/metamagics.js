// Metamagic picker helpers (COV-15). Multi-select analogue of fighting-styles.js
// (which is single-select): the Sorcerer picks N metamagic options, capped by
// sorcerer level. Two deliberate duplications of the Go side, at different
// confidence levels:
//   - Grant-count logic (metamagicGrantCount) is hand-reimplemented, matching how
//     invocations.js / fighting-styles.js reimplement their grant tables.
//   - The metamagic DATA (METAMAGICS) is hand-copied — UNLIKE invocations, whose
//     catalog is a generated JSON with a `make` drift guard. Justified only
//     because this list is tiny and capped to the combat-wired options, but it has
//     NO drift guard: keep it in sync with MetamagicCatalog() by hand (the Go side
//     is pinned by TestMetamagicCatalog_MatchesWiredCombatSet).

// METAMAGICS holds only the combat-wired options — each id is the
// mechanical_effect slug the combat cast gate reads (e.g. "quickened").
export const METAMAGICS = [
  { id: 'careful', name: 'Careful Spell', description: 'When you cast a spell that forces other creatures to make a saving throw, a number of them equal to your Charisma modifier (minimum 1) automatically succeed.' },
  { id: 'distant', name: 'Distant Spell', description: 'When you cast a spell with a range of at least 5 feet, you can double its range; a touch spell instead gains a range of 30 feet.' },
  { id: 'empowered', name: 'Empowered Spell', description: 'When you roll damage for a spell, you can reroll a number of the damage dice up to your Charisma modifier (minimum 1).' },
  { id: 'extended', name: 'Extended Spell', description: 'When you cast a spell with a duration of 1 minute or longer, you can double its duration, to a maximum of 24 hours.' },
  { id: 'heightened', name: 'Heightened Spell', description: 'When you cast a spell that forces a saving throw, one target has disadvantage on its first save against the spell.' },
  { id: 'quickened', name: 'Quickened Spell', description: 'When you cast a spell with a casting time of an action, you can cast it as a bonus action instead.' },
  { id: 'subtle', name: 'Subtle Spell', description: 'When you cast a spell, you can cast it without any verbal or somatic components.' },
  { id: 'twinned', name: 'Twinned Spell', description: 'When you cast a spell that targets only one creature and lacks a range of self, you can target a second creature in range.' },
];

const METAMAGIC_IDS = new Set(METAMAGICS.map(m => m.id));

/**
 * Sorcerer class level in the character's classes (0 if none). Mirrors
 * submissionSorcererLevel (Go).
 * @param {Array<{class?:string, level?:number}>} classEntries
 * @returns {number}
 */
function sorcererLevelOf(classEntries) {
  for (const c of classEntries || []) {
    if ((c.class || '').toLowerCase() === 'sorcerer') return Number(c.level) || 0;
  }
  return 0;
}

/**
 * How many metamagic options the character may pick (2/3/4 at sorcerer level
 * 3/10/17; 0 below 3). Mirrors refdata.MetamagicKnown (Go).
 * @param {Array<{class?:string, level?:number}>} classEntries
 * @returns {number}
 */
export function metamagicGrantCount(classEntries) {
  const lvl = sorcererLevelOf(classEntries);
  if (lvl < 3) return 0;
  if (lvl < 10) return 2;
  if (lvl < 17) return 3;
  return 4;
}

/**
 * Whether the character can pick any metamagic (so the Class Features step shows
 * the picker). Mirrors submissionSorcererLevel > 0 gate.
 * @param {Array<{class?:string, level?:number}>} classEntries
 * @returns {boolean}
 */
export function metamagicEligible(classEntries) {
  return metamagicGrantCount(classEntries) > 0;
}

/**
 * Reconciles the chosen metamagics at submit time so a stale draft or a
 * class/level change never submits an illegal set: drops unknown ids, dedupes,
 * and caps at the grant count. Returns [] when the character grants none.
 * @param {{classEntries:Array, metamagics:Array<string>}} ctx
 * @returns {Array<string>}
 */
export function reconcileMetamagics({ classEntries, metamagics }) {
  const cap = metamagicGrantCount(classEntries);
  if (cap === 0) return [];
  const out = [];
  for (const id of metamagics || []) {
    if (out.length >= cap) break;
    if (METAMAGIC_IDS.has(id) && !out.includes(id)) out.push(id);
  }
  return out;
}
