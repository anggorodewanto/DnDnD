import { describe, it, expect } from 'vitest';
import { raceGrantedSkills, mergeGrantedSkills } from './race-skills.js';

// Trait fixtures copied from internal/refdata/seed_races.go shapes.
const ELF_TRAITS = [
  {
    name: 'Darkvision',
    description: 'Accustomed to twilit forests and the night sky.',
    mechanical_effect: 'darkvision_60',
  },
  {
    name: 'Keen Senses',
    description: 'You have proficiency in the Perception skill.',
    mechanical_effect: 'proficiency_perception',
  },
  {
    name: 'Fey Ancestry',
    description: 'You have advantage on saving throws against being charmed.',
    mechanical_effect: 'advantage_saves_charmed,immune_magical_sleep',
  },
];

const HALF_ORC_TRAITS = [
  {
    name: 'Menacing',
    description: 'You gain proficiency in the Intimidation skill.',
    mechanical_effect: 'proficiency_intimidation',
  },
  {
    name: 'Relentless Endurance',
    description: 'When reduced to 0 hit points but not killed outright...',
    mechanical_effect: 'drop_to_1hp_instead_of_0_once_per_long_rest',
  },
];

const DWARF_TRAITS = [
  {
    name: 'Dwarven Combat Training',
    description: 'You have proficiency with the battleaxe, handaxe, light hammer, and warhammer.',
    mechanical_effect: 'proficiency_battleaxe_handaxe_light_hammer_warhammer',
  },
  {
    name: 'Tool Proficiency',
    description: "You gain proficiency with the artisan's tools of your choice.",
    mechanical_effect: 'choose_tool_proficiency_smiths_brewers_masons',
  },
  {
    name: 'Stonecunning',
    description: 'You are considered proficient in the History skill and add double your proficiency bonus.',
    mechanical_effect: 'double_proficiency_history_stonework',
  },
];

describe('raceGrantedSkills', () => {
  it('extracts a single flat skill proficiency (Elf -> perception)', () => {
    expect(raceGrantedSkills(ELF_TRAITS)).toEqual(['perception']);
  });

  it('extracts Half-Orc Menacing -> intimidation', () => {
    expect(raceGrantedSkills(HALF_ORC_TRAITS)).toEqual(['intimidation']);
  });

  it('excludes weapon, tool, and double-proficiency codes (Dwarf -> [])', () => {
    expect(raceGrantedSkills(DWARF_TRAITS)).toEqual([]);
  });

  it('extracts a skill from a comma-separated mechanical_effect', () => {
    const traits = [
      {
        name: 'Mixed',
        description: '',
        mechanical_effect: 'resistance_poison,proficiency_stealth,advantage_saves_charmed',
      },
    ];
    expect(raceGrantedSkills(traits)).toEqual(['stealth']);
  });

  it('normalizes underscores to hyphens for multi-word skills', () => {
    const traits = [
      { name: 'X', description: '', mechanical_effect: 'proficiency_animal_handling' },
      { name: 'Y', description: '', mechanical_effect: 'proficiency_sleight_of_hand' },
    ];
    expect(raceGrantedSkills(traits)).toEqual(['animal-handling', 'sleight-of-hand']);
  });

  it('dedupes and preserves first-seen order across traits', () => {
    const traits = [
      { name: 'A', description: '', mechanical_effect: 'proficiency_perception' },
      { name: 'B', description: '', mechanical_effect: 'proficiency_stealth' },
      { name: 'C', description: '', mechanical_effect: 'proficiency_perception' },
    ];
    expect(raceGrantedSkills(traits)).toEqual(['perception', 'stealth']);
  });

  it('accepts a JSON-string traits blob', () => {
    expect(raceGrantedSkills(JSON.stringify(ELF_TRAITS))).toEqual(['perception']);
  });

  it('returns [] for null, undefined, empty string, and invalid JSON', () => {
    expect(raceGrantedSkills(null)).toEqual([]);
    expect(raceGrantedSkills(undefined)).toEqual([]);
    expect(raceGrantedSkills('')).toEqual([]);
    expect(raceGrantedSkills('{not json')).toEqual([]);
    expect(raceGrantedSkills([])).toEqual([]);
  });

  it('ignores traits with missing or empty mechanical_effect', () => {
    const traits = [
      { name: 'No effect', description: 'flavor only' },
      { name: 'Empty', description: '', mechanical_effect: '' },
      { name: 'Real', description: '', mechanical_effect: 'proficiency_survival' },
    ];
    expect(raceGrantedSkills(traits)).toEqual(['survival']);
  });

  it('excludes a proficiency_ code that is not one of the 18 known skills', () => {
    const traits = [
      { name: 'Bogus', description: '', mechanical_effect: 'proficiency_tinkers_tools' },
    ];
    expect(raceGrantedSkills(traits)).toEqual([]);
  });
});

describe('mergeGrantedSkills', () => {
  it('appends granted skills not already chosen', () => {
    expect(mergeGrantedSkills(['stealth'], ['perception'])).toEqual(['stealth', 'perception']);
  });

  it('dedupes granted skills against chosen', () => {
    expect(mergeGrantedSkills(['perception', 'stealth'], ['perception'])).toEqual([
      'perception',
      'stealth',
    ]);
  });

  it('returns a copy of chosen when granted is empty', () => {
    const chosen = ['athletics'];
    const result = mergeGrantedSkills(chosen, []);
    expect(result).toEqual(['athletics']);
    expect(result).not.toBe(chosen);
  });

  it('does not mutate either input array', () => {
    const chosen = ['stealth'];
    const granted = ['perception'];
    mergeGrantedSkills(chosen, granted);
    expect(chosen).toEqual(['stealth']);
    expect(granted).toEqual(['perception']);
  });

  it('handles null/undefined chosen and granted', () => {
    expect(mergeGrantedSkills(null, null)).toEqual([]);
    expect(mergeGrantedSkills(undefined, ['perception'])).toEqual(['perception']);
    expect(mergeGrantedSkills(['stealth'], undefined)).toEqual(['stealth']);
  });
});
