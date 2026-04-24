package dashboard

import (
	"bytes"
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ab/dndnd/internal/errorlog"
)

// ErrorsHandler serves the /dashboard/errors panel and, via SetErrorReader
// on the main dashboard Handler, feeds the sidebar badge its 24h count.
// Phase 112 spec lines 2967-2971.
type ErrorsHandler struct {
	logger *slog.Logger
	reader errorlog.Reader
	clock  func() time.Time
	tmpl   *template.Template
}

// NewErrorsHandler constructs an ErrorsHandler. Pass time.Now for clock
// unless a test needs determinism.
func NewErrorsHandler(logger *slog.Logger, reader errorlog.Reader, clock func() time.Time) *ErrorsHandler {
	if logger == nil {
		logger = slog.Default()
	}
	if clock == nil {
		clock = time.Now
	}
	return &ErrorsHandler{
		logger: logger,
		reader: reader,
		clock:  clock,
		tmpl:   template.Must(template.New("errors").Parse(errorsTemplate)),
	}
}

// errorsPageData drives the errors-panel template.
type errorsPageData struct {
	Nav     []NavEntry
	Count   int
	Entries []errorEntryView
}

type errorEntryView struct {
	Timestamp string
	Command   string
	UserID    string
	Summary   string
}

// ServePage renders GET /dashboard/errors.
func (h *ErrorsHandler) ServePage(w http.ResponseWriter, r *http.Request) {
	if !hasAuthUser(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	now := h.clock()
	count, err := h.reader.CountSince(ctx, now.Add(-24*time.Hour))
	if err != nil {
		h.logger.Error("errorlog count-since failed", "error", err)
		count = 0
	}

	raw, err := h.reader.ListRecent(ctx, 100)
	if err != nil {
		h.logger.Error("errorlog list-recent failed", "error", err)
		raw = nil
	}

	entries := make([]errorEntryView, 0, len(raw))
	for _, e := range raw {
		entries = append(entries, errorEntryView{
			Timestamp: e.CreatedAt.UTC().Format(time.RFC3339),
			Command:   e.Command,
			UserID:    e.UserID,
			Summary:   e.Summary,
		})
	}

	data := errorsPageData{
		Nav:     navWithErrorBadge(count),
		Count:   count,
		Entries: entries,
	}

	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, data); err != nil {
		h.logger.Error("errors template render failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// RegisterErrorsRoutes mounts GET /dashboard/errors behind authMiddleware.
func RegisterErrorsRoutes(r chi.Router, h *ErrorsHandler, authMiddleware func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/dashboard/errors", h.ServePage)
	})
}

// MountErrorsRoutes is the one-call convenience wiring for main.go: it
// builds an ErrorsHandler, mounts /dashboard/errors behind authMiddleware,
// and injects the same reader/clock into dash so the Campaign Home sidebar
// shows the 24h badge consistent with the panel.
func MountErrorsRoutes(r chi.Router, dash *Handler, reader errorlog.Reader, clock func() time.Time, authMiddleware func(http.Handler) http.Handler) *ErrorsHandler {
	if clock == nil {
		clock = time.Now
	}
	handler := NewErrorsHandler(nil, reader, clock)
	RegisterErrorsRoutes(r, handler, authMiddleware)
	if dash != nil {
		dash.SetErrorReader(reader, clock)
	}
	return handler
}

// navWithErrorBadge returns a copy of SidebarNav with "(N)" appended to the
// Errors entry's label so every page shows the same 24h count.
func navWithErrorBadge(count int) []NavEntry {
	out := make([]NavEntry, len(SidebarNav))
	copy(out, SidebarNav)
	for i := range out {
		if out[i].Path != "/dashboard/errors" {
			continue
		}
		if count > 0 {
			out[i].Label = out[i].Label + " (" + formatErrorCount(count) + ")"
		}
		break
	}
	return out
}

// formatErrorCount renders small non-negative ints without importing
// strconv here (keeps the helper self-contained and cheap).
func formatErrorCount(n int) string {
	if n <= 0 {
		return "0"
	}
	// Small ints only; >999 truncates to 999+ for the sidebar badge.
	if n > 999 {
		return "999+"
	}
	// Inline base-10 formatting (ASCII digits).
	var buf [4]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// errorLogAdapter bridges the dashboard Handler's badge rendering to the
// errorlog.Reader + clock so the main Campaign Home template can show the
// same 24h count that appears in the Errors panel.
type errorLogAdapter struct {
	reader errorlog.Reader
	clock  func() time.Time
}

func (a errorLogAdapter) count(ctx context.Context) int {
	if a.reader == nil {
		return 0
	}
	c, err := a.reader.CountSince(ctx, a.clock().Add(-24*time.Hour))
	if err != nil {
		return 0
	}
	return c
}

const errorsTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>DnDnD — Errors</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: system-ui, -apple-system, sans-serif; display: flex; min-height: 100vh; background: #1a1a2e; color: #e0e0e0; }
        .sidebar { width: 250px; background: #16213e; padding: 1rem 0; border-right: 1px solid #0f3460; }
        .sidebar h2 { padding: 0 1rem 1rem; color: #e94560; font-size: 1.2rem; }
        .sidebar nav a { display: flex; align-items: center; gap: 0.75rem; padding: 0.75rem 1rem; color: #e0e0e0; text-decoration: none; }
        .sidebar nav a:hover { background: #0f3460; }
        .main { flex: 1; padding: 2rem; }
        .main h1 { color: #e94560; margin-bottom: 1rem; }
        table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
        th, td { padding: 0.5rem 0.75rem; text-align: left; border-bottom: 1px solid #0f3460; }
        th { color: #e94560; }
        .empty { color: #888; padding: 2rem; text-align: center; }
    </style>
</head>
<body>
    <div class="sidebar">
        <h2>DnDnD Dashboard</h2>
        <nav>
            {{range .Nav}}<a href="{{.Path}}"><span>{{.Icon}}</span><span>{{.Label}}</span></a>
            {{end}}
        </nav>
    </div>
    <div class="main">
        <h1>Recent Errors</h1>
        <p>Errors recorded in the last 24 hours: <strong>{{.Count}}</strong></p>
        {{if .Entries}}
        <table>
            <thead><tr><th>Timestamp</th><th>Command</th><th>Player</th><th>Summary</th></tr></thead>
            <tbody>
                {{range .Entries}}<tr>
                    <td>{{.Timestamp}}</td>
                    <td>{{if .Command}}/{{.Command}}{{else}}—{{end}}</td>
                    <td>{{if .UserID}}@{{.UserID}}{{else}}—{{end}}</td>
                    <td>{{.Summary}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
        {{else}}
        <div class="empty">No errors recorded in the last 24 hours. 🎉</div>
        {{end}}
    </div>
</body>
</html>`
