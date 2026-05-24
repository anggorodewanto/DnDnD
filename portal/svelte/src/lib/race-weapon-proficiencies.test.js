import { describe, it, expect } from 'vitest';
import {
  raceGrantedWeaponProficiencies,
  weaponProficiencyLabel,
} from './race-weapon-proficiencies.js';

const WEAPON_IDS = ['battleaxe', 'handaxe', 'light-hammer', 'warhammer', 'longsword'];

const DWARF_TRAITS = [
  {
    name: 'Dwarven Combat Training',
    description: 'You have proficiency with the battleaxe, handaxe, light hammer, and warhammer.',
    mechanical_effect:
      'proficiency_battleaxe,proficiency_handaxe,proficiency_light-hammer,proficiency_warhammer',
  },
];

describe('raceGrantedWeaponProficiencies', () => {
  it('extracts Dwarven Combat Training weapon proficiencies against an allow-list', () => {
    expect(raceGrantedWeaponProficiencies(DWARF_TRAITS, WEAPON_IDS)).toEqual([
      'battleaxe',
      'handaxe',
      'light-hammer',
      'warhammer',
    ]);
  });

  it('excludes skill and tool proficiency codes not in the weapon allow-list', () => {
    const traits = [
      {
        name: 'Mixed',
        description: '',
        mechanical_effect:
          'proficiency_battleaxe,proficiency_perception,proficiency_tinkers-tools',
      },
    ];
    expect(raceGrantedWeaponProficiencies(traits, WEAPON_IDS)).toEqual(['battleaxe']);
  });

  it('normalizes underscores to hyphens before matching the allow-list', () => {
    const traits = [
      { name: 'X', description: '', mechanical_effect: 'proficiency_light_hammer' },
    ];
    expect(raceGrantedWeaponProficiencies(traits, ['light-hammer'])).toEqual(['light-hammer']);
  });

  it('accepts the allow-list as a Set', () => {
    const allow = new Set(WEAPON_IDS);
    expect(raceGrantedWeaponProficiencies(DWARF_TRAITS, allow)).toEqual([
      'battleaxe',
      'handaxe',
      'light-hammer',
      'warhammer',
    ]);
  });

  it('returns [] when the allow-list is null, undefined, or empty', () => {
    expect(raceGrantedWeaponProficiencies(DWARF_TRAITS, null)).toEqual([]);
    expect(raceGrantedWeaponProficiencies(DWARF_TRAITS, undefined)).toEqual([]);
    expect(raceGrantedWeaponProficiencies(DWARF_TRAITS, [])).toEqual([]);
    expect(raceGrantedWeaponProficiencies(DWARF_TRAITS, new Set())).toEqual([]);
  });

  it('returns [] for null/empty traits', () => {
    expect(raceGrantedWeaponProficiencies(null, WEAPON_IDS)).toEqual([]);
    expect(raceGrantedWeaponProficiencies(undefined, WEAPON_IDS)).toEqual([]);
    expect(raceGrantedWeaponProficiencies('', WEAPON_IDS)).toEqual([]);
    expect(raceGrantedWeaponProficiencies([], WEAPON_IDS)).toEqual([]);
  });

  it('dedupes a weapon that appears in two traits, first-seen order', () => {
    const traits = [
      { name: 'A', description: '', mechanical_effect: 'proficiency_battleaxe' },
      { name: 'B', description: '', mechanical_effect: 'proficiency_handaxe' },
      { name: 'C', description: '', mechanical_effect: 'proficiency_battleaxe' },
    ];
    expect(raceGrantedWeaponProficiencies(traits, WEAPON_IDS)).toEqual(['battleaxe', 'handaxe']);
  });

  it('parses a JSON-string traits blob', () => {
    expect(raceGrantedWeaponProficiencies(JSON.stringify(DWARF_TRAITS), WEAPON_IDS)).toEqual([
      'battleaxe',
      'handaxe',
      'light-hammer',
      'warhammer',
    ]);
  });
});

describe('weaponProficiencyLabel', () => {
  it('title-cases a hyphenated weapon id', () => {
    expect(weaponProficiencyLabel('light-hammer')).toBe('Light Hammer');
  });

  it('title-cases a single-word weapon id', () => {
    expect(weaponProficiencyLabel('battleaxe')).toBe('Battleaxe');
  });

  it('returns "" for empty, null, and undefined', () => {
    expect(weaponProficiencyLabel('')).toBe('');
    expect(weaponProficiencyLabel(null)).toBe('');
    expect(weaponProficiencyLabel(undefined)).toBe('');
  });
});
