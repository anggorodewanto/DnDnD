<script>
  /**
   * ItemPicker — shared dashboard component for item selection.
   * Used in loot pool management, shop creation, inventory grants.
   *
   * Props:
   *   campaignId {string} — campaign UUID
   *   encounterID {string|null} — encounter UUID (enables creature inventory tab)
   *   onselect {function} — callback receiving the selected items array
   */
  let {
    campaignId = '',
    encounterId = null,
    onselect = () => {}
  } = $props();

  let searchQuery = $state('');
  let category = $state('');
  let searchResults = $state([]);
  let selectedItems = $state([]);
  let activeTab = $state('search');
  let creatureInventories = $state([]);
  let loading = $state(false);
  let customForm = $state({ name: '', description: '', quantity: 1, gold_value: 0 });

  let debounceTimer = null;

  async function fetchItems() {
    loading = true;
    try {
      const params = new URLSearchParams();
      if (searchQuery) params.set('q', searchQuery);
      if (category) params.set('category', category);
      const res = await fetch(`/api/campaigns/${campaignId}/items/search?${params}`);
      if (!res.ok) throw new Error('Search failed');
      searchResults = await res.json();
    } catch (e) {
      console.error('Item search failed:', e);
      searchResults = [];
    } finally {
      loading = false;
    }
  }

  async function fetchCreatureInventories() {
    if (!encounterId) return;
    try {
      const res = await fetch(`/api/campaigns/${campaignId}/encounters/${encounterId}/creature-inventories/`);
      if (!res.ok) throw new Error('Failed to load creature inventories');
      const data = await res.json();
      creatureInventories = data.creatures || [];
    } catch (e) {
      console.error('Creature inventory fetch failed:', e);
      creatureInventories = [];
    }
  }

  function onSearchInput() {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(fetchItems, 300);
  }

  function onCategoryChange() {
    fetchItems();
  }

  function addItem(item) {
    const existing = selectedItems.find(s => s.id === item.id && s.type === item.type);
    if (existing) return;
    selectedItems = [...selectedItems, {
      ...item,
      narrative: '',
      price_override: item.cost_gp || 0,
      quantity: 1
    }];
    onselect(selectedItems);
  }

  function addCreatureItem(creatureName, item) {
    const id = `creature-${creatureName}-${item.item_id || item.name}`;
    const existing = selectedItems.find(s => s.id === id);
    if (existing) return;
    selectedItems = [...selectedItems, {
      id: id,
      name: item.name,
      type: item.type || 'other',
      description: '',
      cost_gp: 0,
      narrative: '',
      price_override: 0,
      quantity: item.quantity || 1,
      source: creatureName
    }];
    onselect(selectedItems);
  }

  function addCustomItem() {
    if (!customForm.name.trim()) return;
    const id = `custom-${Date.now()}`;
    selectedItems = [...selectedItems, {
      id: id,
      name: customForm.name.trim(),
      type: 'custom',
      description: customForm.description,
      cost_gp: customForm.gold_value,
      narrative: '',
      price_override: customForm.gold_value,
      quantity: customForm.quantity || 1
    }];
    customForm = { name: '', description: '', quantity: 1, gold_value: 0 };
    onselect(selectedItems);
  }

  function removeItem(index) {
    selectedItems = selectedItems.filter((_, i) => i !== index);
    onselect(selectedItems);
  }

  function updateNarrative(index, value) {
    selectedItems[index].narrative = value;
    selectedItems = [...selectedItems];
    onselect(selectedItems);
  }

  function updatePriceOverride(index, value) {
    selectedItems[index].price_override = parseInt(value) || 0;
    selectedItems = [...selectedItems];
    onselect(selectedItems);
  }

  function updateQuantity(index, value) {
    selectedItems[index].quantity = parseInt(value) || 1;
    selectedItems = [...selectedItems];
    onselect(selectedItems);
  }

  // Load initial data
  $effect(() => {
    fetchItems();
    if (encounterId) {
      fetchCreatureInventories();
    }
  });
</script>

