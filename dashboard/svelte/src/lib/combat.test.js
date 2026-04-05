import { describe, it, expect } from 'vitest';
import {
  applyDamage,
  applyHealing,
  healthTier,
  STANDARD_CONDITIONS,
  addCondition,
  removeCondition,
  colToIndex,
  tokenOpacity,
} from './combat.js';

// TDD Cycle 1: applyDamage respects temp HP
describe('applyDamage', () => {
  it('reduces temp HP first before current HP', () => {
    const result = applyDamage({ hp_current: 20, hp_max: 20, temp_hp: 5 }, 8);
    expect(result.hp_current).toBe(17);
    expect(result.temp_hp).toBe(0);
    expect(result.is_alive).toBe(true);
  });

  it('subtracts only from current HP when no temp HP', () => {
    const result = applyDamage({ hp_current: 20, hp_max: 20, temp_hp: 0 }, 5);
    expect(result.hp_current).toBe(15);
    expect(result.temp_hp).toBe(0);
    expect(result.is_alive).toBe(true);
  });

  it('marks dead when HP reaches 0', () => {
    const result = applyDamage({ hp_current: 5, hp_max: 20, temp_hp: 0 }, 10);
    expect(result.hp_current).toBe(0);
    expect(result.is_alive).toBe(false);
  });

  it('does not go below 0', () => {
    const result = applyDamage({ hp_current: 3, hp_max: 20, temp_hp: 0 }, 100);
    expect(result.hp_current).toBe(0);
  });

  it('handles 0 damage', () => {
    const result = applyDamage({ hp_current: 20, hp_max: 20, temp_hp: 5 }, 0);
    expect(result.hp_current).toBe(20);
    expect(result.temp_hp).toBe(5);
  });

  it('damage exactly equal to temp HP', () => {
    const result = applyDamage({ hp_current: 20, hp_max: 20, temp_hp: 5 }, 5);
    expect(result.hp_current).toBe(20);
    expect(result.temp_hp).toBe(0);
  });
});

// TDD Cycle 2: applyHealing
describe('applyHealing', () => {
  it('adds healing up to hp_max', () => {
    const result = applyHealing({ hp_current: 10, hp_max: 20 }, 5);
    expect(result.hp_current).toBe(15);
    expect(result.is_alive).toBe(true);
  });

  it('caps at hp_max', () => {
    const result = applyHealing({ hp_current: 18, hp_max: 20 }, 10);
    expect(result.hp_current).toBe(20);
  });

  it('handles 0 healing', () => {
    const result = applyHealing({ hp_current: 10, hp_max: 20 }, 0);
    expect(result.hp_current).toBe(10);
  });

  it('revives from 0 HP', () => {
    const result = applyHealing({ hp_current: 0, hp_max: 20 }, 1);
    expect(result.hp_current).toBe(1);
    expect(result.is_alive).toBe(true);
  });
});

// TDD Cycle 3: healthTier
describe('healthTier', () => {
  it('returns healthy above 75%', () => {
    expect(healthTier(20, 20)).toBe('healthy');
    expect(healthTier(16, 20)).toBe('healthy');
  });

  it('returns wounded between 50-75%', () => {
    expect(healthTier(15, 20)).toBe('wounded');
    expect(healthTier(11, 20)).toBe('wounded');
  });

  it('returns bloodied between 25-50%', () => {
    expect(healthTier(10, 20)).toBe('bloodied');
    expect(healthTier(6, 20)).toBe('bloodied');
  });

  it('returns critical at 25% or below', () => {
    expect(healthTier(5, 20)).toBe('critical');
    expect(healthTier(1, 20)).toBe('critical');
  });

  it('returns dead at 0', () => {
    expect(healthTier(0, 20)).toBe('dead');
  });

  it('handles 0 max HP', () => {
    expect(healthTier(0, 0)).toBe('dead');
  });
});

// TDD Cycle 4: addCondition / removeCondition
describe('addCondition', () => {
  it('adds a condition to empty array', () => {
    expect(addCondition([], 'Blinded')).toEqual(['Blinded']);
  });

  it('does not add duplicate', () => {
    expect(addCondition(['Blinded'], 'Blinded')).toEqual(['Blinded']);
  });

  it('adds to existing conditions', () => {
    expect(addCondition(['Blinded'], 'Prone')).toEqual(['Blinded', 'Prone']);
  });
});

describe('removeCondition', () => {
  it('removes a condition', () => {
    expect(removeCondition(['Blinded', 'Prone'], 'Blinded')).toEqual(['Prone']);
  });

  it('returns same array if condition not found', () => {
    expect(removeCondition(['Blinded'], 'Prone')).toEqual(['Blinded']);
  });

  it('returns empty array when removing last condition', () => {
    expect(removeCondition(['Blinded'], 'Blinded')).toEqual([]);
  });
});

// TDD Cycle 5: colToIndex
describe('colToIndex', () => {
  it('converts A to 0', () => {
    expect(colToIndex('A')).toBe(0);
  });

  it('converts Z to 25', () => {
    expect(colToIndex('Z')).toBe(25);
  });

  it('converts AA to 26', () => {
    expect(colToIndex('AA')).toBe(26);
  });

  it('handles empty/null', () => {
    expect(colToIndex('')).toBe(0);
    expect(colToIndex(null)).toBe(0);
  });
});

// TDD Cycle 6: tokenOpacity
describe('tokenOpacity', () => {
  it('returns 1.0 for visible combatants', () => {
    expect(tokenOpacity({ is_visible: true })).toBe(1.0);
  });

  it('returns 0.4 for invisible combatants', () => {
    expect(tokenOpacity({ is_visible: false })).toBe(0.4);
  });

  it('returns 1.0 when is_visible is undefined', () => {
    expect(tokenOpacity({})).toBe(1.0);
  });
});

// TDD Cycle 7: STANDARD_CONDITIONS
describe('STANDARD_CONDITIONS', () => {
  it('contains 14 standard 5e conditions', () => {
    expect(STANDARD_CONDITIONS).toHaveLength(14);
    expect(STANDARD_CONDITIONS).toContain('Blinded');
    expect(STANDARD_CONDITIONS).toContain('Unconscious');
  });
});
