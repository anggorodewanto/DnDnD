<script>
  let { campaignId } = $props();
  let entries = $state([]);
  let loading = $state(true);
  let error = $state(null);

  $effect(() => {
    if (campaignId) loadApprovals();
  });

  async function loadApprovals() {
    loading = true;
    error = null;
    try {
      const res = await fetch('/dashboard/api/approvals/', { credentials: 'same-origin' });
      if (!res.ok) throw new Error(`${res.status}`);
      entries = await res.json();
    } catch (e) {
      error = e.message;
      entries = [];
    } finally {
      loading = false;
    }
  }

  async function approve(id) {
    await fetch(`/dashboard/api/approvals/${id}/approve`, { method: 'POST', credentials: 'same-origin' });
    await loadApprovals();
  }

  async function reject(id) {
    const feedback = prompt('Rejection reason:');
    if (!feedback) return;
    await fetch(`/dashboard/api/approvals/${id}/reject`, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ feedback }),
    });
    await loadApprovals();
  }

  async function requestChanges(id) {
    const feedback = prompt('What changes are needed?');
    if (!feedback) return;
    await fetch(`/dashboard/api/approvals/${id}/request-changes`, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ feedback }),
    });
    await loadApprovals();
  }
</script>

<div class="approval-queue" data-testid="character-approval-queue">
  <h2>Character Approval Queue</h2>
  {#if loading}
    <p class="placeholder">Loading…</p>
  {:else if error}
    <p class="error">Failed to load approvals.</p>
  {:else if entries.length === 0}
    <p class="placeholder">No pending character approvals.</p>
  {:else}
    <ul class="approval-list">
      {#each entries as entry}
        <li class="approval-entry" data-id={entry.id}>
          <span class="char-name">{entry.character_name}</span>
          <span class="status">{entry.status}</span>
          <div class="actions">
            <button onclick={() => approve(entry.id)}>Approve</button>
            <button onclick={() => requestChanges(entry.id)}>Request Changes</button>
            <button onclick={() => reject(entry.id)}>Reject</button>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .approval-queue { padding: 0.5rem; }
  .approval-list { list-style: none; padding: 0; margin: 0; }
  .approval-entry {
    display: flex; flex-wrap: wrap; align-items: center; gap: 0.5rem;
    padding: 0.75rem; margin-bottom: 0.5rem;
    background: #16213e; border-radius: 4px; border: 1px solid #0f3460;
  }
  .char-name { font-weight: bold; color: #e0e0e0; }
  .status { color: #a0aec0; font-size: 0.85rem; }
  .actions { margin-left: auto; display: flex; gap: 0.25rem; }
  .actions button {
    padding: 0.25rem 0.5rem; font-size: 0.75rem; border-radius: 3px;
    border: 1px solid #0f3460; background: #1a1a2e; color: #e0e0e0; cursor: pointer;
  }
  .placeholder { color: #a0aec0; font-style: italic; }
  .error { color: #e94560; }
</style>
