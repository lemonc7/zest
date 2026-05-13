package zest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testRequest is a helper that creates an HTTP request and serves it through the Zest instance.
func testRequest(z *Zest, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	z.ServeHTTP(rec, req)
	return rec
}

// TestNew verifies that New() creates a properly initialized Zest instance.
func TestNew(t *testing.T) {
	z := New()

	if z == nil {
		t.Fatal("New() returned nil")
	}
	if z.mux == nil {
		t.Error("mux is nil")
	}
	if z.ErrHandler == nil {
		t.Error("ErrHandler is nil")
	}
	if z.pool.New == nil {
		t.Error("pool.New is nil")
	}

	// Verify pool produces a valid context
	c := z.pool.Get().(*Context)
	if c == nil {
		t.Fatal("pool.Get() returned nil context")
	}
	z.pool.Put(c)

	// Verify the 404 catch-all route works
	rec := testRequest(z, http.MethodGet, "/nonexistent")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent route, got %d", rec.Code)
	}

	// Verify 404 response is JSON
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON response body, got error: %v", err)
	}
	if body["error"] != "Not Found" {
		t.Errorf("expected error message 'not found', got %v", body["error"])
	}
}

// TestZest_GET verifies GET route registration and dispatching.
func TestZest_GET(t *testing.T) {
	z := New()

	z.GET("/hello", func(c *Context) error {
		return c.String(http.StatusOK, "hello get")
	})

	// GET should work
	rec := testRequest(z, http.MethodGet, "/hello")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET /hello, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "hello get" {
		t.Errorf("expected body 'hello get', got %q", rec.Body.String())
	}

	// POST on same path should 404
	rec = testRequest(z, http.MethodPost, "/hello")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for POST /hello, got %d", rec.Code)
	}
}

// TestZest_POST verifies POST route registration and dispatching.
func TestZest_POST(t *testing.T) {
	z := New()

	z.POST("/submit", func(c *Context) error {
		return c.String(http.StatusCreated, "created via post")
	})

	// POST should work
	rec := testRequest(z, http.MethodPost, "/submit")
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201 for POST /submit, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "created via post" {
		t.Errorf("expected body 'created via post', got %q", rec.Body.String())
	}

	// GET on same path should 404
	rec = testRequest(z, http.MethodGet, "/submit")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for GET /submit, got %d", rec.Code)
	}
}

// TestZest_PUT verifies PUT route registration and dispatching.
func TestZest_PUT(t *testing.T) {
	z := New()

	z.PUT("/items/1", func(c *Context) error {
		return c.String(http.StatusOK, "updated")
	})

	rec := testRequest(z, http.MethodPut, "/items/1")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for PUT /items/1, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "updated" {
		t.Errorf("expected body 'updated', got %q", rec.Body.String())
	}

	// GET on PUT path should 404
	rec = testRequest(z, http.MethodGet, "/items/1")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for GET on PUT-only path, got %d", rec.Code)
	}
}

// TestZest_PATCH verifies PATCH route registration and dispatching.
func TestZest_PATCH(t *testing.T) {
	z := New()

	z.PATCH("/items/1", func(c *Context) error {
		return c.String(http.StatusOK, "patched")
	})

	rec := testRequest(z, http.MethodPatch, "/items/1")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for PATCH /items/1, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "patched" {
		t.Errorf("expected body 'patched', got %q", rec.Body.String())
	}

	// GET on PATCH path should 404
	rec = testRequest(z, http.MethodGet, "/items/1")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for GET on PATCH-only path, got %d", rec.Code)
	}
}

// TestZest_DELETE verifies DELETE route registration and dispatching.
func TestZest_DELETE(t *testing.T) {
	z := New()

	z.DELETE("/items/1", func(c *Context) error {
		return c.NoContent()
	})

	rec := testRequest(z, http.MethodDelete, "/items/1")
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for DELETE /items/1, got %d", rec.Code)
	}

	// GET on DELETE path should 404
	rec = testRequest(z, http.MethodGet, "/items/1")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for GET on DELETE-only path, got %d", rec.Code)
	}
}

// TestZest_OPTIONS verifies OPTIONS route registration and dispatching.
func TestZest_OPTIONS(t *testing.T) {
	z := New()

	z.OPTIONS("/items", func(c *Context) error {
		c.SetHeader("Allow", "GET, POST, OPTIONS")
		return c.NoContent()
	})

	rec := testRequest(z, http.MethodOptions, "/items")
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS /items, got %d", rec.Code)
	}
	if rec.Header().Get("Allow") != "GET, POST, OPTIONS" {
		t.Errorf("expected Allow header 'GET, POST, OPTIONS', got %q", rec.Header().Get("Allow"))
	}

	// GET on OPTIONS path should 404
	rec = testRequest(z, http.MethodGet, "/items")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for GET on OPTIONS-only path, got %d", rec.Code)
	}
}

