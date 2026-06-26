/**
 * Pure helpers for the DM inventory editor. Kept framework-free so the mapping
 * from an ItemPicker selection to an inventory "add" payload is unit-testable
 * without mounting the Svelte component.
 */

/**
 * Items selected through ItemPicker carry synthetic ids for custom and creature
 * sourced entries (e.g. "custom-1700000000000", "creature-Goblin-dagger"). Those
 * are UI-only and must not be persisted as the canonical item_id — only real
 * catalog ids round-trip. Mirrors the loot pool's id-resolution rule.
 * @param {string} [id]
 * @returns {boolean}
 */
export function isCatalogItemId(id) {
  return typeof id === 'string' && id !== '' && !id.startsWith('custom-') && !id.startsWith('creature-');
}

/**
 * Map a single ItemPicker selection to the InventoryItem payload expected by
 * POST /api/inventory/add. Synthetic picker ids collapse to an empty item_id;
 * the "custom" type normalises to "other".
 * @param {object} picked - One entry from ItemPicker's onselect array.
 * @returns {object} InventoryItem payload.
 */
export function toAddItemPayload(picked) {
  const quantity = Number.parseInt(picked.quantity, 10);
  return {
    item_id: isCatalogItemId(picked.id) ? picked.id : '',
    name: picked.name,
    quantity: Number.isFinite(quantity) && quantity > 0 ? quantity : 1,
    type: picked.type === 'custom' ? 'other' : picked.type || 'other',
    is_magic: !!picked.is_magic,
    magic_bonus: picked.magic_bonus || 0,
    magic_properties: picked.magic_properties || '',
    requires_attunement: !!picked.requires_attunement,
    rarity: picked.rarity || '',
  };
}

/**
 * Whether an inventory item is a magic item the DM has hidden from the player.
 * Identified is a tri-state on the server (*bool): absent/true = revealed,
 * explicit false = hidden. Only magic items are ever hidden.
 * @param {object} item - InventoryItem.
 * @returns {boolean}
 */
export function isUnidentified(item) {
  return !!item.is_magic && item.identified === false;
}
