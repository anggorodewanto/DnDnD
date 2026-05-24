import { describe, it, expect } from 'vitest';
import { proficientWeaponIds, masteryEligibleWeapons } from './weapon-proficiency.js';

const WEAPONS = [
  { id: 'club', name: 'Club', weapon_type: 'simple_melee', mastery: 'slow' },
  { id: 'dagger', name: 'Dagger', weapon_type: 'simple_melee', mastery: 'nick' },
  { id: 'shortbow', name: 'Shortbow', weapon_type: 'simple_ranged', mastery: 'vex' },
  { id: 'longsword', name: 'Longsword', weapon_type: 'martial_melee', mastery: 'sap' },
  { id: 'greataxe', name: 'Greataxe', weapon_type: 'martial_melee', mastery: 'cleave' },
  { id: 'net', name: 'Net', weapon_type: 'martial_ranged', mastery: '' },
  { id: 'hand-crossbow', name: 'Hand crossbow', weapon_type: 'martial_ranged', mastery: 'vex' },
];

describe('proficientWeaponIds', () => {
  it('grants all simple_* ids for the simple category', () => {
    expect(proficientWeaponIds(WEAPONS, ['simple'], [])).toEqual(['club', 'dagger', 'shortbow']);
  });

  it('grants all martial_* ids for the martial category', () => {
    expect(proficientWeaponIds(WEAPONS, ['martial'], [])).toEqual([
      'longsword',
      'greataxe',
      'net',
      'hand-crossbow',
    ]);
  });

  it('grants every weapon for simple + martial', () => {
    expect(proficientWeaponIds(WEAPONS, ['simple', 'martial'], [])).toEqual([
      'club',
      'dagger',
      'shortbow',
      'longsword',
      'greataxe',
      'net',
      'hand-crossbow',
    ]);
  });

  it("accepts the 'simple weapons' token (with a space) as the simple category", () => {
    expect(proficientWeaponIds(WEAPONS, ['simple weapons'], [])).toEqual([
      'club',
      'dagger',
      'shortbow',
    ]);
  });

  it("accepts the 'martial weapons' token (with a space) as the martial category", () => {
    expect(proficientWeaponIds(WEAPONS, ['martial weapons'], [])).toEqual([
      'longsword',
      'greataxe',
      'net',
      'hand-crossbow',
    ]);
  });

  it('normalises a specific weapon name with a space to a hyphenated id', () => {
    expect(proficientWeaponIds(WEAPONS, ['hand crossbow'], [])).toEqual(['hand-crossbow']);
  });

  it('matches a specific weapon name regardless of case', () => {
    expect(proficientWeaponIds(WEAPONS, ['Hand Crossbow'], [])).toEqual(['hand-crossbow']);
  });

  it('grants race-provided specific ids with an empty class list', () => {
    expect(proficientWeaponIds(WEAPONS, [], ['longsword'])).toEqual(['longsword']);
  });

  it('lowercases race-provided ids', () => {
    expect(proficientWeaponIds(WEAPONS, [], ['LONGSWORD'])).toEqual(['longsword']);
  });

  it('combines a class category with a race id, deduped and in weapon order', () => {
    expect(proficientWeaponIds(WEAPONS, ['simple'], ['longsword'])).toEqual([
      'club',
      'dagger',
      'shortbow',
      'longsword',
    ]);
  });

  it('does not duplicate an id granted by both class and race', () => {
    expect(proficientWeaponIds(WEAPONS, ['simple'], ['club'])).toEqual([
      'club',
      'dagger',
      'shortbow',
    ]);
  });

  it('returns [] for null weapons', () => {
    expect(proficientWeaponIds(null, ['simple'], ['longsword'])).toEqual([]);
  });

  it('returns [] for undefined weapons', () => {
    expect(proficientWeaponIds(undefined, ['simple'], [])).toEqual([]);
  });

  it('tolerates null classProficiencies and raceWeaponIds', () => {
    expect(proficientWeaponIds(WEAPONS, null, null)).toEqual([]);
  });

  it('tolerates undefined classProficiencies and raceWeaponIds', () => {
    expect(proficientWeaponIds(WEAPONS, undefined, undefined)).toEqual([]);
  });

  it('ignores empty/whitespace tokens in classProficiencies', () => {
    expect(proficientWeaponIds(WEAPONS, ['', '  '], [])).toEqual([]);
  });
});

describe('masteryEligibleWeapons', () => {
  it('returns proficient weapon objects that have a non-empty mastery', () => {
    const ids = ['longsword', 'greataxe', 'net', 'hand-crossbow'];
    expect(masteryEligibleWeapons(WEAPONS, ids)).toEqual([
      { id: 'longsword', name: 'Longsword', weapon_type: 'martial_melee', mastery: 'sap' },
      { id: 'greataxe', name: 'Greataxe', weapon_type: 'martial_melee', mastery: 'cleave' },
      { id: 'hand-crossbow', name: 'Hand crossbow', weapon_type: 'martial_ranged', mastery: 'vex' },
    ]);
  });

  it('excludes net even when proficient because its mastery is empty', () => {
    const result = masteryEligibleWeapons(WEAPONS, ['net', 'longsword']);
    expect(result.map((w) => w.id)).toEqual(['longsword']);
  });

  it('excludes weapons that are not in proficientIds', () => {
    expect(masteryEligibleWeapons(WEAPONS, ['club']).map((w) => w.id)).toEqual(['club']);
  });

  it('accepts a Set for proficientIds', () => {
    const ids = new Set(['shortbow', 'hand-crossbow']);
    expect(masteryEligibleWeapons(WEAPONS, ids).map((w) => w.id)).toEqual([
      'shortbow',
      'hand-crossbow',
    ]);
  });

  it('preserves the input weapon order', () => {
    const ids = ['hand-crossbow', 'club'];
    expect(masteryEligibleWeapons(WEAPONS, ids).map((w) => w.id)).toEqual(['club', 'hand-crossbow']);
  });

  it('does not mutate the input weapons array', () => {
    const copy = WEAPONS.map((w) => ({ ...w }));
    masteryEligibleWeapons(WEAPONS, ['club']);
    expect(WEAPONS).toEqual(copy);
  });

  it('returns [] for null weapons', () => {
    expect(masteryEligibleWeapons(null, ['club'])).toEqual([]);
  });

  it('returns [] for null proficientIds', () => {
    expect(masteryEligibleWeapons(WEAPONS, null)).toEqual([]);
  });

  it('returns [] for undefined inputs', () => {
    expect(masteryEligibleWeapons(undefined, undefined)).toEqual([]);
  });
});
