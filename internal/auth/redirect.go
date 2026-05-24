package auth

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"strings"
)

// RedirectNavigationOnUnauth wraps an auth middleware so that when the inner
// middleware rejects a top-level browser navigation with 401, the response is
// rewritten as a 302 redirect to loginPath. Non-navigation requests (XHR,
// fetch, websocket handshakes) and non-401 responses pass through unchanged —
// in particular a 403 is never redirected (it would loop for an already
// authenticated non-DM user).
func RedirectNavigationOnUnauth(loginPath string, inner func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		wrapped := inner(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &redirectInterceptor{
				ResponseWriter: w,
				req:            r,
				loginPath:      loginPath,
			}
			wrapped.ServeHTTP(rw, r)
		})
	}
}

// redirectInterceptor sits between the inner auth middleware and the real
// ResponseWriter. It rewrites a 401 on a navigation request into a 302
// redirect to loginPath and swallows the inner middleware's error body.
type redirectInterceptor struct {
	http.ResponseWriter
	req         *http.Request
	loginPath   string
	intercepted bool
}

func (w *redirectInterceptor) WriteHeader(code int) {
	if code == http.StatusUnauthorized && isNavigationRequest(w.req) {
		http.Redirect(w.ResponseWriter, w.req, w.loginPath, http.StatusFound)
		w.intercepted = true
		return
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *redirectInterceptor) Write(b []byte) (int, error) {
	if w.intercepted {
		// The redirect already wrote the status line; drop the inner
		// middleware's "unauthorized\n" body so it can't trail the 302.
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}

// Hijack forwards to the underlying writer so the websocket upgrade on
// /dashboard/ws still works once auth has passed. Hijack is only reached
// after the inner middleware authorizes the request, never on the 401 path.
func (w *redirectInterceptor) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("auth: underlying ResponseWriter does not support hijacking")
	}
	return hj.Hijack()
}

// Flush forwards to the underlying writer when it supports flushing; it is a
// no-op otherwise so streaming handlers keep working through the wrapper.
func (w *redirectInterceptor) Flush() {
	f, ok := w.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}
	f.Flush()
}

// isNavigationRequest reports whether r looks like a top-level browser
// navigation (address bar, link click) as opposed to an XHR/fetch/websocket
// handshake. Such navigations are the only requests we redirect to login.
func isNavigationRequest(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	if r.Header.Get("Sec-Fetch-Mode") == "navigate" {
		return true
	}
	// Fallback for browsers/proxies that strip the Sec-Fetch-* headers: a
	// document navigation advertises text/html in its Accept header.
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}
