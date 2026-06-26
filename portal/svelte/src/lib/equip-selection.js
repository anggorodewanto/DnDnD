// Pure helpers for the character builder's "Active Loadout" pickers (worn
// armor + equipped weapon). Kept Svelte-free so the equip-persistence rules
// are cheap to unit-test and can never be silently undone by render ordering.
//
// ISSUE-011: the previous inline logic decided whether a selected item was
// armor/weapon *solely* from the asynchronously-loaded equipment catalog's
// `category`. The reset effects run while that catalog is still empty (e.g.
// right after a draft restore), so a legitimate pack pick (leather, a light
// crossbow) failed the category check, was excluded from the option list, and
// the effect reset the player's choice to '' — permanently, because the
// recovered catalog never re-populated the cleared value. The submission then
// shipped empty equipped_weapon / worn_armor.
//
// The fix: recognise an item as equippable from a static SRD id set as a
// fallback, so a pick that is genuinely in the selected-equipment list survives
// the pre-catalog window; and only clear a pick when its id is truly absent
// from the selected equipment (not merely unresolved).
//
// ISSUE-017 phase 4: those static id sets are no longer hand-maintained. They
// are derived from items-catalog.json, GENERATED from the canonical Go item
// catalog (internal/refdata.ItemCatalog via scripts/gen_items_catalog), so the
// Go backend and this classifier share ONE source — no parallel SRD list to
// drift. `shield` is category 'armor' in the catalog, so it lands in the armor
// set as the builder expects (it occupies the off-hand at persist time but is
// surfaced under Worn Armor).
import itemsCatalog from './items-catalog.json';

const idsForCategory = (category) =>
  new Set(itemsCatalog.filter((it) => it.category === category).map((it) => it.id));

const KNOWN_WEAPON_IDS = idsForCategory('weapon');
const KNOWN_ARMOR_IDS = idsForCategory('armor');

function isArmorId(id, byId) {
  if (KNOWN_ARMOR_IDS.has(id)) return true;
  const item = byId?.get(id);
  return Boolean(item && item.category === 'armor');
}

function isWeaponId(id, byId) {
  if (KNOWN_WEAPON_IDS.has(id)) return true;
  const item = byId?.get(id);
  return Boolean(item && item.category === 'weapon');
}

/**
 * Armor ids the player may equip, from their selected equipment. An id counts
 * as armor when the loaded catalog marks it category 'armor' OR it is a known
 * SRD armor id — the fallback keeps pack armor selectable before the catalog
 * resolves.
 * @param {string[]} selected - ids from selectedEquipment()
 * @param {Map<string, {category?: string}>} byId - catalog index (may be empty)
 * @returns {string[]}
 */
export function armorOptionIds(selected, byId) {
  return (selected ?? []).filter((id) => isArmorId(id, byId));
}

/**
 * Weapon ids the player may equip, from their selected equipment. Mirrors
 * armorOptionIds with the weapon catalog/fallback.
 * @param {string[]} selected - ids from selectedEquipment()
 * @param {Map<string, {category?: string}>} byId - catalog index (may be empty)
 * @returns {string[]}
 */
export function weaponOptionIds(selected, byId) {
  return (selected ?? []).filter((id) => isWeaponId(id, byId));
}

/**
 * Reconcile a worn-armor / equipped-weapon pick against the current options.
 * Returns the pick unchanged when it is still a legal option, or '' when it
 * must clear.
 *
 * Equippability — including the pre-catalog known-id fallback — is already
 * encoded in `options` (see armorOptionIds / weaponOptionIds), so a pack item
 * the player kept survives the catalog-loading window while a swapped-away pick
 * (gone from selectedEquipment, so absent from options) or a non-equippable
 * pick (e.g. gear chosen as a weapon) correctly clears. The third argument is
 * accepted for call-site symmetry but is intentionally unused.
 *
 * @param {string} pick - current wornArmor / equippedWeapon value
 * @param {string[]} options - armorOptionIds() / weaponOptionIds() result
 * @returns {string} the kept pick, or ''
 */
export function reconcileEquipPick(pick, options) {
  if (!pick) return '';
  return options.includes(pick) ? pick : '';
}
