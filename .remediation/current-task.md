finding_id: B-H02
severity: High
title: RenderMap mutates caller-supplied MapData.TileSize
location: internal/gamemap/renderer/renderer.go:13-16
spec_ref: spec §Map Size Limits; Phase 22
problem: |
  RenderMap overwrites md.TileSize in place. The same *MapData pointer is stored in RenderQueue.latest and visible to future enqueues.
suggested_fix: |
  Use a local variable: tileSize := md.TileSize; if md.Width > 100 || md.Height > 100 { tileSize = 32 }. Pass tileSize everywhere instead of mutating.
acceptance_criterion: |
  After RenderMap returns, the original MapData.TileSize is unchanged. A test demonstrates this.
