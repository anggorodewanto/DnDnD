/**
 * CharacterReviewPanel.svelte contract.
 *
 * The repo's vitest config runs under the node environment with no DOM, so
 * Svelte components can't be rendered. Following the existing pattern we parse
 * the .svelte source as text and assert the panel's data contract; the pure
 * formatting logic is unit-tested in characterReview.test.js, and the rendered
 * behaviour is verified in the browser.
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const src = readFileSync(
  fileURLToPath(new URL('../CharacterReviewPanel.svelte', import.meta.url)),
  'utf8',
);

describe('CharacterReviewPanel', () => {
  it('uses the review formatting helpers', () => {
    expect(src).toContain("from './lib/characterReview.js'");
    expect(src).toContain('reviewChanges');
    expect(src).toContain('reviewFields');
  });

  it('lazily fetches the approval detail by id with same-origin credentials', () => {
    expect(src).toContain('fetch(`/dashboard/api/approvals/${id}`');
    expect(src).toContain("credentials: 'same-origin'");
  });

  it('renders the change diff only when a before-baseline is present', () => {
    expect(src).toMatch(/\{#if detail\.review_before\}/);
    expect(src).toContain('Changes since last approval');
  });

  it('computes changes from review_before -> review', () => {
    expect(src).toContain('reviewChanges(detail.review_before, detail.review)');
  });

  it('always renders the full current-state review', () => {
    expect(src).toContain('reviewFields(detail.review)');
  });

  it('deep-links to the full character sheet', () => {
    expect(src).toContain('href={`/portal/character/${characterId}`}');
  });
});
