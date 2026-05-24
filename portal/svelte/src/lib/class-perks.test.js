import { describe, it, expect } from 'vitest';
import { formatSkillChoices, titleCaseSlug } from './class-perks.js';

describe('titleCaseSlug', () => {
  it('title-cases a single word', () => {
    expect(titleCaseSlug('stealth')).toBe('Stealth');
  });

  it('replaces hyphens with spaces and title-cases each part', () => {
    expect(titleCaseSlug('animal-handling')).toBe('Animal Handling');
    expect(titleCaseSlug('sleight-of-hand')).toBe('Sleight Of Hand');
  });

  it('returns "" for falsy input', () => {
    expect(titleCaseSlug('')).toBe('');
    expect(titleCaseSlug(null)).toBe('');
    expect(titleCaseSlug(undefined)).toBe('');
  });
});

describe('formatSkillChoices', () => {
  it('formats the verified {choose, from} shape', () => {
    expect(
      formatSkillChoices({
        choose: 2,
        from: ['animal-handling', 'athletics', 'intimidation'],
      })
    ).toBe('Choose 2: Animal Handling, Athletics, Intimidation');
  });

  it('title-cases multi-word and hyphenated skills', () => {
    expect(
      formatSkillChoices({ choose: 1, from: ['sleight-of-hand', 'stealth'] })
    ).toBe('Choose 1: Sleight Of Hand, Stealth');
  });

  it('accepts a JSON string', () => {
    expect(
      formatSkillChoices('{"choose":3,"from":["acrobatics","arcana","history"]}')
    ).toBe('Choose 3: Acrobatics, Arcana, History');
  });

  it('returns "" for none/empty/invalid', () => {
    expect(formatSkillChoices(null)).toBe('');
    expect(formatSkillChoices(undefined)).toBe('');
    expect(formatSkillChoices('')).toBe('');
    expect(formatSkillChoices('not-json')).toBe('');
    expect(formatSkillChoices({})).toBe('');
  });

  it('returns "" when "from" is empty or missing', () => {
    expect(formatSkillChoices({ choose: 2, from: [] })).toBe('');
    expect(formatSkillChoices({ choose: 2 })).toBe('');
  });

  it('defaults the count to the number of options when "choose" is absent', () => {
    expect(formatSkillChoices({ from: ['arcana', 'history'] })).toBe(
      'Choose 2: Arcana, History'
    );
  });
});
