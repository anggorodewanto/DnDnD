<script>
  /**
   * ShopList — displays all shops for a campaign.
   *
   * Props:
   *   campaignId {string} — campaign UUID
   *   oncreate {function} — callback to create a new shop
   *   onedit {function(shopId)} — callback to edit a shop
   */
  import { listShops, deleteShop } from '$lib/api.js';

  let {
    campaignId = '',
    oncreate = () => {},
    onedit = () => {}
  } = $props();

  let shops = $state([]);
  let loading = $state(true);
  let error = $state('');

  async function fetchShops() {
    loading = true;
    error = '';
    try {
      shops = await listShops(campaignId);
    } catch (e) {
      error = 'Failed to load shops: ' + e.message;
      shops = [];
    } finally {
      loading = false;
    }
  }

  async function handleDelete(shopId, shopName) {
    if (!confirm(`Delete "${shopName}"? This cannot be undone.`)) return;
    try {
      await deleteShop(campaignId, shopId);
      shops = shops.filter(s => s.id !== shopId);
    } catch (e) {
      error = 'Failed to delete shop: ' + e.message;
    }
  }

  $effect(() => {
    fetchShops();
  });
</script>

<div class="shop-list">
  <div class="list-header">
    <h2>Shops & Merchants</h2>
    <button class="create-btn" onclick={oncreate}>+ New Shop</button>
  </div>

  {#if error}
    <div class="error">{error}</div>
  {/if}

  {#if loading}
    <p class="loading">Loading shops...</p>
  {:else if shops.length === 0}
    <p class="empty">No shops yet. Create one to get started!</p>
  {:else}
    <div class="shops">
      {#each shops as shop}
        <div class="shop-card">
          <div class="shop-info">
            <h3>{shop.name}</h3>
            {#if shop.description}
              <p class="shop-desc">{shop.description}</p>
            {/if}
          </div>
          <div class="shop-actions">
            <button class="edit-btn" onclick={() => onedit(shop.id)}>Edit</button>
            <button class="delete-btn" onclick={() => handleDelete(shop.id, shop.name)}>Delete</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .shop-list {
    max-width: 900px;
  }

  .list-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .list-header h2 {
    color: #e94560;
    margin: 0;
  }

  .create-btn {
    padding: 0.5rem 1rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }

  .create-btn:hover { background: #c73852; }

  .error {
    background: #3e1a1a;
    color: #ff6b6b;
    padding: 0.75rem;
    border-radius: 4px;
    margin-bottom: 1rem;
  }

  .shops {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .shop-card {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
  }

  .shop-card:hover {
    border-color: #e94560;
  }

  .shop-info h3 {
    margin: 0 0 0.25rem;
    color: #e0e0e0;
  }

  .shop-desc {
    margin: 0;
    font-size: 0.85rem;
    color: #a0a0c0;
  }

  .shop-actions {
    display: flex;
    gap: 0.5rem;
  }

  .edit-btn {
    padding: 0.4rem 0.75rem;
    background: #0f3460;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }

  .edit-btn:hover { background: #1a4a8a; }

  .delete-btn {
    padding: 0.4rem 0.75rem;
    background: transparent;
    color: #e94560;
    border: 1px solid #e94560;
    border-radius: 4px;
    cursor: pointer;
  }

  .delete-btn:hover {
    background: #e94560;
    color: white;
  }

  .loading, .empty {
    text-align: center;
    padding: 2rem;
    color: #a0a0c0;
  }
</style>
