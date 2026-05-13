package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lemonc7/zest"
)

// =============================================================================
// Recovery — normal request passes through
// =============================================================================

func TestRecoveryNormalRequest(t *testing.T) {
	mw := Recovery()

	called := false
	next := func(c *zest.Context) error {
		called = true
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
	if !called {
		t.Error("expected handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// =============================================================================
// Recovery — panic in handler is recovered, returns 500
// =============================================================================

func TestRecoveryPanic(t *testing.T) {
	mw := Recovery()

	next := func(c *zest.Context) error {
		panic("something went horribly wrong")
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)
	if err == nil {
		t.Fatal("expected error from recovered panic")
	}

	httpErr, ok := err.(*zest.HTTPError)
	if !ok {
		t.Fatalf("expected *zest.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", httpErr.Code)
	}
}

// =============================================================================
// Recovery — panic value is captured in error message
// =============================================================================

func TestRecoveryPanicValue(t *testing.T) {
	mw := Recovery()

	next := func(c *zest.Context) error {
		panic("custom panic value 42")
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)

	httpErr, ok := err.(*zest.HTTPError)
	if !ok {
		t.Fatalf("expected *zest.HTTPError, got %T", err)
	}
	if httpErr.Message != "custom panic value 42" {
		t.Errorf("expected message 'custom panic value 42', got %q", httpErr.Message)
	}
}

// =============================================================================
// Recovery — panic with non-string value is captured
// =============================================================================

func TestRecoveryPanicNonString(t *testing.T) {
	mw := Recovery()

	next := func(c *zest.Context) error {
		panic(12345)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)

	httpErr, ok := err.(*zest.HTTPError)
	if !ok {
		t.Fatalf("expected *zest.HTTPError, got %T", err)
	}
	if httpErr.Message != "12345" {
		t.Errorf("expected message '12345', got %q", httpErr.Message)
	}
}

// =============================================================================
// Recovery — custom LogFunc is called on panic
// =============================================================================

func TestRecoveryCustomLogFunc(t *testing.T) {
	var logCalls []string
	customLog := func(format string, v ...any) {
		logCalls = append(logCalls, fmt.Sprintf(format, v...))
	}

	cfg := RecoveryConfig{
		LogFunc: customLog,
	}
	mw := Recovery(cfg)

	next := func(c *zest.Context) error {
		panic("logged panic")
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	_ = mw(next)(c)

	if len(logCalls) == 0 {
		t.Error("expected LogFunc to be called on panic")
	}
	// The formatted output should contain the recovery header and the panic value.
	output := logCalls[0]
	if !strings.Contains(output, "[Recovery] panic recovered") {
		t.Errorf("expected recovery header in log output, got: %s", output)
	}
	if !strings.Contains(output, "logged panic") {
		t.Errorf("expected panic value in log output, got: %s", output)
	}
}

// =============================================================================
// Recovery — handler returning an error passes through normally
// =============================================================================

func TestRecoveryHandlerError(t *testing.T) {
	mw := Recovery()

	next := func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusBadRequest, "bad request")
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)
	if err == nil {
		t.Fatal("expected error to propagate")
	}

	httpErr, ok := err.(*zest.HTTPError)
	if !ok {
		t.Fatalf("expected *zest.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", httpErr.Code)
	}
}

func TestRecoveryBrokenPipe(t *testing.T) {
	mw := Recovery()

	next := func(c *zest.Context) error {
		// net.OpError implements the netError interface used in recovery
		// We create an error that has Error() containing "broken pipe"
		panic(&testNetError{msg: "write tcp 127.0.0.1:8080->127.0.0.1:54321: write: broken pipe"})
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	// Recovery should return nil for broken pipe (no error propagated)
	err := mw(next)(c)
	if err != nil {
		t.Errorf("expected nil error for broken pipe, got %v", err)
	}
}

func TestRecoveryConnectionReset(t *testing.T) {
	mw := Recovery()

	next := func(c *zest.Context) error {
		panic(&testNetError{msg: "read tcp 127.0.0.1:8080->127.0.0.1:54321: read: connection reset by peer"})
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	err := mw(next)(c)
	if err != nil {
		t.Errorf("expected nil error for connection reset, got %v", err)
	}
}

// testNetError implements the netError interface used by Recovery
type testNetError struct {
	msg string
}

func (e *testNetError) Error() string {
	return e.msg
}
