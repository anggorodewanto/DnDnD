<script>
  import { getLegendaryActionPlan, executeLegendaryAction } from './lib/api.js';

  let { encounterId, combatantId, combatantName, budgetRemaining: initialBudget, onclose, onexecute } = $props();

  let plan = $state(null);
  let loading = $state(true);
  let error = $state(null);
  let executing = $state(false);
  let executionResult = $state(null);
  let currentBudget = $state(initialBudget);

  $effect(() => {
    if (encounterId && combatantId) {
      loadPlan();
    }
  });

  async function loadPlan() {
    loading = true;
    error = null;
    try {
      plan = await getLegendaryActionPlan(encounterId, combatantId, currentBudget);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function useAction(action) {
    if (!action.affordable) return;
    executing = true;
    error = null;
    try {
      const result = await executeLegendaryAction(encounterId, {
        combatant_id: combatantId,
        action_name: action.name,
        budget_remaining: currentBudget,
      });
      executionResult = result;
      currentBudget = result.budget_remaining;
      if (onexecute) onexecute(result);
    } catch (e) {
      error = e.message;
    } finally {
      executing = false;
    }
  }
</script>

<div class="legendary-panel">
  <div class="lp-header">
    <h3>{combatantName} - Legendary Action</h3>
    <button class="close-btn" onclick={onclose}>Close</button>
  </div>

  {#if loading}
    <div class="lp-loading">Loading legendary actions...</div>
  {:else if error}
    <div class="lp-error">{error}</div>
  {:else if executionResult}
    <div class="lp-result">
      <pre class="combat-log">{executionResult.combat_log}</pre>
      <p class="budget-info">Budget remaining: {currentBudget} / {plan?.budget_total || 3}</p>
      <div class="lp-actions">
        <button class="lp-btn" onclick={() => { executionResult = null; loadPlan(); }}>Use Another</button>
        <button class="lp-btn primary" onclick={onclose}>Done</button>
      </div>
    </div>
  {:else if plan}
    <div class="budget-display">
      Budget: {plan.budget_remaining} / {plan.budget_total}
    </div>

    {#each plan.available_actions as action}
      <button
        class="action-card"
        class:disabled={!action.affordable}
        disabled={!action.affordable || executing}
        onclick={() => useAction(action)}
      >
        <div class="action-header">
          <span class="action-name">{action.name}</span>
          <span class="action-cost">Cost: {action.cost}</span>
        </div>
        <p class="action-desc">{action.description}</p>
      </button>
    {/each}

    <div class="lp-actions">
      <button class="lp-btn" onclick={onclose}>Pass (No Legendary Action)</button>
    </div>
  {/if}
</div>

<style>
  .legendary-panel {
    background: #1a1a2e;
    border: 1px solid #e9a020;
    border-radius: 8px;
    padding: 1rem;
    max-width: 500px;
    margin: 0 auto;
  }

  .lp-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .lp-header h3 {
    margin: 0;
    color: #e9a020;
  }

  .close-btn {
    background: #333;
    color: #ccc;
    border: 1px solid #555;
    border-radius: 4px;
    padding: 0.25rem 0.75rem;
    cursor: pointer;
  }

  .budget-display {
    text-align: center;
    font-size: 1.1rem;
    font-weight: bold;
    color: #e9a020;
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
    border-color: #e9a020;
  }

  .action-card.disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .action-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 0.25rem;
  }

  .action-name {
    font-weight: bold;
  }

  .action-cost {
    color: #e9a020;
    font-size: 0.85rem;
  }

  .action-desc {
    font-size: 0.8rem;
    color: #888;
    margin: 0;
  }

  .lp-loading, .lp-error {
    padding: 1rem;
    text-align: center;
  }

  .lp-error { color: #e94560; }

  .lp-result {
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

  .budget-info {
    color: #e9a020;
    font-weight: bold;
  }

  .lp-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 1rem;
    justify-content: center;
  }

  .lp-btn {
    padding: 0.5rem 1rem;
    border: 1px solid #0f3460;
    background: #16213e;
    color: #e0e0e0;
    border-radius: 4px;
    cursor: pointer;
  }

  .lp-btn:hover { background: #0f3460; }

  .lp-btn.primary {
    background: #e9a020;
    border-color: #e9a020;
    color: #1a1a2e;
  }
</style>
