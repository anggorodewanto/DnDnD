<script>
  // F-8 / Phase 111 — Open5e source catalog + per-campaign toggle.
  //
  // Two concerns live here:
  //   1. Global catalog management (admin): add/remove *custom* Open5e
  //      document slugs that extend the 11 built-in books. Custom sources
  //      join the catalog for every campaign. (POST/DELETE /api/open5e/sources)
  //   2. Per-campaign enable: the DM checks which catalog slugs the current
  //      campaign trusts; the backend persists the list to
  //      `campaigns.settings.open5e_sources` JSONB. Without an enabled slug,
  //      every open5e:* cached row is hidden by the per-campaign filter
  //      (`internal/open5e/filter.go` + `refdata_adapter.go`), so flipping
  //      checkboxes here directly controls what shows up in spell lists / stat
  //      block library / character builder.
  //
  // The catalog is fetched from `/api/open5e/sources` — the backend stays the
  // single source of truth for slug→title mapping so we don't duplicate the
  // list in two places.
  import {
    listOpen5eSources,
    getCampaignOpen5eSources,
    updateCampaignOpen5eSources,
    addOpen5eSource,
    deleteOpen5eSource,
  } from './lib/api.js';

  let { campaignId } = $props();

  let catalog = $state([]);
  let enabled = $state(new Set());
  let loading = $state(true);
  let saving = $state(false);
  let error = $state('');
  let savedAt = $state(null);

  // Add-source form + catalog-management state.
  let newSlug = $state('');
  let newTitle = $state('');
  let newPublisher = $state('');
  let newDescription = $state('');
  let adding = $state(false);
  let manageError = $state('');
  let manageNotice = $state('');
  let pendingDelete = $state(null);

  $effect(() => {
    load();
  });

  async function load() {
    error = '';
    loading = true;
    try {
      const [cat, current] = await Promise.all([
        listOpen5eSources(),
        campaignId ? getCampaignOpen5eSources(campaignId) : Promise.resolve({ enabled: [] }),
      ]);
      catalog = cat.sources || [];
      enabled = new Set(current.enabled || []);
    } catch (e) {
      error = e.message || 'Failed to load Open5e sources.';
    } finally {
      loading = false;
    }
  }

  async function toggle(slug) {
    if (!campaignId) {
      error = 'No active campaign — cannot toggle sources.';
      return;
    }
    // Optimistic local flip; revert on backend failure.
    const next = new Set(enabled);
    if (next.has(slug)) {
      next.delete(slug);
    } else {
      next.add(slug);
    }
    enabled = next;
    await persist();
  }

  async function persist() {
    saving = true;
    error = '';
    try {
      const list = Array.from(enabled);
      const resp = await updateCampaignOpen5eSources(campaignId, list);
      // Trust the backend's canonical, validated list (de-duped + order
      // matches input). This keeps the UI in lock-step with the JSONB.
      enabled = new Set(resp.enabled || []);
      savedAt = new Date();
    } catch (e) {
      error = e.message || 'Failed to update sources.';
      // Roll back to the persisted state.
      try {
        const current = await getCampaignOpen5eSources(campaignId);
        enabled = new Set(current.enabled || []);
      } catch {
        /* leave optimistic state — user can retry */
      }
    } finally {
      saving = false;
    }
  }

  function disableAll() {
    enabled = new Set();
    persist();
  }

  function clearManageMessages() {
    manageError = '';
    manageNotice = '';
  }

  function clearForm() {
    newSlug = '';
    newTitle = '';
    newPublisher = '';
    newDescription = '';
  }

  async function addSource(e) {
    e.preventDefault();
    clearManageMessages();
    const slug = newSlug.trim();
    const title = newTitle.trim();
    if (!slug || !title) {
      manageError = 'Slug and title are both required.';
      return;
    }
    adding = true;
    try {
      const created = await addOpen5eSource({
        slug,
        title,
        publisher: newPublisher.trim(),
        description: newDescription.trim(),
      });
      manageNotice = `Added "${created.title}" — tick its checkbox below to enable it.`;
      clearForm();
      await load();
    } catch (err) {
      manageError = err.message || 'Failed to add source.';
    } finally {
      adding = false;
    }
  }

  async function removeSource(slug) {
    clearManageMessages();
    pendingDelete = null;
    try {
      await deleteOpen5eSource(slug);
      // Server keeps a stale slug in any campaign that still lists it (it
      // simply matches no content), but the local UI shouldn't show a
      // just-deleted source as enabled.
      if (enabled.has(slug)) {
        const next = new Set(enabled);
        next.delete(slug);
        enabled = next;
      }
      await load();
    } catch (err) {
      manageError = err.message || 'Failed to delete source.';
    }
  }
