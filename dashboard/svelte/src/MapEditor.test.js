/**
 * MapEditor.svelte unsaved-changes guard contract.
 *
 * Node-env vitest can't render Svelte, so we assert on the source text (see
 * HomePanel tests for the established pattern).
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const src = readFileSync(
  fileURLToPath(new URL('./MapEditor.svelte', import.meta.url)),
  'utf8',
);

describe('MapEditor.svelte unsaved-changes guard', () => {
  it('registers its dirty state with the shared navigation guard', () => {
    expect(src).toContain("from './lib/navigationGuard.js'");
    expect(src).toContain('registerDirtyCheck');
    expect(src).toMatch(/registerDirtyCheck\(\(\)\s*=>\s*dirty\)/);
  });
});
