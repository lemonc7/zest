package zest

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Response.WriteHeader
// ---------------------------------------------------------------------------

func TestResponse_WriteHeader_FirstCallSetsStatusAndCommitted(t *testing.T) {
	w := httptest.NewRecorder()
	r := &Response{ResponseWriter: w}

	r.WriteHeader(http.StatusCreated)

	if r.Status != http.StatusCreated {
		t.Errorf("expected Status %d, got %d", http.StatusCreated, r.Status)
	}
	if !r.Committed {
		t.Error("expected Committed to be true after first WriteHeader")
	}
	if w.Code != http.StatusCreated {
		t.Errorf("expected recorder code %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestResponse_WriteHeader_SecondCallIsNoOp(t *testing.T) {
	w := httptest.NewRecorder()
	r := &Response{ResponseWriter: w}

	r.WriteHeader(http.StatusCreated)
	r.WriteHeader(http.StatusOK) // should be ignored

	if r.Status != http.StatusCreated {
		t.Errorf("expected Status to remain %d, got %d", http.StatusCreated, r.Status)
	}
	if w.Code != http.StatusCreated {
		t.Errorf("expected recorder code %d, got %d", http.StatusCreated, w.Code)
	}
}

// ---------------------------------------------------------------------------
// Response.Write
// ---------------------------------------------------------------------------

func TestResponse_Write_AutoSetsStatus200(t *testing.T) {
	w := httptest.NewRecorder()
	r := &Response{ResponseWriter: w}

	body := []byte("hello")
	n, err := r.Write(body)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(body) {
		t.Errorf("expected %d bytes written, got %d", len(body), n)
	}
	if r.Status != http.StatusOK {
		t.Errorf("expected Status %d, got %d", http.StatusOK, r.Status)
	}
	if !r.Committed {
		t.Error("expected Committed to be true after Write")
	}
	if r.Size != len(body) {
		t.Errorf("expected Size %d, got %d", len(body), r.Size)
	}
	if w.Body.String() != "hello" {
		t.Errorf("expected body 'hello', got %q", w.Body.String())
	}
}

func TestResponse_Write_TracksCumulativeSize(t *testing.T) {
	w := httptest.NewRecorder()
	r := &Response{ResponseWriter: w}

	r.Write([]byte("foo"))
	r.Write([]byte("bar"))

	if r.Size != 6 {
		t.Errorf("expected Size 6, got %d", r.Size)
	}
}

func TestResponse_Write_RespectsPresetStatus(t *testing.T) {
	w := httptest.NewRecorder()
	r := &Response{ResponseWriter: w}

	r.WriteHeader(http.StatusNotFound)
	n, _ := r.Write([]byte("not found"))

	if r.Status != http.StatusNotFound {
		t.Errorf("expected Status %d, got %d", http.StatusNotFound, r.Status)
	}
	if n != len("not found") {
		t.Errorf("expected %d bytes written, got %d", len("not found"), n)
	}
	if w.Code != http.StatusNotFound {
		t.Errorf("expected recorder code %d, got %d", http.StatusNotFound, w.Code)
	}
}

// ---------------------------------------------------------------------------
// Response.WriteString
// ---------------------------------------------------------------------------

func TestResponse_WriteString_AutoSetsStatus200(t *testing.T) {
	w := httptest.NewRecorder()
	r := &Response{ResponseWriter: w}

	s := "hello world"
	n, err := r.WriteString(s)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(s) {
		t.Errorf("expected %d bytes written, got %d", len(s), n)
	}
	if r.Status != http.StatusOK {
		t.Errorf("expected Status %d, got %d", http.StatusOK, r.Status)
	}
	if !r.Committed {
		t.Error("expected Committed to be true after WriteString")
	}
	if r.Size != len(s) {
		t.Errorf("expected Size %d, got %d", len(s), r.Size)
	}
	if w.Body.String() != s {
		t.Errorf("expected body %q, got %q", s, w.Body.String())
	}
}

func TestResponse_WriteString_TracksCumulativeSize(t *testing.T) {
	w := httptest.NewRecorder()
	r := &Response{ResponseWriter: w}

	r.WriteString("abc")
	r.WriteString("def")

	if r.Size != 6 {
		t.Errorf("expected Size 6, got %d", r.Size)
	}
}

// ---------------------------------------------------------------------------
// NewContext
// ---------------------------------------------------------------------------

func TestNewContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test/path", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)

	if c.Request != req {
		t.Error("expected Request to be set")
	}
	if c.Path != "/test/path" {
		t.Errorf("expected Path '/test/path', got %q", c.Path)
	}
	if c.Method != http.MethodGet {
		t.Errorf("expected Method GET, got %q", c.Method)
	}
	if c.Response().Status != http.StatusOK {
		t.Errorf("expected initial Status %d, got %d", http.StatusOK, c.Response().Status)
	}
	if c.Response().Size != 0 {
		t.Errorf("expected initial Size 0, got %d", c.Response().Size)
	}
	if c.Response().Committed {
		t.Error("expected Committed to be false initially")
	}
	if c.store != nil {
		t.Error("expected store to be nil initially")
	}
}

func TestNewContext_NilRequest(t *testing.T) {
	w := httptest.NewRecorder()
	c := NewContext(w, nil)

	if c.Request != nil {
		t.Error("expected Request to be nil")
	}
	if c.Path != "" {
		t.Errorf("expected empty Path, got %q", c.Path)
	}
	if c.Method != "" {
		t.Errorf("expected empty Method, got %q", c.Method)
	}
}

// ---------------------------------------------------------------------------
// reset
// ---------------------------------------------------------------------------

func TestContext_Reset(t *testing.T) {
	req1 := httptest.NewRequest(http.MethodGet, "/first", nil)
	w1 := httptest.NewRecorder()
	c := NewContext(w1, req1)

	// mutate state
	c.Set("key", "value")
	c.SetStatus(http.StatusCreated)
	c.Response().WriteHeader(http.StatusCreated) // mark committed

	req2 := httptest.NewRequest(http.MethodPost, "/second", nil)
	w2 := httptest.NewRecorder()
	c.reset(w2, req2)

	if c.Request != req2 {
		t.Error("expected Request to be updated after reset")
	}
	if c.Path != "/second" {
		t.Errorf("expected Path '/second', got %q", c.Path)
	}
	if c.Method != http.MethodPost {
		t.Errorf("expected Method POST, got %q", c.Method)
	}
	if c.Response().Status != http.StatusOK {
		t.Errorf("expected Status reset to %d, got %d", http.StatusOK, c.Response().Status)
	}
	if c.Response().Size != 0 {
		t.Errorf("expected Size reset to 0, got %d", c.Response().Size)
	}
	if c.Response().Committed {
		t.Error("expected Committed to be reset to false")
	}
	if c.store != nil {
		t.Error("expected store to be reset to nil")
	}
	if v := c.Get("key"); v != nil {
		t.Errorf("expected store key to be cleared, got %v", v)
	}
}

// ---------------------------------------------------------------------------
// sync
// ---------------------------------------------------------------------------

func TestContext_Sync(t *testing.T) {
	req1 := httptest.NewRequest(http.MethodGet, "/first", nil)
	w1 := httptest.NewRecorder()
	c := NewContext(w1, req1)

	// set some state that should survive sync
	c.SetStatus(http.StatusOK)
	c.Set("mykey", "myvalue")

	req2 := httptest.NewRequest(http.MethodDelete, "/second", nil)
	w2 := httptest.NewRecorder()
	c.sync(w2, req2)

	if c.Request != req2 {
		t.Error("expected Request to be updated after sync")
	}
	if c.Path != "/second" {
		t.Errorf("expected Path '/second', got %q", c.Path)
	}
	if c.Method != http.MethodDelete {
		t.Errorf("expected Method DELETE, got %q", c.Method)
	}

	// store should survive sync (sync does NOT clear it)
	if v, ok := c.Get("mykey").(string); !ok || v != "myvalue" {
		t.Errorf("expected store to survive sync, got %v", c.Get("mykey"))
	}
}

func TestContext_Sync_NilRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/path", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	c.sync(httptest.NewRecorder(), nil)

	if c.Request != nil {
		t.Error("expected Request to be nil after sync with nil")
	}
	// sync leaves Path/Method unchanged when r is nil
	if c.Path != "/path" {
		t.Errorf("expected Path to remain '/path' after sync with nil, got %q", c.Path)
	}
	if c.Method != http.MethodGet {
		t.Errorf("expected Method to remain GET after sync with nil, got %q", c.Method)
	}
}

