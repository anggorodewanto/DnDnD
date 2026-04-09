<script>
  import { advanceTurn, getCombatWorkspace } from './lib/api.js';
  import {
    pauseCampaign,
    resumeCampaign,
    nextCampaignStatus,
  } from './lib/campaignActions.js';

  let { campaignId } = $props();

  let campaignStatus = $state('active');
  let busy = $state(false);
  let error = $state(null);
  let info = $state(null);
  let activeEncounterId = $state(null);

  $effect(() => {
    if (campaignId) loadWorkspace();
  });

  async function loadWorkspace() {
    try {
      const data = await getCombatWorkspace(campaignId);
      const list = data.encounters || [];
      if (list.length > 0) activeEncounterId = list[0].id;
    } catch (e) {
      // non-fatal: End Turn will be disabled
      activeEncounterId = null;
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
