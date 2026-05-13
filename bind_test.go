package zest

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// --- Test helper types ---

type pathBind struct {
	ID int `param:"id"`
}

type queryBind struct {
	Name string `query:"name"`
}

type jsonBind struct {
	Name string `json:"name"`
}

type formBind struct {
	Name string `form:"name"`
}

type intBind struct {
	Value int `query:"value"`
}

type floatBind struct {
	Value float64 `query:"value"`
}

type boolBind struct {
	Flag bool `query:"flag"`
}

type caseBind struct {
	Name string `query:"Name"`
}

// Named map type for map[string]string destinations with Bind.
type MapValidator map[string]string

type combinedBind struct {
	ID   int    `param:"id"`
	Name string `query:"name"`
}

type postBodyBind struct {
	Name string `json:"name"`
}

// --- Helper to create a Context with an httptest recorder ---

func newTestContext(req *http.Request) *Context {
	w := httptest.NewRecorder()
	return NewContext(w, req)
}

// ============================================================
// 1. TestBindPathValues
// ============================================================

func TestBindPathValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	req.SetPathValue("id", "42")
	req.Pattern = "GET /users/{id}"

	c := newTestContext(req)
	dst := &pathBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.ID != 42 {
		t.Errorf("expected ID=42, got %d", dst.ID)
	}
}

// ============================================================
// 2. TestBindQueryParams
// ============================================================

func TestBindQueryParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?name=test", nil)

	c := newTestContext(req)
	dst := &queryBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "test" {
		t.Errorf("expected Name='test', got %q", dst.Name)
	}
}

// ============================================================
// 3. TestBindJSON
// ============================================================

