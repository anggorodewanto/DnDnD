<script>
  // F-12: aggregate every pending dm-queue item type (whisper, freeform,
  // rest, etc.) into one dashboard panel so the DM can browse them
  // without scrolling Discord #dm-queue. The server resolves the active
  // campaign from the authenticated session, so this component does not
  // need a campaign id prop.
  import { fetchDMQueueList, iconForKind } from './lib/dmqueue.js';

  let items = $state([]);
  let loading = $state(true);
  let error = $state(null);

  async function load() {
    loading = true;
    error = null;
    try {
      items = await fetchDMQueueList();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    load();
  });
</script>

<div class="dmqueue-panel">
  <header class="panel-header">
    <h2>Pending DM Queue</h2>
    <button class="refresh-btn" onclick={load}>Refresh</button>
  </header>

  {#if loading}
    <p>Loading queue...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if items.length === 0}
    <p class="empty">No pending items. Players have nothing waiting for you.</p>
  {:else}
    <table class="queue-table">
      <thead>
        <tr>
          <th class="col-kind">Kind</th>
          <th class="col-player">Player</th>
          <th class="col-summary">Summary</th>
          <th class="col-open">Resolve</th>
        </tr>
      </thead>
      <tbody>
        {#each items as item}
          <tr>
            <td class="col-kind">
              <span class="kind-icon" aria-hidden="true">{iconForKind(item.kind)}</span>
              <span class="kind-label">{item.kind_label}</span>
            </td>
            <td class="col-player">{item.player_name}</td>
            <td class="col-summary">{item.summary}</td>
            <td class="col-open">
              <a class="open-link" href={item.resolve_path}>Open</a>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

<style>
  .dmqueue-panel {
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

  .queue-table {
    width: 100%;
    border-collapse: collapse;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 6px;
    overflow: hidden;
  }

  .queue-table th,
  .queue-table td {
    text-align: left;
    padding: 0.5rem 0.75rem;
    border-bottom: 1px solid #0f3460;
  }

  .queue-table th {
    background: #0f3460;
    color: #e94560;
    font-size: 0.85rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .queue-table tr:last-child td {
    border-bottom: none;
  }

  .kind-icon {
    display: inline-block;
    margin-right: 0.4rem;
    font-weight: bold;
    color: #e94560;
    width: 1.8rem;
  }

  .kind-label {
    color: #aaa;
    font-size: 0.9rem;
  }

  .col-summary {
    color: #e0e0e0;
  }

  .open-link {
    color: #e94560;
    text-decoration: none;
    font-weight: bold;
  }

  .open-link:hover {
    text-decoration: underline;
  }

  .empty {
    color: #888;
    font-style: italic;
  }

  .error {
    color: #ff4444;
  }
</style>
