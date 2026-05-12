import { describe, it, expect } from 'vitest';
import {
  subraceOptions,
  subclassOptions,
  isSubclassEligible,
  emptyClassRow,
  addClassRow,
  removeClassRow,
  updateClassRow,
  totalLevel,
} from './builder-options.js';

describe('subraceOptions', () => {
  it('returns [] for race with no subraces', () => {
    expect(subraceOptions({ id: 'human', name: 'Human' })).toEqual([]);
    expect(subraceOptions(null)).toEqual([]);
  });

  it('parses array of subrace objects', () => {
    const race = {
      id: 'elf',
      subraces: [
        { id: 'high-elf', name: 'High Elf' },
        { id: 'wood-elf', name: 'Wood Elf' },
      ],
    };
    expect(subraceOptions(race)).toEqual([
      { id: 'high-elf', name: 'High Elf' },
      { id: 'wood-elf', name: 'Wood Elf' },
    ]);
  });

  it('parses JSON string blob', () => {
    const race = {
      id: 'dwarf',
      subraces: JSON.stringify([{ id: 'hill-dwarf', name: 'Hill Dwarf' }]),
    };
    expect(subraceOptions(race)).toEqual([
      { id: 'hill-dwarf', name: 'Hill Dwarf' },
    ]);
  });

  it('tolerates malformed json', () => {
    const race = { id: 'x', subraces: 'not-json' };
    expect(subraceOptions(race)).toEqual([]);
  });
});

describe('subclassOptions', () => {
  it('returns [] for class with no subclasses', () => {
    expect(subclassOptions({ id: 'fighter' })).toEqual([]);
    expect(subclassOptions(null)).toEqual([]);
  });

  it('parses object keyed by subclass id', () => {
    const cls = {
      id: 'fighter',
      subclasses: {
        champion: { name: 'Champion' },
        'battle-master': { name: 'Battle Master' },
      },
    };
    const opts = subclassOptions(cls);
    expect(opts).toEqual(
      expect.arrayContaining([
        { id: 'champion', name: 'Champion' },
        { id: 'battle-master', name: 'Battle Master' },
      ])
    );
  });

  it('parses JSON string blob', () => {
    const cls = {
      id: 'wizard',
      subclasses: JSON.stringify({ evocation: { name: 'School of Evocation' } }),
    };
    expect(subclassOptions(cls)).toEqual([
      { id: 'evocation', name: 'School of Evocation' },
    ]);
  });
});

describe('isSubclassEligible', () => {
  it('false when class has no threshold', () => {
    expect(isSubclassEligible({ id: 'x' }, 5)).toBe(false);
    expect(isSubclassEligible(null, 5)).toBe(false);
  });

  it('respects subclass_level threshold', () => {
    const fighter = { id: 'fighter', subclass_level: 3 };
    expect(isSubclassEligible(fighter, 1)).toBe(false);
    expect(isSubclassEligible(fighter, 2)).toBe(false);
    expect(isSubclassEligible(fighter, 3)).toBe(true);
    expect(isSubclassEligible(fighter, 5)).toBe(true);
  });

  it('handles cleric/sorcerer (level 1)', () => {
    const cleric = { id: 'cleric', subclass_level: 1 };
    expect(isSubclassEligible(cleric, 1)).toBe(true);
  });
});

describe('class row helpers', () => {
  it('emptyClassRow returns blank row', () => {
    expect(emptyClassRow()).toEqual({ class: '', level: 1, subclass: '' });
  });

  it('addClassRow appends a blank row', () => {
    const start = [{ class: 'fighter', level: 5, subclass: 'champion' }];
    const next = addClassRow(start);
    expect(next).toHaveLength(2);
    expect(next[0]).toEqual(start[0]);
    expect(next[1]).toEqual({ class: '', level: 1, subclass: '' });
  });

  it('removeClassRow removes non-first row', () => {
    const rows = [
      { class: 'fighter', level: 5, subclass: '' },
      { class: 'wizard', level: 3, subclass: '' },
    ];
    expect(removeClassRow(rows, 1)).toEqual([rows[0]]);
  });

  it('removeClassRow refuses to remove the only row', () => {
    const rows = [{ class: 'fighter', level: 5, subclass: '' }];
    expect(removeClassRow(rows, 0)).toEqual(rows);
  });

  it('removeClassRow refuses to remove the first row of many', () => {
    const rows = [
      { class: 'fighter', level: 5, subclass: '' },
      { class: 'wizard', level: 3, subclass: '' },
    ];
    expect(removeClassRow(rows, 0)).toEqual(rows);
  });

  it('updateClassRow patches a single row', () => {
    const rows = [
      { class: 'fighter', level: 5, subclass: '' },
      { class: 'wizard', level: 3, subclass: '' },
    ];
    const next = updateClassRow(rows, 1, { subclass: 'evocation' });
    expect(next[0]).toBe(rows[0]); // untouched reference
    expect(next[1]).toEqual({ class: 'wizard', level: 3, subclass: 'evocation' });
  });

  it('totalLevel sums and caps at 20', () => {
    expect(totalLevel([
      { class: 'fighter', level: 11 },
      { class: 'wizard', level: 5 },
    ])).toBe(16);
    expect(totalLevel([
      { class: 'fighter', level: 15 },
      { class: 'wizard', level: 10 },
    ])).toBe(20);
  });
});
