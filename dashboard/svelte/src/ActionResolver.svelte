<script>
  import { getPendingActions, resolvePendingAction } from './lib/api.js';
  import { STANDARD_CONDITIONS } from './lib/combat.js';

  let { encounterId, combatants, onResolved } = $props();

  let actions = $state([]);
  let loading = $state(true);
  let error = $state(null);

  // Expanded action state
  let expandedActionId = $state(null);
  let outcomeText = $state('');
  let resolving = $state(false);

  // Effect builder state
  let effectType = $state('damage');
  let effectTargetId = $state('');
  let effectDamageAmount = $state(0);
  let effectCondition = $state('');
  let effectMoveCol = $state('');
  let effectMoveRow = $state(0);
  let pendingEffects = $state([]);

  $effect(() => {
    if (encounterId) {
      loadActions();
    }
  });

  async function loadActions() {
    try {
      const data = await getPendingActions(encounterId);
      actions = data || [];
      error = null;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function toggleExpand(actionId) {
    if (expandedActionId === actionId) {
      expandedActionId = null;
      return;
    }
    expandedActionId = actionId;
    outcomeText = '';
    pendingEffects = [];
    effectType = 'damage';
    effectTargetId = '';
    effectDamageAmount = 0;
    effectCondition = '';
    effectMoveCol = '';
    effectMoveRow = 0;
  }

  function addEffect() {
    if (!effectTargetId) return;

    let value;
    switch (effectType) {
      case 'damage':
        if (effectDamageAmount <= 0) return;
        value = { amount: effectDamageAmount };
        break;
      case 'condition_add':
      case 'condition_remove':
        if (!effectCondition) return;
        value = { condition: effectCondition };
        break;
      case 'move':
        if (!effectMoveCol) return;
        value = { col: effectMoveCol, row: effectMoveRow };
        break;
      default:
        return;
    }

    const targetName = combatants?.find(c => c.id === effectTargetId)?.display_name || effectTargetId;

    pendingEffects = [...pendingEffects, {
      type: effectType,
      target_id: effectTargetId,
      target_name: targetName,
      value,
    }];

    // Reset inputs
    effectDamageAmount = 0;
    effectCondition = '';
    effectMoveCol = '';
    effectMoveRow = 0;
  }

  function removeEffect(index) {
    pendingEffects = pendingEffects.filter((_, i) => i !== index);
  }

  function formatEffect(eff) {
    switch (eff.type) {
      case 'damage':
        return `${eff.value.amount} damage to ${eff.target_name}`;
      case 'condition_add':
        return `Add ${eff.value.condition} to ${eff.target_name}`;
      case 'condition_remove':
        return `Remove ${eff.value.condition} from ${eff.target_name}`;
      case 'move':
        return `Move ${eff.target_name} to ${eff.value.col}${eff.value.row}`;
      default:
        return `${eff.type} on ${eff.target_name}`;
    }
  }

  async function handleResolve(actionId) {
    if (resolving) return;
    resolving = true;
    try {
      const effects = pendingEffects.map(e => ({
        type: e.type,
        target_id: e.target_id,
        value: e.value,
      }));
      await resolvePendingAction(encounterId, actionId, {
        outcome: outcomeText,
        effects,
      });
      expandedActionId = null;
      outcomeText = '';
      pendingEffects = [];
      await loadActions();
      if (onResolved) onResolved();
      error = null;
    } catch (e) {
      error = e.message;
    } finally {
      resolving = false;
    }
  }

  function getCombatantName(combatantId) {
    const c = combatants?.find(c => c.id === combatantId);
    return c ? c.display_name : combatantId;
  }
</script>

<div class="action-resolver" data-testid="action-resolver">
  <h3>Action Queue</h3>

  {#if loading}
    <p class="status-msg">Loading...</p>
  {:else if error}
    <p class="error-msg">{error}</p>
  {:else if actions.length === 0}
    <p class="status-msg">No pending actions.</p>
  {:else}
    <ul class="action-list" data-testid="action-list">
      {#each actions as action}
        <li class="action-item" class:resolved={action.status === 'resolved'} data-testid="action-item-{action.id}">
          <button
            class="action-header"
            onclick={() => toggleExpand(action.id)}
            disabled={action.status !== 'pending'}
          >
            <span class="action-actor">{getCombatantName(action.combatant_id)}</span>
            <span class="action-text">{action.action_text}</span>
            {#if action.status === 'resolved'}
              <span class="resolved-badge">Resolved</span>
            {/if}
          </button>

          {#if expandedActionId === action.id && action.status === 'pending'}
            <div class="resolve-controls" data-testid="resolve-controls">
              <div class="outcome-row">
                <label>
                  Outcome:
                  <input
                    type="text"
                    bind:value={outcomeText}
                    placeholder="Describe the outcome..."
                    data-testid="outcome-input"
                  />
                </label>
              </div>

              <!-- Effect builder -->
              <div class="effect-builder">
                <h4>Effects</h4>
                <div class="effect-row">
                  <select bind:value={effectType} data-testid="effect-type-select">
                    <option value="damage">Damage</option>
                    <option value="condition_add">Add Condition</option>
                    <option value="condition_remove">Remove Condition</option>
                    <option value="move">Move</option>
                  </select>

                  <select bind:value={effectTargetId} data-testid="effect-target-select">
                    <option value="">-- Target --</option>
                    {#each combatants || [] as c}
                      <option value={c.id}>{c.display_name}</option>
                    {/each}
                  </select>

                  {#if effectType === 'damage'}
                    <input type="number" min="0" bind:value={effectDamageAmount} placeholder="Amount" data-testid="effect-damage-input" />
                  {:else if effectType === 'condition_add' || effectType === 'condition_remove'}
                    <select bind:value={effectCondition} data-testid="effect-condition-select">
                      <option value="">-- Condition --</option>
                      {#each STANDARD_CONDITIONS as cond}
                        <option value={cond.toLowerCase()}>{cond}</option>
                      {/each}
                    </select>
                  {:else if effectType === 'move'}
                    <input type="text" bind:value={effectMoveCol} placeholder="Col" class="col-input" data-testid="effect-move-col" />
                    <input type="number" min="0" bind:value={effectMoveRow} placeholder="Row" class="row-input" data-testid="effect-move-row" />
                  {/if}

                  <button class="add-effect-btn" onclick={addEffect} data-testid="add-effect-btn">+ Add</button>
                </div>

                {#if pendingEffects.length > 0}
                  <ul class="effect-list" data-testid="effect-list">
                    {#each pendingEffects as eff, i}
                      <li class="effect-tag">
                        {formatEffect(eff)}
                        <button class="remove-effect-btn" onclick={() => removeEffect(i)}>x</button>
                      </li>
                    {/each}
                  </ul>
                {/if}
              </div>

              <button
                class="resolve-btn"
                onclick={() => handleResolve(action.id)}
                disabled={resolving}
                data-testid="resolve-btn"
              >
                {resolving ? 'Resolving...' : 'Resolve'}
              </button>
            </div>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .action-resolver {
    padding: 0.75rem;
    background: #16213e;
    border-radius: 4px;
    border: 1px solid #0f3460;
  }

  .action-resolver h3 {
    margin: 0 0 0.5rem 0;
    color: #e94560;
    font-size: 0.95rem;
  }

  .action-list {
    list-style: none;
    padding: 0;
    margin: 0;
  }

  .action-item {
    margin-bottom: 0.5rem;
    border: 1px solid #0f3460;
    border-radius: 4px;
    overflow: hidden;
  }

  .action-item.resolved {
    opacity: 0.6;
  }

  .action-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    padding: 0.5rem;
    background: #1a1a2e;
    border: none;
    color: #e0e0e0;
    cursor: pointer;
    text-align: left;
    font-size: 0.85rem;
  }

  .action-header:hover:not(:disabled) {
    background: #0f3460;
  }

  .action-header:disabled {
    cursor: default;
  }

  .action-actor {
    font-weight: bold;
    color: #3b82f6;
    min-width: 80px;
  }

  .action-text {
    flex: 1;
  }

  .resolved-badge {
    font-size: 0.7rem;
    padding: 0.1rem 0.4rem;
    background: #22c55e;
    color: #1a1a2e;
    border-radius: 3px;
    font-weight: bold;
  }

  .resolve-controls {
    padding: 0.5rem;
    background: #1a1a2e;
    border-top: 1px solid #0f3460;
  }

  .outcome-row {
    margin-bottom: 0.5rem;
  }

  .outcome-row label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.85rem;
    color: #a0aec0;
  }

  .outcome-row input {
    padding: 0.4rem;
    background: #16213e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }

  .effect-builder {
    margin-bottom: 0.5rem;
  }

  .effect-builder h4 {
    margin: 0 0 0.25rem 0;
    color: #a0aec0;
    font-size: 0.8rem;
  }

  .effect-row {
    display: flex;
    gap: 0.3rem;
    align-items: center;
    flex-wrap: wrap;
  }

  .effect-row select,
  .effect-row input {
    padding: 0.3rem;
    background: #16213e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    font-size: 0.8rem;
  }

  .effect-row input[type='number'] {
    width: 60px;
  }

  .col-input {
    width: 50px;
  }

  .row-input {
    width: 60px;
  }

  .add-effect-btn {
    padding: 0.3rem 0.6rem;
    background: #0f3460;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.8rem;
  }

  .add-effect-btn:hover {
    background: #e94560;
    border-color: #e94560;
  }

  .effect-list {
    list-style: none;
    padding: 0;
    margin: 0.3rem 0;
  }

  .effect-tag {
    display: inline-flex;
    align-items: center;
    gap: 0.2rem;
    padding: 0.15rem 0.5rem;
    background: #0f3460;
    color: #e0e0e0;
    border-radius: 12px;
    font-size: 0.75rem;
    margin: 0.15rem;
  }

  .remove-effect-btn {
    background: none;
    border: none;
    color: #ef4444;
    cursor: pointer;
    font-size: 0.75rem;
    padding: 0;
    margin-left: 0.2rem;
  }

  .resolve-btn {
    width: 100%;
    padding: 0.4rem;
    background: #22c55e;
    color: #1a1a2e;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-weight: bold;
    font-size: 0.85rem;
  }

  .resolve-btn:hover:not(:disabled) {
    background: #16a34a;
  }

  .resolve-btn:disabled {
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
