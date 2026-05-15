import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const shellSrc = readFileSync(resolve(__dirname, '../MobileShell.svelte'), 'utf-8');

describe('MobileShell approvals tab (F-23)', () => {
  it('imports CharacterApprovalQueue, not ActionResolver', () => {
    expect(shellSrc).toContain("import CharacterApprovalQueue from './CharacterApprovalQueue.svelte'");
    expect(shellSrc).not.toContain("import ActionResolver from './ActionResolver.svelte'");
  });

  it('renders CharacterApprovalQueue for the approvals tab', () => {
    // Extract the approvals tab block
    const approvalsIdx = shellSrc.indexOf("activeTab === 'approvals'");
    expect(approvalsIdx).toBeGreaterThan(-1);
    const afterApprovals = shellSrc.slice(approvalsIdx, approvalsIdx + 200);
    expect(afterApprovals).toContain('CharacterApprovalQueue');
    expect(afterApprovals).not.toContain('ActionResolver');
  });
});
