package portal

// Background skill + equipment data is generated from the canonical
// portal/svelte/src/lib/backgrounds.json (shared with the Svelte builder) into
// backgrounds_gen.go. Edit the JSON, then regenerate; `make backgrounds-check`
// fails CI if the committed generated file drifts from the JSON.
//
//go:generate go run github.com/ab/dndnd/scripts/gen_backgrounds -in ../../portal/svelte/src/lib/backgrounds.json -out backgrounds_gen.go

// The canonical item catalog (internal/refdata.ItemCatalog) is the Go-side SSOT
// for every equipment id's name / category / default quantity. It is generated
// out to portal/svelte/src/lib/items-catalog.json so the Svelte equip pickers
// classify weapon/armor ids from one source instead of a hand-maintained set;
// `make items-catalog-check` fails CI if the committed JSON drifts.
//
//go:generate go run github.com/ab/dndnd/scripts/gen_items_catalog -out ../../portal/svelte/src/lib/items-catalog.json
