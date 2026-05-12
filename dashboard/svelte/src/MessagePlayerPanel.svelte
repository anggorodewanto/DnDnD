<script>
  // Phase 101 (G-101): Message Player panel — character picker + history.
  //
  // Previously a send-only form requiring a manual UUID paste. Now:
  //   - The player target comes from a dropdown driven by
  //     `/api/character-overview` (campaign-scoped party list).
  //   - The panel renders the message history for the selected player via
  //     `/api/message-player/history`.
  //   - The panel can be embedded inside `CharacterOverview.svelte` with
  //     `playerCharacterId` preselected (the embed hides the dropdown).
  import {
    validateMessagePlayerInput,
    sendPlayerMessage,
    fetchHistory,
    fetchPartyCharacters,
  } from './lib/messageplayer.js';

  let {
    campaignId,
    authorUserId = 'dm',
    playerCharacterId: preselectedPlayerId = '',
    playerName = '',
    hidePicker = false,
  } = $props();

  let characters = $state([]);
  let charactersError = $state(null);

  // Initialize via untrack-style read of the prop so Svelte 5 doesn't warn
  // about coupling reactive state to a top-level prop reference. The picker
  // updates this in-place once a player is selected.
  let playerCharacterId = $state('');
  $effect(() => {
    if (preselectedPlayerId && !playerCharacterId) {
      playerCharacterId = preselectedPlayerId;
    }
  });
  let body = $state('');
  let sending = $state(false);
  let error = $state(null);
  let success = $state(null);

  let history = $state([]);
  let historyLoading = $state(false);
  let historyError = $state(null);

  // Load the campaign roster on mount so the picker has options.
  $effect(() => {
    if (!campaignId || hidePicker) return;
    fetchPartyCharacters(campaignId)
      .then((data) => {
        characters = data.characters || [];
      })
      .catch((e) => {
        charactersError = e.message;
      });
  });

  // Re-load history whenever the selection or campaign changes.
  $effect(() => {
    if (!campaignId || !playerCharacterId) {
      history = [];
      return;
    }
    historyLoading = true;
    historyError = null;
    fetchHistory({ campaignId, playerCharacterId })
      .then((msgs) => {
        history = Array.isArray(msgs) ? msgs : [];
      })
      .catch((e) => {
        historyError = e.message;
        history = [];
      })
      .finally(() => {
        historyLoading = false;
      });
  });

  async function handleSend() {
    error = null;
    success = null;
    const check = validateMessagePlayerInput({
      campaignId,
      playerCharacterId,
      authorUserId,
      body,
    });
    if (!check.ok) {
      error = check.error;
      return;
    }
    sending = true;
    try {
      await sendPlayerMessage({ campaignId, playerCharacterId, authorUserId, body });
      success = 'Message sent.';
      body = '';
      // Refresh history so the just-sent message shows up.
      try {
        const msgs = await fetchHistory({ campaignId, playerCharacterId });
        history = Array.isArray(msgs) ? msgs : [];
      } catch (_) {
        /* swallow; the send succeeded */
      }
    } catch (e) {
      error = e.message;
    } finally {
      sending = false;
    }
  }

  function formatTimestamp(ts) {
    if (!ts) return '';
    try {
      return new Date(ts).toLocaleString();
    } catch (_) {
      return String(ts);
    }
  }
</script>

<div class="msg-panel" data-testid="message-player-panel">
  <h3>Message Player{playerName ? ` — ${playerName}` : ''}</h3>

  {#if !hidePicker}
    <label>
      Player
      <select bind:value={playerCharacterId} data-testid="message-player-picker">
        <option value="">Select player...</option>
        {#each characters as c (c.character_id)}
          <option value={c.character_id}>{c.name}</option>
        {/each}
      </select>
    </label>
    {#if charactersError}
      <p class="error">{charactersError}</p>
    {/if}
  {/if}

  <label>
    Message
    <textarea bind:value={body} rows="4"></textarea>
  </label>
  <button onclick={handleSend} disabled={sending || !playerCharacterId}>
    {sending ? 'Sending...' : 'Send DM'}
  </button>
  {#if error}<p class="error">{error}</p>{/if}
  {#if success}<p class="success">{success}</p>{/if}

  <section class="history" data-testid="message-player-history">
    <h4>History</h4>
    {#if !playerCharacterId}
      <p class="muted">Select a player to view past messages.</p>
    {:else if historyLoading}
      <p class="muted">Loading history...</p>
    {:else if historyError}
      <p class="error">Failed to load history: {historyError}</p>
    {:else if history.length === 0}
      <p class="muted">No prior messages.</p>
    {:else}
      <ul>
        {#each history as m (m.id)}
          <li>
            <div class="meta">
              <span class="author">{m.author_user_id || 'dm'}</span>
              <span class="ts">{formatTimestamp(m.created_at)}</span>
            </div>
            <div class="body">{m.body}</div>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</div>

<style>
  .msg-panel {
    padding: 1rem;
    background: #16213e;
    border-radius: 4px;
    border: 1px solid #0f3460;
  }
  .msg-panel h3 { margin: 0 0 0.5rem 0; color: #e94560; }
  .msg-panel h4 { margin: 0 0 0.4rem 0; color: #e0e0e0; font-size: 0.95rem; }
  label { display: block; margin-bottom: 0.5rem; font-size: 0.85rem; color: #a0aec0; }
  textarea, select {
    width: 100%;
    padding: 0.5rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    box-sizing: border-box;
    margin-top: 0.25rem;
  }
  button {
    width: 100%;
    padding: 0.6rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-weight: bold;
  }
  button:disabled { opacity: 0.5; cursor: not-allowed; }
  .error { color: #ef4444; font-size: 0.85rem; }
  .success { color: #10b981; font-size: 0.85rem; }
  .muted { color: #a0aec0; font-size: 0.85rem; font-style: italic; }
  .history {
    margin-top: 1rem;
    border-top: 1px solid #0f3460;
    padding-top: 0.75rem;
  }
  .history ul {
    list-style: none;
    padding: 0;
    margin: 0;
    max-height: 18rem;
    overflow-y: auto;
  }
  .history li {
    background: #1a1a2e;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 0.5rem 0.6rem;
    margin-bottom: 0.4rem;
  }
  .history .meta {
    display: flex;
    justify-content: space-between;
    color: #a0aec0;
    font-size: 0.75rem;
    margin-bottom: 0.2rem;
  }
  .history .body {
    white-space: pre-wrap;
    color: #e0e0e0;
    font-size: 0.9rem;
  }
</style>
