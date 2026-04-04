import { describe, it, expect } from 'vitest';
import {
  scoreCost,
  totalCost,
  remainingPoints,
  abilityModifier,
  canIncrement,
  canDecrement,
} from './pointbuy.js';

describe('scoreCost', () => {
  it('returns 0 for score 8', () => {
    expect(scoreCost(8)).toBe(0);
  });

  it('returns costs for 8-13 (1pt each)', () => {
    expect(scoreCost(9)).toBe(1);
    expect(scoreCost(10)).toBe(2);
    expect(scoreCost(11)).toBe(3);
    expect(scoreCost(12)).toBe(4);
    expect(scoreCost(13)).toBe(5);
  });

  it('returns 7 for score 14 (5 + 2)', () => {
    expect(scoreCost(14)).toBe(7);
  });

  it('returns 9 for score 15 (5 + 4)', () => {
    expect(scoreCost(15)).toBe(9);
  });

  it('returns 0 for out-of-range scores', () => {
    expect(scoreCost(7)).toBe(0);
    expect(scoreCost(16)).toBe(0);
  });
});

describe('totalCost', () => {
  it('returns 0 for all 8s', () => {
    expect(totalCost({ str: 8, dex: 8, con: 8, int: 8, wis: 8, cha: 8 })).toBe(0);
  });

  it('returns 27 for standard array equivalent', () => {
    expect(totalCost({ str: 15, dex: 14, con: 13, int: 12, wis: 10, cha: 8 })).toBe(27);
  });
});

describe('remainingPoints', () => {
  it('returns 27 for all 8s', () => {
    expect(remainingPoints({ str: 8, dex: 8, con: 8, int: 8, wis: 8, cha: 8 })).toBe(27);
  });

  it('returns 0 when exactly 27 spent', () => {
    expect(remainingPoints({ str: 15, dex: 14, con: 13, int: 12, wis: 10, cha: 8 })).toBe(0);
  });
});

describe('abilityModifier', () => {
  it('calculates modifiers correctly', () => {
    expect(abilityModifier(8)).toBe(-1);
    expect(abilityModifier(10)).toBe(0);
    expect(abilityModifier(14)).toBe(2);
    expect(abilityModifier(15)).toBe(2);
    expect(abilityModifier(20)).toBe(5);
  });
});

describe('canIncrement', () => {
  it('allows increment when points available', () => {
    const scores = { str: 8, dex: 8, con: 8, int: 8, wis: 8, cha: 8 };
    expect(canIncrement(scores, 'str')).toBe(true);
  });

  it('prevents increment above 15', () => {
    const scores = { str: 15, dex: 8, con: 8, int: 8, wis: 8, cha: 8 };
    expect(canIncrement(scores, 'str')).toBe(false);
  });

  it('prevents increment when not enough points', () => {
    // 27 spent: can't increment anything
    const scores = { str: 15, dex: 14, con: 13, int: 12, wis: 10, cha: 8 };
    expect(canIncrement(scores, 'cha')).toBe(false);
  });
});

describe('canDecrement', () => {
  it('allows decrement above 8', () => {
    const scores = { str: 10, dex: 8, con: 8, int: 8, wis: 8, cha: 8 };
    expect(canDecrement(scores, 'str')).toBe(true);
  });

  it('prevents decrement at 8', () => {
    const scores = { str: 8, dex: 8, con: 8, int: 8, wis: 8, cha: 8 };
    expect(canDecrement(scores, 'str')).toBe(false);
  });
});
