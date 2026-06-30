<script>
  // In-combat surface for unresolved monster (NPC) AoE saving throws. Polls
  // GET /api/combat/{id}/pending-saves (works regardless of whose turn it is),
  // lets the DM roll each one via the resolve endpoint, and keeps a short log
  // of the rolled outcomes so the result survives the list refreshing the
  // resolved save off. onResolved lets the parent reload the workspace so the
  // applied damage shows on the affected token's HP.
  import { getPendingSaves, resolveMonsterSave } from './lib/api.js';
  import { formatMonsterSaveResult } from './lib/combat.js';

  let { encounterId, onResolved } = $props();

  let saves = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let resolving = $state(null); // save ID currently being resolved
  let results = $state([]); // [{ text, ok }] newest-first rolled outcomes

  $effect(() => {
    if (!encounterId) return;
    // Reset per-encounter state when the active tab changes.
    results = [];
    loading = true;
    loadSaves();
    const timer = setInterval(loadSaves, 5000);
    return () => clearInterval(timer);
  });

  async function loadSaves() {
    try {
      const data = await getPendingSaves(encounterId);
      saves = data || [];
      error = null;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function handleResolve(saveId) {
    if (resolving) return;
    resolving = saveId;
    try {
      const result = await resolveMonsterSave(encounterId, saveId);
      results = [{ text: formatMonsterSaveResult(result), ok: true }, ...results];
      error = null;
      await loadSaves(); // the resolved save drops off the list
      if (onResolved) onResolved(); // let the workspace refresh HP
    } catch (e) {
      // 409 already-resolved / player-save, 400 bad ids → plain-text message.
      results = [{ text: e.message, ok: false }, ...results];
    } finally {
      resolving = null;
    }
  }
</script>

{#if saves.length > 0 || results.length > 0}
  <div class="saves-panel" data-testid="pending-saves-panel">
    <h3>Pending monster saves</h3>

    {#if error}
      <p class="error-msg">{error}</p>
    {/if}

    {#if saves.length > 0}
      <ul class="saves-list">
        {#each saves as save (save.id)}
          <li class="save-item" data-testid="pending-save-{save.id}">
            <span class="save-info">
              <span class="save-name">{save.combatant_name}</span>
              <span class="save-ability">{(save.ability || '').toUpperCase()}</span>
              <span class="save-dc">DC {save.dc}</span>
            </span>
            <button
              class="resolve-btn"
              onclick={() => handleResolve(save.id)}
              disabled={resolving === save.id}
              data-testid="resolve-save-{save.id}"
            >
              {resolving === save.id ? 'Resolving…' : 'Resolve save'}
            </button>
          </li>
        {/each}
      </ul>
    {:else}
      <p class="status-msg">No unresolved monster saves.</p>
    {/if}

    {#if results.length > 0}
      <ul class="results-list" data-testid="save-results">
        {#each results as result, i (i)}
          <li class="result-item" class:failure={!result.ok} data-testid="save-result-{i}">
            {result.text}
          </li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}

<style>
  .saves-panel {
    padding: 0.75rem;
    background: #16213e;
    border-radius: 4px;
    border: 1px solid #0f3460;
  }

  .saves-panel h3 {
    margin: 0 0 0.5rem 0;
    color: #e94560;
    font-size: 0.95rem;
  }

  .saves-list,
  .results-list {
    list-style: none;
    padding: 0;
    margin: 0;
  }

  .save-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.4rem 0.5rem;
    border-top: 1px solid #0f3460;
    font-size: 0.8rem;
  }

  .save-info {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
  }

  .save-name {
    font-weight: bold;
    color: #e0e0e0;
  }

  .save-ability {
    font-size: 0.65rem;
    padding: 0.1rem 0.35rem;
    border-radius: 3px;
    font-weight: bold;
    background: #0f3460;
    color: #a0aec0;
  }

  .save-dc {
    color: #a0aec0;
    font-size: 0.75rem;
  }

  .resolve-btn {
    padding: 0.2rem 0.5rem;
    background: #22c55e;
    color: #1a1a2e;
    border: none;
    border-radius: 3px;
    cursor: pointer;
    font-size: 0.75rem;
    font-weight: bold;
    white-space: nowrap;
  }

  .resolve-btn:hover:not(:disabled) {
    background: #16a34a;
  }

  .resolve-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .result-item {
    margin-top: 0.3rem;
    padding: 0.3rem 0.5rem;
    border-radius: 3px;
    font-size: 0.78rem;
    background: rgba(34, 197, 94, 0.12);
    border-left: 3px solid #22c55e;
    color: #e0e0e0;
  }

  .result-item.failure {
    background: rgba(239, 68, 68, 0.12);
    border-left-color: #ef4444;
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
