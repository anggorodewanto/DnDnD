import { describe, it, expect } from 'vitest';
import {
  applyDamage,
  applyHealing,
  healthTier,
  STANDARD_CONDITIONS,
  addCondition,
  removeCondition,
  colToIndex,
  indexToCol,
  tokenOpacity,
  gridDistance,
  tilesInRange,
  isWallBetween,
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

// TDD Cycle 8: gridDistance (Chebyshev * 5ft)
describe('gridDistance', () => {
  it('returns 0 for same tile', () => {
    expect(gridDistance(0, 0, 0, 0)).toBe(0);
  });

  it('returns 5 for adjacent cardinal tile', () => {
    expect(gridDistance(0, 0, 1, 0)).toBe(5);
    expect(gridDistance(0, 0, 0, 1)).toBe(5);
  });

  it('returns 5 for adjacent diagonal tile (Chebyshev)', () => {
    expect(gridDistance(0, 0, 1, 1)).toBe(5);
  });

  it('calculates longer distances', () => {
    expect(gridDistance(0, 0, 3, 4)).toBe(20); // max(3,4) * 5
  });

  it('handles negative direction', () => {
    expect(gridDistance(5, 5, 2, 3)).toBe(15); // max(3,2) * 5
  });
});

// TDD Cycle 9: indexToCol (reverse of colToIndex)
describe('indexToCol', () => {
  it('converts 0 to A', () => {
    expect(indexToCol(0)).toBe('A');
  });

  it('converts 25 to Z', () => {
    expect(indexToCol(25)).toBe('Z');
  });

  it('converts 26 to AA', () => {
    expect(indexToCol(26)).toBe('AA');
  });

  it('roundtrips with colToIndex', () => {
    for (const col of ['A', 'B', 'Z', 'AA', 'AB']) {
      expect(indexToCol(colToIndex(col))).toBe(col);
    }
  });
});

// TDD Cycle 10: tilesInRange
describe('tilesInRange', () => {
  it('returns tiles within Chebyshev distance', () => {
    const tiles = tilesInRange(2, 2, 1, 5, 5);
    expect(tiles).toContainEqual({ col: 1, row: 1 });
    expect(tiles).toContainEqual({ col: 3, row: 3 });
    expect(tiles).toHaveLength(8); // 3x3 - 1 (center)
  });

  it('clips to map bounds', () => {
    const tiles = tilesInRange(0, 0, 1, 3, 3);
    // Only 3 neighbors + not center
    expect(tiles).toContainEqual({ col: 1, row: 0 });
    expect(tiles).toContainEqual({ col: 0, row: 1 });
    expect(tiles).toContainEqual({ col: 1, row: 1 });
    expect(tiles).toHaveLength(3);
  });

  it('returns empty array for range 0', () => {
    expect(tilesInRange(2, 2, 0, 5, 5)).toEqual([]);
  });
});

// TDD Cycle 11: isWallBetween
describe('isWallBetween', () => {
  const tileSize = 48;

  it('detects horizontal wall blocking vertical movement', () => {
    // Horizontal wall at y=48 (between row 0 and row 1), x=0 to x=48
    const walls = [{ x: 0, y: 48, width: 48, height: 0 }];
    expect(isWallBetween(0, 0, 0, 1, walls, tileSize)).toBe(true);
  });

  it('detects vertical wall blocking horizontal movement', () => {
    // Vertical wall at x=48 (between col 0 and col 1), y=0 to y=48
    const walls = [{ x: 48, y: 0, width: 0, height: 48 }];
    expect(isWallBetween(0, 0, 1, 0, walls, tileSize)).toBe(true);
  });

  it('returns false when no wall blocks the path', () => {
    const walls = [{ x: 96, y: 0, width: 0, height: 48 }];
    expect(isWallBetween(0, 0, 1, 0, walls, tileSize)).toBe(false);
  });

  it('returns false with no walls', () => {
    expect(isWallBetween(0, 0, 1, 0, [], tileSize)).toBe(false);
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
