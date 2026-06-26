<script>
  /**
   * InventoryEditorPanel — DM dashboard widget for adjusting one player
   * character's inventory and gold. Embedded per-character on the Party page
   * (CharacterOverview), mirroring the LootPoolPanel CRUD pattern over the
   * existing DM-only /api/inventory/* endpoints.
   *
   * Props:
   *   characterId   {string} — character UUID to edit.
   *   characterName {string} — display name (for transfer log / labels).
   *   campaignId    {string} — campaign UUID (ItemPicker scope).
   *   party         {Array<{character_id: string, name: string}>} — full party,
   *                  used to populate the per-item transfer target dropdown.
   */
  import {
    getCharacterInventory,
    addInventoryItem,
    removeInventoryItem,
    setCharacterGold,
    setInventoryItemIdentified,
    transferInventoryItem,
  } from '$lib/api.js';
  import { toAddItemPayload, isUnidentified } from '$lib/inventoryEditor.js';
  import ItemPicker from './ItemPicker.svelte';

  let { characterId = '', characterName = '', campaignId = '', party = [] } = $props();

  let items = $state([]);
  let gold = $state(0);
  let loading = $state(false);
  let savingGold = $state(false);
  let adding = $state(false);
  let showItemPicker = $state(false);
  let pendingAdd = $state([]);
  let qtyById = $state({});
  let transferTo = $state({});
  let busyItemId = $state('');
  let error = $state('');
  let info = $state('');

  let transferTargets = $derived(party.filter((p) => p.character_id !== characterId));

  async function load() {
    if (!characterId) return;
    loading = true;
    error = '';
    try {
      const data = await getCharacterInventory(characterId);
      items = data.items || [];
      gold = typeof data.gold === 'number' ? data.gold : 0;
    } catch (e) {
      error = 'Failed to load inventory: ' + e.message;
    } finally {
      loading = false;
    }
  }

  function qtyFor(id) {
    const v = qtyById[id];
    return Number.isFinite(v) && v > 0 ? v : 1;
  }

  function setQty(id, value) {
    qtyById = { ...qtyById, [id]: parseInt(value, 10) || 1 };
  }

  async function commitAdds() {
    if (pendingAdd.length === 0) return;
    adding = true;
    error = '';
    info = '';
    try {
      for (const picked of pendingAdd) {
        await addInventoryItem(characterId, toAddItemPayload(picked));
      }
      info = `Added ${pendingAdd.length} item${pendingAdd.length === 1 ? '' : 's'}.`;
      pendingAdd = [];
      showItemPicker = false;
      await load();
    } catch (e) {
      error = 'Failed to add item: ' + e.message;
    } finally {
      adding = false;
    }
  }

  async function remove(item) {
    busyItemId = item.item_id;
    error = '';
    info = '';
    try {
      await removeInventoryItem(characterId, item.item_id, qtyFor(item.item_id));
      await load();
    } catch (e) {
      error = 'Failed to remove item: ' + e.message;
    } finally {
      busyItemId = '';
    }
  }

  async function toggleIdentified(item) {
    busyItemId = item.item_id;
    error = '';
    info = '';
    try {
      // identified is tri-state server-side: absent/true = revealed.
      const reveal = item.identified === false;
      await setInventoryItemIdentified(characterId, item.item_id, reveal);
      await load();
    } catch (e) {
      error = 'Failed to update identification: ' + e.message;
    } finally {
      busyItemId = '';
    }
  }

  async function transfer(item) {
    const toId = transferTo[item.item_id];
    if (!toId) return;
    busyItemId = item.item_id;
    error = '';
    info = '';
    try {
      await transferInventoryItem(characterId, toId, item.item_id, qtyFor(item.item_id));
      const target = party.find((p) => p.character_id === toId);
      info = `Transferred ${item.name} to ${target ? target.name : 'party member'}.`;
      transferTo = { ...transferTo, [item.item_id]: '' };
      await load();
    } catch (e) {
      error = 'Failed to transfer item: ' + e.message;
    } finally {
      busyItemId = '';
    }
  }

  async function saveGold() {
    savingGold = true;
    error = '';
    info = '';
    try {
      await setCharacterGold(characterId, parseInt(gold, 10) || 0);
      info = 'Gold updated.';
      await load();
    } catch (e) {
      error = 'Failed to update gold: ' + e.message;
    } finally {
      savingGold = false;
    }
  }

  $effect(() => {
    if (characterId) {
      load();
    }
  });
</script>

