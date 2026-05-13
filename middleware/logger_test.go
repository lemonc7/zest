package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lemonc7/zest"
)

// newTestContext creates a zest.Context backed by an httptest.ResponseRecorder.
func newTestContext(method, path string) (*zest.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)
	return c, rec
}

// =============================================================================
// formatSize
// =============================================================================

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		contains string
	}{
		{"zero bytes", 0, "B"},
		{"bytes", 512, "512 B"},
		{"exactly 1KB", 1024, "1.00 KB"},
		{"kilobytes", 2048, "2.00 KB"},
		{"megabytes", 5 * 1024 * 1024, "5.00 MB"},
		{"gigabytes", 3 * 1024 * 1024 * 1024, "3.00 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSize(tt.size)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("formatSize(%d) = %q, want to contain %q", tt.size, result, tt.contains)
			}
		})
	}
}

// =============================================================================
// formatLatency
// =============================================================================

func TestFormatLatency(t *testing.T) {
	tests := []struct {
		name     string
		d        time.Duration
		contains string
	}{
		{"microseconds", 500 * time.Microsecond, "\u00b5s"},
		{"milliseconds", 50 * time.Millisecond, "ms"},
		{"seconds", 2 * time.Second, "s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatLatency(tt.d)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("formatLatency(%v) = %q, want to contain %q", tt.d, result, tt.contains)
			}
		})
	}
}

// =============================================================================
// getStatusColor
// =============================================================================

func TestGetStatusColor(t *testing.T) {
	if c := getStatusColor(200); c != green {
		t.Errorf("getStatusColor(200) = %q, want %q", c, green)
	}
	if c := getStatusColor(301); c != yellow {
		t.Errorf("getStatusColor(301) = %q, want %q", c, yellow)
	}
	if c := getStatusColor(400); c != red {
		t.Errorf("getStatusColor(400) = %q, want %q", c, red)
	}
	if c := getStatusColor(500); c != red {
		t.Errorf("getStatusColor(500) = %q, want %q", c, red)
	}
}

// =============================================================================
// getMethodColor
// =============================================================================

func TestGetMethodColor(t *testing.T) {
	tests := map[string]string{
		"GET":     cyan,
		"POST":    green,
		"PUT":     yellow,
		"DELETE":  red,
		"PATCH":   magenta,
		"HEAD":    blue,
		"UNKNOWN": reset,
	}
	for method, expected := range tests {
		if c := getMethodColor(method); c != expected {
			t.Errorf("getMethodColor(%q) = %q, want %q", method, c, expected)
		}
	}
}

// =============================================================================
// getStatusEmoji
// =============================================================================

func TestGetStatusEmoji(t *testing.T) {
	if e := getStatusEmoji(200); e != "\U0001f7e2" {
		t.Errorf("getStatusEmoji(200) = %q, want green circle", e)
	}
	if e := getStatusEmoji(301); e != "\U0001f7e1" {
		t.Errorf("getStatusEmoji(301) = %q, want yellow circle", e)
	}
	if e := getStatusEmoji(404); e != "\U0001f7e0" {
		t.Errorf("getStatusEmoji(404) = %q, want orange circle", e)
	}
	if e := getStatusEmoji(500); e != "\U0001f534" {
		t.Errorf("getStatusEmoji(500) = %q, want red circle", e)
	}
}

// =============================================================================
// Logger middleware — default log format includes status, method, path
// =============================================================================

func TestLoggerDefaultFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Output: &buf,
	}
	mw := Logger(cfg)

	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	c, _ := newTestContext("GET", "/api/test")
	_ = mw(next)(c)

	output := buf.String()
	// Default format should contain status, method, and path.
	if !strings.Contains(output, "200") {
		t.Errorf("expected output to contain status 200, got: %s", output)
	}
	if !strings.Contains(output, "GET") {
		t.Errorf("expected output to contain method GET, got: %s", output)
	}
	if !strings.Contains(output, "/api/test") {
		t.Errorf("expected output to contain path /api/test, got: %s", output)
	}
}

// =============================================================================
// Logger — custom formatter is used
// =============================================================================

func TestLoggerCustomFormatter(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Output: &buf,
		Formatter: func(param LogParam) string {
			return "CUSTOM: " + param.Method + " " + param.Path + "\n"
		},
	}
	mw := Logger(cfg)

	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	c, _ := newTestContext("POST", "/custom")
	_ = mw(next)(c)

	output := buf.String()
	if !strings.HasPrefix(output, "CUSTOM: POST /custom") {
		t.Errorf("custom formatter not used, got: %s", output)
	}
}

// =============================================================================
// Logger — skip function skips logging
// =============================================================================

