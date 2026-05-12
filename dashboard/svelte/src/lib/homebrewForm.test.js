import { describe, it, expect } from 'vitest';
import {
  HOMEBREW_CATEGORIES,
  emptyFormModel,
  buildHomebrewPayload,
  entryToFormModel,
} from './homebrewForm.js';

describe('HOMEBREW_CATEGORIES', () => {
  it('includes all seven core categories and the class-feature path', () => {
    const keys = HOMEBREW_CATEGORIES.map((c) => c.key);
    expect(keys).toEqual(
      expect.arrayContaining([
        'creatures',
        'spells',
        'weapons',
        'magic-items',
        'races',
        'feats',
        'classes',
        'class-features',
      ]),
    );
  });

  it('routes class-features through the existing /classes path', () => {
    const cf = HOMEBREW_CATEGORIES.find((c) => c.key === 'class-features');
    expect(cf.path).toBe('classes');
  });
});

describe('emptyFormModel', () => {
  it('returns sensible defaults per category', () => {
    expect(emptyFormModel('creatures').size).toBe('Medium');
    expect(emptyFormModel('spells').level).toBe(0);
    expect(emptyFormModel('weapons').weapon_type).toBe('simple-melee');
    expect(emptyFormModel('magic-items').rarity).toBe('common');
    expect(emptyFormModel('races').speed_ft).toBe(30);
    expect(emptyFormModel('feats').description).toBe('');
    expect(emptyFormModel('classes').hit_die).toBe('d8');
    expect(emptyFormModel('class-features').level).toBe(1);
  });
});

describe('buildHomebrewPayload — creatures', () => {
  it('produces the Upsert*Params wire shape and flags homebrew=true', () => {
    const model = {
      ...emptyFormModel('creatures'),
      id: 'goblin-king',
      name: 'Goblin King',
      cr: '1/4',
      ac: 13,
      hp_average: 7,
      languages: 'common, goblin',
      damage_resistances: 'cold, fire',
    };
    const payload = buildHomebrewPayload('creatures', model);
    expect(payload.id).toBe('goblin-king');
    expect(payload.name).toBe('Goblin King');
    expect(payload.cr).toBe('1/4');
    expect(payload.ac).toBe(13);
    expect(payload.hp_average).toBe(7);
    expect(payload.languages).toEqual(['common', 'goblin']);
    expect(payload.damage_resistances).toEqual(['cold', 'fire']);
    expect(payload.homebrew).toBe(true);
    // speed/ability_scores/attacks come through as parsed JSON objects.
    expect(typeof payload.speed).toBe('object');
    expect(typeof payload.ability_scores).toBe('object');
    expect(Array.isArray(payload.attacks)).toBe(true);
  });

  it('throws when a required field is missing', () => {
    expect(() => buildHomebrewPayload('creatures', { ...emptyFormModel('creatures'), id: '' }))
      .toThrow(/id is required/);
  });

  it('throws on invalid JSON in structured fields', () => {
    expect(() =>
      buildHomebrewPayload('creatures', {
        ...emptyFormModel('creatures'),
        id: 'x',
        name: 'X',
        speed_json: 'not-json',
      }),
    ).toThrow(/invalid JSON/);
  });
});

describe('buildHomebrewPayload — spells', () => {
  it('serializes optional fields as null when empty', () => {
    const payload = buildHomebrewPayload('spells', {
      ...emptyFormModel('spells'),
      id: 'magic-arrow',
      name: 'Magic Arrow',
      description: 'A glowing arrow.',
    });
    expect(payload.range_ft).toBeNull();
    expect(payload.material_description).toBeNull();
    expect(payload.material_cost_gp).toBeNull();
    expect(payload.material_consumed).toBe(false);
    expect(payload.components).toEqual(['V', 'S']);
    expect(payload.homebrew).toBe(true);
  });
});

describe('buildHomebrewPayload — weapons', () => {
  it('parses numeric range fields and coerces empty values to null', () => {
    const payload = buildHomebrewPayload('weapons', {
      ...emptyFormModel('weapons'),
      id: 'shortbow-plus-1',
      name: 'Shortbow +1',
      range_normal_ft: '40',
      range_long_ft: '160',
      properties: 'ammunition, range, two-handed',
    });
    expect(payload.range_normal_ft).toBe(40);
    expect(payload.range_long_ft).toBe(160);
    expect(payload.properties).toEqual(['ammunition', 'range', 'two-handed']);
    expect(payload.weight_lb).toBeNull();
    expect(payload.homebrew).toBe(true);
  });
});

