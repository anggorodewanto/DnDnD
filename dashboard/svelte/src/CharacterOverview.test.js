/**
 * CharacterOverview.svelte character-sheet link contract.
 *
 * The repo's vitest config runs under the node environment with no DOM, so
 * Svelte components can't be rendered. Following the existing pattern (see
 * App/HomePanel tests) we parse the .svelte source as text and assert the
 * markup contract the party page must honour.
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const src = readFileSync(
  fileURLToPath(new URL('./CharacterOverview.svelte', import.meta.url)),
  'utf8',
);

describe('CharacterOverview character-sheet link', () => {
  it('links each card to the player character sheet', () => {
    expect(src).toContain('data-testid="character-sheet-{c.character_id}"');
    // The read-only sheet link points at /portal/character/{id} with no /edit
    // suffix (which the separate edit link uses).
    expect(src).toContain('/portal/character/${c.character_id}`');
  });

  it('opens the sheet in a new tab without leaking the opener', () => {
    const block = src.match(
      /data-testid="character-sheet-\{c\.character_id\}"[\s\S]*?<\/a>/,
    );
    expect(block).not.toBeNull();
    expect(block[0]).toContain('target="_blank"');
    expect(block[0]).toContain('rel="noopener"');
  });
});

/**
 * DM "Edit status" affordance (out-of-combat HP / temp HP / exhaustion /
 * conditions). The vitest harness runs under the node environment with no DOM,
 * so the genuine fetch behaviour (POST URL, request body, 409 error text) is
 * covered as a real mocked-fetch unit test in lib/characterStatus.test.js. Here
 * we assert the component's markup + wiring contract against the .svelte source,
 * matching the existing text-parsing convention above.
 */
describe('CharacterOverview status editor', () => {
  it('delegates the save to the characterStatus lib helper', () => {
    expect(src).toContain("from './lib/characterStatus.js'");
    expect(src).toContain('saveCharacterStatus');
  });

  it('renders an "Edit status" toggle button on each card', () => {
    expect(src).toContain('data-testid="character-status-toggle-{c.character_id}"');
    expect(src).toContain('Edit status');
    // The toggle opens the editor for that card and closes it again.
    expect(src).toContain('openStatusEditor(c)');
    expect(src).toContain('closeStatusEditor()');
  });

  it('prefills the editor from the card\'s current overview values', () => {
    const fn = src.match(/function openStatusEditor\(c\)\s*\{[\s\S]*?\n  \}/);
    expect(fn).not.toBeNull();
    expect(fn[0]).toContain('c.hp_current');
    expect(fn[0]).toContain('c.hp_max');
    expect(fn[0]).toContain('c.temp_hp');
    expect(fn[0]).toContain('c.exhaustion_level');
    expect(fn[0]).toContain('c.conditions');
  });

  it('exposes inputs for HP, temp HP, exhaustion, conditions and reason', () => {
    expect(src).toContain('data-testid="status-hp-current"');
    expect(src).toContain('data-testid="status-hp-max"');
    expect(src).toContain('data-testid="status-temp-hp"');
    expect(src).toContain('data-testid="status-exhaustion"');
    expect(src).toContain('data-testid="status-reason"');
    // All 14 conditions are rendered as checkboxes via the shared list.
    expect(src).toContain('{#each STATUS_CONDITIONS as cond}');
    expect(src).toContain('data-testid="status-condition-{cond}"');
  });

  it('re-fetches the overview via load() after a successful save', () => {
    const fn = src.match(/async function saveStatus\(characterId\)\s*\{[\s\S]*?\n  \}/);
    expect(fn).not.toBeNull();
    // On success: clear the editor id, then reload the list.
    expect(fn[0]).toContain('await saveCharacterStatus(characterId, statusForm)');
    expect(fn[0]).toContain('editingStatusId = null');
    expect(fn[0]).toContain('await load()');
    // On error: keep the editor open (no id reset) and surface the message.
    expect(fn[0]).toContain('statusError = e.message');
    // The id is only nulled inside the try (success path), not the catch.
    const successIdx = fn[0].indexOf('editingStatusId = null');
    const catchIdx = fn[0].indexOf('} catch');
    expect(successIdx).toBeGreaterThan(-1);
    expect(successIdx).toBeLessThan(catchIdx);
  });

  it('shows the server error and a Cancel button while editing', () => {
    expect(src).toContain('data-testid="status-error-{c.character_id}"');
    expect(src).toContain('{statusError}');
    expect(src).toContain('data-testid="status-cancel-{c.character_id}"');
  });

  it('shows temp HP, exhaustion and conditions in read mode', () => {
    expect(src).toContain('temp)');
    expect(src).toContain('Exhaustion: {c.exhaustion_level');
    expect(src).toContain('Conditions:');
  });
});

/**
 * DM "Edit slots" affordance (out-of-combat spell + pact slot editing). As with
 * the status editor, the genuine fetch behaviour (POST URL, 409 in-combat text)
 * is covered as a mocked-fetch unit test in lib/api.test.js (saveCharacterSlots)
 * and the pure value-shaping in SlotEditor.test.js. Here we assert the card's
 * markup + wiring contract against the .svelte source.
 */
describe('CharacterOverview slot editor', () => {
  it('imports the reusable SlotEditor and the saveCharacterSlots helper', () => {
    expect(src).toContain("import SlotEditor");
    expect(src).toContain("from './SlotEditor.svelte'");
    expect(src).toContain('saveCharacterSlots');
    expect(src).toContain("from './lib/api.js'");
  });

  it('shows a compact read-only slot + pact summary on each card', () => {
    expect(src).toContain('formatSpellSummary(c.spell_slots)');
    expect(src).toContain('formatPactSummary(c.pact_magic_slots)');
    expect(src).toContain('data-testid="slots-summary-{c.character_id}"');
    expect(src).toContain('data-testid="pact-summary-{c.character_id}"');
  });

  it('renders an "Edit slots" toggle that opens SlotEditor seeded from the card', () => {
    expect(src).toContain('data-testid="character-slots-toggle-{c.character_id}"');
    expect(src).toContain('Edit slots');
    expect(src).toContain('openSlotEditor(c)');
    expect(src).toContain('closeSlotEditor()');
    const open = src.match(/function openSlotEditor\(c\)\s*\{[\s\S]*?\n  \}/);
    expect(open).not.toBeNull();
    expect(open[0]).toContain('c.spell_slots');
    expect(open[0]).toContain('c.pact_magic_slots');
  });

  it('wires SlotEditor onSave to saveSlots and re-fetches on success', () => {
    expect(src).toMatch(/<SlotEditor[\s\S]*?onSave=\{\(payload\) => saveSlots\(c\.character_id, payload\)\}/);
    expect(src).toMatch(/<SlotEditor[\s\S]*?onCancel=\{closeSlotEditor\}/);
    const fn = src.match(/async function saveSlots\(characterId, payload\)\s*\{[\s\S]*?\n  \}/);
    expect(fn).not.toBeNull();
    expect(fn[0]).toContain('await saveCharacterSlots(characterId, payload)');
    expect(fn[0]).toContain('editingSlotsId = null');
    expect(fn[0]).toContain('await load()');
    // On error: surface the message (the 409 in-combat text must reach the DM).
    expect(fn[0]).toContain('slotsError = e.message');
    const successIdx = fn[0].indexOf('editingSlotsId = null');
    const catchIdx = fn[0].indexOf('} catch');
    expect(successIdx).toBeGreaterThan(-1);
    expect(successIdx).toBeLessThan(catchIdx);
  });
});
