// gen_invocations_catalog generates portal/svelte/src/lib/invocations-catalog.json
// from the canonical Go catalog (internal/refdata.PactBoonCatalog +
// InvocationCatalog), so the Svelte builder's Class Features picker and the Go
// backend validation/resolution share ONE source of truth for Warlock pact
// boons and Eldritch Invocations (ids, names, descriptions, prerequisites,
// granted spells). The Go catalog is the SSOT (invocation_catalog.go); the
// Svelte picker reads the generated JSON instead of a hand-maintained copy.
//
// Run via `go generate ./internal/portal/...` or `make invocations-catalog-check`
// (which also fails CI when the committed JSON drifts from the Go catalog).
//
// Flags:
//
//	-out  generated JSON file to write (default ../../portal/svelte/src/lib/invocations-catalog.json)
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

// catalog is the on-disk shape: one file holding both lists so the Svelte
// picker imports a single JSON.
type catalog struct {
	PactBoons   []refdata.PactBoon   `json:"pact_boons"`
	Invocations []refdata.Invocation `json:"invocations"`
}

func run(outPath string) error {
	data, err := json.MarshalIndent(catalog{
		PactBoons:   refdata.PactBoonCatalog(),
		Invocations: refdata.InvocationCatalog(),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal invocation catalog: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

func main() {
	out := flag.String("out", "../../portal/svelte/src/lib/invocations-catalog.json", "generated JSON file to write")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "gen_invocations_catalog:", err)
		os.Exit(1)
	}
}
