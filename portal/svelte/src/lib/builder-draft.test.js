import { describe, it, expect } from 'vitest';
import { DRAFT_VERSION, draftKey, serializeDraft, parseDraft } from './builder-draft.js';

describe('DRAFT_VERSION', () => {
  it('is 1', () => {
    expect(DRAFT_VERSION).toBe(1);
  });
});

describe('draftKey', () => {
  it('namespaces a token', () => {
    expect(draftKey('abc')).toBe('dndnd-builder-draft:abc');
  });

  it('falls back to :default for empty string', () => {
    expect(draftKey('')).toBe('dndnd-builder-draft:default');
  });

  it('falls back to :default for null', () => {
    expect(draftKey(null)).toBe('dndnd-builder-draft:default');
  });

  it('falls back to :default for undefined', () => {
    expect(draftKey(undefined)).toBe('dndnd-builder-draft:default');
  });
});

describe('serializeDraft', () => {
  it('always carries the version', () => {
    const parsed = JSON.parse(serializeDraft({}));
    expect(parsed.v).toBe(DRAFT_VERSION);
  });

  it('includes present fields', () => {
    const parsed = JSON.parse(serializeDraft({ name: 'Gimli', currentStep: 2 }));
    expect(parsed.name).toBe('Gimli');
    expect(parsed.currentStep).toBe(2);
  });

  it('omits fields that are undefined', () => {
    const parsed = JSON.parse(serializeDraft({ name: 'Gimli', race: undefined }));
    expect('race' in parsed).toBe(false);
  });

  it('ignores keys not in DRAFT_FIELDS', () => {
    const parsed = JSON.parse(serializeDraft({ name: 'Gimli', token: 'secret', extra: 1 }));
    expect('token' in parsed).toBe(false);
    expect('extra' in parsed).toBe(false);
    expect(parsed.name).toBe('Gimli');
  });

  it('serializes only the version for null state', () => {
    expect(serializeDraft(null)).toBe('{"v":1}');
  });

  it('serializes only the version for undefined state', () => {
    expect(serializeDraft(undefined)).toBe('{"v":1}');
  });
});

describe('parseDraft', () => {
  it('returns null for null', () => {
    expect(parseDraft(null)).toBe(null);
  });

  it('returns null for empty string', () => {
    expect(parseDraft('')).toBe(null);
  });

  it('returns null for undefined', () => {
    expect(parseDraft(undefined)).toBe(null);
  });

  it('returns null for malformed JSON', () => {
    expect(parseDraft('{not json')).toBe(null);
  });

  it('returns null for a JSON number', () => {
    expect(parseDraft('5')).toBe(null);
  });

  it('returns null for a JSON string', () => {
    expect(parseDraft('"x"')).toBe(null);
  });

  it('returns null for JSON null', () => {
    expect(parseDraft('null')).toBe(null);
  });

  it('returns null for a JSON array', () => {
    expect(parseDraft('[1,2]')).toBe(null);
  });

  it('returns null on version mismatch', () => {
    expect(parseDraft('{"v":0,"name":"Gimli"}')).toBe(null);
  });

  it('returns the known fields only, stripping v and unknown keys', () => {
    const result = parseDraft('{"v":1,"name":"Gimli","race":"dwarf","bogus":true}');
    expect(result).toEqual({ name: 'Gimli', race: 'dwarf' });
  });

  it('drops fields that are explicitly undefined-equivalent (absent)', () => {
    const result = parseDraft('{"v":1,"currentStep":3}');
    expect(result).toEqual({ currentStep: 3 });
  });
});

describe('round trip', () => {
  it('parseDraft(serializeDraft(input)) equals the DRAFT_FIELDS subset', () => {
    const input = {
      name: 'Gandalf',
      race: 'human',
      currentStep: 4,
      scores: { str: 10, dex: 14, con: 12, int: 18, wis: 13, cha: 11 },
      selectedSpells: ['fireball', 'magic-missile'],
      token: 'should-be-dropped',
    };
    const expected = {
      name: 'Gandalf',
      race: 'human',
      currentStep: 4,
      scores: { str: 10, dex: 14, con: 12, int: 18, wis: 13, cha: 11 },
      selectedSpells: ['fireball', 'magic-missile'],
    };
    expect(parseDraft(serializeDraft(input))).toEqual(expected);
  });
});
