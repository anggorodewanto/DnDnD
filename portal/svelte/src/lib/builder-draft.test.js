import { describe, it, expect } from 'vitest';
import { DRAFT_VERSION, draftKey, draftScope, serializeDraft, parseDraft, draftHasContent } from './builder-draft.js';

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

describe('draftScope', () => {
  it('keys a player draft by campaign, not the single-use token', () => {
    expect(draftScope('player', 'camp-1', 'tok-abc')).toBe('player:camp-1');
  });

  it('is stable across token rotation for the same campaign', () => {
    // The whole point of T10: a reissued /create-character link (new token)
    // must restore the same in-progress draft, not a blank form.
    expect(draftScope('player', 'camp-1', 'tok-OLD'))
      .toBe(draftScope('player', 'camp-1', 'tok-NEW'));
  });

  it('falls back to the token when no campaign is known', () => {
    expect(draftScope('player', '', 'tok-abc')).toBe('player:tok-abc');
  });

  it('keys a DM draft by campaign', () => {
    expect(draftScope('dm', 'camp-1', '')).toBe('dm:camp-1');
  });

  it('does not collide a player PC draft with a DM NPC draft for the same campaign', () => {
    // Portal and dashboard share one localStorage origin, so the mode must
    // namespace the scope.
    expect(draftScope('player', 'camp-1', 't')).not.toBe(draftScope('dm', 'camp-1', ''));
  });

  it('collapses to an empty scope when nothing identifies the draft', () => {
    // Empty scope -> draftKey() -> the shared :default namespace.
    expect(draftScope('player', '', '')).toBe('');
    expect(draftKey(draftScope('player', '', ''))).toBe('dndnd-builder-draft:default');
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

describe('draftHasContent', () => {
  it('returns false for null', () => {
    expect(draftHasContent(null)).toBe(false);
  });

  it('returns false for undefined', () => {
    expect(draftHasContent(undefined)).toBe(false);
  });

  it('returns false for an empty object', () => {
    expect(draftHasContent({})).toBe(false);
  });

  it('treats a bare version envelope as no content', () => {
    // An empty {v:1} draft (parseDraft strips v, leaving {}) must be "no draft"
    // so the hydration logic still reaches out to the server.
    expect(draftHasContent(parseDraft('{"v":1}'))).toBe(false);
  });

  it('returns true when a name is present', () => {
    expect(draftHasContent({ name: 'Gimli' })).toBe(true);
  });

  it('returns true when a race is present', () => {
    expect(draftHasContent({ race: 'dwarf' })).toBe(true);
  });

  it('returns true when a subrace is present', () => {
    expect(draftHasContent({ subrace: 'hill-dwarf' })).toBe(true);
  });

  it('returns true when a background is present', () => {
    expect(draftHasContent({ background: 'soldier' })).toBe(true);
  });

  it('returns true when currentStep is a non-zero step', () => {
    expect(draftHasContent({ currentStep: 2 })).toBe(true);
  });

  it('treats currentStep 0 (the first/Basics step) as no content', () => {
    // Step 0 is the default landing step; on its own it is not "started".
    expect(draftHasContent({ currentStep: 0 })).toBe(false);
  });

  it('returns true when the first class entry has a class', () => {
    expect(draftHasContent({ classEntries: [{ class: 'wizard' }] })).toBe(true);
  });

  it('treats class entries with no chosen class as no content', () => {
    expect(draftHasContent({ classEntries: [{ class: '' }] })).toBe(false);
  });

  it('treats an empty classEntries array as no content', () => {
    expect(draftHasContent({ classEntries: [] })).toBe(false);
  });

  it('returns true when at least one skill is selected', () => {
    expect(draftHasContent({ selectedSkills: ['arcana'] })).toBe(true);
  });

  it('treats an empty selectedSkills array as no content', () => {
    expect(draftHasContent({ selectedSkills: [] })).toBe(false);
  });

  it('returns true when at least one spell is selected', () => {
    expect(draftHasContent({ selectedSpells: ['fireball'] })).toBe(true);
  });

  it('treats an empty selectedSpells array as no content', () => {
    expect(draftHasContent({ selectedSpells: [] })).toBe(false);
  });

  it('returns true when manual equipment was added', () => {
    expect(draftHasContent({ manualEquipment: ['rope-hempen'] })).toBe(true);
  });

  it('treats an empty manualEquipment array as no content', () => {
    expect(draftHasContent({ manualEquipment: [] })).toBe(false);
  });

  it('returns true for a full parsed draft', () => {
    const parsed = parseDraft(serializeDraft({ name: 'Gandalf', currentStep: 3 }));
    expect(draftHasContent(parsed)).toBe(true);
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

  it('round-trips the optional appearance and backstory flavor fields', () => {
    // The Basics-step free-form description fields must survive save -> restore
    // so a resumed / "request changes" build repopulates them.
    const input = {
      name: 'Gimli',
      appearance: 'Stout dwarf, red beard, battle-scarred.',
      backstory: 'Last of his clan, seeking the lost halls of Khazad-dûm.',
    };
    const restored = parseDraft(serializeDraft(input));
    expect(restored.appearance).toBe(input.appearance);
    expect(restored.backstory).toBe(input.backstory);
  });
});
