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
// from the selected equipment (not merely unresolved). Mirrors the Go
// knownWeapons / knownArmor maps in builder_store_adapter.go.

// SRD weapon ids (mirror of knownWeapons in internal/portal/builder_store_adapter.go).
const KNOWN_WEAPON_IDS = new Set([
  'club', 'dagger', 'greatclub', 'handaxe', 'javelin',
  'light-hammer', 'mace', 'quarterstaff', 'sickle', 'spear',
  'light-crossbow', 'dart', 'shortbow', 'sling',
  'battleaxe', 'flail', 'glaive', 'greataxe', 'greatsword',
  'halberd', 'lance', 'longsword', 'maul', 'morningstar',
  'pike', 'rapier', 'scimitar', 'shortsword', 'trident',
  'war-pick', 'warhammer', 'whip', 'blowgun', 'hand-crossbow',
  'heavy-crossbow', 'longbow', 'net',
]);

// SRD armor ids (mirror of knownArmor in internal/portal/builder_store_adapter.go).
// `shield` lives in the armor slot's option list (it occupies the off-hand at
// persist time, but the builder surfaces it under Worn Armor like the shield).
const KNOWN_ARMOR_IDS = new Set([
  'padded', 'leather', 'studded-leather',
  'hide', 'chain-shirt', 'scale-mail', 'breastplate', 'half-plate',
  'ring-mail', 'chain-mail', 'splint', 'plate',
  'shield',
]);

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
