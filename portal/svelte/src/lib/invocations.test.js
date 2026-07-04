import { describe, it, expect } from 'vitest';
import {
  invocationsKnown,
  pactBoonGranted,
  warlockLevelOf,
  hasClassFeatureChoices,
  computeInvocationState,
  reconcileInvocations,
  reconcilePactBoon,
  pactBoonList,
} from './invocations.js';

describe('invocationsKnown', () => {
  it('matches the 2014 PHB warlock table', () => {
    const table = {
      1: 0, 2: 2, 4: 2, 5: 3, 6: 3, 7: 4, 8: 4, 9: 5,
      11: 5, 12: 6, 14: 6, 15: 7, 17: 7, 18: 8, 20: 8,
    };
    for (const [lvl, want] of Object.entries(table)) {
      expect(invocationsKnown(Number(lvl))).toBe(want);
    }
  });
});

describe('pactBoonGranted', () => {
  it('is true from level 3', () => {
    expect(pactBoonGranted(2)).toBe(false);
    expect(pactBoonGranted(3)).toBe(true);
    expect(pactBoonGranted(20)).toBe(true);
  });
});

describe('warlockLevelOf / hasClassFeatureChoices', () => {
  it('finds the warlock level in a multiclass list', () => {
    expect(warlockLevelOf([{ class: 'fighter', level: 2 }, { class: 'Warlock', level: 4 }])).toBe(4);
    expect(warlockLevelOf([{ class: 'wizard', level: 5 }])).toBe(0);
    expect(warlockLevelOf([])).toBe(0);
  });

  it('shows the step from warlock level 2', () => {
    expect(hasClassFeatureChoices([{ class: 'warlock', level: 1 }])).toBe(false);
    expect(hasClassFeatureChoices([{ class: 'warlock', level: 2 }])).toBe(true);
    expect(hasClassFeatureChoices([{ class: 'fighter', level: 5 }])).toBe(false);
  });
});

describe('computeInvocationState prerequisites', () => {
  function optionFor(id, args) {
    return computeInvocationState(args).options.find((o) => o.id === id);
  }

  it('gates Agonizing Blast on the eldritch blast cantrip', () => {
    const without = optionFor('agonizing_blast', { warlockLevel: 4, pactBoon: '', knownSpells: [], selected: [] });
    expect(without.available).toBe(false);
    expect(without.disabled).toBe(true);
    expect(without.reason).toMatch(/Eldritch Blast/i);

    const withEB = optionFor('agonizing_blast', { warlockLevel: 4, pactBoon: '', knownSpells: ['eldritch-blast'], selected: [] });
    expect(withEB.available).toBe(true);
    expect(withEB.disabled).toBe(false);
  });

  it('gates Thirsting Blade on Pact of the Blade and level 5', () => {
    const low = optionFor('thirsting_blade', { warlockLevel: 4, pactBoon: 'pact_of_the_blade', knownSpells: [], selected: [] });
    expect(low.available).toBe(false);

    const ok = optionFor('thirsting_blade', { warlockLevel: 5, pactBoon: 'pact_of_the_blade', knownSpells: [], selected: [] });
    expect(ok.available).toBe(true);

    const wrongBoon = optionFor('thirsting_blade', { warlockLevel: 5, pactBoon: 'pact_of_the_tome', knownSpells: [], selected: [] });
    expect(wrongBoon.available).toBe(false);
  });

  it('passes each invocation edition through for the picker badge', () => {
    const args = { warlockLevel: 9, pactBoon: '', knownSpells: [], selected: [] };
    const mind = optionFor('eldritch_mind', args);
    expect(mind.edition).toBe('2024');
    const carryover = optionFor('devils_sight', args);
    expect(carryover.edition).toBe(''); // edition-agnostic (both PHBs)
  });

  it('disables unchecked options once the grant is spent, keeps checked toggleable', () => {
    // Warlock 2 grants 2. Pick 2 prereq-free invocations => a third unchecked is disabled.
    const state = computeInvocationState({
      warlockLevel: 2,
      pactBoon: '',
      knownSpells: [],
      selected: ['devils_sight', 'beguiling_influence'],
    });
    expect(state.max).toBe(2);
    expect(state.chosen).toBe(2);
    const chosen = state.options.find((o) => o.id === 'devils_sight');
    expect(chosen.checked).toBe(true);
    expect(chosen.disabled).toBe(false); // can still toggle off
    const other = state.options.find((o) => o.id === 'eldritch_sight');
    expect(other.checked).toBe(false);
    expect(other.disabled).toBe(true); // grant spent
  });
});

describe('reconcileInvocations', () => {
  it('caps at the grant and drops unknown/duplicate/prereq-failing picks', () => {
    const kept = reconcileInvocations({
      warlockLevel: 2, // grant 2
      pactBoon: '',
      knownSpells: [], // no eldritch blast
      selected: [
        'agonizing_blast', // prereq fail (no EB) -> dropped
        'not_real', // unknown -> dropped
        'devils_sight', // keep #1
        'devils_sight', // dup -> dropped
        'beguiling_influence', // keep #2
        'eldritch_sight', // over cap -> dropped
      ],
    });
    expect(kept).toEqual(['devils_sight', 'beguiling_influence']);
  });

  it('drops a boon-gated invocation when the boon changes away', () => {
    const args = { warlockLevel: 5, knownSpells: [], selected: ['book_of_ancient_secrets'] };
    expect(reconcileInvocations({ ...args, pactBoon: 'pact_of_the_tome' })).toEqual(['book_of_ancient_secrets']);
    expect(reconcileInvocations({ ...args, pactBoon: 'pact_of_the_blade' })).toEqual([]);
  });

  it('returns [] below warlock level 2', () => {
    expect(reconcileInvocations({ warlockLevel: 1, pactBoon: '', knownSpells: [], selected: ['devils_sight'] })).toEqual([]);
  });
});

describe('reconcilePactBoon', () => {
  it('clears below level 3 and for unknown ids, keeps a valid pick', () => {
    expect(reconcilePactBoon({ warlockLevel: 2, pactBoon: 'pact_of_the_blade' })).toBe('');
    expect(reconcilePactBoon({ warlockLevel: 3, pactBoon: 'not_a_boon' })).toBe('');
    expect(reconcilePactBoon({ warlockLevel: 3, pactBoon: 'pact_of_the_blade' })).toBe('pact_of_the_blade');
  });
});

describe('pactBoonList', () => {
  it('exposes the four boons', () => {
    const ids = pactBoonList().map((b) => b.id);
    expect(ids).toContain('pact_of_the_blade');
    expect(ids).toContain('pact_of_the_chain');
    expect(ids).toContain('pact_of_the_tome');
    expect(ids).toContain('pact_of_the_talisman');
  });
});