<div class="inventory-editor" data-testid="inventory-editor-{characterId}">
  {#if error}
    <div class="error">{error}</div>
  {/if}
  {#if info}
    <div class="success">{info}</div>
  {/if}

  <div class="gold-row">
    <label>
      Gold (gp)
      <input type="number" bind:value={gold} min="0" data-testid="inventory-gold-input" />
    </label>
    <button class="secondary-btn" onclick={saveGold} disabled={savingGold}>
      {savingGold ? 'Saving…' : 'Update gold'}
    </button>
  </div>

  <div class="items-header">
    <h4>Items ({items.length})</h4>
    <button class="primary-btn" onclick={() => (showItemPicker = !showItemPicker)}>
      {showItemPicker ? 'Close picker' : '+ Add items'}
    </button>
  </div>

  {#if showItemPicker}
    <ItemPicker {campaignId} onselect={(sel) => (pendingAdd = sel)} />
    <button class="primary-btn add-commit" onclick={commitAdds} disabled={adding || pendingAdd.length === 0}>
      {adding ? 'Adding…' : `Add ${pendingAdd.length || ''} to inventory`}
    </button>
  {/if}

  {#if loading}
    <p class="muted">Loading inventory…</p>
  {:else if items.length === 0}
    <p class="muted">No items. Use the picker above to grant some.</p>
  {:else}
    <ul class="items-list">
      {#each items as item (item.item_id + item.name)}
        <li class="inv-item">
          <div class="item-info">
            <strong>{item.name}</strong>
            {#if item.quantity > 1}<span class="qty">x{item.quantity}</span>{/if}
            {#if item.type}<span class="item-type">{item.type}</span>{/if}
            {#if item.is_magic}
              <span class="magic-tag">magic{item.rarity ? ` · ${item.rarity}` : ''}</span>
            {/if}
            {#if item.requires_attunement}<span class="attune-tag">attunement</span>{/if}
            {#if isUnidentified(item)}<span class="hidden-tag">hidden</span>{/if}
          </div>

          <div class="item-actions">
            <input
              class="qty-input"
              type="number"
              min="1"
              value={qtyFor(item.item_id)}
              oninput={(e) => setQty(item.item_id, e.target.value)}
              aria-label="quantity"
            />
            <button class="remove-btn" onclick={() => remove(item)} disabled={busyItemId === item.item_id}>
              Remove
            </button>
            {#if item.is_magic}
              <button class="ghost-btn" onclick={() => toggleIdentified(item)} disabled={busyItemId === item.item_id}>
                {item.identified === false ? 'Reveal' : 'Hide'}
              </button>
            {/if}
            {#if transferTargets.length > 0}
              <select
                value={transferTo[item.item_id] || ''}
                onchange={(e) => (transferTo = { ...transferTo, [item.item_id]: e.target.value })}
                aria-label="transfer target"
              >
                <option value="">Give to…</option>
                {#each transferTargets as t}
                  <option value={t.character_id}>{t.name}</option>
                {/each}
              </select>
              <button
                class="ghost-btn"
                onclick={() => transfer(item)}
                disabled={!transferTo[item.item_id] || busyItemId === item.item_id}
              >Give</button>
            {/if}
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .inventory-editor {
    background: #14203a;
    border: 1px solid #0f3460;
    border-radius: 6px;
    padding: 0.75rem;
    margin-top: 0.5rem;
  }

  .error {
    background: #3e1a1a;
    color: #ff6b6b;
    padding: 0.5rem;
    border-radius: 4px;
    margin-bottom: 0.5rem;
    font-size: 0.85rem;
  }

  .success {
    background: #1a3e2a;
    color: #6bff8b;
    padding: 0.5rem;
    border-radius: 4px;
    margin-bottom: 0.5rem;
    font-size: 0.85rem;
  }

  .gold-row {
    display: flex;
    gap: 0.5rem;
    align-items: flex-end;
    margin-bottom: 0.75rem;
  }

  .gold-row label {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    color: #a0a0c0;
    font-size: 0.8rem;
  }

  .gold-row input {
    padding: 0.4rem;
    background: #1a1a2e;
    color: #ffd700;
    border: 1px solid #0f3460;
    border-radius: 4px;
    width: 100px;
  }

  .items-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.5rem;
  }

  .items-header h4 {
    margin: 0;
    color: #e0e0e0;
  }

  .add-commit {
    margin: 0.5rem 0;
  }

  .items-list {
    list-style: none;
    padding: 0;
    margin: 0;
  }

  .inv-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
    padding: 0.4rem 0;
    border-bottom: 1px solid #0f3460;
    flex-wrap: wrap;
  }

  .item-info {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
  }

  .item-info strong {
    color: #e0e0e0;
  }

  .qty {
    color: #ffd700;
    font-size: 0.85rem;
  }

  .item-type {
    font-size: 0.7rem;
    padding: 0.1rem 0.4rem;
    background: #0f3460;
    border-radius: 3px;
    color: #a0a0c0;
  }

  .magic-tag {
    font-size: 0.7rem;
    padding: 0.1rem 0.4rem;
    background: #5b3a8c;
    border-radius: 3px;
    color: #e0d0ff;
  }

  .attune-tag {
    font-size: 0.7rem;
    padding: 0.1rem 0.4rem;
    background: #6b4226;
    border-radius: 3px;
    color: #ffd9a0;
  }

  .hidden-tag {
    font-size: 0.7rem;
    padding: 0.1rem 0.4rem;
    background: #3a3a4a;
    border-radius: 3px;
    color: #c0c0d0;
  }

  .item-actions {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    flex-wrap: wrap;
  }

  .qty-input {
    width: 54px;
    padding: 0.3rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }

  .item-actions select {
    padding: 0.3rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    font-size: 0.8rem;
  }

  .primary-btn {
    padding: 0.4rem 0.8rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
  }

  .primary-btn:hover { background: #c73852; }
  .primary-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .secondary-btn {
    padding: 0.4rem 0.8rem;
    background: #0f3460;
    color: #e0e0e0;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
  }

  .secondary-btn:hover { background: #16213e; }
  .secondary-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .remove-btn {
    padding: 0.25rem 0.6rem;
    background: transparent;
    color: #e94560;
    border: 1px solid #e94560;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.78rem;
  }

  .remove-btn:hover { background: #e94560; color: white; }
  .remove-btn:disabled { opacity: 0.4; cursor: not-allowed; }

  .ghost-btn {
    padding: 0.25rem 0.6rem;
    background: transparent;
    color: #a0a0c0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.78rem;
  }

  .ghost-btn:hover { background: #0f3460; color: #e0e0e0; }
  .ghost-btn:disabled { opacity: 0.4; cursor: not-allowed; }

  .muted {
    color: #a0a0c0;
    font-style: italic;
    font-size: 0.85rem;
  }
</style>
