/**
 * App.svelte unsaved-changes guard contract.
 *
 * The repo's vitest config runs under the node environment with no DOM, so
 * Svelte components can't be rendered. Following the existing pattern (see
 * HomePanel/CampaignsPage tests) we parse the .svelte source as text and assert
 * the navigation-guard contract the shell must honour.
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const src = readFileSync(
  fileURLToPath(new URL('./App.svelte', import.meta.url)),
  'utf8',
);

describe('App.svelte unsaved-changes guard', () => {
  it('imports the navigation-guard helpers', () => {
    expect(src).toContain("from './lib/navigationGuard.js'");
    expect(src).toContain('confirmDiscard');
    expect(src).toContain('beforeUnloadHandler');
  });

  it('registers and tears down a beforeunload listener', () => {
    expect(src).toContain("addEventListener('beforeunload'");
    expect(src).toContain("removeEventListener('beforeunload'");
  });

  it('routes the discard prompt through window.confirm', () => {
    expect(src).toContain('window.confirm');
    expect(src).toContain('confirmDiscard(');
  });

  it('guards every in-app exit from the editors with a discard confirmation', () => {
    // onBack (map), onBackFromEncounter, and navigateTo (sidebar) each bail
    // when the user declines the discard prompt — three guard sites minimum.
    const callSites = src.match(/mayDiscardChanges\(\)/g) || [];
    expect(callSites.length).toBeGreaterThanOrEqual(3);
  });
});
