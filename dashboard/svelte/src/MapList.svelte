<script>
  import { listMaps, deleteMap } from './lib/api.js';
  import { createEventDispatcher } from 'svelte';

  let { campaignId, oncreate, onedit } = $props();

  let maps = $state([]);
  let loading = $state(true);
  let error = $state(null);

  const dispatch = createEventDispatcher();

  async function loadMaps() {
    loading = true;
    error = null;
    try {
      maps = await listMaps(campaignId);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function handleDelete(id, name) {
    if (!confirm(`Delete map "${name}"?`)) return;
    try {
      await deleteMap(id);
      await loadMaps();
    } catch (e) {
      error = e.message;
    }
  }

  $effect(() => {
    if (campaignId) {
      loadMaps();
    }
  });
</script>

<div class="map-list">
  <div class="actions">
    <button class="create-btn" onclick={oncreate}>+ New Map</button>
  </div>

  {#if loading}
    <p>Loading maps...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if maps.length === 0}
    <p>No maps yet. Create one to get started.</p>
  {:else}
    <div class="grid">
      {#each maps as map}
        <div class="map-card">
          <h3>{map.name}</h3>
          <p>{map.width} x {map.height} squares</p>
          <div class="card-actions">
            <button onclick={() => onedit && dispatch('edit', { id: map.id })}>Edit</button>
            <button class="delete-btn" onclick={() => handleDelete(map.id, map.name)}>Delete</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .map-list {
    max-width: 800px;
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
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 1rem;
  }

  .map-card {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    padding: 1rem;
  }

  .map-card h3 {
    color: #e94560;
    margin: 0 0 0.5rem;
  }

  .card-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.5rem;
  }

  .card-actions button {
    padding: 0.4rem 0.8rem;
    background: #0f3460;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }

  .delete-btn {
    background: #8b0000 !important;
    border-color: #8b0000 !important;
  }

  .error {
    color: #ff4444;
  }
</style>
