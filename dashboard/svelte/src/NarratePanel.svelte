<script>
  import { renderDiscord, insertReadAloudBlock } from './lib/narration.js';
  import { extractPlaceholders, substitutePlaceholders } from './lib/narrationTemplates.js';
  import {
    uploadAsset,
    postNarration,
    listNarrationHistory,
    listNarrationTemplates,
    createNarrationTemplate,
    updateNarrationTemplate,
    deleteNarrationTemplate,
    duplicateNarrationTemplate,
  } from './lib/api.js';

  let { campaignId, authorUserId = 'dm' } = $props();

  let source = $state('');
  let attachments = $state([]); // {id, url, name}
  let history = $state([]);
  let error = $state('');
  let busy = $state(false);
  let textareaRef;

  // Template library state (Phase 100b)
  let templates = $state([]);
  let templateSearch = $state('');
  let templateCategory = $state('');
  let editingTemplate = $state(null); // {id?, name, category, body}
  let applyingTemplate = $state(null); // {template, values}

  let rendered = $derived(renderDiscord(source));
  let applyPreview = $derived(
    applyingTemplate ? substitutePlaceholders(applyingTemplate.template.body, applyingTemplate.values) : ''
  );
  let editingPlaceholders = $derived(
    editingTemplate ? extractPlaceholders(editingTemplate.body) : []
  );

  async function loadHistory() {
    try {
      history = await listNarrationHistory(campaignId);
    } catch (e) {
      error = `failed to load history: ${e.message}`;
    }
  }

  async function loadTemplates() {
    try {
      templates = await listNarrationTemplates(campaignId, {
        category: templateCategory,
        q: templateSearch,
      });
    } catch (e) {
      error = `failed to load templates: ${e.message}`;
    }
  }

  // kick off initial history + template load
  loadHistory();
  loadTemplates();

  function startNewTemplate() {
    editingTemplate = { name: '', category: '', body: source || '' };
  }

  function editTemplate(tpl) {
    editingTemplate = { id: tpl.id, name: tpl.name, category: tpl.category, body: tpl.body };
  }

  function cancelTemplateEdit() {
    editingTemplate = null;
  }

  // runTemplateOp wraps a template mutation with shared busy/error/reload
  // bookkeeping so each action stays a single line of intent.
  async function runTemplateOp(label, fn) {
    error = '';
    busy = true;
    try {
      await fn();
      await loadTemplates();
    } catch (e) {
      error = `${label} failed: ${e.message}`;
    } finally {
      busy = false;
    }
  }

  async function saveTemplate() {
    if (!editingTemplate?.name?.trim() || !editingTemplate?.body?.trim()) {
      error = 'template name and body required';
      return;
    }
    const { id, name, category, body } = editingTemplate;
    await runTemplateOp('save template', async () => {
      if (id) {
        await updateNarrationTemplate(id, { name, category, body });
      } else {
        await createNarrationTemplate({ campaign_id: campaignId, name, category, body });
      }
      editingTemplate = null;
    });
  }

  async function removeTemplate(id) {
    if (!confirm('Delete this template?')) return;
    await runTemplateOp('delete', () => deleteNarrationTemplate(id));
  }

  async function duplicate(id) {
    await runTemplateOp('duplicate', () => duplicateNarrationTemplate(id));
  }

  function startApplyTemplate(tpl) {
    const values = Object.fromEntries(extractPlaceholders(tpl.body).map((name) => [name, '']));
    applyingTemplate = { template: tpl, values };
  }

  function cancelApplyTemplate() {
    applyingTemplate = null;
  }

  function commitApplyTemplate() {
    if (!applyingTemplate) return;
    source = substitutePlaceholders(applyingTemplate.template.body, applyingTemplate.values);
    applyingTemplate = null;
  }

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

  <section class="templates">
    <div class="templates-header">
      <h3>Templates</h3>
      <button type="button" onclick={startNewTemplate}>+ New Template</button>
    </div>

    <div class="templates-filters">
      <input
        type="search"
        placeholder="Search templates..."
        bind:value={templateSearch}
        oninput={loadTemplates}
      />
      <input
        type="text"
        placeholder="Category"
        bind:value={templateCategory}
        oninput={loadTemplates}
      />
    </div>

    {#if templates.length === 0}
      <p class="empty">No templates yet.</p>
    {:else}
      <ul class="template-list">
        {#each templates as tpl (tpl.id)}
          <li>
            <div class="tpl-meta">
              <strong>{tpl.name}</strong>
              {#if tpl.category}<span class="tpl-cat">{tpl.category}</span>{/if}
            </div>
            <pre class="tpl-body">{tpl.body}</pre>
            <div class="tpl-actions">
              <button type="button" onclick={() => startApplyTemplate(tpl)}>Apply</button>
              <button type="button" onclick={() => editTemplate(tpl)}>Edit</button>
              <button type="button" onclick={() => duplicate(tpl.id)}>Duplicate</button>
              <button type="button" class="danger" onclick={() => removeTemplate(tpl.id)}>Delete</button>
            </div>
          </li>
        {/each}
      </ul>
    {/if}

    {#if editingTemplate}
      <div class="template-editor">
        <h4>{editingTemplate.id ? 'Edit Template' : 'New Template'}</h4>
        <input
          type="text"
          placeholder="Template name"
          bind:value={editingTemplate.name}
        />
        <input
          type="text"
          placeholder="Category (optional)"
          bind:value={editingTemplate.category}
        />
        <textarea
          rows="6"
          placeholder="Template body. Use {'{player_name}'} for placeholders."
          bind:value={editingTemplate.body}
        ></textarea>
        {#if editingPlaceholders.length > 0}
          <div class="tpl-tokens">
            Placeholders: {editingPlaceholders.join(', ')}
          </div>
        {/if}
        <div class="tpl-editor-actions">
          <button type="button" onclick={saveTemplate} disabled={busy}>Save</button>
          <button type="button" onclick={cancelTemplateEdit}>Cancel</button>
        </div>
      </div>
    {/if}

    {#if applyingTemplate}
      <div class="template-apply">
        <h4>Apply: {applyingTemplate.template.name}</h4>
        {#if Object.keys(applyingTemplate.values).length === 0}
          <p class="empty">No placeholders — applies as-is.</p>
        {:else}
          {#each Object.keys(applyingTemplate.values) as name (name)}
            <label>
              {name}
              <input type="text" bind:value={applyingTemplate.values[name]} />
            </label>
          {/each}
        {/if}
        <div class="apply-preview">
          <strong>Preview:</strong>
          <pre>{applyPreview}</pre>
        </div>
        <div class="tpl-editor-actions">
          <button type="button" onclick={commitApplyTemplate}>Insert into Editor</button>
          <button type="button" onclick={cancelApplyTemplate}>Cancel</button>
        </div>
      </div>
    {/if}
  </section>

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
  .templates { margin-top: 1.5rem; }
  .templates-header { display: flex; justify-content: space-between; align-items: center; }
  .templates-header button {
    background: #16213e; color: #e0e0e0; border: 1px solid #0f3460;
    padding: 0.4rem 0.75rem; border-radius: 4px; cursor: pointer;
  }
  .templates-filters { display: flex; gap: 0.5rem; margin: 0.5rem 0; }
  .templates-filters input {
    background: #0f1b2d; color: #e0e0e0; border: 1px solid #0f3460;
    border-radius: 4px; padding: 0.4rem 0.5rem; flex: 1;
  }
  .template-list { list-style: none; padding: 0; margin: 0; }
  .template-list li {
    background: #16213e; border: 1px solid #0f3460; border-radius: 4px;
    padding: 0.5rem 0.75rem; margin-bottom: 0.5rem;
  }
  .tpl-meta { display: flex; gap: 0.5rem; align-items: baseline; }
  .tpl-cat { font-size: 0.75rem; color: #d4af37; background: #1c2541; padding: 0.1rem 0.4rem; border-radius: 3px; }
  .tpl-body { white-space: pre-wrap; word-break: break-word; font-family: inherit; margin: 0.25rem 0; color: #c0c0c0; }
  .tpl-actions { display: flex; gap: 0.5rem; }
  .tpl-actions button {
    background: #0f3460; color: #e0e0e0; border: none;
    padding: 0.25rem 0.6rem; border-radius: 4px; cursor: pointer; font-size: 0.85rem;
  }
  .tpl-actions button.danger { background: #7a1f2b; }
  .template-editor, .template-apply {
    background: #1c2541; border: 1px solid #0f3460; border-radius: 4px;
    padding: 0.75rem; margin-top: 0.75rem;
  }
  .template-editor input, .template-apply input,
  .template-editor textarea {
    width: 100%; background: #0f1b2d; color: #e0e0e0;
    border: 1px solid #0f3460; border-radius: 4px; padding: 0.4rem 0.5rem;
    margin-bottom: 0.5rem; font-family: inherit;
  }
  .template-apply label { display: block; margin-bottom: 0.5rem; font-size: 0.85rem; color: #9a9ab0; }
  .tpl-tokens { font-size: 0.8rem; color: #d4af37; margin-bottom: 0.5rem; }
  .tpl-editor-actions { display: flex; gap: 0.5rem; }
  .tpl-editor-actions button {
    background: #e94560; color: white; border: none;
    padding: 0.4rem 0.75rem; border-radius: 4px; cursor: pointer;
  }
  .apply-preview { background: #0f1b2d; border-radius: 4px; padding: 0.5rem; margin: 0.5rem 0; }
  .apply-preview pre { white-space: pre-wrap; word-break: break-word; margin: 0.25rem 0 0; font-family: inherit; }
</style>
