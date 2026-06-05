/**
 * Skill-proficiency selection rules for the character builder (5e RAW).
 *
 * Skill proficiencies come from three sources, each with different rules:
 *   - Background — 2 fixed skills (locked, non-editable).
 *   - Race       — fixed grants (e.g. Elf → Perception, locked) and/or a
 *                  "choose any N" grant (e.g. Half-Elf → 2 of any skill).
 *   - Class      — choose N from the starting class's specific list.
 * Per the no-duplicate rule, a skill already granted by background/race is
 * removed from the class pool (you pick a different one of the same kind).
 *
 * This module is pure and framework-free: it computes per-skill UI state and
 * reconciles an arbitrary skill list down to a legal set. The Svelte wiring
 * and storage live elsewhere.
 */

import { skillsForBackground } from './backgrounds.js';
import { raceGrantedSkills } from './race-skills.js';
import { SKILL_ABILITY } from './skills.js';
import { parseJSONField } from './race-perks.js';

/** Race "choose any N skills" trait code, e.g. Half-Elf's `gain_2_skill_proficiencies`. */
const RACE_CHOOSE_RE = /^gain_(\d+)_skill_proficiencies$/;

/**
 * Counts the free skill picks a race grants via `gain_<N>_skill_proficiencies`
 * trait codes (summed across traits). Most races return 0; Half-Elf returns 2.
 * @param {*} traitsRaw - trait array or JSON string
 * @returns {number}
 */
export function raceChooseSkillCount(traitsRaw) {
  const parsed = parseJSONField(traitsRaw);
  if (!Array.isArray(parsed)) return 0;

  let total = 0;
  for (const trait of parsed) {
    if (!trait || typeof trait.mechanical_effect !== 'string') continue;
    for (const rawCode of trait.mechanical_effect.split(',')) {
      const match = rawCode.trim().match(RACE_CHOOSE_RE);
      if (match) total += Number(match[1]);
    }
  }
  return total;
}

/**
 * Returns the always-granted, non-editable skills: background fixed skills
 * unioned with race fixed skills, deduped (first-seen order).
 * @param {string} background - background slug
 * @param {*} raceTraits - trait array or JSON string
 * @returns {string[]}
 */
export function lockedSkills(background, raceTraits) {
  const merged = [...skillsForBackground(background), ...raceGrantedSkills(raceTraits)];
  return [...new Set(merged)];
}

/**
 * Normalizes a class `skill_choices` blob to `{ from, choose }`.
 * @param {*} raw - object, JSON string, or null
 * @returns {{ from: string[], choose: number }}
 */
export function normalizeClassChoices(raw) {
  const parsed = parseJSONField(raw);
  if (!parsed || typeof parsed !== 'object') return { from: [], choose: 0 };
  const from = Array.isArray(parsed.from) ? parsed.from : [];
  return { from, choose: Number(parsed.choose) || 0 };
}

/** @returns {boolean} whether `skill` is one of the 18 canonical skills. */
function isCanonical(skill) {
  return Object.prototype.hasOwnProperty.call(SKILL_ABILITY, skill);
}

/**
 * Greedily assigns each chosen (non-locked) skill to a budget: a skill in the
 * class pool fills a class slot first, otherwise it consumes a race free pick.
 * @returns {{ source: Record<string,string>, classChosen: number, raceChosen: number }}
 */
function allocate(chosen, classPoolSet, classMax) {
  const source = {};
  let classChosen = 0;
  for (const s of chosen) {
    if (classPoolSet.has(s) && classChosen < classMax) {
      source[s] = 'class';
      classChosen += 1;
      continue;
    }
    source[s] = 'race';
  }
  return { source, classChosen, raceChosen: chosen.length - classChosen };
}

/**
 * Resolves the inputs to the locked set, class pool, and budgets shared by the
 * compute/reconcile/complete helpers.
 */