</script>

<section class="open5e-sources-panel">
  <header>
    <h2>Open5e Sources</h2>
    <p class="hint">
      Enable Open5e third-party books for this campaign. Disabled books are
      hidden from spell pickers, the stat block library, and character builder.
    </p>
  </header>

  <!-- Global catalog management — available regardless of active campaign. -->
  <div class="manage">
    <details class="howto">
      <summary>How to add a source</summary>
      <ol>
        <li>
          Find the book on
          <a href="https://open5e.com" target="_blank" rel="noopener">open5e.com</a>
          (or api.open5e.com). Its <strong>slug</strong> is the
          <code>document__slug</code> value — lowercase and hyphenated, e.g.
          <code>tome-of-beasts</code>.
        </li>
        <li>Enter the slug and a title below (publisher and description are optional), then click <strong>Add source</strong>.</li>
        <li>The book appears in the list below — tick its checkbox to enable it for this campaign.</li>
        <li>
          Note: a source only shows real content once its monsters/spells have
          been imported from Open5e. Adding the slug makes the book
          <em>selectable</em>; it does not fetch content by itself.
        </li>
      </ol>
    </details>

    <form class="add-form" onsubmit={addSource}>
      <div class="fields">
        <input
          type="text"
          placeholder="slug (e.g. warlock-grimoire)"
          bind:value={newSlug}
          disabled={adding}
          aria-label="Source slug"
        />
        <input
          type="text"
          placeholder="Title"
          bind:value={newTitle}
          disabled={adding}
          aria-label="Source title"
        />
        <input
          type="text"
          placeholder="Publisher (optional)"
          bind:value={newPublisher}
          disabled={adding}
          aria-label="Source publisher"
        />
        <input
          type="text"
          placeholder="Description (optional)"
          bind:value={newDescription}
          disabled={adding}
          aria-label="Source description"
        />
      </div>
      <button type="submit" disabled={adding}>
        {adding ? 'Adding...' : 'Add source'}
      </button>
    </form>

    {#if manageError}
      <p class="error" role="alert">{manageError}</p>
    {/if}
    {#if manageNotice}
      <p class="notice">{manageNotice}</p>
    {/if}
  </div>

  {#if loading}
    <p>Loading Open5e sources...</p>
  {:else}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
    {#if !campaignId}
      <p class="empty">No active campaign — checkboxes are read-only. You can still add or remove catalog sources above.</p>
    {/if}

    <ul class="source-list">
      {#each catalog as src (src.slug)}
        <li>
          <label>
            <input
              type="checkbox"
              checked={enabled.has(src.slug)}
              disabled={saving || !campaignId}
              onchange={() => toggle(src.slug)}
            />
            <span class="title">{src.title}</span>
            {#if src.publisher}<span class="publisher">— {src.publisher}</span>{/if}
            {#if src.description}<span class="description">{src.description}</span>{/if}
            <span class="slug">{src.slug}</span>
          </label>
          {#if !src.builtin}
            <div class="row-actions">
              {#if pendingDelete === src.slug}
                <button type="button" class="confirm-del" onclick={() => removeSource(src.slug)}>Remove?</button>
                <button type="button" class="cancel-del" onclick={() => (pendingDelete = null)}>Cancel</button>
              {:else}
                <button
                  type="button"
                  class="del"
                  title="Remove this custom source from the catalog"
                  aria-label="Remove {src.title}"
                  onclick={() => (pendingDelete = src.slug)}
                >✕</button>
              {/if}
            </div>
          {:else}
            <span class="builtin-badge" title="Built-in source — cannot be removed">built-in</span>
          {/if}
        </li>
      {/each}
    </ul>

    <footer class="actions">
      <button type="button" onclick={disableAll} disabled={saving || !campaignId || enabled.size === 0}>
        Disable all
      </button>
      {#if saving}
        <span class="status">Saving...</span>
      {:else if savedAt}
        <span class="status saved">Saved {savedAt.toLocaleTimeString()}</span>
      {/if}
    </footer>
  {/if}
</section>

<style>
  .open5e-sources-panel {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 1rem;
    max-width: 720px;
  }
  header h2 {
    margin: 0 0 0.25rem;
    color: #e94560;
  }
  .hint {
    color: #b0b0c0;
    margin: 0 0 1rem;
    font-size: 0.9rem;
  }
  .empty {
    color: #b0b0c0;
    font-style: italic;
    margin: 0 0 1rem;
  }
  .manage {
    background: #0f3460;
    border-radius: 4px;
    padding: 0.75rem;
    margin: 0 0 1rem;
  }
  .howto summary {
    cursor: pointer;
    color: #e0e0e0;
    font-weight: 600;
  }
  .howto ol {
    margin: 0.5rem 0 0.75rem;
    padding-left: 1.25rem;
    color: #b0b0c0;
    font-size: 0.85rem;
    line-height: 1.5;
  }
  .howto code {
    font-family: ui-monospace, monospace;
    background: #16213e;
    padding: 0 0.25rem;
    border-radius: 3px;
    color: #e0e0e0;
  }
  .howto a {
    color: #6cb6ff;
  }
  .add-form {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    align-items: flex-start;
  }
  .add-form .fields {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.5rem;
    flex: 1 1 auto;
  }
  .add-form input {
    background: #16213e;
    color: #e0e0e0;
    border: 1px solid #1f4068;
    border-radius: 4px;
    padding: 0.4rem 0.6rem;
    font-size: 0.85rem;
  }
  .add-form button {
    background: #e94560;
    color: #fff;
    border: none;
    border-radius: 4px;
    padding: 0.45rem 0.9rem;
    cursor: pointer;
    font-weight: 600;
  }
  .add-form button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .notice {
    color: #6ce26c;
    font-size: 0.85rem;
    margin: 0.5rem 0 0;
  }
  .source-list {
    list-style: none;
    padding: 0;
    margin: 0 0 1rem;
    display: grid;
    gap: 0.5rem;
  }
  .source-list li {
    background: #0f3460;
    padding: 0.5rem 0.75rem;
    border-radius: 4px;
    display: flex;
    align-items: flex-start;
    gap: 0.5rem;
  }
  .source-list label {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 0.25rem 0.75rem;
    align-items: baseline;
    cursor: pointer;
    flex: 1 1 auto;
  }
  .title {
    color: #e0e0e0;
    font-weight: 600;
  }
  .publisher {
    color: #b0b0c0;
    grid-column: 2;
    font-size: 0.85rem;
  }
  .description {
    color: #b0b0c0;
    grid-column: 2;
    font-size: 0.85rem;
  }
  .slug {
    grid-column: 2;
    font-family: ui-monospace, monospace;
    font-size: 0.75rem;
    color: #888;
  }
  .row-actions {
    display: flex;
    gap: 0.25rem;
    align-items: center;
    flex: 0 0 auto;
  }
  .del {
    background: transparent;
    color: #ff6b6b;
    border: 1px solid transparent;
    border-radius: 4px;
    padding: 0.1rem 0.4rem;
    cursor: pointer;
    font-size: 0.9rem;
  }
  .del:hover {
    border-color: #ff6b6b;
  }
  .confirm-del {
    background: #e94560;
    color: #fff;
    border: none;
    border-radius: 4px;
    padding: 0.2rem 0.5rem;
    cursor: pointer;
    font-size: 0.75rem;
  }
  .cancel-del {
    background: #1f4068;
    color: #e0e0e0;
    border: none;
    border-radius: 4px;
    padding: 0.2rem 0.5rem;
    cursor: pointer;
    font-size: 0.75rem;
  }
  .builtin-badge {
    flex: 0 0 auto;
    color: #888;
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    align-self: center;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 1rem;
  }
  .actions button {
    background: #0f3460;
    color: #e0e0e0;
    border: 1px solid #16213e;
    border-radius: 4px;
    padding: 0.4rem 0.8rem;
    cursor: pointer;
  }
  .actions button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .status {
    color: #b0b0c0;
    font-size: 0.85rem;
  }
  .status.saved {
    color: #6ce26c;
  }
  .error {
    color: #ff6b6b;
    margin: 0.5rem 0 0;
  }
</style>
