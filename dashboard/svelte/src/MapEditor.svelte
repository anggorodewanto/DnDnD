<script>
  import { createMap, getMap, updateMap, uploadAsset } from './lib/api.js';
  import {
    TERRAIN_TYPES,
    terrainByGid,
    generateBlankMap,
    setTerrain,
    addWall,
    removeWall,
    getWalls,
    validateDimensions,
  } from './lib/mapdata.js';

  let { campaignId, mapId = null, onback } = $props();

  // Map state
  let mapName = $state('New Map');
  let mapWidth = $state(20);
  let mapHeight = $state(15);
  let tiledMap = $state(null);
  let savedMapId = $state(null);
  let dirty = $state(false);

  // Background image state
  let backgroundImageId = $state(null);
  let backgroundImageUrl = $state(null);
  let backgroundImage = $state(null); // HTMLImageElement
  let backgroundOpacity = $state(0.5);
  let uploadingImage = $state(false);

  // Tool state
  let activeTool = $state('terrain');
  let selectedTerrain = $state('open_ground');

  // UI state
  let loading = $state(false);
  let saving = $state(false);
  let error = $state(null);
  let statusMsg = $state('');
  let showNewMapForm = $state(!mapId && !tiledMap);

  // Canvas ref
  let canvasEl = $state(null);

  // File input ref
  let fileInputEl = $state(null);

  // Mouse state for painting
  let isPainting = $state(false);

  // Sync savedMapId from prop and load existing map
  $effect(() => {
    if (mapId) {
      savedMapId = mapId;
      loadMap(mapId);
    }
  });

  // Redraw canvas when map changes
  $effect(() => {
    if (tiledMap && canvasEl) {
      drawMap();
    }
  });

  async function loadMap(id) {
    loading = true;
    error = null;
    try {
      const data = await getMap(id);
      mapName = data.name;
      mapWidth = data.width;
      mapHeight = data.height;
      tiledMap = typeof data.tiled_json === 'string' ? JSON.parse(data.tiled_json) : data.tiled_json;
      savedMapId = data.id;
      if (data.background_image_id) {
        backgroundImageId = data.background_image_id;
        backgroundImageUrl = `/api/assets/${data.background_image_id}`;
        loadBackgroundImage(backgroundImageUrl);
      }
      showNewMapForm = false;
      dirty = false;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function createNewMap() {
    const dimError = validateDimensions(mapWidth, mapHeight);
    if (dimError) {
      error = dimError;
      return;
    }
    error = null;
    tiledMap = generateBlankMap(mapWidth, mapHeight);
    showNewMapForm = false;
    dirty = true;
  }

  async function saveMap() {
    if (!tiledMap) return;

    saving = true;
    error = null;
    statusMsg = '';
    try {
      if (savedMapId) {
        const updatePayload = {
          name: mapName,
          width: mapWidth,
          height: mapHeight,
          tiled_json: tiledMap,
        };
        if (backgroundImageId) {
          updatePayload.background_image_id = backgroundImageId;
        }
        await updateMap(savedMapId, updatePayload);
        statusMsg = 'Map saved.';
      } else {
        const createPayload = {
          campaign_id: campaignId,
          name: mapName,
          width: mapWidth,
          height: mapHeight,
          tiled_json: tiledMap,
        };
        if (backgroundImageId) {
          createPayload.background_image_id = backgroundImageId;
        }
        const result = await createMap(createPayload);
        savedMapId = result.id;
        statusMsg = 'Map created.';
      }
      dirty = false;
    } catch (e) {
      error = e.message;
    } finally {
      saving = false;
    }
  }

  function loadBackgroundImage(url) {
    const img = new Image();
    img.onload = () => {
      backgroundImage = img;
      drawMap();
    };
    img.onerror = () => {
      error = 'Failed to load background image';
    };
    img.src = url;
  }

  async function handleImageUpload(e) {
    const file = e.target.files?.[0];
    if (!file) return;

    // Validate file type
    if (!file.type.startsWith('image/')) {
      error = 'Please select an image file (PNG or JPG)';
      return;
    }

    uploadingImage = true;
    error = null;
    try {
      const result = await uploadAsset({
        campaignId: campaignId,
        type: 'map_background',
        file,
      });
      backgroundImageId = result.id;
      backgroundImageUrl = result.url;
      loadBackgroundImage(result.url);
      dirty = true;
    } catch (e) {
      error = e.message;
    } finally {
      uploadingImage = false;
    }
  }

  function handleOpacityChange(e) {
    backgroundOpacity = parseFloat(e.target.value);
    drawMap();
  }

  function getTileSize() {
    return tiledMap?.tilewidth || 48;
  }

  function drawMap() {
    if (!canvasEl || !tiledMap) return;

    const tileSize = getTileSize();
    const ctx = canvasEl.getContext('2d');
    canvasEl.width = tiledMap.width * tileSize;
    canvasEl.height = tiledMap.height * tileSize;

    // Draw background image if present
    if (backgroundImage) {
      ctx.globalAlpha = backgroundOpacity;
      ctx.drawImage(backgroundImage, 0, 0, canvasEl.width, canvasEl.height);
      ctx.globalAlpha = 1.0;
    }

    // Draw terrain (semi-transparent if background image present)
    const terrainLayer = tiledMap.layers?.find(l => l.name === 'terrain');
    if (terrainLayer?.data) {
      if (backgroundImage) {
        ctx.globalAlpha = 0.4;
      }
      for (let y = 0; y < tiledMap.height; y++) {
        for (let x = 0; x < tiledMap.width; x++) {
          const idx = y * tiledMap.width + x;
          const gid = terrainLayer.data[idx] || 1;
          const terrain = terrainByGid(gid);

          ctx.fillStyle = terrain.color;
          ctx.fillRect(x * tileSize, y * tileSize, tileSize, tileSize);
        }
      }
      ctx.globalAlpha = 1.0;

      // Grid lines always at full opacity
      for (let y = 0; y < tiledMap.height; y++) {
        for (let x = 0; x < tiledMap.width; x++) {
          ctx.strokeStyle = 'rgba(255,255,255,0.15)';
          ctx.lineWidth = 1;
          ctx.strokeRect(x * tileSize, y * tileSize, tileSize, tileSize);
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
        // Horizontal wall
        ctx.moveTo(wall.x, wall.y);
        ctx.lineTo(wall.x + wall.width, wall.y);
      } else if (wall.height > 0) {
        // Vertical wall
        ctx.moveTo(wall.x, wall.y);
        ctx.lineTo(wall.x, wall.y + wall.height);
      }
      ctx.stroke();
    }
  }

  function canvasPixel(e) {
    const rect = canvasEl.getBoundingClientRect();
    const scaleX = canvasEl.width / rect.width;
    const scaleY = canvasEl.height / rect.height;
    return {
      px: (e.clientX - rect.left) * scaleX,
      py: (e.clientY - rect.top) * scaleY,
    };
  }

  function handleCanvasMouseDown(e) {
    if (!tiledMap) return;

    const { px, py } = canvasPixel(e);

    if (activeTool === 'terrain') {
      isPainting = true;
      paintTerrain(px, py);
    } else if (activeTool === 'wall') {
      placeWall(px, py);
    } else if (activeTool === 'eraseWall') {
      eraseWall(px, py);
    }
  }

  function handleCanvasMouseMove(e) {
    if (!isPainting || activeTool !== 'terrain') return;

    const { px, py } = canvasPixel(e);
    paintTerrain(px, py);
  }

  function handleCanvasMouseUp() {
    isPainting = false;
  }

  function paintTerrain(px, py) {
    const tileSize = getTileSize();
    const tx = Math.floor(px / tileSize);
    const ty = Math.floor(py / tileSize);

    if (tx < 0 || tx >= tiledMap.width || ty < 0 || ty >= tiledMap.height) return;

    const gid = TERRAIN_TYPES[selectedTerrain]?.gid || 1;
    tiledMap = setTerrain(tiledMap, tx, ty, gid);
    dirty = true;
    drawMap();
  }

  function placeWall(px, py) {
    const tileSize = getTileSize();

    // Snap to nearest edge
    const gridX = Math.round(px / tileSize) * tileSize;
    const gridY = Math.round(py / tileSize) * tileSize;

    // Determine if horizontal or vertical based on proximity
    const dx = Math.abs(px - gridX);
    const dy = Math.abs(py - gridY);

    let wallX, wallY, orientation;
    if (dy < dx) {
      // Closer to horizontal edge
      wallX = Math.floor(px / tileSize) * tileSize;
      wallY = gridY;
      orientation = 'horizontal';
    } else {
      // Closer to vertical edge
      wallX = gridX;
      wallY = Math.floor(py / tileSize) * tileSize;
      orientation = 'vertical';
    }

    tiledMap = addWall(tiledMap, wallX, wallY, orientation);
    dirty = true;
    drawMap();
  }

  function eraseWall(px, py) {
    const tileSize = getTileSize();
    tiledMap = removeWall(tiledMap, px, py, tileSize / 4);
    dirty = true;
    drawMap();
  }
</script>

<div class="editor">
  {#if loading}
    <p>Loading map...</p>
  {:else if showNewMapForm}
    <div class="new-map-form">
      <h2>Create New Map</h2>
      <div class="form-row">
        <label>
          Name:
          <input type="text" bind:value={mapName} />
        </label>
      </div>
      <div class="form-row">
        <label>
          Width (squares):
          <input type="number" bind:value={mapWidth} min="1" max="200" />
        </label>
        <label>
          Height (squares):
          <input type="number" bind:value={mapHeight} min="1" max="200" />
        </label>
      </div>
      {#if mapWidth > 100 || mapHeight > 100}
        <p class="warning">Large map: tile size will be auto-downscaled to 32px.</p>
      {/if}
      {#if error}
        <p class="error">{error}</p>
      {/if}
      <button class="primary-btn" onclick={createNewMap}>Create Map</button>
    </div>
  {:else}
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-section">
        <label>
          Name:
          <input type="text" bind:value={mapName} oninput={() => dirty = true} />
        </label>
      </div>

      <div class="toolbar-section">
        <span class="section-label">Tool:</span>
        <button
          class:active={activeTool === 'terrain'}
          onclick={() => activeTool = 'terrain'}
        >Terrain</button>
        <button
          class:active={activeTool === 'wall'}
          onclick={() => activeTool = 'wall'}
        >Wall</button>
        <button
          class:active={activeTool === 'eraseWall'}
          onclick={() => activeTool = 'eraseWall'}
        >Erase Wall</button>
      </div>

      {#if activeTool === 'terrain'}
        <div class="toolbar-section terrain-palette">
          <span class="section-label">Terrain:</span>
          {#each Object.entries(TERRAIN_TYPES) as [key, terrain]}
            <button
              class="terrain-btn"
              class:active={selectedTerrain === key}
              onclick={() => selectedTerrain = key}
              style="background: {terrain.color}"
              title={terrain.label}
            >{terrain.label}</button>
          {/each}
        </div>
      {/if}

      <div class="toolbar-section">
        <input
          type="file"
          accept="image/png,image/jpeg"
          style="display:none"
          bind:this={fileInputEl}
          onchange={handleImageUpload}
        />
        <button
          class="import-btn"
          onclick={() => fileInputEl?.click()}
          disabled={uploadingImage}
        >{uploadingImage ? 'Uploading...' : 'Import Image'}</button>
        {#if backgroundImage}
          <label class="opacity-label">
            Opacity:
            <input
              type="range"
              min="0"
              max="1"
              step="0.05"
              value={backgroundOpacity}
              oninput={handleOpacityChange}
            />
            <span>{Math.round(backgroundOpacity * 100)}%</span>
          </label>
        {/if}
      </div>

      <div class="toolbar-section">
        <button class="save-btn" onclick={saveMap} disabled={saving || !dirty}>
          {saving ? 'Saving...' : 'Save'}
        </button>
        {#if statusMsg}
          <span class="status">{statusMsg}</span>
        {/if}
        {#if dirty}
          <span class="dirty-indicator">*unsaved</span>
        {/if}
      </div>
    </div>

    {#if error}
      <p class="error">{error}</p>
    {/if}

    <!-- Canvas -->
    <div class="canvas-container">
      <canvas
        bind:this={canvasEl}
        onmousedown={handleCanvasMouseDown}
        onmousemove={handleCanvasMouseMove}
        onmouseup={handleCanvasMouseUp}
        onmouseleave={handleCanvasMouseUp}
      ></canvas>
    </div>

    <div class="info-bar">
      <span>{mapWidth} x {mapHeight} squares</span>
      <span>Tile size: {getTileSize()}px</span>
      {#if savedMapId}
        <span>ID: {savedMapId}</span>
      {/if}
    </div>
  {/if}
</div>

<style>
  .editor {
    width: 100%;
  }

  .new-map-form {
    max-width: 400px;
    background: #16213e;
    padding: 1.5rem;
    border-radius: 8px;
    border: 1px solid #0f3460;
  }

  .new-map-form h2 {
    color: #e94560;
    margin: 0 0 1rem;
  }

  .form-row {
    display: flex;
    gap: 1rem;
    margin-bottom: 1rem;
  }

  .form-row label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .form-row input {
    padding: 0.5rem;
    background: #1a1a2e;
    border: 1px solid #0f3460;
    color: #e0e0e0;
    border-radius: 4px;
  }

  .toolbar {
    display: flex;
    flex-wrap: wrap;
    gap: 1rem;
    align-items: center;
    padding: 0.75rem;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    margin-bottom: 0.5rem;
  }

  .toolbar-section {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .section-label {
    font-size: 0.85rem;
    color: #888;
  }

  .toolbar button {
    padding: 0.4rem 0.8rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }

  .toolbar button:hover {
    background: #0f3460;
  }

  .toolbar button.active {
    background: #e94560;
    border-color: #e94560;
    color: white;
  }

  .terrain-btn {
    font-size: 0.8rem;
    min-width: 80px;
    text-align: center;
    color: white !important;
    text-shadow: 1px 1px 2px rgba(0,0,0,0.8);
  }

  .toolbar input {
    padding: 0.4rem;
    background: #1a1a2e;
    border: 1px solid #0f3460;
    color: #e0e0e0;
    border-radius: 4px;
    width: 150px;
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

  .status {
    color: #28a745;
    font-size: 0.85rem;
  }

  .dirty-indicator {
    color: #ffc107;
    font-size: 0.85rem;
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

  .primary-btn {
    padding: 0.75rem 1.5rem;
    background: #e94560;
    color: white;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    font-size: 1rem;
  }

  .primary-btn:hover {
    background: #c73852;
  }

  .warning {
    color: #ffc107;
    font-size: 0.9rem;
  }

  .error {
    color: #ff4444;
  }

  .import-btn {
    background: #17a2b8 !important;
    border-color: #17a2b8 !important;
    color: white !important;
  }

  .import-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed !important;
  }

  .opacity-label {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.85rem;
    color: #ccc;
  }

  .opacity-label input[type="range"] {
    width: 80px;
  }
</style>
