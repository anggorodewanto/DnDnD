<script>
  // F-8 / Phase 111 — per-campaign Open5e source toggle.
  //
  // The DM checks one or more Open5e document slugs (Tome of Beasts, Deep
  // Magic, ...) for the current campaign; the backend persists the list to
  // `campaigns.settings.open5e_sources` JSONB. Without an enabled slug,
  // every open5e:* cached row is hidden by the per-campaign filter
  // (`internal/open5e/filter.go` + `refdata_adapter.go`), so flipping
  // checkboxes here directly controls what shows up in spell lists / stat
  // block library / character builder.
  //
  // The catalog is fetched from `/api/open5e/sources` — the backend stays
  // the single source of truth for slug→title mapping so we don't
  // duplicate the list in two places.
  import {
    listOpen5eSources,
    getCampaignOpen5eSources,
    updateCampaignOpen5eSources,
  } from './lib/api.js';

  let { campaignId } = $props();

  let catalog = $state([]);
  let enabled = $state(new Set());
  let loading = $state(true);
  let saving = $state(false);
  let error = $state('');
  let savedAt = $state(null);

  $effect(() => {
    load();
  });

  async function load() {
    error = '';
    loading = true;
    try {
      const [cat, current] = await Promise.all([
        listOpen5eSources(),
        campaignId ? getCampaignOpen5eSources(campaignId) : Promise.resolve({ enabled: [] }),
      ]);
      catalog = cat.sources || [];
      enabled = new Set(current.enabled || []);
    } catch (e) {
      error = e.message || 'Failed to load Open5e sources.';
    } finally {
      loading = false;
    }
  }

  async function toggle(slug) {
    if (!campaignId) {
      error = 'No active campaign — cannot toggle sources.';
      return;
    }
    // Optimistic local flip; revert on backend failure.
    const next = new Set(enabled);
    if (next.has(slug)) {
      next.delete(slug);
    } else {
      next.add(slug);
    }
    enabled = next;
    await persist();
  }

  async function persist() {
    saving = true;
    error = '';
    try {
      const list = Array.from(enabled);
      const resp = await updateCampaignOpen5eSources(campaignId, list);
      // Trust the backend's canonical, validated list (de-duped + order
      // matches input). This keeps the UI in lock-step with the JSONB.
      enabled = new Set(resp.enabled || []);
      savedAt = new Date();
    } catch (e) {
      error = e.message || 'Failed to update sources.';
      // Roll back to the persisted state.
      try {
        const current = await getCampaignOpen5eSources(campaignId);
        enabled = new Set(current.enabled || []);
      } catch {
        /* leave optimistic state — user can retry */
      }
    } finally {
      saving = false;
    }
  }

  function disableAll() {
    enabled = new Set();
    persist();
  }
</script>

<section class="open5e-sources-panel">
  <header>
    <h2>Open5e Sources</h2>
    <p class="hint">
      Enable Open5e third-party books for this campaign. Disabled books are
      hidden from spell pickers, the stat block library, and character builder.
    </p>
  </header>

  {#if !campaignId}
    <p class="empty">Active campaign required.</p>
  {:else if loading}
    <p>Loading Open5e sources...</p>
  {:else}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}

    <ul class="source-list">
      {#each catalog as src (src.slug)}
        <li>
          <label>
            <input
              type="checkbox"
              checked={enabled.has(src.slug)}
              disabled={saving}
              onchange={() => toggle(src.slug)}
            />
            <span class="title">{src.title}</span>
            {#if src.publisher}<span class="publisher">— {src.publisher}</span>{/if}
            {#if src.description}<span class="description">{src.description}</span>{/if}
            <span class="slug">{src.slug}</span>
          </label>
        </li>
      {/each}
    </ul>

    <footer class="actions">
      <button type="button" onclick={disableAll} disabled={saving || enabled.size === 0}>
        Disable all
      </button>
      {#if saving}
        <span class="status">Saving...</span>
      {:else if savedAt}
        <span class="status saved">Saved {savedAt.toLocaleTimeString()}</span>
      {/if}
    </footer>
  {/if}
</section>

<style>
  .open5e-sources-panel {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 1rem;
    max-width: 720px;
  }
  header h2 {
    margin: 0 0 0.25rem;
    color: #e94560;
  }
  .hint {
    color: #b0b0c0;
    margin: 0 0 1rem;
    font-size: 0.9rem;
  }
  .empty {
    color: #b0b0c0;
    font-style: italic;
  }
  .source-list {
    list-style: none;
    padding: 0;
    margin: 0 0 1rem;
    display: grid;
    gap: 0.5rem;
  }
  .source-list li {
    background: #0f3460;
    padding: 0.5rem 0.75rem;
    border-radius: 4px;
  }
  .source-list label {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 0.25rem 0.75rem;
    align-items: baseline;
    cursor: pointer;
  }
  .title {
    color: #e0e0e0;
    font-weight: 600;
  }
  .publisher {
    color: #b0b0c0;
    grid-column: 2;
    font-size: 0.85rem;
  }
  .description {
    color: #b0b0c0;
    grid-column: 2;
    font-size: 0.85rem;
  }
  .slug {
    grid-column: 2;
    font-family: ui-monospace, monospace;
    font-size: 0.75rem;
    color: #888;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 1rem;
  }
  .actions button {
    background: #0f3460;
    color: #e0e0e0;
    border: 1px solid #16213e;
    border-radius: 4px;
    padding: 0.4rem 0.8rem;
    cursor: pointer;
  }
  .actions button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .status {
    color: #b0b0c0;
    font-size: 0.85rem;
  }
  .status.saved {
    color: #6ce26c;
  }
  .error {
    color: #ff6b6b;
    margin: 0 0 1rem;
  }
</style>
