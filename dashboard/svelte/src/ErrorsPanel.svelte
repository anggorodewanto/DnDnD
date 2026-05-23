<script>
  // Errors panel: reads the recent 24h error list from GET /api/errors
  // and renders it in a sortable-by-time table. The endpoint already
  // filters to the 24h window so the panel does not need to re-check
  // timestamps.
  import { fetchRecentErrors, formatErrorTimestamp } from './lib/errors.js';

  let { campaignId = '' } = $props();
  // campaignId is accepted for shell-uniformity but unused: the server
  // resolves the DM session and returns errors for the whole deploy.
  void campaignId;

  let entries = $state([]);
  let loading = $state(true);
  let error = $state(null);

  async function load() {
    loading = true;
    error = null;
    try {
      entries = await fetchRecentErrors();
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

<div class="errors-panel">
  <header class="panel-header">
    <h2>Errors (last 24h)</h2>
    <button class="refresh-btn" onclick={load}>Refresh</button>
  </header>

  {#if loading}
    <p class="status">Loading errors...</p>
  {:else if error}
    <p class="error">Failed to load errors: {error}</p>
  {:else if entries.length === 0}
    <p class="empty">No errors logged in the last 24 hours.</p>
  {:else}
    <table class="errors-table">
      <thead>
        <tr>
          <th class="col-time">Timestamp</th>
          <th class="col-command">Command</th>
          <th class="col-user">Player</th>
          <th class="col-summary">Summary</th>
        </tr>
      </thead>
      <tbody>
        {#each entries as entry}
          <tr>
            <td class="col-time">{formatErrorTimestamp(entry.timestamp)}</td>
            <td class="col-command">{entry.command ? `/${entry.command}` : '—'}</td>
            <td class="col-user">{entry.user_id ? `@${entry.user_id}` : '—'}</td>
            <td class="col-summary">{entry.summary}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

<style>
  .errors-panel {
    max-width: 1100px;
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

  .status {
    color: #a0a0c0;
  }

  .error {
    color: #ff4444;
  }

  .empty {
    color: #888;
    font-style: italic;
  }

  .errors-table {
    width: 100%;
    border-collapse: collapse;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 6px;
    overflow: hidden;
  }

  .errors-table th,
  .errors-table td {
    text-align: left;
    padding: 0.5rem 0.75rem;
    border-bottom: 1px solid #0f3460;
    vertical-align: top;
  }

  .errors-table th {
    background: #0f3460;
    color: #e94560;
    font-size: 0.85rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .errors-table tr:last-child td {
    border-bottom: none;
  }

  .col-time {
    white-space: nowrap;
    color: #a0a0c0;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 0.85rem;
  }

  .col-command,
  .col-user {
    white-space: nowrap;
    color: #cbd5e1;
  }

  .col-summary {
    color: #e0e0e0;
  }
</style>
