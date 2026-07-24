<script>
  import { renderDiscord } from './lib/narration.js';
  import { listChannels, postToChannel } from './lib/api.js';

  let { campaignId } = $props();

  let channels = $state([]);
  let channel = $state('');
  let source = $state('');
  let error = $state('');
  let notice = $state('');
  let busy = $state(false);

  let rendered = $derived(renderDiscord(source));

  async function loadChannels() {
    if (!campaignId) return;
    error = '';
    try {
      channels = await listChannels(campaignId);
      if (!channel && channels.length > 0) {
        channel = channels.includes('in-character') ? 'in-character' : channels[0];
      }
    } catch (e) {
      error = `failed to load channels: ${e.message}`;
    }
  }

  // Reload when campaignId resolves. On a direct page-load the panel can mount
  // before App.svelte has fetched the current campaign, so a one-shot call
  // would bail early on the empty id and never retry. The effect tracks only
  // campaignId (the async body's later reads are untracked), so it re-runs once
  // the id arrives without looping on the channels/channel writes.
  $effect(() => {
    if (campaignId) loadChannels();
  });

  async function onPost() {
    error = '';
    notice = '';
    if (!channel) {
      error = 'pick a channel';
      return;
    }
    if (!source.trim()) {
      error = 'body required';
      return;
    }
    try {
      busy = true;
      const res = await postToChannel({ campaign_id: campaignId, channel, body: source });
      const n = res?.discord_message_ids?.length || 1;
      notice = `Posted to #${channel} (${n} message${n === 1 ? '' : 's'}).`;
      source = '';
    } catch (e) {
      error = `post failed: ${e.message}`;
    } finally {
      busy = false;
    }
  }
</script>

<div class="channel-panel">
  <h2>Post to Channel</h2>
  <p class="sub">
    Broadcast as the bot to any of this campaign's Discord channels. Supports
    <code>:::read-aloud</code> blocks (gold boxed text).
  </p>

  {#if !campaignId}
    <p class="empty">No active campaign selected.</p>
  {:else}
    {#if error}<div class="error">{error}</div>{/if}
    {#if notice}<div class="notice">{notice}</div>{/if}

    <div class="grid">
      <section class="editor">
        <label class="field">
          Channel
          <select bind:value={channel}>
            {#if channels.length === 0}
              <option value="" disabled>No channels found</option>
            {/if}
            {#each channels as ch (ch)}
              <option value={ch}>#{ch}</option>
            {/each}
          </select>
        </label>

        <textarea
          bind:value={source}
          placeholder={"Type a message to post as the bot...\n\nUse **bold**, *italic*, > quote.\nInsert a :::read-aloud block for boxed text."}
          rows="10"
        ></textarea>

        <button type="button" class="post-btn" disabled={busy || !channel || !source.trim()} onclick={onPost}>
          {busy ? 'Posting…' : `Post to #${channel || '…'}`}
        </button>
      </section>

      <section class="preview">
        <h3>Preview</h3>
        <div class="preview-body">
          {#if rendered.body}
            <pre>{rendered.body}</pre>
          {:else}
            <em class="empty">(empty)</em>
          {/if}
        </div>
        {#each rendered.embeds as embed}
          <div class="embed"><pre>{embed.description}</pre></div>
        {/each}
      </section>
    </div>
  {/if}
</div>

<style>
  .channel-panel { padding: 1rem; color: #e0e0e0; }
  h2, h3 { color: #e94560; }
  .sub { color: #9a9ab0; font-size: 0.85rem; margin-top: -0.25rem; }
  .sub code { background: #0f1b2d; padding: 0.05rem 0.3rem; border-radius: 3px; }
  .error { background: #e94560; color: white; padding: 0.5rem; border-radius: 4px; margin-bottom: 1rem; }
  .notice { background: #1c5c3a; color: #d6ffe6; padding: 0.5rem; border-radius: 4px; margin-bottom: 1rem; }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; }
  .field { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 0.5rem; font-size: 0.85rem; color: #9a9ab0; }
  select {
    background: #0f1b2d; color: #e0e0e0; border: 1px solid #0f3460;
    border-radius: 4px; padding: 0.5rem; font-family: inherit; font-size: 1rem;
  }
  textarea {
    width: 100%; background: #0f1b2d; color: #e0e0e0;
    border: 1px solid #0f3460; border-radius: 4px; padding: 0.75rem;
    font-family: ui-monospace, monospace; resize: vertical;
  }
  .post-btn { background: #e94560; color: white; border: none; padding: 0.75rem 1.25rem; border-radius: 4px; cursor: pointer; margin-top: 0.5rem; font-size: 1rem; }
  .post-btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .preview { background: #16213e; border: 1px solid #0f3460; border-radius: 4px; padding: 0.75rem; }
  .preview-body pre, .embed pre { white-space: pre-wrap; word-break: break-word; font-family: inherit; margin: 0; }
  .embed { border-left: 4px solid #d4af37; background: #1c2541; padding: 0.5rem 0.75rem; margin-top: 0.5rem; border-radius: 0 4px 4px 0; }
  .empty { color: #7a7a8f; }
  @media (max-width: 768px) { .grid { grid-template-columns: 1fr; } }
</style>
