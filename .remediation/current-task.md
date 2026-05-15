finding_id: B-H03
severity: High
title: Asset upload accepts arbitrary MIME types (XSS / file-type abuse risk)
location: internal/asset/handler.go:36-83, internal/asset/service.go:121-135
spec_ref: phases §Phase 20, spec §Asset Storage
problem: |
  UploadAsset trusts the multipart Content-Type header verbatim. A DM can upload HTML/JS/SVG as a "map_background" — ServeAsset then sets Content-Type: text/html enabling stored XSS.
suggested_fix: |
  Maintain an allowlist per AssetType (map_background/token → image/png|image/jpeg|image/webp, tileset → application/json). Reject everything else.
acceptance_criterion: |
  Upload with Content-Type: text/html is rejected. Upload with image/png is accepted. A test demonstrates both.
