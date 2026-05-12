import { describe, it, expect } from 'vitest';
import { BACKGROUND_SKILLS, skillsForBackground, mergeBackgroundSkills } from './backgrounds.js';

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
