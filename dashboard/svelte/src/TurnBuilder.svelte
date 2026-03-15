<script>
  import { getEnemyTurnPlan, executeEnemyTurn } from './lib/api.js';

  let { encounterId, combatantId, combatantName, onclose } = $props();

  let plan = $state(null);
  let loading = $state(true);
  let error = $state(null);
  let currentStep = $state(0);
  let executing = $state(false);
  let executionResult = $state(null);
  let reviewMode = $state(false);

  // Load the turn plan on mount
  $effect(() => {
    if (encounterId && combatantId) {
      loadPlan();
    }
  });

  async function loadPlan() {
    loading = true;
    error = null;
    try {
      plan = await getEnemyTurnPlan(encounterId, combatantId);
      currentStep = 0;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function nextStep() {
    if (!plan) return;
    if (currentStep < plan.steps.length - 1) {
      currentStep++;
    } else {
      reviewMode = true;
    }
  }

  function prevStep() {
    if (currentStep > 0) {
      currentStep--;
      reviewMode = false;
    }
  }

  function skipStep() {
    if (!plan) return;
    plan.steps = plan.steps.filter((_, i) => i !== currentStep);
    if (currentStep >= plan.steps.length) {
      currentStep = Math.max(0, plan.steps.length - 1);
    }
    if (plan.steps.length === 0) {
      reviewMode = true;
    }
  }

  function removeStep(index) {
    if (!plan) return;
    plan.steps = plan.steps.filter((_, i) => i !== index);
  }

  function updateRoll(stepIndex, field, value) {
    if (!plan) return;
    const step = plan.steps[stepIndex];
    if (step.attack && step.attack.roll_result) {
      step.attack.roll_result[field] = parseInt(value) || 0;
    }
  }

  async function confirmAndPost() {
    if (!plan) return;
    executing = true;
    error = null;
    try {
      const result = await executeEnemyTurn(encounterId, {
        combatant_id: plan.combatant_id,
        steps: plan.steps,
      });
      executionResult = result;
    } catch (e) {
      error = e.message;
    } finally {
      executing = false;
    }
  }

  function stepTypeLabel(type) {
    switch (type) {
      case 'movement': return 'Movement';
      case 'attack': return 'Attack';
      case 'multiattack': return 'Multiattack';
      case 'ability': return 'Special Ability';
      case 'bonus_action': return 'Bonus Action';
      default: return type;
    }
  }
</script>

<div class="turn-builder">
  <div class="tb-header">
    <h2>{combatantName || 'Enemy'} Turn Builder</h2>
    <button class="close-btn" onclick={onclose}>Close</button>
  </div>

  {#if loading}
    <div class="tb-loading">Loading turn plan...</div>
  {:else if error}
    <div class="tb-error">{error}</div>
  {:else if executionResult}
    <div class="tb-result">
      <h3>Turn Complete</h3>
      <pre class="combat-log">{executionResult.combat_log}</pre>
      <p class="posted-indicator">Sent to #combat-log</p>
      <button class="tb-btn primary" onclick={onclose}>Close</button>
    </div>
  {:else if plan}
    {#if plan.reactions && plan.reactions.length > 0}
      <div class="tb-reactions">
        <h3>Pending Reactions</h3>
        {#each plan.reactions as reaction}
          <div class="reaction-card">
            <span class="reaction-desc">{reaction.description}</span>
            <span class="reaction-status">({reaction.status})</span>
          </div>
        {/each}
      </div>
    {/if}

    {#if !reviewMode}
      <!-- Step-by-step view -->
      <div class="tb-step-indicator">
        Step {currentStep + 1} of {plan.steps.length}
      </div>

      {#if plan.steps.length > 0}
        {@const step = plan.steps[currentStep]}
        <div class="tb-step-card">
          <div class="step-type">{stepTypeLabel(step.type)}</div>

          {#if step.type === 'movement' && step.movement}
            <div class="step-detail">
              <p>Move {step.movement.total_cost_ft}ft</p>
              <p class="step-path">Path: {step.movement.path?.length || 0} tiles</p>
            </div>
          {/if}

          {#if step.type === 'attack' && step.attack}
            <div class="step-detail">
              <p><strong>{step.attack.weapon_name}</strong> vs {step.attack.target_name}</p>
              <p>To Hit: +{step.attack.to_hit} | Damage: {step.attack.damage_dice} {step.attack.damage_type}</p>
              <p>Reach: {step.attack.reach_ft}ft</p>
            </div>
          {/if}

          {#if step.type === 'ability' && step.ability}
            <div class="step-detail">
              <p><strong>{step.ability.name}</strong></p>
              <p class="ability-desc">{step.ability.description}</p>
              {#if step.ability.is_recharge}
                <p class="recharge-note">Recharge: {step.ability.recharge_min}+</p>
              {/if}
            </div>
          {/if}

          {#if step.type === 'bonus_action' && step.ability}
            <div class="step-detail">
              <p><strong>{step.ability.name}</strong></p>
              <p class="ability-desc">{step.ability.description}</p>
            </div>
          {/if}

          <div class="step-actions">
            <button class="tb-btn confirm" onclick={nextStep}>
              {currentStep < plan.steps.length - 1 ? 'Confirm & Next' : 'Review'}
            </button>
            <button class="tb-btn skip" onclick={skipStep}>Skip</button>
          </div>
        </div>
      {:else}
        <div class="tb-empty">No steps in plan. <button class="tb-btn" onclick={() => reviewMode = true}>Go to Review</button></div>
      {/if}

      <div class="tb-nav">
        <button class="tb-btn" onclick={prevStep} disabled={currentStep === 0}>Previous</button>
      </div>
    {:else}
      <!-- Review mode -->
      <div class="tb-review">
        <h3>Review Turn</h3>
        {#if plan.steps.length === 0}
          <p>No actions to execute.</p>
        {/if}
        {#each plan.steps as step, i}
          <div class="review-step">
            <span class="review-type">{stepTypeLabel(step.type)}</span>
            {#if step.type === 'movement' && step.movement}
              <span>Move {step.movement.total_cost_ft}ft</span>
            {/if}
            {#if step.type === 'attack' && step.attack}
              <span>{step.attack.weapon_name} vs {step.attack.target_name} (+{step.attack.to_hit})</span>
              {#if step.attack.roll_result}
                <div class="roll-fudge">
                  <label>To Hit: <input type="number" value={step.attack.roll_result.to_hit_total} onchange={(e) => updateRoll(i, 'to_hit_total', e.target.value)} /></label>
                  <label>Damage: <input type="number" value={step.attack.roll_result.damage_total} onchange={(e) => updateRoll(i, 'damage_total', e.target.value)} /></label>
                </div>
              {/if}
            {/if}
            {#if step.type === 'ability' && step.ability}
              <span>{step.ability.name}</span>
            {/if}
            {#if step.type === 'bonus_action' && step.ability}
              <span>{step.ability.name}</span>
            {/if}
            <button class="remove-btn" onclick={() => removeStep(i)}>Remove</button>
          </div>
        {/each}

        <div class="review-actions">
          <button class="tb-btn" onclick={() => { reviewMode = false; currentStep = 0; }}>Back to Steps</button>
          <button class="tb-btn primary" onclick={confirmAndPost} disabled={executing}>
            {executing ? 'Executing...' : 'Confirm & Post'}
          </button>
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .turn-builder {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    padding: 1rem;
    max-width: 600px;
    margin: 0 auto;
  }

  .tb-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .tb-header h2 {
    margin: 0;
    color: #e94560;
  }

  .close-btn {
    background: #333;
    color: #ccc;
    border: 1px solid #555;
    border-radius: 4px;
    padding: 0.25rem 0.75rem;
    cursor: pointer;
  }

  .tb-loading, .tb-error {
    padding: 1rem;
    text-align: center;
  }

  .tb-error {
    color: #e94560;
  }

  .tb-reactions {
    background: #1a1a2e;
    border: 1px solid #e94560;
    border-radius: 4px;
    padding: 0.75rem;
    margin-bottom: 1rem;
  }

  .tb-reactions h3 {
    margin: 0 0 0.5rem;
    color: #e94560;
    font-size: 0.9rem;
  }

  .reaction-card {
    padding: 0.25rem 0;
    font-size: 0.85rem;
  }

  .reaction-status {
    color: #888;
  }

  .tb-step-indicator {
    text-align: center;
    color: #888;
    margin-bottom: 0.5rem;
    font-size: 0.85rem;
  }

  .tb-step-card {
    background: #1a1a2e;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 1rem;
    margin-bottom: 1rem;
  }

  .step-type {
    font-weight: bold;
    color: #e94560;
    text-transform: uppercase;
    font-size: 0.8rem;
    margin-bottom: 0.5rem;
  }

  .step-detail p {
    margin: 0.25rem 0;
  }

  .step-path, .ability-desc {
    color: #888;
    font-size: 0.85rem;
  }

  .recharge-note {
    color: #ffaa00;
    font-weight: bold;
  }

  .step-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.75rem;
  }

  .tb-btn {
    padding: 0.5rem 1rem;
    border: 1px solid #0f3460;
    background: #16213e;
    color: #e0e0e0;
    border-radius: 4px;
    cursor: pointer;
  }

  .tb-btn:hover:not(:disabled) {
    background: #0f3460;
  }

  .tb-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .tb-btn.primary {
    background: #e94560;
    border-color: #e94560;
    color: white;
  }

  .tb-btn.primary:hover:not(:disabled) {
    background: #c73a52;
  }

  .tb-btn.confirm {
    background: #28a745;
    border-color: #28a745;
    color: white;
  }

  .tb-btn.skip {
    background: #6c757d;
    border-color: #6c757d;
    color: white;
  }

  .tb-nav {
    display: flex;
    gap: 0.5rem;
  }

  .tb-review h3 {
    margin: 0 0 0.75rem;
    color: #e94560;
  }

  .review-step {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem;
    background: #1a1a2e;
    border: 1px solid #0f3460;
    border-radius: 4px;
    margin-bottom: 0.5rem;
    flex-wrap: wrap;
  }

  .review-type {
    color: #e94560;
    font-weight: bold;
    font-size: 0.75rem;
    text-transform: uppercase;
    min-width: 80px;
  }

  .roll-fudge {
    display: flex;
    gap: 0.5rem;
    width: 100%;
    margin-top: 0.25rem;
  }

  .roll-fudge label {
    font-size: 0.8rem;
    color: #888;
  }

  .roll-fudge input {
    width: 60px;
    padding: 0.2rem;
    background: #16213e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 2px;
  }

  .remove-btn {
    margin-left: auto;
    background: #6c757d;
    color: white;
    border: none;
    border-radius: 4px;
    padding: 0.25rem 0.5rem;
    cursor: pointer;
    font-size: 0.75rem;
  }

  .remove-btn:hover {
    background: #e94560;
  }

  .review-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 1rem;
    justify-content: space-between;
  }

  .tb-result {
    text-align: center;
  }

  .posted-indicator {
    color: #28a745;
    font-size: 0.85rem;
    margin: 0.5rem 0;
  }

  .combat-log {
    background: #1a1a2e;
    padding: 1rem;
    border-radius: 4px;
    text-align: left;
    white-space: pre-wrap;
    font-family: monospace;
    font-size: 0.85rem;
    margin: 1rem 0;
  }

  .tb-empty {
    text-align: center;
    padding: 1rem;
    color: #888;
  }
</style>
