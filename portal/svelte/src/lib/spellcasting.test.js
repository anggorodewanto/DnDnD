import { describe, it, expect } from 'vitest';
import {
  spellcastingAbilityForClass,
  isSpellcaster,
  spellPrepCap,
  levelsUpTo,
  cantripsKnown,
  spellsKnown,
  leveledSpellCap,
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

describe('cantripsKnown', () => {
  it('returns the per-level cantrip count for cantrip classes', () => {
    expect(cantripsKnown('wizard', 1)).toBe(3);
    expect(cantripsKnown('wizard', 4)).toBe(4);
    expect(cantripsKnown('wizard', 10)).toBe(5);
    expect(cantripsKnown('bard', 1)).toBe(2);
    expect(cantripsKnown('cleric', 1)).toBe(3);
    expect(cantripsKnown('druid', 1)).toBe(2);
    expect(cantripsKnown('sorcerer', 1)).toBe(4);
    expect(cantripsKnown('warlock', 4)).toBe(3);
  });
  it('is case-insensitive and floors the level at 1', () => {
    expect(cantripsKnown('Wizard', 0)).toBe(3);
  });
  it('returns 0 for classes without cantrips', () => {
    expect(cantripsKnown('paladin', 5)).toBe(0);
    expect(cantripsKnown('ranger', 5)).toBe(0);
    expect(cantripsKnown('fighter', 5)).toBe(0);
    expect(cantripsKnown('', 1)).toBe(0);
  });
});

describe('spellsKnown', () => {
  it('returns the Spells Known count for known casters', () => {
    expect(spellsKnown('bard', 1)).toBe(4);
    expect(spellsKnown('bard', 20)).toBe(22);
    expect(spellsKnown('sorcerer', 1)).toBe(2);
    expect(spellsKnown('ranger', 1)).toBe(0);
    expect(spellsKnown('ranger', 2)).toBe(2);
    expect(spellsKnown('warlock', 1)).toBe(2);
  });
  it('clamps the level into [1,20]', () => {
    expect(spellsKnown('bard', 0)).toBe(4);
    expect(spellsKnown('bard', 99)).toBe(22);
  });
  it('returns null for prepared casters and non-casters', () => {
    expect(spellsKnown('wizard', 1)).toBe(null);
    expect(spellsKnown('cleric', 1)).toBe(null);
    expect(spellsKnown('fighter', 1)).toBe(null);
  });
});

describe('leveledSpellCap', () => {
  it('uses ability mod + level for full prepared casters', () => {
    expect(leveledSpellCap('wizard', 1, 3)).toBe(4);
    expect(leveledSpellCap('cleric', 1, 0)).toBe(1);
    expect(leveledSpellCap('wizard', 1, -1)).toBe(1); // floors at 1
  });
  it('uses the Spells Known table for known casters (ability mod ignored)', () => {
    expect(leveledSpellCap('bard', 1, 5)).toBe(4);
    expect(leveledSpellCap('sorcerer', 1, 0)).toBe(2);
    expect(leveledSpellCap('ranger', 1, 5)).toBe(0);
  });
  it('treats paladins as half-casters with no spells before level 2', () => {
    expect(leveledSpellCap('paladin', 1, 3)).toBe(0);
    expect(leveledSpellCap('paladin', 2, 2)).toBe(3); // max(1, 2 + 2/2)
  });
  it('returns 0 for non-casters', () => {
    expect(leveledSpellCap('fighter', 5, 5)).toBe(0);
  });
});
