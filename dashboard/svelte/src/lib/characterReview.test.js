import { describe, it, expect } from 'vitest';
import {
  reviewChanges,
  listDiff,
  formatReviewField,
  reviewFields,
  REVIEW_FIELD_ORDER,
} from './characterReview.js';

const baseReview = {
  name: 'Thorin',
  race: 'dwarf',
  classes: ['Fighter 3 (Champion)'],
  level: 3,
  ability_scores: { str: 16, dex: 12, con: 15, int: 10, wis: 12, cha: 8 },
  hp_max: 28,
  ac: 16,
  speed_ft: 25,
  skills: ['athletics', 'intimidation'],
  expertise: [],
  saves: ['con', 'str'],
  languages: ['common', 'dwarvish'],
  equipment: ['Longsword', 'Shield'],
  spells: [],
  weapon_masteries: [],
  features: ['Second Wind'],
};

describe('reviewChanges', () => {
  it('returns no rows when before and after match', () => {
    expect(reviewChanges(baseReview, { ...baseReview })).toEqual([]);
  });

  it('reports a changed scalar field as before -> after', () => {
    const after = { ...baseReview, ac: 18 };
    const rows = reviewChanges(baseReview, after);
    expect(rows).toEqual([{ field: 'ac', label: 'AC', kind: 'scalar', before: 16, after: 18 }]);
  });

  it('expands ability_scores into per-ability scalar rows', () => {
    const after = { ...baseReview, ability_scores: { ...baseReview.ability_scores, cha: 10 } };
    const rows = reviewChanges(baseReview, after);
    expect(rows).toEqual([{ field: 'cha', label: 'CHA', kind: 'scalar', before: 8, after: 10 }]);
  });

  it('reports list fields as added/removed sets', () => {
    const after = { ...baseReview, skills: ['athletics', 'stealth'] };
    const rows = reviewChanges(baseReview, after);
    expect(rows).toEqual([
      { field: 'skills', label: 'Skills', kind: 'list', added: ['stealth'], removed: ['intimidation'] },
    ]);
  });

  it('handles a null before-baseline (treats everything as added)', () => {
    const rows = reviewChanges(null, { skills: ['stealth'], ac: 12 });
    const skillsRow = rows.find((r) => r.field === 'skills');
    const acRow = rows.find((r) => r.field === 'ac');
    expect(skillsRow).toEqual({ field: 'skills', label: 'Skills', kind: 'list', added: ['stealth'], removed: [] });
    expect(acRow).toEqual({ field: 'ac', label: 'AC', kind: 'scalar', before: undefined, after: 12 });
  });
});

describe('listDiff', () => {
  it('computes added and removed elements', () => {
    expect(listDiff(['a', 'b'], ['b', 'c'])).toEqual({ added: ['c'], removed: ['a'] });
  });

  it('handles empty inputs', () => {
    expect(listDiff([], ['x'])).toEqual({ added: ['x'], removed: [] });
    expect(listDiff(['x'], [])).toEqual({ added: [], removed: ['x'] });
  });
});

describe('formatReviewField', () => {
  it('joins arrays and shows a dash for empties', () => {
    expect(formatReviewField('skills', ['athletics', 'stealth'])).toBe('athletics, stealth');
    expect(formatReviewField('skills', [])).toBe('—');
  });

  it('renders ability_scores compactly', () => {
    expect(formatReviewField('ability_scores', { str: 16, dex: 12, con: 15, int: 10, wis: 12, cha: 8 })).toBe(
      'STR 16 · DEX 12 · CON 15 · INT 10 · WIS 12 · CHA 8',
    );
  });

  it('stringifies scalars and dashes empty text', () => {
    expect(formatReviewField('ac', 18)).toBe('18');
    expect(formatReviewField('appearance', '')).toBe('—');
    expect(formatReviewField('subrace', null)).toBe('—');
  });
});

describe('reviewFields', () => {
  it('returns ordered label/value rows for the full review', () => {
    const rows = reviewFields(baseReview);
    expect(rows[0]).toEqual({ field: 'name', label: 'Name', value: 'Thorin' });
    const fields = rows.map((r) => r.field);
    // Ordering follows REVIEW_FIELD_ORDER.
    expect(fields.indexOf('ability_scores')).toBeLessThan(fields.indexOf('skills'));
  });

  it('skips absent optional text fields (subrace/background/appearance/backstory)', () => {
    const rows = reviewFields(baseReview);
    const fields = rows.map((r) => r.field);
    expect(fields).not.toContain('subrace');
    expect(fields).not.toContain('appearance');
  });

  it('includes optional fields when present', () => {
    const rows = reviewFields({ ...baseReview, subrace: 'hill-dwarf', appearance: 'Stout.' });
    const fields = rows.map((r) => r.field);
    expect(fields).toContain('subrace');
    expect(fields).toContain('appearance');
  });

  it('only emits known review fields', () => {
    const rows = reviewFields({ ...baseReview, unexpected: 'x' });
    for (const r of rows) expect(REVIEW_FIELD_ORDER).toContain(r.field);
  });
});
