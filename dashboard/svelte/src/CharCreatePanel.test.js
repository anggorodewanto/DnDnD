// Component-level smoke tests for CharCreatePanel.
//
// The Svelte project runs Vitest under the bare node environment (see
// vite.config.js), which means we cannot exercise the .svelte file's
// rendered DOM directly. Instead we exercise the contract surface that
// matters: the panel must end up POSTing the right submission shape
// through the lib/charcreate.js helpers, which are themselves backed by
// the JSON API tests in internal/dashboard/charcreate_handler_test.go.
//
// The tests below reproduce the gatherSubmission and goToStep mini-
// state machines from CharCreatePanel.svelte so we can catch regressions
// in the wire format without spinning up a JSDOM. When the panel
// changes the submission shape it should fail to construct, this test
// file is the canary.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  fetchRaces,
  fetchClasses,
  fetchEquipment,
  fetchAbilityMethods,
  previewCharacter,
  createCharacter,
  parseSubclasses,
  roll4d6DropLowest,
} from './lib/charcreate.js';

/**
 * Mimic the panel's gatherSubmission(). Kept lock-step with the
 * implementation in CharCreatePanel.svelte.
 */
function gatherSubmission({
  charName,
  charRace,
  charBackground,
  classEntries,
  abilityScores,
  abilityMethod,
  abilityRolls,
  selectedEquipment,
  selectedSpells,
  equippedWeapon,
  wornArmor,
}) {
  return {
    name: charName,
    race: charRace,
    background: charBackground,
    classes: classEntries
      .filter((e) => e.class)
      .map((e) => ({
        class: e.class,
        subclass: e.subclass,
        level: Number(e.level) || 1,
      })),
    ability_scores: {
      str: Number(abilityScores.str) || 10,
      dex: Number(abilityScores.dex) || 10,
      con: Number(abilityScores.con) || 10,
      int: Number(abilityScores.int) || 10,
      wis: Number(abilityScores.wis) || 10,
      cha: Number(abilityScores.cha) || 10,
    },
    ability_method: abilityMethod,
    ability_rolls: abilityRolls,
    equipment: selectedEquipment,
    spells: selectedSpells,
    equipped_weapon: equippedWeapon,
    worn_armor: wornArmor,
  };
}

const baseState = () => ({
  charName: 'Thorin',
  charRace: 'Dwarf',
  charBackground: 'Soldier',
  classEntries: [{ class: 'Fighter', subclass: 'Champion', level: 5 }],
  abilityScores: { str: 16, dex: 12, con: 14, int: 10, wis: 8, cha: 10 },
  abilityMethod: 'roll',
  abilityRolls: { str: [6, 6, 4, 1] },
  selectedEquipment: ['longsword'],
  selectedSpells: [],
  equippedWeapon: 'longsword',
  wornArmor: '',
});

describe('CharCreatePanel.gatherSubmission', () => {
  it('produces the exact shape the /preview API expects', () => {
    const sub = gatherSubmission(baseState());
    expect(sub).toEqual({
      name: 'Thorin',
      race: 'Dwarf',
      background: 'Soldier',
      classes: [{ class: 'Fighter', subclass: 'Champion', level: 5 }],
      ability_scores: { str: 16, dex: 12, con: 14, int: 10, wis: 8, cha: 10 },
      ability_method: 'roll',
      ability_rolls: { str: [6, 6, 4, 1] },
      equipment: ['longsword'],
      spells: [],
      equipped_weapon: 'longsword',
      worn_armor: '',
    });
  });

  it('drops blank class entries so the server never sees an empty slot', () => {
    const state = baseState();
    state.classEntries = [
      { class: '', subclass: '', level: 1 },
      { class: 'Wizard', subclass: '', level: 3 },
    ];
    const sub = gatherSubmission(state);
    expect(sub.classes).toEqual([{ class: 'Wizard', subclass: '', level: 3 }]);
  });

  it('coerces non-numeric ability scores back to 10', () => {
    const state = baseState();
    state.abilityScores = { str: '', dex: 'x', con: undefined, int: 0, wis: NaN, cha: 'eight' };
    const sub = gatherSubmission(state);
    expect(sub.ability_scores).toEqual({
      str: 10, dex: 10, con: 10, int: 10, wis: 10, cha: 10,
    });
  });

  it('coerces a non-numeric level back to 1', () => {
    const state = baseState();
    state.classEntries = [{ class: 'Fighter', subclass: '', level: 'NaN' }];
    expect(gatherSubmission(state).classes[0].level).toBe(1);
  });
});

