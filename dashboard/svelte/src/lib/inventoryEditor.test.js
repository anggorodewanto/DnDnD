import { describe, it, expect } from 'vitest';
import { isCatalogItemId, toAddItemPayload, isUnidentified } from './inventoryEditor.js';

describe('isCatalogItemId', () => {
  it('accepts real catalog ids', () => {
    expect(isCatalogItemId('longsword')).toBe(true);
  });
  it('rejects synthetic custom/creature ids and blanks', () => {
    expect(isCatalogItemId('custom-1700000000000')).toBe(false);
    expect(isCatalogItemId('creature-Goblin-dagger')).toBe(false);
    expect(isCatalogItemId('')).toBe(false);
    expect(isCatalogItemId(undefined)).toBe(false);
  });
});

describe('toAddItemPayload', () => {
  it('maps a catalog selection to an InventoryItem payload', () => {
    const payload = toAddItemPayload({
      id: 'longsword',
      name: 'Longsword',
      type: 'weapon',
      quantity: 2,
      is_magic: false,
    });
    expect(payload).toEqual({
      item_id: 'longsword',
      name: 'Longsword',
      quantity: 2,
      type: 'weapon',
      is_magic: false,
      magic_bonus: 0,
      magic_properties: '',
      requires_attunement: false,
      rarity: '',
    });
  });

  it('blanks item_id and normalises type for custom items', () => {
    const payload = toAddItemPayload({
      id: 'custom-123',
      name: 'Mysterious Key',
      type: 'custom',
      quantity: 1,
    });
    expect(payload.item_id).toBe('');
    expect(payload.type).toBe('other');
  });

  it('defaults a non-positive or missing quantity to 1', () => {
    expect(toAddItemPayload({ id: 'rope', name: 'Rope', type: 'gear' }).quantity).toBe(1);
    expect(toAddItemPayload({ id: 'rope', name: 'Rope', quantity: 0 }).quantity).toBe(1);
  });

  it('carries magic metadata through', () => {
    const payload = toAddItemPayload({
      id: 'flametongue',
      name: 'Flame Tongue',
      type: 'weapon',
      quantity: 1,
      is_magic: true,
      magic_bonus: 1,
      magic_properties: '+1d6 fire',
      requires_attunement: true,
      rarity: 'rare',
    });
    expect(payload.is_magic).toBe(true);
    expect(payload.magic_bonus).toBe(1);
    expect(payload.magic_properties).toBe('+1d6 fire');
    expect(payload.requires_attunement).toBe(true);
    expect(payload.rarity).toBe('rare');
  });
});

describe('isUnidentified', () => {
  it('is true only for magic items explicitly marked identified=false', () => {
    expect(isUnidentified({ is_magic: true, identified: false })).toBe(true);
  });
  it('is false for revealed or non-magic items', () => {
    expect(isUnidentified({ is_magic: true, identified: true })).toBe(false);
    expect(isUnidentified({ is_magic: true })).toBe(false);
    expect(isUnidentified({ is_magic: false, identified: false })).toBe(false);
  });
});
