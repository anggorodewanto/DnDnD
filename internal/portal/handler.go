package portal

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ab/dndnd/internal/auth"
)

// TokenValidator validates portal tokens.
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (*PortalToken, error)
}

// Handler serves the player portal pages.
type Handler struct {
	logger      *slog.Logger
	validator   TokenValidator
	landingTmpl *template.Template
	createTmpl  *template.Template
	errorTmpl   *template.Template
}

// NewHandler creates a new portal Handler.
func NewHandler(logger *slog.Logger, validator TokenValidator) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	jsFile, cssFile := resolvePortalAssets()
	tmplStr := strings.ReplaceAll(createTemplate, "{{.JSFile}}", jsFile)
	tmplStr = strings.ReplaceAll(tmplStr, "{{.CSSFile}}", cssFile)
	return &Handler{
		logger:      logger,
		validator:   validator,
		landingTmpl: template.Must(template.New("landing").Parse(landingTemplate)),
		createTmpl:  template.Must(template.New("create").Parse(tmplStr)),
		errorTmpl:   template.Must(template.New("error").Parse(errorTemplate)),
	}
}

// resolvePortalAssets reads the embedded assets/ directory to find the actual
// hashed .js and .css filenames, avoiding stale hard-coded hashes.
func resolvePortalAssets() (jsFile, cssFile string) {
	jsFile = "index.js"
	cssFile = "index.css"
	entries, err := fs.ReadDir(Assets, "assets/assets")
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "index-") && strings.HasSuffix(name, ".js") {
			jsFile = name
		}
		if strings.HasPrefix(name, "index-") && strings.HasSuffix(name, ".css") {
			cssFile = name
		}
	}
	return
}

// SetLandingTemplate overrides the landing template (for testing).
func (h *Handler) SetLandingTemplate(t *template.Template) {
	h.landingTmpl = t
}

// SetCreateTemplate overrides the create template (for testing).
func (h *Handler) SetCreateTemplate(t *template.Template) {
	h.createTmpl = t
}

// SetErrorTemplate overrides the error template (for testing).
func (h *Handler) SetErrorTemplate(t *template.Template) {
	h.errorTmpl = t
}

// LandingData holds data for the landing page.
type LandingData struct {
	UserID string
}

// ServeLanding serves the portal landing page (requires auth).
func (h *Handler) ServeLanding(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	h.render(w, h.landingTmpl, LandingData{UserID: userID})
}

// CreateData holds data for the character builder page.
type CreateData struct {
	Token      string
	CampaignID string
	UserID     string
}

// ServeCreate validates the token and serves the character builder shell.
func (h *Handler) ServeCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.DiscordUserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		h.renderError(w, http.StatusBadRequest, "missing token", "No token was provided. Please use the link from Discord.")
		return
	}

	tok, err := h.validator.ValidateToken(r.Context(), tokenStr)
	if err != nil {
		h.handleTokenError(w, err)
		return
	}

	if tok.DiscordUserID != userID {
		h.renderError(w, http.StatusForbidden, "forbidden", "This link belongs to a different user.")
		return
	}

	h.render(w, h.createTmpl, CreateData{
		Token:      tok.Token,
		CampaignID: tok.CampaignID.String(),
		UserID:     tok.DiscordUserID,
	})
}

