<script>
  // F-16 — SPA migration of the Phase 89 server-rendered level-up widget
  // (internal/levelup/handler.go). Mirrors that page's functionality:
  //   * Character ID + Class + New Level inputs
  //   * POST /api/levelup -> result card (level / HP / prof bonus / attacks)
  //   * ASI prompt + subclass alert when the response flags them.
  //
  // The legacy /dashboard/levelup HTML page is preserved as a fallback for
  // now; this panel is the canonical UI going forward.
  //
  // NOTE: DDB-imported characters should re-import via Phase 90 rather than
  // leveling here (same caveat as the legacy widget).
  import { applyLevelUp } from './lib/api.js';

  let characterId = $state('');
  let classId = $state('');
  let newLevel = $state(1);

  let result = $state(null);
  let status = $state(null); // { type: 'success' | 'error' | 'warning', msg }
  let submitting = $state(false);

  async function onSubmit() {
    const charID = characterId.trim();
    const classID = classId.trim();
    const lvl = Number(newLevel);
    if (!charID || !classID || !lvl || lvl < 1) {
      status = { type: 'error', msg: 'Please fill in all fields.' };
      return;
    }

    submitting = true;
    status = null;
    try {
      const resp = await applyLevelUp({
        character_id: charID,
        class_id: classID,
        new_level: lvl,
      });
      result = resp;
      status = { type: 'success', msg: 'Level up applied successfully!' };
    } catch (e) {
      result = null;
      status = { type: 'error', msg: 'Error: ' + (e.message || 'request failed') };
    } finally {
      submitting = false;
    }
  }
</script>

<section class="levelup-panel">
  <header>
    <h2>Level Up Character</h2>
    <p class="hint">
      Apply a class level-up. DDB-imported characters should re-import via
      Phase 90 instead of leveling here.
    </p>
  </header>

  <div class="form-group">
    <label for="lu-character-id">Character ID</label>
    <input
      id="lu-character-id"
      type="text"
      placeholder="Enter character UUID"
      bind:value={characterId}
      disabled={submitting}
    />
  </div>

  <div class="form-group">
    <label for="lu-class-id">Class</label>
    <input
      id="lu-class-id"
      type="text"
      placeholder="e.g. fighter, wizard, cleric"
      bind:value={classId}
      disabled={submitting}
    />
  </div>

  <div class="form-group">
    <label for="lu-new-level">New Class Level</label>
    <input
      id="lu-new-level"
      type="number"
      min="1"
      max="20"
      bind:value={newLevel}
      disabled={submitting}
    />
  </div>

  <button class="btn" type="button" onclick={onSubmit} disabled={submitting}>
    {submitting ? 'Applying...' : 'Apply Level Up'}
  </button>

  {#if result}
    <div class="result-card">
      <h3>Level Up Result</h3>
      <div class="stat"><span class="label">New Total Level:</span> <span data-testid="res-level">{result.new_level}</span></div>
      <div class="stat"><span class="label">HP Gained:</span> <span data-testid="res-hp-gained">+{result.hp_gained}</span></div>
      <div class="stat"><span class="label">New HP Max:</span> <span data-testid="res-hp-max">{result.new_hp_max}</span></div>
      <div class="stat"><span class="label">Proficiency Bonus:</span> +<span data-testid="res-prof">{result.new_proficiency_bonus}</span></div>
      <div class="stat"><span class="label">Attacks per Action:</span> <span data-testid="res-attacks">{result.new_attacks_per_action}</span></div>
    </div>

    {#if result.grants_asi}
      <div class="asi-section">
        <h3>ASI / Feat Choice Pending</h3>
        <p>This level grants an Ability Score Improvement. The player will be prompted in Discord to choose:</p>
        <ul>
          <li>+2 to one ability score</li>
          <li>+1 to two different ability scores</li>
          <li>A feat (with prerequisite check)</li>
        </ul>
        <p>The choice will appear in the DM queue for approval.</p>
      </div>
    {/if}

    {#if result.needs_subclass}
      <div class="alert alert-warning">
        <strong>Subclass Selection Needed:</strong>
        This character needs to choose a subclass. Work with the player to select one.
      </div>
    {/if}
  {/if}

  {#if status}
    <div class="alert alert-{status.type}" role={status.type === 'error' ? 'alert' : 'status'}>
      {status.msg}
    </div>
  {/if}
</section>

<style>
  .levelup-panel {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 1rem;
    max-width: 720px;
  }
  header h2 {
    margin: 0 0 0.25rem;
    color: #e94560;
  }
  .hint {
    color: #b0b0c0;
    margin: 0 0 1rem;
    font-size: 0.9rem;
  }
  .form-group {
    margin-bottom: 1rem;
  }
  .form-group label {
    display: block;
    margin-bottom: 0.25rem;
    font-weight: 600;
  }
  .form-group input {
    width: 100%;
    padding: 0.5rem;
    border-radius: 4px;
    border: 1px solid #0f3460;
    background: #16213e;
    color: #e0e0e0;
    font-size: 1rem;
    box-sizing: border-box;
  }
  .btn {
    padding: 0.6rem 1.2rem;
    background: #e94560;
    color: #fff;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 1rem;
  }
  .btn:hover:not(:disabled) {
    background: #c73852;
  }
  .btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
  .result-card {
    background: #0f3460;
    border-radius: 4px;
    padding: 1rem;
    border: 1px solid #16213e;
    margin-top: 1rem;
  }
  .result-card h3 {
    color: #e94560;
    margin: 0 0 0.5rem;
  }
  .stat {
    margin: 0.25rem 0;
  }
  .stat .label {
    color: #b0b0c0;
  }
  .asi-section {
    margin-top: 1rem;
    padding: 1rem;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }
  .asi-section h3 {
    color: #e94560;
    margin: 0 0 0.5rem;
  }
  .asi-section ul {
    margin: 0.25rem 0 0.5rem 1.5rem;
  }
  .alert {
    padding: 0.6rem 0.8rem;
    border-radius: 4px;
    margin-top: 1rem;
  }
  .alert-success {
    background: #1b4332;
    border: 1px solid #2d6a4f;
  }
  .alert-warning {
    background: #533a1b;
    border: 1px solid #6a4f2d;
  }
  .alert-error {
    background: #4a1b1b;
    border: 1px solid #6a2d2d;
    color: #ff6b6b;
  }
</style>
