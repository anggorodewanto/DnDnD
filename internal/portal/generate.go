package portal

// Background skill + equipment data is generated from the canonical
// portal/svelte/src/lib/backgrounds.json (shared with the Svelte builder) into
// backgrounds_gen.go. Edit the JSON, then regenerate; `make backgrounds-check`
// fails CI if the committed generated file drifts from the JSON.
//
//go:generate go run github.com/ab/dndnd/scripts/gen_backgrounds -in ../../portal/svelte/src/lib/backgrounds.json -out backgrounds_gen.go
