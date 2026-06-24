import { describe, it, expect } from 'vitest';
import { ordinal, levelLabel, spellHeadline, classLabels, spellDetailMeta } from './spell-details.js';

describe('ordinal', () => {
  it('formats common ordinals', () => {
    expect(ordinal(1)).toBe('1st');
    expect(ordinal(2)).toBe('2nd');
    expect(ordinal(3)).toBe('3rd');
    expect(ordinal(4)).toBe('4th');
    expect(ordinal(9)).toBe('9th');
  });

  it('handles teens as "th"', () => {
    expect(ordinal(11)).toBe('11th');
    expect(ordinal(12)).toBe('12th');
    expect(ordinal(13)).toBe('13th');
  });

  it('accepts numeric strings', () => {
    expect(ordinal('3')).toBe('3rd');
  });
});

describe('levelLabel', () => {
  it('labels cantrips', () => {
    expect(levelLabel(0)).toBe('Cantrip');
  });

  it('labels leveled spells', () => {
    expect(levelLabel(1)).toBe('Level 1');
    expect(levelLabel(9)).toBe('Level 9');
  });

  it('accepts numeric strings', () => {
    expect(levelLabel('2')).toBe('Level 2');
  });

  it('returns "" for non-numeric/missing', () => {
    expect(levelLabel(undefined)).toBe('');
    expect(levelLabel(null)).toBe('');
    expect(levelLabel('nope')).toBe('');
  });
});

describe('spellHeadline', () => {
  it('formats a cantrip with its school', () => {
    expect(spellHeadline({ level: 0, school: 'evocation' })).toBe('Evocation cantrip');
  });

  it('formats a leveled spell with its school (D&D phrasing)', () => {
    expect(spellHeadline({ level: 3, school: 'abjuration' })).toBe('3rd-level abjuration');
    expect(spellHeadline({ level: 1, school: 'illusion' })).toBe('1st-level illusion');
  });

  it('omits the school when missing', () => {
    expect(spellHeadline({ level: 0 })).toBe('Cantrip');
    expect(spellHeadline({ level: 2, school: '' })).toBe('2nd-level');
  });

  it('returns "" for a null spell', () => {
    expect(spellHeadline(null)).toBe('');
    expect(spellHeadline(undefined)).toBe('');
  });
});

describe('classLabels', () => {
  it('title-cases class slugs', () => {
    expect(classLabels(['wizard', 'cleric'])).toEqual(['Wizard', 'Cleric']);
  });

  it('drops falsy entries and handles empty/missing', () => {
    expect(classLabels(['wizard', '', null])).toEqual(['Wizard']);
    expect(classLabels([])).toEqual([]);
    expect(classLabels(undefined)).toEqual([]);
  });
});

describe('spellDetailMeta', () => {
  it('returns rows for every present field in order', () => {
    const rows = spellDetailMeta({
      casting_time: '1 action',
      range: '120ft',
      components: ['V', 'S', 'M'],
      duration: 'Concentration, up to 1 minute',
      classes: ['wizard', 'sorcerer'],
    });
    expect(rows).toEqual([
      { label: 'Casting Time', value: '1 action' },
      { label: 'Range', value: '120ft' },
      { label: 'Components', value: 'V, S, M' },
      { label: 'Duration', value: 'Concentration, up to 1 minute' },
      { label: 'Classes', value: 'Wizard, Sorcerer' },
    ]);
  });

  it('omits empty/missing fields', () => {
    expect(spellDetailMeta({ casting_time: '1 action' })).toEqual([
      { label: 'Casting Time', value: '1 action' },
    ]);
    expect(spellDetailMeta({ components: [] })).toEqual([]);
    expect(spellDetailMeta({})).toEqual([]);
    expect(spellDetailMeta(null)).toEqual([]);
  });

  it('trims field values', () => {
    expect(spellDetailMeta({ range: '  Self  ' })).toEqual([
      { label: 'Range', value: 'Self' },
    ]);
  });
});