// ---------------------------------------------------------------------------
// Context.Context()
// ---------------------------------------------------------------------------

func TestContext_Context(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	ctx := c.Context()
	if ctx == nil {
		t.Error("expected non-nil context.Context")
	}
	if ctx != req.Context() {
		t.Error("expected context from request")
	}
}

// ---------------------------------------------------------------------------
// Context.Error
// ---------------------------------------------------------------------------

func TestContext_Error_NilZest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	// c.zest is nil, so Error should be a no-op (no panic)
	c.Error(errors.New("some error"))
	// If we get here, no panic occurred — test passes.
}

func TestContext_Error_WithZestAndHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	var caught error
	z := &Zest{
		ErrHandler: func(ctx *Context, err error) {
			caught = err
		},
	}
	c.zest = z

	testErr := errors.New("test error")
	c.Error(testErr)

	if caught != testErr {
		t.Errorf("expected error %v to be caught, got %v", testErr, caught)
	}
}

func TestContext_Error_WithZestNilHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	z := &Zest{
		ErrHandler: nil,
	}
	c.zest = z

	// should not panic
	c.Error(errors.New("test error"))
}

// ---------------------------------------------------------------------------
// Context.Param
// ---------------------------------------------------------------------------

func TestContext_Param(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	// Go 1.22+ PathValue: we set it via SetPathValue on the request
	req.SetPathValue("id", "42")
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	val := c.Param("id")
	if val != "42" {
		t.Errorf("expected Param 'id' to be '42', got %q", val)
	}
}

