import { describe, it, expect } from 'vitest';
import {
  BACKGROUND_SKILLS,
  skillsForBackground,
  mergeBackgroundSkills,
  BACKGROUND_DETAILS,
  backgroundDetails,
  formatLanguages,
} from './backgrounds.js';

describe('backgrounds', () => {
  it('exposes every PHB background', () => {
    const expected = [
      'acolyte', 'charlatan', 'criminal', 'entertainer', 'folk-hero',
      'guild-artisan', 'hermit', 'noble', 'outlander', 'sage', 'sailor',
      'soldier', 'urchin',
    ];
    for (const bg of expected) {
      expect(BACKGROUND_SKILLS).toHaveProperty(bg);
      expect(BACKGROUND_SKILLS[bg]).toHaveLength(2);
    }
  });

  it('skillsForBackground returns soldier skills', () => {
    expect(skillsForBackground('soldier')).toEqual(['athletics', 'intimidation']);
  });

  it('skillsForBackground returns [] for unknown', () => {
    expect(skillsForBackground('not-a-real-bg')).toEqual([]);
    expect(skillsForBackground('')).toEqual([]);
    expect(skillsForBackground(null)).toEqual([]);
  });

  it('mergeBackgroundSkills adds new skills', () => {
    const merged = mergeBackgroundSkills(['perception'], 'soldier');
    expect(merged).toContain('perception');
    expect(merged).toContain('athletics');
    expect(merged).toContain('intimidation');
  });

  it('mergeBackgroundSkills dedupes against existing picks', () => {
    const merged = mergeBackgroundSkills(['athletics', 'perception'], 'soldier');
    // 'athletics' should not appear twice
    expect(merged.filter(s => s === 'athletics')).toHaveLength(1);
    expect(merged).toContain('intimidation');
  });

  it('mergeBackgroundSkills handles empty/missing background', () => {
    expect(mergeBackgroundSkills(['perception'], '')).toEqual(['perception']);
    expect(mergeBackgroundSkills([], '')).toEqual([]);
    expect(mergeBackgroundSkills(null, '')).toEqual([]);
  });

  it('mergeBackgroundSkills does not mutate input', () => {
    const input = ['perception'];
    mergeBackgroundSkills(input, 'soldier');
    expect(input).toEqual(['perception']);
  });
});

describe('background details', () => {
  it('has details for every background in BACKGROUND_SKILLS', () => {
    const skillSlugs = Object.keys(BACKGROUND_SKILLS).sort();
    const detailSlugs = Object.keys(BACKGROUND_DETAILS).sort();
    expect(detailSlugs).toEqual(skillSlugs);
  });

  it('every detail has tools[], integer languages, and a feature', () => {
    for (const [slug, detail] of Object.entries(BACKGROUND_DETAILS)) {
      expect(Array.isArray(detail.tools), `${slug} tools`).toBe(true);
      expect(Number.isInteger(detail.languages), `${slug} languages`).toBe(true);
      expect(detail.languages, `${slug} languages >= 0`).toBeGreaterThanOrEqual(0);
      expect(typeof detail.feature.name, `${slug} feature.name`).toBe('string');
      expect(detail.feature.name.length, `${slug} feature.name nonempty`).toBeGreaterThan(0);
      expect(typeof detail.feature.description, `${slug} feature.description`).toBe('string');
    }
  });

  it('feature descriptions are short paraphrases (guards against copied PHB text)', () => {
    for (const [slug, detail] of Object.entries(BACKGROUND_DETAILS)) {
      expect(detail.feature.description.length, `${slug} description length`).toBeLessThanOrEqual(90);
    }
  });

  it('encodes known mechanical grants', () => {
    expect(BACKGROUND_DETAILS.acolyte.languages).toBe(2);
    expect(BACKGROUND_DETAILS.sage.languages).toBe(2);
    expect(BACKGROUND_DETAILS.hermit.languages).toBe(1);
    expect(BACKGROUND_DETAILS.soldier.languages).toBe(0);
    expect(BACKGROUND_DETAILS.criminal.tools).toContain("Thieves' tools");
    expect(BACKGROUND_DETAILS.acolyte.tools).toEqual([]);
  });

  it('backgroundDetails merges skills with detail data for a known slug', () => {
    const d = backgroundDetails('criminal');
    expect(d.skills).toEqual(skillsForBackground('criminal'));
    expect(d.tools).toEqual(BACKGROUND_DETAILS.criminal.tools);
    expect(d.languages).toBe(BACKGROUND_DETAILS.criminal.languages);
    expect(d.feature).toEqual(BACKGROUND_DETAILS.criminal.feature);
  });

  it('backgroundDetails returns null for unknown/empty', () => {
    expect(backgroundDetails('not-a-real-bg')).toBeNull();
    expect(backgroundDetails('')).toBeNull();
    expect(backgroundDetails(null)).toBeNull();
  });

  it('formatLanguages renders counts', () => {
    expect(formatLanguages(0)).toBe('');
    expect(formatLanguages(null)).toBe('');
    expect(formatLanguages(undefined)).toBe('');
    expect(formatLanguages(1)).toBe('One language of your choice');
    expect(formatLanguages(2)).toBe('2 languages of your choice');
  });
});
