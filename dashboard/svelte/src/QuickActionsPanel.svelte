<script>
  import { advanceTurn, getCombatWorkspace } from './lib/api.js';
  import {
    pauseCampaign,
    resumeCampaign,
    nextCampaignStatus,
  } from './lib/campaignActions.js';

  let { campaignId, encounters: externalEncounters = null } = $props();

  let campaignStatus = $state('active');
  let busy = $state(false);
  let error = $state(null);
  let info = $state(null);
  let internalEncounters = $state([]);
  let selectedEncounterIdx = $state(0);

  // Use externally-provided encounters (from MobileShell) or load our own.
  let encounterList = $derived(externalEncounters || internalEncounters);
  let activeEncounterId = $derived(encounterList[selectedEncounterIdx]?.id || null);

  $effect(() => {
    if (campaignId && !externalEncounters) loadWorkspace();
  });

  async function loadWorkspace() {
    try {
      const data = await getCombatWorkspace(campaignId);
      internalEncounters = data.encounters || [];
    } catch (e) {
      internalEncounters = [];
    }
  }

  async function handleEndTurn() {
    if (!activeEncounterId) {
      error = 'No active encounter.';
      return;
    }
    busy = true;
    error = null;
    info = null;
    try {
      await advanceTurn(activeEncounterId);
      info = 'Turn advanced.';
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }

  async function handleToggle() {
    busy = true;
    error = null;
    info = null;
    try {
      const result = campaignStatus === 'active'
        ? await pauseCampaign(campaignId)
        : await resumeCampaign(campaignId);
      campaignStatus = result.status || nextCampaignStatus(campaignStatus);
      info = `Campaign ${campaignStatus}.`;
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }
</script>

<div class="quick-actions" data-testid="quick-actions">
  <h3>Quick Actions</h3>
  {#if encounterList.length > 1}
    <select class="encounter-select" bind:value={selectedEncounterIdx} data-testid="encounter-select">
      {#each encounterList as enc, i}
        <option value={i}>{enc.display_name || enc.name}</option>
      {/each}
    </select>
  {/if}
  <button onclick={handleEndTurn} disabled={busy || !activeEncounterId}>
    End Turn
  </button>
  <button onclick={handleToggle} disabled={busy}>
    {campaignStatus === 'active' ? 'Pause Campaign' : 'Resume Campaign'}
  </button>
  {#if error}<p class="error">{error}</p>{/if}
  {#if info}<p class="info">{info}</p>{/if}
</div>

<style>
  .quick-actions {
    padding: 1rem;
    background: #16213e;
    border-radius: 4px;
    border: 1px solid #0f3460;
  }
  .quick-actions h3 { margin: 0 0 0.5rem 0; color: #e94560; }
  .encounter-select {
    width: 100%;
    padding: 0.5rem;
    margin-bottom: 0.5rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }
  button {
    display: block;
    width: 100%;
    padding: 0.7rem;
    margin-bottom: 0.5rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-weight: bold;
  }
  button:disabled { opacity: 0.5; cursor: not-allowed; }
  .error { color: #ef4444; font-size: 0.85rem; }
  .info { color: #10b981; font-size: 0.85rem; }
</style>
