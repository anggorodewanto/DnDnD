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
  import HomebrewEditor from './HomebrewEditor.svelte';
  import CharacterOverview from './CharacterOverview.svelte';
  import StatBlockLibrary from './StatBlockLibrary.svelte';
  import MessagePlayerPanel from './MessagePlayerPanel.svelte';
  import Open5eSourcesPanel from './Open5eSourcesPanel.svelte';
  import DMQueuePanel from './DMQueuePanel.svelte';
  import LootPoolPanel from './LootPoolPanel.svelte';
  import LevelUpPanel from './LevelUpPanel.svelte';
  import CharacterApprovalQueue from './CharacterApprovalQueue.svelte';
  import CampaignsPage from './CampaignsPage.svelte';
  import HomePanel from './HomePanel.svelte';
  import ErrorsPanel from './ErrorsPanel.svelte';
  import ExplorationPanel from './ExplorationPanel.svelte';
  import CharCreatePanel from './CharCreatePanel.svelte';
  import { resolveDashboardViewFromHash } from './lib/dashboardRouter.js';
  import { getCurrentUser } from './lib/api.js';
  import {
    dashboardNavItems,
    dashboardViewTitle,
    isDashboardNavItemActive,
  } from './lib/dashboardNavigation.js';

  let currentView = $state('home');
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
  let drawerOpen = $state(false);

  async function refreshCurrentCampaign() {
    if (typeof window === 'undefined') return;
    try {
      const data = await getCurrentUser();
      campaignId = data && typeof data.campaign_id === 'string' ? data.campaign_id : '';
    } catch {
      /* swallow: campaignId stays empty so panels can render an empty state */
    }
  }

  // Fetch the active campaign id on boot. Best-effort: any network /
  // unauthenticated response leaves campaignId='' so the SPA still renders.
  $effect(() => {
    refreshCurrentCampaign();
  });

  // Determine initial view from URL hash
  function getInitialView() {
    return resolveDashboardViewFromHash(window.location.hash);
  }

  currentView = getInitialView();

  $effect(() => {
    if (typeof window === 'undefined') return;
    const handler = () => {
      currentView = getInitialView();
    };
    window.addEventListener('hashchange', handler);
    return () => window.removeEventListener('hashchange', handler);
  });

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

  function navigateTo(item) {
    currentView = item.view;

    if (item.view !== 'editor') editingMapId = null;
    if (item.view !== 'encounter-editor') editingEncounterId = null;
    if (item.view !== 'shop-editor') editingShopId = null;
    if (item.view !== 'turn-builder') {
      turnBuilderEncounterId = null;
      turnBuilderCombatantId = null;
      turnBuilderCombatantName = null;
    }

    if (typeof window !== 'undefined' && item.hash && window.location.hash !== item.hash) {
      window.location.hash = item.hash;
    }

    drawerOpen = false;
  }
</script>

