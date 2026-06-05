import { describe, it, expect } from 'vitest';
import {
  countAgainstCap,
  isLevelSelectable,
  isSpellDisabled,
  disabledReason,
  toggleSelected,
  isSpellHidden,
  visibleSpells,
} from './spell-picker.js';

function spell(overrides = {}) {
  return {
    id: overrides.id ?? 'id',
    name: overrides.name ?? 'Spell',
    level: overrides.level ?? 1,
    school: overrides.school ?? 'evocation',
  };
}

describe('countAgainstCap', () => {
  it('counts all selected when none are always-prepared', () => {
    expect(countAgainstCap(['a', 'b', 'c'])).toBe(3);
  });

  it('excludes always-prepared ids from the count', () => {
    expect(countAgainstCap(['a', 'b', 'c'], ['b'])).toBe(2);
  });

  it('handles null/undefined inputs', () => {
    expect(countAgainstCap(null)).toBe(0);
    expect(countAgainstCap(undefined, undefined)).toBe(0);
  });
});

describe('isLevelSelectable', () => {
  it('cantrips are always selectable regardless of gate', () => {
    expect(isLevelSelectable(0, [])).toBe(true);
    expect(isLevelSelectable(0, [3])).toBe(true);
  });

  it('null gate allows every level', () => {
    expect(isLevelSelectable(5, null)).toBe(true);
    expect(isLevelSelectable(9, undefined)).toBe(true);
  });

  it('gates leveled spells by the selectable set (array or Set)', () => {
    expect(isLevelSelectable(1, [1, 2])).toBe(true);
    expect(isLevelSelectable(3, [1, 2])).toBe(false);
    expect(isLevelSelectable(2, new Set([2]))).toBe(true);
    expect(isLevelSelectable(2, new Set([1]))).toBe(false);
  });

  it('coerces numeric strings', () => {
    expect(isLevelSelectable('1', ['1'])).toBe(true);
  });
});

describe('isSpellDisabled', () => {
  it('always-prepared spells are locked (disabled)', () => {
    expect(isSpellDisabled(spell({ id: 'bless' }), { alwaysPrepared: ['bless'] })).toBe(true);
  });

  it('already-selected spells are never disabled (can be removed)', () => {
    expect(
      isSpellDisabled(spell({ id: 'a', level: 9 }), { selected: ['a'], max: 1, selectableLevels: [1] }),
    ).toBe(false);
  });

  it('disables un-castable levels even under the cap', () => {
    expect(
      isSpellDisabled(spell({ id: 'fireball', level: 3 }), { selected: [], max: 5, selectableLevels: [1] }),
    ).toBe(true);
  });

  it('disables once the cap is reached', () => {
    expect(
      isSpellDisabled(spell({ id: 'new', level: 1 }), { selected: ['a', 'b'], max: 2, selectableLevels: [1] }),
    ).toBe(true);
  });

  it('allows selection under the cap at a castable level', () => {
    expect(
      isSpellDisabled(spell({ id: 'new', level: 1 }), { selected: ['a'], max: 2, selectableLevels: [1] }),
    ).toBe(false);
  });

  it('always-prepared do not consume cap room', () => {
    // cap 1, one always-prepared selected, a fresh level-1 spell still selectable
    expect(
      isSpellDisabled(spell({ id: 'new', level: 1 }), {
        selected: ['always-1'],
        alwaysPrepared: ['always-1'],
        max: 1,
        selectableLevels: [1],
      }),
    ).toBe(false);
  });
});

describe('disabledReason', () => {
  it('reports always-prepared', () => {
    expect(disabledReason(spell({ id: 'bless' }), { alwaysPrepared: ['bless'] })).toMatch(/always prepared/i);
  });
  it('reports level lock', () => {
    expect(disabledReason(spell({ level: 3 }), { selectableLevels: [1] })).toMatch(/slot/i);
  });
  it('reports cap reached', () => {
    expect(disabledReason(spell({ id: 'x', level: 1 }), { selected: ['a'], max: 1, selectableLevels: [1] })).toMatch(/limit/i);
  });
  it('returns empty when selectable', () => {
    expect(disabledReason(spell({ id: 'x', level: 1 }), { selected: [], max: 5, selectableLevels: [1] })).toBe('');
  });
  it('returns empty for an already-selected spell', () => {
    expect(disabledReason(spell({ id: 'x', level: 9 }), { selected: ['x'], max: 1, selectableLevels: [1] })).toBe('');
  });
});

describe('isSpellHidden', () => {
  it('hides a spell that is disabled and not already on (uncastable level)', () => {
    expect(
      isSpellHidden(spell({ id: 'fireball', level: 3 }), { selected: [], max: 5, selectableLevels: [1] }),
    ).toBe(true);
  });

  it('hides a spell when the cap is reached and it is not selected', () => {
    expect(
      isSpellHidden(spell({ id: 'new', level: 1 }), { selected: ['a', 'b'], max: 2, selectableLevels: [1] }),
    ).toBe(true);
  });

  it('keeps a selectable spell visible', () => {
    expect(
      isSpellHidden(spell({ id: 'new', level: 1 }), { selected: ['a'], max: 5, selectableLevels: [1] }),
    ).toBe(false);
  });

  it('keeps an already-selected spell visible even when its level is uncastable', () => {
    expect(
      isSpellHidden(spell({ id: 'a', level: 9 }), { selected: ['a'], max: 1, selectableLevels: [1] }),
    ).toBe(false);
  });

  it('keeps always-prepared spells visible', () => {
    expect(
      isSpellHidden(spell({ id: 'bless' }), { selected: ['bless'], alwaysPrepared: ['bless'] }),
    ).toBe(false);
  });
});

describe('visibleSpells', () => {
  const fireball = spell({ id: 'fireball', level: 3 });
  const magicMissile = spell({ id: 'mm', level: 1 });
  const list = [magicMissile, fireball];

  it('returns the list unchanged when the toggle is off', () => {
    expect(visibleSpells(list, false, { selectableLevels: [1] })).toEqual(list);
  });

  it('drops unselectable spells when the toggle is on', () => {
    expect(visibleSpells(list, true, { selected: [], max: 5, selectableLevels: [1] })).toEqual([magicMissile]);
  });

  it('does not mutate the input list', () => {
    const input = [...list];
    visibleSpells(input, true, { selectableLevels: [1] });
    expect(input).toEqual(list);
  });

  it('handles null/undefined input', () => {
    expect(visibleSpells(null, true)).toEqual([]);
    expect(visibleSpells(undefined, false)).toEqual([]);
  });
});

describe('toggleSelected', () => {
  it('adds an id', () => {
    expect(toggleSelected(['a'], 'b')).toEqual(['a', 'b']);
  });
  it('removes an existing id', () => {
    expect(toggleSelected(['a', 'b'], 'a')).toEqual(['b']);
  });
  it('does not mutate the input array', () => {
    const input = ['a'];
    toggleSelected(input, 'b');
    expect(input).toEqual(['a']);
  });
  it('is a no-op for always-prepared ids', () => {
    expect(toggleSelected(['a'], 'bless', ['bless'])).toEqual(['a']);
  });
  it('handles null selected', () => {
    expect(toggleSelected(null, 'a')).toEqual(['a']);
  });
});
