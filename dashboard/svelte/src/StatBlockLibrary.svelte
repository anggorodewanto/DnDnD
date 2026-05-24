<script>
  import { listStatBlocks } from './lib/statblockLibrary.js';

  let { campaignId } = $props();

  let search = $state('');
  let typeFilter = $state('');
  let sizeFilter = $state('');
  let crMinInput = $state('');
  let crMaxInput = $state('');
  let sourceFilter = $state('');

  let entries = $state([]);
  let loading = $state(false);
  let error = $state(null);
  let selected = $state(null);

  const CREATURE_TYPES = [
    'aberration', 'beast', 'celestial', 'construct', 'dragon', 'elemental',
    'fey', 'fiend', 'giant', 'humanoid', 'monstrosity', 'ooze', 'plant', 'undead',
  ];
  const SIZES = ['Tiny', 'Small', 'Medium', 'Large', 'Huge', 'Gargantuan'];

  function buildFilters() {
    const filters = { campaignId };
    if (search.trim()) filters.search = search.trim();
    if (typeFilter) filters.types = [typeFilter];
    if (sizeFilter) filters.sizes = [sizeFilter];
    if (crMinInput !== '') {
      const n = parseFloat(crMinInput);
      if (!Number.isNaN(n)) filters.crMin = n;
    }
    if (crMaxInput !== '') {
      const n = parseFloat(crMaxInput);
      if (!Number.isNaN(n)) filters.crMax = n;
    }
    if (sourceFilter) filters.source = sourceFilter;
    return filters;
  }

  async function load() {
    loading = true;
    error = null;
    try {
      entries = await listStatBlocks(buildFilters());
    } catch (e) {
      error = e.message;
      entries = [];
    } finally {
      loading = false;
    }
  }

  function handleSearchSubmit(e) {
    e.preventDefault();
    load();
  }

  function handleReset() {
    search = '';
    typeFilter = '';
    sizeFilter = '';
    crMinInput = '';
    crMaxInput = '';
    sourceFilter = '';
    load();
  }

  $effect(() => {
    if (campaignId !== undefined) {
      load();
    }
  });

  function selectEntry(entry) {
    selected = entry;
  }

  function closeDetail() {
    selected = null;
  }
</script>

