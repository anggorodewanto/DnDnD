// gen_items_catalog generates portal/svelte/src/lib/items-catalog.json from the
// canonical Go item catalog (internal/refdata.ItemCatalog), so the Svelte
// builder and the Go backend share ONE source of truth for item id -> name /
// category / default quantity. The Go catalog is the SSOT (the refdata seeder
// consumes it); the Svelte equip pickers classify weapon/armor ids from the
// generated JSON instead of a hand-maintained parallel set (see docs/live-play
// ISSUE-017 phase 4 — the drift between hand-maintained copies caused the
// crossbow-ammo and background-slug bugs).
//
// Run via `go generate ./internal/portal/...` or `make items-catalog-check`
// (which also fails CI when the committed JSON drifts from the Go catalog).
//
// Flags:
//
//	-out  generated JSON file to write (default ../../portal/svelte/src/lib/items-catalog.json)
//
// The default assumes the working directory is internal/portal (where the
// //go:generate directive lives). Exit codes: 0 ok, non-zero on IO/marshal error.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ab/dndnd/internal/refdata"
)

func run(outPath string) error {
	data, err := json.MarshalIndent(refdata.ItemCatalog(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal item catalog: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

func main() {
	out := flag.String("out", "../../portal/svelte/src/lib/items-catalog.json", "generated JSON file to write")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "gen_items_catalog:", err)
		os.Exit(1)
	}
}
