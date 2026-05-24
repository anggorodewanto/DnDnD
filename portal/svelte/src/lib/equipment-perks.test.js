import { describe, it, expect } from 'vitest';
import { formatProperties, armorACText } from './equipment-perks.js';

// Property shapes verified against internal/refdata/seeder.go:
//   weapon Properties are plain lowercase strings, e.g.
//   ["light"], ["finesse","light","thrown"], ["versatile"], ["two-handed"],
//   ["heavy","reach","two-handed"], ["ammunition","loading"].
// VersatileDamage / range live in separate struct fields, so the API
// normally emits plain strings. We ALSO support "key:value" defensively
// (homebrew / future shapes) per the spec.
// armor_type values verified: "light", "medium", "heavy", "shield".

describe('formatProperties', () => {
  it('returns empty string for undefined', () => {
    expect(formatProperties(undefined)).toBe('');
  });

  it('returns empty string for null', () => {
    expect(formatProperties(null)).toBe('');
  });

  it('returns empty string for empty array', () => {
    expect(formatProperties([])).toBe('');
  });

  it('title-cases a single simple property', () => {
    expect(formatProperties(['finesse'])).toBe('Finesse');
  });

  it('title-cases and comma-joins simple properties (real dagger shape)', () => {
    expect(formatProperties(['finesse', 'light', 'thrown'])).toBe(
      'Finesse, Light, Thrown',
    );
  });

  it('title-cases a hyphenated property (real two-handed shape)', () => {
    expect(formatProperties(['two-handed'])).toBe('Two-Handed');
  });

  it('formats real heavy crossbow shape', () => {
    expect(formatProperties(['ammunition', 'heavy', 'loading', 'two-handed'])).toBe(
      'Ammunition, Heavy, Loading, Two-Handed',
    );
  });

  it('formats a key:value property as Key (value)', () => {
    expect(formatProperties(['versatile:1d10'])).toBe('Versatile (1d10)');
  });

  it('formats multiple key:value properties', () => {
    expect(formatProperties(['versatile:1d10', 'thrown:20/60'])).toBe(
      'Versatile (1d10), Thrown (20/60)',
    );
  });

  it('mixes simple and key:value properties', () => {
    expect(formatProperties(['finesse', 'versatile:1d8'])).toBe(
      'Finesse, Versatile (1d8)',
    );
  });

  it('skips empty / blank entries', () => {
    expect(formatProperties(['', '  ', 'light'])).toBe('Light');
  });
});

describe('armorACText', () => {
  it('returns empty string when acBase is null', () => {
    expect(armorACText('light', null)).toBe('');
  });

  it('returns empty string when acBase is undefined', () => {
    expect(armorACText('light', undefined)).toBe('');
  });

  it('formats light armor', () => {
    expect(armorACText('light', 11)).toBe('AC 11 + DEX');
  });

  it('formats medium armor with DEX cap', () => {
    expect(armorACText('medium', 14)).toBe('AC 14 + DEX (max 2)');
  });

  it('formats heavy armor (no DEX)', () => {
    expect(armorACText('heavy', 18)).toBe('AC 18');
  });

  it('formats shield as a bonus', () => {
    expect(armorACText('shield', 2)).toBe('+2 AC');
  });

  it('is case-insensitive on armorType', () => {
    expect(armorACText('LIGHT', 12)).toBe('AC 12 + DEX');
    expect(armorACText('Medium', 13)).toBe('AC 13 + DEX (max 2)');
    expect(armorACText('Shield', 2)).toBe('+2 AC');
  });

  it('falls back to plain AC for unknown armorType', () => {
    expect(armorACText('exotic', 15)).toBe('AC 15');
  });

  it('falls back to plain AC for empty armorType', () => {
    expect(armorACText('', 10)).toBe('AC 10');
  });

  it('handles acBase of 0 (not treated as missing)', () => {
    expect(armorACText('heavy', 0)).toBe('AC 0');
  });
});
