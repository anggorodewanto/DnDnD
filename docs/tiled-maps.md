# Building Battle Maps with Tiled

DnDnD can render full battle maps built in the free [Tiled map editor](https://www.mapeditor.org/)
using real game tilesets, so your encounters show actual tile art (stone floors,
walls, water, lava, foliage) instead of a blank grid. This guide walks a DM from
a blank Tiled project to a map that renders with real sprites in both the web map
preview and the Discord `#combat-map` posts.

You build the *art* in Tiled with ordinary tilesets, then add a few specially
**named layers** that DnDnD reads to drive game mechanics (terrain costs,
line-of-sight walls, lighting, elevation, and spawn zones). The art and the
mechanics live in the same `.tmj` file.

## Prerequisites

- [Tiled](https://www.mapeditor.org/) (any recent version).
- One or more **tileset images** (PNG, etc.) — orthogonal, grid-based tilesets.
- A DnDnD campaign and access to the web dashboard's map editor.

## Step-by-step

### 1. Create an orthogonal, finite map

In Tiled, choose **File > New > New Map** and set:

- **Orientation:** `Orthogonal`. This is the only supported orientation —
  isometric, staggered, and hexagonal maps are rejected on import.
- **Map size:** `Fixed` (not *Infinite*). Infinite maps are rejected on import.
- **Map width / height:** between **1 and 200** tiles on each side.
  - Maps **up to 100** tiles on a side render at **48 px** per tile (standard).
  - Maps **101–200** tiles on a side are automatically **downscaled to 32 px**
    per tile so they stay readable when posted to Discord.
  - Anything **0 or below**, or **above 200**, is rejected.
- **Tile size:** match it to your tileset's native tile size (e.g. 32×32).
  DnDnD reads your map's `tilewidth` for spacing/scaling; pick a square tile.

### 2. Add your tilesets and paint the art

Add tilesets via the **Tilesets** panel (**New Tileset...**) and paint your
floors, walls, props, and scenery across one or more tile layers. Use as many
tilesets and decorative tile layers as you like — DnDnD renders them all.

**Image layers** are also supported: add an **Image Layer** to drop a single
pre-made image (e.g. a painted backdrop) onto the map. Its image file must be
included in the import selection (Step 6), just like a tileset image.

### 3. EMBED every tileset (critical)

> **This is the step people miss.** DnDnD v1 rejects maps that reference
> **external** `.tsx` tileset files. Every tileset must be **embedded** in the
> map itself.

To embed a tileset:

1. Select the tileset in the **Tilesets** panel.
2. Click the **Embed Tileset** button on the tileset toolbar.

Alternatively, set **Map > Map Properties** (or the New Map dialog) to embed
tilesets so new tilesets are embedded automatically. When a tileset is embedded,
its definition is written inline in the `.tmj` instead of pointing at a separate
`.tsx` file.

The tileset **image** files are still separate PNGs — that's expected. You will
select those images alongside the `.tmj` during import (Step 5). Embedding only
removes the external `.tsx` *definition* reference, not the image.

### 4. Add the game-mechanic layers (optional but recommended)

On top of the art, add layers with these exact **names** and **types** to give
the map D&D mechanics. See the [reference table](#game-mechanic-layers) below
for the specifics. Layers with any other name are treated as pure decoration and
rendered as-is.

### 5. Export as `.tmj`

Save or export the map in the **Tiled JSON** format:

- **File > Save As...** and choose `.tmj`, **or**
- **File > Export As...** and choose `.tmj` / `.json`.

Keep all **image filenames intact** — do not rename the tileset images (or any
image-layer images) after export. The importer matches images to the map by
**filename (basename)**, so the names in the `.tmj` must match the files you
upload.

### 6. Import into DnDnD

In the DnDnD web map editor, click **Import Tiled Project** and, **in one
selection**, choose:

- the `.tmj` map file, **and**
- **every image file it references** — all tileset images (and any image-layer
  images).

On import, DnDnD validates and sanitizes the map, uploads the images, links them
by filename, and persists the map. From then on the map renders with real tile
sprites in the web preview and in `#combat-map` posts. The import result also
reports any features that were [stripped](#what-gets-stripped--rejected).

## Game-mechanic layers

These layers are recognized **by name and by layer type**. Names are
case-sensitive and must match exactly. Tile-layer mechanics are keyed off each
tile's **Type** field (set in the tileset via **Tile > Custom Properties / Type**,
or the tile `type` in Tiled's tile editor). Object-layer mechanics use
**rectangle** objects.

| Layer name    | Layer type     | What it does                                              | How to author it                                                                                                              |
| ------------- | -------------- | --------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `terrain`     | Tile layer     | Assigns terrain type per tile (movement cost, hazards).   | Paint tiles whose tile **Type** is one of: `open_ground`, `difficult_terrain`, `water`, `lava`, `pit`. Untyped = open ground. |
| `walls`       | Object layer   | Line-of-sight / movement blockers.                        | Draw **rectangle** objects along walls. Each rectangle becomes a wall segment (in tile units).                                |
| `lighting`    | Tile layer     | Lighting / obscurement overlay.                           | Paint tiles whose **Type** is one of: `dim_light`, `darkness`, `magical_darkness`, `fog`, `light_obscurement`.                |
| `elevation`   | Tile layer     | Per-tile elevation, in **feet**.                          | The tile's raw value is read as feet, row-major across the grid.                                                              |
| `spawn_zones` | Object layer   | Where players / enemies start.                            | Draw **rectangle** objects and set each object's **Type** to `player` or `enemy`.                                             |

Notes:

- A `terrain` tile with no recognized `type` (or no `terrain` layer at all)
  defaults to **open ground**.
- For `walls` and `spawn_zones`, object coordinates are converted from pixels to
  tile units using the map's tile size, so align objects to the grid.
- These layers are *additive*: you can ship a map with art only and add
  mechanics later, or vice-versa.

## What gets stripped / rejected

### Hard-rejected (import fails)

If your `.tmj` contains any of these, the import is refused with an error and
nothing is saved — fix the map and re-export:

- **Infinite maps** — use a fixed/finite map size.
- **Non-orthogonal orientation** — only `orthogonal` is supported.
- **Dimensions < 1 or > 200** on either side.
- **External (non-embedded) tilesets** — embed every tileset (see Step 3).

### Stripped on import (import succeeds, feature removed)

These are silently removed during sanitization; the import result lists each
class that was stripped so you know what changed:

- **Tile animations** — animated tiles render as their base tile.
- **Parallax scrolling** — parallax factors are removed; layers render flat.
- **Group layers** — flattened into the top-level layer list (their child
  layers are kept).
- **Text objects** — dropped from object layers.
- **Point objects** — dropped from object layers.
- **Wang sets** (terrain/corner brushes) — removed from tilesets; already-painted
  tiles are unaffected.

If you relied on any stripped feature for *art*, bake it into a normal tile layer
before exporting.

## Troubleshooting

**"Import says it's missing image X" / a tile renders blank.**
You didn't include every referenced image in the import selection, or a filename
changed. Re-run **Import Tiled Project** and select the `.tmj` *plus* all of its
tileset/image-layer images at once. Make sure the filenames still match the names
recorded in the `.tmj` — the importer matches by basename.

**"Tileset is external" / import rejects the tileset.**
One or more tilesets are still `.tsx` references. In Tiled, select each tileset
in the **Tilesets** panel and click **Embed Tileset**, then re-export. See Step 3.

**"Only orthogonal orientation is supported."**
Your map is isometric/staggered/hexagonal. Recreate it as an **Orthogonal** map.

**"Infinite maps are not supported."**
The map was created as *Infinite*. In **Map > Map Properties** switch it to a
fixed size (or recreate it), then re-export.

**"Map dimensions ..." errors.**
Width or height is below 1 or above 200. Resize the map to within 1–200 tiles per
side.

**My map looks lower-resolution / smaller tiles than expected.**
A map larger than 100 tiles on a side is auto-downscaled to 32 px tiles. Shrink
the map to 100 or fewer per side to keep the 48 px standard tile size.

**A terrain/lighting tile has no effect.**
The tile's **Type** isn't one of the recognized values, or it's on a layer whose
name/type doesn't match the table above. Check the layer name (exact,
case-sensitive) and the tile's `type`.

---

*This document is referenced from the project README.*