func (h *Handler) handleTokenError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrTokenExpired) {
		h.renderError(w, http.StatusGone, "Link expired", "This link has expired. Please request a new one from Discord using /create-character.")
		return
	}
	if errors.Is(err, ErrTokenUsed) {
		h.renderError(w, http.StatusGone, "Link already been used", "This link has already been used. Please request a new one from Discord using /create-character.")
		return
	}
	if errors.Is(err, ErrTokenNotFound) {
		h.renderError(w, http.StatusNotFound, "Link not found", "This link is invalid or not found. Please use the link from Discord.")
		return
	}
	h.logger.Error("token validation error", "error", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

// ErrorData holds data for error pages.
type ErrorData struct {
	Title   string
	Message string
}

func (h *Handler) renderError(w http.ResponseWriter, status int, title, message string) {
	h.renderWithStatus(w, status, h.errorTmpl, ErrorData{Title: title, Message: message})
}

// render executes a template into a buffer and writes the response with 200 OK.
func (h *Handler) render(w http.ResponseWriter, tmpl *template.Template, data any) {
	h.renderWithStatus(w, http.StatusOK, tmpl, data)
}

// renderWithStatus executes a template into a buffer and writes the response with the given status.
func (h *Handler) renderWithStatus(w http.ResponseWriter, status int, tmpl *template.Template, data any) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		h.logger.Error("failed to render template", "template", tmpl.Name(), "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write(buf.Bytes())
}

const landingTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DnDnD — Player Portal</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: system-ui, -apple-system, sans-serif; min-height: 100vh; background: #1a1a2e; color: #e0e0e0; }
        .header { background: #16213e; border-bottom: 1px solid #0f3460; padding: 1rem 2rem; display: flex; align-items: center; justify-content: space-between; }
        .header h1 { color: #e94560; font-size: 1.4rem; }
        .header nav { display: flex; gap: 1.5rem; }
        .header nav a { color: #e0e0e0; text-decoration: none; padding: 0.5rem 0; }
        .header nav a:hover { color: #e94560; }
        .main { max-width: 800px; margin: 2rem auto; padding: 0 1rem; }
        .main h2 { color: #e94560; margin-bottom: 1rem; }
        .card { background: #16213e; border-radius: 8px; padding: 1.5rem; border: 1px solid #0f3460; margin-bottom: 1rem; }
        @media (max-width: 600px) {
            .header { flex-direction: column; gap: 0.5rem; text-align: center; }
            .main { padding: 0 0.5rem; }
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>DnDnD Player Portal</h1>
        <nav>
            <a href="/portal/">Home</a>
            <a href="/portal/auth/logout">Logout</a>
        </nav>
    </div>
    <div class="main">
        <h2>Welcome</h2>
        <div class="card">
            <p>Use the links from Discord to create or view your characters.</p>
        </div>
    </div>
</body>
</html>`

const createTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DnDnD — Character Builder</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: system-ui, -apple-system, sans-serif; min-height: 100vh; background: #1a1a2e; color: #e0e0e0; }
        .header { background: #16213e; border-bottom: 1px solid #0f3460; padding: 1rem 2rem; display: flex; align-items: center; justify-content: space-between; }
        .header h1 { color: #e94560; font-size: 1.4rem; }
        .header nav { display: flex; gap: 1.5rem; }
        .header nav a { color: #e0e0e0; text-decoration: none; padding: 0.5rem 0; }
        .header nav a:hover { color: #e94560; }
        .main { max-width: 800px; margin: 2rem auto; padding: 0 1rem; }
        .main h2 { color: #e94560; margin-bottom: 1rem; }
        .card { background: #16213e; border-radius: 8px; padding: 1.5rem; border: 1px solid #0f3460; margin-bottom: 1rem; }
        @media (max-width: 600px) {
            .header { flex-direction: column; gap: 0.5rem; text-align: center; }
            .main { padding: 0 0.5rem; }
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>DnDnD Player Portal</h1>
        <nav>
            <a href="/portal/">Home</a>
            <a href="/portal/auth/logout">Logout</a>
        </nav>
    </div>
    <div class="main">
        <h2>Character Builder</h2>
        <div class="card" id="character-builder">
            <p>Loading character builder...</p>
            <input type="hidden" id="portal-token" value="{{.Token}}">
            <input type="hidden" id="campaign-id" value="{{.CampaignID}}">
        </div>
    </div>
    <script type="module" crossorigin src="/portal/app/assets/{{.JSFile}}"></script>
    <link rel="stylesheet" crossorigin href="/portal/app/assets/{{.CSSFile}}">
</body>
</html>`

const errorTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DnDnD — Error</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: system-ui, -apple-system, sans-serif; min-height: 100vh; background: #1a1a2e; color: #e0e0e0; display: flex; flex-direction: column; }
        .header { background: #16213e; border-bottom: 1px solid #0f3460; padding: 1rem 2rem; display: flex; align-items: center; justify-content: space-between; }
        .header h1 { color: #e94560; font-size: 1.4rem; }
        .main { max-width: 600px; margin: 4rem auto; padding: 0 1rem; text-align: center; }
        .main h2 { color: #e94560; margin-bottom: 1rem; }
        .card { background: #16213e; border-radius: 8px; padding: 2rem; border: 1px solid #0f3460; }
    </style>
</head>
<body>
    <div class="header">
        <h1>DnDnD Player Portal</h1>
    </div>
    <div class="main">
        <h2>{{.Title}}</h2>
        <div class="card">
            <p>{{.Message}}</p>
        </div>
    </div>
</body>
</html>`
