<script>
  import MapEditor from './MapEditor.svelte';
  import MapList from './MapList.svelte';
  import EncounterBuilder from './EncounterBuilder.svelte';
  import EncounterList from './EncounterList.svelte';
  import TurnBuilder from './TurnBuilder.svelte';
  import ShopList from './ShopList.svelte';
  import ShopBuilder from './ShopBuilder.svelte';
  import CombatManager from './CombatManager.svelte';

  let currentView = $state('list');
  let editingMapId = $state(null);
  let editingEncounterId = $state(null);
  let editingShopId = $state(null);
  let turnBuilderEncounterId = $state(null);
  let turnBuilderCombatantId = $state(null);
  let turnBuilderCombatantName = $state(null);
  // For demo purposes, use a fixed campaign ID. In production this comes from session.
  let campaignId = $state('00000000-0000-0000-0000-000000000001');

  // Determine initial view from URL hash
  function getInitialView() {
    const hash = window.location.hash;
    if (hash === '#combat') return 'combat';
    if (hash === '#encounters') return 'encounter-list';
    if (hash === '#encounter-new') return 'encounter-editor';
    if (hash.startsWith('#turn-builder')) return 'turn-builder';
    if (hash === '#shops') return 'shop-list';
    if (hash === '#shop-new') return 'shop-editor';
    return 'list';
  }

  currentView = getInitialView();

  function onCreateNew() {
    editingMapId = null;
    currentView = 'editor';
  }

  function onEditMap(id) {
    editingMapId = id;
    currentView = 'editor';
  }

  function onBack() {
    currentView = 'list';
    editingMapId = null;
  }

  function onCreateEncounter() {
    editingEncounterId = null;
    currentView = 'encounter-editor';
  }

  function onEditEncounter(id) {
    editingEncounterId = id;
    currentView = 'encounter-editor';
  }

  function onBackFromEncounter() {
    currentView = 'encounter-list';
    editingEncounterId = null;
  }

  function onShowEncounters() {
    currentView = 'encounter-list';
  }

  function onShowMaps() {
    currentView = 'list';
  }

  function onOpenTurnBuilder(encId, combId, combName) {
    turnBuilderEncounterId = encId;
    turnBuilderCombatantId = combId;
    turnBuilderCombatantName = combName;
    currentView = 'turn-builder';
  }

  function onCloseTurnBuilder() {
    currentView = 'encounter-list';
    turnBuilderEncounterId = null;
    turnBuilderCombatantId = null;
    turnBuilderCombatantName = null;
  }

  function onShowShops() {
    currentView = 'shop-list';
  }

  function onCreateShop() {
    editingShopId = null;
    currentView = 'shop-editor';
  }

  function onEditShop(id) {
    editingShopId = id;
    currentView = 'shop-editor';
  }

  function onBackFromShop() {
    currentView = 'shop-list';
    editingShopId = null;
  }

  function onShowCombat() {
    currentView = 'combat';
  }
</script>

<main>
  <header>
    {#if currentView === 'combat'}
      <h1>Combat Manager</h1>
    {:else if currentView === 'list' || currentView === 'editor'}
      <h1>Map Editor</h1>
    {:else if currentView === 'shop-list' || currentView === 'shop-editor'}
      <h1>Shops & Merchants</h1>
    {:else}
      <h1>Encounter Builder</h1>
    {/if}

    <nav class="view-nav">
      <button class:active={currentView === 'combat'} onclick={onShowCombat}>Combat</button>
      <button class:active={currentView === 'list' || currentView === 'editor'} onclick={onShowMaps}>Maps</button>
      <button class:active={currentView === 'encounter-list' || currentView === 'encounter-editor'} onclick={onShowEncounters}>Encounters</button>
      <button class:active={currentView === 'turn-builder'} onclick={() => currentView = 'turn-builder'}>Turn Builder</button>
      <button class:active={currentView === 'shop-list' || currentView === 'shop-editor'} onclick={onShowShops}>Shops</button>
    </nav>

    {#if currentView === 'editor'}
      <button class="back-btn" onclick={onBack}>Back to Map List</button>
    {/if}
    {#if currentView === 'encounter-editor'}
      <button class="back-btn" onclick={onBackFromEncounter}>Back to Encounter List</button>
    {/if}
    {#if currentView === 'shop-editor'}
      <button class="back-btn" onclick={onBackFromShop}>Back to Shop List</button>
    {/if}
  </header>

  {#if currentView === 'combat'}
    <CombatManager {campaignId} />
  {:else if currentView === 'list'}
    <MapList {campaignId} oncreate={onCreateNew} onedit={onEditMap} />
  {:else if currentView === 'editor'}
    <MapEditor {campaignId} mapId={editingMapId} onback={onBack} />
  {:else if currentView === 'encounter-list'}
    <EncounterList {campaignId} oncreate={onCreateEncounter} onedit={onEditEncounter} />
  {:else if currentView === 'encounter-editor'}
    <EncounterBuilder {campaignId} encounterId={editingEncounterId} onback={onBackFromEncounter} />
  {:else if currentView === 'turn-builder'}
    <TurnBuilder
      encounterId={turnBuilderEncounterId}
      combatantId={turnBuilderCombatantId}
      combatantName={turnBuilderCombatantName}
      onclose={onCloseTurnBuilder}
    />
  {:else if currentView === 'shop-list'}
    <ShopList {campaignId} oncreate={onCreateShop} onedit={onEditShop} />
  {:else if currentView === 'shop-editor'}
    <ShopBuilder {campaignId} shopId={editingShopId} onback={onBackFromShop} />
  {/if}
</main>

<style>
  :global(body) {
    margin: 0;
    font-family: system-ui, -apple-system, sans-serif;
    background: #1a1a2e;
    color: #e0e0e0;
  }

  main {
    padding: 1rem;
    max-width: 100%;
  }

  header {
    display: flex;
    align-items: center;
    gap: 1rem;
    margin-bottom: 1rem;
  }

  header h1 {
    color: #e94560;
    margin: 0;
  }

  .view-nav {
    display: flex;
    gap: 0.25rem;
  }

  .view-nav button {
    padding: 0.5rem 1rem;
    background: #16213e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }

  .view-nav button:hover {
    background: #0f3460;
  }

  .view-nav button.active {
    background: #e94560;
    border-color: #e94560;
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

  .back-btn:hover {
    background: #0f3460;
  }
</style>