// TestZest_HEAD verifies HEAD route registration and dispatching.
func TestZest_HEAD(t *testing.T) {
	z := New()

	z.HEAD("/data", func(c *Context) error {
		c.SetHeader("X-Custom", "head-ok")
		return c.NoContent()
	})

	rec := testRequest(z, http.MethodHead, "/data")
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for HEAD /data, got %d", rec.Code)
	}
	if rec.Header().Get("X-Custom") != "head-ok" {
		t.Errorf("expected X-Custom header 'head-ok', got %q", rec.Header().Get("X-Custom"))
	}

	// GET on HEAD path should 404
	rec = testRequest(z, http.MethodGet, "/data")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for GET on HEAD-only path, got %d", rec.Code)
	}
}

// TestZest_RouteWithPathParam tests Go 1.22+ path parameter matching.
func TestZest_RouteWithPathParam(t *testing.T) {
	z := New()

	var capturedID string
	z.GET("/users/{id}", func(c *Context) error {
		capturedID = c.Param("id")
		return c.String(http.StatusOK, "user:"+capturedID)
	})

	// Request /users/42 — Go 1.22 mux should match {id} and set PathValue
	rec := testRequest(z, http.MethodGet, "/users/42")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET /users/42, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "user:42" {
		t.Errorf("expected body 'user:42', got %q", rec.Body.String())
	}
	if capturedID != "42" {
		t.Errorf("expected capturedID '42', got %q", capturedID)
	}

	// Request /users/99 — verify different id
	rec = testRequest(z, http.MethodGet, "/users/99")
	if strings.TrimSpace(rec.Body.String()) != "user:99" {
		t.Errorf("expected body 'user:99', got %q", rec.Body.String())
	}
	if capturedID != "99" {
		t.Errorf("expected capturedID '99', got %q", capturedID)
	}
}

// TestZest_RouteWithWildcard tests Go 1.22+ wildcard path matching.
func TestZest_RouteWithWildcard(t *testing.T) {
	z := New()

	var capturedPath string
	z.GET("/files/{path...}", func(c *Context) error {
		capturedPath = c.Param("path")
		return c.String(http.StatusOK, "file:"+capturedPath)
	})

	// Single segment
	rec := testRequest(z, http.MethodGet, "/files/readme.txt")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET /files/readme.txt, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "file:readme.txt" {
		t.Errorf("expected body 'file:readme.txt', got %q", rec.Body.String())
	}
	if capturedPath != "readme.txt" {
		t.Errorf("expected capturedPath 'readme.txt', got %q", capturedPath)
	}

	// Nested path
	rec = testRequest(z, http.MethodGet, "/files/a/b/c/deep.png")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET /files/a/b/c/deep.png, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "file:a/b/c/deep.png" {
		t.Errorf("expected body 'file:a/b/c/deep.png', got %q", rec.Body.String())
	}
	if capturedPath != "a/b/c/deep.png" {
		t.Errorf("expected capturedPath 'a/b/c/deep.png', got %q", capturedPath)
	}
}

// TestZest_Middleware verifies that a route-level middleware runs and modifies the response.
func TestZest_Middleware(t *testing.T) {
	z := New()

	addHeader := func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			c.SetHeader("X-Middleware", "applied")
			return next(c)
		}
	}

	z.GET("/mw", func(c *Context) error {
		return c.String(http.StatusOK, "ok")
	}, addHeader)

	rec := testRequest(z, http.MethodGet, "/mw")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET /mw, got %d", rec.Code)
	}
	if rec.Header().Get("X-Middleware") != "applied" {
		t.Errorf("expected X-Middleware header 'applied', got %q", rec.Header().Get("X-Middleware"))
	}
	if strings.TrimSpace(rec.Body.String()) != "ok" {
		t.Errorf("expected body 'ok', got %q", rec.Body.String())
	}
}

