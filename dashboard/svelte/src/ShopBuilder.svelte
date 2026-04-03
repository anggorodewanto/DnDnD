<script>
  /**
   * ShopBuilder — DM dashboard component for creating and managing shop templates.
   *
   * Props:
   *   campaignId {string} — campaign UUID
   *   onback {function} — callback to return to the shop list
   *   shopId {string|null} — if editing an existing shop
   */
  import { getShop, createShop, updateShop, addShopItem, removeShopItem, postShopToDiscord } from '$lib/api.js';
  import ItemPicker from './ItemPicker.svelte';

  let {
    campaignId = '',
    shopId = null,
    onback = () => {}
  } = $props();

  let shopName = $state('');
  let shopDescription = $state('');
  let shopItems = $state([]);
  let saving = $state(false);
  let posting = $state(false);
  let postResult = $state('');
  let error = $state('');
  let currentShopId = $state(shopId);
  let showItemPicker = $state(false);

  async function loadShop() {
    if (!currentShopId) return;
    try {
      const data = await getShop(campaignId, currentShopId);
      shopName = data.shop.name;
      shopDescription = data.shop.description || '';
      shopItems = data.items || [];
    } catch (e) {
      error = 'Failed to load shop: ' + e.message;
    }
  }

  async function saveShop() {
    if (!shopName.trim()) {
      error = 'Shop name is required';
      return;
    }
    saving = true;
    error = '';
    try {
      if (currentShopId) {
        await updateShop(campaignId, currentShopId, {
          name: shopName.trim(),
          description: shopDescription
        });
      } else {
        const created = await createShop(campaignId, {
          name: shopName.trim(),
          description: shopDescription
        });
        currentShopId = created.id;
      }
    } catch (e) {
      error = 'Failed to save shop: ' + e.message;
    } finally {
      saving = false;
    }
  }

  async function addSelectedItems(items) {
    if (!currentShopId) {
      // Must save shop first
      await saveShop();
      if (!currentShopId) return;
    }
    error = '';
    for (const item of items) {
      try {
        const created = await addShopItem(campaignId, currentShopId, {
          item_id: item.id || '',
          name: item.name,
          description: item.description || item.narrative || '',
          price_gp: item.price_override || item.cost_gp || 0,
          quantity: item.quantity || 1,
          type: item.type || 'other'
        });
        shopItems = [...shopItems, created];
      } catch (e) {
        error = 'Failed to add item: ' + e.message;
      }
    }
    showItemPicker = false;
  }

  async function removeItem(itemId) {
    try {
      await removeShopItem(campaignId, currentShopId, itemId);
      shopItems = shopItems.filter(i => i.id !== itemId);
    } catch (e) {
      error = 'Failed to remove item: ' + e.message;
    }
  }

  async function postToDiscord() {
    if (!currentShopId) return;
    posting = true;
    postResult = '';
    error = '';
    try {
      await postShopToDiscord(campaignId, currentShopId);
      postResult = 'Posted to #the-story!';
    } catch (e) {
      error = 'Failed to post: ' + e.message;
    } finally {
      posting = false;
    }
  }

  $effect(() => {
    if (shopId) {
      currentShopId = shopId;
      loadShop();
    }
  });
</script>

<div class="shop-builder">
  <div class="shop-header">
    <h2>{currentShopId ? 'Edit Shop' : 'New Shop'}</h2>
    <button class="back-btn" onclick={onback}>Back to Shops</button>
  </div>

  {#if error}
    <div class="error">{error}</div>
  {/if}
  {#if postResult}
    <div class="success">{postResult}</div>
  {/if}

  <div class="shop-form">
    <label>
      Shop Name
      <input type="text" bind:value={shopName} placeholder="e.g. Ironforge Smithy" />
    </label>
    <label>
      Description
      <textarea bind:value={shopDescription} placeholder="Optional shop description..."></textarea>
    </label>
    <div class="form-actions">
      <button class="save-btn" onclick={saveShop} disabled={saving}>
        {saving ? 'Saving...' : 'Save Shop'}
      </button>
      {#if currentShopId}
        <button class="post-btn" onclick={postToDiscord} disabled={posting}>
          {posting ? 'Posting...' : 'Post to #the-story'}
        </button>
      {/if}
    </div>
  </div>

  <div class="shop-inventory">
    <div class="inventory-header">
      <h3>Shop Inventory ({shopItems.length} items)</h3>
      <button class="add-btn" onclick={() => showItemPicker = !showItemPicker}>
        {showItemPicker ? 'Close Picker' : '+ Add Items'}
      </button>
    </div>

    {#if showItemPicker}
      <ItemPicker {campaignId} onselect={addSelectedItems} />
    {/if}

    {#if shopItems.length === 0}
      <p class="empty">No items yet. Add items using the Item Picker above.</p>
    {:else}
      <div class="items-list">
        {#each shopItems as item}
          <div class="shop-item">
            <div class="item-details">
              <strong>{item.name}</strong>
              {#if item.price_gp > 0}
                <span class="price">{item.price_gp} gp</span>
              {/if}
              <span class="item-type">{item.type}</span>
              {#if item.quantity > 1}
                <span class="qty">x{item.quantity}</span>
              {/if}
              {#if item.description}
                <p class="item-desc">{item.description}</p>
              {/if}
            </div>
            <button class="remove-btn" onclick={() => removeItem(item.id)}>Remove</button>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .shop-builder {
    max-width: 900px;
  }

  .shop-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .shop-header h2 {
    color: #e94560;
    margin: 0;
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

  .shop-form {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    padding: 1rem;
    margin-bottom: 1rem;
  }

  .shop-form label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.85rem;
    color: #a0a0c0;
    margin-bottom: 0.75rem;
  }

  .shop-form input, .shop-form textarea {
    padding: 0.5rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }

  .shop-form textarea {
    min-height: 60px;
    resize: vertical;
  }

  .form-actions {
    display: flex;
    gap: 0.5rem;
  }

  .save-btn {
    padding: 0.5rem 1rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }

  .save-btn:hover { background: #c73852; }
  .save-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .post-btn {
    padding: 0.5rem 1rem;
    background: #5865F2;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }

  .post-btn:hover { background: #4752c4; }
  .post-btn:disabled { opacity: 0.5; cursor: not-allowed; }

  .shop-inventory {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    padding: 1rem;
  }

  .inventory-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .inventory-header h3 {
    color: #e94560;
    margin: 0;
  }

  .add-btn {
    padding: 0.4rem 0.75rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }

  .add-btn:hover { background: #c73852; }

  .items-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .shop-item {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 0.75rem;
    background: #1a1a2e;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }

  .item-details {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .price {
    color: #ffd700;
    font-weight: bold;
  }

  .item-type {
    font-size: 0.75rem;
    padding: 0.15rem 0.5rem;
    background: #0f3460;
    border-radius: 3px;
    color: #a0a0c0;
  }

  .qty {
    font-size: 0.85rem;
    color: #ffd700;
  }

  .item-desc {
    width: 100%;
    font-size: 0.85rem;
    color: #a0a0c0;
    margin-top: 0.25rem;
  }

  .remove-btn {
    padding: 0.2rem 0.5rem;
    background: transparent;
    color: #e94560;
    border: 1px solid #e94560;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.8rem;
    white-space: nowrap;
  }

  .remove-btn:hover {
    background: #e94560;
    color: white;
  }

  .back-btn {
    padding: 0.5rem 1rem;
    background: #16213e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }

  .back-btn:hover { background: #0f3460; }

  .empty {
    text-align: center;
    padding: 1rem;
    color: #a0a0c0;
  }
</style>
