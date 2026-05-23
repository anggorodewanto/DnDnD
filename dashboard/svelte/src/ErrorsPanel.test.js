import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const panelSrc = readFileSync(
  fileURLToPath(new URL('./ErrorsPanel.svelte', import.meta.url)),
  'utf8',
);

describe('ErrorsPanel wiring', () => {
  it('imports the JSON fetch helper from lib/errors.js (not lib/api.js)', () => {
    expect(panelSrc).toContain("from './lib/errors.js'");
    expect(panelSrc).toContain('fetchRecentErrors');
    expect(panelSrc).toContain('formatErrorTimestamp');
    expect(panelSrc).not.toContain("from './lib/api.js'");
  });

  it('calls the fetch helper from a load() function reused by the refresh button', () => {
    expect(panelSrc).toContain('await fetchRecentErrors()');
    expect(panelSrc).toContain('onclick={load}');
  });

  it('wraps the fetch in try/catch/finally so loading always clears', () => {
    expect(panelSrc).toContain('try {');
    expect(panelSrc).toContain('} catch');
    expect(panelSrc).toContain('finally');
    expect(panelSrc).toContain('loading = false');
  });

  it('triggers load() inside $effect on mount', () => {
    expect(panelSrc).toContain('$effect(() => {');
    expect(panelSrc).toMatch(/\$effect\(\(\)\s*=>\s*\{\s*load\(\);/);
  });
});

describe('ErrorsPanel UI states', () => {
  it('renders the loading, error, empty, and populated states', () => {
    expect(panelSrc).toMatch(/\{#if loading\}/);
    expect(panelSrc).toMatch(/\{:else if error\}/);
    expect(panelSrc).toMatch(/\{:else if entries\.length === 0\}/);
    expect(panelSrc).toContain('No errors logged in the last 24 hours.');
  });

  it('shows the 24h header title', () => {
    expect(panelSrc).toContain('Errors (last 24h)');
  });

  it('renders a table with timestamp, command, player, and summary columns', () => {
    expect(panelSrc).toContain('<th class="col-time">Timestamp</th>');
    expect(panelSrc).toContain('<th class="col-command">Command</th>');
    expect(panelSrc).toContain('<th class="col-user">Player</th>');
    expect(panelSrc).toContain('<th class="col-summary">Summary</th>');
  });

  it('formats command and user with leading / and @, falling back to em-dash', () => {
    expect(panelSrc).toContain("entry.command ? `/${entry.command}` : '—'");
    expect(panelSrc).toContain("entry.user_id ? `@${entry.user_id}` : '—'");
  });
});

describe('ErrorsPanel campaign prop', () => {
  it('accepts a campaignId prop for shell-uniformity even though the server resolves the session', () => {
    expect(panelSrc).toContain("let { campaignId = '' } = $props();");
  });
});
