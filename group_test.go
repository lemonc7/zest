package zest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestGroup_GET
// ---------------------------------------------------------------------------
func TestGroup_GET(t *testing.T) {
	z := New()
	api := z.Group("/api")

	api.GET("/hello", func(c *Context) error {
		return c.String(http.StatusOK, "hello from group")
	})

	srv := httptest.NewServer(z)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if !strings.Contains(string(body), "hello from group") {
		t.Fatalf("expected body to contain 'hello from group', got %s", string(body))
	}

	// Also verify the route does NOT match without the prefix.
	resp2, err := http.Get(srv.URL + "/hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for route outside group prefix, got %d", resp2.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// TestGroup_POST
// ---------------------------------------------------------------------------
func TestGroup_POST(t *testing.T) {
	z := New()
	api := z.Group("/api")

	api.POST("/users", func(c *Context) error {
		return c.String(http.StatusCreated, "created")
	})

	srv := httptest.NewServer(z)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/users", "text/plain", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}
	if !strings.Contains(string(body), "created") {
		t.Fatalf("expected body to contain 'created', got %s", string(body))
	}

	// GET on the same pattern should 404.
	resp2, err := http.Get(srv.URL + "/api/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for wrong method, got %d", resp2.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// TestGroup_Middleware — group middleware applies to routes in the group.
// ---------------------------------------------------------------------------
func TestGroup_Middleware(t *testing.T) {
	z := New()
	api := z.Group("/api")

	// A middleware that sets a custom header.
	api.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			c.SetHeader("X-Group-MW", "applied")
			return next(c)
		}
	})

	api.GET("/mw", func(c *Context) error {
		return c.String(http.StatusOK, "ok")
	})

	srv := httptest.NewServer(z)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/mw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Group-MW") != "applied" {
		t.Fatalf("expected X-Group-MW header to be 'applied', got '%s'", resp.Header.Get("X-Group-MW"))
	}
}

// ---------------------------------------------------------------------------
// TestGroup_MiddlewareNotLeaking — group middleware must NOT affect routes
// registered outside the group (on the root Zest instance).
// ---------------------------------------------------------------------------
func TestGroup_MiddlewareNotLeaking(t *testing.T) {
	z := New()
	api := z.Group("/api")

	api.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			c.SetHeader("X-Group-MW", "applied")
			return next(c)
		}
	})

	// Route inside the group.
	api.GET("/leak", func(c *Context) error {
		return c.String(http.StatusOK, "group")
	})

	// Route outside the group, directly on z.
	z.GET("/no-group", func(c *Context) error {
		return c.String(http.StatusOK, "no-group")
	})

	srv := httptest.NewServer(z)
	defer srv.Close()

	// Group route should have the header.
	resp1, err := http.Get(srv.URL + "/api/leak")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.Header.Get("X-Group-MW") != "applied" {
		t.Fatalf("group route: expected X-Group-MW 'applied', got '%s'", resp1.Header.Get("X-Group-MW"))
	}

	// Non-group route should NOT have the header.
	resp2, err := http.Get(srv.URL + "/no-group")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.Header.Get("X-Group-MW") != "" {
		t.Fatalf("non-group route: expected no X-Group-MW, got '%s'", resp2.Header.Get("X-Group-MW"))
	}
}

