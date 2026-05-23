/**
 * DiscordHealthBanner structural tests.
 *
 * The vitest config runs under node without a DOM, so we mirror the existing
 * panel tests (e.g. HomePanel.test.js) and parse the .svelte source as text
 * to assert on the contract the banner must honour: data source, conditional
 * render, dismiss control, and failure list shape.
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const bannerSrc = readFileSync(
  fileURLToPath(new URL('./DiscordHealthBanner.svelte', import.meta.url)),
  'utf8',
);

describe('DiscordHealthBanner.svelte', () => {
  it('fetches the report from lib/discordChecks.js on mount', () => {
    expect(bannerSrc).toContain("from './lib/discordChecks.js'");
    expect(bannerSrc).toContain('fetchDiscordChecks');
    expect(bannerSrc).toContain('$effect');
  });

  it('uses failingChecks to filter the report down to errors', () => {
    expect(bannerSrc).toContain('failingChecks');
  });

  it('renders nothing when there are no failures', () => {
    expect(bannerSrc).toContain('shouldRender');
    expect(bannerSrc).toMatch(/\{#if shouldRender\(\)\}/);
  });

  it('lists every failure with name + detail when rendered', () => {
    expect(bannerSrc).toMatch(/\{#each failures as failure\}/);
    expect(bannerSrc).toContain('failure.name');
    expect(bannerSrc).toContain('failure.detail');
  });

  it('exposes a dismiss control that hides the banner via local state', () => {
    expect(bannerSrc).toContain('dismissed');
    expect(bannerSrc).toContain('aria-label="Dismiss"');
    expect(bannerSrc).toMatch(/onclick=\{dismiss\}/);
  });

  it('marks the alert with role="alert" so screen readers surface it', () => {
    expect(bannerSrc).toContain('role="alert"');
  });
});

describe('DiscordHealthBanner App integration', () => {
  const appSrc = readFileSync(
    fileURLToPath(new URL('./App.svelte', import.meta.url)),
    'utf8',
  );

  it('App.svelte imports and mounts DiscordHealthBanner above the main view', () => {
    expect(appSrc).toContain("import DiscordHealthBanner from './DiscordHealthBanner.svelte';");
    expect(appSrc).toContain('<DiscordHealthBanner');
  });
});
