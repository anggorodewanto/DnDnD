<script>
  import {
    createEncounter,
    getEncounter,
    updateEncounter,
    listMaps,
    getMap,
    listCreatures,
    startCombat,
  } from './lib/api.js';
  import { collectSurprisedShortIDs } from './lib/combat.js';
  import {
    terrainByGid,
    getWalls,
  } from './lib/mapdata.js';
  import ItemPicker from './ItemPicker.svelte';
  import DisplayNameEditor from './DisplayNameEditor.svelte';

  let { campaignId, encounterId = null, onback } = $props();

  // Encounter state
  let encounterName = $state('New Encounter');
  let displayName = $state('');
  let selectedMapId = $state(null);
  let creatures = $state([]);
  let savedEncounterId = $state(null);
  let dirty = $state(false);

  // Map state
  let maps = $state([]);
  let loadedMap = $state(null);

  // Creature library state
  let availableCreatures = $state([]);
  let creatureSearch = $state('');

  // UI state
  let loading = $state(false);
  let saving = $state(false);
  let error = $state(null);
  let statusMsg = $state('');

  // Canvas state
  let canvasEl = $state(null);

  // Drag state for placing creatures
  let draggingCreature = $state(null);
  let dragPreviewPos = $state(null);

  // Loot panel state
  let showLootPanel = $state(false);
  let lootItems = $state([]);

  function onLootSelect(items) {
    lootItems = items;
  }

  // Short ID counter per creature type
  let shortIdCounters = $state({});

  // Phase 114 — per-creature surprised toggle, keyed by index into `creatures`.
  // Reset whenever the encounter is reloaded.
  let surprisedByIndex = $state({});
  // Start-combat flow state.
  let startingCombat = $state(false);
  let startCombatError = $state(null);
  let startCombatMsg = $state('');

  // Load existing encounter
  $effect(() => {
    if (encounterId) {
      savedEncounterId = encounterId;
      loadEncounter(encounterId);
    }
  });

  // Load maps for selection
  $effect(() => {
    if (campaignId) {
      loadMaps();
      loadCreatureLibrary();
    }
  });

  // Redraw when map or creatures change
  $effect(() => {
    if (loadedMap && canvasEl) {
      drawMap();
    }
  });

  async function loadEncounter(id) {
    loading = true;
    error = null;
    try {
      const data = await getEncounter(id);
      encounterName = data.name;
      displayName = data.display_name || '';
      selectedMapId = data.map_id;
      creatures = data.creatures || [];
      savedEncounterId = data.id;
      dirty = false;

      // Rebuild short ID counters
      rebuildShortIdCounters();

      if (data.map_id) {
        await loadMapData(data.map_id);
      }
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function loadMaps() {
    try {
      maps = await listMaps(campaignId);
    } catch (e) {
      // Maps may not be available, that's ok
      maps = [];
    }
  }

  async function loadCreatureLibrary() {
    try {
      availableCreatures = await listCreatures();
    } catch (e) {
      // Creatures may not be available
      availableCreatures = [];
    }
  }

  async function loadMapData(mapId) {
    try {
      const data = await getMap(mapId);
      loadedMap = typeof data.tiled_json === 'string' ? JSON.parse(data.tiled_json) : data.tiled_json;
      drawMap();
    } catch (e) {
      error = 'Failed to load map: ' + e.message;
    }
  }

  function handleMapSelect(e) {
    const mapId = e.target.value;
    selectedMapId = mapId || null;
    dirty = true;
    if (mapId) {
      loadMapData(mapId);
    } else {
      loadedMap = null;
    }
  }

  function generateShortId(creatureName) {
    // Generate a short ID like "G1", "G2", "OS1" from creature name
    const words = creatureName.split(/\s+/);
    let prefix;
    if (words.length === 1) {
      prefix = words[0].substring(0, 1).toUpperCase();
    } else {
      prefix = words.map(w => w[0].toUpperCase()).join('');
    }

    if (!shortIdCounters[prefix]) {
      shortIdCounters[prefix] = 0;
    }
    shortIdCounters[prefix]++;
    return prefix + shortIdCounters[prefix];
  }

  function rebuildShortIdCounters() {
    shortIdCounters = {};
    for (const c of creatures) {
      if (!c.short_id) continue;
      const match = c.short_id.match(/^([A-Z]+)(\d+)$/);
      if (!match) continue;
      const prefix = match[1];
      const num = parseInt(match[2], 10);
      if (!shortIdCounters[prefix] || shortIdCounters[prefix] < num) {
        shortIdCounters[prefix] = num;
      }
    }
  }

  function addCreature(creature) {
    const shortId = generateShortId(creature.name);
    creatures = [...creatures, {
      creature_ref_id: creature.id,
      short_id: shortId,
      display_name: creature.name,
      quantity: 1,
      position_col: null,
      position_row: null,
    }];
    dirty = true;
  }

  function removeCreature(index) {
    creatures = creatures.filter((_, i) => i !== index);
    dirty = true;
    drawMap();
  }

  function updateCreatureQuantity(index, qty) {
    const updated = [...creatures];
    updated[index] = { ...updated[index], quantity: Math.max(1, qty) };
    creatures = updated;
    dirty = true;
  }

  function getTileSize() {
    return loadedMap?.tilewidth || 48;
  }

  function drawMap() {
    if (!canvasEl || !loadedMap) return;

    const tileSize = getTileSize();
    const ctx = canvasEl.getContext('2d');
    canvasEl.width = loadedMap.width * tileSize;
    canvasEl.height = loadedMap.height * tileSize;

    // Draw terrain
    const terrainLayer = loadedMap.layers?.find(l => l.name === 'terrain');
    if (terrainLayer?.data) {
      for (let y = 0; y < loadedMap.height; y++) {
        for (let x = 0; x < loadedMap.width; x++) {
          const idx = y * loadedMap.width + x;
          const gid = terrainLayer.data[idx] || 1;
          const terrain = terrainByGid(gid);
          ctx.fillStyle = terrain.color;
          ctx.fillRect(x * tileSize, y * tileSize, tileSize, tileSize);
        }
      }

      // Grid lines
      ctx.strokeStyle = 'rgba(255,255,255,0.15)';
      ctx.lineWidth = 1;
      for (let y = 0; y < loadedMap.height; y++) {
        for (let x = 0; x < loadedMap.width; x++) {
          ctx.strokeRect(x * tileSize, y * tileSize, tileSize, tileSize);
        }
      }
    }

    // Draw walls
    const walls = getWalls(loadedMap);
    ctx.strokeStyle = '#ff0000';
    ctx.lineWidth = 3;
    for (const wall of walls) {
      ctx.beginPath();
      if (wall.width > 0) {
        ctx.moveTo(wall.x, wall.y);
        ctx.lineTo(wall.x + wall.width, wall.y);
      } else if (wall.height > 0) {
        ctx.moveTo(wall.x, wall.y);
        ctx.lineTo(wall.x, wall.y + wall.height);
      }
      ctx.stroke();
    }

    // Draw placed creature tokens
    for (const c of creatures) {
      if (c.position_col == null || c.position_row == null) continue;
      drawCreatureToken(ctx, c.position_col, c.position_row, c.short_id, tileSize);
    }

    // Draw drag preview
    if (draggingCreature && dragPreviewPos) {
      ctx.globalAlpha = 0.5;
      drawCreatureToken(ctx, dragPreviewPos.col, dragPreviewPos.row, draggingCreature.short_id, tileSize);
      ctx.globalAlpha = 1.0;
    }
  }

  function drawCreatureToken(ctx, col, row, label, tileSize) {
    const x = col * tileSize;
    const y = row * tileSize;
    const radius = tileSize * 0.4;

    ctx.beginPath();
    ctx.arc(x + tileSize / 2, y + tileSize / 2, radius, 0, Math.PI * 2);
    ctx.fillStyle = '#e94560';
    ctx.fill();
    ctx.strokeStyle = '#fff';
    ctx.lineWidth = 2;
    ctx.stroke();

    ctx.fillStyle = '#fff';
    ctx.font = `bold ${Math.max(10, tileSize * 0.3)}px sans-serif`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(label, x + tileSize / 2, y + tileSize / 2);
  }

  function canvasTileCoords(e) {
    const rect = canvasEl.getBoundingClientRect();
    const scaleX = canvasEl.width / rect.width;
    const scaleY = canvasEl.height / rect.height;
    const tileSize = getTileSize();
    const col = Math.floor((e.clientX - rect.left) * scaleX / tileSize);
    const row = Math.floor((e.clientY - rect.top) * scaleY / tileSize);
    return { col, row };
  }

  function isTileInBounds(col, row) {
    return col >= 0 && col < loadedMap.width && row >= 0 && row < loadedMap.height;
  }

  function handleCanvasDrop(e) {
    e.preventDefault();
    if (!draggingCreature || !loadedMap) return;

    const { col, row } = canvasTileCoords(e);
    if (!isTileInBounds(col, row)) return;

    const idx = creatures.indexOf(draggingCreature);
    if (idx === -1) return;

    const updated = [...creatures];
    updated[idx] = { ...updated[idx], position_col: col, position_row: row };
    creatures = updated;
    dirty = true;
    draggingCreature = null;
    dragPreviewPos = null;
    drawMap();
  }

  function handleCanvasDragOver(e) {
    e.preventDefault();
    if (!draggingCreature || !loadedMap) return;

    const { col, row } = canvasTileCoords(e);
    if (isTileInBounds(col, row)) {
      dragPreviewPos = { col, row };
      drawMap();
    }
  }

  function handleCanvasDragLeave() {
    dragPreviewPos = null;
    drawMap();
  }

  function startDragCreature(creature) {
    draggingCreature = creature;
  }

  // Phase 114 — Start combat from the prepared template. Passes the list of
  // short IDs the DM flagged as surprised so the backend can mark them in
  // round 1. PC selection/positions aren't managed by this builder yet — the
  // backend treats missing character_ids as "no PCs" and will still roll
  // initiative for the creatures on the template.
  async function handleStartCombat() {
    if (!savedEncounterId) {
      startCombatError = 'Save the encounter first before starting combat.';
      return;
    }
    startingCombat = true;
    startCombatError = null;
    startCombatMsg = '';
    try {
      const payload = {
        template_id: savedEncounterId,
        character_ids: [],
        character_positions: {},
      };
      const surprisedShortIDs = collectSurprisedShortIDs(creatures, surprisedByIndex);
      if (surprisedShortIDs.length > 0) {
        payload.surprised_combatant_short_ids = surprisedShortIDs;
      }
      const result = await startCombat(payload);
      const surprisedCount = surprisedShortIDs.length;
      const suffix = surprisedCount === 0
        ? ''
        : ` with ${surprisedCount} surprised combatant${surprisedCount === 1 ? '' : 's'}`;
      startCombatMsg = `Combat started${suffix}. Encounter: ${result?.encounter?.id ?? savedEncounterId}`;
    } catch (e) {
      startCombatError = e.message;
    } finally {
      startingCombat = false;
    }
  }

  async function saveEncounter() {
    saving = true;
    error = null;
    statusMsg = '';
    try {
      const payload = {
        name: encounterName,
        display_name: displayName || undefined,
        map_id: selectedMapId || undefined,
        creatures: creatures,
      };

      if (savedEncounterId) {
        await updateEncounter(savedEncounterId, payload);
        statusMsg = 'Encounter saved.';
      } else {
        payload.campaign_id = campaignId;
        const result = await createEncounter(payload);
        savedEncounterId = result.id;
        statusMsg = 'Encounter created.';
      }
      dirty = false;
    } catch (e) {
      error = e.message;
    } finally {
      saving = false;
    }
  }

  let filteredCreatures = $derived(
    availableCreatures.filter(c =>
      c.name.toLowerCase().includes(creatureSearch.toLowerCase())
    )
  );
</script>

<div class="encounter-builder">
  {#if loading}
    <p>Loading encounter...</p>
  {:else}
    <div class="builder-layout">
      <!-- Left panel: Settings & Creature Library -->
      <div class="left-panel">
        <div class="settings-section">
          <h3>Encounter Settings</h3>
          <div class="form-group">
            <label>Internal Name (DM-only):</label>
            <input type="text" bind:value={encounterName} oninput={() => dirty = true} />
          </div>
          <!-- Phase 105c — shared DisplayNameEditor. Persistence stays on
               the builder's Save button (template PUT), so the commit handler
               just updates local state and marks the form dirty. -->
          <div class="form-group">
            <DisplayNameEditor
              value={displayName}
              fallback={encounterName}
              label="Display Name (player-facing, optional)"
              onCommit={(v) => { displayName = v; dirty = true; }}
            />
          </div>
          <div class="form-group">
            <label>Map:</label>
            <select onchange={handleMapSelect} value={selectedMapId || ''}>
              <option value="">-- No map --</option>
              {#each maps as map}
                <option value={map.id}>{map.name} ({map.width}x{map.height})</option>
              {/each}
            </select>
          </div>
        </div>

        <div class="creature-library">
          <h3>Stat Block Library</h3>
          <input type="text" class="search-input" bind:value={creatureSearch}
                 placeholder="Search creatures..." />
          <div class="creature-list">
            {#each filteredCreatures.slice(0, 50) as creature}
              <div class="creature-item">
                <span class="creature-name">{creature.name}</span>
                <span class="creature-cr">CR {creature.cr}</span>
                <button class="add-btn" onclick={() => addCreature(creature)}>+</button>
              </div>
            {/each}
            {#if filteredCreatures.length === 0 && availableCreatures.length > 0}
              <p class="no-results">No creatures match your search.</p>
            {/if}
            {#if availableCreatures.length === 0}
              <p class="no-results">No creatures in library. Import stat blocks first.</p>
            {/if}
          </div>
        </div>

        <div class="encounter-creatures">
          <h3>Encounter Creatures ({creatures.length})</h3>
          {#each creatures as creature, idx}
            <div class="placed-creature"
                 draggable="true"
                 ondragstart={() => startDragCreature(creature)}>
              <span class="short-id">{creature.short_id}</span>
              <span class="creature-display-name">{creature.display_name}</span>
              <input type="number" class="qty-input" min="1"
                     value={creature.quantity}
                     oninput={(e) => updateCreatureQuantity(idx, parseInt(e.target.value) || 1)} />
              <label class="surprised-label" title="Surprised in round 1">
                <input type="checkbox"
                       checked={!!surprisedByIndex[idx]}
                       onchange={(e) => (surprisedByIndex = { ...surprisedByIndex, [idx]: e.target.checked })} />
                Surprised
              </label>
              <span class="placement-status">
                {creature.position_col != null ? `(${creature.position_col},${creature.position_row})` : 'Not placed'}
              </span>
              <button class="remove-btn" onclick={() => removeCreature(idx)}>x</button>
            </div>
          {/each}
          {#if creatures.length === 0}
            <p class="no-results">No creatures added yet.</p>
          {/if}
        </div>

        <div class="loot-section">
          <button class="loot-toggle" onclick={() => showLootPanel = !showLootPanel}>
            {showLootPanel ? 'Hide' : 'Show'} Loot Picker ({lootItems.length} items)
          </button>
          {#if showLootPanel}
            <ItemPicker
              {campaignId}
              encounterId={savedEncounterId}
              onselect={onLootSelect}
            />
          {/if}
        </div>
      </div>

      <!-- Right panel: Map with creature placement -->
      <div class="right-panel">
        <div class="toolbar">
          <button class="save-btn" onclick={saveEncounter} disabled={saving || !dirty}>
            {saving ? 'Saving...' : 'Save'}
          </button>
          <button class="start-btn"
                  onclick={handleStartCombat}
                  disabled={startingCombat || !savedEncounterId || dirty}
                  title={!savedEncounterId ? 'Save the encounter first' : dirty ? 'Save pending changes first' : ''}>
            {startingCombat ? 'Starting...' : 'Start Combat'}
          </button>
          {#if statusMsg}
            <span class="status">{statusMsg}</span>
          {/if}
          {#if startCombatMsg}
            <span class="status">{startCombatMsg}</span>
          {/if}
          {#if dirty}
            <span class="dirty-indicator">*unsaved</span>
          {/if}
          <button class="back-btn" onclick={onback}>Back</button>
        </div>

        {#if error}
          <p class="error">{error}</p>
        {/if}
        {#if startCombatError}
          <p class="error">{startCombatError}</p>
        {/if}

        {#if loadedMap}
          <p class="hint">Drag creatures from the list onto the map to place them.</p>
          <div class="canvas-container">
            <canvas
              bind:this={canvasEl}
              ondrop={handleCanvasDrop}
              ondragover={handleCanvasDragOver}
              ondragleave={handleCanvasDragLeave}
            ></canvas>
          </div>
          <div class="info-bar">
            <span>{loadedMap.width} x {loadedMap.height} squares</span>
          </div>
        {:else}
          <div class="no-map-placeholder">
            <p>Select a map to place creature tokens.</p>
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .encounter-builder {
    width: 100%;
  }

  .builder-layout {
    display: flex;
    gap: 1rem;
  }

  .left-panel {
    width: 350px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .right-panel {
    flex: 1;
    min-width: 0;
  }

  .settings-section, .creature-library, .encounter-creatures {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    padding: 1rem;
  }

  .settings-section h3, .creature-library h3, .encounter-creatures h3 {
    color: #e94560;
    margin: 0 0 0.75rem;
    font-size: 1rem;
  }

  .form-group {
    margin-bottom: 0.75rem;
  }

  .form-group label {
    display: block;
    font-size: 0.85rem;
    color: #aaa;
    margin-bottom: 0.25rem;
  }

  .form-group input, .form-group select {
    width: 100%;
    padding: 0.5rem;
    background: #1a1a2e;
    border: 1px solid #0f3460;
    color: #e0e0e0;
    border-radius: 4px;
  }

  .search-input {
    width: 100%;
    padding: 0.5rem;
    background: #1a1a2e;
    border: 1px solid #0f3460;
    color: #e0e0e0;
    border-radius: 4px;
    margin-bottom: 0.5rem;
  }

  .creature-list {
    max-height: 200px;
    overflow-y: auto;
  }

  .creature-item {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.3rem 0;
    border-bottom: 1px solid #0f3460;
  }

  .creature-name {
    flex: 1;
    font-size: 0.85rem;
  }

  .creature-cr {
    color: #888;
    font-size: 0.75rem;
  }

  .add-btn {
    padding: 0.2rem 0.5rem;
    background: #28a745;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
  }

  .add-btn:hover {
    background: #218838;
  }

  .placed-creature {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.3rem 0;
    border-bottom: 1px solid #0f3460;
    cursor: grab;
    font-size: 0.85rem;
  }

  .short-id {
    background: #e94560;
    color: white;
    padding: 0.1rem 0.4rem;
    border-radius: 3px;
    font-weight: bold;
    font-size: 0.75rem;
    min-width: 2em;
    text-align: center;
  }

  .creature-display-name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .qty-input {
    width: 45px;
    padding: 0.2rem;
    background: #1a1a2e;
    border: 1px solid #0f3460;
    color: #e0e0e0;
    border-radius: 3px;
    text-align: center;
  }

  .placement-status {
    color: #666;
    font-size: 0.75rem;
    min-width: 5em;
  }

  .remove-btn {
    padding: 0.1rem 0.4rem;
    background: #8b0000;
    color: white;
    border: none;
    border-radius: 3px;
    cursor: pointer;
    font-size: 0.75rem;
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    margin-bottom: 0.5rem;
  }

  .toolbar button {
    padding: 0.4rem 0.8rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }

  .save-btn {
    background: #28a745 !important;
    border-color: #28a745 !important;
    color: white !important;
  }

  .save-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed !important;
  }

  .start-btn {
    background: #e94560 !important;
    border-color: #e94560 !important;
    color: white !important;
  }

  .start-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed !important;
  }

  .surprised-label {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    font-size: 0.75rem;
    color: #ffc107;
    cursor: pointer;
    white-space: nowrap;
  }

  .surprised-label input[type="checkbox"] {
    accent-color: #ffc107;
    margin: 0;
  }

  .back-btn {
    margin-left: auto;
  }

  .status {
    color: #28a745;
    font-size: 0.85rem;
  }

  .dirty-indicator {
    color: #ffc107;
    font-size: 0.85rem;
  }

  .hint {
    font-size: 0.85rem;
    color: #888;
    font-style: italic;
    margin-bottom: 0.5rem;
  }

  .canvas-container {
    overflow: auto;
    max-width: 100%;
    max-height: 70vh;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }

  canvas {
    display: block;
    cursor: crosshair;
  }

  .info-bar {
    display: flex;
    gap: 2rem;
    padding: 0.5rem;
    font-size: 0.85rem;
    color: #888;
  }

  .no-map-placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 300px;
    background: #16213e;
    border: 1px dashed #0f3460;
    border-radius: 8px;
    color: #666;
  }

  .no-results {
    color: #666;
    font-size: 0.85rem;
    font-style: italic;
    padding: 0.5rem 0;
  }

  .loot-section {
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    padding: 1rem;
  }

  .loot-toggle {
    width: 100%;
    padding: 0.5rem 1rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.9rem;
  }

  .loot-toggle:hover {
    background: #0f3460;
  }

  .error {
    color: #ff4444;
    margin-bottom: 0.5rem;
  }
</style>
