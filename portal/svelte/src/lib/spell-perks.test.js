import { describe, it, expect } from 'vitest';
import { isConcentration, formatCastingTime, shortDescription } from './spell-perks.js';

describe('isConcentration', () => {
  it('returns true when duration contains "Concentration"', () => {
    expect(isConcentration('Concentration, up to 1 minute')).toBe(true);
  });

  it('is case-insensitive', () => {
    expect(isConcentration('concentration, up to 10 minutes')).toBe(true);
    expect(isConcentration('Up to CONCENTRATION 1 hour')).toBe(true);
  });

  it('returns false for non-concentration durations', () => {
    expect(isConcentration('Instantaneous')).toBe(false);
    expect(isConcentration('1 minute')).toBe(false);
  });

  it('returns false for empty/undefined', () => {
    expect(isConcentration('')).toBe(false);
    expect(isConcentration(undefined)).toBe(false);
    expect(isConcentration(null)).toBe(false);
  });
});

describe('formatCastingTime', () => {
  it('returns "" for empty/undefined', () => {
    expect(formatCastingTime('')).toBe('');
    expect(formatCastingTime(undefined)).toBe('');
    expect(formatCastingTime(null)).toBe('');
  });

  it('returns the trimmed string unchanged for non-empty', () => {
    expect(formatCastingTime('1 action')).toBe('1 action');
    expect(formatCastingTime('  1 bonus action  ')).toBe('1 bonus action');
  });
});

describe('shortDescription', () => {
  it('returns "" for empty/undefined', () => {
    expect(shortDescription('')).toBe('');
    expect(shortDescription(undefined)).toBe('');
    expect(shortDescription(null)).toBe('');
  });

  it('returns the trimmed string unchanged when it already fits (no ellipsis)', () => {
    const desc = 'A short spell description.';
    expect(shortDescription(desc)).toBe(desc);
    expect(shortDescription(desc)).not.toContain('…');
  });

  it('truncates a long description at a word boundary with an ellipsis', () => {
    const desc =
      'You create up to four torch-sized lights within range making them appear as torches lanterns or glowing orbs that hover in the air for the duration.';
    const result = shortDescription(desc);
    expect(result.endsWith('…')).toBe(true);
    // never exceeds maxLen + 1 (the ellipsis)
    expect(result.length).toBeLessThanOrEqual(140 + 1);
    // word-boundary cut never produces a partial word
    const words = result.slice(0, -1).trimEnd().split(' ');
    const lastWord = words[words.length - 1];
    expect(desc.split(' ')).toContain(lastWord);
  });

  it('respects a custom maxLen', () => {
    const desc = 'You touch one willing creature and grant it a boon for a while.';
    const result = shortDescription(desc, 20);
    expect(result.endsWith('…')).toBe(true);
    expect(result.length).toBeLessThanOrEqual(20 + 1);
    expect(desc.startsWith(result.slice(0, -1).trimEnd())).toBe(true);
  });

  it('does not add an ellipsis when exactly at maxLen', () => {
    const desc = 'abcde';
    expect(shortDescription(desc, 5)).toBe('abcde');
    expect(shortDescription(desc, 5)).not.toContain('…');
  });
});
