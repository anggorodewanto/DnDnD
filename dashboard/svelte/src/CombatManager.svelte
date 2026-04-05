<script>
  import { getCombatWorkspace, updateCombatantHP, updateCombatantConditions } from './lib/api.js';
  import {
    applyDamage,
    applyHealing,
    healthTier,
    STANDARD_CONDITIONS,
    addCondition,
    removeCondition,
    colToIndex,
    tokenOpacity,
  } from './lib/combat.js';
  import {
    terrainByGid,
    lightingByGid,
    getWalls,
    getLightingData,
  } from './lib/mapdata.js';

  let { campaignId } = $props();

  // Data state
  let encounters = $state([]);
  let loading = $state(true);
  let error = $state(null);

  // Tab state
  let activeEncounterIndex = $state(0);

  // Selected token state
  let selectedCombatantId = $state(null);

  // HP/Condition tracker inputs
  let damageInput = $state(0);
  let healInput = $state(0);
  let conditionToAdd = $state('');

  // Canvas ref
  let canvasEl = $state(null);

  // Polling interval
  let pollTimer = $state(null);

  $effect(() => {
    loadWorkspace();
    pollTimer = setInterval(loadWorkspace, 5000);
    return () => {
      if (pollTimer) clearInterval(pollTimer);
    };
  });

  // Redraw when data or selection changes
  $effect(() => {
    if (canvasEl && activeEncounter?.map) {
      drawMap();
    }
  });

  let activeEncounter = $derived(encounters[activeEncounterIndex] || null);
  let selectedCombatant = $derived(
    activeEncounter?.combatants?.find(c => c.id === selectedCombatantId) || null
  );

  async function loadWorkspace() {
    try {
      const data = await getCombatWorkspace(campaignId);
      encounters = data.encounters || [];
      if (activeEncounterIndex >= encounters.length && encounters.length > 0) {
        activeEncounterIndex = 0;
      }
      error = null;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function getTileSize(tiledMap) {
    return tiledMap?.tilewidth || 48;
  }

  function drawMap() {
    if (!canvasEl || !activeEncounter?.map) return;

    const mapData = activeEncounter.map;
    const tiledMap = typeof mapData.tiled_json === 'string'
      ? JSON.parse(mapData.tiled_json)
      : mapData.tiled_json;

    if (!tiledMap) return;

    const tileSize = getTileSize(tiledMap);
    const ctx = canvasEl.getContext('2d');
    canvasEl.width = tiledMap.width * tileSize;
    canvasEl.height = tiledMap.height * tileSize;

    // Draw terrain
    const terrainLayer = tiledMap.layers?.find(l => l.name === 'terrain');
    if (terrainLayer?.data) {
      for (let y = 0; y < tiledMap.height; y++) {
        for (let x = 0; x < tiledMap.width; x++) {
          const idx = y * tiledMap.width + x;
          const gid = terrainLayer.data[idx] || 1;
          const terrain = terrainByGid(gid);
          ctx.fillStyle = terrain.color;
          ctx.fillRect(x * tileSize, y * tileSize, tileSize, tileSize);
        }
      }

      // Grid lines
      ctx.strokeStyle = 'rgba(255,255,255,0.15)';
      ctx.lineWidth = 1;
      for (let y = 0; y < tiledMap.height; y++) {
        for (let x = 0; x < tiledMap.width; x++) {
          ctx.strokeRect(x * tileSize, y * tileSize, tileSize, tileSize);
        }
      }
    }

    // Draw lighting overlay
    const lightingData = getLightingData(tiledMap);
    if (lightingData.length > 0) {
      for (let y = 0; y < tiledMap.height; y++) {
        for (let x = 0; x < tiledMap.width; x++) {
          const idx = y * tiledMap.width + x;
          const gid = lightingData[idx];
          if (gid === 0) continue;
          const lighting = lightingByGid(gid);
          ctx.fillStyle = lighting.color;
          ctx.globalAlpha = 0.4;
          ctx.fillRect(x * tileSize, y * tileSize, tileSize, tileSize);
          ctx.globalAlpha = 1.0;
        }
      }
    }

    // Draw walls
    const walls = getWalls(tiledMap);
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

    // Draw encounter zones
    if (activeEncounter.zones) {
      for (const zone of activeEncounter.zones) {
        const zCol = colToIndex(zone.origin_col);
        const zRow = zone.origin_row;
        ctx.fillStyle = zone.overlay_color || 'rgba(128,0,255,0.2)';
        ctx.globalAlpha = 0.3;
        // Simple single-tile zone for now
        ctx.fillRect(zCol * tileSize, zRow * tileSize, tileSize, tileSize);
        ctx.globalAlpha = 1.0;
      }
    }

    // Draw tokens
    drawTokens(ctx, tiledMap, tileSize);
  }

  function drawTokens(ctx, tiledMap, tileSize) {
    if (!activeEncounter?.combatants) return;

    for (const comb of activeEncounter.combatants) {
      const opacity = tokenOpacity(comb);
      ctx.globalAlpha = opacity;

      const col = colToIndex(comb.position_col);
      const row = comb.position_row;
      const cx = col * tileSize + tileSize / 2;
      const cy = row * tileSize + tileSize / 2;
      const radius = tileSize * 0.4;

      // Health tier color
      const tier = healthTier(comb.hp_current, comb.hp_max);
      const tierColors = {
        healthy: '#22c55e',
        wounded: '#eab308',
        bloodied: '#f97316',
        critical: '#ef4444',
        dead: '#6b7280',
      };

      // Token circle
      ctx.beginPath();
      ctx.arc(cx, cy, radius, 0, 2 * Math.PI);
      ctx.fillStyle = comb.is_npc ? '#dc2626' : '#3b82f6';
      ctx.fill();

      // Health ring
      ctx.lineWidth = 3;
      ctx.strokeStyle = tierColors[tier] || '#6b7280';
      ctx.stroke();

      // Selected indicator
      if (comb.id === selectedCombatantId) {
        ctx.beginPath();
        ctx.arc(cx, cy, radius + 4, 0, 2 * Math.PI);
        ctx.strokeStyle = '#ffffff';
        ctx.lineWidth = 2;
        ctx.setLineDash([4, 2]);
        ctx.stroke();
        ctx.setLineDash([]);
      }

      // Active turn indicator
      if (comb.id === activeEncounter.active_turn_combatant_id) {
        ctx.beginPath();
        ctx.arc(cx, cy, radius + 7, 0, 2 * Math.PI);
        ctx.strokeStyle = '#fbbf24';
        ctx.lineWidth = 2;
        ctx.stroke();
      }

      // Short ID label
      ctx.font = `bold ${Math.max(9, tileSize * 0.22)}px sans-serif`;
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillStyle = '#ffffff';
      ctx.fillText(comb.short_id, cx, cy);

      // Dead X
      if (!comb.is_alive) {
        ctx.strokeStyle = '#ffffff';
        ctx.lineWidth = 2;
        const d = radius * 0.5;
        ctx.beginPath();
        ctx.moveTo(cx - d, cy - d);
        ctx.lineTo(cx + d, cy + d);
        ctx.moveTo(cx + d, cy - d);
        ctx.lineTo(cx - d, cy + d);
        ctx.stroke();
      }

      ctx.globalAlpha = 1.0;
    }
  }

  function handleCanvasClick(e) {
    if (!canvasEl || !activeEncounter?.map) return;

    const tiledMap = typeof activeEncounter.map.tiled_json === 'string'
      ? JSON.parse(activeEncounter.map.tiled_json)
      : activeEncounter.map.tiled_json;

    if (!tiledMap) return;

    const tileSize = getTileSize(tiledMap);
    const rect = canvasEl.getBoundingClientRect();
    const scaleX = canvasEl.width / rect.width;
    const scaleY = canvasEl.height / rect.height;
    const px = (e.clientX - rect.left) * scaleX;
    const py = (e.clientY - rect.top) * scaleY;
    const clickCol = Math.floor(px / tileSize);
    const clickRow = Math.floor(py / tileSize);

    // Find combatant at clicked tile
    const clicked = activeEncounter.combatants?.find(c => {
      return colToIndex(c.position_col) === clickCol && c.position_row === clickRow;
    });

    if (clicked) {
      selectedCombatantId = clicked.id;
      damageInput = 0;
      healInput = 0;
    } else {
      selectedCombatantId = null;
    }

    drawMap();
  }

  async function handleApplyDamage() {
    if (!selectedCombatant || damageInput <= 0) return;

    const result = applyDamage(selectedCombatant, damageInput);
    try {
      await updateCombatantHP(
        activeEncounter.id,
        selectedCombatant.id,
        result,
      );
      damageInput = 0;
      await loadWorkspace();
    } catch (e) {
      error = e.message;
    }
  }

  async function handleApplyHealing() {
    if (!selectedCombatant || healInput <= 0) return;

    const result = applyHealing(selectedCombatant, healInput);
    try {
      await updateCombatantHP(
        activeEncounter.id,
        selectedCombatant.id,
        { hp_current: result.hp_current, temp_hp: selectedCombatant.temp_hp, is_alive: result.is_alive },
      );
      healInput = 0;
      await loadWorkspace();
    } catch (e) {
      error = e.message;
    }
  }

  function currentConditions() {
    return Array.isArray(selectedCombatant?.conditions) ? selectedCombatant.conditions : [];
  }

  async function saveConditions(newConditions) {
    await updateCombatantConditions(activeEncounter.id, selectedCombatant.id, newConditions);
    await loadWorkspace();
  }

  async function handleAddCondition() {
    if (!selectedCombatant || !conditionToAdd) return;
    try {
      await saveConditions(addCondition(currentConditions(), conditionToAdd));
      conditionToAdd = '';
    } catch (e) {
      error = e.message;
    }
  }

  async function handleRemoveCondition(condition) {
    if (!selectedCombatant) return;
    try {
      await saveConditions(removeCondition(currentConditions(), condition));
    } catch (e) {
      error = e.message;
    }
  }
</script>

<div class="combat-manager">
  <!-- Encounter Overview Bar -->
  <div class="encounter-overview" data-testid="encounter-overview">
    {#if activeEncounter}
      <span class="overview-item">Round {activeEncounter.round_number}</span>
      <span class="overview-item">{activeEncounter.combatants?.length || 0} combatants</span>
      <span class="overview-item">Status: {activeEncounter.status}</span>
    {:else}
      <span class="overview-item">No active encounters</span>
    {/if}
  </div>

  <!-- Encounter Tabs -->
  {#if encounters.length > 0}
    <div class="encounter-tabs" data-testid="encounter-tabs">
      {#each encounters as enc, i}
        <button
          class="tab-btn"
          class:active={i === activeEncounterIndex}
          onclick={() => { activeEncounterIndex = i; selectedCombatantId = null; }}
          data-testid="encounter-tab-{i}"
        >
          {enc.name}
        </button>
      {/each}
    </div>
  {/if}

  {#if loading}
    <p class="status-msg">Loading combat workspace...</p>
  {:else if error}
    <p class="error-msg">{error}</p>
  {:else if !activeEncounter}
    <p class="status-msg">No active encounters in this campaign.</p>
  {:else}
    <div class="workspace-layout">
      <!-- Left panel: Map + Tokens -->
      <div class="map-panel" data-testid="map-panel">
        {#if activeEncounter.map}
          <canvas
            bind:this={canvasEl}
            class="combat-canvas"
            onclick={handleCanvasClick}
            data-testid="combat-canvas"
          ></canvas>
        {:else}
          <div class="no-map">
            <p>No map assigned to this encounter.</p>
            <h3>Combatants</h3>
            <ul>
              {#each activeEncounter.combatants || [] as comb}
                <li>
                  <button
                    class="combatant-btn"
                    class:selected={comb.id === selectedCombatantId}
                    onclick={() => { selectedCombatantId = comb.id; damageInput = 0; healInput = 0; }}
                  >
                    {comb.short_id} - {comb.display_name}
                    ({comb.hp_current}/{comb.hp_max} HP)
                  </button>
                </li>
              {/each}
            </ul>
          </div>
        {/if}
      </div>

      <!-- HP & Condition Tracker (shown when a token is selected) -->
      {#if selectedCombatant}
        <div class="tracker-panel" data-testid="tracker-panel">
          <h3>{selectedCombatant.display_name} ({selectedCombatant.short_id})</h3>

          <div class="stat-row">
            <span>HP: {selectedCombatant.hp_current} / {selectedCombatant.hp_max}</span>
            {#if selectedCombatant.temp_hp > 0}
              <span class="temp-hp">(+{selectedCombatant.temp_hp} temp)</span>
            {/if}
          </div>
          <div class="stat-row">
            <span>AC: {selectedCombatant.ac}</span>
          </div>

          <!-- Damage -->
          <div class="action-row">
            <label>
              Damage:
              <input
                type="number"
                min="0"
                bind:value={damageInput}
                data-testid="damage-input"
              />
            </label>
            <button onclick={handleApplyDamage} data-testid="apply-damage-btn">Apply Damage</button>
          </div>

          <!-- Healing -->
          <div class="action-row">
            <label>
              Healing:
              <input
                type="number"
                min="0"
                bind:value={healInput}
                data-testid="heal-input"
              />
            </label>
            <button onclick={handleApplyHealing} data-testid="apply-heal-btn">Apply Healing</button>
          </div>

          <!-- Conditions -->
          <div class="conditions-section">
            <h4>Conditions</h4>
            <div class="condition-list" data-testid="condition-list">
              {#each currentConditions() as cond}
                <span class="condition-tag">
                  {cond}
                  <button
                    class="remove-cond-btn"
                    onclick={() => handleRemoveCondition(cond)}
                    data-testid="remove-condition-{cond}"
                  >x</button>
                </span>
              {/each}
            </div>
            <div class="action-row">
              <select bind:value={conditionToAdd} data-testid="condition-select">
                <option value="">-- Select --</option>
                {#each STANDARD_CONDITIONS as cond}
                  <option value={cond}>{cond}</option>
                {/each}
              </select>
              <button onclick={handleAddCondition} data-testid="add-condition-btn">Add Condition</button>
            </div>
          </div>

          <button class="close-tracker-btn" onclick={() => selectedCombatantId = null}>Close</button>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .combat-manager {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .encounter-overview {
    display: flex;
    gap: 1.5rem;
    padding: 0.5rem 1rem;
    background: #16213e;
    border-radius: 4px;
    border: 1px solid #0f3460;
    font-size: 0.9rem;
  }

  .overview-item {
    color: #a0aec0;
  }

  .encounter-tabs {
    display: flex;
    gap: 0.25rem;
  }

  .tab-btn {
    padding: 0.4rem 1rem;
    background: #16213e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px 4px 0 0;
    cursor: pointer;
    font-size: 0.85rem;
  }

  .tab-btn:hover {
    background: #0f3460;
  }

  .tab-btn.active {
    background: #e94560;
    border-color: #e94560;
    color: white;
  }

  .workspace-layout {
    display: flex;
    gap: 1rem;
  }

  .map-panel {
    flex: 0 0 60%;
    overflow: auto;
  }

  .combat-canvas {
    max-width: 100%;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }

  .no-map {
    padding: 1rem;
    background: #16213e;
    border-radius: 4px;
    border: 1px solid #0f3460;
  }

  .combatant-btn {
    background: none;
    border: 1px solid #0f3460;
    color: #e0e0e0;
    padding: 0.3rem 0.6rem;
    border-radius: 4px;
    cursor: pointer;
    margin: 0.2rem 0;
    text-align: left;
    width: 100%;
  }

  .combatant-btn:hover {
    background: #0f3460;
  }

  .combatant-btn.selected {
    border-color: #e94560;
    background: rgba(233, 69, 96, 0.15);
  }

  .tracker-panel {
    flex: 0 0 35%;
    padding: 1rem;
    background: #16213e;
    border-radius: 4px;
    border: 1px solid #0f3460;
  }

  .tracker-panel h3 {
    margin: 0 0 0.5rem 0;
    color: #e94560;
  }

  .stat-row {
    margin: 0.3rem 0;
    color: #e0e0e0;
  }

  .temp-hp {
    color: #3b82f6;
    margin-left: 0.3rem;
  }

  .action-row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    margin: 0.5rem 0;
  }

  .action-row input[type='number'] {
    width: 60px;
    padding: 0.3rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }

  .action-row button, .close-tracker-btn {
    padding: 0.3rem 0.8rem;
    background: #0f3460;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }

  .action-row button:hover, .close-tracker-btn:hover {
    background: #e94560;
    border-color: #e94560;
  }

  .conditions-section {
    margin-top: 0.8rem;
  }

  .conditions-section h4 {
    margin: 0 0 0.3rem;
    color: #a0aec0;
  }

  .condition-list {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
    margin-bottom: 0.5rem;
  }

  .condition-tag {
    display: inline-flex;
    align-items: center;
    gap: 0.2rem;
    padding: 0.15rem 0.5rem;
    background: #e94560;
    color: white;
    border-radius: 12px;
    font-size: 0.8rem;
  }

  .remove-cond-btn {
    background: none;
    border: none;
    color: white;
    cursor: pointer;
    font-size: 0.8rem;
    padding: 0;
    margin-left: 0.2rem;
  }

  .action-row select {
    padding: 0.3rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }

  .close-tracker-btn {
    margin-top: 0.8rem;
    width: 100%;
  }

  .status-msg {
    color: #a0aec0;
    font-style: italic;
  }

  .error-msg {
    color: #ef4444;
  }
</style>