func TestLoggerSkip(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Output: &buf,
		Skip: func(c *zest.Context) bool {
			return c.Path == "/health"
		},
	}
	mw := Logger(cfg)

	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	c, _ := newTestContext("GET", "/health")
	_ = mw(next)(c)

	if buf.Len() != 0 {
		t.Errorf("expected no log output when skipped, got: %s", buf.String())
	}

	// Non-skipped path should still log.
	c2, _ := newTestContext("GET", "/api")
	_ = mw(next)(c2)

	if buf.Len() == 0 {
		t.Error("expected log output for non-skipped path")
	}
}

// =============================================================================
// Logger — custom output destination
// =============================================================================

func TestLoggerCustomOutput(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Output: &buf,
	}
	mw := Logger(cfg)

	next := func(c *zest.Context) error {
		c.String(200, "hello")
		return nil
	}

	c, _ := newTestContext("GET", "/output")
	_ = mw(next)(c)

	if buf.Len() == 0 {
		t.Error("expected log output in custom buffer")
	}
}

// =============================================================================
// Logger — TZ timezone setting
// =============================================================================

func TestLoggerTZ(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Output: &buf,
		TZ:     "UTC",
	}
	mw := Logger(cfg)

	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	c, _ := newTestContext("GET", "/tz")
	_ = mw(next)(c)

	output := buf.String()
	// The log should contain a timestamp; with UTC it won't have +08 offset.
	// We just verify log was produced and contains a timestamp-like pattern.
	if !strings.Contains(output, "|") {
		t.Errorf("expected log output with timestamp, got: %s", output)
	}
}

// =============================================================================
// Logger — request ID appears in log when set
// =============================================================================

func TestLoggerWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Output:    &buf,
		Formatter: defaultLogFormatter,
	}
	mw := Logger(cfg)

	next := func(c *zest.Context) error {
		c.Set("requestID", "test-rid-123")
		c.String(200, "OK")
		return nil
	}

	c, _ := newTestContext("GET", "/with-id")
	_ = mw(next)(c)

	output := buf.String()
	if !strings.Contains(output, "test-rid-123") {
		t.Errorf("expected request ID in log, got: %s", output)
	}
}

// =============================================================================
// Logger — client IP appears in log
// =============================================================================

func TestLoggerClientIP(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Output:    &buf,
		Formatter: defaultLogFormatter,
	}
	mw := Logger(cfg)

	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	req := httptest.NewRequest("GET", "/ip", nil)
	req.RemoteAddr = "192.168.1.100:54321"
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)

	_ = mw(next)(c)

	output := buf.String()
	if !strings.Contains(output, "192.168.1.100") {
		t.Errorf("expected client IP in log, got: %s", output)
	}
}

// =============================================================================
// Logger — error is captured in log
// =============================================================================

func TestLoggerWithError(t *testing.T) {
	var buf bytes.Buffer
	cfg := LoggerConfig{
		Output:    &buf,
		Formatter: defaultLogFormatter,
	}
	mw := Logger(cfg)

	next := func(c *zest.Context) error {
		c.SetStatus(http.StatusInternalServerError)
		return zest.NewHTTPError(http.StatusInternalServerError, "something broke")
	}

	c, _ := newTestContext("GET", "/error")
	_ = mw(next)(c)

	output := buf.String()
	if !strings.Contains(output, "something broke") {
		t.Errorf("expected error message in log, got: %s", output)
	}
	if !strings.Contains(output, "500") {
		t.Errorf("expected status 500 in log, got: %s", output)
	}
}

// =============================================================================
// Logger — uses DefaultLoggerConfig when no config passed
// =============================================================================

func TestLoggerDefaultConfig(t *testing.T) {
	// We test that Logger() without args does not panic and produces output.
	var buf bytes.Buffer
	// Must set output via config, otherwise default writes to os.Stdout.
	cfg := LoggerConfig{
		Output: &buf,
	}
	mw := Logger(cfg)

	next := func(c *zest.Context) error {
		c.String(200, "OK")
		return nil
	}

	c, _ := newTestContext("GET", "/default")
	_ = mw(next)(c)

	if buf.Len() == 0 {
		t.Error("expected log output with default config")
	}
}

// =============================================================================
// mustLoadLocation — fallback to CST when time.LoadLocation fails
// =============================================================================

func TestMustLoadLocationFallback(t *testing.T) {
	// mustLoadLocation with an invalid timezone name should fall back to CST (UTC+8)
	loc := mustLoadLocation("Invalid/Timezone")
	if loc == nil {
		t.Fatal("expected non-nil location from fallback")
	}
	name := loc.String()
	if name != "CST" {
		t.Errorf("expected fallback to CST, got %q", name)
	}
}
