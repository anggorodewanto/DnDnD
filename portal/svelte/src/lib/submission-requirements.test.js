import { describe, it, expect } from 'vitest';
import { submissionRequirements, canSubmit } from './submission-requirements.js';

const fullRolls = () => ({
  str: [6, 5, 4, 1],
  dex: [6, 5, 4, 1],
  con: [6, 5, 4, 1],
  int: [6, 5, 4, 1],
  wis: [6, 5, 4, 1],
  cha: [6, 5, 4, 1],
});

describe('submissionRequirements', () => {
  it('lists exactly name, race, class for non-roll methods, all met', () => {
    const reqs = submissionRequirements({
      name: 'Aria',
      race: 'elf',
      selectedClass: 'wizard',
      abilityMethod: 'point_buy',
    });
    expect(reqs.map((r) => r.key)).toEqual(['name', 'race', 'class']);
    expect(reqs.every((r) => r.met)).toBe(true);
    expect(canSubmit(reqs)).toBe(true);
  });

  it('uses the exact labels in order', () => {
    const reqs = submissionRequirements({
      name: 'Aria',
      race: 'elf',
      selectedClass: 'wizard',
      abilityMethod: 'roll',
      abilityRolls: fullRolls(),
    });
    expect(reqs.map((r) => r.label)).toEqual([
      'Name your character',
      'Choose a race',
      'Choose a class',
      'Roll your ability scores',
    ]);
  });

  it('marks each base requirement unmet when its field is missing', () => {
    const reqs = submissionRequirements({ abilityMethod: 'standard_array' });
    expect(reqs.map((r) => [r.key, r.met])).toEqual([
      ['name', false],
      ['race', false],
      ['class', false],
    ]);
    expect(canSubmit(reqs)).toBe(false);
  });

  it("treats a whitespace-only name as unmet", () => {
    const reqs = submissionRequirements({
      name: '   ',
      race: 'elf',
      selectedClass: 'wizard',
    });
    const name = reqs.find((r) => r.key === 'name');
    expect(name.met).toBe(false);
  });

  describe("when abilityMethod is 'roll'", () => {
    it('adds a rolls requirement, unmet when nothing has been rolled', () => {
      const reqs = submissionRequirements({
        name: 'Aria',
        race: 'elf',
        selectedClass: 'wizard',
        abilityMethod: 'roll',
        abilityRolls: {},
      });
      expect(reqs.map((r) => r.key)).toEqual(['name', 'race', 'class', 'rolls']);
      const rolls = reqs.find((r) => r.key === 'rolls');
      expect(rolls.met).toBe(false);
      expect(canSubmit(reqs)).toBe(false);
    });

    it('treats undefined abilityRolls as not-yet-rolled', () => {
      const reqs = submissionRequirements({
        name: 'Aria',
        race: 'elf',
        selectedClass: 'wizard',
        abilityMethod: 'roll',
      });
      const rolls = reqs.find((r) => r.key === 'rolls');
      expect(rolls.met).toBe(false);
    });

    it('is met when all six abilities have at least four dice', () => {
      const reqs = submissionRequirements({
        name: 'Aria',
        race: 'elf',
        selectedClass: 'wizard',
        abilityMethod: 'roll',
        abilityRolls: fullRolls(),
      });
      const rolls = reqs.find((r) => r.key === 'rolls');
      expect(rolls.met).toBe(true);
      expect(canSubmit(reqs)).toBe(true);
    });

    it('is unmet when only some abilities have been rolled', () => {
      const reqs = submissionRequirements({
        name: 'Aria',
        race: 'elf',
        selectedClass: 'wizard',
        abilityMethod: 'roll',
        abilityRolls: { str: [6, 5, 4, 1] },
      });
      const rolls = reqs.find((r) => r.key === 'rolls');
      expect(rolls.met).toBe(false);
    });

    it('is unmet when an ability has fewer than four dice', () => {
      const rolls = fullRolls();
      rolls.cha = [6, 5, 4];
      const reqs = submissionRequirements({
        name: 'Aria',
        race: 'elf',
        selectedClass: 'wizard',
        abilityMethod: 'roll',
        abilityRolls: rolls,
      });
      const rollsReq = reqs.find((r) => r.key === 'rolls');
      expect(rollsReq.met).toBe(false);
    });
  });

  it.each(['point_buy', 'standard_array', undefined])(
    'never includes a rolls requirement for method %s',
    (method) => {
      const reqs = submissionRequirements({
        name: 'Aria',
        race: 'elf',
        selectedClass: 'wizard',
        abilityMethod: method,
      });
      expect(reqs.map((r) => r.key)).toEqual(['name', 'race', 'class']);
    },
  );
});

describe('canSubmit', () => {
  it('is true only when every requirement is met', () => {
    expect(canSubmit([{ met: true }, { met: true }])).toBe(true);
    expect(canSubmit([{ met: true }, { met: false }])).toBe(false);
  });

  it('is true for an empty list', () => {
    expect(canSubmit([])).toBe(true);
  });
});
