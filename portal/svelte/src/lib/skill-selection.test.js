import { describe, it, expect } from 'vitest';
import {
  raceChooseSkillCount,
  lockedSkills,
  normalizeClassChoices,
  computeSkillState,
  reconcileSkills,
  isSkillSelectionComplete,
} from './skill-selection.js';

// --- Fixtures (shapes mirror the DB seed data) -----------------------------

const WARLOCK_CHOICES = {
  from: ['arcana', 'deception', 'history', 'intimidation', 'investigation', 'nature', 'religion'],
  choose: 2,
};

// Tiefling grants no skills (no proficiency_* and no gain_*_skill_proficiencies).
const TIEFLING_TRAITS = [
  { name: 'Darkvision', description: '…', mechanical_effect: 'darkvision_60' },
  { name: 'Hellish Resistance', description: '…', mechanical_effect: 'resist_fire' },
  { name: 'Infernal Legacy', description: '…', mechanical_effect: 'innate_spell_thaumaturgy' },
];

const ELF_TRAITS = [
  { name: 'Keen Senses', description: '…', mechanical_effect: 'proficiency_perception' },
];

const HALF_ELF_TRAITS = [
  { name: 'Fey Ancestry', description: '…', mechanical_effect: 'advantage_saves_charmed' },
  { name: 'Skill Versatility', description: '…', mechanical_effect: 'gain_2_skill_proficiencies' },
];

const ALL_SKILLS = [
  'acrobatics', 'animal-handling', 'arcana', 'athletics', 'deception',
  'history', 'insight', 'intimidation', 'investigation', 'medicine',
  'nature', 'perception', 'performance', 'persuasion', 'religion',
  'sleight-of-hand', 'stealth', 'survival',
];

function byName(state, skill) {
  return state.skills.find((s) => s.skill === skill);
}

// --- raceChooseSkillCount --------------------------------------------------

describe('raceChooseSkillCount', () => {
  it('returns 0 for races without a choose grant', () => {
    expect(raceChooseSkillCount(TIEFLING_TRAITS)).toBe(0);
    expect(raceChooseSkillCount(ELF_TRAITS)).toBe(0);
  });

  it('parses gain_N_skill_proficiencies (half-elf = 2)', () => {
    expect(raceChooseSkillCount(HALF_ELF_TRAITS)).toBe(2);
  });

  it('sums multiple grants and tolerates a JSON string', () => {
    const traits = [
      { mechanical_effect: 'gain_1_skill_proficiencies,darkvision_60' },
      { mechanical_effect: 'gain_2_skill_proficiencies' },
    ];
    expect(raceChooseSkillCount(traits)).toBe(3);
    expect(raceChooseSkillCount(JSON.stringify(HALF_ELF_TRAITS))).toBe(2);
  });

  it('returns 0 for junk input', () => {
    expect(raceChooseSkillCount(null)).toBe(0);
    expect(raceChooseSkillCount('not json')).toBe(0);
    expect(raceChooseSkillCount(undefined)).toBe(0);
  });
});

// --- lockedSkills ----------------------------------------------------------

describe('lockedSkills', () => {
  it('unions background + race fixed skills, deduped', () => {
    expect(lockedSkills('acolyte', ELF_TRAITS).sort()).toEqual(
      ['insight', 'perception', 'religion'].sort(),
    );
  });

  it('returns only background skills when race grants none', () => {
    expect(lockedSkills('acolyte', TIEFLING_TRAITS).sort()).toEqual(['insight', 'religion']);
  });

  it('dedups when background and race overlap', () => {
    // criminal = deception, stealth; a race granting stealth must not double it.
    const stealthRace = [{ mechanical_effect: 'proficiency_stealth' }];
    expect(lockedSkills('criminal', stealthRace).sort()).toEqual(['deception', 'stealth']);
  });

  it('returns [] for empty background and no traits', () => {
    expect(lockedSkills('', null)).toEqual([]);
  });
});

// --- normalizeClassChoices -------------------------------------------------

