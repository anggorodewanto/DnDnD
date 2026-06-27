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
