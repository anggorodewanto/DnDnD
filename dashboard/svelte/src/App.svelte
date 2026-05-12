<script>
  import MapEditor from './MapEditor.svelte';
  import MapList from './MapList.svelte';
  import EncounterBuilder from './EncounterBuilder.svelte';
  import EncounterList from './EncounterList.svelte';
  import TurnBuilder from './TurnBuilder.svelte';
  import ShopList from './ShopList.svelte';
  import ShopBuilder from './ShopBuilder.svelte';
  import CombatManager from './CombatManager.svelte';
  import NarratePanel from './NarratePanel.svelte';
  import MobileShell from './MobileShell.svelte';
  import MobileRedirect from './MobileRedirect.svelte';
  import HomebrewEditor from './HomebrewEditor.svelte';
  import CharacterOverview from './CharacterOverview.svelte';
  import StatBlockLibrary from './StatBlockLibrary.svelte';
  import MessagePlayerPanel from './MessagePlayerPanel.svelte';
  import Open5eSourcesPanel from './Open5eSourcesPanel.svelte';
  import DMQueuePanel from './DMQueuePanel.svelte';
  import { isMobileViewport, isDesktopOnly } from './lib/layout.js';

  let innerWidth = $state(typeof window !== 'undefined' ? window.innerWidth : 1920);
  let currentView = $state('list');
  let editingMapId = $state(null);
  let editingEncounterId = $state(null);
  let editingShopId = $state(null);
  let turnBuilderEncounterId = $state(null);
  let turnBuilderCombatantId = $state(null);
  let turnBuilderCombatantName = $state(null);
  // med-39 / Phase 21a: campaign id is fetched from /api/me on boot so the
  // Svelte panels operate against the authenticated DM's actual campaign
  // instead of a hard-coded placeholder UUID. Falls back to '' (empty) when
  // the user has no active campaign yet so panels can render an empty state.
  let campaignId = $state('');

  // Fetch the active campaign id on boot. Best-effort: any network /
  // unauthenticated response leaves campaignId='' so the SPA still renders.
  $effect(() => {
    if (typeof window === 'undefined') return;
    fetch('/api/me', { credentials: 'same-origin' })
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (data && typeof data.campaign_id === 'string' && data.campaign_id !== '') {
          campaignId = data.campaign_id;
        }
      })
      .catch(() => {
        /* swallow: campaignId stays empty so panels can render an empty state */
      });
  });

  // Determine initial view from URL hash
  function getInitialView() {
    const hash = window.location.hash;
    if (hash === '#combat') return 'combat';
    if (hash === '#encounters') return 'encounter-list';
    if (hash === '#encounter-new') return 'encounter-editor';
    if (hash.startsWith('#turn-builder')) return 'turn-builder';
    if (hash === '#shops') return 'shop-list';
    if (hash === '#shop-new') return 'shop-editor';
    if (hash === '#narrate') return 'narrate';
    if (hash === '#homebrew') return 'homebrew';
    if (hash === '#party') return 'party';
    if (hash === '#stat-block-library') return 'stat-block-library';
    if (hash === '#message-player') return 'message-player';
    if (hash === '#open5e-sources') return 'open5e-sources';
    if (hash === '#dm-queue') return 'dm-queue';
    return 'list';
  }

  currentView = getInitialView();

  // Track viewport width so we can swap to the mobile-lite shell (Phase 102).
  $effect(() => {
    if (typeof window === 'undefined') return;
    const handler = () => {
      innerWidth = window.innerWidth;
    };
    window.addEventListener('resize', handler);
    return () => window.removeEventListener('resize', handler);
  });

  // Map the internal desktop `currentView` tokens to the spec's desktop-only
  // feature ids used by the layout helpers.
  function currentDesktopOnlyID() {
    if (currentView === 'list' || currentView === 'editor') return 'map-editor';
    if (currentView === 'encounter-list' || currentView === 'encounter-editor') return 'encounter-builder';
    if (currentView === 'combat') return 'combat-workspace';
    if (currentView === 'stat-block-library') return 'stat-block-library';
    return null;
  }

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

  function onShowNarrate() {
    currentView = 'narrate';
  }

  function onShowHomebrew() {
    currentView = 'homebrew';
  }

  function onShowParty() {
    currentView = 'party';
  }

  function onShowStatBlockLibrary() {
    currentView = 'stat-block-library';
  }

  function onShowMessagePlayer() {
    currentView = 'message-player';
  }

  function onShowOpen5eSources() {
    currentView = 'open5e-sources';
  }

  function onShowDMQueue() {
    currentView = 'dm-queue';
  }
