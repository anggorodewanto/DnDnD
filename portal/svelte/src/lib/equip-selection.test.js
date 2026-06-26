import { describe, it, expect } from 'vitest';
import {
  armorOptionIds,
  weaponOptionIds,
  reconcileEquipPick,
} from './equip-selection.js';

// ISSUE-011 repro fixture: a Warlock with the entertainer background.
// Starting gear (guaranteed + pack choices, after the builder's id parsing)
// includes leather armor + dagger + a light crossbow — the exact case that
// dropped the player's equip selection. `selectedEquipment` is what the
// builder's selectedEquipment() derives (pack ids already split off ":qty"
// and ",": e.g. "light-crossbow:1,crossbow-bolt:20" -> light-crossbow +
// crossbow-bolt; "dagger:2" -> dagger).
const WARLOCK_SELECTED = ['leather', 'dagger', 'light-crossbow', 'crossbow-bolt', 'shield'];

// The full SRD equipment catalog the builder loads asynchronously. Each entry
// carries the authoritative `category` ('armor' | 'weapon').
const CATALOG = [
  { id: 'leather', name: 'Leather', category: 'armor' },
  { id: 'chain-mail', name: 'Chain mail', category: 'armor' },
  { id: 'shield', name: 'Shield', category: 'armor' },
  { id: 'dagger', name: 'Dagger', category: 'weapon' },
  { id: 'light-crossbow', name: 'Light crossbow', category: 'weapon' },
  { id: 'crossbow-bolt', name: 'Crossbow bolt', category: 'gear' },
];

function byIdFrom(catalog) {
  const m = new Map();
  for (const it of catalog) m.set(it.id, it);
  return m;
}

describe('armorOptionIds', () => {
  it('resolves a pack armor id (leather) once the catalog is loaded', () => {
    const opts = armorOptionIds(WARLOCK_SELECTED, byIdFrom(CATALOG));
    expect(opts).toContain('leather');
    expect(opts).toContain('shield');
    expect(opts).not.toContain('dagger');
  });

  it('still recognises a known armor id before the catalog loads (empty byId)', () => {
    // This is the real ISSUE-011 trigger: the reset effect runs while
    // allEquipment is still []. Without a fallback, leather is unresolved and
    // wrongly excluded, so the legitimate pick gets cleared and never recovers.
    const opts = armorOptionIds(WARLOCK_SELECTED, new Map());
    expect(opts).toContain('leather');
    expect(opts).toContain('shield');
  });
});

describe('weaponOptionIds', () => {
  it('resolves a pack weapon id (light-crossbow) once the catalog is loaded', () => {
    const opts = weaponOptionIds(WARLOCK_SELECTED, byIdFrom(CATALOG));
    expect(opts).toContain('light-crossbow');
    expect(opts).toContain('dagger');
    expect(opts).not.toContain('leather');
    expect(opts).not.toContain('crossbow-bolt');
  });

  it('still recognises a known weapon id before the catalog loads (empty byId)', () => {
    const opts = weaponOptionIds(WARLOCK_SELECTED, new Map());
    expect(opts).toContain('light-crossbow');
    expect(opts).toContain('dagger');
  });

  // ISSUE-017 phase 4: the static weapon/armor sets are derived from the
  // generated items-catalog.json, not a hand-typed list. An SRD id that is NOT
  // in the async catalog fixture above must still classify from the generated
  // catalog with an empty byId — proving the codegen wiring drives it.
  it('classifies an SRD weapon/armor id from the generated catalog (empty byId)', () => {
    expect(weaponOptionIds(['greatsword'], new Map())).toEqual(['greatsword']);
    expect(armorOptionIds(['plate'], new Map())).toEqual(['plate']);
    // ammunition is neither weapon nor armor.
    expect(weaponOptionIds(['arrow'], new Map())).toEqual([]);
    expect(armorOptionIds(['arrow'], new Map())).toEqual([]);
  });
});

describe('reconcileEquipPick', () => {
  it('keeps a valid pick that is in the selected equipment (catalog loaded)', () => {
    const armorOpts = armorOptionIds(WARLOCK_SELECTED, byIdFrom(CATALOG));
    expect(reconcileEquipPick('leather', armorOpts)).toBe('leather');
  });

  it('does NOT clobber a valid pick during the pre-catalog window (empty byId)', () => {
    // The bug: with an empty catalog the old filter excluded leather and the
    // effect reset wornArmor to ''. The pick must SURVIVE because the option
    // list still recognises leather via the known-id fallback.
    const armorOpts = armorOptionIds(WARLOCK_SELECTED, new Map());
    expect(reconcileEquipPick('leather', armorOpts)).toBe('leather');

    const weaponOpts = weaponOptionIds(WARLOCK_SELECTED, new Map());
    expect(reconcileEquipPick('light-crossbow', weaponOpts)).toBe('light-crossbow');
  });

  it('clears a pick that genuinely fell out of the selected equipment', () => {
    // Player swapped their pack choice away from leather: it is no longer in
    // selectedEquipment at all, so the stale pick must clear.
    const without = WARLOCK_SELECTED.filter((id) => id !== 'leather');
    const armorOpts = armorOptionIds(without, byIdFrom(CATALOG));
    expect(reconcileEquipPick('leather', armorOpts)).toBe('');
  });

  it('clears a non-equippable pick (gear chosen as a weapon)', () => {
    // crossbow-bolt is in selectedEquipment but is gear, not a weapon, so it is
    // not a legal weapon option and must clear.
    const weaponOpts = weaponOptionIds(WARLOCK_SELECTED, byIdFrom(CATALOG));
    expect(reconcileEquipPick('crossbow-bolt', weaponOpts)).toBe('');
  });

  it('leaves an empty pick empty', () => {
    expect(reconcileEquipPick('', armorOptionIds(WARLOCK_SELECTED, byIdFrom(CATALOG)))).toBe('');
  });
});
