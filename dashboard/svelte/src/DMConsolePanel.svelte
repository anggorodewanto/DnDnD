<script>
  // DM Console: the "start here" overview that aggregates everything a DM
  // needs at a glance into one read-only panel. It hits a single endpoint
  // (GET /api/dm/situation) and renders, top to bottom: the single suggested
  // next step, the unified pending worklist (queue + approvals + level-ups),
  // the live encounter state, and the recent timeline. The server resolves the
  // active campaign from the authenticated session, so no campaign id prop.
  import { fetchDMSituation } from './lib/dmsituation.js';

  let situation = $state(null);
  let loading = $state(true);
  let loadError = $state(null);

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
              {#if item.resolve_url}
                <a class="pending-link" href={item.resolve_url}>Resolve</a>
              {/if}
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
                <th>ID</th>
                <th>Name</th>
                <th>HP</th>
                <th>AC</th>
                <th>Pos</th>
                <th>Conditions</th>
              </tr>
            </thead>
            <tbody>
              {#each situation.state.combatants as c}
                <tr class:current={c.is_current} class:dead={!c.is_alive}>
                  <td class="col-id">{c.short_id}</td>
                  <td class="col-name">
                    {c.name}
                    {#if c.is_npc}<span class="npc-tag">NPC</span>{/if}
                  </td>
                  <td class="col-hp">{c.hp_current}/{c.hp_max}</td>
                  <td class="col-ac">{c.ac}</td>
                  <td class="col-pos">{c.position}</td>
                  <td class="col-conditions">
                    {#if c.conditions && c.conditions.length > 0}
                      {c.conditions.join(', ')}
                    {:else}
                      <span class="muted">—</span>
                    {/if}
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