<div class="statblock-library" data-testid="stat-block-library">
  <h2>Stat Block Library</h2>

  <form class="filters" onsubmit={handleSearchSubmit}>
    <input
      type="search"
      placeholder="Search by name"
      bind:value={search}
      data-testid="statblock-search"
    />
    <select bind:value={typeFilter} data-testid="statblock-type">
      <option value="">Any type</option>
      {#each CREATURE_TYPES as t}
        <option value={t}>{t}</option>
      {/each}
    </select>
    <select bind:value={sizeFilter} data-testid="statblock-size">
      <option value="">Any size</option>
      {#each SIZES as s}
        <option value={s}>{s}</option>
      {/each}
    </select>
    <input
      type="number"
      step="0.25"
      placeholder="CR min"
      bind:value={crMinInput}
      data-testid="statblock-cr-min"
    />
    <input
      type="number"
      step="0.25"
      placeholder="CR max"
      bind:value={crMaxInput}
      data-testid="statblock-cr-max"
    />
    <select bind:value={sourceFilter} data-testid="statblock-source">
      <option value="">SRD + Homebrew</option>
      <option value="srd">SRD only</option>
      <option value="homebrew">Homebrew only</option>
    </select>
    <button type="submit">Apply</button>
    <button type="button" onclick={handleReset}>Reset</button>
  </form>

  <div class="body">
    <div class="list-col">
      {#if loading}
        <p>Loading stat blocks...</p>
      {:else if error}
        <p class="error">{error}</p>
      {:else if entries.length === 0}
        <p>No stat blocks match these filters.</p>
      {:else}
        <ul class="entry-list" data-testid="statblock-results">
          {#each entries as entry (entry.id)}
            <li>
              <button class="entry-row" onclick={() => selectEntry(entry)}>
                <strong>{entry.name}</strong>
                <span class="meta">
                  {entry.size || ''} {entry.type || ''}
                  {#if entry.cr !== undefined && entry.cr !== ''} · CR {entry.cr}{/if}
                  {#if entry.homebrew} · Homebrew{/if}
                </span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    <div class="detail-col">
      {#if selected}
        <div class="detail open" data-testid="statblock-detail">
          <header>
            <h3>{selected.name}</h3>
            <button class="close" onclick={closeDetail}>x</button>
          </header>
          <p class="meta">
            {selected.size || ''} {selected.type || ''}
            {#if selected.cr !== undefined && selected.cr !== ''} · CR {selected.cr}{/if}
          </p>
          <pre>{JSON.stringify(selected, null, 2)}</pre>
        </div>
      {:else}
        <p class="empty detail-placeholder">Select a stat block to view its details.</p>
      {/if}
    </div>
  </div>

  {#if selected}
    <button
      class="detail-backdrop"
      aria-label="Close detail"
      onclick={closeDetail}
    ></button>
  {/if}
</div>

<style>
  .statblock-library {
    max-width: 1100px;
  }
  h2 {
    color: #e94560;
  }
  .filters {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin-bottom: 1rem;
  }
  .filters input,
  .filters select {
    background: #0f1626;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 0.4rem 0.5rem;
  }
  .filters button {
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    padding: 0.4rem 0.9rem;
    cursor: pointer;
  }
  .filters button[type='button'] {
    background: #0f3460;
  }
  .body {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(20rem, 26rem);
    gap: 1rem;
    align-items: start;
  }
  .detail-col {
    position: sticky;
    top: 1rem;
    max-height: calc(100vh - 2rem);
    overflow-y: auto;
  }
  .entry-list {
    list-style: none;
    padding: 0;
    margin: 0;
  }
  .entry-list li {
    margin-bottom: 0.25rem;
  }
  .entry-row {
    display: flex;
    width: 100%;
    text-align: left;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 4px;
    color: #e0e0e0;
    padding: 0.55rem 0.75rem;
    cursor: pointer;
    gap: 1rem;
    align-items: baseline;
  }
  .entry-row:hover {
    background: #0f3460;
  }
  .meta {
    color: #b0b0c0;
    font-size: 0.85rem;
  }
  .empty {
    color: #7a7a8f;
  }
  .detail {
    background: #16213e;
    border-radius: 6px;
    border: 1px solid #0f3460;
    padding: 1rem;
  }
  .detail header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .detail h3 {
    margin: 0;
    color: #e94560;
  }
  .detail .close {
    background: transparent;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 0.25rem 0.5rem;
    cursor: pointer;
  }
  .detail pre {
    background: #0f1626;
    color: #e0e0e0;
    padding: 0.5rem;
    border-radius: 4px;
    overflow-x: auto;
    font-size: 0.8rem;
  }
  .error {
    color: #ff6b6b;
  }
  .detail-backdrop {
    display: none;
  }

  @media (max-width: 768px) {
    .body {
      grid-template-columns: 1fr;
    }
    /* Drop sticky on mobile: position:sticky creates a stacking context that
       traps the fixed .detail (z-index 30) below the backdrop (z-index 20),
       darkening the viewer. Static keeps .detail in the root stacking order. */
    .detail-col {
      position: static;
      max-height: none;
      overflow: visible;
    }
    .detail-placeholder {
      display: none;
    }
    .detail {
      position: fixed;
      top: 0;
      right: 0;
      z-index: 30;
      width: min(20rem, 90vw);
      height: 100vh;
      transform: translateX(100%);
      transition: transform 0.2s ease;
      overflow-y: auto;
    }
    .detail.open {
      transform: translateX(0);
    }
    .detail-backdrop {
      display: block;
      position: fixed;
      inset: 0;
      z-index: 20;
      background: rgba(0, 0, 0, 0.5);
      border: none;
      padding: 0;
    }
  }
</style>
