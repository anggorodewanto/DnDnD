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
