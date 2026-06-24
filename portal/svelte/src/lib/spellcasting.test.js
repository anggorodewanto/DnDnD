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

describe('third-caster subclasses (Eldritch Knight / Arcane Trickster)', () => {
  describe('spellcastingAbilityForClass', () => {
    it('treats a Fighter/Eldritch Knight at level >= 3 as an INT caster', () => {
      expect(spellcastingAbilityForClass('fighter', 'eldritch-knight', 3)).toBe('int');
      expect(spellcastingAbilityForClass('fighter', 'Eldritch Knight', 3)).toBe('int');
    });
    it('treats a Rogue/Arcane Trickster at level >= 3 as an INT caster', () => {
      expect(spellcastingAbilityForClass('rogue', 'arcane-trickster', 3)).toBe('int');
      expect(spellcastingAbilityForClass('rogue', 'Arcane Trickster', 3)).toBe('int');
    });
    it('is not a caster below level 3 (subclass not yet chosen)', () => {
      expect(spellcastingAbilityForClass('fighter', 'eldritch-knight', 2)).toBe(null);
      expect(spellcastingAbilityForClass('rogue', 'arcane-trickster', 1)).toBe(null);
    });
    it('is not a caster without the EK/AT subclass', () => {
      expect(spellcastingAbilityForClass('fighter', '', 3)).toBe(null);
      expect(spellcastingAbilityForClass('fighter', 'champion', 3)).toBe(null);
      expect(spellcastingAbilityForClass('rogue', 'thief', 5)).toBe(null);
    });
    it('still maps base caster classes ignoring subclass', () => {
      expect(spellcastingAbilityForClass('wizard', 'evocation', 3)).toBe('int');
    });
  });

  describe('isSpellcaster', () => {
    it('is true for Fighter/EK and Rogue/AT at level >= 3', () => {
      expect(isSpellcaster('fighter', 'eldritch-knight', 3)).toBe(true);
      expect(isSpellcaster('rogue', 'arcane-trickster', 3)).toBe(true);
    });
    it('is false for a plain Fighter and a Fighter/EK at level 2', () => {
      expect(isSpellcaster('fighter')).toBe(false);
      expect(isSpellcaster('fighter', 'champion', 3)).toBe(false);
      expect(isSpellcaster('fighter', 'eldritch-knight', 2)).toBe(false);
    });
  });

  describe('cantripsKnown', () => {
    it('returns EK cantrips (2 at L3, 3 at L10)', () => {
      expect(cantripsKnown('fighter', 3, 'eldritch-knight')).toBe(2);
      expect(cantripsKnown('fighter', 9, 'eldritch-knight')).toBe(2);
      expect(cantripsKnown('fighter', 10, 'eldritch-knight')).toBe(3);
    });
    it('returns AT cantrips (3 at L3, 4 at L10)', () => {
      expect(cantripsKnown('rogue', 3, 'arcane-trickster')).toBe(3);
      expect(cantripsKnown('rogue', 10, 'arcane-trickster')).toBe(4);
    });
    it('returns 0 below level 3 and for non-EK/AT subclasses', () => {
      expect(cantripsKnown('fighter', 2, 'eldritch-knight')).toBe(0);
      expect(cantripsKnown('fighter', 5, 'champion')).toBe(0);
      expect(cantripsKnown('fighter', 5)).toBe(0);
    });
  });

  describe('leveledSpellCap', () => {
    it('uses the third-caster Spells Known table for EK/AT (ability mod ignored)', () => {
      expect(leveledSpellCap('fighter', 3, 5, 'eldritch-knight')).toBe(3);
      expect(leveledSpellCap('fighter', 4, 5, 'eldritch-knight')).toBe(4);
      expect(leveledSpellCap('fighter', 7, 5, 'eldritch-knight')).toBe(5);
      expect(leveledSpellCap('fighter', 20, 5, 'eldritch-knight')).toBe(13);
      expect(leveledSpellCap('rogue', 3, 0, 'arcane-trickster')).toBe(3);
    });
    it('returns 0 below level 3 and for non-EK/AT subclasses', () => {
      expect(leveledSpellCap('fighter', 2, 5, 'eldritch-knight')).toBe(0);
      expect(leveledSpellCap('fighter', 5, 5, 'champion')).toBe(0);
      expect(leveledSpellCap('fighter', 5, 5)).toBe(0);
    });
  });
});