<div class="item-picker">
  <div class="tabs">
    <button class:active={activeTab === 'search'} onclick={() => activeTab = 'search'}>Search Items</button>
    <button class:active={activeTab === 'custom'} onclick={() => activeTab = 'custom'}>Add Custom</button>
    {#if encounterId}
      <button class:active={activeTab === 'creatures'} onclick={() => { activeTab = 'creatures'; fetchCreatureInventories(); }}>
        Creature Loot
      </button>
    {/if}
  </div>

  {#if activeTab === 'search'}
    <div class="search-bar">
      <input
        type="text"
        placeholder="Search items..."
        bind:value={searchQuery}
        oninput={onSearchInput}
      />
      <select bind:value={category} onchange={onCategoryChange}>
        <option value="">All Categories</option>
        <option value="weapons">Weapons</option>
        <option value="armor">Armor</option>
        <option value="magic_items">Magic Items</option>
      </select>
    </div>

    <div class="results">
      {#if loading}
        <p class="loading">Loading...</p>
      {:else if searchResults.length === 0}
        <p class="empty">No items found</p>
      {:else}
        {#each searchResults as item}
          <div class="result-item">
            <div class="item-info">
              <strong>{item.name}</strong>
              <span class="item-type">{item.type}</span>
              {#if item.description}
                <p class="item-desc">{item.description}</p>
              {/if}
            </div>
            <button class="add-btn" onclick={() => addItem(item)}>+ Add</button>
          </div>
        {/each}
      {/if}
    </div>
  {:else if activeTab === 'custom'}
    <div class="custom-form">
      <label>
        Name
        <input type="text" bind:value={customForm.name} placeholder="e.g. Mysterious Key" />
      </label>
      <label>
        Description
        <textarea bind:value={customForm.description} placeholder="Optional flavor text"></textarea>
      </label>
      <div class="custom-row">
        <label>
          Quantity
          <input type="number" bind:value={customForm.quantity} min="1" />
        </label>
        <label>
          Gold Value
          <input type="number" bind:value={customForm.gold_value} min="0" />
        </label>
      </div>
      <button class="add-btn" onclick={addCustomItem}>Add Custom Item</button>
    </div>
  {:else if activeTab === 'creatures'}
    <div class="creature-inventories">
      {#if creatureInventories.length === 0}
        <p class="empty">No defeated creatures with inventory found</p>
      {:else}
        {#each creatureInventories as creature}
          <div class="creature-section">
            <h4>{creature.name} {#if creature.gold > 0}<span class="gold">({creature.gold} gp)</span>{/if}</h4>
            {#if creature.items.length === 0}
              <p class="empty">No items</p>
            {:else}
              {#each creature.items as item}
                <div class="result-item">
                  <div class="item-info">
                    <strong>{item.name}</strong>
                    {#if item.quantity > 1}<span class="qty">x{item.quantity}</span>{/if}
                    <span class="item-type">{item.type}</span>
                  </div>
                  <button class="add-btn" onclick={() => addCreatureItem(creature.name, item)}>+ Add</button>
                </div>
              {/each}
            {/if}
          </div>
        {/each}
      {/if}
    </div>
  {/if}

  {#if selectedItems.length > 0}
    <div class="selected-items">
      <h3>Selected Items ({selectedItems.length})</h3>
      {#each selectedItems as item, index}
        <div class="selected-item">
          <div class="selected-header">
            <strong>{item.name}</strong>
            {#if item.source}<span class="source">from {item.source}</span>{/if}
            <button class="remove-btn" onclick={() => removeItem(index)}>Remove</button>
          </div>
          <div class="selected-fields">
            <label>
              Qty
              <input type="number" value={item.quantity} min="1"
                oninput={(e) => updateQuantity(index, e.target.value)} />
            </label>
            <label>
              Price (gp)
              <input type="number" value={item.price_override} min="0"
                oninput={(e) => updatePriceOverride(index, e.target.value)} />
            </label>
            <label class="narrative-label">
              Narrative
              <input type="text" value={item.narrative} placeholder="DM flavor note..."
                oninput={(e) => updateNarrative(index, e.target.value)} />
            </label>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .item-picker {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    padding: 1rem;
  }

  .tabs {
    display: flex;
    gap: 0.25rem;
    margin-bottom: 1rem;
  }

  .tabs button {
    padding: 0.5rem 1rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }

  .tabs button:hover { background: #0f3460; }
  .tabs button.active { background: #e94560; border-color: #e94560; color: white; }

  .search-bar {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 0.75rem;
  }

  .search-bar input {
    flex: 1;
    padding: 0.5rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }

  .search-bar select {
    padding: 0.5rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }

  .results, .creature-inventories {
    max-height: 300px;
    overflow-y: auto;
  }

  .result-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.5rem;
    border-bottom: 1px solid #0f3460;
  }

  .result-item:hover { background: #1a1a2e; }

  .item-info {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .item-type {
    font-size: 0.75rem;
    padding: 0.15rem 0.5rem;
    background: #0f3460;
    border-radius: 3px;
    color: #a0a0c0;
  }

  .item-desc {
    font-size: 0.85rem;
    color: #a0a0c0;
    width: 100%;
    margin-top: 0.25rem;
  }

  .add-btn {
    padding: 0.25rem 0.75rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    white-space: nowrap;
  }

  .add-btn:hover { background: #c73852; }

  .custom-form {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .custom-form label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.85rem;
    color: #a0a0c0;
  }

  .custom-form input, .custom-form textarea {
    padding: 0.5rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }

  .custom-form textarea {
    min-height: 60px;
    resize: vertical;
  }

  .custom-row {
    display: flex;
    gap: 1rem;
  }

  .custom-row label { flex: 1; }

  .creature-section {
    margin-bottom: 1rem;
  }

  .creature-section h4 {
    color: #e94560;
    margin-bottom: 0.5rem;
  }

  .gold { color: #ffd700; font-weight: normal; }

  .qty {
    font-size: 0.85rem;
    color: #ffd700;
  }

  .selected-items {
    margin-top: 1rem;
    border-top: 1px solid #0f3460;
    padding-top: 1rem;
  }

  .selected-items h3 {
    color: #e94560;
    margin-bottom: 0.75rem;
  }

  .selected-item {
    background: #1a1a2e;
    border: 1px solid #0f3460;
    border-radius: 4px;
    padding: 0.75rem;
    margin-bottom: 0.5rem;
  }

  .selected-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.5rem;
  }

  .source {
    font-size: 0.8rem;
    color: #a0a0c0;
  }

  .remove-btn {
    margin-left: auto;
    padding: 0.2rem 0.5rem;
    background: transparent;
    color: #e94560;
    border: 1px solid #e94560;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.8rem;
  }

  .remove-btn:hover {
    background: #e94560;
    color: white;
  }

  .selected-fields {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .selected-fields label {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    font-size: 0.8rem;
    color: #a0a0c0;
  }

  .selected-fields input {
    padding: 0.35rem;
    background: #16213e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    width: 80px;
  }

  .narrative-label {
    flex: 1;
  }

  .narrative-label input {
    width: 100% !important;
  }

  .loading, .empty {
    text-align: center;
    padding: 1rem;
    color: #a0a0c0;
  }
</style>
