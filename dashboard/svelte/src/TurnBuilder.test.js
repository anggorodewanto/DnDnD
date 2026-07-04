/**
 * TurnBuilder.svelte — enemy-turn Turn Builder.
 *
 * The vitest harness runs under the `node` environment with no DOM, so the
 * component can't be mounted. The pure reaction-selection logic lives in the
 * component's `<script module>` block and is imported + exercised directly here
 * (mirroring the SlotEditor pattern). The 5b reaction-window markup contract is
 * asserted against the .svelte source text.
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { resolveChosenReaction } from './TurnBuilder.svelte';

const src = readFileSync(
  fileURLToPath(new URL('./TurnBuilder.svelte', import.meta.url)),
  'utf8',
);

const DD = { id: 'defensive-duelist', label: 'Defensive Duelist (+3 AC)', ac_bonus: 3, reason: 'Defensive Duelist' };

describe('resolveChosenReaction', () => {
  it('returns the matching option by id', () => {
    expect(resolveChosenReaction([DD], 'defensive-duelist')).toEqual(DD);
  });

  it('returns null for the "None" selection (empty id)', () => {
    expect(resolveChosenReaction([DD], '')).toBeNull();
  });

  it('returns null for an unknown id', () => {
    expect(resolveChosenReaction([DD], 'shield')).toBeNull();
  });

  it('tolerates a missing reactions list', () => {
    expect(resolveChosenReaction(undefined, 'defensive-duelist')).toBeNull();
    expect(resolveChosenReaction(null, '')).toBeNull();
  });
});

describe('TurnBuilder 5b markup contract', () => {
  it('renders a reaction selector for attacks that surface available_reactions', () => {
    expect(src).toContain('available_reactions');
    expect(src).toContain('chooseReaction(currentStep, e.target.value)');
  });

  it('carries the chosen reaction into the execute payload via the raw steps', () => {
    // executeEnemyTurn posts { combatant_id, steps: plan.steps } verbatim, so a
    // chosen_reaction set on a step's attack round-trips to the server.
    expect(src).toContain('chosen_reaction');
    expect(src).toMatch(/steps:\s*plan\.steps/);
  });

  it('echoes the chosen reaction in the review step', () => {
    expect(src).toContain('review-reaction');
  });
});
