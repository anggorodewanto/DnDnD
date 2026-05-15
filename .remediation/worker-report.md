# Worker Report: B-H03 — MIME Type Allowlist

## Finding
Asset upload accepted arbitrary MIME types (e.g., `text/html`), enabling stored XSS via `ServeAsset`.

## Fix Applied

**File:** `internal/asset/service.go`

Added `allowedMIMETypes` map in `validateUpload`:
- `map_background`, `token` → `image/png`, `image/jpeg`, `image/webp`
- `tileset` → `application/json`

Uploads with disallowed MIME types are rejected with a 400 error: `"mime type not allowed for asset type <type>"`.

## Tests Added

**File:** `internal/asset/handler_test.go`

| Test | Asserts |
|------|---------|
| `TestHandler_UploadAsset_DisallowedMimeType` | `text/html` upload → 400, body contains "mime type not allowed" |
| `TestHandler_UploadAsset_AllowedMimeType_ImagePNG` | `image/png` for `map_background` → 201 |
| `TestHandler_UploadAsset_AllowedMimeType_JSONForTileset` | `application/json` for `tileset` → 201 |

Existing tests updated to use explicit valid MIME types via `CreatePart` instead of `CreateFormFile` (which defaults to `application/octet-stream`).

## Verification

- `make test` — PASS
- `make cover-check` — PASS (all thresholds met)

## TDD Cycle

1. **Red:** Wrote `TestHandler_UploadAsset_DisallowedMimeType` — confirmed 201 (fail).
2. **Green:** Added `allowedMIMETypes` check in `validateUpload` — all tests pass.
3. **Refactor:** Updated existing tests to use explicit valid MIME types for correctness.