function resolve({ background, raceTraits, classChoices }) {
  const locked = lockedSkills(background, raceTraits);
  const lockedSet = new Set(locked);
  const { from, choose } = normalizeClassChoices(classChoices);
  const classPool = from.filter((s) => !lockedSet.has(s));
  const classPoolSet = new Set(classPool);
  const classMax = Math.min(choose, classPool.length);
  const raceMax = raceChooseSkillCount(raceTraits);
  return { locked, lockedSet, classPoolSet, classMax, raceMax };
}

/**
 * Computes per-skill UI state plus a budget summary.
 *
 * Each entry: `{ skill, checked, disabled, locked, source }` where source is
 * 'background' | 'race' (locked) or 'class' | 'race' (a discretionary pick) or
 * null (unchosen). A box is disabled when locked, or when picking it would
 * exceed the remaining class/race budget.
 *
 * @param {{ allSkills: string[], background: string, raceTraits: *, classChoices: *, selected: string[] }} args
 * @returns {{ skills: Array<object>, summary: object }}
 */
export function computeSkillState({ allSkills, background, raceTraits, classChoices, selected }) {
  const { locked, lockedSet, classPoolSet, classMax, raceMax } = resolve({ background, raceTraits, classChoices });
  const bgSet = new Set(skillsForBackground(background));
  const selectedSet = new Set(selected || []);

  const chosen = (allSkills || []).filter((s) => selectedSet.has(s) && !lockedSet.has(s));
  const { source, classChosen, raceChosen } = allocate(chosen, classPoolSet, classMax);
  const classRemaining = Math.max(0, classMax - classChosen);
  const raceRemaining = Math.max(0, raceMax - raceChosen);

  const skills = (allSkills || []).map((skill) => {
    if (lockedSet.has(skill)) {
      return { skill, checked: true, disabled: true, locked: true, source: bgSet.has(skill) ? 'background' : 'race' };
    }
    if (selectedSet.has(skill)) {
      return { skill, checked: true, disabled: false, locked: false, source: source[skill] || 'race' };
    }
    const canClass = classPoolSet.has(skill) && classRemaining > 0;
    const canRace = raceRemaining > 0;
    return { skill, checked: false, disabled: !(canClass || canRace), locked: false, source: null };
  });

  return {
    skills,
    summary: { classChosen, classMax, raceChosen, raceMax, lockedCount: locked.length },
  };
}

/**
 * Reduces an arbitrary skill list to a legal set: always includes the locked
 * skills, keeps as many valid discretionary picks as the class/race budgets
 * allow (in input order), and drops the rest. Idempotent.
 *
 * @param {{ background: string, raceTraits: *, classChoices: *, selected: string[] }} args
 * @returns {string[]}
 */
export function reconcileSkills({ background, raceTraits, classChoices, selected }) {
  const { locked, lockedSet, classPoolSet, classMax, raceMax } = resolve({ background, raceTraits, classChoices });

  const seen = new Set();
  const chosen = (selected || []).filter((s) => {
    if (!isCanonical(s) || lockedSet.has(s) || seen.has(s)) return false;
    seen.add(s);
    return true;
  });

  const kept = [];
  let classUsed = 0;
  let raceUsed = 0;
  for (const s of chosen) {
    if (classPoolSet.has(s) && classUsed < classMax) {
      kept.push(s);
      classUsed += 1;
      continue;
    }
    if (raceUsed < raceMax) {
      kept.push(s);
      raceUsed += 1;
    }
  }

  return [...new Set([...locked.filter(isCanonical), ...kept])];
}

/**
 * Reports whether both the class and race skill budgets are fully spent.
 * @param {{ background: string, raceTraits: *, classChoices: *, selected: string[] }} args
 * @returns {boolean}
 */
export function isSkillSelectionComplete({ background, raceTraits, classChoices, selected }) {
  const { classMax, raceMax } = resolve({ background, raceTraits, classChoices });
  const { summary } = computeSkillState({
    allSkills: Object.keys(SKILL_ABILITY),
    background,
    raceTraits,
    classChoices,
    selected,
  });
  return summary.classChosen >= classMax && summary.raceChosen >= raceMax;
}
