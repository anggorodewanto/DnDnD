import { describe, it, expect } from 'vitest';
import { SKILL_ABILITY, abilityForSkill, abilityLabel } from './skills.js';

describe('SKILL_ABILITY', () => {
  it('maps all 18 D&D 5e skills to their ability', () => {
    expect(Object.keys(SKILL_ABILITY)).toHaveLength(18);
    expect(SKILL_ABILITY.stealth).toBe('dex');
    expect(SKILL_ABILITY.arcana).toBe('int');
    expect(SKILL_ABILITY['animal-handling']).toBe('wis');
    expect(SKILL_ABILITY['sleight-of-hand']).toBe('dex');
    expect(SKILL_ABILITY.persuasion).toBe('cha');
    expect(SKILL_ABILITY.athletics).toBe('str');
  });
});

describe('abilityForSkill', () => {
  it('returns the ability code for a known skill', () => {
    expect(abilityForSkill('stealth')).toBe('dex');
    expect(abilityForSkill('arcana')).toBe('int');
    expect(abilityForSkill('animal-handling')).toBe('wis');
  });

  it('returns "" for an unknown skill', () => {
    expect(abilityForSkill('basket-weaving')).toBe('');
  });

  it('returns "" for null/undefined/empty', () => {
    expect(abilityForSkill(null)).toBe('');
    expect(abilityForSkill(undefined)).toBe('');
    expect(abilityForSkill('')).toBe('');
  });
});

describe('abilityLabel', () => {
  it('returns the uppercased ability code for a known skill', () => {
    expect(abilityLabel('stealth')).toBe('DEX');
    expect(abilityLabel('arcana')).toBe('INT');
    expect(abilityLabel('persuasion')).toBe('CHA');
  });

  it('returns "" for an unknown skill', () => {
    expect(abilityLabel('basket-weaving')).toBe('');
  });

  it('returns "" for null/undefined/empty', () => {
    expect(abilityLabel(null)).toBe('');
    expect(abilityLabel(undefined)).toBe('');
    expect(abilityLabel('')).toBe('');
  });
});