describe('normalizeClassChoices', () => {
  it('normalizes an object blob', () => {
    expect(normalizeClassChoices(WARLOCK_CHOICES)).toEqual({
      from: WARLOCK_CHOICES.from,
      choose: 2,
    });
  });

  it('parses a JSON string', () => {
    expect(normalizeClassChoices(JSON.stringify(WARLOCK_CHOICES)).choose).toBe(2);
  });

  it('returns an empty pool for null/garbage', () => {
    expect(normalizeClassChoices(null)).toEqual({ from: [], choose: 0 });
    expect(normalizeClassChoices('nope')).toEqual({ from: [], choose: 0 });
  });
});

// --- computeSkillState -----------------------------------------------------

describe('computeSkillState', () => {
  const base = {
    allSkills: ALL_SKILLS,
    background: 'acolyte',
    raceTraits: TIEFLING_TRAITS,
    classChoices: WARLOCK_CHOICES,
  };

  it('locks background skills: checked, disabled, locked', () => {
    const state = computeSkillState({ ...base, selected: ['insight', 'religion'] });
    const insight = byName(state, 'insight');
    expect(insight).toMatchObject({ checked: true, disabled: true, locked: true });
    expect(byName(state, 'religion')).toMatchObject({ checked: true, disabled: true, locked: true });
  });

  it('excludes locked skills from the class pool (dedup)', () => {
    // religion is in the warlock list AND granted by acolyte -> it is locked,
    // not an available class pick.
    const state = computeSkillState({ ...base, selected: ['insight', 'religion'] });
    expect(byName(state, 'religion').source).toBe('background');
    // arcana is a fresh class option -> selectable.
    expect(byName(state, 'arcana')).toMatchObject({ checked: false, disabled: false, locked: false });
  });

  it('caps class picks at the choose count', () => {
    const state = computeSkillState({ ...base, selected: ['insight', 'religion', 'arcana', 'deception'] });
    expect(state.summary).toMatchObject({ classChosen: 2, classMax: 2, raceChosen: 0, raceMax: 0 });
    // budget full: another in-pool, unchosen skill is now disabled.
    expect(byName(state, 'history')).toMatchObject({ checked: false, disabled: true });
    // chosen ones stay enabled so they can be unchecked.
    expect(byName(state, 'arcana')).toMatchObject({ checked: true, disabled: false, source: 'class' });
  });

  it('disables off-list skills when the race grants no free choice (RC=0)', () => {
    const state = computeSkillState({ ...base, selected: ['insight', 'religion'] });
    // stealth is not in the warlock list and tiefling grants no free pick.
    expect(byName(state, 'stealth')).toMatchObject({ checked: false, disabled: true });
  });

  it('marks race-fixed skills as locked with a race source', () => {
    const state = computeSkillState({
      ...base,
      raceTraits: ELF_TRAITS,
      selected: ['insight', 'religion', 'perception'],
    });
    expect(byName(state, 'perception')).toMatchObject({ checked: true, disabled: true, locked: true, source: 'race' });
  });

  it('allows any off-list skill while race free picks remain (half-elf RC=2)', () => {
    const state = computeSkillState({
      ...base,
      raceTraits: HALF_ELF_TRAITS,
      selected: ['insight', 'religion', 'arcana', 'deception'],
    });
    // class budget full (arcana/deception), but 2 race picks remain ->
    // an off-list skill like stealth is still selectable.
    expect(state.summary).toMatchObject({ classChosen: 2, raceChosen: 0, raceMax: 2 });
    expect(byName(state, 'stealth')).toMatchObject({ checked: false, disabled: false });
  });

  it('allocates extra class-list picks to the race budget (half-elf)', () => {
    const state = computeSkillState({
      ...base,
      raceTraits: HALF_ELF_TRAITS,
      // 3 warlock-list picks: 2 fill class, 1 overflows into race.
      selected: ['insight', 'religion', 'arcana', 'deception', 'history', 'stealth'],
    });
    expect(state.summary).toMatchObject({ classChosen: 2, raceChosen: 2, raceMax: 2 });
    // both budgets full -> a fresh skill is disabled.
    expect(byName(state, 'acrobatics')).toMatchObject({ checked: false, disabled: true });
  });

  it('with no class chosen, no skill is selectable unless race grants picks', () => {
    const state = computeSkillState({
      allSkills: ALL_SKILLS,
      background: 'acolyte',
      raceTraits: TIEFLING_TRAITS,
      classChoices: null,
      selected: ['insight', 'religion'],
    });
    expect(byName(state, 'arcana')).toMatchObject({ disabled: true });
    expect(state.summary).toMatchObject({ classChosen: 0, classMax: 0 });
  });
});

