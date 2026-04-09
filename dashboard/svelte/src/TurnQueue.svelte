<script>
  import { getTurnQueue, advanceTurn } from './lib/api.js';

  let { encounterId, activeTurnCombatantId, onTurnAdvanced, readOnly = false } = $props();

  let entries = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let advancing = $state(false);

  $effect(() => {
    if (encounterId) {
      loadQueue();
    }
  });

  async function loadQueue() {
    try {
      const data = await getTurnQueue(encounterId);
      entries = data.entries || [];
      error = null;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function handleEndTurn() {
    if (advancing) return;
    advancing = true;
    try {
      await advanceTurn(encounterId);
      await loadQueue();
      if (onTurnAdvanced) onTurnAdvanced();
      error = null;
    } catch (e) {
      error = e.message;
    } finally {
      advancing = false;
    }
  }
</script>

<div class="turn-queue" data-testid="turn-queue">
  <h3>Turn Queue</h3>

  {#if loading}
    <p class="status-msg">Loading...</p>
  {:else if error}
    <p class="error-msg">{error}</p>
  {:else if entries.length === 0}
    <p class="status-msg">No combatants in initiative.</p>
  {:else}
    <ul class="queue-list" data-testid="queue-list">
      {#each entries as entry}
        <li
          class="queue-entry"
          class:active={entry.combatant_id === activeTurnCombatantId}
          class:npc={entry.is_npc}
          data-testid="queue-entry-{entry.combatant_id || entry.display_name}"
        >
          <span class="entry-initiative">{entry.initiative}</span>
          <span class="entry-name">{entry.display_name}</span>
          {#if entry.type !== 'combatant'}
            <span class="entry-type-badge">{entry.type}</span>
          {/if}
          {#if entry.combatant_id === activeTurnCombatantId}
            <span class="active-indicator">CURRENT</span>
          {/if}
        </li>
      {/each}
    </ul>

    {#if !readOnly}
      <button
        class="end-turn-btn"
        onclick={handleEndTurn}
        disabled={advancing}
        data-testid="end-turn-btn"
      >
        {advancing ? 'Advancing...' : 'End Turn'}
      </button>
    {/if}
  {/if}
</div>

<style>
  .turn-queue {
    padding: 0.75rem;
    background: #16213e;
    border-radius: 4px;
    border: 1px solid #0f3460;
  }

  .turn-queue h3 {
    margin: 0 0 0.5rem 0;
    color: #e94560;
    font-size: 0.95rem;
  }

  .queue-list {
    list-style: none;
    padding: 0;
    margin: 0 0 0.5rem 0;
  }

  .queue-entry {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.35rem 0.5rem;
    border-radius: 3px;
    font-size: 0.85rem;
    color: #e0e0e0;
  }

  .queue-entry.active {
    background: rgba(251, 191, 36, 0.2);
    border: 1px solid #fbbf24;
  }

  .queue-entry.npc {
    color: #f87171;
  }

  .entry-initiative {
    min-width: 24px;
    text-align: right;
    font-weight: bold;
    color: #a0aec0;
  }

  .entry-name {
    flex: 1;
  }

  .entry-type-badge {
    font-size: 0.7rem;
    padding: 0.1rem 0.3rem;
    background: #0f3460;
    border-radius: 3px;
    text-transform: uppercase;
  }

  .active-indicator {
    font-size: 0.7rem;
    padding: 0.1rem 0.4rem;
    background: #fbbf24;
    color: #1a1a2e;
    border-radius: 3px;
    font-weight: bold;
  }

  .end-turn-btn {
    width: 100%;
    padding: 0.5rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.9rem;
    font-weight: bold;
  }

  .end-turn-btn:hover:not(:disabled) {
    background: #c53050;
  }

  .end-turn-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
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
