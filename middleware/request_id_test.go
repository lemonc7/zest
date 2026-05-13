package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lemonc7/zest"
)

// =============================================================================
// RequestID — generates request ID if none present
// =============================================================================

func TestRequestIDGenerates(t *testing.T) {
	mw := RequestID()

	var capturedID string
	next := func(c *zest.Context) error {
		v := c.Get("requestID")
		if v == nil {
			t.Error("expected requestID in context store")
			return nil
		}
		id, ok := v.(string)
		if !ok {
			t.Errorf("expected requestID to be string, got %T", v)
			return nil
		}
		capturedID = id
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No X-Request-ID header.
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedID == "" {
		t.Error("expected a generated request ID")
	}
	// Generated IDs should be hex encoded 32 chars (16 bytes).
	if len(capturedID) != 32 {
		t.Errorf("expected 32-char hex ID, got %d chars: %q", len(capturedID), capturedID)
	}
}

// =============================================================================
// RequestID — uses existing request ID from header
// =============================================================================

func TestRequestIDExisting(t *testing.T) {
	mw := RequestID()

	var capturedID string
	next := func(c *zest.Context) error {
		v := c.Get("requestID")
		id, ok := v.(string)
		if !ok {
			t.Errorf("expected requestID to be string, got %T", v)
			return nil
		}
		capturedID = id
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "incoming-id-abc123")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedID != "incoming-id-abc123" {
		t.Errorf("expected existing ID 'incoming-id-abc123', got %q", capturedID)
	}
}

// =============================================================================
// RequestID — sets X-Request-ID response header
// =============================================================================

func TestRequestIDResponseHeader(t *testing.T) {
	mw := RequestID()

	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	respID := rec.Header().Get("X-Request-ID")
	if respID == "" {
		t.Error("expected X-Request-ID response header")
	}
}

// =============================================================================
// RequestID — custom generator function is used
// =============================================================================

func TestRequestIDCustomGenerator(t *testing.T) {
	cfg := RequestIDConfig{
		Generator: func() string {
			return "custom-gen-id-999"
		},
	}
	mw := RequestID(cfg)

	var capturedID string
	next := func(c *zest.Context) error {
		v := c.Get("requestID")
		id, ok := v.(string)
		if !ok {
			t.Errorf("expected requestID to be string, got %T", v)
			return nil
		}
		capturedID = id
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedID != "custom-gen-id-999" {
		t.Errorf("expected custom generated ID 'custom-gen-id-999', got %q", capturedID)
	}
}

// =============================================================================
// RequestID — request ID stored in context (c.Get("requestID"))
// =============================================================================

func TestRequestIDInContext(t *testing.T) {
	mw := RequestID()

	var storeVal any
	next := func(c *zest.Context) error {
		storeVal = c.Get("requestID")
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "ctx-test-id")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	_ = mw(next)(c)

	if storeVal == nil {
		t.Error("expected requestID in context store")
		return
	}
	if storeVal != "ctx-test-id" {
		t.Errorf("expected 'ctx-test-id', got %v", storeVal)
	}
}

// =============================================================================
// RequestID — custom header name is used
// =============================================================================

func TestRequestIDCustomHeader(t *testing.T) {
	cfg := RequestIDConfig{
		Header: "X-Correlation-ID",
	}
	mw := RequestID(cfg)

	var capturedID string
	next := func(c *zest.Context) error {
		v := c.Get("requestID")
		id, _ := v.(string)
		capturedID = id
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Correlation-ID", "corr-123")
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	_ = mw(next)(c)

	if capturedID != "corr-123" {
		t.Errorf("expected 'corr-123' from custom header, got %q", capturedID)
	}

	// Response should also use the custom header name.
	respID := rec.Header().Get("X-Correlation-ID")
	if respID != "corr-123" {
		t.Errorf("expected 'corr-123' in response header, got %q", respID)
	}
}

// =============================================================================
// RequestID — two different requests get different IDs
// =============================================================================

func TestRequestIDUniqueness(t *testing.T) {
	mw := RequestID()

	ids := make(map[string]bool)

	for i := 0; i < 5; i++ {
		var capturedID string
		next := func(c *zest.Context) error {
			v := c.Get("requestID")
			id, _ := v.(string)
			capturedID = id
			return nil
		}

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		c := zest.NewContext(rec, req)

		_ = mw(next)(c)

		if capturedID == "" {
			t.Error("expected a generated request ID")
			continue
		}
		if ids[capturedID] {
			t.Errorf("duplicate ID generated: %q", capturedID)
		}
		ids[capturedID] = true
	}
}