// --- reconcileSkills -------------------------------------------------------

describe('reconcileSkills', () => {
  it('prunes an over-selected draft to a legal set (the stale-draft bug)', () => {
    // All 18 checked under tiefling warlock acolyte -> must collapse to
    // locked (insight, religion) + exactly 2 warlock-list picks.
    const result = reconcileSkills({
      background: 'acolyte',
      raceTraits: TIEFLING_TRAITS,
      classChoices: WARLOCK_CHOICES,
      selected: [...ALL_SKILLS],
    });
    expect(result).toContain('insight');
    expect(result).toContain('religion');
    expect(result.length).toBe(4);
    const extras = result.filter((s) => s !== 'insight' && s !== 'religion');
    extras.forEach((s) => expect(WARLOCK_CHOICES.from).toContain(s));
  });

  it('always keeps locked skills even if absent from the input', () => {
    const result = reconcileSkills({
      background: 'acolyte',
      raceTraits: ELF_TRAITS,
      classChoices: WARLOCK_CHOICES,
      selected: ['arcana'],
    });
    expect(result).toEqual(expect.arrayContaining(['insight', 'religion', 'perception', 'arcana']));
  });

  it('keeps off-list picks within the race free-pick budget', () => {
    const result = reconcileSkills({
      background: 'acolyte',
      raceTraits: HALF_ELF_TRAITS,
      classChoices: WARLOCK_CHOICES,
      selected: ['arcana', 'deception', 'stealth', 'acrobatics', 'survival'],
    });
    // 2 class picks (arcana, deception) + 2 race picks kept, 1 dropped.
    expect(result.length).toBe(6); // insight, religion + 4 chosen
    expect(result).toContain('arcana');
    expect(result).toContain('deception');
  });

  it('drops class picks no longer on the list after a class switch', () => {
    // Picked under rogue (stealth legal); switching to warlock makes it illegal.
    const result = reconcileSkills({
      background: 'acolyte',
      raceTraits: TIEFLING_TRAITS,
      classChoices: WARLOCK_CHOICES,
      selected: ['insight', 'religion', 'stealth', 'arcana'],
    });
    expect(result).not.toContain('stealth');
    expect(result).toContain('arcana');
  });

  it('is idempotent', () => {
    const args = {
      background: 'acolyte',
      raceTraits: HALF_ELF_TRAITS,
      classChoices: WARLOCK_CHOICES,
      selected: [...ALL_SKILLS],
    };
    const once = reconcileSkills(args);
    const twice = reconcileSkills({ ...args, selected: once });
    expect(twice.slice().sort()).toEqual(once.slice().sort());
  });
});

// --- isSkillSelectionComplete ----------------------------------------------

describe('isSkillSelectionComplete', () => {
  const base = { background: 'acolyte', classChoices: WARLOCK_CHOICES };

  it('is false until both class and race budgets are filled', () => {
    expect(isSkillSelectionComplete({ ...base, raceTraits: HALF_ELF_TRAITS, selected: ['insight', 'religion', 'arcana', 'deception'] })).toBe(false);
  });

  it('is true when class and race budgets are exactly met', () => {
    expect(isSkillSelectionComplete({ ...base, raceTraits: TIEFLING_TRAITS, selected: ['insight', 'religion', 'arcana', 'deception'] })).toBe(true);
  });
});