describe('CharCreatePanel submission round-trip via the api helpers', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('createCharacter is invoked with a campaign_id-prefixed payload', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ character_id: 'c-1', player_character_id: 'pc-1' }),
    });
    const submission = gatherSubmission(baseState());
    const payload = { campaign_id: 'camp-7', ...submission };
    const out = await createCharacter(payload);
    expect(out).toEqual({ character_id: 'c-1', player_character_id: 'pc-1' });
    const sent = JSON.parse(fetch.mock.calls[0][1].body);
    expect(sent.campaign_id).toBe('camp-7');
    expect(sent.name).toBe('Thorin');
    expect(sent.classes).toHaveLength(1);
  });

  it('previewCharacter surfaces server features in the response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({
        hp_max: 44,
        ac: 11,
        speed_ft: 25,
        proficiency_bonus: 3,
        total_level: 5,
        max_spell_level: 0,
        features: [{ name: 'Action Surge', source: 'Fighter', level: 2, description: 'X' }],
      }),
    });
    const stats = await previewCharacter(gatherSubmission(baseState()));
    expect(stats.hp_max).toBe(44);
    expect(stats.features).toHaveLength(1);
    expect(stats.features[0].name).toBe('Action Surge');
  });
});

describe('CharCreatePanel boot fetches', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('fetches races, classes, equipment, and ability methods at mount time', async () => {
    globalThis.fetch = vi.fn().mockImplementation((url) => {
      if (url.endsWith('/ref/races')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve([{ name: 'Elf' }]) });
      }
      if (url.endsWith('/ref/classes')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve([{ name: 'Wizard', subclasses: [{ name: 'Evocation' }] }]),
        });
      }
      if (url.startsWith('/dashboard/api/characters/ref/equipment')) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve([{ id: 'staff', name: 'Quarterstaff', category: 'weapon' }]),
        });
      }
      if (url.startsWith('/dashboard/api/characters/ability-methods')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve(['point_buy', 'roll']) });
      }
      throw new Error(`unexpected url ${url}`);
    });
    const [races, classes, equip, methods] = await Promise.all([
      fetchRaces(),
      fetchClasses(),
      fetchEquipment('camp-1'),
      fetchAbilityMethods('camp-1'),
    ]);
    expect(races[0].name).toBe('Elf');
    expect(classes[0].name).toBe('Wizard');
    expect(equip[0].id).toBe('staff');
    expect(methods).toEqual(['point_buy', 'roll']);
  });
});

describe('CharCreatePanel subclass resolution', () => {
  it('flattens whatever shape the server returned for the subclasses column', () => {
    const goString = JSON.stringify([{ name: 'Champion' }, { name: 'Battle Master' }]);
    const goObject = { champion: { name: 'Champion' }, fiend: { name: 'Fiend' } };
    expect(parseSubclasses(goString)).toEqual([
      { value: 'Champion', label: 'Champion' },
      { value: 'Battle Master', label: 'Battle Master' },
    ]);
    const got = parseSubclasses(goObject);
    expect(got).toHaveLength(2);
    expect(got).toContainEqual({ value: 'Champion', label: 'Champion' });
  });
});

describe('CharCreatePanel ability-roll integration', () => {
  it('roll4d6DropLowest produces a four-element dice array suitable for ability_rolls', () => {
    const state = baseState();
    const next = {};
    const rolls = {};
    for (const ab of ['str', 'dex', 'con', 'int', 'wis', 'cha']) {
      const { score, dice } = roll4d6DropLowest();
      next[ab] = score;
      rolls[ab] = dice;
    }
    state.abilityScores = next;
    state.abilityRolls = rolls;
    const sub = gatherSubmission(state);
    expect(Object.keys(sub.ability_rolls)).toHaveLength(6);
    for (const ab of ['str', 'dex', 'con', 'int', 'wis', 'cha']) {
      expect(sub.ability_rolls[ab]).toHaveLength(4);
      expect(sub.ability_scores[ab]).toBeGreaterThanOrEqual(3);
      expect(sub.ability_scores[ab]).toBeLessThanOrEqual(18);
    }
  });
});
