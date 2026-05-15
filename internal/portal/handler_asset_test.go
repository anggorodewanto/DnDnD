package portal

import (
	"strings"
	"testing"
)

func TestResolvePortalAssets(t *testing.T) {
	js, css := resolvePortalAssets()
	if !strings.HasPrefix(js, "index-") || !strings.HasSuffix(js, ".js") {
		t.Errorf("expected JS file like index-XXX.js, got %q", js)
	}
	if !strings.HasPrefix(css, "index-") || !strings.HasSuffix(css, ".css") {
		t.Errorf("expected CSS file like index-XXX.css, got %q", css)
	}
}

func TestNewHandler_CreateTemplateContainsResolvedAssets(t *testing.T) {
	h := NewHandler(nil, nil)
	// The template should NOT contain the stale hard-coded hashes
	var buf strings.Builder
	err := h.createTmpl.Execute(&buf, CreateData{Token: "t", CampaignID: "c", UserID: "u"})
	if err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if strings.Contains(html, "index-GlbG7cuy.js") {
		t.Error("template still contains stale JS hash")
	}
	if strings.Contains(html, "index-DwHZaRQb.css") {
		t.Error("template still contains stale CSS hash")
	}
	// Should contain the actual resolved filenames
	js, css := resolvePortalAssets()
	if !strings.Contains(html, js) {
		t.Errorf("template does not contain resolved JS file %q", js)
	}
	if !strings.Contains(html, css) {
		t.Errorf("template does not contain resolved CSS file %q", css)
	}
}
