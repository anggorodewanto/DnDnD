finding_id: B-H01
severity: High
title: Map size limits not enforced when rendering, only at create-time
location: internal/gamemap/renderer/renderer.go:12-16
spec_ref: spec §Map Size Limits ("rejected: >200 in either dimension")
problem: |
  RenderMap only checks >100 to downscale tile size but accepts arbitrarily large Width/Height. A stale stored map exceeding 200x200 will OOM the renderer.
suggested_fix: |
  Add if md.Width > HardLimitDimension || md.Height > HardLimitDimension early-return error in RenderMap.
acceptance_criterion: |
  RenderMap returns an error for maps exceeding 200 in either dimension. A test demonstrates this.
