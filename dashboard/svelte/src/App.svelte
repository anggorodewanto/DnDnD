<script>
  import MapEditor from './MapEditor.svelte';
  import MapList from './MapList.svelte';

  let currentView = $state('list');
  let editingMapId = $state(null);
  // For demo purposes, use a fixed campaign ID. In production this comes from session.
  let campaignId = $state('00000000-0000-0000-0000-000000000001');

  function onCreateNew() {
    editingMapId = null;
    currentView = 'editor';
  }

  function onEditMap(event) {
    editingMapId = event.detail.id;
    currentView = 'editor';
  }

  function onBack() {
    currentView = 'list';
    editingMapId = null;
  }
</script>

<main>
  <header>
    <h1>Map Editor</h1>
    {#if currentView === 'editor'}
      <button class="back-btn" onclick={onBack}>Back to Map List</button>
    {/if}
  </header>

  {#if currentView === 'list'}
    <MapList {campaignId} oncreate={onCreateNew} onedit={onEditMap} />
  {:else}
    <MapEditor {campaignId} mapId={editingMapId} onback={onBack} />
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
