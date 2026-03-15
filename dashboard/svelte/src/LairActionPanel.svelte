<script>
  import { getLairActionPlan, executeLairAction } from './lib/api.js';

  let { encounterId, lastUsedAction, onclose, onexecute } = $props();

  let plan = $state(null);
  let loading = $state(true);
  let error = $state(null);
  let executing = $state(false);
  let executionResult = $state(null);

  $effect(() => {
    if (encounterId) {
      loadPlan();
    }
  });

  async function loadPlan() {
    loading = true;
    error = null;
    try {
      plan = await getLairActionPlan(encounterId, lastUsedAction);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function useAction(action) {
    executing = true;
    error = null;
    try {
      const result = await executeLairAction(encounterId, {
        action_name: action.name,
        last_used_action: lastUsedAction || '',
      });
      executionResult = result;
      if (onexecute) onexecute(result);
    } catch (e) {
      error = e.message;
    } finally {
      executing = false;
    }
  }
</script>

<div class="lair-panel">
  <div class="lair-header">
    <h3>Lair Action (Initiative 20)</h3>
    <button class="close-btn" onclick={onclose}>Close</button>
  </div>

  {#if loading}
    <div class="lair-loading">Loading lair actions...</div>
  {:else if error}
    <div class="lair-error">{error}</div>
  {:else if executionResult}
    <div class="lair-result">
      <pre class="combat-log">{executionResult.combat_log}</pre>
      <button class="lair-btn primary" onclick={onclose}>Done</button>
    </div>
  {:else if plan}
    <p class="lair-creature">{plan.creature_name}</p>

    {#each plan.available_actions as action}
      <button
        class="action-card"
        disabled={executing}
        onclick={() => useAction(action)}
      >
        <span class="action-name">{action.name}</span>
        <p class="action-desc">{action.description}</p>
      </button>
    {/each}

    {#if plan.disabled_actions?.length > 0}
      {#each plan.disabled_actions as action}
        <div class="action-card disabled">
          <span class="action-name">{action.name}</span>
          <span class="disabled-tag">(used last round)</span>
          <p class="action-desc">{action.description}</p>
        </div>
      {/each}
    {/if}

    <div class="lair-actions">
      <button class="lair-btn" onclick={onclose}>Skip Lair Action</button>
    </div>
  {/if}
</div>

<style>
  .lair-panel {
    background: #1a1a2e;
    border: 1px solid #7b68ee;
    border-radius: 8px;
    padding: 1rem;
    max-width: 500px;
    margin: 0 auto;
  }

  .lair-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .lair-header h3 {
    margin: 0;
    color: #7b68ee;
  }

  .close-btn {
    background: #333;
    color: #ccc;
    border: 1px solid #555;
    border-radius: 4px;
    padding: 0.25rem 0.75rem;
    cursor: pointer;
  }

  .lair-creature {
    text-align: center;
    font-weight: bold;
    color: #ccc;
    margin-bottom: 1rem;
  }

  .action-card {
    display: block;
    width: 100%;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 0.75rem;
    margin-bottom: 0.5rem;
    cursor: pointer;
    text-align: left;
    color: #e0e0e0;
  }

  .action-card:hover:not(.disabled) {
    border-color: #7b68ee;
  }

  .action-card.disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .action-name {
    font-weight: bold;
  }

  .disabled-tag {
    color: #e94560;
    font-size: 0.75rem;
    margin-left: 0.5rem;
  }

  .action-desc {
    font-size: 0.8rem;
    color: #888;
    margin: 0.25rem 0 0;
  }

  .lair-loading, .lair-error {
    padding: 1rem;
    text-align: center;
  }

  .lair-error { color: #e94560; }

  .lair-result {
    text-align: center;
  }

  .combat-log {
    background: #16213e;
    padding: 0.75rem;
    border-radius: 4px;
    white-space: pre-wrap;
    font-family: monospace;
    font-size: 0.85rem;
    text-align: left;
  }

  .lair-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 1rem;
    justify-content: center;
  }

  .lair-btn {
    padding: 0.5rem 1rem;
    border: 1px solid #0f3460;
    background: #16213e;
    color: #e0e0e0;
    border-radius: 4px;
    cursor: pointer;
  }

  .lair-btn:hover { background: #0f3460; }

  .lair-btn.primary {
    background: #7b68ee;
    border-color: #7b68ee;
    color: white;
  }
</style>
