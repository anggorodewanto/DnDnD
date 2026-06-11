/**
 * Pure selection logic shared by the SpellPicker component (used by both the
 * character builder's spell step and the standalone spell-prep page).
 *
 * The model: a player may have at most `max` spells that count against the
 * cap, browse spells of every level, but only *select* spells whose level is
 * castable (`selectableLevels`). Always-prepared spells (subclass grants) are
 * locked on and never count against the cap.
 *
 * Operates on the portal API `SpellInfo` shape ({ id, name, level, school, ... }).
 * All functions are non-mutating.
 */

/**
 * Number of selected spells that count against the cap: every selected id that
 * is not an always-prepared id.
 * @param {string[]} selected
 * @param {string[]} [alwaysPrepared=[]]
 * @returns {number}
 */
export function countAgainstCap(selected, alwaysPrepared = []) {
  const always = new Set(alwaysPrepared || []);
  return (selected || []).filter((id) => !always.has(id)).length;
}

/**
 * Whether a spell of the given level is counted against the separate cantrip
 * budget. Cantrips only get their own bucket when a finite `cantripMax` is in
 * play; otherwise (the standalone prep page, which has one combined cap) they
 * fall into the leveled bucket so single-cap behaviour is preserved.
 * @param {number|string} level
 * @param {number} cantripMax
 * @returns {boolean}
 */
function isCantripBucket(level, cantripMax) {
  return Number(level) === 0 && Number.isFinite(cantripMax);
}

/**
 * Splits the selected (non-always-prepared) spells into cantrip and leveled
 * counts, resolving each id's level from the `spells` catalog. Ids missing from
 * the catalog are counted as leveled. With no finite `cantripMax` every spell
 * lands in the leveled bucket (single-cap mode).
 * @param {string[]} selected
 * @param {{spells?: object[], alwaysPrepared?: string[], cantripMax?: number}} [opts]
 * @returns {{cantrips: number, leveled: number}}
 */
export function countByBucket(selected, opts = {}) {
  const { spells = [], alwaysPrepared = [], cantripMax = Infinity } = opts || {};
  const always = new Set(alwaysPrepared || []);
  const levelById = new Map((spells || []).map((s) => [s?.id, Number(s?.level) || 0]));
  let cantrips = 0;
  let leveled = 0;
  for (const id of selected || []) {
    if (always.has(id)) continue;
    const level = levelById.has(id) ? levelById.get(id) : 1;
    if (isCantripBucket(level, cantripMax)) cantrips += 1;
    else leveled += 1;
  }
  return { cantrips, leveled };
}

/**
 * Whether a spell of `level` may be selected given the slot-level gate.
 * Cantrips (level 0) are always selectable. A null/undefined `selectableLevels`
 * means no gate (every level selectable).
 * @param {number|string} level
 * @param {(number[]|Set<number>|null)} [selectableLevels]
 * @returns {boolean}
 */
export function isLevelSelectable(level, selectableLevels) {
  if (Number(level) === 0) return true;
  if (selectableLevels == null) return true;
  const set = selectableLevels instanceof Set ? selectableLevels : new Set(selectableLevels.map(Number));
  return set.has(Number(level));
}

/**
 * Whether a spell's checkbox should be disabled.
 * Always-prepared spells are locked. Already-selected spells are never disabled
 * (so they can be removed). Otherwise a spell is disabled when its level is not
 * selectable or its budget is already full. Cantrips are checked against
 * `cantripMax` (when finite) and leveled spells against `max`; in single-cap
 * mode every spell shares `max`.
 * @param {object} spell
 * @param {{selected?: string[], alwaysPrepared?: string[], max?: number, cantripMax?: number, selectableLevels?: (number[]|Set<number>|null), spells?: object[]}} [opts]
 * @returns {boolean}
 */
export function isSpellDisabled(spell, opts = {}) {
  const { selected = [], alwaysPrepared = [], max = Infinity, cantripMax = Infinity, selectableLevels = null } = opts || {};
  const id = spell?.id;
  if ((alwaysPrepared || []).includes(id)) return true;
  if ((selected || []).includes(id)) return false;
  if (!isLevelSelectable(spell?.level, selectableLevels)) return true;
  const { cantrips, leveled } = countByBucket(selected, opts);
  if (isCantripBucket(spell?.level, cantripMax)) return cantrips >= cantripMax;
  return leveled >= max;
}

/**
 * A short human reason a spell can't currently be selected, or '' when it is
 * selectable or already selected. Used for the checkbox title/tooltip.
 * @param {object} spell
 * @param {{selected?: string[], alwaysPrepared?: string[], max?: number, cantripMax?: number, selectableLevels?: (number[]|Set<number>|null), spells?: object[]}} [opts]
 * @returns {string}
 */
export function disabledReason(spell, opts = {}) {
  const { selected = [], alwaysPrepared = [], max = Infinity, cantripMax = Infinity, selectableLevels = null } = opts || {};
  const id = spell?.id;
  if ((alwaysPrepared || []).includes(id)) return 'Always prepared (subclass)';
  if ((selected || []).includes(id)) return '';
  if (!isLevelSelectable(spell?.level, selectableLevels)) return 'No spell slots of this level yet';
  const { cantrips, leveled } = countByBucket(selected, opts);
  if (isCantripBucket(spell?.level, cantripMax)) {
    return cantrips >= cantripMax ? 'Cantrip limit reached' : '';
  }
  return leveled >= max ? 'Preparation limit reached' : '';
}

/**
 * Whether a spell should be hidden when the "hide unselectable" toggle is on.
 * Hidden spells are exactly the ones the picker greys out: disabled and not
 * already on. Selected and always-prepared spells stay visible so the player
 * can still see and remove their picks.
 * @param {object} spell
 * @param {{selected?: string[], alwaysPrepared?: string[], max?: number, selectableLevels?: (number[]|Set<number>|null)}} [opts]
 * @returns {boolean}
 */
export function isSpellHidden(spell, opts = {}) {
  const { selected = [], alwaysPrepared = [] } = opts || {};
  const id = spell?.id;
  if ((selected || []).includes(id)) return false;
  if ((alwaysPrepared || []).includes(id)) return false;
  return isSpellDisabled(spell, opts);
}

/**
 * Filters a spell list for display. With `hide` false the list is returned as
 * is; with `hide` true every unselectable spell (see isSpellHidden) is dropped.
 * Non-mutating.
 * @param {object[]} spells
 * @param {boolean} hide
 * @param {{selected?: string[], alwaysPrepared?: string[], max?: number, selectableLevels?: (number[]|Set<number>|null)}} [opts]
 * @returns {object[]}
 */
export function visibleSpells(spells, hide, opts = {}) {
  const list = spells || [];
  if (!hide) return list;
  return list.filter((spell) => !isSpellHidden(spell, opts));
}

/**
 * Toggles a spell id in the selected list, returning a NEW array. Always-
 * prepared ids are immutable (no-op).
 * @param {string[]} selected
 * @param {string} id
 * @param {string[]} [alwaysPrepared=[]]
 * @returns {string[]}
 */
export function toggleSelected(selected, id, alwaysPrepared = []) {
  if ((alwaysPrepared || []).includes(id)) return selected || [];
  const list = selected || [];
  if (list.includes(id)) return list.filter((s) => s !== id);
  return [...list, id];
}
