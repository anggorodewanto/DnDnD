import { describe, it, expect } from 'vitest';
import {
  parseJSONField,
  formatAbilityBonuses,
  parseTraits,
  formatDarkvision,
  subracePerks,
  applyAbilityBonuses,
} from './race-perks.js';

describe('parseJSONField', () => {
  it('returns objects/arrays unchanged', () => {
    expect(parseJSONField({ con: 2 })).toEqual({ con: 2 });
    expect(parseJSONField([1, 2])).toEqual([1, 2]);
  });

  it('parses JSON strings', () => {
    expect(parseJSONField('{"con":2}')).toEqual({ con: 2 });
    expect(parseJSONField('[{"name":"x"}]')).toEqual([{ name: 'x' }]);
  });

  it('returns null for null/undefined', () => {
    expect(parseJSONField(null)).toBeNull();
    expect(parseJSONField(undefined)).toBeNull();
  });

  it('returns null on parse failure', () => {
    expect(parseJSONField('not-json')).toBeNull();
    expect(parseJSONField('{broken')).toBeNull();
  });

  it('returns null for empty string', () => {
    expect(parseJSONField('')).toBeNull();
  });
});

describe('formatAbilityBonuses', () => {
  it('formats a single bonus', () => {
    expect(formatAbilityBonuses({ con: 2 })).toBe('+2 CON');
  });

  it('formats multiple bonuses in canonical order', () => {
    expect(formatAbilityBonuses({ cha: 1, str: 2 })).toBe('+2 STR, +1 CHA');
    expect(formatAbilityBonuses({ str: 2, con: 1 })).toBe('+2 STR, +1 CON');
  });

  it('collapses exactly +1 to all six into a single phrase', () => {
    expect(
      formatAbilityBonuses({ str: 1, dex: 1, con: 1, int: 1, wis: 1, cha: 1 })
    ).toBe('+1 to all abilities');
  });

  it('does NOT collapse when not all six are +1', () => {
    expect(formatAbilityBonuses({ str: 2, dex: 1, con: 1, int: 1, wis: 1, cha: 1 }))
      .toBe('+2 STR, +1 DEX, +1 CON, +1 INT, +1 WIS, +1 CHA');
  });

  it('handles the half-elf choose construct', () => {
    expect(
      formatAbilityBonuses({
        cha: 2,
        choose: { count: 2, amount: 1, from: ['str', 'dex', 'con', 'int', 'wis'] },
      })
    ).toBe('+2 CHA, +1 to 2 abilities of your choice');
  });

  it('handles a choose construct with no fixed bonuses', () => {
    expect(
      formatAbilityBonuses({ choose: { count: 2, amount: 1, from: ['str', 'dex'] } })
    ).toBe('+1 to 2 abilities of your choice');
  });

  it('accepts a JSON string', () => {
    expect(formatAbilityBonuses('{"con":2}')).toBe('+2 CON');
  });

  it('returns "" for empty/none', () => {
    expect(formatAbilityBonuses(null)).toBe('');
    expect(formatAbilityBonuses(undefined)).toBe('');
    expect(formatAbilityBonuses({})).toBe('');
    expect(formatAbilityBonuses('')).toBe('');
    expect(formatAbilityBonuses('not-json')).toBe('');
  });
});

describe('parseTraits', () => {
  it('returns [{name, description}], dropping mechanical_effect', () => {
    const raw = [
      { name: 'Lucky', description: 'Reroll 1s.', mechanical_effect: 'reroll_natural_1' },
      { name: 'Brave', description: 'Adv vs frightened.', mechanical_effect: 'x' },
    ];
    expect(parseTraits(raw)).toEqual([
      { name: 'Lucky', description: 'Reroll 1s.' },
      { name: 'Brave', description: 'Adv vs frightened.' },
    ]);
  });

  it('accepts a JSON string', () => {
    const raw = JSON.stringify([
      { name: 'Trance', description: 'Meditate 4h.', mechanical_effect: 'long_rest_4_hours' },
    ]);
    expect(parseTraits(raw)).toEqual([{ name: 'Trance', description: 'Meditate 4h.' }]);
  });

  it('returns [] for none/invalid', () => {
    expect(parseTraits(null)).toEqual([]);
    expect(parseTraits(undefined)).toEqual([]);
    expect(parseTraits('not-json')).toEqual([]);
    expect(parseTraits({})).toEqual([]);
    expect(parseTraits([])).toEqual([]);
  });

  it('tolerates entries missing description', () => {
    expect(parseTraits([{ name: 'X' }])).toEqual([{ name: 'X', description: '' }]);
  });
});

