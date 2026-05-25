<script>
  import { createMap, getMap, updateMap, uploadAsset, importTiledMap } from './lib/api.js';
  import { decodeGID, tilesetForGID, tileSrcRect } from './lib/tiledSprites.js';
  import {
    TERRAIN_TYPES,
    terrainByGid,
    LIGHTING_TYPES,
    lightingByGid,
    ELEVATION_MAX,
    generateBlankMap,
    setTerrain,
    setLighting,
    getLightingData,
    setElevation,
    getElevationData,
    addWall,
    removeWall,
    getWalls,
    addSpawnZone,
    getSpawnZones,
    removeSpawnZone,
    validateDimensions,
    cloneMap,
    extractRegion,
    pasteRegion,
    duplicateMap,
    UndoStack,
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
  let selectedLighting = $state('dim_light');
  let selectedElevation = $state(1);
  let selectedSpawnType = $state('player');

  // UI state
  let loading = $state(false);
  let saving = $state(false);
  let error = $state(null);
  let statusMsg = $state('');
  let showNewMapForm = $state(!mapId && !tiledMap);

  // Canvas ref
  let canvasEl = $state(null);

  // F-7: cache of HTMLImageElements keyed by asset URL, shared by sprite
  // tilesets and image layers so we never reload an image per frame. Loading
  // is async; each load triggers a single redraw once decoded. Not reactive
  // state — it's a render-side cache the canvas reads directly.
  const spriteImageCache = new Map();

  // File input ref
  let fileInputEl = $state(null);

  // F-7: Tiled .tmj import state + file input ref.
  let tmjFileInputEl = $state(null);
  let importingTmj = $state(false);
  let skippedFeatures = $state(null);

  // Mouse state for painting
  let isPainting = $state(false);

  // Spawn zone drag state
  let spawnDragStart = $state(null);
  let spawnDragEnd = $state(null);

  // Undo/redo
  let undoStack = new UndoStack();

  // Selection state
  let selectDragStart = $state(null);
  let selectDragEnd = $state(null);
  let selectionRect = $state(null); // { x, y, width, height } in tiles

  // Copy/paste state
  let clipboard = $state(null); // region data
  let pastePreview = $state(null); // { tx, ty } tile position for paste preview

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
      const data = await getMap(id, campaignId);
      mapName = data.name;
      mapWidth = data.width;
      mapHeight = data.height;
      tiledMap = typeof data.tiled_json === 'string' ? JSON.parse(data.tiled_json) : data.tiled_json;
      // F-18: Restore backgroundOpacity from tiled_json.
      if (tiledMap.backgroundOpacity != null) {
        backgroundOpacity = tiledMap.backgroundOpacity;
      }
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
      // F-18: Persist backgroundOpacity inside tiled_json.
      tiledMap.backgroundOpacity = backgroundOpacity;
      const payload = {
        name: mapName,
        width: mapWidth,
        height: mapHeight,
        tiled_json: tiledMap,
      };
      if (backgroundImageId) {
        payload.background_image_id = backgroundImageId;
      }

      if (savedMapId) {
        await updateMap(savedMapId, campaignId, payload);
        statusMsg = 'Map saved.';
      } else {
        payload.campaign_id = campaignId;
        const result = await createMap(payload);
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

  // F-7: handle a multi-file Tiled-project selection. The user picks the .tmj
  // together with every tileset/image-layer image it references. We split the
  // selection into the single .tmj (extension .tmj or .json) and the image
  // files, POST them as multipart/form-data, then load the persisted map into
  // the editor so the DM can review/save it.
  async function handleTmjImport(e) {
    const files = Array.from(e.target.files || []);
    if (files.length === 0) return;

    const tmjFile = files.find((f) => /\.(tmj|json)$/i.test(f.name));
    if (!tmjFile) {
      error = 'Select a .tmj (or .json) file along with its images.';
      if (tmjFileInputEl) tmjFileInputEl.value = '';
      return;
    }
    const imageFiles = files.filter((f) => f !== tmjFile);

    importingTmj = true;
    error = null;
    skippedFeatures = null;
    statusMsg = '';
    try {
      const result = await importTiledMap({
        campaignId,
        name: mapName || tmjFile.name.replace(/\.(tmj|json)$/i, ''),
        tmjFile,
        imageFiles,
      });
      // Backend returns { map, skipped }. Load the persisted map into the
      // editor view by reusing the same flow GetMap uses.
      const m = result.map;
      mapName = m.name;
      mapWidth = m.width;
      mapHeight = m.height;
      tiledMap = typeof m.tiled_json === 'string' ? JSON.parse(m.tiled_json) : m.tiled_json;
      savedMapId = m.id;
      if (m.background_image_id) {
        backgroundImageId = m.background_image_id;
        backgroundImageUrl = `/api/assets/${m.background_image_id}`;
        loadBackgroundImage(backgroundImageUrl);
      }
      showNewMapForm = false;
      dirty = false;
      skippedFeatures = Array.isArray(result.skipped) ? result.skipped : [];
      statusMsg = `Imported "${m.name}".`;
    } catch (err) {
      // The backend's missing-image 400 is a plain-text body; apiFetch
      // surfaces it as err.message.
      error = err.message;
    } finally {
      importingTmj = false;
      // Reset the input so selecting the same files again re-triggers change.
      if (tmjFileInputEl) tmjFileInputEl.value = '';
    }
  }

  function handleOpacityChange(e) {
    backgroundOpacity = parseFloat(e.target.value);
    drawMap();
  }

  function pushUndo() {
    if (!tiledMap) return;
    undoStack.push(cloneMap(tiledMap));
  }

  function performUndo() {
    if (!tiledMap) return;
    const prev = undoStack.undo(cloneMap(tiledMap));
    if (!prev) return;
    tiledMap = prev;
    dirty = true;
    drawMap();
  }

  function performRedo() {
    if (!tiledMap) return;
    const next = undoStack.redo(cloneMap(tiledMap));
    if (!next) return;
    tiledMap = next;
    dirty = true;
    drawMap();
  }

  function copySelection() {
    if (!tiledMap || !selectionRect) return;
    clipboard = extractRegion(tiledMap, selectionRect.x, selectionRect.y, selectionRect.width, selectionRect.height);
  }

  function startPaste() {
    if (!clipboard) return;
    activeTool = 'paste';
    pastePreview = null;
  }

  function performPaste(tx, ty) {
    if (!tiledMap || !clipboard) return;
    pushUndo();
    tiledMap = pasteRegion(tiledMap, clipboard, tx, ty);
    dirty = true;
    pastePreview = null;
    activeTool = 'select';
    drawMap();
  }

  function performDuplicate() {
    if (!tiledMap) return;
    tiledMap = duplicateMap(tiledMap);
    mapName = mapName + ' (copy)';
    savedMapId = null;
    dirty = true;
    undoStack.clear();
    drawMap();
  }

  function handleKeydown(e) {
    if (!tiledMap) return;

    // Ctrl+Z = undo, Ctrl+Shift+Z = redo
    if (e.ctrlKey && e.key === 'z' && !e.shiftKey) {
      e.preventDefault();
      performUndo();
      return;
    }
    if (e.ctrlKey && e.key === 'Z' && e.shiftKey) {
      e.preventDefault();
      performRedo();
      return;
    }
    // Ctrl+C = copy
    if (e.ctrlKey && e.key === 'c') {
      e.preventDefault();
      copySelection();
      return;
    }
    // Ctrl+V = paste
    if (e.ctrlKey && e.key === 'v') {
      e.preventDefault();
      startPaste();
      return;
    }
  }

  function getTileSize() {
    return tiledMap?.tilewidth || 48;
  }

  // F-7: report whether the loaded map carries real tileset/image art. When it
  // does, the terrain tint paints translucent so the art shows through (mirrors
  // the Go renderer's MapData.HasSpriteArt). Abstract color-terrain maps return
  // false and keep rendering opaque fills as before.
  function drawnSpriteArt() {
    if (!tiledMap) return false;
    const tilesets = tiledMap.tilesets;
    const hasImageTileset =
      Array.isArray(tilesets) && tilesets.some((ts) => typeof ts?.image === 'string' && ts.image);
    const layers = tiledMap.layers || [];
    const hasImageLayer = layers.some((l) => l.type === 'imagelayer' && l.image);
    return hasImageTileset || hasImageLayer;
  }

  // F-7: get a cached HTMLImageElement for an asset URL, kicking off an async
  // load on first request. Returns the element only once it has decoded
  // (img.complete && naturalWidth > 0); otherwise null so the caller skips it
  // this frame. A redraw is triggered on load so the tiles appear.
  function getSpriteImage(url) {
    if (!url) return null;
    let img = spriteImageCache.get(url);
    if (!img) {
      img = new Image();
      img.onload = () => drawMap();
      img.src = url;
      spriteImageCache.set(url, img);
    }
    return img.complete && img.naturalWidth > 0 ? img : null;
  }

  // F-7: blit one tile, applying Tiled's H/V/diagonal flip flags. The diagonal
  // flag rotates the tile in combination with H/V (Tiled's anti-diagonal
  // transform). We translate to the destination cell center, apply the
  // transform, then draw the source rect centered.
  function drawTile(ctx, img, src, dx, dy, dw, dh, flip) {
    const noFlip = !flip.flipH && !flip.flipV && !flip.flipD;
    if (noFlip) {
      ctx.drawImage(img, src.sx, src.sy, src.sw, src.sh, dx, dy, dw, dh);
      return;
    }
    ctx.save();
    // Move origin to the cell center so scale/rotate pivot there.
    ctx.translate(dx + dw / 2, dy + dh / 2);
    if (flip.flipD) {
      // Anti-diagonal flip = transpose. Compose with H/V per Tiled's scheme.
      ctx.rotate(Math.PI / 2);
      ctx.scale(flip.flipV ? -1 : 1, flip.flipH ? -1 : 1);
    } else {
      ctx.scale(flip.flipH ? -1 : 1, flip.flipV ? -1 : 1);
    }
    ctx.drawImage(img, src.sx, src.sy, src.sw, src.sh, -dw / 2, -dh / 2, dw, dh);
    ctx.restore();
  }

  // F-7: render imported visual tile layers (real tileset art) beneath the
  // editor's semantic overlays. Each `tilelayer` whose GIDs resolve to an
  // image-backed tileset is blitted cell-by-cell. Abstract (color-terrain)
  // maps have no image-backed tilesets, so this is a no-op for them.
  function drawSpriteLayers(ctx, tileSize) {
    const tilesets = tiledMap.tilesets;
    if (!Array.isArray(tilesets) || tilesets.length === 0) return;
    const imageTilesets = tilesets.filter((ts) => typeof ts?.image === 'string' && ts.image);
    if (imageTilesets.length === 0) return;

    const layers = tiledMap.layers || [];
    for (const layer of layers) {
      if (layer.type !== 'tilelayer' || !Array.isArray(layer.data)) continue;
      // Skip the editor's own semantic layers — they carry small terrain/
      // lighting/elevation GIDs, not tileset art.
      if (layer.name === 'terrain' || layer.name === 'lighting' || layer.name === 'elevation') {
        continue;
      }
      const lw = layer.width || tiledMap.width;
      for (let i = 0; i < layer.data.length; i++) {
        const flip = decodeGID(layer.data[i]);
        if (flip.id === 0) continue; // empty cell
        const ts = tilesetForGID(imageTilesets, flip.id);
        if (!ts) continue;
        const img = getSpriteImage(ts.image);
        if (!img) continue; // still loading — redraw fires when decoded
        const src = tileSrcRect(ts, flip.id);
        const cx = (i % lw) * tileSize;
        const cy = Math.floor(i / lw) * tileSize;
        drawTile(ctx, img, src, cx, cy, tileSize, tileSize, flip);
      }
    }
  }

  // F-7: render Tiled image layers at their pixel offset, scaled from the map's
  // original tile-pixel space into the active tile size.
  function drawImageLayers(ctx, tileSize) {
    const layers = tiledMap.layers || [];
    const baseTile = tiledMap.tilewidth || tileSize;
    const scale = tileSize / baseTile;
    for (const layer of layers) {
      if (layer.type !== 'imagelayer' || !layer.image) continue;
      const img = getSpriteImage(layer.image);
      if (!img) continue;
      const ox = (layer.offsetx || 0) * scale;
      const oy = (layer.offsety || 0) * scale;
      ctx.drawImage(img, ox, oy, img.naturalWidth * scale, img.naturalHeight * scale);
    }
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

    // F-7: blit imported tileset/image art beneath the semantic overlays.
    // No-op for abstract color-terrain maps (no image-backed tilesets).
    drawSpriteLayers(ctx, tileSize);
    drawImageLayers(ctx, tileSize);
    const hasSpriteArt = drawnSpriteArt();

    // Draw terrain (semi-transparent if background image or sprite art present
    // so the underlying art shows through).
    const terrainLayer = tiledMap.layers?.find(l => l.name === 'terrain');
    if (terrainLayer?.data) {
      if (backgroundImage || hasSpriteArt) {
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

    // Draw lighting overlay
    const lightingData = getLightingData(tiledMap);
    if (lightingData.length > 0) {
      for (let y = 0; y < tiledMap.height; y++) {
        for (let x = 0; x < tiledMap.width; x++) {
          const idx = y * tiledMap.width + x;
          const gid = lightingData[idx];
          if (gid === 0) continue; // normal — no overlay
          const lighting = lightingByGid(gid);
          ctx.fillStyle = lighting.color;
          ctx.globalAlpha = 0.4;
          ctx.fillRect(x * tileSize, y * tileSize, tileSize, tileSize);
          ctx.globalAlpha = 1.0;
        }
      }
    }

    // Draw elevation labels
    const elevationData = getElevationData(tiledMap);
    if (elevationData.length > 0) {
      ctx.font = `${Math.max(10, tileSize * 0.3)}px sans-serif`;
      ctx.textAlign = 'right';
      ctx.textBaseline = 'bottom';
      for (let y = 0; y < tiledMap.height; y++) {
        for (let x = 0; x < tiledMap.width; x++) {
          const idx = y * tiledMap.width + x;
          const elev = elevationData[idx];
          if (elev === 0) continue; // ground level — no label
          ctx.fillStyle = 'rgba(255,255,255,0.9)';
          ctx.strokeStyle = 'rgba(0,0,0,0.8)';
          ctx.lineWidth = 2;
          const text = `E${elev}`;
          const tx = (x + 1) * tileSize - 2;
          const ty = (y + 1) * tileSize - 2;
          ctx.strokeText(text, tx, ty);
          ctx.fillText(text, tx, ty);
        }
      }
    }

    // Draw spawn zones
    const spawnZones = getSpawnZones(tiledMap);
    for (const zone of spawnZones) {
      const isPlayer = zone.type === 'player';
      ctx.fillStyle = isPlayer ? 'rgba(0, 128, 255, 0.25)' : 'rgba(255, 64, 64, 0.25)';
      ctx.fillRect(zone.x, zone.y, zone.width, zone.height);
      ctx.strokeStyle = isPlayer ? '#0080ff' : '#ff4040';
      ctx.lineWidth = 2;
      ctx.setLineDash([6, 3]);
      ctx.strokeRect(zone.x, zone.y, zone.width, zone.height);
      ctx.setLineDash([]);

      // Label
      ctx.font = `${Math.max(10, tileSize * 0.28)}px sans-serif`;
      ctx.textAlign = 'left';
      ctx.textBaseline = 'top';
      ctx.fillStyle = isPlayer ? '#0080ff' : '#ff4040';
      ctx.fillText(isPlayer ? 'Player' : 'Enemy', zone.x + 3, zone.y + 3);
    }

    // Draw spawn zone drag preview
    if (spawnDragStart && spawnDragEnd) {
      const sx = Math.min(spawnDragStart.tx, spawnDragEnd.tx);
      const sy = Math.min(spawnDragStart.ty, spawnDragEnd.ty);
      const sw = Math.abs(spawnDragEnd.tx - spawnDragStart.tx) + 1;
      const sh = Math.abs(spawnDragEnd.ty - spawnDragStart.ty) + 1;
      const isPlayer = selectedSpawnType === 'player';
      ctx.fillStyle = isPlayer ? 'rgba(0, 128, 255, 0.3)' : 'rgba(255, 64, 64, 0.3)';
      ctx.fillRect(sx * tileSize, sy * tileSize, sw * tileSize, sh * tileSize);
      ctx.strokeStyle = isPlayer ? '#0080ff' : '#ff4040';
      ctx.lineWidth = 2;
      ctx.setLineDash([4, 2]);
      ctx.strokeRect(sx * tileSize, sy * tileSize, sw * tileSize, sh * tileSize);
      ctx.setLineDash([]);
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

    // Draw selection rectangle
    if (selectionRect) {
      ctx.strokeStyle = '#00ffff';
      ctx.lineWidth = 2;
      ctx.setLineDash([6, 3]);
      ctx.strokeRect(selectionRect.x * tileSize, selectionRect.y * tileSize, selectionRect.width * tileSize, selectionRect.height * tileSize);
      ctx.setLineDash([]);
      ctx.fillStyle = 'rgba(0, 255, 255, 0.1)';
      ctx.fillRect(selectionRect.x * tileSize, selectionRect.y * tileSize, selectionRect.width * tileSize, selectionRect.height * tileSize);
    }

    // Draw select drag preview
    if (selectDragStart && selectDragEnd) {
      const sx = Math.min(selectDragStart.tx, selectDragEnd.tx);
      const sy = Math.min(selectDragStart.ty, selectDragEnd.ty);
      const sw = Math.abs(selectDragEnd.tx - selectDragStart.tx) + 1;
      const sh = Math.abs(selectDragEnd.ty - selectDragStart.ty) + 1;
      ctx.strokeStyle = '#00ffff';
      ctx.lineWidth = 2;
      ctx.setLineDash([4, 2]);
      ctx.strokeRect(sx * tileSize, sy * tileSize, sw * tileSize, sh * tileSize);
      ctx.setLineDash([]);
      ctx.fillStyle = 'rgba(0, 255, 255, 0.15)';
      ctx.fillRect(sx * tileSize, sy * tileSize, sw * tileSize, sh * tileSize);
    }

    // Draw paste preview
    if (activeTool === 'paste' && clipboard && pastePreview) {
      ctx.fillStyle = 'rgba(0, 255, 128, 0.2)';
      ctx.strokeStyle = '#00ff80';
      ctx.lineWidth = 2;
      ctx.setLineDash([4, 2]);
      const pw = Math.min(clipboard.width, tiledMap.width - pastePreview.tx);
      const ph = Math.min(clipboard.height, tiledMap.height - pastePreview.ty);
      ctx.fillRect(pastePreview.tx * tileSize, pastePreview.ty * tileSize, pw * tileSize, ph * tileSize);
      ctx.strokeRect(pastePreview.tx * tileSize, pastePreview.ty * tileSize, pw * tileSize, ph * tileSize);
      ctx.setLineDash([]);
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

  function tileFromPixel(px, py) {
    const tileSize = getTileSize();
    return {
      tx: Math.floor(px / tileSize),
      ty: Math.floor(py / tileSize),
    };
  }

  function handleCanvasMouseDown(e) {
    if (!tiledMap) return;

    const { px, py } = canvasPixel(e);

    if (activeTool === 'terrain' || activeTool === 'lighting' || activeTool === 'elevation') {
      pushUndo();
      isPainting = true;
      paintTile(px, py);
    } else if (activeTool === 'wall') {
      pushUndo();
      placeWall(px, py);
    } else if (activeTool === 'eraseWall') {
      pushUndo();
      eraseWall(px, py);
    } else if (activeTool === 'spawn') {
      const { tx, ty } = tileFromPixel(px, py);
      if (tileInBounds(tx, ty)) {
        spawnDragStart = { tx, ty };
        spawnDragEnd = { tx, ty };
      }
    } else if (activeTool === 'eraseSpawn') {
      pushUndo();
      eraseSpawnZone(px, py);
    } else if (activeTool === 'select') {
      const { tx, ty } = tileFromPixel(px, py);
      if (tileInBounds(tx, ty)) {
        selectDragStart = { tx, ty };
        selectDragEnd = { tx, ty };
        selectionRect = null;
      }
    } else if (activeTool === 'paste') {
      const { tx, ty } = tileFromPixel(px, py);
      performPaste(tx, ty);
    }
  }

  function handleCanvasMouseMove(e) {
    if (!tiledMap) return;

    const { px, py } = canvasPixel(e);

    if (isPainting && (activeTool === 'terrain' || activeTool === 'lighting' || activeTool === 'elevation')) {
      paintTile(px, py);
    } else if (activeTool === 'spawn' && spawnDragStart) {
      const { tx, ty } = tileFromPixel(px, py);
      spawnDragEnd = clampTile(tx, ty);
      drawMap();
    } else if (activeTool === 'select' && selectDragStart) {
      const { tx, ty } = tileFromPixel(px, py);
      selectDragEnd = clampTile(tx, ty);
      drawMap();
    } else if (activeTool === 'paste' && clipboard) {
      const { tx, ty } = tileFromPixel(px, py);
      pastePreview = { tx, ty };
      drawMap();
    }
  }

  function handleCanvasMouseUp() {
    if (activeTool === 'spawn' && spawnDragStart && spawnDragEnd) {
      pushUndo();
      const { x, y, width, height } = rectFromDrag(spawnDragStart, spawnDragEnd);
      tiledMap = addSpawnZone(tiledMap, x, y, width, height, selectedSpawnType);
      dirty = true;
      spawnDragStart = null;
      spawnDragEnd = null;
      drawMap();
    } else if (activeTool === 'select' && selectDragStart && selectDragEnd) {
      selectionRect = rectFromDrag(selectDragStart, selectDragEnd);
      selectDragStart = null;
      selectDragEnd = null;
      drawMap();
    }
    isPainting = false;
  }

  function tileInBounds(tx, ty) {
    return tx >= 0 && tx < tiledMap.width && ty >= 0 && ty < tiledMap.height;
  }

  function clampTile(tx, ty) {
    return {
      tx: Math.max(0, Math.min(tiledMap.width - 1, tx)),
      ty: Math.max(0, Math.min(tiledMap.height - 1, ty)),
    };
  }

  function rectFromDrag(start, end) {
    const x = Math.min(start.tx, end.tx);
    const y = Math.min(start.ty, end.ty);
    const width = Math.abs(end.tx - start.tx) + 1;
    const height = Math.abs(end.ty - start.ty) + 1;
    return { x, y, width, height };
  }

  function paintTile(px, py) {
    const { tx, ty } = tileFromPixel(px, py);
    if (!tileInBounds(tx, ty)) return;

    if (activeTool === 'terrain') {
      const gid = TERRAIN_TYPES[selectedTerrain]?.gid || 1;
      tiledMap = setTerrain(tiledMap, tx, ty, gid);
    } else if (activeTool === 'lighting') {
      const gid = LIGHTING_TYPES[selectedLighting]?.gid ?? 0;
      tiledMap = setLighting(tiledMap, tx, ty, gid);
    } else if (activeTool === 'elevation') {
      tiledMap = setElevation(tiledMap, tx, ty, selectedElevation);
    }
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

  function eraseSpawnZone(px, py) {
    const zones = getSpawnZones(tiledMap);
    for (const zone of zones) {
      if (px >= zone.x && px <= zone.x + zone.width && py >= zone.y && py <= zone.y + zone.height) {
        tiledMap = removeSpawnZone(tiledMap, zone.id);
        dirty = true;
        drawMap();
        return;
      }
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

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
      <div class="form-row">
        <button class="primary-btn" onclick={createNewMap}>Create Map</button>
        <!-- F-7: import a Tiled project (.tmj + its tileset/image-layer images)
             instead of authoring blank. Select the .tmj together with every
             image it references in one go. -->
        <input
          type="file"
          multiple
          accept=".tmj,.json,application/json,image/png,image/jpeg,image/webp"
          style="display:none"
          bind:this={tmjFileInputEl}
          onchange={handleTmjImport}
        />
        <button
          class="import-tmj-btn"
          onclick={() => tmjFileInputEl?.click()}
          disabled={importingTmj}
          title="Import a Tiled project: select the .tmj plus every tileset/image-layer image it references"
        >{importingTmj ? 'Importing...' : 'Import Tiled (.tmj + images)'}</button>
      </div>
    </div>
  {:else}
    <!-- Top bar: document actions (row 1) + mode selection (row 2) -->
    <div class="topbar">
      <div class="topbar-row doc-actions">
        <div class="toolbar-section">
          <label>
            Name:
            <input type="text" bind:value={mapName} oninput={() => dirty = true} />
          </label>
        </div>

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

        <div class="toolbar-section doc-actions-right">
          <button class="undo-btn" onclick={performUndo} disabled={!undoStack.canUndo()} title="Undo (Ctrl+Z)">Undo</button>
          <button class="redo-btn" onclick={performRedo} disabled={!undoStack.canRedo()} title="Redo (Ctrl+Shift+Z)">Redo</button>
          {#if clipboard}
            <button onclick={startPaste} title="Paste (Ctrl+V)">Paste</button>
          {/if}
          <button class="duplicate-btn" onclick={performDuplicate} title="Duplicate Map">Duplicate Map</button>
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

      <div class="topbar-row mode-bar">
        <span class="section-label">Mode:</span>
        <button
          class:active={activeTool === 'terrain'}
          onclick={() => activeTool = 'terrain'}
        >Terrain</button>
        <button
          class:active={activeTool === 'lighting'}
          onclick={() => activeTool = 'lighting'}
        >Lighting</button>
        <button
          class:active={activeTool === 'elevation'}
          onclick={() => activeTool = 'elevation'}
        >Elevation</button>
        <button
          class:active={activeTool === 'wall'}
          onclick={() => activeTool = 'wall'}
        >Wall</button>
        <button
          class:active={activeTool === 'eraseWall'}
          onclick={() => activeTool = 'eraseWall'}
        >Erase Wall</button>
        <button
          class:active={activeTool === 'spawn'}
          onclick={() => activeTool = 'spawn'}
        >Spawn Zone</button>
        <button
          class:active={activeTool === 'eraseSpawn'}
          onclick={() => activeTool = 'eraseSpawn'}
        >Erase Spawn</button>
        <button
          class:active={activeTool === 'select'}
          onclick={() => activeTool = 'select'}
        >Select</button>
      </div>
    </div>

    {#if error}
      <p class="error">{error}</p>
    {/if}

    {#if skippedFeatures && skippedFeatures.length > 0}
      <!-- F-7: surface Tiled features the importer stripped. -->
      <div class="skipped-features">
        <strong>Tiled import: stripped {skippedFeatures.length} unsupported feature{skippedFeatures.length === 1 ? '' : 's'}:</strong>
        <ul>
          {#each skippedFeatures as feat}
            <li>{feat.feature || 'feature'}{feat.detail ? ` — ${feat.detail}` : ''}</li>
          {/each}
        </ul>
      </div>
    {/if}

    <!-- Workspace: canvas on the left, active-mode tool palette on the right -->
    <div class="workspace">
      <div class="canvas-container">
        <canvas
          bind:this={canvasEl}
          onmousedown={handleCanvasMouseDown}
          onmousemove={handleCanvasMouseMove}
          onmouseup={handleCanvasMouseUp}
          onmouseleave={handleCanvasMouseUp}
        ></canvas>
      </div>

      <aside class="tool-panel">
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

        {#if activeTool === 'lighting'}
          <div class="toolbar-section lighting-palette">
            <span class="section-label">Lighting:</span>
            {#each Object.entries(LIGHTING_TYPES).filter(([k]) => k !== 'normal') as [key, lt]}
              <button
                class="lighting-btn"
                class:active={selectedLighting === key}
                onclick={() => selectedLighting = key}
                style="background: {lt.color}"
                title={lt.label}
              >{lt.label}</button>
            {/each}
            <button
              class="lighting-btn"
              class:active={selectedLighting === 'normal'}
              onclick={() => selectedLighting = 'normal'}
              title="Erase lighting"
            >Clear</button>
          </div>
        {/if}

        {#if activeTool === 'elevation'}
          <div class="toolbar-section">
            <span class="section-label">Level:</span>
            <input
              type="number"
              class="elevation-input"
              min="0"
              max={ELEVATION_MAX}
              bind:value={selectedElevation}
            />
            <input
              type="range"
              min="0"
              max={ELEVATION_MAX}
              bind:value={selectedElevation}
              class="elevation-slider"
            />
            <span class="elevation-label">{selectedElevation}</span>
          </div>
        {/if}

        {#if activeTool === 'spawn'}
          <div class="toolbar-section">
            <span class="section-label">Type:</span>
            <button
              class="spawn-btn player"
              class:active={selectedSpawnType === 'player'}
              onclick={() => selectedSpawnType = 'player'}
            >Player</button>
            <button
              class="spawn-btn enemy"
              class:active={selectedSpawnType === 'enemy'}
              onclick={() => selectedSpawnType = 'enemy'}
            >Enemy</button>
            <span class="section-label hint">Click & drag to draw zone</span>
          </div>
        {/if}

        {#if activeTool === 'select' && selectionRect}
          <div class="toolbar-section">
            <span class="section-label">Selection:</span>
            <button onclick={copySelection}>Copy (Ctrl+C)</button>
          </div>
        {/if}

        {#if activeTool === 'paste'}
          <div class="toolbar-section">
            <span class="section-label hint">Click to place pasted region</span>
          </div>
        {/if}

        {#if activeTool === 'wall' || activeTool === 'eraseWall' || activeTool === 'eraseSpawn' || (activeTool === 'select' && !selectionRect)}
          <div class="toolbar-section">
            <span class="section-label hint">Click & drag on the map.</span>
          </div>
        {/if}
      </aside>
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

  .topbar {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    padding: 0.75rem;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
    margin-bottom: 0.5rem;
  }

  .topbar-row {
    display: flex;
    flex-wrap: wrap;
    gap: 1rem;
    align-items: center;
  }

  /* Mode selector sits below the document actions, set off by a divider. */
  .mode-bar {
    gap: 0.5rem;
    padding-top: 0.5rem;
    border-top: 1px solid #0f3460;
  }

  /* Push the undo/redo/save cluster to the right edge of the top row. */
  .doc-actions-right {
    margin-left: auto;
  }

  .workspace {
    display: flex;
    gap: 0.5rem;
    align-items: flex-start;
  }

  /* Right-hand panel holds the options for the active mode. */
  .tool-panel {
    flex: 0 0 240px;
    max-width: 240px;
    align-self: stretch;
    max-height: 70vh;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    padding: 0.75rem;
    background: #16213e;
    border: 1px solid #0f3460;
    border-radius: 8px;
  }

  .tool-panel .toolbar-section {
    flex-wrap: wrap;
    align-items: flex-start;
    gap: 0.4rem;
  }

  /* Section label sits on its own line above its controls inside the panel. */
  .tool-panel .section-label {
    flex-basis: 100%;
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

  .hint {
    font-style: italic;
    font-size: 0.75rem;
  }

  .topbar button,
  .tool-panel button {
    padding: 0.4rem 0.8rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 4px;
    cursor: pointer;
  }

  .topbar button:hover,
  .tool-panel button:hover {
    background: #0f3460;
  }

  .topbar button.active,
  .tool-panel button.active {
    background: #e94560;
    border-color: #e94560;
    color: white;
  }

  .terrain-btn, .lighting-btn {
    font-size: 0.8rem;
    min-width: 80px;
    text-align: center;
    color: white !important;
    text-shadow: 1px 1px 2px rgba(0,0,0,0.8);
  }

  .spawn-btn.player {
    border-color: #0080ff !important;
  }

  .spawn-btn.player.active {
    background: #0080ff !important;
    border-color: #0080ff !important;
  }

  .spawn-btn.enemy {
    border-color: #ff4040 !important;
  }

  .spawn-btn.enemy.active {
    background: #ff4040 !important;
    border-color: #ff4040 !important;
  }

  .elevation-input {
    width: 50px !important;
  }

  .elevation-slider {
    width: 80px !important;
  }

  .elevation-label {
    font-size: 0.85rem;
    color: #ccc;
    min-width: 1.5em;
  }

  .topbar input,
  .tool-panel input {
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
    flex: 1;
    min-width: 0;
    overflow: auto;
    max-width: 100%;
    max-height: 70vh;
    border: 1px solid #0f3460;
    border-radius: 4px;
  }

  /* Stack the tool panel under the canvas on narrow screens. */
  @media (max-width: 700px) {
    .workspace {
      flex-direction: column;
    }

    .tool-panel {
      flex-basis: auto;
      max-width: none;
      align-self: stretch;
    }
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

  /* F-7: Tiled .tmj import affordance. */
  .import-tmj-btn {
    padding: 0.75rem 1.25rem;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #0f3460;
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.95rem;
  }

  .import-tmj-btn:hover:not(:disabled) {
    background: #0f3460;
  }

  .import-tmj-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .skipped-features {
    margin: 0.5rem 0;
    padding: 0.6rem 0.9rem;
    background: #2a2410;
    border-left: 3px solid #ffc107;
    color: #ffc107;
    font-size: 0.85rem;
    border-radius: 4px;
  }

  .skipped-features ul {
    margin: 0.25rem 0 0 1rem;
    padding: 0;
  }

  .undo-btn, .redo-btn {
    background: #6c757d !important;
    border-color: #6c757d !important;
  }

  .undo-btn:disabled, .redo-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed !important;
  }

  .duplicate-btn {
    background: #6f42c1 !important;
    border-color: #6f42c1 !important;
    color: white !important;
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
