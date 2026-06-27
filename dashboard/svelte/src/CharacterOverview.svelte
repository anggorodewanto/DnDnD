<script>
  // med-37 / Phase 101: Character Overview Svelte UI. Minimal viable
  // implementation per the bundled task constraints — fetch the
  // /api/character-overview endpoint and render party cards plus the
  // shared-language rollup. No editing affordances; the DM dashboard
  // already exposes mutating endpoints via other panels.
  //
  // G-101: Each card now embeds a MessagePlayerPanel with the character's
  // UUID preselected, so the DM can send a DM (and see history) without
  // leaving the party page.
  import MessagePlayerPanel from './MessagePlayerPanel.svelte';
  import InventoryEditorPanel from './InventoryEditorPanel.svelte';
  import {
    STATUS_CONDITIONS,
    MAX_EXHAUSTION,
    saveCharacterStatus,
  } from './lib/characterStatus.js';

  let { campaignId } = $props();

  let loading = $state(true);
  let error = $state(null);
  let characters = $state([]);
  let partyLanguages = $state([]);
  let messagingCharacterId = $state(null);
  let inventoryCharacterId = $state(null);

  // Out-of-combat status editor state. Only one card's editor is open at a
  // time; `statusForm` is prefilled from that card's current values.
  let editingStatusId = $state(null);
  let statusForm = $state({
    hpCurrent: 0,
    hpMax: 0,
    tempHp: 0,
    exhaustionLevel: 0,
    conditions: [],
    reason: '',
  });
  let statusSaving = $state(false);
  let statusError = $state(null);

  async function load() {
    if (!campaignId) {
      loading = false;
      return;
    }
    loading = true;
    error = null;
    try {
      const res = await fetch(
        `/api/character-overview?campaign_id=${encodeURIComponent(campaignId)}`,
        { credentials: 'same-origin' }
      );
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`);
      }
      const data = await res.json();
      characters = data.characters || [];
      partyLanguages = data.party_languages || [];
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    if (campaignId) {
      load();
    } else {
      characters = [];
      partyLanguages = [];
      loading = false;
      error = null;
    }
  });

  function classSummary(rawClasses) {
    if (!rawClasses) return '';
    try {
      const parsed = typeof rawClasses === 'string' ? JSON.parse(rawClasses) : rawClasses;
      if (!Array.isArray(parsed)) return '';
      return parsed.map((c) => `${c.class} ${c.level}`).join(' / ');
    } catch (_) {
      return '';
    }
  }

  // Open the inline status editor for a card, prefilled from its current
  // overview values (hp_current / hp_max / temp_hp / exhaustion_level /
  // conditions). Conditions are copied so checkbox toggles don't mutate the
  // fetched data in place.
  function openStatusEditor(c) {
    editingStatusId = c.character_id;
    statusError = null;
    statusSaving = false;
    statusForm = {
      hpCurrent: c.hp_current ?? 0,
      hpMax: c.hp_max ?? 0,
      tempHp: c.temp_hp ?? 0,
      exhaustionLevel: c.exhaustion_level ?? 0,
      conditions: Array.isArray(c.conditions) ? [...c.conditions] : [],
      reason: '',
    };
  }

  function closeStatusEditor() {
    editingStatusId = null;
    statusError = null;
  }

  function toggleStatusCondition(name) {
    const conditions = statusForm.conditions.includes(name)
      ? statusForm.conditions.filter((n) => n !== name)
      : [...statusForm.conditions, name];
    statusForm = { ...statusForm, conditions };
  }

  async function saveStatus(characterId) {
    statusSaving = true;
    statusError = null;
    try {
      await saveCharacterStatus(characterId, statusForm);
      // Success: close the editor and re-fetch so the card shows new values.
      editingStatusId = null;
      await load();
    } catch (e) {
      // Keep the editor open and surface the server's explanation (e.g. 409).
      statusError = e.message;
    } finally {
      statusSaving = false;
    }
  }
</script>

<div class="character-overview">
  <h2>Party Overview</h2>

  {#if loading}
    <p>Loading party...</p>
  {:else if error}
    <p class="error">Failed to load: {error}</p>
  {:else if !campaignId}
    <p>No active campaign selected.</p>
  {:else if characters.length === 0}
    <p>No approved characters in this campaign yet.</p>
  {:else}
    <div class="grid">
      {#each characters as c (c.character_id)}
        <div class="card">
          <h3>{c.name}</h3>
          <p class="meta">{c.race} — {classSummary(c.classes) || `Level ${c.level}`}</p>
          <ul>
            <li>
              HP: {c.hp_current} / {c.hp_max}{#if c.temp_hp} (+{c.temp_hp} temp){/if}
            </li>
            <li>AC: {c.ac}</li>
            <li>Speed: {c.speed_ft} ft</li>
            <li>Exhaustion: {c.exhaustion_level || 0}</li>
            <li>
              Conditions:
              {#if c.conditions && c.conditions.length > 0}
                {c.conditions.join(', ')}
              {:else}
                none
              {/if}
            </li>
          </ul>
          {#if c.languages && c.languages.length > 0}
            <p class="langs">Languages: {c.languages.join(', ')}</p>
          {/if}
          {#if c.ddb_url}
            <a href={c.ddb_url} target="_blank" rel="noopener">D&amp;D Beyond sheet</a>
          {/if}
          <a
            class="sheet-link"
            data-testid="character-sheet-{c.character_id}"
            href={`/portal/character/${c.character_id}`}
            target="_blank"
            rel="noopener"
          >View character sheet</a>
          <a
            class="edit-link"
            data-testid="character-edit-{c.character_id}"
            href={`/portal/character/${c.character_id}/edit`}
            target="_blank"
            rel="noopener"
          >Edit character</a>
          <button
            class="msg-toggle"
            data-testid="character-message-toggle-{c.character_id}"
            onclick={() => (messagingCharacterId = messagingCharacterId === c.character_id ? null : c.character_id)}
          >
            {messagingCharacterId === c.character_id ? 'Close message panel' : 'Message this player'}
          </button>
          {#if messagingCharacterId === c.character_id}
            <div class="msg-embed">
              <MessagePlayerPanel
                {campaignId}
                playerCharacterId={c.character_id}
                playerName={c.name}
                hidePicker={true}
              />
            </div>
          {/if}
          <button
            class="msg-toggle"
            data-testid="character-inventory-toggle-{c.character_id}"
            onclick={() => (inventoryCharacterId = inventoryCharacterId === c.character_id ? null : c.character_id)}
          >
            {inventoryCharacterId === c.character_id ? 'Close inventory' : 'Manage inventory'}
          </button>
          {#if inventoryCharacterId === c.character_id}
            <div class="msg-embed">
              <InventoryEditorPanel
                {campaignId}
                characterId={c.character_id}
                characterName={c.name}
                party={characters.map((p) => ({ character_id: p.character_id, name: p.name }))}
              />
            </div>
          {/if}
          <button
            class="msg-toggle"
            data-testid="character-status-toggle-{c.character_id}"
            onclick={() => (editingStatusId === c.character_id ? closeStatusEditor() : openStatusEditor(c))}
          >
            {editingStatusId === c.character_id ? 'Close status editor' : 'Edit status'}
          </button>
          {#if editingStatusId === c.character_id}
            <div class="status-editor" data-testid="status-editor-{c.character_id}">
              {#if statusError}
                <div class="status-error" data-testid="status-error-{c.character_id}">{statusError}</div>
              {/if}
              <div class="status-row">
                <label>
                  HP current
                  <input type="number" data-testid="status-hp-current" bind:value={statusForm.hpCurrent} />
                </label>
                <label>
                  HP max
                  <input type="number" data-testid="status-hp-max" bind:value={statusForm.hpMax} />
                </label>
              </div>
              <div class="status-row">
                <label>
                  Temp HP
                  <input type="number" min="0" data-testid="status-temp-hp" bind:value={statusForm.tempHp} />
                </label>
                <label>
                  Exhaustion
                  <input
                    type="number"
                    min="0"
                    max={MAX_EXHAUSTION}
                    data-testid="status-exhaustion"
                    bind:value={statusForm.exhaustionLevel}
                  />
                </label>
              </div>
              <fieldset class="status-conditions">
                <legend>Conditions</legend>
                {#each STATUS_CONDITIONS as cond}
                  <label class="cond-check">
                    <input
                      type="checkbox"
                      value={cond}
                      data-testid="status-condition-{cond}"
                      checked={statusForm.conditions.includes(cond)}
                      onchange={() => toggleStatusCondition(cond)}
                    />
                    {cond}
                  </label>
                {/each}
              </fieldset>
              <label class="status-reason">
                Reason (optional)
                <input type="text" data-testid="status-reason" bind:value={statusForm.reason} />
              </label>
              <div class="status-actions">
                <button
                  class="status-save"
                  data-testid="status-save-{c.character_id}"
                  onclick={() => saveStatus(c.character_id)}
                  disabled={statusSaving}
                >
                  {statusSaving ? 'Saving…' : 'Save status'}
                </button>
                <button
                  class="status-cancel"
                  data-testid="status-cancel-{c.character_id}"
                  onclick={closeStatusEditor}
                  disabled={statusSaving}
                >Cancel</button>
              </div>
            </div>
          {/if}
        </div>
      {/each}
    </div>

    {#if partyLanguages.length > 0}
      <h3>Shared Languages</h3>
      <ul class="langs-list">
        {#each partyLanguages as row}
          <li><strong>{row.language}:</strong> {row.characters.join(', ')}</li>
        {/each}
      </ul>
    {/if}
  {/if}
</div>

<style>
  .character-overview {
    max-width: 1000px;
  }
  h2, h3 {
    color: #e94560;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 1rem;
    margin-bottom: 1.5rem;
  }
  .card {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 6px;
    padding: 0.75rem 1rem;
  }
  .card h3 {
    margin: 0 0 0.25rem 0;
    color: #e0e0e0;
  }
  .meta {
    color: #b0b0c0;
    margin: 0 0 0.5rem 0;
    font-style: italic;
  }
  ul {
    list-style: none;
    padding: 0;
    margin: 0 0 0.5rem 0;
  }
  ul li {
    padding: 0.15rem 0;
  }
  .langs {
    margin: 0.25rem 0;
    color: #b0b0c0;
    font-size: 0.9rem;
  }
  .langs-list li {
    padding: 0.25rem 0;
  }
  .error {
    color: #ff6b6b;
  }
  a {
    color: #e94560;
  }
  .edit-link {
    display: inline-block;
    margin-top: 0.5rem;
    margin-right: 0.5rem;
    padding: 0.35rem 0.6rem;
    background: #0f3460;
    color: #e94560;
    border-radius: 4px;
    font-size: 0.85rem;
    text-decoration: none;
  }
  .edit-link:hover {
    background: #1a4a8a;
  }
  .sheet-link {
    display: inline-block;
    margin-top: 0.5rem;
    margin-right: 0.5rem;
    padding: 0.35rem 0.6rem;
    background: #e94560;
    color: #16213e;
    border-radius: 4px;
    font-size: 0.85rem;
    font-weight: bold;
    text-decoration: none;
  }
  .sheet-link:hover {
    background: #ff5c77;
  }
  .msg-toggle {
    margin-top: 0.5rem;
    padding: 0.35rem 0.6rem;
    background: #0f3460;
    color: #e0e0e0;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
  }
  .msg-toggle:hover {
    background: #1a4a8a;
  }
  .msg-embed {
    margin-top: 0.5rem;
  }
  .status-editor {
    margin-top: 0.5rem;
    padding: 0.6rem;
    background: #14203a;
    border: 1px solid #0f3460;
    border-radius: 6px;
  }
  .status-error {
    background: #3e1a1a;
    color: #ff6b6b;
    padding: 0.5rem;
    border-radius: 4px;
    margin-bottom: 0.5rem;
    font-size: 0.85rem;
  }
  .status-row {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 0.5rem;
  }
  .status-row label,
  .status-reason {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    color: #a0a0c0;
    font-size: 0.8rem;
    flex: 1;
  }
  .status-reason {
    display: flex;
    margin-bottom: 0.5rem;
  }
  .status-editor input[type='number'],
  .status-reason input[type='text'] {
    padding: 0.4rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    width: 100%;
    box-sizing: border-box;
  }
  .status-conditions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem 0.75rem;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 0.5rem;
    margin: 0 0 0.5rem 0;
  }
  .status-conditions legend {
    color: #a0a0c0;
    font-size: 0.8rem;
    padding: 0 0.3rem;
  }
  .cond-check {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    color: #e0e0e0;
    font-size: 0.8rem;
  }
  .status-actions {
    display: flex;
    gap: 0.5rem;
  }
  .status-save {
    padding: 0.4rem 0.8rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
  }
  .status-save:hover { background: #c73852; }
  .status-save:disabled { opacity: 0.5; cursor: not-allowed; }
  .status-cancel {
    padding: 0.4rem 0.8rem;
    background: transparent;
    color: #a0a0c0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
  }
  .status-cancel:hover { background: #0f3460; color: #e0e0e0; }
  .status-cancel:disabled { opacity: 0.4; cursor: not-allowed; }
</style>
