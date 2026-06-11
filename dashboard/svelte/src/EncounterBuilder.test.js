/**
 * EncounterBuilder.svelte unsaved-changes guard contract.
 *
 * Node-env vitest can't render Svelte, so we assert on the source text (see
 * HomePanel tests for the established pattern).
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const src = readFileSync(
  fileURLToPath(new URL('./EncounterBuilder.svelte', import.meta.url)),
  'utf8',
);

describe('EncounterBuilder.svelte unsaved-changes guard', () => {
  it('registers its dirty state with the shared navigation guard', () => {
    expect(src).toContain("from './lib/navigationGuard.js'");
    expect(src).toContain('registerDirtyCheck');
    expect(src).toMatch(/registerDirtyCheck\(\(\)\s*=>\s*dirty\)/);
  });
});

describe('EncounterBuilder.svelte map-required guard (T21)', () => {
  it('derives a needsMap flag for new encounters without a map', () => {
    expect(src).toMatch(/needsMap\s*=\s*\$derived\(!savedEncounterId\s*&&\s*!mapSelected\)/);
  });

  it('blocks saving a new encounter without a map', () => {
    expect(src).toMatch(/if\s*\(needsMap\)\s*{/);
    expect(src).toContain('Select a map before saving');
  });

  it('disables the save button when a map is required', () => {
    expect(src).toMatch(/disabled=\{saving\s*\|\|\s*!dirty\s*\|\|\s*needsMap\}/);
  });
});
