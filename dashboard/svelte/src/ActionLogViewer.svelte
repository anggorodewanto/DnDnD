<script>
  import { listActionLog } from './lib/api.js';
  import { diffStates } from './lib/diff.js';

  let { encounterId } = $props();

  let entries = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let expanded = $state({}); // id -> boolean

  // Filters
  let selectedTypes = $state([]);
  let actorId = $state('');
  let targetId = $state('');
  let round = $state('');
  let turnId = $state('');
  let sort = $state('desc');

  const ACTION_TYPES = [
    'move', 'attack', 'cast', 'damage', 'heal',
    'condition_add', 'condition_remove', 'death_save', 'dm_override',
    'resolve_pending_action',
  ];

  $effect(() => {
    if (encounterId) {
      loadEntries();
    }
  });

  async function loadEntries() {
    loading = true;
    try {
      const data = await listActionLog(encounterId, buildFilters());
      entries = data || [];
      error = null;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function buildFilters() {
    const f = { sort };
    if (selectedTypes.length > 0) f.actionTypes = selectedTypes;
    if (actorId) f.actorId = actorId;
    if (targetId) f.targetId = targetId;
    if (round) f.round = Number(round);
    if (turnId) f.turnId = turnId;
    return f;
  }

  function toggleType(t) {
    if (selectedTypes.includes(t)) {
      selectedTypes = selectedTypes.filter(x => x !== t);
      return;
    }
    selectedTypes = [...selectedTypes, t];
  }

  function toggleExpanded(id) {
    expanded = { ...expanded, [id]: !expanded[id] };
  }

  function applyFilters() {
    loadEntries();
  }

  function clearFilters() {
    selectedTypes = [];
    actorId = '';
    targetId = '';
    round = '';
    turnId = '';
    sort = 'desc';
    loadEntries();
  }

  function formatValue(v) {
    if (v === undefined) return '∅';
    if (v === null) return 'null';
    if (typeof v === 'object') return JSON.stringify(v);
    return String(v);
  }

  function entryDiff(entry) {
    return diffStates(entry.before_state, entry.after_state);
  }
</script>

<div class="action-log-viewer" data-testid="action-log-viewer">
  <h3>Action Log</h3>

  <div class="filters">
    <div class="filter-row">
      <label>Types:</label>
      <div class="type-buttons">
        {#each ACTION_TYPES as t}
          <button
            type="button"
            class="type-btn"
            class:selected={selectedTypes.includes(t)}
            onclick={() => toggleType(t)}
            data-testid="type-{t}"
          >{t}</button>
        {/each}
      </div>
    </div>
    <div class="filter-row">
      <label>Actor ID: <input type="text" bind:value={actorId} /></label>
      <label>Target ID: <input type="text" bind:value={targetId} /></label>
    </div>
    <div class="filter-row">
      <label>Round: <input type="number" min="1" bind:value={round} /></label>
      <label>Turn ID: <input type="text" bind:value={turnId} /></label>
      <label>Sort:
        <select bind:value={sort}>
          <option value="desc">Newest first</option>
          <option value="asc">Oldest first</option>
        </select>
      </label>
    </div>
    <div class="filter-row">
      <button type="button" onclick={applyFilters} data-testid="apply-filters">Apply</button>
      <button type="button" onclick={clearFilters} data-testid="clear-filters">Clear</button>
    </div>
  </div>

  {#if loading}
    <p class="status-msg">Loading...</p>
  {:else if error}
    <p class="error-msg">{error}</p>
  {:else if entries.length === 0}
    <p class="status-msg">No action log entries.</p>
  {:else}
    <ul class="entry-list">
      {#each entries as entry}
        <li
          class="entry"
          class:override={entry.is_override}
          data-testid="entry-{entry.id}"
        >
          <button
            class="entry-header"
            type="button"
            onclick={() => toggleExpanded(entry.id)}
            data-testid="toggle-{entry.id}"
          >
            <span class="entry-type">{entry.action_type}</span>
            {#if entry.is_override}
              <span class="override-badge">OVERRIDE</span>
            {/if}
            <span class="entry-actor">{entry.actor_display_name || '(unknown)'}</span>
            {#if entry.target_display_name}
              <span class="arrow">→</span>
              <span class="entry-target">{entry.target_display_name}</span>
            {/if}
            <span class="entry-round">R{entry.round_number}</span>
            <span class="entry-time">{entry.created_at}</span>
          </button>
          {#if expanded[entry.id]}
            <div class="entry-detail" data-testid="detail-{entry.id}">
              {#if entry.description}
                <p class="desc">{entry.description}</p>
              {/if}
              <div class="diff">
                <h4>State changes</h4>
                {#each entryDiff(entry) as d}
                  <div class="diff-row" data-testid="diff-{entry.id}-{d.field}">
                    <span class="diff-field">{d.field}:</span>
                    <span class="diff-before">{formatValue(d.before)}</span>
                    <span class="arrow">→</span>
                    <span class="diff-after">{formatValue(d.after)}</span>
                  </div>
                {:else}
                  <p class="no-diff">No field changes.</p>
                {/each}
              </div>
            </div>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .action-log-viewer {
    padding: 0.75rem;
    background: #16213e;
    border-radius: 4px;
    border: 1px solid #0f3460;
    color: #e0e0e0;
  }
  .action-log-viewer h3 {
    margin: 0 0 0.5rem 0;
    color: #e94560;
    font-size: 0.95rem;
  }
  .filters {
    margin-bottom: 0.5rem;
    padding: 0.4rem;
    background: #1a1a2e;
    border-radius: 4px;
  }
  .filter-row {
    display: flex;
    gap: 0.4rem;
    align-items: center;
    flex-wrap: wrap;
    margin-bottom: 0.3rem;
    font-size: 0.75rem;
  }
  .filter-row label {
    display: inline-flex;
    gap: 0.25rem;
    align-items: center;
  }
  .filter-row input,
  .filter-row select {
    padding: 0.15rem 0.35rem;
    background: #0f3460;
    color: #e0e0e0;
    border: 1px solid #3b82f6;
    border-radius: 3px;
    font-size: 0.75rem;
  }
  .filter-row button {
    padding: 0.2rem 0.6rem;
    background: #3b82f6;
    color: #fff;
    border: none;
    border-radius: 3px;
    cursor: pointer;
    font-size: 0.75rem;
  }
  .type-buttons {
    display: flex;
    gap: 0.25rem;
    flex-wrap: wrap;
  }
  .type-btn {
    padding: 0.15rem 0.4rem;
    background: #0f3460;
    color: #a0aec0;
    border: 1px solid #0f3460;
    border-radius: 3px;
    cursor: pointer;
    font-size: 0.7rem;
  }
  .type-btn.selected {
    background: #3b82f6;
    color: #fff;
  }
  .entry-list {
    list-style: none;
    padding: 0;
    margin: 0;
    max-height: 300px;
    overflow-y: auto;
  }
  .entry {
    border: 1px solid #0f3460;
    border-radius: 3px;
    margin-bottom: 0.25rem;
    background: #1a1a2e;
  }
  .entry.override {
    border-left: 4px solid #f59e0b;
    background: rgba(245, 158, 11, 0.08);
  }
  .entry-header {
    width: 100%;
    display: flex;
    gap: 0.4rem;
    align-items: center;
    flex-wrap: wrap;
    padding: 0.35rem 0.5rem;
    background: transparent;
    color: #e0e0e0;
    border: none;
    cursor: pointer;
    font-size: 0.75rem;
    text-align: left;
  }
  .entry-type {
    color: #3b82f6;
    font-weight: bold;
  }
  .override-badge {
    background: #f59e0b;
    color: #1a1a2e;
    padding: 0.05rem 0.3rem;
    border-radius: 3px;
    font-size: 0.6rem;
    font-weight: bold;
  }
  .entry-actor {
    color: #22c55e;
  }
  .entry-target {
    color: #e94560;
  }
  .entry-round {
    color: #a0aec0;
    font-size: 0.7rem;
  }
  .entry-time {
    color: #6b7280;
    margin-left: auto;
    font-size: 0.7rem;
  }
  .entry-detail {
    padding: 0.4rem 0.6rem;
    border-top: 1px solid #0f3460;
    font-size: 0.75rem;
  }
  .desc {
    margin: 0 0 0.4rem 0;
    color: #e0e0e0;
  }
  .diff h4 {
    margin: 0 0 0.25rem 0;
    font-size: 0.7rem;
    color: #a0aec0;
    text-transform: uppercase;
  }
  .diff-row {
    display: flex;
    gap: 0.35rem;
    align-items: baseline;
    padding: 0.1rem 0;
  }
  .diff-field {
    color: #3b82f6;
    font-weight: bold;
  }
  .diff-before {
    color: #ef4444;
    text-decoration: line-through;
  }
  .diff-after {
    color: #22c55e;
  }
  .arrow {
    color: #a0aec0;
  }
  .no-diff {
    margin: 0;
    color: #6b7280;
    font-style: italic;
  }
  .status-msg {
    color: #a0aec0;
    font-style: italic;
    font-size: 0.85rem;
  }
  .error-msg {
    color: #ef4444;
    font-size: 0.85rem;
  }
</style>
