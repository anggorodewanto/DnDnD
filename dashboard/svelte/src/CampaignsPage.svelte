<script>
  import { createCampaign, listCampaigns } from './lib/api.js';

  let { activeCampaignId = '', oncreated = () => {} } = $props();

  let campaigns = $state([]);
  let loading = $state(true);
  let saving = $state(false);
  let error = $state('');
  let success = $state('');
  let name = $state('');
  let guildId = $state('');

  $effect(() => {
    loadCampaigns();
  });

  async function loadCampaigns() {
    loading = true;
    error = '';
    try {
      const data = await listCampaigns();
      campaigns = data.campaigns || [];
    } catch (e) {
      error = e.message || 'Failed to load campaigns.';
      campaigns = [];
    } finally {
      loading = false;
    }
  }

  async function submitCreate(event) {
    event.preventDefault();
    const trimmedName = name.trim();
    const trimmedGuildId = guildId.trim();
    if (!trimmedName || !trimmedGuildId) {
      error = 'Campaign name and guild ID are required.';
      return;
    }

    saving = true;
    error = '';
    success = '';
    try {
      const created = await createCampaign({
        name: trimmedName,
        guild_id: trimmedGuildId,
      });
      name = '';
      guildId = '';
      success = `Created ${created.name}.`;
      await loadCampaigns();
      await oncreated(created);
    } catch (e) {
      error = e.message || 'Failed to create campaign.';
    } finally {
      saving = false;
    }
  }
</script>

<section class="campaigns-page">
  <div class="campaign-grid">
    <form class="campaign-form" onsubmit={submitCreate}>
      <h2>New Campaign</h2>
      <label>
        <span>Name</span>
        <input type="text" bind:value={name} placeholder="Local Playtest Campaign" />
      </label>
      <label>
        <span>Guild ID</span>
        <input type="text" bind:value={guildId} placeholder="local-guild" />
      </label>
      <button type="submit" disabled={saving}>{saving ? 'Creating...' : 'Create Campaign'}</button>
    </form>

    <div class="campaign-list">
      <h2>Campaigns</h2>
      {#if loading}
        <p class="muted">Loading campaigns...</p>
      {:else if campaigns.length === 0}
        <p class="muted">No campaigns yet.</p>
      {:else}
        <ul>
          {#each campaigns as campaign}
            <li class:active={campaign.id === activeCampaignId}>
              <div>
                <strong>{campaign.name}</strong>
                <span>{campaign.guild_id}</span>
              </div>
              <div class="campaign-meta">
                <span>{campaign.status}</span>
                {#if campaign.id === activeCampaignId}
                  <span>Active</span>
                {/if}
              </div>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>

  {#if error}
    <p class="alert error" role="alert">{error}</p>
  {/if}
  {#if success}
    <p class="alert success" role="status">{success}</p>
  {/if}
</section>

<style>
  .campaigns-page {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .campaign-grid {
    display: grid;
    grid-template-columns: minmax(18rem, 24rem) minmax(0, 1fr);
    gap: 1rem;
    align-items: start;
  }

  .campaign-form,
  .campaign-list {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 6px;
    padding: 1rem;
  }

  h2 {
    margin: 0 0 1rem;
    color: #f7f7fb;
    font-size: 1.125rem;
  }

  label {
    display: grid;
    gap: 0.35rem;
    margin-bottom: 0.75rem;
    color: #cbd5e1;
    font-size: 0.875rem;
  }

  input {
    box-sizing: border-box;
    width: 100%;
    padding: 0.65rem 0.75rem;
    background: #0f1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }

  button {
    width: 100%;
    min-height: 2.5rem;
    padding: 0 1rem;
    background: #e94560;
    color: #ffffff;
    border: 1px solid #e94560;
    border-radius: 4px;
    cursor: pointer;
    font-weight: 700;
  }

  button:disabled {
    cursor: not-allowed;
    opacity: 0.65;
  }

  ul {
    display: grid;
    gap: 0.5rem;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  li {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.75rem;
    background: #0f1a2e;
    border: 1px solid #0f3460;
    border-radius: 6px;
  }

  li.active {
    border-color: #e94560;
  }

  strong,
  span {
    display: block;
  }

  strong {
    color: #ffffff;
  }

  span,
  .muted {
    color: #a0aec0;
  }

  .campaign-meta {
    min-width: 5rem;
    text-align: right;
    text-transform: capitalize;
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

  .alert.success {
    color: #bbf7d0;
    background: #14351f;
  }

  @media (max-width: 768px) {
    .campaign-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