// ---------------------------------------------------------------------------
// TestGroup_Nested — nested groups concatenate prefixes correctly.
// ---------------------------------------------------------------------------
func TestGroup_Nested(t *testing.T) {
	z := New()
	api := z.Group("/api")
	v1 := api.Group("/v1")
	v2 := api.Group("/v2")

	v1.GET("/users", func(c *Context) error {
		return c.String(http.StatusOK, "v1 users")
	})
	v2.GET("/users", func(c *Context) error {
		return c.String(http.StatusOK, "v2 users")
	})

	srv := httptest.NewServer(z)
	defer srv.Close()

	// v1
	resp1, err := http.Get(srv.URL + "/api/v1/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp1.Body.Close()
	body1, _ := io.ReadAll(resp1.Body)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("v1: expected 200, got %d", resp1.StatusCode)
	}
	if !strings.Contains(string(body1), "v1 users") {
		t.Fatalf("v1: expected 'v1 users', got '%s'", string(body1))
	}

	// v2
	resp2, err := http.Get(srv.URL + "/api/v2/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("v2: expected 200, got %d", resp2.StatusCode)
	}
	if !strings.Contains(string(body2), "v2 users") {
		t.Fatalf("v2: expected 'v2 users', got '%s'", string(body2))
	}

	// Nested route should not match without parent prefix.
	resp3, err := http.Get(srv.URL + "/v1/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing parent prefix, got %d", resp3.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// TestGroup_NestedMiddleware — middleware inheritance in nested groups.
// ---------------------------------------------------------------------------
func TestGroup_NestedMiddleware(t *testing.T) {
	z := New()

	var order []string

	// Root middleware.
	z.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			order = append(order, "root")
			return next(c)
		}
	})

	api := z.Group("/api", func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			order = append(order, "api")
			return next(c)
		}
	})

	v1 := api.Group("/v1", func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			order = append(order, "v1")
			return next(c)
		}
	})

	v1.GET("/mw", func(c *Context) error {
		order = append(order, "handler")
		return c.String(http.StatusOK, "ok")
	})

	srv := httptest.NewServer(z)
	defer srv.Close()

	// Reset order slice before each request (servers share state across
	// requests, but we only make one request so we just need to read it).
	resp, err := http.Get(srv.URL + "/api/v1/mw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Middleware order: root -> api -> v1 -> handler
	// (global middlewares wrap everything, then group middleswares)
	if len(order) != 4 {
		t.Fatalf("expected 4 entries in order slice, got %d: %v", len(order), order)
	}
	if order[0] != "root" {
		t.Fatalf("order[0]: expected 'root', got '%s'", order[0])
	}
	if order[1] != "api" {
		t.Fatalf("order[1]: expected 'api', got '%s'", order[1])
	}
	if order[2] != "v1" {
		t.Fatalf("order[2]: expected 'v1', got '%s'", order[2])
	}
	if order[3] != "handler" {
		t.Fatalf("order[3]: expected 'handler', got '%s'", order[3])
	}
}

// ---------------------------------------------------------------------------
// TestJoinPath — various input combinations for joinPath.
// ---------------------------------------------------------------------------
func TestJoinPath(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		pattern  string
		expected string
	}{
		// Both empty.
		{"both empty", "", "", ""},
		// One empty.
		{"prefix empty", "", "/foo", "/foo"},
		{"pattern empty", "/api", "", "/api"},
		// Neither with slashes.
		{"neither has slash", "api", "users", "api/users"},
		// Both with slashes.
		{"both have slashes", "/api/", "/users/", "/api/users/"},
		// Trailing slash on prefix only.
		{"prefix trailing slash", "/api/", "users", "/api/users"},
		// Leading slash on pattern only.
		{"pattern leading slash", "/api", "/users", "/api/users"},
		// Both have extra slashes.
		{"extra slashes both sides", "/api//", "///users", "/api/users"},
		// Deep nesting simulation.
		{"deep path", "/api/v1", "users/list", "/api/v1/users/list"},
		// Root prefix.
		{"root prefix", "/", "users", "/users"},
		// Single slash pattern.
		{"single slash pattern", "/api", "/", "/api/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinPath(tt.prefix, tt.pattern)
			if got != tt.expected {
				t.Fatalf("joinPath(%q, %q) = %q, want %q", tt.prefix, tt.pattern, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestGroup_RouteMiddleware — per-route middleware on group routes.
// ---------------------------------------------------------------------------
func TestGroup_RouteMiddleware(t *testing.T) {
	z := New()
	api := z.Group("/api")

	// Group-level middleware.
	api.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			c.SetHeader("X-Group", "yes")
			return next(c)
		}
	})

	// Route-level middleware (passed directly to GET).
	routeMW := func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			c.SetHeader("X-Route", "yes")
			return next(c)
		}
	}

	api.GET("/rmw", func(c *Context) error {
		return c.String(http.StatusOK, "route with middleware")
	}, routeMW)

	srv := httptest.NewServer(z)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/rmw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Group") != "yes" {
		t.Fatalf("expected X-Group 'yes', got '%s'", resp.Header.Get("X-Group"))
	}
	if resp.Header.Get("X-Route") != "yes" {
		t.Fatalf("expected X-Route 'yes', got '%s'", resp.Header.Get("X-Route"))
	}
}