describe('formatDarkvision', () => {
  it('returns "" for falsy/0', () => {
    expect(formatDarkvision(0)).toBe('');
    expect(formatDarkvision(null)).toBe('');
    expect(formatDarkvision(undefined)).toBe('');
  });

  it('returns "<ft> ft" otherwise', () => {
    expect(formatDarkvision(60)).toBe('60 ft');
    expect(formatDarkvision(120)).toBe('120 ft');
  });
});

describe('subracePerks', () => {
  const race = {
    id: 'dwarf',
    subraces: [
      {
        id: 'hill-dwarf',
        name: 'Hill Dwarf',
        ability_bonuses: { wis: 1 },
        traits: [
          {
            name: 'Dwarven Toughness',
            description: 'HP +1 per level.',
            mechanical_effect: 'hp_plus_1_per_level',
          },
        ],
      },
    ],
  };

  it('returns ability bonuses (raw) and parsed traits for a found subrace', () => {
    expect(subracePerks(race, 'hill-dwarf')).toEqual({
      abilityBonuses: { wis: 1 },
      traits: [{ name: 'Dwarven Toughness', description: 'HP +1 per level.' }],
    });
  });

  it('feeds the raw abilityBonuses back into formatAbilityBonuses', () => {
    const perks = subracePerks(race, 'hill-dwarf');
    expect(formatAbilityBonuses(perks.abilityBonuses)).toBe('+1 WIS');
  });

  it('parses subraces from a JSON string blob', () => {
    const strRace = { id: 'elf', subraces: JSON.stringify(race.subraces) };
    const perks = subracePerks(strRace, 'hill-dwarf');
    expect(perks.abilityBonuses).toEqual({ wis: 1 });
  });

  it('returns null for a missing subrace id', () => {
    expect(subracePerks(race, 'mountain-dwarf')).toBeNull();
  });

  it('returns null when race has no subraces', () => {
    expect(subracePerks({ id: 'human' }, 'anything')).toBeNull();
    expect(subracePerks(null, 'x')).toBeNull();
  });

  it('returns null abilityBonuses when subrace omits them', () => {
    const r = { id: 'x', subraces: [{ id: 'plain', name: 'Plain' }] };
    expect(subracePerks(r, 'plain')).toEqual({ abilityBonuses: null, traits: [] });
  });
});

describe('applyAbilityBonuses', () => {
  const base = { str: 8, dex: 15, con: 13, int: 14, wis: 12, cha: 10 };

  it('adds a single race bonus set', () => {
    expect(applyAbilityBonuses(base, { dex: 2 })).toEqual({
      str: 8, dex: 17, con: 13, int: 14, wis: 12, cha: 10,
    });
  });

  it('stacks race and subrace bonuses (High Elf: +2 DEX, +1 INT)', () => {
    expect(applyAbilityBonuses(base, { dex: 2 }, { int: 1 })).toEqual({
      str: 8, dex: 17, con: 13, int: 15, wis: 12, cha: 10,
    });
  });

  it('ignores null/non-object bonus sets and non-ability keys', () => {
    expect(applyAbilityBonuses(base, null, undefined, { choose: { count: 2 }, con: 1 })).toEqual({
      str: 8, dex: 15, con: 14, int: 14, wis: 12, cha: 10,
    });
  });

  it('coerces missing/NaN base scores to 0 and returns only the six abilities', () => {
    expect(applyAbilityBonuses({ str: 10 }, { str: 2 })).toEqual({
      str: 12, dex: 0, con: 0, int: 0, wis: 0, cha: 0,
    });
  });
});
