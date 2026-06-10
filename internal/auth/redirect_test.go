package auth

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const testLoginPath = "/portal/auth/login"

// unauthMw is an inner middleware that always rejects with 401, mimicking
// SessionMiddleware's response for a missing/invalid session cookie.
func unauthMw(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

// forbidMw always rejects with 403, mimicking RequireDM for an authenticated
// non-DM user.
func forbidMw(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	})
}

// okMw passes the request through to next, which writes a 200 body.
func okMw(next http.Handler) http.Handler {
	return next
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("dashboard"))
	})
}

func TestRedirectNavigationOnUnauth_SecFetchNavigate(t *testing.T) {
	mw := RedirectNavigationOnUnauth(testLoginPath, unauthMw)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	req.Header.Set("Sec-Fetch-Mode", "navigate")

	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	want := testLoginPath + "?next=" + url.QueryEscape("/dashboard/")
	if got := rec.Header().Get("Location"); got != want {
		t.Fatalf("expected Location %q, got %q", want, got)
	}
	if strings.Contains(rec.Body.String(), "unauthorized") {
		t.Fatalf("body should not contain inner 401 text, got %q", rec.Body.String())
	}
}

func TestRedirectNavigationOnUnauth_AcceptHTMLFallback(t *testing.T) {
	mw := RedirectNavigationOnUnauth(testLoginPath, unauthMw)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	want := testLoginPath + "?next=" + url.QueryEscape("/dashboard/")
	if got := rec.Header().Get("Location"); got != want {
		t.Fatalf("expected Location %q, got %q", want, got)
	}
}

// A deep link with a query string (the /create-character portal link) is
// carried through to the login page as a ?next= parameter so the OAuth
// callback can return the player to it.
func TestRedirectNavigationOnUnauth_CarriesNextDeepLink(t *testing.T) {
	mw := RedirectNavigationOnUnauth(testLoginPath, unauthMw)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/portal/create?token=abc", nil)
	req.Header.Set("Sec-Fetch-Mode", "navigate")

	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	want := testLoginPath + "?next=" + url.QueryEscape("/portal/create?token=abc")
	if got := rec.Header().Get("Location"); got != want {
		t.Fatalf("expected Location %q, got %q", want, got)
	}
}

// The root path carries no return target (it is not behind auth and only
// redirects onward), so no ?next= is appended.
func TestRedirectNavigationOnUnauth_RootNoNext(t *testing.T) {
	mw := RedirectNavigationOnUnauth(testLoginPath, unauthMw)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Sec-Fetch-Mode", "navigate")

	mw(okHandler()).ServeHTTP(rec, req)

	if got := rec.Header().Get("Location"); got != testLoginPath {
		t.Fatalf("expected bare Location %q, got %q", testLoginPath, got)
	}
}

func TestSafeReturnPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"/portal/create?token=abc", "/portal/create?token=abc"},
		{"/dashboard/", "/dashboard/"},
		{"//evil.com", ""},
		{"https://evil.com", ""},
		{"http://evil.com/path", ""},
		{`/\evil.com`, ""},
		{"relative/path", ""},
	}
	for _, c := range cases {
		if got := safeReturnPath(c.in); got != c.want {
			t.Errorf("safeReturnPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRedirectNavigationOnUnauth_XHRStays401(t *testing.T) {
	mw := RedirectNavigationOnUnauth(testLoginPath, unauthMw)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/state", nil)
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Accept", "application/json")

	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Fatalf("expected no Location header, got %q", got)
	}
	if !strings.Contains(rec.Body.String(), "unauthorized") {
		t.Fatalf("expected inner 401 body, got %q", rec.Body.String())
	}
}

func TestRedirectNavigationOnUnauth_POSTStays401(t *testing.T) {
	mw := RedirectNavigationOnUnauth(testLoginPath, unauthMw)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/dashboard/", nil)
	req.Header.Set("Accept", "text/html")

	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for non-GET, got %d", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Fatalf("expected no Location header, got %q", got)
	}
}

