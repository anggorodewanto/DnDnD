import { describe, it, expect } from 'vitest';
import { extractPlaceholders, substitutePlaceholders } from './narrationTemplates.js';

describe('extractPlaceholders', () => {
  it('returns unique placeholder names in order of first occurrence', () => {
    const out = extractPlaceholders('Hello {player_name}, welcome to {location}. {player_name}!');
    expect(out).toEqual(['player_name', 'location']);
  });

  it('ignores tokens with invalid characters', () => {
    const out = extractPlaceholders('Curly { spaced } and {1bad} but {good_one} works');
    expect(out).toEqual(['good_one']);
  });

  it('returns empty array for blank/null input', () => {
    expect(extractPlaceholders('')).toEqual([]);
    expect(extractPlaceholders(null)).toEqual([]);
    expect(extractPlaceholders(undefined)).toEqual([]);
  });
});

describe('substitutePlaceholders', () => {
  it('replaces known tokens', () => {
    const out = substitutePlaceholders('Hello {player}, in {place}.', {
      player: 'Aragorn',
      place: 'Bree',
    });
    expect(out).toBe('Hello Aragorn, in Bree.');
  });

  it('leaves unknown tokens untouched', () => {
    const out = substitutePlaceholders('Hello {player}, value {missing}.', { player: 'A' });
    expect(out).toBe('Hello A, value {missing}.');
  });

  it('returns body unchanged when values is null/empty', () => {
    expect(substitutePlaceholders('Hi {x}.', null)).toBe('Hi {x}.');
    expect(substitutePlaceholders('Hi {x}.', {})).toBe('Hi {x}.');
  });

  it('handles null body gracefully', () => {
    expect(substitutePlaceholders(null, { x: 'y' })).toBe('');
  });
});
