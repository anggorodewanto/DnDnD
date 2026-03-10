package dashboard

import "embed"

// Assets holds the embedded dashboard static files.
// In production, this will contain the compiled Svelte SPA.
//
//go:embed assets/*
var Assets embed.FS