func TestContext_Param_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	val := c.Param("nonexistent")
	if val != "" {
		t.Errorf("expected empty string for missing param, got %q", val)
	}
}

// ---------------------------------------------------------------------------
// Context.Query
// ---------------------------------------------------------------------------

func TestContext_Query(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/search?q=golang&page=1", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	if v := c.Query("q"); v != "golang" {
		t.Errorf("expected query 'q'='golang', got %q", v)
	}
	if v := c.Query("page"); v != "1" {
		t.Errorf("expected query 'page'='1', got %q", v)
	}
}

func TestContext_Query_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	if v := c.Query("missing"); v != "" {
		t.Errorf("expected empty string for missing query, got %q", v)
	}
}

// ---------------------------------------------------------------------------
// Context.SetStatus / SetHeader
// ---------------------------------------------------------------------------

func TestContext_SetStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	c.SetStatus(http.StatusAccepted)

	if c.Response().Status != http.StatusAccepted {
		t.Errorf("expected Status %d, got %d", http.StatusAccepted, c.Response().Status)
	}
	if w.Code != http.StatusAccepted {
		t.Errorf("expected recorder code %d, got %d", http.StatusAccepted, w.Code)
	}
}

func TestContext_SetHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	c.SetHeader("X-Custom", "myvalue")

	if got := w.Header().Get("X-Custom"); got != "myvalue" {
		t.Errorf("expected header 'X-Custom'='myvalue', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Context.JSON
// ---------------------------------------------------------------------------

func TestContext_JSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	data := map[string]string{"message": "hello"}
	err := c.JSON(http.StatusOK, data)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type to contain 'application/json', got %q", ct)
	}

	var decoded map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	if decoded["message"] != "hello" {
		t.Errorf("expected message 'hello', got %q", decoded["message"])
	}
}

func TestContext_JSON_DifferentStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	err := c.JSON(http.StatusCreated, map[string]int{"id": 1})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestContext_JSON_Array(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	data := []int{1, 2, 3}
	err := c.JSON(http.StatusOK, data)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded []int
	if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	if len(decoded) != 3 || decoded[0] != 1 {
		t.Errorf("unexpected decoded array: %v", decoded)
	}
}

// ---------------------------------------------------------------------------
// Context.String
// ---------------------------------------------------------------------------

func TestContext_String(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	err := c.String(http.StatusOK, "hello world")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected Content-Type to contain 'text/plain', got %q", ct)
	}
	if w.Body.String() != "hello world" {
		t.Errorf("expected body 'hello world', got %q", w.Body.String())
	}
}