func TestRedirectNavigationOnUnauth_PassesThrough200(t *testing.T) {
	mw := RedirectNavigationOnUnauth(testLoginPath, okMw)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	req.Header.Set("Sec-Fetch-Mode", "navigate")

	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "dashboard" {
		t.Fatalf("expected body intact, got %q", rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Fatalf("expected no Location header, got %q", got)
	}
}

func TestRedirectNavigationOnUnauth_Forbidden403NotRedirected(t *testing.T) {
	mw := RedirectNavigationOnUnauth(testLoginPath, forbidMw)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	req.Header.Set("Sec-Fetch-Mode", "navigate")

	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 to pass through, got %d", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Fatalf("403 must not redirect, got Location %q", got)
	}
}

func TestIsNavigationRequest(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		secFetch  string
		accept    string
		want      bool
	}{
		{name: "GET with navigate", method: http.MethodGet, secFetch: "navigate", want: true},
		{name: "GET with cors", method: http.MethodGet, secFetch: "cors", accept: "application/json", want: false},
		{name: "GET with text/html accept", method: http.MethodGet, accept: "text/html,application/xhtml+xml", want: true},
		{name: "GET with json accept", method: http.MethodGet, accept: "application/json", want: false},
		{name: "GET with no hints", method: http.MethodGet, want: false},
		{name: "POST navigate is not navigation", method: http.MethodPost, secFetch: "navigate", want: false},
		{name: "POST text/html is not navigation", method: http.MethodPost, accept: "text/html", want: false},
		{name: "navigate beats missing accept", method: http.MethodGet, secFetch: "navigate", accept: "application/json", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/dashboard/", nil)
			if tt.secFetch != "" {
				req.Header.Set("Sec-Fetch-Mode", tt.secFetch)
			}
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			if got := isNavigationRequest(req); got != tt.want {
				t.Fatalf("isNavigationRequest = %v, want %v", got, tt.want)
			}
		})
	}
}

// hijackSentinel is returned by the test ResponseWriter's Hijack so we can
// prove the interceptor forwards the Hijack call to the underlying writer
// rather than returning its own "not a hijacker" error.
var hijackSentinel = errors.New("test hijack sentinel reached")

// hijackableWriter is an http.ResponseWriter that also implements
// http.Hijacker (returning hijackSentinel) and http.Flusher.
type hijackableWriter struct {
	http.ResponseWriter
	flushed bool
}

func (h *hijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, hijackSentinel
}

func (h *hijackableWriter) Flush() { h.flushed = true }

func TestRedirectNavigationOnUnauth_HijackerPreserved(t *testing.T) {
	// An inner middleware whose handler asserts the ResponseWriter is an
	// http.Hijacker and invokes Hijack — mirroring the websocket upgrade path.
	var hijackErr error
	var asserted bool
	wsMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hj, ok := w.(http.Hijacker)
			asserted = ok
			if !ok {
				return
			}
			_, _, hijackErr = hj.Hijack()
		})
	}

	mw := RedirectNavigationOnUnauth(testLoginPath, wsMw)
	underlying := &hijackableWriter{ResponseWriter: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/ws", nil)

	mw(okHandler()).ServeHTTP(underlying, req)

	if !asserted {
		t.Fatalf("interceptor did not forward http.Hijacker capability")
	}
	if !errors.Is(hijackErr, hijackSentinel) {
		t.Fatalf("expected Hijack forwarded to underlying writer (sentinel), got %v", hijackErr)
	}
}

func TestRedirectNavigationOnUnauth_FlusherPreserved(t *testing.T) {
	underlying := &hijackableWriter{ResponseWriter: httptest.NewRecorder()}
	flushMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		})
	}

	mw := RedirectNavigationOnUnauth(testLoginPath, flushMw)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	mw(okHandler()).ServeHTTP(underlying, req)

	if !underlying.flushed {
		t.Fatalf("interceptor did not forward http.Flusher capability")
	}
}
