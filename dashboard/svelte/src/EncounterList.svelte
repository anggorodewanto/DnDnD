<script>
  import { listEncounters, deleteEncounter, duplicateEncounter } from './lib/api.js';

  let { campaignId, oncreate, onedit } = $props();

  let encounters = $state([]);
  let loading = $state(true);
  let error = $state(null);

  async function loadEncounters() {
    loading = true;
    error = null;
    try {
      encounters = await listEncounters(campaignId);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function handleDelete(id, name) {
    if (!confirm(`Delete encounter "${name}"?`)) return;
    try {
      await deleteEncounter(id);
      await loadEncounters();
    } catch (e) {
      error = e.message;
    }
  }

  async function handleDuplicate(id) {
    try {
      await duplicateEncounter(id);
      await loadEncounters();
    } catch (e) {
      error = e.message;
    }
  }

  $effect(() => {
    if (campaignId) {
      loadEncounters();
    }
  });
</script>

<div class="encounter-list">
  <div class="actions">
    <button class="create-btn" onclick={oncreate}>+ New Encounter</button>
  </div>

  {#if loading}
    <p>Loading encounters...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if encounters.length === 0}
    <p>No saved encounters yet. Create one to get started.</p>
  {:else}
    <div class="grid">
      {#each encounters as enc}
        <div class="encounter-card">
          <h3>{enc.name}</h3>
          {#if enc.display_name}
            <p class="display-name">Display: {enc.display_name}</p>
          {/if}
          <p class="meta">{enc.creature_count} creature type{enc.creature_count !== 1 ? 's' : ''}</p>
          <p class="meta date">{new Date(enc.created_at).toLocaleDateString()}</p>
          <div class="card-actions">
            <button onclick={() => onedit && onedit(enc.id)}>Edit</button>
            <button onclick={() => handleDuplicate(enc.id)}>Duplicate</button>
            <button class="delete-btn" onclick={() => handleDelete(enc.id, enc.name)}>Delete</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .encounter-list {
    max-width: 900px;
  }

  .actions {
    margin-bottom: 1rem;
  }

  .create-btn {
    padding: 0.75rem 1.5rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    font-size: 1rem;
  }

  .create-btn:hover {
    background: #c73852;
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
    gap: 1rem;
  }

  .encounter-card {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    padding: 1rem;
  }

  .encounter-card h3 {
    color: #e94560;
    margin: 0 0 0.5rem;
  }

  .display-name {
    color: #888;
    font-size: 0.85rem;
    font-style: italic;
    margin: 0 0 0.25rem;
  }

  .meta {
    color: #aaa;
    font-size: 0.85rem;
    margin: 0 0 0.25rem;
  }

  .date {
    color: #666;
  }

  .card-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.75rem;
  }

  .card-actions button {
    padding: 0.4rem 0.8rem;
    background: #0f3460;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
  }

  .card-actions button:hover {
    background: #1a3a6e;
  }

  .delete-btn {
    background: #8b0000 !important;
    border-color: #8b0000 !important;
  }

  .delete-btn:hover {
    background: #a00000 !important;
  }

  .error {
    color: #ff4444;
  }
</style>
