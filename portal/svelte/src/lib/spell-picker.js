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
 * selectable or the cap is already reached.
 * @param {object} spell
 * @param {{selected?: string[], alwaysPrepared?: string[], max?: number, selectableLevels?: (number[]|Set<number>|null)}} [opts]
 * @returns {boolean}
 */
export function isSpellDisabled(spell, opts = {}) {
  const { selected = [], alwaysPrepared = [], max = Infinity, selectableLevels = null } = opts || {};
  const id = spell?.id;
  if ((alwaysPrepared || []).includes(id)) return true;
  if ((selected || []).includes(id)) return false;
  if (!isLevelSelectable(spell?.level, selectableLevels)) return true;
  return countAgainstCap(selected, alwaysPrepared) >= max;
}

/**
 * A short human reason a spell can't currently be selected, or '' when it is
 * selectable or already selected. Used for the checkbox title/tooltip.
 * @param {object} spell
 * @param {{selected?: string[], alwaysPrepared?: string[], max?: number, selectableLevels?: (number[]|Set<number>|null)}} [opts]
 * @returns {string}
 */
export function disabledReason(spell, opts = {}) {
  const { selected = [], alwaysPrepared = [], max = Infinity, selectableLevels = null } = opts || {};
  const id = spell?.id;
  if ((alwaysPrepared || []).includes(id)) return 'Always prepared (subclass)';
  if ((selected || []).includes(id)) return '';
  if (!isLevelSelectable(spell?.level, selectableLevels)) return 'No spell slots of this level yet';
  if (countAgainstCap(selected, alwaysPrepared) >= max) return 'Preparation limit reached';
  return '';
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
