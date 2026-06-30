<script>
  // DM Console: the "start here" overview that aggregates everything a DM
  // needs at a glance into one read-only panel. It hits a single endpoint
  // (GET /api/dm/situation) and renders, top to bottom: the single suggested
  // next step, the unified pending worklist (queue + approvals + level-ups),
  // the live encounter state, and the recent timeline. The server resolves the
  // active campaign from the authenticated session, so no campaign id prop.
  import { fetchDMSituation, formatConditions, formatDeathSaves } from './lib/dmsituation.js';
  import { resolveMonsterSaveByUrl } from './lib/api.js';
  import { formatMonsterSaveResult } from './lib/combat.js';

  let situation = $state(null);
  let loading = $state(true);
  let loadError = $state(null);

  // Monster-save resolution: the situation `pending[]` carries kind:"monster_save"
  // items whose resolve_url is a POST endpoint (not a hash route), so they get a
  // Resolve button instead of a plain link. resolvingSaveId guards a double-click;
  // saveResults keeps the rolled outcome around after the item drops off the list.
  let resolvingSaveId = $state(null);
  let saveResults = $state([]); // [{ text, ok }] newest-first

  async function load() {
    loading = true;
    loadError = null;
    try {
      situation = await fetchDMSituation();
    } catch (e) {
      loadError = e.message;
    } finally {
      loading = false;
    }
  }

  async function resolveSave(item) {
    if (resolvingSaveId) return;
    resolvingSaveId = item.id;
    try {
      const result = await resolveMonsterSaveByUrl(item.resolve_url);
      saveResults = [{ text: formatMonsterSaveResult(result), ok: true }, ...saveResults];
      await load(); // the resolved save drops off pending
    } catch (e) {
      // 404 not found, 409 already-resolved / player-save, 400 bad ids → plain text.
      saveResults = [{ text: e.message, ok: false }, ...saveResults];
    } finally {
      resolvingSaveId = null;
    }
  }

  // Most urgent first (lower priority value = more urgent). The endpoint may
  // already sort, but sorting here keeps the render order well-defined.
  function sortedPending(pending) {
    return [...pending].sort((a, b) => a.priority - b.priority);
  }

  function isUrgent(item) {
    return item.priority <= 1;
  }

  // Newest-first timeline. Parse RFC3339 `at` and compare descending.
  function sortedTimeline(timeline) {
    return [...timeline].sort((a, b) => new Date(b.at) - new Date(a.at));
  }

  $effect(() => {
    load();
  });
</script>

