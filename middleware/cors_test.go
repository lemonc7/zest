package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lemonc7/zest"
)

// =============================================================================
// CORS — preflight OPTIONS returns correct headers
// =============================================================================

func TestCORSPreflight(t *testing.T) {
	mw := CORS()
	next := func(c *zest.Context) error {
		return nil
	}

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check status is 204 NoContent.
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}

	// Check preflight headers.
	if v := rec.Header().Get("Access-Control-Allow-Methods"); v == "" {
		t.Error("expected Access-Control-Allow-Methods header")
	}
	if v := rec.Header().Get("Access-Control-Allow-Headers"); v == "" {
		t.Error("expected Access-Control-Allow-Headers header")
	}
	if v := rec.Header().Get("Access-Control-Max-Age"); v == "" {
		t.Error("expected Access-Control-Max-Age header")
	}
}

// =============================================================================
// CORS — allowed origin returns Access-Control-Allow-Origin
// =============================================================================

func TestCORSAllowedOrigin(t *testing.T) {
	cfg := CORSConfig{
		AllowOrigins: []string{"http://example.com"},
	}
	mw := CORS(cfg)
	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	_ = mw(next)(c)

	allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "http://example.com" {
		t.Errorf("expected Access-Control-Allow-Origin 'http://example.com', got %q", allowOrigin)
	}
}

// =============================================================================
// CORS — disallowed origin returns no CORS headers
// =============================================================================

func TestCORSDisallowedOrigin(t *testing.T) {
	cfg := CORSConfig{
		AllowOrigins: []string{"http://allowed.com"},
	}
	mw := CORS(cfg)
	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://evil.com")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	_ = mw(next)(c)

	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for disallowed origin, got %q", v)
	}
}

// =============================================================================
// CORS — wildcard origin works
// =============================================================================

func TestCORSWildcardOrigin(t *testing.T) {
	cfg := CORSConfig{
		AllowOrigins: []string{"*"},
	}
	mw := CORS(cfg)
	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	_ = mw(next)(c)

	allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin '*', got %q", allowOrigin)
	}
}

// =============================================================================
// CORS — AllowCredentials sets correct header
// =============================================================================

func TestCORSAllowCredentials(t *testing.T) {
	cfg := CORSConfig{
		AllowOrigins:     []string{"http://example.com"},
		AllowCredentials: true,
	}
	mw := CORS(cfg)
	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	_ = mw(next)(c)

	creds := rec.Header().Get("Access-Control-Allow-Credentials")
	if creds != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials 'true', got %q", creds)
	}
}

// =============================================================================
// CORS — AllowCredentials with wildcard origin echoes origin
// =============================================================================

func TestCORSAllowCredentialsWildcard(t *testing.T) {
	cfg := CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: true,
	}
	mw := CORS(cfg)
	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	_ = mw(next)(c)

	allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
	// When credentials are enabled, wildcard must echo the actual origin.
	if allowOrigin != "http://example.com" {
		t.Errorf("expected Access-Control-Allow-Origin 'http://example.com' with credentials+wildcard, got %q", allowOrigin)
	}
}

// =============================================================================
// CORS — custom AllowOriginFunc
// =============================================================================

func TestCORSAllowOriginFunc(t *testing.T) {
	cfg := CORSConfig{
		AllowOriginFunc: func(origin string) bool {
			return origin == "http://func-allowed.com"
		},
	}
	mw := CORS(cfg)
	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	// Allowed origin.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://func-allowed.com")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)
	_ = mw(next)(c)

	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "http://func-allowed.com" {
		t.Errorf("expected origin allowed by func, got %q", v)
	}

	// Disallowed origin.
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("Origin", "http://bad.com")
	rec2 := httptest.NewRecorder()
	c2 := zest.NewContext(rec2, req2)
	_ = mw(next)(c2)

	if v := rec2.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("expected no origin for disallowed by func, got %q", v)
	}
}

// =============================================================================
// CORS — MaxAge header
// =============================================================================

func TestCORSMaxAge(t *testing.T) {
	cfg := CORSConfig{
		AllowOrigins: []string{"*"},
		MaxAge:       600 * time.Second,
	}
	mw := CORS(cfg)
	next := func(c *zest.Context) error {
		return nil
	}

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	_ = mw(next)(c)

	maxAge := rec.Header().Get("Access-Control-Max-Age")
	if maxAge != "600" {
		t.Errorf("expected Access-Control-Max-Age '600', got %q", maxAge)
	}
}

// =============================================================================
// CORS — Vary header when multiple origins
// =============================================================================

func TestCORSVaryMultipleOrigins(t *testing.T) {
	cfg := CORSConfig{
		AllowOrigins: []string{"http://one.com", "http://two.com"},
	}
	mw := CORS(cfg)
	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://one.com")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	_ = mw(next)(c)

	vary := rec.Header().Get("Vary")
	if vary != "Origin" {
		t.Errorf("expected Vary: Origin with multiple allowed origins, got %q", vary)
	}
}

// =============================================================================
// CORS — no Vary header when single origin
// =============================================================================

func TestCORSNoVarySingleOrigin(t *testing.T) {
	cfg := CORSConfig{
		AllowOrigins: []string{"http://only.com"},
	}
	mw := CORS(cfg)
	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://only.com")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	_ = mw(next)(c)

	vary := rec.Header().Get("Vary")
	if vary == "Origin" {
		t.Error("expected no Vary header with single allowed origin")
	}
}

// =============================================================================
// CORS — no Origin header passes through
// =============================================================================

func TestCORSNoOriginHeader(t *testing.T) {
	cfg := CORSConfig{
		AllowOrigins: []string{"http://example.com"},
	}
	mw := CORS(cfg)

	called := false
	next := func(c *zest.Context) error {
		called = true
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No Origin header set.
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	_ = mw(next)(c)

	if !called {
		t.Error("expected next handler to be called when no Origin header")
	}
}

// =============================================================================
// CORS — ExposeHeaders header
// =============================================================================

func TestCORSExposeHeaders(t *testing.T) {
	cfg := CORSConfig{
		AllowOrigins:  []string{"*"},
		ExposeHeaders: []string{"X-Custom-Header", "X-Another"},
	}
	mw := CORS(cfg)
	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	_ = mw(next)(c)

	expose := rec.Header().Get("Access-Control-Expose-Headers")
	if expose != "X-Custom-Header, X-Another" {
		t.Errorf("expected Expose-Headers 'X-Custom-Header, X-Another', got %q", expose)
	}
}