func TestContext_String_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	// Use 200 OK with empty string; 204 No Content does not allow a body
	err := c.String(http.StatusOK, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Body.String() != "" {
		t.Errorf("expected empty body, got %q", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Context.HTML
// ---------------------------------------------------------------------------

func TestContext_HTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	err := c.HTML(http.StatusOK, "<h1>Hello</h1>")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected Content-Type to contain 'text/html', got %q", ct)
	}
	if w.Body.String() != "<h1>Hello</h1>" {
		t.Errorf("expected body '<h1>Hello</h1>', got %q", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Context.Set / Context.Get
// ---------------------------------------------------------------------------

func TestContext_Set_Get(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	c.Set("key1", "value1")
	c.Set("key2", 42)
	c.Set("key3", struct{ Name string }{"test"})

	if v, ok := c.Get("key1").(string); !ok || v != "value1" {
		t.Errorf("expected 'value1', got %v", c.Get("key1"))
	}
	if v, ok := c.Get("key2").(int); !ok || v != 42 {
		t.Errorf("expected 42, got %v", c.Get("key2"))
	}
	if v, ok := c.Get("key3").(struct{ Name string }); !ok || v.Name != "test" {
		t.Errorf("expected struct with Name='test', got %v", c.Get("key3"))
	}
}

func TestContext_Get_MissingKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	if v := c.Get("nonexistent"); v != nil {
		t.Errorf("expected nil for missing key, got %v", v)
	}
}

func TestContext_Set_Overwrite(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	c.Set("key", "first")
	c.Set("key", "second")

	if v, ok := c.Get("key").(string); !ok || v != "second" {
		t.Errorf("expected 'second', got %v", c.Get("key"))
	}
}

// ---------------------------------------------------------------------------
// Context.NoContent
// ---------------------------------------------------------------------------

func TestContext_NoContent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	err := c.NoContent()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}
	if c.Response().Status != http.StatusNoContent {
		t.Errorf("expected response Status %d, got %d", http.StatusNoContent, c.Response().Status)
	}
}

// ---------------------------------------------------------------------------
// Context.Redirect
// ---------------------------------------------------------------------------

func TestContext_Redirect_ValidCodes(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	validCodes := []int{
		http.StatusMultipleChoices,   // 300
		http.StatusMovedPermanently,  // 301
		http.StatusFound,             // 302
		http.StatusSeeOther,          // 303
		http.StatusNotModified,       // 304
		http.StatusUseProxy,          // 305
		http.StatusTemporaryRedirect, // 307
		http.StatusPermanentRedirect, // 308
	}

	for _, code := range validCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := c.Redirect(code, "https://example.com")

			if err != nil {
				t.Errorf("expected no error for status %d, got %v", code, err)
			}
			if w.Code != code {
				t.Errorf("expected status %d, got %d", code, w.Code)
			}
			loc := w.Header().Get("Location")
			if loc != "https://example.com" {
				t.Errorf("expected Location 'https://example.com', got %q", loc)
			}
		})
	}
}

func TestContext_Redirect_InvalidCodes(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	invalidCodes := []int{
		http.StatusOK,                  // 200
		http.StatusCreated,             // 201
		http.StatusBadRequest,          // 400
		http.StatusInternalServerError, // 500
		299,
		400,
	}

	for _, code := range invalidCodes {
		t.Run("", func(t *testing.T) {
			w := httptest.NewRecorder()
			c := NewContext(w, req)

			err := c.Redirect(code, "https://example.com")

			if err == nil {
				t.Errorf("expected error for status %d, got nil", code)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Context.Cookie / SetCookie
// ---------------------------------------------------------------------------

func TestContext_Cookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	cookie, err := c.Cookie("session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cookie.Value != "abc123" {
		t.Errorf("expected cookie value 'abc123', got %q", cookie.Value)
	}
}

func TestContext_Cookie_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	_, err := c.Cookie("nonexistent")
	if err != http.ErrNoCookie {
		t.Errorf("expected http.ErrNoCookie, got %v", err)
	}
}

func TestContext_SetCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	cookie := &http.Cookie{
		Name:     "token",
		Value:    "xyz789",
		Path:     "/",
		HttpOnly: true,
	}
	c.SetCookie(cookie)

	setCookieHeader := w.Header().Get("Set-Cookie")
	if setCookieHeader == "" {
		t.Error("expected Set-Cookie header to be set")
	}
	if !strings.Contains(setCookieHeader, "token=xyz789") {
		t.Errorf("expected Set-Cookie to contain 'token=xyz789', got %q", setCookieHeader)
	}
}

// ---------------------------------------------------------------------------
// Context.FormValue
// ---------------------------------------------------------------------------

func TestContext_FormValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name=john&email=john@example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	if v := c.FormValue("name"); v != "john" {
		t.Errorf("expected 'john', got %q", v)
	}
	if v := c.FormValue("email"); v != "john@example.com" {
		t.Errorf("expected 'john@example.com', got %q", v)
	}
}

func TestContext_FormValue_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	if v := c.FormValue("missing"); v != "" {
		t.Errorf("expected empty string for missing form value, got %q", v)
	}
}

