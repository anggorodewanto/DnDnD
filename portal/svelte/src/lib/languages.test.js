import { describe, it, expect } from 'vitest';
import {
  STANDARD_LANGUAGES,
  EXOTIC_LANGUAGES,
  ALL_LANGUAGES,
  raceBaseLanguages,
  availableLanguageChoices,
  assembleLanguages,
  bonusLanguageCount,
} from './languages.js';

describe('language catalogs', () => {
  it('STANDARD_LANGUAGES are the eight PHB standard languages', () => {
    expect(STANDARD_LANGUAGES).toEqual([
      'Common', 'Dwarvish', 'Elvish', 'Giant', 'Gnomish', 'Goblin', 'Halfling', 'Orc',
    ]);
  });

  it('EXOTIC_LANGUAGES are the eight PHB exotic languages', () => {
    expect(EXOTIC_LANGUAGES).toEqual([
      'Abyssal', 'Celestial', 'Draconic', 'Deep Speech', 'Infernal', 'Primordial', 'Sylvan', 'Undercommon',
    ]);
  });

  it('ALL_LANGUAGES is standard followed by exotic', () => {
    expect(ALL_LANGUAGES).toEqual([...STANDARD_LANGUAGES, ...EXOTIC_LANGUAGES]);
    expect(ALL_LANGUAGES).toHaveLength(16);
  });

  it('ALL_LANGUAGES has no duplicates', () => {
    expect(new Set(ALL_LANGUAGES).size).toBe(ALL_LANGUAGES.length);
  });
});

describe('raceBaseLanguages', () => {
  it('returns the race languages preserving casing', () => {
    expect(raceBaseLanguages({ languages: ['Common', 'Dwarvish'] })).toEqual(['Common', 'Dwarvish']);
  });

  it('returns [] for missing raceData', () => {
    expect(raceBaseLanguages(undefined)).toEqual([]);
    expect(raceBaseLanguages(null)).toEqual([]);
  });

  it('returns [] when languages is absent or empty', () => {
    expect(raceBaseLanguages({})).toEqual([]);
    expect(raceBaseLanguages({ languages: [] })).toEqual([]);
  });

  it('filters out empty / whitespace-only entries', () => {
    expect(raceBaseLanguages({ languages: ['Common', '', '  ', 'Orc'] })).toEqual(['Common', 'Orc']);
  });

  it('ignores a non-array languages field', () => {
    expect(raceBaseLanguages({ languages: 'Common' })).toEqual([]);
  });
});

describe('availableLanguageChoices', () => {
  it('returns ALL_LANGUAGES when nothing is known', () => {
    expect(availableLanguageChoices([])).toEqual(ALL_LANGUAGES);
  });

  it('excludes known languages, preserving ALL_LANGUAGES order', () => {
    const choices = availableLanguageChoices(['Common', 'Dwarvish']);
    expect(choices).not.toContain('Common');
    expect(choices).not.toContain('Dwarvish');
    expect(choices[0]).toBe('Elvish');
    expect(choices).toHaveLength(ALL_LANGUAGES.length - 2);
  });

  it('compares known case-insensitively', () => {
    const choices = availableLanguageChoices(['common', 'DRACONIC', 'deep speech']);
    expect(choices).not.toContain('Common');
    expect(choices).not.toContain('Draconic');
    expect(choices).not.toContain('Deep Speech');
  });

  it('ignores unknown / empty known entries', () => {
    expect(availableLanguageChoices(['', '  ', 'NotALanguage'])).toEqual(ALL_LANGUAGES);
  });

  it('handles a missing known argument', () => {
    expect(availableLanguageChoices()).toEqual(ALL_LANGUAGES);
  });
});

describe('assembleLanguages', () => {
  it('unions race languages then chosen, preserving first-seen order', () => {
    expect(assembleLanguages(['Common', 'Dwarvish'], ['Giant'])).toEqual(['Common', 'Dwarvish', 'Giant']);
  });

  it('de-dupes case-insensitively, keeping first-seen casing', () => {
    expect(assembleLanguages(['Common'], ['common', 'Giant'])).toEqual(['Common', 'Giant']);
  });

  it('drops empty / whitespace entries', () => {
    expect(assembleLanguages(['Common', ''], ['  ', 'Orc'])).toEqual(['Common', 'Orc']);
  });

  it('handles missing arguments', () => {
    expect(assembleLanguages()).toEqual([]);
    expect(assembleLanguages(['Common'])).toEqual(['Common']);
    expect(assembleLanguages(undefined, ['Orc'])).toEqual(['Orc']);
  });

  it('de-dupes within chosen as well', () => {
    expect(assembleLanguages([], ['Giant', 'giant', 'GIANT'])).toEqual(['Giant']);
  });
});

describe('bonusLanguageCount', () => {
  it('coerces the background languages field to a non-negative integer', () => {
    expect(bonusLanguageCount({ languages: 2 })).toBe(2);
    expect(bonusLanguageCount({ languages: '1' })).toBe(1);
  });

  it('returns 0 for missing / null / non-numeric / negative', () => {
    expect(bonusLanguageCount(null)).toBe(0);
    expect(bonusLanguageCount(undefined)).toBe(0);
    expect(bonusLanguageCount({})).toBe(0);
    expect(bonusLanguageCount({ languages: 'abc' })).toBe(0);
    expect(bonusLanguageCount({ languages: -3 })).toBe(0);
  });
});
