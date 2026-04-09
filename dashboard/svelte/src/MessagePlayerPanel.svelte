<script>
  import {
    validateMessagePlayerInput,
    sendPlayerMessage,
  } from './lib/messageplayer.js';

  let { campaignId, authorUserId = 'dm' } = $props();

  let playerCharacterId = $state('');
  let body = $state('');
  let sending = $state(false);
  let error = $state(null);
  let success = $state(null);

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
    } catch (e) {
      error = e.message;
    } finally {
      sending = false;
    }
  }
</script>

<div class="msg-panel" data-testid="message-player-panel">
  <h3>Message Player</h3>
  <label>
    Player character ID
    <input type="text" bind:value={playerCharacterId} placeholder="uuid" />
  </label>
  <label>
    Message
    <textarea bind:value={body} rows="4"></textarea>
  </label>
  <button onclick={handleSend} disabled={sending}>
    {sending ? 'Sending...' : 'Send DM'}
  </button>
  {#if error}<p class="error">{error}</p>{/if}
  {#if success}<p class="success">{success}</p>{/if}
</div>

<style>
  .msg-panel {
    padding: 1rem;
    background: #16213e;
    border-radius: 4px;
    border: 1px solid #0f3460;
  }
  .msg-panel h3 { margin: 0 0 0.5rem 0; color: #e94560; }
  label { display: block; margin-bottom: 0.5rem; font-size: 0.85rem; color: #a0aec0; }
  input, textarea {
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
</style>
