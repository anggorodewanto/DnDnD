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

describe('CombatManager stacked-token selection', () => {
  // Co-located tokens fully overlap on the canvas; a plain left-click must
  // cycle through them so each stacked combatant is reachable.
  it('cycles selection through the tile stack on click', () => {
    const block = src.match(/function handleCanvasClick[\s\S]*?\n  }/);
    expect(block).not.toBeNull();
    expect(block[0]).toContain('combatantsAtTile(');
    expect(block[0]).toContain('nextStackedSelection(stack, selectedCombatantId)');
  });

  it('targets the selected-or-top token for drag and right-click', () => {
    expect(src).toMatch(
      /function combatantAtFor[\s\S]*?stack\.find\(c => c\.id === selectedCombatantId\) \|\| stack\[0\]/,
    );
    // Both drag-start and the context menu route through the shared picker.
    const down = src.match(/function handleCanvasMouseDown[\s\S]*?\n  }/);
    expect(down[0]).toContain('combatantAtFor(tile.col, tile.row)');
    const ctx = src.match(/function handleCanvasContextMenu[\s\S]*?\n  }/);
    expect(ctx[0]).toContain('combatantAtFor(tile.col, tile.row)');
  });

  it('draws an ×N badge on tiles holding more than one combatant', () => {
    const block = src.match(/function drawTokens[\s\S]*?\n  }/);
    expect(block).not.toBeNull();
    expect(block[0]).toContain('stackedTileCounts(activeEncounter.combatants)');
    expect(block[0]).toContain('`×${count}`');
  });
});

describe('CombatManager spell/pact slot override', () => {
  // The raw-JSON spell-slots textarea was replaced with the reusable
  // SlotEditor, seeded from a fresh GET and saved through the in-combat
  // override endpoint. The genuine fetch behaviour is covered in
  // lib/api.test.js; here we assert the .svelte wiring contract.
  it('drops the legacy raw-JSON spell-slots control', () => {
    expect(src).not.toContain('dmOverrideSpellSlotsText');
    expect(src).not.toContain('overrideCharacterSpellSlots');
    expect(src).not.toContain('data-testid="override-spell-slots"');
  });

  it('mounts the reusable SlotEditor for a player combatant', () => {
    expect(src).toContain("import SlotEditor from './SlotEditor.svelte'");
    expect(src).toMatch(/\{#if selectedCombatant\.character_id\}[\s\S]*?<SlotEditor/);
    expect(src).toContain('data-testid="override-slots-open-btn"');
    expect(src).toMatch(/<SlotEditor[\s\S]*?onSave=\{handleOverrideSlots\}/);
    expect(src).toMatch(/<SlotEditor[\s\S]*?onCancel=\{closeSlotEditor\}/);
  });

  it('seeds the editor from a fresh getCharacterSlots read', () => {
    const fn = src.match(/async function openSlotEditor\(\)\s*\{[\s\S]*?\n  \}/);
    expect(fn).not.toBeNull();
    expect(fn[0]).toContain('getCharacterSlots(selectedCombatant.character_id)');
    expect(fn[0]).toContain('slotEditorSpell = data.spell_slots');
    expect(fn[0]).toContain('slotEditorPact = data.pact_magic_slots');
  });

  it('saves through overrideCharacterSlots then reloads the workspace', () => {
    const fn = src.match(/async function handleOverrideSlots\(payload\)\s*\{[\s\S]*?\n  \}/);
    expect(fn).not.toBeNull();
    expect(fn[0]).toContain('overrideCharacterSlots(activeEncounter.id, selectedCombatant.character_id, payload)');
    expect(fn[0]).toContain('await loadWorkspace()');
    // Keeps the existing dmOverrideMessage success/error pattern.
    expect(fn[0]).toContain('dmOverrideMessage');
    expect(fn[0]).toContain('slotEditorError = e.message');
  });
});

describe('CombatManager feature-uses override', () => {
  // Mirrors the spell/pact slot override: a reusable FeatureUsesEditor seeded
  // from a fresh GET and saved one feature at a time through the in-combat
  // override endpoint. The fetch behaviour is covered in lib/api.test.js;
  // here we assert the .svelte wiring contract.
  it('mounts the reusable FeatureUsesEditor for a player combatant', () => {
    expect(src).toContain("import FeatureUsesEditor from './FeatureUsesEditor.svelte'");
    expect(src).toMatch(/\{#if selectedCombatant\.character_id\}[\s\S]*?<FeatureUsesEditor/);
    expect(src).toContain('data-testid="override-feature-uses-open-btn"');
    expect(src).toMatch(/<FeatureUsesEditor[\s\S]*?onSave=\{handleOverrideFeatureUses\}/);
    expect(src).toMatch(/<FeatureUsesEditor[\s\S]*?onCancel=\{closeFeatureUsesEditor\}/);
  });

  it('seeds the editor from a fresh getCharacterFeatureUses read', () => {
    const fn = src.match(/async function openFeatureUsesEditor\(\)\s*\{[\s\S]*?\n  \}/);
    expect(fn).not.toBeNull();
    expect(fn[0]).toContain('getCharacterFeatureUses(selectedCombatant.character_id)');
    expect(fn[0]).toContain('featureUsesEditorSeed = data.feature_uses');
  });

  it('saves each change through overrideCharacterFeatureUses then reloads the workspace', () => {
    const fn = src.match(/async function handleOverrideFeatureUses\(payload\)\s*\{[\s\S]*?\n  \}/);
    expect(fn).not.toBeNull();
    expect(fn[0]).toContain('for (const change of payload.changes)');
    expect(fn[0]).toContain('overrideCharacterFeatureUses(activeEncounter.id, selectedCombatant.character_id,');
    expect(fn[0]).toContain('feature: change.feature');
    expect(fn[0]).toContain('current: change.current');
    expect(fn[0]).toContain('reason: payload.reason');
    expect(fn[0]).toContain("dmOverrideMessage = 'Feature uses override saved.'");
    expect(fn[0]).toContain('await loadWorkspace()');
    expect(fn[0]).toContain('featureUsesEditorError = e.message');
  });
});