// ---------------------------------------------------------------------------
// TestGroup_Static — static file serving within a group.
// ---------------------------------------------------------------------------
func TestGroup_Static(t *testing.T) {
	z := New()

	// Create a temporary dir by relying on the testdata convention is overkill
	// here — we register the project root (or a dedicated dir) for a quick
	// smoke test of the routing layer. Instead we simply verify that the
	// Static method registers routes correctly by checking against 404 on the
	// expected pattern. A full integration test requires a real directory.

	// Use the current directory as root and expect group_test.go to be
	// accessible (it exists).
	api := z.Group("/public")
	api.Static("/assets", ".") // serves files from current dir at /public/assets/

	srv := httptest.NewServer(z)
	defer srv.Close()

	// group_test.go should be served since "." is the project dir.
	resp, err := http.Get(srv.URL + "/public/assets/group_test.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for static file, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "package zest") {
		t.Fatal("expected response body to contain 'package zest' (the test file itself)")
	}
}

// ---------------------------------------------------------------------------
// TestGroup_OPTIONS — OPTIONS method on a group route.
// ---------------------------------------------------------------------------
func TestGroup_OPTIONS(t *testing.T) {
	z := New()
	api := z.Group("/api")

	api.OPTIONS("/cors", func(c *Context) error {
		c.SetHeader("Allow", "GET, POST, OPTIONS")
		return c.NoContent()
	})

	srv := httptest.NewServer(z)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodOptions, srv.URL+"/api/cors", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Allow") != "GET, POST, OPTIONS" {
		t.Fatalf("expected Allow header, got '%s'", resp.Header.Get("Allow"))
	}
}

// ---------------------------------------------------------------------------
// TestGroup_PUT_PATCH_DELETE — quick coverage for remaining HTTP methods.
// ---------------------------------------------------------------------------
func TestGroup_PUT_PATCH_DELETE(t *testing.T) {
	z := New()
	api := z.Group("/api")

	api.PUT("/item", func(c *Context) error {
		return c.String(http.StatusOK, "put")
	})
	api.PATCH("/item", func(c *Context) error {
		return c.String(http.StatusOK, "patch")
	})
	api.DELETE("/item", func(c *Context) error {
		return c.String(http.StatusNoContent, "")
	})

	srv := httptest.NewServer(z)
	defer srv.Close()

	doReq := func(method, url string) *http.Response {
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return resp
	}

	respPut := doReq(http.MethodPut, srv.URL+"/api/item")
	defer respPut.Body.Close()
	bodyPut, _ := io.ReadAll(respPut.Body)
	if respPut.StatusCode != http.StatusOK || !strings.Contains(string(bodyPut), "put") {
		t.Fatalf("PUT: expected 200 'put', got %d '%s'", respPut.StatusCode, string(bodyPut))
	}

	respPatch := doReq(http.MethodPatch, srv.URL+"/api/item")
	defer respPatch.Body.Close()
	bodyPatch, _ := io.ReadAll(respPatch.Body)
	if respPatch.StatusCode != http.StatusOK || !strings.Contains(string(bodyPatch), "patch") {
		t.Fatalf("PATCH: expected 200 'patch', got %d '%s'", respPatch.StatusCode, string(bodyPatch))
	}

	respDelete := doReq(http.MethodDelete, srv.URL+"/api/item")
	defer respDelete.Body.Close()
	if respDelete.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE: expected 204, got %d", respDelete.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// TestGroup_Use — appending middlewares to a group after creation.
// ---------------------------------------------------------------------------
func TestGroup_Use(t *testing.T) {
	z := New()
	api := z.Group("/api")

	// Register a route BEFORE adding middleware.
	// handle() captures g.middlewares at call time via append,
	// so this route will NOT see middleware added later.
	api.GET("/before", func(c *Context) error {
		return c.String(http.StatusOK, "before")
	})

	// Add middleware after the first route has been registered.
	api.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			c.SetHeader("X-Added", "later")
			return next(c)
		}
	})

	// Register a route AFTER adding middleware — it should get the header.
	api.GET("/after", func(c *Context) error {
		return c.String(http.StatusOK, "after")
	})

	srv := httptest.NewServer(z)
	defer srv.Close()

	// /api/before was registered when g.middlewares was empty; no header.
	resp1, err := http.Get(srv.URL + "/api/before")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.Header.Get("X-Added") != "" {
		t.Fatalf("/api/before: expected no X-Added, got '%s'", resp1.Header.Get("X-Added"))
	}

	// /api/after was registered after Use() mutating g.middlewares; has header.
	resp2, err := http.Get(srv.URL + "/api/after")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.Header.Get("X-Added") != "later" {
		t.Fatalf("/api/after: expected X-Added 'later', got '%s'", resp2.Header.Get("X-Added"))
	}
}
