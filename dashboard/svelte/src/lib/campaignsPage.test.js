import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const pageSrc = readFileSync(
  fileURLToPath(new URL('../CampaignsPage.svelte', import.meta.url)),
  'utf8',
);

describe('CampaignsPage', () => {
  it('loads and creates campaigns through the shared API client', () => {
    expect(pageSrc).toContain("import { createCampaign, listCampaigns } from './lib/api.js'");
    expect(pageSrc).toContain('await listCampaigns()');
    expect(pageSrc).toContain('await createCampaign({');
  });

  it('refreshes parent campaign state after creating a campaign', () => {
    expect(pageSrc).toContain('oncreated = () => {}');
    expect(pageSrc).toContain('await oncreated(created)');
  });
});