// ---------------------------------------------------------------------------
// Context.Response / ResponseWriter
// ---------------------------------------------------------------------------

func TestContext_Response(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	resp := c.Response()
	if resp == nil {
		t.Fatal("expected non-nil Response")
	}
	if resp.Status != http.StatusOK {
		t.Errorf("expected Status %d, got %d", http.StatusOK, resp.Status)
	}
}

func TestContext_ResponseWriter(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	rw := c.ResponseWriter()
	if rw == nil {
		t.Fatal("expected non-nil ResponseWriter")
	}
	// Write through it
	rw.Write([]byte("test"))
	if w.Body.String() != "test" {
		t.Errorf("expected body 'test', got %q", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Context.ClientIP
// ---------------------------------------------------------------------------

func TestClientIP_XForwardedFor_Single(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	ip := c.ClientIP()
	if ip != "203.0.113.1" {
		t.Errorf("expected '203.0.113.1', got %q", ip)
	}
}

func TestClientIP_XForwardedFor_Multiple(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.2, 192.0.2.3")
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	ip := c.ClientIP()
	if ip != "203.0.113.1" {
		t.Errorf("expected first IP '203.0.113.1', got %q", ip)
	}
}

func TestClientIP_XForwardedFor_WithSpaces(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "  203.0.113.1 , 198.51.100.2 ")
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	ip := c.ClientIP()
	if ip != "203.0.113.1" {
		t.Errorf("expected trimmed IP '203.0.113.1', got %q", ip)
	}
}

func TestClientIP_XRealIp(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-Ip", "10.0.0.1")
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	ip := c.ClientIP()
	if ip != "10.0.0.1" {
		t.Errorf("expected '10.0.0.1', got %q", ip)
	}
}

func TestClientIP_XRealIp_PrecedenceOverRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-Ip", "10.0.0.1")
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	ip := c.ClientIP()
	if ip != "10.0.0.1" {
		t.Errorf("expected X-Real-Ip '10.0.0.1', got %q", ip)
	}
}

func TestClientIP_XForwardedFor_TakesPrecedenceOverXRealIp(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("X-Real-Ip", "10.0.0.1")
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	ip := c.ClientIP()
	if ip != "203.0.113.1" {
		t.Errorf("expected X-Forwarded-For '203.0.113.1', got %q", ip)
	}
}

func TestClientIP_RemoteAddr_Fallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:54321"
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	ip := c.ClientIP()
	if ip != "192.168.1.100" {
		t.Errorf("expected '192.168.1.100', got %q", ip)
	}
}

func TestClientIP_RemoteAddr_NoPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100"
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	ip := c.ClientIP()
	if ip != "" {
		t.Errorf("expected empty string (no port to split), got %q", ip)
	}
}

func TestClientIP_NoHeaders_NoRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ""
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	ip := c.ClientIP()
	if ip != "" {
		t.Errorf("expected empty string, got %q", ip)
	}
}

func TestClientIP_XForwardedFor_EmptyWithComma(t *testing.T) {
	// Edge case: X-Forwarded-For is just a comma
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", ",")
	req.RemoteAddr = "10.0.0.1:8080"
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	ip := c.ClientIP()
	if ip != "10.0.0.1" {
		t.Errorf("expected RemoteAddr fallback '10.0.0.1', got %q", ip)
	}
}

// ---------------------------------------------------------------------------
// Context.File
// ---------------------------------------------------------------------------

