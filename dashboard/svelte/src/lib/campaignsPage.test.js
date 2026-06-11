import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const pageSrc = readFileSync(
  fileURLToPath(new URL('../CampaignsPage.svelte', import.meta.url)),
  'utf8',
);

describe('CampaignsPage', () => {
  it('loads and creates campaigns through the shared API client', () => {
    expect(pageSrc).toContain("import { createCampaign, listCampaigns, setActiveCampaign, listGuilds } from './lib/api.js'");
    expect(pageSrc).toContain('await listCampaigns()');
    expect(pageSrc).toContain('await createCampaign({');
  });

  it('refreshes parent campaign state after creating a campaign', () => {
    expect(pageSrc).toContain('oncreated = () => {}');
    expect(pageSrc).toContain('await oncreated(created)');
  });

  it('lets the DM set the active campaign and refreshes parent state', () => {
    expect(pageSrc).toContain('onactivechange = () => {}');
    expect(pageSrc).toContain('await setActiveCampaign(campaign.id)');
    expect(pageSrc).toContain('await onactivechange()');
    expect(pageSrc).toContain('Set active');
  });

  it('offers a guild dropdown sourced from the bot guild list', () => {
    expect(pageSrc).toContain('await listGuilds()');
    expect(pageSrc).toContain('<select bind:value={guildId}>');
    expect(pageSrc).toContain('{#each guilds as guild}');
  });
});