func TestBindJSON(t *testing.T) {
	body := strings.NewReader(`{"name":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set(HeaderContentType, MIMEApplicationJSON)

	c := newTestContext(req)
	dst := &jsonBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "test" {
		t.Errorf("expected Name='test', got %q", dst.Name)
	}
}

// ============================================================
// 4. TestBindForm
// ============================================================

func TestBindForm(t *testing.T) {
	formData := url.Values{}
	formData.Set("name", "formvalue")
	body := strings.NewReader(formData.Encode())

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set(HeaderContentType, MIMEApplicationForm)

	c := newTestContext(req)
	dst := &formBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "formvalue" {
		t.Errorf("expected Name='formvalue', got %q", dst.Name)
	}
}

// ============================================================
// 5. TestGetPathParamNames
// ============================================================

func TestGetPathParamNames(t *testing.T) {
	tests := []struct {
		pattern  string
		expected []string
	}{
		{"GET /users/{id}", []string{"id"}},
		{"POST /users/{id}/posts/{postID}", []string{"id", "postID"}},
		{"GET /files/{path...}", []string{"path"}},
		{"GET /static", nil},
		{"GET /{a}/{b}/{c...}", []string{"a", "b", "c"}},
		{"", nil},
		{"GET /users/{id}/{name...}/detail", []string{"id", "name"}},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := getPathParamNames(tt.pattern)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}

// ============================================================
// 7. TestBindInt
// ============================================================

func TestBindInt(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?value=123", nil)

	c := newTestContext(req)
	dst := &intBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Value != 123 {
		t.Errorf("expected Value=123, got %d", dst.Value)
	}
}

// ============================================================
// 8. TestBindFloat
// ============================================================

func TestBindFloat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?value=3.14", nil)

	c := newTestContext(req)
	dst := &floatBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Value != 3.14 {
		t.Errorf("expected Value=3.14, got %f", dst.Value)
	}
}

// ============================================================
// 9. TestBindBool
// ============================================================

func TestBindBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?flag="+tt.input, nil)

			c := newTestContext(req)
			dst := &boolBind{}

			err := c.Bind(dst)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dst.Flag != tt.expected {
				t.Errorf("expected Flag=%v, got %v", tt.expected, dst.Flag)
			}
		})
	}
}

// ============================================================
// 10. TestBindMap
// ============================================================

func TestBindMap(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?key1=val1&key2=val2", nil)

	c := newTestContext(req)
	dst := &MapValidator{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := *dst
	if m["key1"] != "val1" {
		t.Errorf("expected key1='val1', got %q", m["key1"])
	}
	if m["key2"] != "val2" {
		t.Errorf("expected key2='val2', got %q", m["key2"])
	}
}

// ============================================================
// 11. TestBindCaseInsensitive
// ============================================================

func TestBindCaseInsensitive(t *testing.T) {
	// Query param key is lowercase "name", struct tag is "Name"
	req := httptest.NewRequest(http.MethodGet, "/?name=caseTest", nil)

	c := newTestContext(req)
	dst := &caseBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "caseTest" {
		t.Errorf("expected Name='caseTest', got %q", dst.Name)
	}
}

// ============================================================
// 12. TestBindUnsupportedMediaType
// ============================================================

func TestBindUnsupportedMediaType(t *testing.T) {
	body := strings.NewReader("some plain text")
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set(HeaderContentType, "text/plain")

	c := newTestContext(req)
	dst := &jsonBind{}

	err := c.Bind(dst)
	if err == nil {
		t.Fatal("expected error for unsupported media type, got nil")
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected *HTTPError, got %T: %v", err, err)
	}
	if httpErr.Code != http.StatusUnsupportedMediaType {
		t.Errorf("expected status %d, got %d", http.StatusUnsupportedMediaType, httpErr.Code)
	}
}

// ============================================================
// Additional edge case tests
// ============================================================

// TestBindPathAndQueryCombined verifies both path values and query params
// are bound together in a single Bind call.
func TestBindPathAndQueryCombined(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users/99?name=combo", nil)
	req.SetPathValue("id", "99")
	req.Pattern = "GET /users/{id}"

	c := newTestContext(req)
	dst := &combinedBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.ID != 99 {
		t.Errorf("expected ID=99, got %d", dst.ID)
	}
	if dst.Name != "combo" {
		t.Errorf("expected Name='combo', got %q", dst.Name)
	}
}

// TestBindNoQueryParamsForPost verifies that query params are NOT bound
// for POST requests (only GET, DELETE, HEAD).
func TestBindNoQueryParamsForPost(t *testing.T) {
	body := strings.NewReader(`{"name":"bodyval"}`)
	req := httptest.NewRequest(http.MethodPost, "/?name=queryval", body)
	req.Header.Set(HeaderContentType, MIMEApplicationJSON)

	c := newTestContext(req)
	dst := &postBodyBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "bodyval" {
		t.Errorf("expected Name='bodyval' (from JSON body), got %q", dst.Name)
	}
}

// TestBindQueryParamsForDelete verifies query params ARE bound for DELETE.
func TestBindQueryParamsForDelete(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/?name=deleteme", nil)

	c := newTestContext(req)
	dst := &queryBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "deleteme" {
		t.Errorf("expected Name='deleteme', got %q", dst.Name)
	}
}

// TestBindQueryParamsForHead verifies query params ARE bound for HEAD.
func TestBindQueryParamsForHead(t *testing.T) {
	req := httptest.NewRequest(http.MethodHead, "/?name=headtest", nil)

	c := newTestContext(req)
	dst := &queryBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "headtest" {
		t.Errorf("expected Name='headtest', got %q", dst.Name)
	}
}

// ============================================================
// XML Body Binding
// ============================================================

type xmlBind struct {
	Name string `xml:"name"`
}

func TestBindXML(t *testing.T) {
	xmlBody := `<xmlBind><name>xmlvalue</name></xmlBind>`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(xmlBody))
	req.Header.Set("Content-Type", "application/xml")

	c := newTestContext(req)
	dst := &xmlBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "xmlvalue" {
		t.Errorf("expected Name='xmlvalue', got %q", dst.Name)
	}
}

func TestBindXML_TextXML(t *testing.T) {
	xmlBody := `<xmlBind><name>textxml</name></xmlBind>`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(xmlBody))
	req.Header.Set("Content-Type", "text/xml")

	c := newTestContext(req)
	dst := &xmlBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "textxml" {
		t.Errorf("expected Name='textxml', got %q", dst.Name)
	}
}

// ============================================================
// Multipart Form Binding
// ============================================================

func TestBindMultipartForm(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("name", "multivalue")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c := newTestContext(req)
	dst := &formBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "multivalue" {
		t.Errorf("expected Name='multivalue', got %q", dst.Name)
	}
}

// ============================================================
// Multipart File Upload Binding
// ============================================================

type fileBind struct {
	Avatar *multipart.FileHeader `form:"avatar"`
}

func TestBindMultipartFile(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("avatar", "test.png")
	part.Write([]byte("fake-png-data"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c := newTestContext(req)
	dst := &fileBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Avatar == nil {
		t.Fatal("expected Avatar to be non-nil")
	}
	if dst.Avatar.Filename != "test.png" {
		t.Errorf("expected filename 'test.png', got %q", dst.Avatar.Filename)
	}
}

type filesBind struct {
	Files []*multipart.FileHeader `form:"files"`
}

func TestBindMultipartFiles(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for _, name := range []string{"a.txt", "b.txt"} {
		part, _ := writer.CreateFormFile("files", name)
		part.Write([]byte("content"))
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c := newTestContext(req)
	dst := &filesBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dst.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(dst.Files))
	}
	if dst.Files[0].Filename != "a.txt" {
		t.Errorf("expected first file 'a.txt', got %q", dst.Files[0].Filename)
	}
	if dst.Files[1].Filename != "b.txt" {
		t.Errorf("expected second file 'b.txt', got %q", dst.Files[1].Filename)
	}
}

// ============================================================
// Uint Types
// ============================================================

type uintBind struct {
	Count uint `query:"count"`
}

func TestBindUint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?count=42", nil)

	c := newTestContext(req)
	dst := &uintBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Count != 42 {
		t.Errorf("expected Count=42, got %d", dst.Count)
	}
}

type uint8Bind struct {
	Val uint8 `query:"val"`
}

func TestBindUint8(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?val=255", nil)

	c := newTestContext(req)
	dst := &uint8Bind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Val != 255 {
		t.Errorf("expected Val=255, got %d", dst.Val)
	}
}

// ============================================================
// formParams multipart branch
// ============================================================

type formOnlyBind struct {
	Field string `form:"field"`
}

func TestBindFormMultipartContentType(t *testing.T) {
	// simulate multipart/form-data content-type to hit the formParams multipart branch
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("field", "multipart_form_value")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c := newTestContext(req)
	dst := &formOnlyBind{}

	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Field != "multipart_form_value" {
		t.Errorf("expected Field='multipart_form_value', got %q", dst.Field)
	}
}

// ============================================================
// StructValidator integration tests
// ============================================================

type mockStructValidator struct {
	err error
}

func (m *mockStructValidator) ValidateStruct(v any) error {
	return m.err
}

// structWithValidateTag is a bind target with struct tags for validation.
type structWithValidateTag struct {
	Name string `query:"name"`
}

func TestBind_StructValidatorOnly(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?name=test", nil)
	c := newTestContext(req)
	c.zest = &Zest{StructValidator: &mockStructValidator{}}

	dst := &structWithValidateTag{}
	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "test" {
		t.Errorf("expected Name='test', got %q", dst.Name)
	}
}

func TestBind_StructValidatorReturnsError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?name=test", nil)
	c := newTestContext(req)
	c.zest = &Zest{StructValidator: &mockStructValidator{err: errors.New("tag validation failed")}}

	dst := &structWithValidateTag{}
	err := c.Bind(dst)
	if err == nil {
		t.Fatal("expected error from StructValidator, got nil")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected *HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", httpErr.Code)
	}
}

func TestBind_NoValidatorNoError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?name=test", nil)
	c := newTestContext(req)

	dst := &structWithValidateTag{}
	err := c.Bind(dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "test" {
		t.Errorf("expected Name='test', got %q", dst.Name)
	}
}

func TestBind_NilZestDoesNotPanic(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?name=test", nil)
	c := newTestContext(req)
	c.zest = nil

	dst := &structWithValidateTag{}
	if err := c.Bind(dst); err != nil {
		t.Fatalf("unexpected error with nil zest: %v", err)
	}
}