<div class="dmconsole-panel">
  <header class="panel-header">
    <h2>DM Console</h2>
    <button class="refresh-btn" onclick={load}>Refresh</button>
  </header>

  {#if loading}
    <p>Loading situation...</p>
  {:else if loadError}
    <p class="error">{loadError}</p>
  {:else if situation}
    <!-- Next step banner -->
    <section class="next-step" aria-live="polite">
      {#if situation.next_step}
        <span class="next-step-label">Next step</span>
        <p class="next-step-text">{situation.next_step}</p>
      {:else}
        <p class="next-step-text calm">Nothing needs you right now.</p>
      {/if}
    </section>

    <!-- Pending worklist -->
    <section class="block">
      <h3 class="block-title">
        Pending
        <span class="count">{situation.pending.length}</span>
      </h3>
      {#if situation.pending.length === 0}
        <p class="empty">No pending actions.</p>
      {:else}
        <ul class="pending-list">
          {#each sortedPending(situation.pending) as item}
            <li class="pending-item" class:urgent={isUrgent(item)}>
              <div class="pending-head">
                <span class="pending-label">{item.label}</span>
                {#if item.player}
                  <span class="pending-player">{item.player}</span>
                {/if}
                {#if isUrgent(item)}
                  <span class="urgent-tag">urgent</span>
                {/if}
              </div>
              {#if item.summary}
                <p class="pending-summary">{item.summary}</p>
              {/if}
              {#if item.kind === 'monster_save'}
                <button
                  class="resolve-save-btn"
                  onclick={() => resolveSave(item)}
                  disabled={resolvingSaveId === item.id}
                  data-testid="resolve-save-{item.id}"
                >
                  {resolvingSaveId === item.id ? 'Resolving…' : 'Resolve'}
                </button>
              {:else if item.resolve_url}
                <a class="pending-link" href={item.resolve_url}>Resolve</a>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
      {#if saveResults.length > 0}
        <ul class="save-results" data-testid="save-results">
          {#each saveResults as result, i (i)}
            <li class="save-result" class:failure={!result.ok} data-testid="save-result-{i}">
              {result.text}
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <!-- Encounter state -->
    <section class="block">
      <h3 class="block-title">State</h3>
      {#if !situation.state.has_encounter}
        <p class="empty">No active encounter.</p>
      {:else}
        <p class="state-summary">
          Round {situation.state.round} · {situation.state.name} ({situation.state.mode})
        </p>
        {#if situation.state.combatants.length === 0}
          <p class="empty">No combatants.</p>
        {:else}
          <table class="combatant-table">
            <thead>
              <tr>
                <th>Init</th>
                <th>ID</th>
                <th>Name</th>
                <th>HP</th>
                <th>AC</th>
                <th>Pos</th>
                <th>Conditions</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {#each situation.state.combatants as c}
                <tr class:current={c.is_current} class:dead={!c.is_alive}>
                  <td class="col-init">{c.initiative}</td>
                  <td class="col-id">{c.short_id}</td>
                  <td class="col-name">
                    {c.name}
                    {#if c.is_npc}<span class="npc-tag">NPC</span>{/if}
                  </td>
                  <td class="col-hp">
                    {c.hp_current}/{c.hp_max}{#if c.temp_hp > 0}<span class="temp-hp" title="temporary HP">+{c.temp_hp}</span>{/if}
                  </td>
                  <td class="col-ac">{c.ac}</td>
                  <td class="col-pos">{c.position}</td>
                  <td class="col-conditions">
                    {#if c.conditions && c.conditions.length > 0}
                      {formatConditions(c.conditions)}
                    {:else}
                      <span class="muted">—</span>
                    {/if}
                  </td>
                  <td class="col-status">
                    {#if c.is_raging}<span class="badge rage" title="raging{c.rage_rounds_remaining ? ` — ${c.rage_rounds_remaining} rounds left` : ''}">🔥 Rage{#if c.rage_rounds_remaining}&nbsp;{c.rage_rounds_remaining}{/if}</span>{/if}
                    {#if c.concentration}<span class="badge conc" title="concentrating on {c.concentration}">C: {c.concentration}</span>{/if}
                    {#if c.exhaustion > 0}<span class="badge exh" title="exhaustion level">Exh {c.exhaustion}</span>{/if}
                    {#if c.death_saves}<span class="badge death" title="death saves (successes / failures)">{formatDeathSaves(c.death_saves)}</span>{/if}
                    {#if !c.is_raging && !c.concentration && !c.exhaustion && !c.death_saves}<span class="muted">—</span>{/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      {/if}
    </section>

    <!-- Timeline -->
    <section class="block">
      <h3 class="block-title">Timeline</h3>
      {#if situation.timeline.length === 0}
        <p class="empty">No recent events.</p>
      {:else}
        <ul class="timeline-list">
          {#each sortedTimeline(situation.timeline) as ev}
            <li class="timeline-item">
              <span class="timeline-source">{ev.source}</span>
              <span class="timeline-sep">·</span>
              <span class="timeline-actor">{ev.actor}</span>
              <span class="timeline-dash">—</span>
              <span class="timeline-summary">{ev.summary}</span>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  {/if}
</div>

<style>
  .dmconsole-panel {
    max-width: 900px;
  }

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 1rem;
  }

  .panel-header h2 {
    color: #e94560;
    margin: 0;
  }

  .refresh-btn {
    padding: 0.4rem 0.8rem;
    background: #16213e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }

  .refresh-btn:hover {
    background: #0f3460;
  }

  .error {
    color: #ff4444;
  }

  .empty {
    color: #888;
    font-style: italic;
  }

  .muted {
    color: #666;
  }

  /* Next step banner */
  .next-step {
    background: #16213e;
    border: 1px solid #e94560;
    border-radius: 6px;
    padding: 1rem 1.25rem;
    margin-bottom: 1.5rem;
  }

  .next-step-label {
    display: inline-block;
    color: #e94560;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-weight: 700;
    margin-bottom: 0.25rem;
  }

  .next-step-text {
    margin: 0;
    font-size: 1.15rem;
    color: #f7f7fb;
  }

  .next-step-text.calm {
    color: #a0a0c0;
    font-style: italic;
    font-size: 1rem;
  }

  /* Section blocks */
  .block {
    margin-bottom: 1.5rem;
  }

  .block-title {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    color: #e94560;
    margin: 0 0 0.5rem;
    font-size: 1.1rem;
  }

  .count {
    display: inline-grid;
    place-items: center;
    min-width: 1.4rem;
    height: 1.4rem;
    padding: 0 0.4rem;
    background: #0f3460;
    color: #e0e0e0;
    border-radius: 999px;
    font-size: 0.8rem;
    font-weight: 700;
  }

  /* Pending worklist */
  .pending-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .pending-item {
    background: #16213e;
    border: 1px solid #0f3460;
    border-left: 3px solid #0f3460;
    border-radius: 6px;
    padding: 0.6rem 0.85rem;
  }

  .pending-item.urgent {
    border-left-color: #e94560;
  }

  .pending-head {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
  }

  .pending-label {
    color: #e0e0e0;
    font-weight: 700;
  }

  .pending-player {
    color: #a0a0c0;
    font-size: 0.9rem;
  }

  .urgent-tag {
    background: #e94560;
    color: #fff;
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-weight: 700;
    padding: 0.1rem 0.45rem;
    border-radius: 4px;
  }

  .pending-summary {
    margin: 0.35rem 0 0;
    color: #c8c8d8;
  }

  .pending-link {
    display: inline-block;
    margin-top: 0.4rem;
    color: #e94560;
    font-weight: 700;
    text-decoration: none;
  }

  .pending-link:hover {
    text-decoration: underline;
  }

  /* Monster-save Resolve button + rolled-outcome log. */
  .resolve-save-btn {
    display: inline-block;
    margin-top: 0.4rem;
    padding: 0.25rem 0.6rem;
    background: #22c55e;
    color: #0f1a2e;
    border: none;
    border-radius: 4px;
    font-weight: 700;
    cursor: pointer;
  }

  .resolve-save-btn:hover:not(:disabled) {
    background: #16a34a;
  }

  .resolve-save-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .save-results {
    list-style: none;
    margin: 0.5rem 0 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }

  .save-result {
    padding: 0.35rem 0.6rem;
    border-radius: 4px;
    font-size: 0.9rem;
    background: rgba(34, 197, 94, 0.12);
    border-left: 3px solid #22c55e;
    color: #e0e0e0;
  }

  .save-result.failure {
    background: rgba(239, 68, 68, 0.12);
    border-left-color: #ef4444;
  }

  /* Encounter state */
  .state-summary {
    margin: 0 0 0.75rem;
    color: #e0e0e0;
    font-weight: 700;
  }

  .combatant-table {
    width: 100%;
    border-collapse: collapse;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 6px;
    overflow: hidden;
  }

  .combatant-table th,
  .combatant-table td {
    text-align: left;
    padding: 0.4rem 0.7rem;
    border-bottom: 1px solid #0f3460;
  }

  .combatant-table th {
    background: #0f3460;
    color: #e94560;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .combatant-table tr:last-child td {
    border-bottom: none;
  }

  .combatant-table tr.current td {
    background: #1c2a4a;
  }

  .combatant-table tr.current .col-name {
    color: #e94560;
    font-weight: 700;
  }

  .combatant-table tr.dead td {
    color: #777;
    text-decoration: line-through;
  }

  .col-id {
    color: #a0a0c0;
    font-family: ui-monospace, monospace;
    font-size: 0.85rem;
  }

  .col-name {
    color: #e0e0e0;
  }

  .npc-tag {
    margin-left: 0.4rem;
    background: #0f3460;
    color: #a0a0c0;
    font-size: 0.65rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-weight: 700;
    padding: 0.1rem 0.35rem;
    border-radius: 3px;
  }

  .col-conditions {
    color: #c8c8d8;
    font-size: 0.9rem;
  }

  .col-init {
    color: #a0a0c0;
    font-family: ui-monospace, monospace;
    font-size: 0.85rem;
    text-align: center;
  }

  .temp-hp {
    margin-left: 0.2rem;
    color: #4ec9a0;
    font-size: 0.8rem;
    font-weight: 700;
  }

  /* Status badges: rage / concentration / exhaustion / death saves */
  .col-status {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
  }

  .badge {
    display: inline-block;
    padding: 0.05rem 0.4rem;
    border-radius: 4px;
    font-size: 0.72rem;
    font-weight: 700;
    white-space: nowrap;
    border: 1px solid transparent;
  }

  .badge.rage {
    background: #3a1414;
    color: #ff8a5c;
    border-color: #6e2a1c;
  }

  .badge.conc {
    background: #14233a;
    color: #6cb6ff;
    border-color: #1c456e;
  }

  .badge.exh {
    background: #2e2a14;
    color: #d8c86c;
    border-color: #5a4f1c;
  }

  .badge.death {
    background: #2a142e;
    color: #e06cd0;
    border-color: #5a1c5a;
  }

  /* Timeline */
  .timeline-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }

  .timeline-item {
    padding: 0.3rem 0;
    color: #c8c8d8;
    border-bottom: 1px solid #0f3460;
  }

  .timeline-item:last-child {
    border-bottom: none;
  }

  .timeline-source {
    color: #e94560;
    font-weight: 700;
    text-transform: capitalize;
  }

  .timeline-sep,
  .timeline-dash {
    color: #666;
    margin: 0 0.3rem;
  }

  .timeline-actor {
    color: #e0e0e0;
    font-weight: 700;
  }
</style>
