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
  import LootPoolPanel from './LootPoolPanel.svelte';
  import LevelUpPanel from './LevelUpPanel.svelte';
  import CharacterApprovalQueue from './CharacterApprovalQueue.svelte';
  import { isMobileViewport, isDesktopOnly } from './lib/layout.js';
  import { resolveDashboardViewFromHash } from './lib/dashboardRouter.js';
  import {
    dashboardNavItems,
    dashboardViewTitle,
    isDashboardNavItemActive,
  } from './lib/dashboardNavigation.js';

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
    return resolveDashboardViewFromHash(window.location.hash);
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

  $effect(() => {
    if (typeof window === 'undefined') return;
    const handler = () => {
      currentView = getInitialView();
    };
    window.addEventListener('hashchange', handler);
    return () => window.removeEventListener('hashchange', handler);
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
  }
</script>

{#if isMobileViewport(innerWidth)}
  {#if isDesktopOnly(currentDesktopOnlyID())}
    <MobileRedirect view={currentDesktopOnlyID()} />
  {/if}
  <MobileShell {campaignId} />
{:else}
<div class="desktop-shell">
  <aside class="sidebar" aria-label="Dashboard navigation">
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

  <main>
    <header>
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
    {:else if currentView === 'loot'}
      <LootPoolPanel {campaignId} />
    {:else if currentView === 'levelup'}
      <LevelUpPanel />
    {:else if currentView === 'approvals'}
      <CharacterApprovalQueue {campaignId} />
    {/if}
  </main>
</div>
{/if}

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
</style>