<div class="desktop-shell">
  <aside class="sidebar" class:open={drawerOpen} aria-label="Dashboard navigation">
    <a class="brand" href="#maps" onclick={(event) => { event.preventDefault(); navigateTo(dashboardNavItems[0]); }}>
      <span class="brand-mark">D</span>
      <span>DnDnD</span>
    </a>
    <nav class="sidebar-nav">
      {#each dashboardNavItems as item}
        <a
          href={item.hash}
          class:active={isDashboardNavItemActive(item, currentView)}
          onclick={(event) => { event.preventDefault(); navigateTo(item); }}
        >
          {item.label}
        </a>
      {/each}
    </nav>
  </aside>

  {#if drawerOpen}
    <button class="drawer-backdrop" aria-label="Close menu" onclick={() => (drawerOpen = false)}></button>
  {/if}

  <main>
    <header>
      <button class="hamburger" aria-label="Open menu" onclick={() => (drawerOpen = !drawerOpen)}>☰</button>
      <h1>{dashboardViewTitle(currentView)}</h1>

      <div class="page-actions">
        {#if currentView === 'editor'}
          <button class="back-btn" onclick={onBack}>Back to Map List</button>
        {/if}
        {#if currentView === 'encounter-editor'}
          <button class="back-btn" onclick={onBackFromEncounter}>Back to Encounter List</button>
        {/if}
        {#if currentView === 'shop-editor'}
          <button class="back-btn" onclick={onBackFromShop}>Back to Shop List</button>
        {/if}
      </div>
    </header>

    {#if currentView === 'home'}
      <HomePanel {campaignId} />
    {:else if currentView === 'combat'}
      <CombatManager {campaignId} />
    {:else if currentView === 'campaigns'}
      <CampaignsPage activeCampaignId={campaignId} oncreated={refreshCurrentCampaign} />
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
    {:else if currentView === 'loot'}
      <LootPoolPanel {campaignId} />
    {:else if currentView === 'levelup'}
      <LevelUpPanel />
    {:else if currentView === 'approvals'}
      <CharacterApprovalQueue {campaignId} />
    {:else if currentView === 'errors'}
      <ErrorsPanel {campaignId} />
    {:else if currentView === 'exploration'}
      <ExplorationPanel {campaignId} />
    {:else if currentView === 'characters-new'}
      <CharCreatePanel {campaignId} />
    {/if}
  </main>
</div>

<style>
  :global(body) {
    margin: 0;
    font-family: system-ui, -apple-system, sans-serif;
    background: #1a1a2e;
    color: #e0e0e0;
  }

  .desktop-shell {
    display: grid;
    grid-template-columns: 15rem minmax(0, 1fr);
    min-height: 100vh;
  }

  .sidebar {
    position: sticky;
    top: 0;
    align-self: start;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    height: 100vh;
    padding: 1rem;
    background: #0f1a2e;
    border-right: 1px solid #0f3460;
    box-sizing: border-box;
    overflow-y: auto;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    min-height: 2.5rem;
    color: #f7f7fb;
    font-size: 1rem;
    font-weight: 700;
    text-decoration: none;
  }

  .brand-mark {
    display: inline-grid;
    place-items: center;
    width: 2rem;
    height: 2rem;
    border-radius: 6px;
    background: #e94560;
    color: #ffffff;
  }

  .sidebar-nav {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .sidebar-nav a {
    display: flex;
    align-items: center;
    min-height: 2.25rem;
    padding: 0 0.75rem;
    color: #cbd5e1;
    border-radius: 6px;
    text-decoration: none;
    white-space: nowrap;
  }

  .sidebar-nav a:hover {
    background: #16213e;
    color: #ffffff;
  }

  .sidebar-nav a.active {
    background: #e94560;
    color: #ffffff;
    font-weight: 700;
  }

  main {
    min-width: 0;
    padding: 1rem;
    max-width: 100%;
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 1rem;
    min-height: 2.5rem;
  }

  header h1 {
    color: #e94560;
    margin: 0;
    font-size: 1.75rem;
  }

  .page-actions {
    display: flex;
    justify-content: flex-end;
    min-width: 12rem;
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

  .hamburger {
    display: none;
    background: transparent;
    border: 1px solid #0f3460;
    color: #e0e0e0;
    padding: 0.4rem 0.7rem;
    border-radius: 4px;
    font-size: 1.25rem;
    cursor: pointer;
  }

  .drawer-backdrop {
    display: none;
  }

  @media (max-width: 768px) {
    .desktop-shell {
      grid-template-columns: 1fr;
    }

    .sidebar {
      position: fixed;
      top: 0;
      left: 0;
      z-index: 30;
      width: 16rem;
      height: 100vh;
      transform: translateX(-100%);
      transition: transform 0.2s ease;
      border-right: 1px solid #0f3460;
    }

    .sidebar.open {
      transform: translateX(0);
    }

    .drawer-backdrop {
      display: block;
      position: fixed;
      inset: 0;
      z-index: 20;
      background: rgba(0, 0, 0, 0.5);
      border: none;
      padding: 0;
    }

    .hamburger {
      display: inline-flex;
    }

    header h1 {
      font-size: 1.25rem;
    }

    header {
      gap: 0.5rem;
    }

    .page-actions {
      min-width: 0;
    }
  }
</style>
