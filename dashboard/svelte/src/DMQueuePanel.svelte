<script>
  // F-12 dashboard aggregator: every pending dm-queue item type (whisper,
  // freeform, rest, etc.) listed in one panel so the DM can browse them
  // without scrolling Discord #dm-queue. Selecting an entry opens an
  // inline detail view that posts the JSON resolve/reply/narrate payloads.
  // The server resolves the active campaign from the authenticated session,
  // so this component does not need a campaign id prop.
  import {
    fetchDMQueueList,
    fetchDMQueueItem,
    resolveItem,
    replyItem,
    narrateItem,
    iconForKind,
  } from './lib/dmqueue.js';

  let items = $state([]);
  let loading = $state(true);
  let listError = $state(null);

  let selectedID = $state(null);
  let detail = $state(null);
  let detailLoading = $state(false);
  let detailError = $state(null);

  let inputValue = $state('');
  let submitting = $state(false);
  let submitError = $state(null);

  async function load() {
    loading = true;
    listError = null;
    try {
      items = await fetchDMQueueList();
    } catch (e) {
      listError = e.message;
    } finally {
      loading = false;
    }
  }

  async function openItem(id) {
    selectedID = id;
    detail = null;
    detailError = null;
    submitError = null;
    inputValue = '';
    detailLoading = true;
    try {
      detail = await fetchDMQueueItem(id);
    } catch (e) {
      detailError = e.message;
    } finally {
      detailLoading = false;
    }
  }

  function closeDetail() {
    selectedID = null;
    detail = null;
    detailError = null;
    submitError = null;
    inputValue = '';
  }

  async function submitResolution() {
    if (!detail || !detail.is_pending) return;
    const trimmed = inputValue.trim();
    if (!trimmed) {
      submitError = 'value is required';
      return;
    }
    submitting = true;
    submitError = null;
    try {
      if (detail.is_whisper) {
        await replyItem(detail.id, trimmed);
      } else if (detail.is_skill_check_narration) {
        await narrateItem(detail.id, trimmed);
      } else {
        await resolveItem(detail.id, trimmed);
      }
      // Refresh detail to surface the new outcome/status, then reload
      // the list so the resolved entry drops off.
      detail = await fetchDMQueueItem(detail.id);
      inputValue = '';
      await load();
    } catch (e) {
      submitError = e.message;
    } finally {
      submitting = false;
    }
  }

  function inputLabel(d) {
    if (d.is_whisper) return 'Reply (sent as Discord DM)';
    if (d.is_skill_check_narration) return 'Narration (posted to channel)';
    return 'Outcome summary';
  }

  function inputPlaceholder(d) {
    if (d.is_whisper) return "e.g. You catch the merchant's gaze mid-pull...";
    if (d.is_skill_check_narration) return 'e.g. You spot the trap before stepping on it.';
    return 'e.g. table is flipped, enemies prone';
  }

  function submitLabel(d) {
    if (d.is_whisper) return 'Send Reply';
    if (d.is_skill_check_narration) return 'Send Narration';
    return 'Resolve';
  }

  function statusClass(status) {
    if (status === 'pending') return 'status pending';
    if (status === 'resolved') return 'status resolved';
    if (status === 'cancelled') return 'status cancelled';
    return 'status';
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
  {:else if listError}
    <p class="error">{listError}</p>
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
          <tr class:selected={item.id === selectedID}>
            <td class="col-kind">
              <span class="kind-icon" aria-hidden="true">{iconForKind(item.kind)}</span>
              <span class="kind-label">{item.kind_label}</span>
            </td>
            <td class="col-player">{item.player_name}</td>
            <td class="col-summary">{item.summary}</td>
            <td class="col-open">
              <button class="open-link" type="button" onclick={() => openItem(item.id)}>Open</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}

  {#if selectedID}
    <section class="detail" aria-live="polite">
      {#if detailLoading}
        <p>Loading item...</p>
      {:else if detailError}
        <p class="error">{detailError}</p>
        <button class="back-btn" type="button" onclick={closeDetail}>Close</button>
      {:else if detail}
        <header class="detail-header">
          <h3>{detail.kind_label}</h3>
          <button class="back-btn" type="button" onclick={closeDetail}>Close</button>
        </header>
        <div class="meta">
          <span class="player-name">{detail.player_name}</span>
          <span class={statusClass(detail.status)}>{detail.status}</span>
        </div>
        <p class="summary-text">{detail.summary}</p>

        {#if detail.is_pending}
          <form
            class="resolve-form"
            onsubmit={(e) => {
              e.preventDefault();
              submitResolution();
            }}
          >
            <label for="dmqueue-input">{inputLabel(detail)}</label>
            <input
              id="dmqueue-input"
              type="text"
              bind:value={inputValue}
              placeholder={inputPlaceholder(detail)}
              required
              disabled={submitting}
            />
            <button type="submit" class="submit-btn" disabled={submitting}>
              {submitting ? 'Sending...' : submitLabel(detail)}
            </button>
            {#if submitError}
              <p class="error">{submitError}</p>
            {/if}
          </form>
        {:else if detail.is_resolved}
          <p class="outcome">Outcome: {detail.outcome}</p>
        {:else if detail.is_cancelled}
          <p class="outcome">This item was cancelled.</p>
        {/if}
      {/if}
    </section>
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

  .queue-table tr.selected td {
    background: #1c2a4a;
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
    background: transparent;
    border: none;
    padding: 0;
    font-weight: bold;
    cursor: pointer;
    font: inherit;
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

  .detail {
    margin-top: 1.5rem;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 6px;
    padding: 1.25rem;
  }

  .detail-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 0.5rem;
  }

  .detail-header h3 {
    color: #e94560;
    margin: 0;
  }

  .back-btn {
    padding: 0.3rem 0.7rem;
    background: transparent;
    color: #a0a0c0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }

  .back-btn:hover {
    background: #0f3460;
    color: #e0e0e0;
  }

  .meta {
    display: flex;
    gap: 0.75rem;
    align-items: center;
    margin-bottom: 0.75rem;
    color: #a0a0c0;
  }

  .player-name {
    color: #e0e0e0;
    font-weight: bold;
  }

  .status {
    display: inline-block;
    padding: 0.2rem 0.6rem;
    border-radius: 4px;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-weight: bold;
  }

  .status.pending {
    background: #e94560;
    color: white;
  }

  .status.resolved {
    background: #2d6a4f;
    color: white;
  }

  .status.cancelled {
    background: #555;
    color: white;
  }

  .summary-text {
    font-size: 1.05rem;
    margin: 0.5rem 0 1rem;
  }

  .resolve-form {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .resolve-form label {
    color: #a0a0c0;
    font-size: 0.9rem;
  }

  .resolve-form input[type='text'] {
    padding: 0.5rem;
    background: #0f3460;
    color: #e0e0e0;
    border: 1px solid #e94560;
    border-radius: 4px;
  }

  .submit-btn {
    align-self: flex-start;
    padding: 0.5rem 1rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }

  .submit-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .outcome {
    margin-top: 0.75rem;
    font-style: italic;
    color: #c8c8d8;
  }
</style>
