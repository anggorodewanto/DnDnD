import { describe, it, expect } from 'vitest';
import {
  SCHOOL_ORDER,
  schoolLabel,
  spellMatchesFilters,
  filterSpells,
  groupSpellsBySchool,
  availableLevels,
} from './spell-filter.js';

/**
 * Builds a minimal spell object for tests.
 * @param {object} overrides
 * @returns {object}
 */
function spell(overrides = {}) {
  return {
    id: overrides.id ?? 'id',
    name: overrides.name ?? 'Spell',
    level: overrides.level ?? 0,
    school: overrides.school ?? 'evocation',
    classes: overrides.classes ?? [],
    ...overrides,
  };
}

describe('SCHOOL_ORDER', () => {
  it('lists the 8 schools in canonical order', () => {
    expect(SCHOOL_ORDER).toEqual([
      'abjuration',
      'conjuration',
      'divination',
      'enchantment',
      'evocation',
      'illusion',
      'necromancy',
      'transmutation',
    ]);
  });
});

describe('schoolLabel', () => {
  it('title-cases a known slug', () => {
    expect(schoolLabel('evocation')).toBe('Evocation');
  });

  it('returns Other for empty/null/undefined', () => {
    expect(schoolLabel('')).toBe('Other');
    expect(schoolLabel(null)).toBe('Other');
    expect(schoolLabel(undefined)).toBe('Other');
  });

  it('title-cases an unknown non-empty slug', () => {
    expect(schoolLabel('foo')).toBe('Foo');
  });
});

describe('spellMatchesFilters', () => {
  it('matches name substring case-insensitively', () => {
    const s = spell({ name: 'Fireball' });
    expect(spellMatchesFilters(s, { query: 'fire' })).toBe(true);
    expect(spellMatchesFilters(s, { query: 'BALL' })).toBe(true);
    expect(spellMatchesFilters(s, { query: 'ice' })).toBe(false);
  });

  it('trims the query before matching', () => {
    const s = spell({ name: 'Fireball' });
    expect(spellMatchesFilters(s, { query: '  fire  ' })).toBe(true);
  });

  it('matches everything when query is empty/null/undefined', () => {
    const s = spell({ name: 'Fireball' });
    expect(spellMatchesFilters(s, { query: '' })).toBe(true);
    expect(spellMatchesFilters(s, { query: null })).toBe(true);
    expect(spellMatchesFilters(s, { query: undefined })).toBe(true);
    expect(spellMatchesFilters(s, {})).toBe(true);
  });

  it('matches level given as a number', () => {
    const s = spell({ level: 3 });
    expect(spellMatchesFilters(s, { level: 3 })).toBe(true);
    expect(spellMatchesFilters(s, { level: 2 })).toBe(false);
  });

  it('matches level given as a numeric string', () => {
    const s = spell({ level: 3 });
    expect(spellMatchesFilters(s, { level: '3' })).toBe(true);
    expect(spellMatchesFilters(s, { level: '2' })).toBe(false);
  });

  it('matches all levels when level is empty/null/undefined', () => {
    const s = spell({ level: 3 });
    expect(spellMatchesFilters(s, { level: '' })).toBe(true);
    expect(spellMatchesFilters(s, { level: null })).toBe(true);
    expect(spellMatchesFilters(s, { level: undefined })).toBe(true);
    expect(spellMatchesFilters(s, {})).toBe(true);
  });

  it('treats level 0 (cantrip) as a real constraint', () => {
    const cantrip = spell({ level: 0 });
    const lvl1 = spell({ level: 1 });
    expect(spellMatchesFilters(cantrip, { level: 0 })).toBe(true);
    expect(spellMatchesFilters(lvl1, { level: 0 })).toBe(false);
    expect(spellMatchesFilters(cantrip, { level: '0' })).toBe(true);
  });

  it('requires both query AND level to pass', () => {
    const s = spell({ name: 'Fireball', level: 3 });
    expect(spellMatchesFilters(s, { query: 'fire', level: 3 })).toBe(true);
    expect(spellMatchesFilters(s, { query: 'fire', level: 1 })).toBe(false);
    expect(spellMatchesFilters(s, { query: 'ice', level: 3 })).toBe(false);
  });

  it('matches with no filters object keys at all', () => {
    const s = spell();
    expect(spellMatchesFilters(s, {})).toBe(true);
  });
});