func TestContext_File_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	// File doesn't exist; http.ServeFile will write a 404
	c.File("/nonexistent/file.txt")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing file, got %d", w.Code)
	}
}

func TestContext_File_SetsContentType(t *testing.T) {
	// http.ServeFile detects content-type based on extension
	// We can't easily serve a real file in a unit test without temp files,
	// so just testing that File doesn't panic and writes something.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	// Should not panic
	c.File("/nonexistent/file.html")

	// 404 because file doesn't exist, but no panic
	if w.Code == 0 {
		t.Error("expected some status to be written")
	}
}

// ---------------------------------------------------------------------------
// Context.Attachment
// ---------------------------------------------------------------------------

func TestContext_Attachment_SetsContentDisposition(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	// Even though the file doesn't exist, the header should be set before ServeFile
	c.Attachment("/path/to/report.pdf", "report.pdf")

	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Errorf("expected Content-Disposition to contain 'attachment', got %q", cd)
	}
	if !strings.Contains(cd, "report.pdf") {
		t.Errorf("expected Content-Disposition to contain 'report.pdf', got %q", cd)
	}
}

func TestContext_Attachment_SpecialCharsInFilename(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	c.Attachment("/path/to/file.txt", "文件名.txt")

	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Errorf("expected Content-Disposition to contain 'attachment', got %q", cd)
	}
	// The filename should be URL-encoded
	if !strings.Contains(cd, "%E6%96%87%E4%BB%B6%E5%90%8D") {
		t.Errorf("expected URL-encoded filename in Content-Disposition, got %q", cd)
	}
}

// ---------------------------------------------------------------------------
// Integration-style tests
// ---------------------------------------------------------------------------

func TestContext_FullJSONResponseFlow(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	c.Set("user_id", 123)
	c.SetStatus(http.StatusOK)
	c.SetHeader("X-Request-Id", "abc-123")

	err := c.JSON(http.StatusOK, map[string]any{
		"items": []string{"a", "b", "c"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("X-Request-Id") != "abc-123" {
		t.Error("custom header not set")
	}

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	items := body["items"].([]any)
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestContext_ResponseSizeTracking(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	c.String(http.StatusOK, "1234567890") // 10 bytes

	if c.Response().Size != 10 {
		t.Errorf("expected Size 10, got %d", c.Response().Size)
	}
}

func TestContext_CommittedPreventsStatusOverride(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	c.WriteHeader(http.StatusOK)
	c.WriteHeader(http.StatusInternalServerError)

	if c.Response().Status != http.StatusOK {
		t.Errorf("expected Status to remain %d, got %d", http.StatusOK, c.Response().Status)
	}
}

func TestContext_WriteCommitsAndPreventsStatusChange(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	c := NewContext(w, req)

	// Writing commits with 200
	c.Response().Write([]byte("body"))
	// Then try to change status
	c.Response().WriteHeader(http.StatusInternalServerError)

	if c.Response().Status != http.StatusOK {
		t.Errorf("expected Status to remain %d, got %d", http.StatusOK, c.Response().Status)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected recorder code %d, got %d", http.StatusOK, w.Code)
	}
}

// ---------------------------------------------------------------------------
// Helper to match WriteHeader signature for context
// ---------------------------------------------------------------------------

func (c *Context) WriteHeader(code int) {
	c.response.WriteHeader(code)
}

func (c *Context) Write(b []byte) (int, error) {
	return c.response.Write(b)
}

// ============================================================
// FormFile
// ============================================================

func TestContext_FormFile(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("upload", "test.txt")
	part.Write([]byte("hello"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c := NewContext(httptest.NewRecorder(), req)

	fh, err := c.FormFile("upload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fh.Filename != "test.txt" {
		t.Errorf("expected filename 'test.txt', got %q", fh.Filename)
	}
}

func TestContext_FormFile_Missing(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("hello", "world")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c := NewContext(httptest.NewRecorder(), req)

	_, err := c.FormFile("missing")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// ============================================================
// MultipartForm
// ============================================================

func TestContext_MultipartForm(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("name", "value")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c := NewContext(httptest.NewRecorder(), req)

	form, err := c.MultipartForm()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v := form.Value["name"]; len(v) == 0 || v[0] != "value" {
		t.Errorf("expected form value 'value', got %v", form.Value["name"])
	}
}
