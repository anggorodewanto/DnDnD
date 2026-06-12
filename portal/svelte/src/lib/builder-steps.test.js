import { describe, it, expect } from 'vitest';
import {
  SPELLS_STEP,
  nextStep,
  prevStep,
  isStepVisible,
  spellStepState,
} from './builder-steps.js';

const TOTAL = 7;

describe('SPELLS_STEP', () => {
  it('is the index of the Spells step', () => {
    expect(SPELLS_STEP).toBe(5);
  });
});

describe('nextStep', () => {
  it('increments by 1 for casters', () => {
    expect(nextStep(4, TOTAL, true)).toBe(5);
    expect(nextStep(5, TOTAL, true)).toBe(6);
    expect(nextStep(0, TOTAL, true)).toBe(1);
  });

  it('skips the Spells step for non-casters', () => {
    expect(nextStep(4, TOTAL, false)).toBe(6);
  });

  it('still advances normally before the Spells step for non-casters', () => {
    expect(nextStep(0, TOTAL, false)).toBe(1);
  });

  it('clamps at total-1', () => {
    expect(nextStep(6, TOTAL, true)).toBe(6);
    expect(nextStep(6, TOTAL, false)).toBe(6);
  });
});

describe('prevStep', () => {
  it('decrements by 1 for casters', () => {
    expect(prevStep(6, true)).toBe(5);
    expect(prevStep(5, true)).toBe(4);
  });

  it('skips the Spells step for non-casters', () => {
    expect(prevStep(6, false)).toBe(4);
  });

  it('still decrements normally before the Spells step for non-casters', () => {
    expect(prevStep(1, false)).toBe(0);
  });

  it('clamps at 0', () => {
    expect(prevStep(0, true)).toBe(0);
    expect(prevStep(0, false)).toBe(0);
  });
});

describe('isStepVisible', () => {
  it('shows every step for casters', () => {
    for (let i = 0; i < TOTAL; i++) {
      expect(isStepVisible(i, true)).toBe(true);
    }
  });

  it('hides only the Spells step for non-casters', () => {
    for (let i = 0; i < TOTAL; i++) {
      expect(isStepVisible(i, false)).toBe(i !== SPELLS_STEP);
    }
  });
});

describe('spellStepState', () => {
  it('is not-caster for non-casters regardless of other fields', () => {
    expect(
      spellStepState({ isCaster: false, loading: true, error: 'boom', count: 5 })
    ).toBe('not-caster');
    expect(
      spellStepState({ isCaster: false, loading: false, error: false, count: 0 })
    ).toBe('not-caster');
  });

  it('is error for casters when error is truthy, even while loading', () => {
    expect(
      spellStepState({ isCaster: true, loading: true, error: 'boom', count: 0 })
    ).toBe('error');
    expect(
      spellStepState({ isCaster: true, loading: false, error: true, count: 3 })
    ).toBe('error');
  });

  it('is loading for casters while loading with no error', () => {
    expect(
      spellStepState({ isCaster: true, loading: true, error: false, count: 0 })
    ).toBe('loading');
  });

  it('is empty for casters with no spells, not loading, no error', () => {
    expect(
      spellStepState({ isCaster: true, loading: false, error: false, count: 0 })
    ).toBe('empty');
  });

  it('is ready for casters with spells available', () => {
    expect(
      spellStepState({ isCaster: true, loading: false, error: false, count: 3 })
    ).toBe('ready');
  });
});
