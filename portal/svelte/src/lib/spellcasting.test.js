import { describe, it, expect } from 'vitest';
import {
  spellcastingAbilityForClass,
  isSpellcaster,
  spellPrepCap,
  levelsUpTo,
} from './spellcasting.js';

describe('spellcastingAbilityForClass', () => {
  it('maps caster classes to their ability', () => {
    expect(spellcastingAbilityForClass('wizard')).toBe('int');
    expect(spellcastingAbilityForClass('cleric')).toBe('wis');
    expect(spellcastingAbilityForClass('druid')).toBe('wis');
    expect(spellcastingAbilityForClass('ranger')).toBe('wis');
    expect(spellcastingAbilityForClass('bard')).toBe('cha');
    expect(spellcastingAbilityForClass('sorcerer')).toBe('cha');
    expect(spellcastingAbilityForClass('paladin')).toBe('cha');
    expect(spellcastingAbilityForClass('warlock')).toBe('cha');
  });

  it('is case-insensitive', () => {
    expect(spellcastingAbilityForClass('Wizard')).toBe('int');
  });

  it('returns null for non-casters and empty input', () => {
    expect(spellcastingAbilityForClass('fighter')).toBe(null);
    expect(spellcastingAbilityForClass('barbarian')).toBe(null);
    expect(spellcastingAbilityForClass('')).toBe(null);
    expect(spellcastingAbilityForClass(undefined)).toBe(null);
  });
});

describe('isSpellcaster', () => {
  it('is true for casters, false otherwise', () => {
    expect(isSpellcaster('wizard')).toBe(true);
    expect(isSpellcaster('rogue')).toBe(false);
  });
});

describe('spellPrepCap', () => {
  it('is ability mod + level', () => {
    expect(spellPrepCap(3, 1)).toBe(4);
    expect(spellPrepCap(4, 5)).toBe(9);
  });
  it('floors at 1', () => {
    expect(spellPrepCap(-1, 1)).toBe(1);
    expect(spellPrepCap(0, 0)).toBe(1);
  });
  it('coerces non-numbers', () => {
    expect(spellPrepCap(undefined, undefined)).toBe(1);
  });
});

describe('levelsUpTo', () => {
  it('returns [1..max]', () => {
    expect(levelsUpTo(3)).toEqual([1, 2, 3]);
    expect(levelsUpTo(1)).toEqual([1]);
  });
  it('returns [] for no leveled slots', () => {
    expect(levelsUpTo(0)).toEqual([]);
    expect(levelsUpTo(null)).toEqual([]);
    expect(levelsUpTo(undefined)).toEqual([]);
  });
});
