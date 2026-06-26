import { describe, it, expect } from 'vitest';
import { assembleEquipment } from './equipment-assembly.js';

const pack = {
  guaranteed: ['javelin:4', 'explorers-pack'],
  choices: [
    { options: ['light-crossbow:1,crossbow-bolt:20', 'handaxe:2'] },
  ],
};

describe('assembleEquipment', () => {
  it('strips quantities by default (picker/display ids)', () => {
    const items = assembleEquipment({ pack, packChoices: [0], manualEquipment: [] });
    expect(items).toEqual(['javelin', 'explorers-pack', 'light-crossbow', 'crossbow-bolt']);
  });

  it('preserves quantities for submission (full quiver, two handaxes)', () => {
    const items = assembleEquipment({ pack, packChoices: [0], manualEquipment: [], preserveQuantities: true });
    expect(items).toEqual(['javelin:4', 'explorers-pack', 'light-crossbow:1', 'crossbow-bolt:20']);
  });

  it('expands the alternate option and preserves its quantity', () => {
    const items = assembleEquipment({ pack, packChoices: [1], manualEquipment: [], preserveQuantities: true });
    expect(items).toEqual(['javelin:4', 'explorers-pack', 'handaxe:2']);
  });

  it('dedups manual items against the pack by bare id', () => {
    const items = assembleEquipment({ pack, packChoices: [1], manualEquipment: ['handaxe', 'dagger'], preserveQuantities: true });
    expect(items).toEqual(['javelin:4', 'explorers-pack', 'handaxe:2', 'dagger']);
  });

  it('handles a missing pack', () => {
    expect(assembleEquipment({ pack: null, packChoices: [], manualEquipment: ['rope'] })).toEqual(['rope']);
  });

  it('returns [] for no input', () => {
    expect(assembleEquipment()).toEqual([]);
  });
});
