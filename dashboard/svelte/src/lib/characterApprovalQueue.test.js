import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const queueSrc = readFileSync(
  fileURLToPath(new URL('../CharacterApprovalQueue.svelte', import.meta.url)),
  'utf8',
);

describe('CharacterApprovalQueue loading state', () => {
  it('loads approvals without waiting for campaignId', () => {
    expect(queueSrc).toContain('loadApprovals();');
    expect(queueSrc).not.toContain('if (campaignId) loadApprovals();');
  });

  it('always clears loading after the fetch attempt', () => {
    expect(queueSrc).toContain('finally');
    expect(queueSrc).toContain('loading = false;');
  });
});
