package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lemonc7/zest"
)

// mockJWTer implements JWTer for testing.
type mockJWTer struct {
	claims map[string]any
	err    error
}

func (m *mockJWTer) Parse(token string) (map[string]any, error) {
	return m.claims, m.err
}

// =============================================================================
// JWT — missing Authorization header returns 401
// =============================================================================

func TestJWTMissingAuthHeader(t *testing.T) {
	jwter := &mockJWTer{
		claims: map[string]any{"sub": "user1"},
		err:    nil,
	}
	mw := JWT(jwter)
	next := func(c *zest.Context) error {
		t.Error("handler should not be called when auth header is missing")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	// No Authorization header.
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)

	if err == nil {
		t.Fatal("expected error for missing Authorization header")
	}

	httpErr, ok := err.(*zest.HTTPError)
	if !ok {
		t.Fatalf("expected *zest.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", httpErr.Code)
	}
}

// =============================================================================
// JWT — invalid format (not "Bearer xxx") returns 401
// =============================================================================

func TestJWTInvalidFormat(t *testing.T) {
	jwter := &mockJWTer{
		claims: map[string]any{"sub": "user1"},
		err:    nil,
	}
	mw := JWT(jwter)
	next := func(c *zest.Context) error {
		t.Error("handler should not be called for invalid format")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // Not Bearer.
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)

	if err == nil {
		t.Fatal("expected error for invalid token format")
	}
	httpErr, ok := err.(*zest.HTTPError)
	if !ok {
		t.Fatalf("expected *zest.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", httpErr.Code)
	}
}

// =============================================================================
// JWT — valid token passes, claims are stored in context
// =============================================================================

func TestJWTValidToken(t *testing.T) {
	expectedClaims := map[string]any{
		"sub":  "user123",
		"role": "admin",
		"exp":  float64(9999999999),
	}
	jwter := &mockJWTer{
		claims: expectedClaims,
		err:    nil,
	}
	mw := JWT(jwter)

	var captured *zest.Context
	next := func(c *zest.Context) error {
		captured = c
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid.token.here")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify claims were stored in context.
	for k, expected := range expectedClaims {
		got := captured.Get(k)
		if got != expected {
			t.Errorf("claim %q: got %v, want %v", k, got, expected)
		}
	}
}

// =============================================================================
// JWT — skip function works
// =============================================================================

func TestJWTSkip(t *testing.T) {
	jwter := &mockJWTer{
		err: errors.New("should not be called"),
	}
	mw := JWT(jwter, func(c *zest.Context) bool {
		return c.Path == "/public"
	})

	called := false
	next := func(c *zest.Context) error {
		called = true
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	// No Authorization header — but skip function should prevent the 401.
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)
	if err != nil {
		t.Fatalf("unexpected error when skipped: %v", err)
	}
	if !called {
		t.Error("expected handler to be called when skipped")
	}
}

// =============================================================================
// JWT — invalid token (parser returns error) returns 401
// =============================================================================

func TestJWTInvalidToken(t *testing.T) {
	jwter := &mockJWTer{
		claims: nil,
		err:    errors.New("token expired"),
	}
	mw := JWT(jwter)
	next := func(c *zest.Context) error {
		t.Error("handler should not be called for invalid token")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer expired.token")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)

	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	httpErr, ok := err.(*zest.HTTPError)
	if !ok {
		t.Fatalf("expected *zest.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", httpErr.Code)
	}
	if httpErr.Message != "token expired" {
		t.Errorf("expected error message 'token expired', got %q", httpErr.Message)
	}
}

// =============================================================================
// JWT — nil skipper does not cause panic
// =============================================================================

func TestJWTNilSkipper(t *testing.T) {
	jwter := &mockJWTer{
		err: errors.New("token expired"),
	}
	// Passing nil as skipper should not panic.
	mw := JWT(jwter, nil)
	next := func(c *zest.Context) error {
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	// Should return 401 because no auth header, not panicking.
	err := mw(next)(c)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

// =============================================================================
// JWT — only "Bearer" token format with two parts works (covers split edge case)
// =============================================================================

func TestJWTBearerWithSpaces(t *testing.T) {
	jwter := &mockJWTer{
		claims: map[string]any{"ok": true},
		err:    nil,
	}
	mw := JWT(jwter)

	called := false
	next := func(c *zest.Context) error {
		called = true
		return nil
	}

	// Authorization with extra spaces in the header value.
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer my.token.here")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected handler to be called for valid Bearer token")
	}
}