describe('buildHomebrewPayload — magic-items', () => {
  it('encodes attunement and magic bonus correctly', () => {
    const payload = buildHomebrewPayload('magic-items', {
      ...emptyFormModel('magic-items'),
      id: 'ring-of-fire',
      name: 'Ring of Fire',
      requires_attunement: true,
      attunement_restriction: 'by a sorcerer',
      magic_bonus: '2',
    });
    expect(payload.requires_attunement).toBe(true);
    expect(payload.attunement_restriction).toBe('by a sorcerer');
    expect(payload.magic_bonus).toBe(2);
    expect(payload.homebrew).toBe(true);
  });
});

describe('buildHomebrewPayload — races', () => {
  it('serializes ability bonuses and traits as JSON objects', () => {
    const payload = buildHomebrewPayload('races', {
      ...emptyFormModel('races'),
      id: 'glowing-elf',
      name: 'Glowing Elf',
      languages: 'common, elvish, infernal',
    });
    expect(payload.speed_ft).toBe(30);
    expect(payload.languages).toEqual(['common', 'elvish', 'infernal']);
    expect(typeof payload.ability_bonuses).toBe('object');
    expect(Array.isArray(payload.traits)).toBe(true);
    expect(payload.homebrew).toBe(true);
  });
});

describe('buildHomebrewPayload — feats', () => {
  it('only requires id+name and forwards a description', () => {
    const payload = buildHomebrewPayload('feats', {
      id: 'sharp-eye',
      name: 'Sharp Eye',
      description: 'You see far.',
    });
    expect(payload).toEqual({
      id: 'sharp-eye',
      name: 'Sharp Eye',
      description: 'You see far.',
      homebrew: true,
    });
  });
});

describe('buildHomebrewPayload — classes', () => {
  it('parses features_by_level as JSON', () => {
    const payload = buildHomebrewPayload('classes', {
      ...emptyFormModel('classes'),
      id: 'duelist',
      name: 'Duelist',
      features_by_level_json: '[{"level":1,"name":"Riposte","description":"x"}]',
    });
    expect(Array.isArray(payload.features_by_level)).toBe(true);
    expect(payload.features_by_level[0].name).toBe('Riposte');
  });
});

describe('buildHomebrewPayload — class-features', () => {
  it('builds a single-feature class skeleton', () => {
    const payload = buildHomebrewPayload('class-features', {
      id: 'fighter-cleave',
      class_id: 'fighter',
      class_name: 'Fighter',
      feature_name: 'Cleave',
      level: 5,
      description: 'On a kill, attack again.',
    });
    expect(payload.id).toBe('fighter-cleave');
    expect(payload.name).toBe('Fighter: Cleave');
    expect(payload.features_by_level).toHaveLength(1);
    expect(payload.features_by_level[0]).toMatchObject({
      level: 5,
      name: 'Cleave',
      description: 'On a kill, attack again.',
      parent_class_id: 'fighter',
      parent_class_name: 'Fighter',
    });
    // The skeleton still satisfies the class wire shape (so /api/homebrew/classes accepts it).
    expect(payload.hit_die).toBeDefined();
    expect(payload.primary_ability).toBeDefined();
  });

  it('requires class id/name/feature name', () => {
    expect(() =>
      buildHomebrewPayload('class-features', {
        ...emptyFormModel('class-features'),
        id: 'x',
      }),
    ).toThrow(/feature_name is required/);
  });
});

describe('entryToFormModel — creatures round-trip', () => {
  it('round-trips through build + entry conversion', () => {
    const original = buildHomebrewPayload('creatures', {
      ...emptyFormModel('creatures'),
      id: 'rt-001',
      name: 'Round Trip Creature',
      cr: '2',
      languages: 'common',
    });
    // Simulate the backend echoing the row.
    const echoed = { ...original };
    const model = entryToFormModel('creatures', echoed);
    expect(model.id).toBe('rt-001');
    expect(model.name).toBe('Round Trip Creature');
    expect(model.cr).toBe('2');
    expect(model.languages).toBe('common');
  });
});

describe('entryToFormModel — empty entry', () => {
  it('returns an empty model when entry is null', () => {
    expect(entryToFormModel('feats', null)).toEqual(emptyFormModel('feats'));
  });
});
