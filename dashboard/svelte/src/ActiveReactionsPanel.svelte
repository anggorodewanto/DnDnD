<script>
  import { listReactionsPanel, resolveReaction, cancelReaction } from './lib/api.js';

  let { encounterId, activeTurnCombatantId, activeTurnIsNpc, onReactionResolved } = $props();

  let reactions = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let resolving = $state(null); // reaction ID being resolved
  let cancelling = $state(null); // reaction ID being cancelled

  $effect(() => {
    if (encounterId) {
      loadReactions();
    }
  });

  async function loadReactions() {
    try {
      const data = await listReactionsPanel(encounterId);
      reactions = data || [];
      error = null;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function handleResolve(reactionId) {
    if (resolving) return;
    resolving = reactionId;
    try {
      await resolveReaction(encounterId, reactionId);
      await loadReactions();
      if (onReactionResolved) onReactionResolved();
      error = null;
    } catch (e) {
      error = e.message;
    } finally {
      resolving = null;
    }
  }

  async function handleCancel(reactionId) {
    if (cancelling) return;
    cancelling = reactionId;
    try {
      await cancelReaction(encounterId, reactionId);
      await loadReactions();
      error = null;
    } catch (e) {
      error = e.message;
    } finally {
      cancelling = null;
    }
  }

  function effectiveStatus(reaction) {
    if (reaction.status !== 'active') return reaction.status;
    if (reaction.reaction_used_this_round) return 'dormant';
    return 'active';
  }

  function shouldHighlight(reaction) {
    if (!activeTurnIsNpc) return false;
    return effectiveStatus(reaction) === 'active';
  }

  let groupedReactions = $derived((() => {
    const groups = new Map();
    for (const r of reactions) {
      const key = r.combatant_id;
      if (!groups.has(key)) {
        groups.set(key, {
          combatantId: r.combatant_id,
          displayName: r.combatant_display_name,
          shortId: r.combatant_short_id,
          reactions: [],
        });
      }
      groups.get(key).reactions.push(r);
    }
    return [...groups.values()];
  })());
</script>

<div class="reactions-panel" data-testid="reactions-panel">
  <h3>Active Reactions</h3>

  {#if loading}
    <p class="status-msg">Loading...</p>
  {:else if error}
    <p class="error-msg">{error}</p>
  {:else if groupedReactions.length === 0}
    <p class="status-msg">No reaction declarations.</p>
  {:else}
    {#each groupedReactions as group}
      <div class="combatant-group" data-testid="combatant-group-{group.shortId}">
        <div class="combatant-header">
          <span class="combatant-name">{group.displayName}</span>
          <span class="combatant-short-id">({group.shortId})</span>
        </div>
        <ul class="reaction-list">
          {#each group.reactions as reaction}
            {@const status = effectiveStatus(reaction)}
            <li
              class="reaction-item"
              class:used={status === 'used'}
              class:cancelled={status === 'cancelled'}
              class:dormant={status === 'dormant'}
              class:highlighted={shouldHighlight(reaction)}
              data-testid="reaction-{reaction.id}"
            >
              <div class="reaction-info">
                <span class="reaction-desc">{reaction.description}</span>
                <span class="status-badge status-{status}">
                  {status}
                </span>
                {#if reaction.is_readied_action}
                  <span class="readied-badge">Readied</span>
                {/if}
              </div>
              {#if status === 'active'}
                <div class="reaction-actions">
                  <button
                    class="resolve-btn"
                    onclick={() => handleResolve(reaction.id)}
                    disabled={resolving === reaction.id}
                    data-testid="resolve-{reaction.id}"
                  >
                    {resolving === reaction.id ? 'Resolving...' : 'Resolve'}
                  </button>
                  <button
                    class="dismiss-btn"
                    onclick={() => handleCancel(reaction.id)}
                    disabled={cancelling === reaction.id}
                    data-testid="dismiss-{reaction.id}"
                  >
                    {cancelling === reaction.id ? 'Dismissing...' : 'Dismiss'}
                  </button>
                </div>
              {/if}
            </li>
          {/each}
        </ul>
      </div>
    {/each}
  {/if}
</div>

<style>
  .reactions-panel {
    padding: 0.75rem;
    background: #16213e;
    border-radius: 4px;
    border: 1px solid #0f3460;
  }

  .reactions-panel h3 {
    margin: 0 0 0.5rem 0;
    color: #e94560;
    font-size: 0.95rem;
  }

  .combatant-group {
    margin-bottom: 0.5rem;
    border: 1px solid #0f3460;
    border-radius: 4px;
    overflow: hidden;
  }

  .combatant-header {
    padding: 0.35rem 0.5rem;
    background: #1a1a2e;
    font-size: 0.85rem;
  }

  .combatant-name {
    font-weight: bold;
    color: #3b82f6;
  }

  .combatant-short-id {
    color: #a0aec0;
    font-size: 0.75rem;
    margin-left: 0.25rem;
  }

  .reaction-list {
    list-style: none;
    padding: 0;
    margin: 0;
  }

  .reaction-item {
    padding: 0.4rem 0.5rem;
    border-top: 1px solid #0f3460;
    font-size: 0.8rem;
  }

  .reaction-item.used,
  .reaction-item.cancelled {
    opacity: 0.45;
  }

  .reaction-item.dormant {
    opacity: 0.6;
  }

  .reaction-item.highlighted {
    background: rgba(233, 69, 96, 0.15);
    border-left: 3px solid #e94560;
  }

  .reaction-info {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
  }

  .reaction-desc {
    flex: 1;
    color: #e0e0e0;
  }

  .status-badge {
    font-size: 0.65rem;
    padding: 0.1rem 0.35rem;
    border-radius: 3px;
    font-weight: bold;
    text-transform: uppercase;
  }

  .status-active {
    background: #22c55e;
    color: #1a1a2e;
  }

  .status-used {
    background: #6b7280;
    color: #e0e0e0;
  }

  .status-dormant {
    background: #f59e0b;
    color: #1a1a2e;
  }

  .status-cancelled {
    background: #ef4444;
    color: #e0e0e0;
  }

  .readied-badge {
    font-size: 0.65rem;
    padding: 0.1rem 0.35rem;
    background: #8b5cf6;
    color: #e0e0e0;
    border-radius: 3px;
    font-weight: bold;
  }

  .reaction-actions {
    display: flex;
    gap: 0.3rem;
    margin-top: 0.3rem;
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
  }

  .resolve-btn:hover:not(:disabled) {
    background: #16a34a;
  }

  .resolve-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .dismiss-btn {
    padding: 0.2rem 0.5rem;
    background: #6b7280;
    color: #e0e0e0;
    border: none;
    border-radius: 3px;
    cursor: pointer;
    font-size: 0.75rem;
  }

  .dismiss-btn:hover:not(:disabled) {
    background: #ef4444;
  }

  .dismiss-btn:disabled {
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
