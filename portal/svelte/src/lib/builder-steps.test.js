import { describe, it, expect } from 'vitest';
import {
  SPELLS_STEP,
  CLASS_FEATURES_STEP,
  nextStep,
  prevStep,
  isStepVisible,
  spellStepState,
} from './builder-steps.js';

// 8-step wizard: Basics, Class, Ability Scores, Skills, Equipment, Spells,
// Class Features, Review.
const TOTAL = 8;

const casterWarlock = { isCaster: true, hasClassFeatures: true };
const casterOnly = { isCaster: true, hasClassFeatures: false };
const warlockOnly = { isCaster: false, hasClassFeatures: true };
const neither = { isCaster: false, hasClassFeatures: false };

describe('step indices', () => {
  it('places Spells at 5 and Class Features at 6', () => {
    expect(SPELLS_STEP).toBe(5);
    expect(CLASS_FEATURES_STEP).toBe(6);
  });
});

describe('nextStep', () => {
  it('increments by 1 when the next step is visible', () => {
    expect(nextStep(4, TOTAL, casterWarlock)).toBe(5);
    expect(nextStep(5, TOTAL, casterWarlock)).toBe(6);
    expect(nextStep(6, TOTAL, casterWarlock)).toBe(7);
  });

  it('skips Spells for non-casters', () => {
    expect(nextStep(4, TOTAL, warlockOnly)).toBe(6);
  });

  it('skips Class Features when there are no class-feature choices', () => {
    expect(nextStep(5, TOTAL, casterOnly)).toBe(7);
  });

  it('skips both hidden steps for a non-caster without class features', () => {
    expect(nextStep(4, TOTAL, neither)).toBe(7);
  });

  it('clamps at total-1', () => {
    expect(nextStep(7, TOTAL, casterWarlock)).toBe(7);
    expect(nextStep(7, TOTAL, neither)).toBe(7);
  });
});

describe('prevStep', () => {
  it('decrements by 1 when the previous step is visible', () => {
    expect(prevStep(7, casterWarlock)).toBe(6);
    expect(prevStep(6, casterWarlock)).toBe(5);
  });

  it('skips Class Features when there are no class-feature choices', () => {
    expect(prevStep(7, casterOnly)).toBe(5);
  });

  it('skips Spells for non-casters', () => {
    expect(prevStep(6, warlockOnly)).toBe(4);
  });

  it('skips both hidden steps back to Equipment', () => {
    expect(prevStep(7, neither)).toBe(4);
  });

  it('clamps at 0', () => {
    expect(prevStep(0, casterWarlock)).toBe(0);
    expect(prevStep(0, neither)).toBe(0);
  });
});

describe('isStepVisible', () => {
  it('shows every step for a caster warlock', () => {
    for (let i = 0; i < TOTAL; i++) {
      expect(isStepVisible(i, casterWarlock)).toBe(true);
    }
  });

  it('hides only Spells for a non-caster with class features', () => {
    for (let i = 0; i < TOTAL; i++) {
      expect(isStepVisible(i, warlockOnly)).toBe(i !== SPELLS_STEP);
    }
  });

  it('hides only Class Features for a caster without class features', () => {
    for (let i = 0; i < TOTAL; i++) {
      expect(isStepVisible(i, casterOnly)).toBe(i !== CLASS_FEATURES_STEP);
    }
  });

  it('hides both for neither', () => {
    for (let i = 0; i < TOTAL; i++) {
      const hidden = i === SPELLS_STEP || i === CLASS_FEATURES_STEP;
      expect(isStepVisible(i, neither)).toBe(!hidden);
    }
  });

  it('null-guards a missing ctx to both-hidden', () => {
    expect(isStepVisible(SPELLS_STEP, undefined)).toBe(false);
    expect(isStepVisible(CLASS_FEATURES_STEP, undefined)).toBe(false);
    expect(isStepVisible(0, undefined)).toBe(true);
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