// TestZest_MultipleMiddleware tests the onion pattern ordering of multiple middlewares.
// The `use` function applies middlewares in reverse order, so the first middleware
// in the list wraps the outermost layer and runs first on the way in, last on the way out.
func TestZest_MultipleMiddleware(t *testing.T) {
	z := New()
	execOrder := make([]string, 0)

	// The onion: mw1(mw2(handler))
	// Request comes in: mw1 -> mw2 -> handler
	// Response goes out: handler -> mw2 -> mw1
	mw1 := func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			execOrder = append(execOrder, "mw1-before")
			err := next(c)
			execOrder = append(execOrder, "mw1-after")
			return err
		}
	}
	mw2 := func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			execOrder = append(execOrder, "mw2-before")
			err := next(c)
			execOrder = append(execOrder, "mw2-after")
			return err
		}
	}

	z.GET("/onion", func(c *Context) error {
		execOrder = append(execOrder, "handler")
		return c.String(http.StatusOK, "done")
	}, mw1, mw2)

	rec := testRequest(z, http.MethodGet, "/onion")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// Expected order: mw1-before, mw2-before, handler, mw2-after, mw1-after
	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(execOrder) != len(expected) {
		t.Fatalf("expected %d exec steps, got %d: %v", len(expected), len(execOrder), execOrder)
	}
	for i, step := range expected {
		if execOrder[i] != step {
			t.Errorf("step %d: expected %q, got %q", i, step, execOrder[i])
		}
	}
}

// TestZest_Static verifies static file serving.
func TestZest_Static(t *testing.T) {
	// Create a temp directory with a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hello.txt")
	if err := os.WriteFile(testFile, []byte("hello static"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Also create a nested file
	nestedDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}
	nestedFile := filepath.Join(nestedDir, "nested.txt")
	if err := os.WriteFile(nestedFile, []byte("nested content"), 0644); err != nil {
		t.Fatalf("failed to create nested file: %v", err)
	}

	z := New()
	z.Static("/public/", tmpDir)

	// Request existing file
	rec := testRequest(z, http.MethodGet, "/public/hello.txt")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for /public/hello.txt, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "hello static" {
		t.Errorf("expected body 'hello static', got %q", rec.Body.String())
	}

	// Request nested file
	rec = testRequest(z, http.MethodGet, "/public/sub/nested.txt")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for /public/sub/nested.txt, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "nested content" {
		t.Errorf("expected body 'nested content', got %q", rec.Body.String())
	}

	// Request non-existent file should 404
	rec = testRequest(z, http.MethodGet, "/public/missing.txt")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing static file, got %d", rec.Code)
	}

	// HEAD request for static file
	rec = testRequest(z, http.MethodHead, "/public/hello.txt")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for HEAD /public/hello.txt, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") == "" {
		t.Error("expected Content-Type header on HEAD response")
	}
}

// TestZest_404 verifies that unregistered routes return a 404 JSON response.
func TestZest_404(t *testing.T) {
	z := New()

	// Register some routes to ensure they don't affect 404 handling
	z.GET("/home", func(c *Context) error {
		return c.String(http.StatusOK, "home")
	})

	// Request unregistered path
	rec := testRequest(z, http.MethodGet, "/not-registered")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}

	// Verify JSON structure
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON response body, got error: %v", err)
	}
	if body["error"] != "Not Found" {
		t.Errorf("expected error message 'not found', got %v", body["error"])
	}
}

// TestZest_CustomErrorHandler verifies that a custom ErrHandler is called on error.
func TestZest_CustomErrorHandler(t *testing.T) {
	z := New()

	customCalled := false
	z.ErrHandler = func(c *Context, err error) {
		customCalled = true
		c.JSON(http.StatusBadGateway, Map{"custom_error": err.Error()})
	}

	// Register a route that returns an error
	z.GET("/boom", func(c *Context) error {
		return NewHTTPError(http.StatusInternalServerError, "something went wrong")
	})

	rec := testRequest(z, http.MethodGet, "/boom")
	if !customCalled {
		t.Error("expected custom error handler to be called")
	}
	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON response body, got error: %v", err)
	}
	if body["custom_error"] != "something went wrong" {
		t.Errorf("expected custom_error 'something went wrong', got %v", body["custom_error"])
	}
}

// TestZest_Use verifies that Use() adds global middlewares which run on every request.
func TestZest_Use(t *testing.T) {
	z := New()

	z.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			c.SetHeader("X-Global", "applied")
			return next(c)
		}
	})

	z.GET("/a", func(c *Context) error {
		return c.String(http.StatusOK, "a")
	})
	z.GET("/b", func(c *Context) error {
		return c.String(http.StatusOK, "b")
	})

	// Both routes should have the global header
	for _, path := range []string{"/a", "/b"} {
		rec := testRequest(z, http.MethodGet, path)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for GET %s, got %d", path, rec.Code)
		}
		if rec.Header().Get("X-Global") != "applied" {
			t.Errorf("expected X-Global header 'applied' on %s, got %q", path, rec.Header().Get("X-Global"))
		}
	}
}