</script>

{#if isMobileViewport(innerWidth)}
  {#if isDesktopOnly(currentDesktopOnlyID())}
    <MobileRedirect view={currentDesktopOnlyID()} />
  {/if}
  <MobileShell {campaignId} />
{:else}
<main>
  <header>
    {#if currentView === 'combat'}
      <h1>Combat Manager</h1>
    {:else if currentView === 'list' || currentView === 'editor'}
      <h1>Map Editor</h1>
    {:else if currentView === 'shop-list' || currentView === 'shop-editor'}
      <h1>Shops & Merchants</h1>
    {:else if currentView === 'narrate'}
      <h1>Narrate</h1>
    {:else if currentView === 'homebrew'}
      <h1>Homebrew Editor</h1>
    {:else if currentView === 'party'}
      <h1>Party Overview</h1>
    {:else if currentView === 'stat-block-library'}
      <h1>Stat Block Library</h1>
    {:else if currentView === 'message-player'}
      <h1>Message Player</h1>
    {:else if currentView === 'open5e-sources'}
      <h1>Open5e Sources</h1>
    {:else if currentView === 'dm-queue'}
      <h1>DM Queue</h1>
    {:else}
      <h1>Encounter Builder</h1>
    {/if}

    <nav class="view-nav">
      <button class:active={currentView === 'combat'} onclick={onShowCombat}>Combat</button>
      <button class:active={currentView === 'list' || currentView === 'editor'} onclick={onShowMaps}>Maps</button>
      <button class:active={currentView === 'encounter-list' || currentView === 'encounter-editor'} onclick={onShowEncounters}>Encounters</button>
      <button class:active={currentView === 'turn-builder'} onclick={() => currentView = 'turn-builder'}>Turn Builder</button>
      <button class:active={currentView === 'shop-list' || currentView === 'shop-editor'} onclick={onShowShops}>Shops</button>
      <button class:active={currentView === 'narrate'} onclick={onShowNarrate}>Narrate</button>
      <button class:active={currentView === 'homebrew'} onclick={onShowHomebrew}>Homebrew</button>
      <button class:active={currentView === 'party'} onclick={onShowParty}>Party</button>
      <button class:active={currentView === 'stat-block-library'} onclick={onShowStatBlockLibrary}>Stat Block Library</button>
      <button class:active={currentView === 'message-player'} onclick={onShowMessagePlayer}>Message Player</button>
      <button class:active={currentView === 'open5e-sources'} onclick={onShowOpen5eSources}>Open5e Sources</button>
      <button class:active={currentView === 'dm-queue'} onclick={onShowDMQueue}>DM Queue</button>
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
  {:else if currentView === 'narrate'}
    <NarratePanel {campaignId} />
  {:else if currentView === 'homebrew'}
    <HomebrewEditor {campaignId} />
  {:else if currentView === 'party'}
    <CharacterOverview {campaignId} />
  {:else if currentView === 'stat-block-library'}
    <StatBlockLibrary {campaignId} />
  {:else if currentView === 'message-player'}
    <MessagePlayerPanel {campaignId} />
  {:else if currentView === 'open5e-sources'}
    <Open5eSourcesPanel {campaignId} />
  {:else if currentView === 'dm-queue'}
    <DMQueuePanel />
  {/if}
</main>
{/if}

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
