<script>
  /**
   * LootPoolPanel — DM dashboard widget for managing an encounter loot pool.
   *
   * F-13: backend loot endpoints (POST/GET/DELETE/PUT under
   * /api/campaigns/:cid/encounters/:eid/loot) already exist; this panel
   * wires the existing ItemPicker into a CRUD UI so DMs no longer have to
   * call the API directly.
   *
   * Props:
   *   campaignId {string} — campaign UUID (Phase 21a active campaign).
   */
  import {
    listEligibleLootEncounters,
    getLootPool,
    createLootPool,
    addLootPoolItem,
    removeLootPoolItem,
    setLootGold,
    postLootAnnouncement,
  } from '$lib/api.js';
  import ItemPicker from './ItemPicker.svelte';

  let { campaignId = '' } = $props();

  let encounters = $state([]);
  let selectedEncounterId = $state('');
  let pool = $state(null);
  let items = $state([]);
  let gold = $state(0);
  let loading = $state(false);
  let creating = $state(false);
  let posting = $state(false);
  let savingGold = $state(false);
  let showItemPicker = $state(false);
  let error = $state('');
  let info = $state('');

  async function loadEncounters() {
    if (!campaignId) return;
    try {
      const data = await listEligibleLootEncounters(campaignId);
      encounters = (data && data.encounters) || [];
    } catch (e) {
      error = 'Failed to load encounters: ' + e.message;
    }
  }

  async function loadPool() {
    pool = null;
    items = [];
    gold = 0;
    error = '';
    info = '';
    if (!campaignId || !selectedEncounterId) return;
    loading = true;
    try {
      const result = await getLootPool(campaignId, selectedEncounterId);
      pool = result.Pool || null;
      items = result.Items || [];
      gold = pool && typeof pool.gold_total === 'number' ? pool.gold_total : 0;
    } catch (e) {
      // 404 = no pool exists yet, that's OK — DM can press "Create pool".
      if (!/not found|404/i.test(e.message)) {
        error = 'Failed to load loot pool: ' + e.message;
      }
    } finally {
      loading = false;
    }
  }

  async function createPool() {
    if (!campaignId || !selectedEncounterId) return;
    creating = true;
    error = '';
    info = '';
    try {
      const result = await createLootPool(campaignId, selectedEncounterId);
      pool = result.Pool || null;
      items = result.Items || [];
      gold = pool && typeof pool.gold_total === 'number' ? pool.gold_total : 0;
    } catch (e) {
      error = 'Failed to create loot pool: ' + e.message;
    } finally {
      creating = false;
    }
  }

  async function addSelectedItems(selected) {
    if (!pool) return;
    error = '';
    for (const it of selected) {
      try {
        const created = await addLootPoolItem(campaignId, selectedEncounterId, {
          item_id: typeof it.id === 'string' && !it.id.startsWith('custom-') && !it.id.startsWith('creature-') ? it.id : '',
          name: it.name,
          description: it.description || it.narrative || '',
          quantity: it.quantity || 1,
          type: it.type === 'custom' ? 'other' : (it.type || 'other'),
          is_magic: !!it.is_magic,
          magic_bonus: it.magic_bonus || 0,
          magic_properties: it.magic_properties || '',
          requires_attunement: !!it.requires_attunement,
          rarity: it.rarity || '',
        });
        items = [...items, created];
      } catch (e) {
        error = 'Failed to add item: ' + e.message;
      }
    }
    showItemPicker = false;
  }

  async function removeItem(itemId) {
    try {
      await removeLootPoolItem(campaignId, selectedEncounterId, itemId);
      items = items.filter((i) => i.id !== itemId);
    } catch (e) {
      error = 'Failed to remove item: ' + e.message;
    }
  }

  async function saveGold() {
    if (!pool) return;
    savingGold = true;
    error = '';
    try {
      const updated = await setLootGold(campaignId, selectedEncounterId, parseInt(gold, 10) || 0);
      pool = updated;
      gold = updated.gold_total;
    } catch (e) {
      error = 'Failed to update gold: ' + e.message;
    } finally {
      savingGold = false;
    }
  }

  async function announce() {
    if (!pool) return;
    posting = true;
    error = '';
    info = '';
    try {
      const res = await postLootAnnouncement(campaignId, selectedEncounterId);
      info = res && res.message ? 'Posted announcement: ' + res.message.split('\n')[0] : 'Announcement posted.';
    } catch (e) {
      error = 'Failed to post announcement: ' + e.message;
    } finally {
      posting = false;
    }
  }

  function onEncounterChange() {
    showItemPicker = false;
    loadPool();
  }

  $effect(() => {
    if (campaignId) {
      loadEncounters();
    }
  });
