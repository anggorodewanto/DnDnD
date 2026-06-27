/**
 * CombatManager.svelte character-sheet link contract.
 *
 * vitest runs under the node environment with no DOM (see App.test.js), so we
 * parse the .svelte source as text and assert the markup contract: the DM can
 * jump to a player combatant's character sheet from the tracker panel, and NPCs
 * (which have no character_id) get no such link.
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const src = readFileSync(
  fileURLToPath(new URL('./CombatManager.svelte', import.meta.url)),
  'utf8',
);

describe('CombatManager character-sheet link', () => {
  it('exposes a character-sheet link for the selected combatant', () => {
    expect(src).toContain('data-testid="combatant-sheet-link"');
    expect(src).toContain('/portal/character/${selectedCombatant.character_id}`');
  });

  it('gates the sheet link on a non-null character_id so NPCs are excluded', () => {
    expect(src).toMatch(
      /\{#if selectedCombatant\.character_id\}[\s\S]*?combatant-sheet-link/,
    );
  });

  it('opens the sheet in a new tab without leaking the opener', () => {
    const block = src.match(
      /data-testid="combatant-sheet-link"[\s\S]*?<\/a>/,
    );
    expect(block).not.toBeNull();
    expect(block[0]).toContain('target="_blank"');
    expect(block[0]).toContain('rel="noopener"');
  });
});

describe('CombatManager Run Enemy Turn affordance', () => {
  it('renders a discoverable Run Enemy Turn button gated on the active NPC turn', () => {
    expect(src).toContain('data-testid="run-enemy-turn-btn"');
    // The button is only shown when the current-turn combatant is an NPC, so
    // PCs on their turn never see it.
    expect(src).toMatch(
      /\{#if activeTurnCombatant\?\.is_npc\}[\s\S]*?run-enemy-turn-btn/,
    );
  });

  it('detects the current combatant via active_turn_combatant_id', () => {
    expect(src).toMatch(
      /let activeTurnCombatant = \$derived\([\s\S]*?active_turn_combatant_id/,
    );
  });

  it('opens the Turn Builder via the shared openTurnBuilder handler', () => {
    const block = src.match(
      /class="run-enemy-turn-btn"[\s\S]*?<\/button>/,
    );
    expect(block).not.toBeNull();
    expect(block[0]).toContain('data-testid="run-enemy-turn-btn"');
    expect(block[0]).toContain('openTurnBuilder(activeTurnCombatant)');
  });

  it('routes the right-click Plan Turn through the same openTurnBuilder helper', () => {
    expect(src).toMatch(/function openTurnBuilder\(comb\)/);
    const ctx = src.match(/if \(action === 'plan-turn'\)[\s\S]*?return;/);
    expect(ctx).not.toBeNull();
    // The context-menu path reuses the helper rather than calling
    // onopenturnbuilder inline, so the open logic lives in one place.
    expect(ctx[0]).toContain('openTurnBuilder(comb)');
  });
});

describe('CombatManager tracker-panel context actions', () => {
  // The tracker panel mirrors the right-click context menu so the DM can act on
  // a selected token without hunting for the right-click. Damage/Heal/Conditions
  // already live in the panel; these assert the two NPC-menu buttons that were
  // previously right-click-only: Plan Turn (NPC) and Remove from Encounter.
  it('renders a tracker Plan Turn button gated on the selected NPC', () => {
    expect(src).toContain('data-testid="tracker-plan-turn-btn"');
    // NPC-only: a selected PC never sees Plan Turn, matching the context menu.
    expect(src).toMatch(
      /\{#if selectedCombatant\.is_npc\}[\s\S]*?tracker-plan-turn-btn/,
    );
  });

  it('opens the Turn Builder via the shared openTurnBuilder helper', () => {
    const block = src.match(
      /data-testid="tracker-plan-turn-btn"[\s\S]*?<\/button>/,
    );
    expect(block).not.toBeNull();
    expect(block[0]).toContain('openTurnBuilder(selectedCombatant)');
  });

  it('renders a tracker Remove button reusing handleRemoveCombatant', () => {
    const block = src.match(
      /data-testid="tracker-remove-btn"[\s\S]*?<\/button>/,
    );
    expect(block).not.toBeNull();
    expect(block[0]).toContain('handleRemoveCombatant(selectedCombatant.id)');
  });
});
