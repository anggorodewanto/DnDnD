<script>
  import {
    getHomeData,
    toggleCampaignStatus,
    pauseButtonLabel,
  } from './lib/home.js';

  let { campaignId = '' } = $props();

  let loading = $state(true);
  let error = $state('');
  let data = $state({
    campaign_id: '',
    campaign_status: '',
    dm_queue_count: 0,
    pending_approvals: 0,
    active_encounters: [],
    saved_encounters: [],
  });
  let pauseBusy = $state(false);
  let pauseError = $state('');

  $effect(() => {
    refresh();
  });

  async function refresh() {
    loading = true;
    error = '';
    try {
      const payload = await getHomeData();
      data = {
        campaign_id: payload.campaign_id || '',
        campaign_status: payload.campaign_status || '',
        dm_queue_count: payload.dm_queue_count || 0,
        pending_approvals: payload.pending_approvals || 0,
        active_encounters: Array.isArray(payload.active_encounters) ? payload.active_encounters : [],
        saved_encounters: Array.isArray(payload.saved_encounters) ? payload.saved_encounters : [],
      };
    } catch (e) {
      error = e.message || 'Failed to load Campaign Home.';
    } finally {
      loading = false;
    }
  }

  async function onTogglePause() {
    const id = data.campaign_id || campaignId;
    if (!id) return;
    pauseBusy = true;
    pauseError = '';
    try {
      await toggleCampaignStatus(id, data.campaign_status);
      data.campaign_status = data.campaign_status === 'paused' ? 'active' : 'paused';
    } catch (e) {
      pauseError = e.message || 'Pause/Resume failed.';
    } finally {
      pauseBusy = false;
    }
  }

  function hasCampaign() {
    return Boolean(data.campaign_id);
  }
</script>

<section class="home-panel">
  {#if loading}
    <p class="muted">Loading Campaign Home...</p>
  {:else if error}
    <p class="alert error" role="alert">{error}</p>
  {:else if !hasCampaign()}
    <div class="empty">
      <h2>No active campaign</h2>
      <p>Run <code>/setup</code> in your Discord server to create a campaign and its channel structure, then come back here.</p>
      <p class="muted">Next steps: build a map, then create an encounter to start combat.</p>
    </div>
  {:else}
    <div class="cards">
      <article class="card" id="dm-queue">
        <h3>Pending dm-queue Items</h3>
        <p class="count">{data.dm_queue_count}</p>
      </article>
      <article class="card" id="pending-approvals">
        <h3>Pending Character Approvals</h3>
        <p class="count">{data.pending_approvals}</p>
      </article>
      <article class="card" id="active-encounters">
        <h3>Active Encounters</h3>
        {#if data.active_encounters.length > 0}
          <ul>
            {#each data.active_encounters as name}
              <li>{name}</li>
            {/each}
          </ul>
        {:else}
          <p class="muted">No active encounters</p>
        {/if}
      </article>
      <article class="card" id="saved-encounters">
        <h3>Saved Encounters</h3>
        {#if data.saved_encounters.length > 0}
          <ul>
            {#each data.saved_encounters as name}
              <li>{name}</li>
            {/each}
          </ul>
        {:else}
          <p class="muted">No saved encounters</p>
        {/if}
      </article>
    </div>

    <div class="quick-actions">
      <a class="action" href="#encounter-new">New Encounter</a>
      <a class="action" href="#narrate">Narrate</a>
      <button
        type="button"
        class="action"
        onclick={onTogglePause}
        disabled={pauseBusy || !data.campaign_id}
        data-campaign-id={data.campaign_id}
        data-campaign-status={data.campaign_status}
      >
        {pauseButtonLabel(data.campaign_status)}
      </button>
    </div>

    {#if pauseError}
      <p class="alert error" role="alert">{pauseError}</p>
    {/if}
  {/if}
</section>

<style>
  .home-panel {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    max-width: 1100px;
  }

  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
    gap: 1rem;
  }

  .card {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    padding: 1.25rem;
  }

  .card h3 {
    margin: 0 0 0.5rem;
    color: #e94560;
    font-size: 1rem;
  }

  .card .count {
    margin: 0;
    color: #f7f7fb;
    font-size: 2rem;
    font-weight: 700;
  }

  .card ul {
    margin: 0;
    padding-left: 1.25rem;
    color: #cbd5e1;
  }

  .card li {
    padding: 0.15rem 0;
  }

  .quick-actions {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .action {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 2.5rem;
    padding: 0 1.25rem;
    background: #e94560;
    color: #ffffff;
    border: 1px solid #e94560;
    border-radius: 6px;
    font-weight: 700;
    text-decoration: none;
    cursor: pointer;
  }

  .action:hover {
    background: #c73852;
    border-color: #c73852;
  }

  .action:disabled {
    cursor: not-allowed;
    opacity: 0.65;
  }

  .empty {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    padding: 1.5rem;
    text-align: center;
    color: #cbd5e1;
  }

  .empty h2 {
    margin: 0 0 0.5rem;
    color: #f7f7fb;
    font-size: 1.25rem;
  }

  .muted {
    color: #a0aec0;
  }

  .alert {
    margin: 0;
    padding: 0.75rem 1rem;
    border-radius: 4px;
  }

  .alert.error {
    color: #fecaca;
    background: #451a24;
  }
</style>