describe('filterSpells', () => {
  it('returns only spells passing the filters', () => {
    const spells = [
      spell({ name: 'Fireball', level: 3 }),
      spell({ name: 'Frostbite', level: 0 }),
      spell({ name: 'Mage Hand', level: 0 }),
    ];
    const result = filterSpells(spells, { query: 'f', level: 0 });
    expect(result.map((s) => s.name)).toEqual(['Frostbite']);
  });

  it('returns a new array and does not mutate the input', () => {
    const spells = [spell({ name: 'A' }), spell({ name: 'B' })];
    const snapshot = [...spells];
    const result = filterSpells(spells, {});
    expect(result).not.toBe(spells);
    expect(result).toEqual(spells);
    expect(spells).toEqual(snapshot);
  });

  it('returns [] for null/undefined input', () => {
    expect(filterSpells(null, {})).toEqual([]);
    expect(filterSpells(undefined, {})).toEqual([]);
  });

  it('returns all spells when no filters are supplied', () => {
    const spells = [spell({ name: 'A' }), spell({ name: 'B' })];
    expect(filterSpells(spells, {})).toHaveLength(2);
  });
});

describe('groupSpellsBySchool', () => {
  it('orders groups by SCHOOL_ORDER and skips empty schools', () => {
    const spells = [
      spell({ name: 'Evoke', school: 'evocation' }),
      spell({ name: 'Abjure', school: 'abjuration' }),
      spell({ name: 'Conjure', school: 'conjuration' }),
    ];
    const groups = groupSpellsBySchool(spells);
    expect(groups.map((g) => g.school)).toEqual([
      'abjuration',
      'conjuration',
      'evocation',
    ]);
  });

  it('sets label via schoolLabel', () => {
    const groups = groupSpellsBySchool([spell({ school: 'evocation' })]);
    expect(groups[0].label).toBe('Evocation');
  });

  it('sorts spells within a group by level then name', () => {
    const spells = [
      spell({ name: 'Zebra', level: 1, school: 'evocation' }),
      spell({ name: 'Apple', level: 1, school: 'evocation' }),
      spell({ name: 'Cantrip', level: 0, school: 'evocation' }),
    ];
    const groups = groupSpellsBySchool(spells);
    expect(groups[0].spells.map((s) => s.name)).toEqual([
      'Cantrip',
      'Apple',
      'Zebra',
    ]);
  });

  it('does not mutate the input array or its objects', () => {
    const spells = [
      spell({ name: 'B', level: 1, school: 'evocation' }),
      spell({ name: 'A', level: 0, school: 'evocation' }),
    ];
    const snapshot = spells.map((s) => ({ ...s }));
    const order = [...spells];
    groupSpellsBySchool(spells);
    expect(spells).toEqual(order);
    expect(spells.map((s) => ({ ...s }))).toEqual(snapshot);
  });

  it('places unknown non-empty schools alphabetically after known schools', () => {
    const spells = [
      spell({ name: 'Z', school: 'zeta' }),
      spell({ name: 'A', school: 'alpha' }),
      spell({ name: 'E', school: 'evocation' }),
    ];
    const groups = groupSpellsBySchool(spells);
    expect(groups.map((g) => g.school)).toEqual(['evocation', 'alpha', 'zeta']);
    expect(groups.map((g) => g.label)).toEqual(['Evocation', 'Alpha', 'Zeta']);
  });

  it('collects empty/missing-school spells into a trailing Other group', () => {
    const spells = [
      spell({ name: 'Known', school: 'evocation' }),
      spell({ name: 'NoSchool', school: '' }),
      spell({ name: 'Missing', school: undefined }),
    ];
    const groups = groupSpellsBySchool(spells);
    const last = groups[groups.length - 1];
    expect(last.school).toBe('');
    expect(last.label).toBe('Other');
    expect(last.spells.map((s) => s.name).sort()).toEqual(['Missing', 'NoSchool']);
  });

  it('orders known schools, then unknown slugs, then Other', () => {
    const spells = [
      spell({ name: 'NoSchool', school: '' }),
      spell({ name: 'Weird', school: 'weird' }),
      spell({ name: 'Evoke', school: 'evocation' }),
    ];
    const groups = groupSpellsBySchool(spells);
    expect(groups.map((g) => g.school)).toEqual(['evocation', 'weird', '']);
  });

  it('returns [] for null/undefined input', () => {
    expect(groupSpellsBySchool(null)).toEqual([]);
    expect(groupSpellsBySchool(undefined)).toEqual([]);
  });
});

describe('availableLevels', () => {
  it('dedups and sorts levels ascending numerically', () => {
    const spells = [
      spell({ level: 0 }),
      spell({ level: 3 }),
      spell({ level: 0 }),
      spell({ level: 1 }),
    ];
    expect(availableLevels(spells)).toEqual([0, 1, 3]);
  });

  it('sorts numerically, not lexically', () => {
    const spells = [spell({ level: 10 }), spell({ level: 2 }), spell({ level: 9 })];
    expect(availableLevels(spells)).toEqual([2, 9, 10]);
  });

  it('returns [] for null/undefined input', () => {
    expect(availableLevels(null)).toEqual([]);
    expect(availableLevels(undefined)).toEqual([]);
  });
});
