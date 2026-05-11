<script>
  // med-37 / Phase 99: Homebrew Editor Svelte UI. Minimal viable
  // implementation per the bundled task constraints — list, create,
  // delete homebrew rows by category. Edit reuses the create form
  // (overwrites by ID via the existing PUT endpoint).

  let { campaignId } = $props();

  // Categories supported by /api/homebrew (handler.go).
  const CATEGORIES = [
    { key: 'creatures', label: 'Creatures' },
    { key: 'spells', label: 'Spells' },
    { key: 'weapons', label: 'Weapons' },
    { key: 'magic-items', label: 'Magic Items' },
    { key: 'races', label: 'Races' },
    { key: 'feats', label: 'Feats' },
    { key: 'classes', label: 'Classes' },
  ];

  let category = $state('creatures');
  let entries = $state([]);
  let loading = $state(false);
  let error = $state(null);
  let formOpen = $state(false);
  let formText = $state('');
  let formError = $state(null);
  let editingId = $state(null);

  // Listing relies on the existing per-category list endpoints. We use the
  // generic /api/<plural> endpoints that already filter by campaign +
  // homebrew-only when the campaign_id query is passed (see refdata
  // handlers).
  const LIST_ENDPOINTS = {
    creatures: '/api/creatures',
    spells: '/api/spells',
    weapons: '/api/weapons',
    'magic-items': '/api/magic-items',
    races: '/api/races',
    feats: '/api/feats',
    classes: '/api/classes',
  };

  async function loadEntries() {
    if (!campaignId) {
      entries = [];
      return;
    }
    loading = true;
    error = null;
    try {
      const url = `${LIST_ENDPOINTS[category]}?campaign_id=${encodeURIComponent(campaignId)}&homebrew=true`;
      const res = await fetch(url, { credentials: 'same-origin' });
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`);
      }
      const data = await res.json();
      entries = (Array.isArray(data) ? data : data.items || []).filter((e) => e.homebrew !== false);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function openCreate() {
    editingId = null;
    formText = JSON.stringify({ id: '', name: '' }, null, 2);
    formError = null;
    formOpen = true;
  }

  function openEdit(entry) {
    editingId = entry.id;
    formText = JSON.stringify(entry, null, 2);
    formError = null;
    formOpen = true;
  }

  async function submitForm() {
    formError = null;
    let body;
    try {
      body = JSON.parse(formText);
    } catch (e) {
      formError = `Invalid JSON: ${e.message}`;
      return;
    }
    const baseURL = `/api/homebrew/${category}`;
    const url = editingId
      ? `${baseURL}/${encodeURIComponent(editingId)}?campaign_id=${encodeURIComponent(campaignId)}`
      : `${baseURL}?campaign_id=${encodeURIComponent(campaignId)}`;
    const method = editingId ? 'PUT' : 'POST';
    try {
      const res = await fetch(url, {
        method,
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}: ${await res.text()}`);
      }
      formOpen = false;
      await loadEntries();
    } catch (e) {
      formError = e.message;
    }
  }

  async function handleDelete(entry) {
    if (!confirm(`Delete ${category} "${entry.name || entry.id}"?`)) return;
    try {
      const url = `/api/homebrew/${category}/${encodeURIComponent(entry.id)}?campaign_id=${encodeURIComponent(campaignId)}`;
      const res = await fetch(url, { method: 'DELETE', credentials: 'same-origin' });
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`);
      }
      await loadEntries();
    } catch (e) {
      error = e.message;
    }
  }

  $effect(() => {
    if (campaignId && category) {
      loadEntries();
    }
  });
</script>

<div class="homebrew-editor">
  <h2>Homebrew Editor</h2>

  <div class="category-tabs">
    {#each CATEGORIES as cat}
      <button
        class:active={category === cat.key}
        onclick={() => (category = cat.key)}>{cat.label}</button>
    {/each}
  </div>

  <div class="actions">
    <button class="create-btn" onclick={openCreate}>+ New {category}</button>
  </div>

  {#if loading}
    <p>Loading...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if entries.length === 0}
    <p>No homebrew {category} yet.</p>
  {:else}
    <ul class="entry-list">
      {#each entries as entry (entry.id)}
        <li>
          <strong>{entry.name || entry.id}</strong>
          <span class="muted">{entry.id}</span>
          <button onclick={() => openEdit(entry)}>Edit</button>
          <button class="delete-btn" onclick={() => handleDelete(entry)}>Delete</button>
        </li>
      {/each}
    </ul>
  {/if}

  {#if formOpen}
    <div class="form">
      <h3>{editingId ? 'Edit' : 'New'} {category}</h3>
      <p class="hint">Edit the JSON body directly. Required fields vary by category — see refdata Upsert*Params.</p>
      <textarea bind:value={formText} rows="20"></textarea>
      {#if formError}
        <p class="error">{formError}</p>
      {/if}
      <div class="form-actions">
        <button onclick={submitForm}>Save</button>
        <button onclick={() => (formOpen = false)}>Cancel</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .homebrew-editor {
    max-width: 1000px;
  }
  h2 {
    color: #e94560;
  }
  .category-tabs {
    display: flex;
    gap: 0.25rem;
    margin-bottom: 1rem;
    flex-wrap: wrap;
  }
  .category-tabs button {
    padding: 0.4rem 0.75rem;
    background: #16213e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }
  .category-tabs button.active {
    background: #e94560;
    border-color: #e94560;
  }
  .actions {
    margin-bottom: 0.75rem;
  }
  .create-btn {
    padding: 0.5rem 1rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }
  .entry-list {
    list-style: none;
    padding: 0;
  }
  .entry-list li {
    background: #16213e;
    padding: 0.5rem 0.75rem;
    border-radius: 4px;
    margin-bottom: 0.25rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .muted {
    color: #888;
    font-size: 0.85rem;
  }
  .delete-btn {
    background: #aa2030;
    color: white;
    border: none;
    border-radius: 3px;
    padding: 0.25rem 0.5rem;
    cursor: pointer;
  }
  .form {
    margin-top: 1rem;
    background: #16213e;
    padding: 1rem;
    border-radius: 6px;
    border: 1px solid #0f3460;
  }
  .form textarea {
    width: 100%;
    background: #0f1626;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 0.5rem;
    font-family: monospace;
  }
  .form-actions {
    margin-top: 0.5rem;
    display: flex;
    gap: 0.5rem;
  }
  .form-actions button {
    padding: 0.5rem 1rem;
    background: #0f3460;
    color: #e0e0e0;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }
  .form-actions button:first-child {
    background: #e94560;
    color: white;
  }
  .hint {
    color: #b0b0c0;
    font-size: 0.85rem;
    margin: 0 0 0.5rem 0;
  }
  .error {
    color: #ff6b6b;
  }
</style>
