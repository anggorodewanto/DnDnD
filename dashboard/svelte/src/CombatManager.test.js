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