</script>

<div class="loot-pool-panel">
  <div class="panel-header">
    <h2>Loot Pool</h2>
    <p class="hint">
      Select a completed encounter to view, populate, or announce its loot pool.
      Loot pools are auto-populated from defeated NPC inventories on creation;
      use the Item Picker below to add additional rewards.
    </p>
  </div>

  {#if error}
    <div class="error">{error}</div>
  {/if}
  {#if info}
    <div class="success">{info}</div>
  {/if}

  <div class="encounter-select">
    <label>
      Encounter
      <select bind:value={selectedEncounterId} onchange={onEncounterChange}>
        <option value="">— select a completed encounter —</option>
        {#each encounters as enc}
          <option value={enc.id}>{enc.display_name || enc.name}</option>
        {/each}
      </select>
    </label>
    {#if encounters.length === 0}
      <p class="empty">
        No completed encounters yet. Finish an encounter from the Combat
        Manager to make it eligible for a loot pool.
      </p>
    {/if}
  </div>

  {#if selectedEncounterId}
    {#if loading}
      <p class="loading">Loading pool…</p>
    {:else if !pool}
      <div class="no-pool">
        <p>No loot pool exists for this encounter yet.</p>
        <button class="primary-btn" onclick={createPool} disabled={creating}>
          {creating ? 'Creating…' : 'Create loot pool'}
        </button>
      </div>
    {:else}
      <div class="pool-meta">
        <div class="meta-row">
          <label>
            Gold (gp)
            <input type="number" bind:value={gold} min="0" />
          </label>
          <button class="secondary-btn" onclick={saveGold} disabled={savingGold}>
            {savingGold ? 'Saving…' : 'Update gold'}
          </button>
          <button class="primary-btn" onclick={announce} disabled={posting}>
            {posting ? 'Posting…' : 'Announce to players'}
          </button>
        </div>
        <p class="muted">Status: {pool.status}</p>
      </div>

      <div class="pool-items">
        <div class="items-header">
          <h3>Items ({items.length})</h3>
          <button class="primary-btn" onclick={() => showItemPicker = !showItemPicker}>
            {showItemPicker ? 'Close picker' : '+ Add items'}
          </button>
        </div>

        {#if showItemPicker}
          <ItemPicker {campaignId} encounterId={selectedEncounterId} onselect={addSelectedItems} />
        {/if}

        {#if items.length === 0}
          <p class="empty">No items in the pool. Use the Item Picker to populate.</p>
        {:else}
          <ul class="items-list">
            {#each items as item}
              <li class="loot-item" class:claimed={item.claimed_by && item.claimed_by.Valid}>
                <div class="item-info">
                  <strong>{item.name}</strong>
                  {#if item.quantity > 1}<span class="qty">x{item.quantity}</span>{/if}
                  <span class="item-type">{item.type}</span>
                  {#if item.is_magic}
                    <span class="magic-tag">magic{item.rarity ? ` · ${item.rarity}` : ''}</span>
                  {/if}
                  {#if item.requires_attunement}
                    <span class="attune-tag">attunement</span>
                  {/if}
                  {#if item.claimed_by && item.claimed_by.Valid}
                    <span class="claimed-tag">claimed</span>
                  {/if}
                  {#if item.description}
                    <p class="item-desc">{item.description}</p>
                  {/if}
                </div>
                <button class="remove-btn" onclick={() => removeItem(item.id)}
                  disabled={item.claimed_by && item.claimed_by.Valid}>Remove</button>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    {/if}
  {/if}
</div>

<style>
  .loot-pool-panel {
    max-width: 900px;
  }

  .panel-header h2 {
    color: #e94560;
    margin: 0 0 0.5rem 0;
  }

  .hint {
    color: #a0a0c0;
    font-size: 0.9rem;
    margin: 0 0 1rem 0;
  }

  .error {
    background: #3e1a1a;
    color: #ff6b6b;
    padding: 0.75rem;
    border-radius: 4px;
    margin-bottom: 1rem;
  }

  .success {
    background: #1a3e2a;
    color: #6bff8b;
    padding: 0.75rem;
    border-radius: 4px;
    margin-bottom: 1rem;
  }

  .encounter-select {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    padding: 1rem;
    margin-bottom: 1rem;
  }

  .encounter-select label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    color: #a0a0c0;
    font-size: 0.85rem;
  }

  .encounter-select select {
    padding: 0.5rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }

  .empty {
    color: #a0a0c0;
    font-style: italic;
  }

  .loading {
    color: #a0a0c0;
    padding: 1rem 0;
  }

  .no-pool {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    padding: 1rem;
    margin-bottom: 1rem;
  }

  .pool-meta {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    padding: 1rem;
    margin-bottom: 1rem;
  }

  .meta-row {
    display: flex;
    gap: 0.75rem;
    align-items: flex-end;
    flex-wrap: wrap;
  }

  .meta-row label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    color: #a0a0c0;
    font-size: 0.85rem;
  }

  .meta-row input {
    padding: 0.5rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    width: 110px;
  }

  .muted {
    color: #a0a0c0;
    font-size: 0.8rem;
    margin: 0.5rem 0 0 0;
  }

  .pool-items {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    padding: 1rem;
  }

  .items-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .items-header h3 {
    color: #e94560;
    margin: 0;
  }

  .items-list {
    list-style: none;
    padding: 0;
    margin: 0;
  }

  .loot-item {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 0.5rem;
    padding: 0.5rem;
    border-bottom: 1px solid #0f3460;
  }

  .loot-item:hover { background: #1a1a2e; }

  .loot-item.claimed {
    opacity: 0.6;
  }

  .item-info {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .qty { color: #ffd700; font-size: 0.85rem; }

  .item-type {
    font-size: 0.75rem;
    padding: 0.15rem 0.5rem;
    background: #0f3460;
    border-radius: 3px;
    color: #a0a0c0;
  }

  .magic-tag {
    font-size: 0.75rem;
    padding: 0.15rem 0.5rem;
    background: #5b3a8c;
    border-radius: 3px;
    color: #e0d0ff;
  }

  .attune-tag {
    font-size: 0.75rem;
    padding: 0.15rem 0.5rem;
    background: #6b4226;
    border-radius: 3px;
    color: #ffd9a0;
  }

  .claimed-tag {
    font-size: 0.75rem;
    padding: 0.15rem 0.5rem;
    background: #1a3e2a;
    border-radius: 3px;
    color: #6bff8b;
  }

  .item-desc {
    font-size: 0.85rem;
    color: #a0a0c0;
    width: 100%;
    margin: 0.25rem 0 0 0;
  }

  .primary-btn {
    padding: 0.5rem 1rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }

  .primary-btn:hover { background: #c73852; }
  .primary-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .secondary-btn {
    padding: 0.5rem 1rem;
    background: #0f3460;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }

  .secondary-btn:hover { background: #16213e; }
  .secondary-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .remove-btn {
    padding: 0.25rem 0.75rem;
    background: transparent;
    color: #e94560;
    border: 1px solid #e94560;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.8rem;
  }

  .remove-btn:hover { background: #e94560; color: white; }
  .remove-btn:disabled { opacity: 0.3; cursor: not-allowed; }
</style>
