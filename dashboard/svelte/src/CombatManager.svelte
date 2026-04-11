<script>
  import {
    getCombatWorkspace,
    updateCombatantHP,
    updateCombatantConditions,
    updateCombatantPosition,
    removeCombatant,
    undoLastAction,
    overrideCombatantHP as dmOverrideHP,
    overrideCombatantPosition as dmOverridePosition,
    overrideCombatantConditions as dmOverrideConditions,
    overrideCombatantInitiative,
    overrideCharacterSpellSlots,
  } from './lib/api.js';
  import {
    applyDamage,
    applyHealing,
    healthTier,
    STANDARD_CONDITIONS,
    addCondition,
    removeCondition,
    colToIndex,
    indexToCol,
    tokenOpacity,
    gridDistance,
    tilesInRange,
    findPath,
  } from './lib/combat.js';
  import {
    terrainByGid,
    lightingByGid,
    getWalls,
    getLightingData,
  } from './lib/mapdata.js';
  import { createEncounterTabsWs } from './lib/encounterTabsWs.js';
  import TurnQueue from './TurnQueue.svelte';
  import ActionResolver from './ActionResolver.svelte';
  import ActiveReactionsPanel from './ActiveReactionsPanel.svelte';
  import ActionLogViewer from './ActionLogViewer.svelte';

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

  // Phase 105 — per-tab WebSocket state sync. One wsClient per active
  // encounter, feeds snapshots through mergeSnapshot so the DM's in-progress
  // form edits survive incoming player updates.
  let tabsWs = null;
  // Map of encounter_id -> pending snapshot fields ("HP updated to 3 by
  // player action" indicators). Re-keyed via setState on ws callbacks.
  let pendingByEncounter = $state({});
  // Dirty form field tracking — the form handlers call markDirtyField /
  // clearDirtyField as the DM enters/leaves inputs.
  function markDirtyField(field) {
    if (activeEncounter && tabsWs) {
      tabsWs.markDirty(activeEncounter.id, field);
    }
  }
  function clearDirtyField(field) {
    if (activeEncounter && tabsWs) {
      tabsWs.clearDirty(activeEncounter.id, field);
    }
  }

  // Drag-and-drop state
  let dragging = $state(null); // { combatantId, startCol, startRow }
  let dragCol = $state(null);
  let dragRow = $state(null);

  // Context menu state
  let contextMenu = $state(null); // { x, y, combatantId }

  // Distance measurement tool
  let measureMode = $state(false);
  let measureStart = $state(null); // { col, row }
  let measureEnd = $state(null); // { col, row }

  $effect(() => {
    loadWorkspace();
    pollTimer = setInterval(loadWorkspace, 5000);

    // Phase 105 — open one WebSocket per active encounter tab. The
    // setEncounters call inside loadWorkspace keeps the sockets in
    // sync with the current encounter list.
    const proto = window.location?.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location?.host || 'localhost';
    tabsWs = createEncounterTabsWs({ url: `${proto}//${host}/api/ws` });
    const unsubscribe = tabsWs.subscribe((encID, state) => {
      pendingByEncounter = {
        ...pendingByEncounter,
        [encID]: state._pendingFromSnapshot || {},
      };
    });

    return () => {
      if (pollTimer) clearInterval(pollTimer);
      unsubscribe();
      if (tabsWs) {
        tabsWs.close();
        tabsWs = null;
      }
    };
  });

  // Redraw when data or selection changes
  $effect(() => {
    // Track reactive deps
    void selectedCombatantId;
    void dragging;
    void dragCol;
    void dragRow;
    void measureStart;
    void measureEnd;
    void measureMode;
    if (canvasEl && activeEncounter?.map) {
      drawMap();
    }
  });

  let activeEncounter = $derived(encounters[activeEncounterIndex] || null);
  let selectedCombatant = $derived(
    activeEncounter?.combatants?.find(c => c.id === selectedCombatantId) || null
  );

  // --- DM Dashboard: Undo & Manual Override (Phase 97b) ---
  let dmOverrideOpen = $state(false);
  let dmOverrideReason = $state('');
  let dmOverrideMessage = $state('');
  let dmOverrideHpInput = $state(0);
  let dmOverrideTempHpInput = $state(0);
  let dmOverridePosCol = $state('');
  let dmOverridePosRow = $state(0);
  let dmOverrideAltitude = $state(0);
  let dmOverrideConditionsText = $state('[]');
  let dmOverrideInitiativeRoll = $state(0);
  let dmOverrideInitiativeOrder = $state(0);
  let dmOverrideSpellSlotsText = $state('{}');
  let dmUndoReason = $state('');

  async function handleUndoLastAction() {
    if (!activeEncounter) return;
    dmOverrideMessage = '';
    try {
      await undoLastAction(activeEncounter.id, dmUndoReason);
      dmUndoReason = '';
      dmOverrideMessage = 'Undone.';
      await loadWorkspace();
    } catch (e) {
      dmOverrideMessage = 'Undo failed: ' + e.message;
    }
  }

  async function handleOverrideHP() {
    if (!activeEncounter || !selectedCombatant) return;
    dmOverrideMessage = '';
    try {
      await dmOverrideHP(activeEncounter.id, selectedCombatant.id, {
        hp_current: Number(dmOverrideHpInput),
        temp_hp: Number(dmOverrideTempHpInput),
        reason: dmOverrideReason,
      });
      dmOverrideMessage = 'HP override saved.';
      await loadWorkspace();
    } catch (e) {
      dmOverrideMessage = 'Override failed: ' + e.message;
    }
  }

  async function handleOverridePosition() {
    if (!activeEncounter || !selectedCombatant) return;
    dmOverrideMessage = '';
    try {
      await dmOverridePosition(activeEncounter.id, selectedCombatant.id, {
        position_col: dmOverridePosCol,
        position_row: Number(dmOverridePosRow),
        altitude_ft: Number(dmOverrideAltitude),
        reason: dmOverrideReason,
      });
      dmOverrideMessage = 'Position override saved.';
      await loadWorkspace();
    } catch (e) {
      dmOverrideMessage = 'Override failed: ' + e.message;
    }
  }

  async function handleOverrideConditions() {
    if (!activeEncounter || !selectedCombatant) return;
    dmOverrideMessage = '';
    try {
      const parsed = JSON.parse(dmOverrideConditionsText || '[]');
      await dmOverrideConditions(activeEncounter.id, selectedCombatant.id, {
        conditions: parsed,
        reason: dmOverrideReason,
      });
      dmOverrideMessage = 'Conditions override saved.';
      await loadWorkspace();
    } catch (e) {
      dmOverrideMessage = 'Override failed: ' + e.message;
    }
  }

  async function handleOverrideInitiative() {
    if (!activeEncounter || !selectedCombatant) return;
    dmOverrideMessage = '';
    try {
      await overrideCombatantInitiative(activeEncounter.id, selectedCombatant.id, {
        initiative_roll: Number(dmOverrideInitiativeRoll),
        initiative_order: Number(dmOverrideInitiativeOrder),
        reason: dmOverrideReason,
      });
      dmOverrideMessage = 'Initiative override saved.';
      await loadWorkspace();
    } catch (e) {
      dmOverrideMessage = 'Override failed: ' + e.message;
    }
  }

  async function handleOverrideSpellSlots() {
    if (!activeEncounter || !selectedCombatant?.character_id) return;
    dmOverrideMessage = '';
    try {
      const parsed = JSON.parse(dmOverrideSpellSlotsText || '{}');
      await overrideCharacterSpellSlots(activeEncounter.id, selectedCombatant.character_id, {
        spell_slots: parsed,
        reason: dmOverrideReason,
      });
      dmOverrideMessage = 'Spell slots override saved.';
      await loadWorkspace();
    } catch (e) {
      dmOverrideMessage = 'Override failed: ' + e.message;
    }
  }

  async function loadWorkspace() {
    try {
      const data = await getCombatWorkspace(campaignId);
      encounters = data.encounters || [];
      if (activeEncounterIndex >= encounters.length && encounters.length > 0) {
        activeEncounterIndex = 0;
      }

      // Phase 105 — sync the wsClient tabs to the current active-encounter
      // set. Removed encounters close their socket; new ones open a fresh
      // connection keyed on encounter_id.
      if (tabsWs) {
        tabsWs.setEncounters(encounters.map((e) => e.id));
        for (const enc of encounters) {
          tabsWs.updateState(enc.id, enc);
        }
      }

      error = null;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function parseTiledMap(mapData) {
    if (!mapData?.tiled_json) return null;
    return typeof mapData.tiled_json === 'string'
      ? JSON.parse(mapData.tiled_json)
      : mapData.tiled_json;
  }

  function getTileSize(tiledMap) {
    return tiledMap?.tilewidth || 48;
  }

  function drawMap() {
    if (!canvasEl || !activeEncounter?.map) return;

    const tiledMap = parseTiledMap(activeEncounter.map);
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

    // Draw range circle for selected token
    if (selectedCombatantId && !dragging) {
      drawRangeCircle(ctx, tiledMap, tileSize);
    }

    // Draw drag overlay
    if (dragging && dragCol !== null && dragRow !== null) {
      drawDragOverlay(ctx, tiledMap, tileSize);
    }

    // Draw measurement line
    if (measureStart && measureEnd) {
      drawMeasureLine(ctx, tileSize);
    }
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

  function drawRangeCircle(ctx, tiledMap, tileSize) {
    const comb = selectedCombatant;
    if (!comb) return;

    // Movement range in tiles (speed / 5)
    const speedFt = comb.speed_ft || 30;
    const rangeTiles = Math.floor(speedFt / 5);
    const col = colToIndex(comb.position_col);
    const row = comb.position_row;

    const tiles = tilesInRange(col, row, rangeTiles, tiledMap.width, tiledMap.height);
    ctx.globalAlpha = 0.15;
    ctx.fillStyle = '#3b82f6';
    for (const t of tiles) {
      ctx.fillRect(t.col * tileSize, t.row * tileSize, tileSize, tileSize);
    }
    ctx.globalAlpha = 1.0;

    // Draw range border
    ctx.globalAlpha = 0.4;
    ctx.strokeStyle = '#3b82f6';
    ctx.lineWidth = 2;
    for (const t of tiles) {
      ctx.strokeRect(t.col * tileSize, t.row * tileSize, tileSize, tileSize);
    }
    ctx.globalAlpha = 1.0;
  }

  function drawDragOverlay(ctx, tiledMap, tileSize) {
    const walls = getWalls(tiledMap);
    const startCol = dragging.startCol;
    const startRow = dragging.startRow;

    // Use A* pathfinding for movement validation
    const result = findPath(startCol, startRow, dragCol, dragRow, walls, tiledMap.width, tiledMap.height, tileSize);
    const blocked = !result.found;
    const dist = result.cost;
    const color = blocked ? '#ef4444' : '#22c55e';

    // Draw path tiles
    if (result.found && result.path.length > 1) {
      ctx.globalAlpha = 0.2;
      ctx.fillStyle = color;
      for (let i = 1; i < result.path.length - 1; i++) {
        const step = result.path[i];
        ctx.fillRect(step.col * tileSize, step.row * tileSize, tileSize, tileSize);
      }
      ctx.globalAlpha = 1.0;

      // Draw path line through waypoints
      ctx.beginPath();
      const first = result.path[0];
      ctx.moveTo(first.col * tileSize + tileSize / 2, first.row * tileSize + tileSize / 2);
      for (let i = 1; i < result.path.length; i++) {
        const step = result.path[i];
        ctx.lineTo(step.col * tileSize + tileSize / 2, step.row * tileSize + tileSize / 2);
      }
      ctx.strokeStyle = color;
      ctx.lineWidth = 2;
      ctx.setLineDash([6, 3]);
      ctx.stroke();
      ctx.setLineDash([]);
    } else {
      // No path found: draw direct line in red
      const startCx = startCol * tileSize + tileSize / 2;
      const startCy = startRow * tileSize + tileSize / 2;
      const endCx = dragCol * tileSize + tileSize / 2;
      const endCy = dragRow * tileSize + tileSize / 2;
      ctx.beginPath();
      ctx.moveTo(startCx, startCy);
      ctx.lineTo(endCx, endCy);
      ctx.strokeStyle = color;
      ctx.lineWidth = 2;
      ctx.setLineDash([6, 3]);
      ctx.stroke();
      ctx.setLineDash([]);
    }

    // Highlight target tile
    ctx.globalAlpha = 0.3;
    ctx.fillStyle = color;
    ctx.fillRect(dragCol * tileSize, dragRow * tileSize, tileSize, tileSize);
    ctx.globalAlpha = 1.0;

    // Distance text overlay
    const endCx = dragCol * tileSize + tileSize / 2;
    ctx.font = `bold ${Math.max(12, tileSize * 0.3)}px sans-serif`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'bottom';
    ctx.fillStyle = '#ffffff';
    ctx.strokeStyle = '#000000';
    ctx.lineWidth = 3;
    const label = blocked ? 'Blocked' : `${dist}ft`;
    const textX = endCx;
    const textY = dragRow * tileSize - 4;
    ctx.strokeText(label, textX, textY);
    ctx.fillText(label, textX, textY);
  }

  function drawMeasureLine(ctx, tileSize) {
    const startCx = measureStart.col * tileSize + tileSize / 2;
    const startCy = measureStart.row * tileSize + tileSize / 2;
    const endCx = measureEnd.col * tileSize + tileSize / 2;
    const endCy = measureEnd.row * tileSize + tileSize / 2;

    ctx.beginPath();
    ctx.moveTo(startCx, startCy);
    ctx.lineTo(endCx, endCy);
    ctx.strokeStyle = '#fbbf24';
    ctx.lineWidth = 2;
    ctx.setLineDash([4, 4]);
    ctx.stroke();
    ctx.setLineDash([]);

    const dist = gridDistance(measureStart.col, measureStart.row, measureEnd.col, measureEnd.row);
    const midX = (startCx + endCx) / 2;
    const midY = (startCy + endCy) / 2;
    ctx.font = `bold ${Math.max(12, tileSize * 0.3)}px sans-serif`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillStyle = '#fbbf24';
    ctx.strokeStyle = '#000000';
    ctx.lineWidth = 3;
    const label = `${dist}ft`;
    ctx.strokeText(label, midX, midY);
    ctx.fillText(label, midX, midY);
  }

  function getCanvasTile(e) {
    if (!canvasEl || !activeEncounter?.map) return null;

    const tiledMap = parseTiledMap(activeEncounter.map);
    if (!tiledMap) return null;

    const tileSize = getTileSize(tiledMap);
    const rect = canvasEl.getBoundingClientRect();
    const scaleX = canvasEl.width / rect.width;
    const scaleY = canvasEl.height / rect.height;
    const px = (e.clientX - rect.left) * scaleX;
    const py = (e.clientY - rect.top) * scaleY;
    return {
      col: Math.floor(px / tileSize),
      row: Math.floor(py / tileSize),
    };
  }

  function findCombatantAt(col, row) {
    return activeEncounter?.combatants?.find(c => {
      return colToIndex(c.position_col) === col && c.position_row === row;
    }) || null;
  }

  function handleCanvasMouseDown(e) {
    if (e.button !== 0) return; // left click only
    if (contextMenu) {
      contextMenu = null;
      return;
    }

    const tile = getCanvasTile(e);
    if (!tile) return;

    if (measureMode) return; // handled by click

    const comb = findCombatantAt(tile.col, tile.row);
    if (comb) {
      dragging = {
        combatantId: comb.id,
        startCol: tile.col,
        startRow: tile.row,
      };
      dragCol = tile.col;
      dragRow = tile.row;
    }
  }

  function handleCanvasMouseMove(e) {
    if (!dragging) return;
    const tile = getCanvasTile(e);
    if (!tile) return;

    if (tile.col !== dragCol || tile.row !== dragRow) {
      dragCol = tile.col;
      dragRow = tile.row;
    }
  }

  async function handleCanvasMouseUp(e) {
    if (!dragging) return;

    const startCol = dragging.startCol;
    const startRow = dragging.startRow;
    const combId = dragging.combatantId;

    // Only update if actually moved and path is valid
    if (dragCol !== startCol || dragRow !== startRow) {
      const tiledMap = parseTiledMap(activeEncounter.map);
      const walls = tiledMap ? getWalls(tiledMap) : [];
      const width = tiledMap?.width || 20;
      const height = tiledMap?.height || 15;
      const tileSize = getTileSize(tiledMap);
      const pathResult = findPath(startCol, startRow, dragCol, dragRow, walls, width, height, tileSize);

      if (!pathResult.found) {
        // Path blocked — cancel the move
        dragging = null;
        dragCol = null;
        dragRow = null;
        return;
      }

      try {
        await updateCombatantPosition(
          activeEncounter.id,
          combId,
          { position_col: indexToCol(dragCol), position_row: dragRow },
        );
        await loadWorkspace();
      } catch (err) {
        error = err.message;
      }
    }

    dragging = null;
    dragCol = null;
    dragRow = null;
  }

  function handleCanvasContextMenu(e) {
    e.preventDefault();
    const tile = getCanvasTile(e);
    if (!tile) return;

    const comb = findCombatantAt(tile.col, tile.row);
    if (!comb) {
      contextMenu = null;
      return;
    }

    contextMenu = {
      x: e.clientX,
      y: e.clientY,
      combatantId: comb.id,
    };
  }

  function handleContextAction(action) {
    if (!contextMenu) return;
    const combId = contextMenu.combatantId;
    selectedCombatantId = combId;
    contextMenu = null;

    if (action === 'damage') {
      damageInput = 0;
    } else if (action === 'heal') {
      healInput = 0;
    } else if (action === 'conditions') {
      conditionToAdd = '';
    } else if (action === 'remove') {
      handleRemoveCombatant(combId);
    }
  }

  async function handleRemoveCombatant(combId) {
    try {
      await removeCombatant(activeEncounter.id, combId);
      if (selectedCombatantId === combId) {
        selectedCombatantId = null;
      }
      await loadWorkspace();
    } catch (err) {
      error = err.message;
    }
  }

  function toggleMeasureMode() {
    measureMode = !measureMode;
    measureStart = null;
    measureEnd = null;
    if (measureMode) {
      selectedCombatantId = null;
    }
  }

  function handleCanvasClick(e) {
    if (contextMenu) {
      contextMenu = null;
      return;
    }

    const tile = getCanvasTile(e);
    if (!tile) return;

    // Measurement mode
    if (measureMode) {
      if (!measureStart) {
        measureStart = tile;
        measureEnd = null;
      } else {
        measureEnd = tile;
      }
      return;
    }

    // If we were dragging, don't re-select (mouseup handles it)
    if (dragging) return;

    // Find combatant at clicked tile
    const clicked = findCombatantAt(tile.col, tile.row);

    if (clicked) {
      selectedCombatantId = clicked.id;
      damageInput = 0;
      healInput = 0;
    } else {
      selectedCombatantId = null;
    }
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
  <!-- Encounter Overview Bar — Phase 105: one line per active encounter so
       the DM has cross-encounter awareness without switching tabs. -->
  <div class="encounter-overview" data-testid="encounter-overview">
    {#if encounters.length > 0}
      {#each encounters as enc (enc.id)}
        <div class="overview-row" data-testid="overview-row-{enc.id}">
          <span class="overview-item overview-name">{enc.display_name || enc.name}</span>
          <span class="overview-item">Round {enc.round_number}</span>
          <span class="overview-item">Turn: {enc.active_turn_combatant_name || '—'}</span>
          <span class="overview-item">{enc.combatants?.length || 0} combatants</span>
          {#if enc.pending_queue_count > 0}
            <span class="overview-item overview-badge">{enc.pending_queue_count} queued</span>
          {/if}
        </div>
      {/each}
    {:else}
      <span class="overview-item">No active encounters</span>
    {/if}
  </div>

  <!-- Encounter Tabs — Phase 105: show player-facing display_name with
       pending dm-queue badge. -->
  {#if encounters.length > 0}
    <div class="encounter-tabs" data-testid="encounter-tabs">
      {#each encounters as enc, i (enc.id)}
        <button
          class="tab-btn"
          class:active={i === activeEncounterIndex}
          onclick={() => { activeEncounterIndex = i; selectedCombatantId = null; }}
          data-testid="encounter-tab-{i}"
        >
          {enc.display_name || enc.name}
          {#if enc.pending_queue_count > 0}
            <span class="tab-badge" data-testid="tab-badge-{i}">{enc.pending_queue_count}</span>
          {/if}
        </button>
      {/each}
    </div>
  {/if}

  <!-- Phase 105: "HP updated to N by player action" indicator fed by
       wsClient snapshots through mergeSnapshot. -->
  {#if activeEncounter && pendingByEncounter[activeEncounter.id] && Object.keys(pendingByEncounter[activeEncounter.id]).length > 0}
    <div class="pending-banner" data-testid="pending-banner">
      {#each Object.entries(pendingByEncounter[activeEncounter.id]) as [field, value]}
        <span class="pending-item">{field} updated to {JSON.stringify(value)} by player action</span>
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
        <div class="map-toolbar">
          <button
            class="tool-btn"
            class:active={measureMode}
            onclick={toggleMeasureMode}
            data-testid="measure-tool-btn"
          >
            {measureMode ? 'Exit Measure' : 'Measure Distance'}
          </button>
          {#if measureMode && measureStart && measureEnd}
            <span class="measure-result" data-testid="measure-result">
              {gridDistance(measureStart.col, measureStart.row, measureEnd.col, measureEnd.row)}ft
            </span>
          {/if}
        </div>
        {#if activeEncounter.map}
          <canvas
            bind:this={canvasEl}
            class="combat-canvas"
            onclick={handleCanvasClick}
            onmousedown={handleCanvasMouseDown}
            onmousemove={handleCanvasMouseMove}
            onmouseup={handleCanvasMouseUp}
            oncontextmenu={handleCanvasContextMenu}
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

      <!-- Context menu -->
      {#if contextMenu}
        <div
          class="context-menu"
          style="left: {contextMenu.x}px; top: {contextMenu.y}px;"
          data-testid="context-menu"
        >
          <button class="context-item" onclick={() => handleContextAction('damage')} data-testid="ctx-damage">Damage</button>
          <button class="context-item" onclick={() => handleContextAction('heal')} data-testid="ctx-heal">Heal</button>
          <button class="context-item" onclick={() => handleContextAction('conditions')} data-testid="ctx-conditions">Conditions</button>
          <button class="context-item context-danger" onclick={() => handleContextAction('remove')} data-testid="ctx-remove">Remove from Encounter</button>
        </div>
      {/if}

      <!-- Right panel: Turn Queue, Action Resolver, and Tracker -->
      <div class="right-panel" data-testid="right-panel">
        <TurnQueue
          encounterId={activeEncounter.id}
          activeTurnCombatantId={activeEncounter.active_turn_combatant_id}
          onTurnAdvanced={loadWorkspace}
        />

        <ActionResolver
          encounterId={activeEncounter.id}
          combatants={activeEncounter.combatants}
          onResolved={loadWorkspace}
        />

        <ActiveReactionsPanel
          encounterId={activeEncounter.id}
          activeTurnCombatantId={activeEncounter.active_turn_combatant_id}
          activeTurnIsNpc={activeEncounter.combatants?.find(c => c.id === activeEncounter.active_turn_combatant_id)?.is_npc || false}
          onReactionResolved={loadWorkspace}
        />

        <ActionLogViewer encounterId={activeEncounter.id} />

        <!-- DM Dashboard: Undo & Manual Overrides (Phase 97b) -->
        <div class="dm-override-panel" data-testid="dm-override-panel">
          <div class="dm-override-row">
            <input
              type="text"
              placeholder="Undo reason (optional)"
              bind:value={dmUndoReason}
              data-testid="undo-reason-input"
            />
            <button onclick={handleUndoLastAction} data-testid="undo-last-action-btn">
              Undo Last Action
            </button>
          </div>
          <button
            class="dm-override-toggle"
            onclick={() => (dmOverrideOpen = !dmOverrideOpen)}
            data-testid="dm-override-toggle"
          >
            {dmOverrideOpen ? '▼' : '▶'} Manual Override
          </button>
          {#if dmOverrideOpen}
            <div class="dm-override-content" data-testid="dm-override-content">
              {#if !selectedCombatant}
                <p class="dm-override-hint">Select a combatant to enable manual overrides.</p>
              {:else}
                <p>Overriding: <strong>{selectedCombatant.display_name}</strong></p>
                <textarea
                  placeholder="Reason (required for clarity)"
                  bind:value={dmOverrideReason}
                  rows="2"
                  data-testid="override-reason-input"
                ></textarea>

                <fieldset>
                  <legend>HP</legend>
                  <label>Current: <input type="number" bind:value={dmOverrideHpInput} data-testid="override-hp-current" /></label>
                  <label>Temp: <input type="number" bind:value={dmOverrideTempHpInput} data-testid="override-hp-temp" /></label>
                  <button onclick={handleOverrideHP} data-testid="override-hp-btn">Apply HP</button>
                </fieldset>

                <fieldset>
                  <legend>Position</legend>
                  <label>Col: <input type="text" bind:value={dmOverridePosCol} data-testid="override-pos-col" /></label>
                  <label>Row: <input type="number" bind:value={dmOverridePosRow} data-testid="override-pos-row" /></label>
                  <label>Altitude (ft): <input type="number" bind:value={dmOverrideAltitude} data-testid="override-pos-altitude" /></label>
                  <button onclick={handleOverridePosition} data-testid="override-pos-btn">Apply Position</button>
                </fieldset>

                <fieldset>
                  <legend>Conditions (JSON array)</legend>
                  <textarea bind:value={dmOverrideConditionsText} rows="2" data-testid="override-conditions"></textarea>
                  <button onclick={handleOverrideConditions} data-testid="override-conditions-btn">Apply Conditions</button>
                </fieldset>

                <fieldset>
                  <legend>Initiative</legend>
                  <label>Roll: <input type="number" bind:value={dmOverrideInitiativeRoll} data-testid="override-init-roll" /></label>
                  <label>Order: <input type="number" bind:value={dmOverrideInitiativeOrder} data-testid="override-init-order" /></label>
                  <button onclick={handleOverrideInitiative} data-testid="override-init-btn">Apply Initiative</button>
                </fieldset>

                {#if selectedCombatant.character_id}
                  <fieldset>
                    <legend>Spell Slots (JSON)</legend>
                    <textarea bind:value={dmOverrideSpellSlotsText} rows="3" data-testid="override-spell-slots"></textarea>
                    <button onclick={handleOverrideSpellSlots} data-testid="override-spell-slots-btn">Apply Spell Slots</button>
                  </fieldset>
                {/if}
              {/if}
              {#if dmOverrideMessage}
                <p class="dm-override-message" data-testid="dm-override-message">{dmOverrideMessage}</p>
              {/if}
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
    flex-direction: column;
    gap: 0.25rem;
    padding: 0.5rem 1rem;
    background: #16213e;
    border-radius: 4px;
    border: 1px solid #0f3460;
    font-size: 0.9rem;
  }

  .overview-row {
    display: flex;
    gap: 1.5rem;
  }

  .overview-item {
    color: #a0aec0;
  }

  .overview-name {
    color: #e0e0e0;
    font-weight: 600;
  }

  .overview-badge {
    background: #e53e3e;
    color: white;
    padding: 0 0.5rem;
    border-radius: 999px;
    font-size: 0.75rem;
  }

  .tab-badge {
    display: inline-block;
    margin-left: 0.35rem;
    background: #e53e3e;
    color: white;
    padding: 0 0.5rem;
    border-radius: 999px;
    font-size: 0.7rem;
  }

  .pending-banner {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
    padding: 0.35rem 0.75rem;
    background: #2b2440;
    border: 1px solid #6b46c1;
    border-radius: 4px;
    font-size: 0.8rem;
    color: #d6bcfa;
  }

  .pending-item {
    font-family: monospace;
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

  .right-panel {
    flex: 0 0 38%;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    max-height: calc(100vh - 150px);
    overflow-y: auto;
  }

  .tracker-panel {
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

  .map-toolbar {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    margin-bottom: 0.5rem;
  }

  .tool-btn {
    padding: 0.3rem 0.8rem;
    background: #0f3460;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
  }

  .tool-btn:hover {
    background: #e94560;
    border-color: #e94560;
  }

  .tool-btn.active {
    background: #fbbf24;
    color: #1a1a2e;
    border-color: #fbbf24;
  }

  .measure-result {
    color: #fbbf24;
    font-weight: bold;
  }

  .context-menu {
    position: fixed;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 4px;
    z-index: 100;
    min-width: 160px;
    box-shadow: 0 4px 12px rgba(0,0,0,0.5);
  }

  .context-item {
    display: block;
    width: 100%;
    padding: 0.5rem 1rem;
    background: none;
    border: none;
    color: #e0e0e0;
    text-align: left;
    cursor: pointer;
    font-size: 0.85rem;
  }

  .context-item:hover {
    background: #0f3460;
  }

  .context-danger {
    color: #ef4444;
  }

  .context-danger:hover {
    background: rgba(239, 68, 68, 0.2);
  }
</style>
