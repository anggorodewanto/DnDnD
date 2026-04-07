<script>
  import { renderDiscord, insertReadAloudBlock } from './lib/narration.js';
  import {
    uploadAsset,
    postNarration,
    listNarrationHistory,
  } from './lib/api.js';

  let { campaignId, authorUserId = 'dm' } = $props();

  let source = $state('');
  let attachments = $state([]); // {id, url, name}
  let history = $state([]);
  let error = $state('');
  let busy = $state(false);
  let textareaRef;

  let rendered = $derived(renderDiscord(source));

  async function loadHistory() {
    try {
      history = await listNarrationHistory(campaignId);
    } catch (e) {
      error = `failed to load history: ${e.message}`;
    }
  }

  // kick off initial history load
  loadHistory();

  function insertBlock() {
    const caret = textareaRef?.selectionStart ?? source.length;
    const next = insertReadAloudBlock(source, caret);
    source = next.text;
    // Restore focus + caret after DOM updates.
    queueMicrotask(() => {
      if (textareaRef) {
        textareaRef.focus();
        textareaRef.setSelectionRange(next.caret, next.caret);
      }
    });
  }

  async function onAttach(event) {
    const file = event.target.files?.[0];
    if (!file) return;
    try {
      busy = true;
      const asset = await uploadAsset({ campaignId, type: 'narration', file });
      attachments = [...attachments, { id: asset.id, url: asset.url, name: file.name }];
      event.target.value = '';
    } catch (e) {
      error = `upload failed: ${e.message}`;
    } finally {
      busy = false;
    }
  }

  function removeAttachment(id) {
    attachments = attachments.filter((a) => a.id !== id);
  }

  async function onPost() {
    error = '';
    if (!source.trim()) {
      error = 'body required';
      return;
    }
    try {
      busy = true;
      await postNarration({
        campaign_id: campaignId,
        author_user_id: authorUserId,
        body: source,
        attachment_asset_ids: attachments.map((a) => a.id),
      });
      source = '';
      attachments = [];
      await loadHistory();
    } catch (e) {
      error = `post failed: ${e.message}`;
    } finally {
      busy = false;
    }
  }
</script>

<div class="narrate-panel">
  <h2>Narrate Panel</h2>

  {#if error}
    <div class="error">{error}</div>
  {/if}

  <div class="grid">
    <section class="editor">
      <div class="toolbar">
        <button type="button" onclick={insertBlock}>Insert Read-Aloud Block</button>
        <label class="file-btn">
          Attach Image
          <input type="file" accept="image/*" onchange={onAttach} />
        </label>
      </div>

      <textarea
        bind:this={textareaRef}
        bind:value={source}
        placeholder={"Narrate the next scene...\n\nUse **bold**, *italic*, > quote.\nInsert a :::read-aloud block for boxed text."}
        rows="12"
      ></textarea>

      {#if attachments.length > 0}
        <ul class="attachments">
          {#each attachments as att (att.id)}
            <li>
              <img src={att.url} alt={att.name} />
              <span>{att.name}</span>
              <button type="button" onclick={() => removeAttachment(att.id)}>×</button>
            </li>
          {/each}
        </ul>
      {/if}

      <button type="button" class="post-btn" disabled={busy || !source.trim()} onclick={onPost}>
        Post to #the-story
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
        <div class="embed">
          <pre>{embed.description}</pre>
        </div>
      {/each}
      {#if attachments.length > 0}
        <div class="embed-attachments">
          {#each attachments as att (att.id)}
            <img src={att.url} alt={att.name} />
          {/each}
        </div>
      {/if}
    </section>
  </div>

  <section class="history">
    <h3>Post History</h3>
    {#if history.length === 0}
      <p class="empty">No narrations posted yet.</p>
    {:else}
      <ul>
        {#each history as post (post.id)}
          <li>
            <div class="meta">{new Date(post.posted_at).toLocaleString()} — {post.author_user_id}</div>
            <pre>{post.body}</pre>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</div>

<style>
  .narrate-panel { padding: 1rem; color: #e0e0e0; }
  h2, h3 { color: #e94560; }
  .error { background: #e94560; color: white; padding: 0.5rem; border-radius: 4px; margin-bottom: 1rem; }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; }
  .toolbar { display: flex; gap: 0.5rem; margin-bottom: 0.5rem; }
  .toolbar button, .file-btn {
    background: #16213e; color: #e0e0e0; border: 1px solid #0f3460;
    padding: 0.5rem 0.75rem; border-radius: 4px; cursor: pointer;
  }
  .file-btn input { display: none; }
  textarea {
    width: 100%; background: #0f1b2d; color: #e0e0e0;
    border: 1px solid #0f3460; border-radius: 4px; padding: 0.75rem;
    font-family: ui-monospace, monospace; resize: vertical;
  }
  .attachments { list-style: none; padding: 0; margin: 0.5rem 0; display: flex; flex-wrap: wrap; gap: 0.5rem; }
  .attachments li { background: #16213e; padding: 0.25rem; border-radius: 4px; display: flex; align-items: center; gap: 0.5rem; }
  .attachments img { max-width: 60px; max-height: 60px; }
  .attachments button { background: none; border: none; color: #e94560; cursor: pointer; font-size: 1.1rem; }
  .post-btn { background: #e94560; color: white; border: none; padding: 0.75rem 1.25rem; border-radius: 4px; cursor: pointer; margin-top: 0.5rem; font-size: 1rem; }
  .post-btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .preview { background: #16213e; border: 1px solid #0f3460; border-radius: 4px; padding: 0.75rem; }
  .preview-body pre, .embed pre, .history pre { white-space: pre-wrap; word-break: break-word; font-family: inherit; margin: 0; }
  .embed { border-left: 4px solid #d4af37; background: #1c2541; padding: 0.5rem 0.75rem; margin-top: 0.5rem; border-radius: 0 4px 4px 0; }
  .embed-attachments { display: flex; gap: 0.5rem; flex-wrap: wrap; margin-top: 0.5rem; }
  .embed-attachments img { max-width: 160px; border-radius: 4px; }
  .empty { color: #7a7a8f; }
  .history { margin-top: 1.5rem; }
  .history ul { list-style: none; padding: 0; }
  .history li { background: #16213e; border: 1px solid #0f3460; border-radius: 4px; padding: 0.5rem 0.75rem; margin-bottom: 0.5rem; }
  .history .meta { font-size: 0.8rem; color: #9a9ab0; margin-bottom: 0.25rem; }
</style>
