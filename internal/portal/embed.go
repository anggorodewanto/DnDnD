package portal

import "embed"

// Assets holds the embedded portal static files (compiled Svelte SPA).
//
//go:embed assets/*
var Assets embed.FS
