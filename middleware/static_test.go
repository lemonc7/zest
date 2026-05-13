package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lemonc7/zest"
)

// newStaticContext 创建一个用于静态文件测试的 context
func newStaticContext(method, path string) (*zest.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	c := zest.NewContext(rec, req)
	return c, rec
}

// setupTempDir 创建临时目录并写入测试文件，返回目录路径和清理函数
func setupTempDir(t *testing.T, files map[string]string) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "zest-static-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	for name, content := range files {
		fullPath := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			os.RemoveAll(dir)
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			os.RemoveAll(dir)
			t.Fatalf("failed to write file: %v", err)
		}
	}

	return dir, func() { os.RemoveAll(dir) }
}

// ============================================================
// 基本文件服务
// ============================================================

func TestStaticServeFile(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{
		"hello.txt": "Hello, World!",
	})
	defer cleanup()

	cfg := StaticConfig{Root: dir}
	mw := Static(cfg)

	c, rec := newStaticContext(http.MethodGet, "/hello.txt")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Hello, World!") {
		t.Errorf("expected body to contain 'Hello, World!', got %q", rec.Body.String())
	}
}

func TestStaticServeFileHEAD(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{
		"hello.txt": "Hello, World!",
	})
	defer cleanup()

	cfg := StaticConfig{Root: dir}
	mw := Static(cfg)

	c, rec := newStaticContext(http.MethodHead, "/hello.txt")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for HEAD, got %d", rec.Code)
	}
	// HEAD 不应该有 body
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body for HEAD, got %d bytes", rec.Body.Len())
	}
}

func TestStaticFileNotFound(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{})
	defer cleanup()

	cfg := StaticConfig{Root: dir}
	mw := Static(cfg)

	c, _ := newStaticContext(http.MethodGet, "/nonexistent.txt")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}

	he, ok := err.(*zest.HTTPError)
	if !ok {
		t.Fatalf("expected *zest.HTTPError, got %T", err)
	}
	if he.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", he.Code)
	}
}

// ============================================================
// 目录索引文件 (index.html)
// ============================================================

func TestStaticServeIndexHtml(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{
		"index.html": "<h1>Index</h1>",
	})
	defer cleanup()

	cfg := StaticConfig{Root: dir}
	mw := Static(cfg)

	c, rec := newStaticContext(http.MethodGet, "/")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<h1>Index</h1>") {
		t.Errorf("expected body to contain '<h1>Index</h1>', got %q", rec.Body.String())
	}
}

func TestStaticCustomIndex(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{
		"default.htm": "<h1>Custom Index</h1>",
	})
	defer cleanup()

	cfg := StaticConfig{
		Root:  dir,
		Index: "default.htm",
	}
	mw := Static(cfg)

	c, rec := newStaticContext(http.MethodGet, "/")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Custom Index") {
		t.Errorf("expected custom index content, got %q", rec.Body.String())
	}
}

// ============================================================
// HTML5 模式 (SPA)
// ============================================================

func TestStaticHTML5Mode(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{
		"index.html": "<h1>SPA</h1>",
	})
	defer cleanup()

	cfg := StaticConfig{
		Root:  dir,
		HTML5: true,
	}
	mw := Static(cfg)

	c, rec := newStaticContext(http.MethodGet, "/some/spa/route")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<h1>SPA</h1>") {
		t.Errorf("expected spa index content, got %q", rec.Body.String())
	}
}

func TestStaticHTML5Mode_NoIndexHtml(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{})
	defer cleanup()

	cfg := StaticConfig{
		Root:  dir,
		HTML5: true,
	}
	mw := Static(cfg)

	c, _ := newStaticContext(http.MethodGet, "/some/spa/route")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err == nil {
		t.Fatal("expected error when index.html missing in HTML5 mode")
	}
	he, ok := err.(*zest.HTTPError)
	if !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %v", err)
	}
}

// ============================================================
// 目录浏览
// ============================================================

func TestStaticBrowse(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{
		"a.txt": "aaa",
		"b.txt": "bbb",
	})
	defer cleanup()

	cfg := StaticConfig{
		Root:   dir,
		Browse: true,
	}
	mw := Static(cfg)

	c, rec := newStaticContext(http.MethodGet, "/")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "a.txt") {
		t.Errorf("expected directory listing to contain 'a.txt', got %q", body)
	}
	if !strings.Contains(body, "b.txt") {
		t.Errorf("expected directory listing to contain 'b.txt', got %q", body)
	}
	// 目录浏览应该包含 HTML 标签
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("expected HTML doctype in directory listing")
	}
}

func TestStaticBrowsePriority_IndexOverBrowse(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{
		"index.html": "<h1>Index</h1>",
		"other.txt":  "other",
	})
	defer cleanup()

	cfg := StaticConfig{
		Root:   dir,
		Browse: true,
	}
	mw := Static(cfg)

	c, rec := newStaticContext(http.MethodGet, "/")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 有 index.html 时，应返回 index.html，而不是目录列表
	if !strings.Contains(rec.Body.String(), "<h1>Index</h1>") {
		t.Errorf("expected index.html content, got %q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "other.txt") {
		t.Error("should NOT show directory listing when index.html exists")
	}
}

// ============================================================
// 跳过非 GET/HEAD 请求
// ============================================================

func TestStaticPassThroughNonGetHead(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{
		"hello.txt": "Hello",
	})
	defer cleanup()

	cfg := StaticConfig{Root: dir}
	mw := Static(cfg)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			c, _ := newStaticContext(method, "/hello.txt")
			nextCalled := false
			handler := mw(func(c *zest.Context) error {
				nextCalled = true
				return nil
			})

			err := handler(c)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !nextCalled {
				t.Errorf("expected next handler to be called for %s", method)
			}
		})
	}
}

// ============================================================
// 嵌套路径
// ============================================================

func TestStaticNestedFile(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{
		"sub/dir/deep.txt": "deep content",
	})
	defer cleanup()

	cfg := StaticConfig{Root: dir}
	mw := Static(cfg)

	c, rec := newStaticContext(http.MethodGet, "/sub/dir/deep.txt")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "deep content") {
		t.Errorf("expected 'deep content', got %q", rec.Body.String())
	}
}

// ============================================================
// URL 编码路径
// ============================================================

func TestStaticURLEncodedPath(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{
		"中文文件.txt": "中文内容",
	})
	defer cleanup()

	cfg := StaticConfig{Root: dir}
	mw := Static(cfg)

	c, rec := newStaticContext(http.MethodGet, "/%E4%B8%AD%E6%96%87%E6%96%87%E4%BB%B6.txt")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "中文内容") {
		t.Errorf("expected '中文内容', got %q", rec.Body.String())
	}
}

// ============================================================
// 自定义 FileSystem
// ============================================================

func TestStaticCustomFileSystem(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{
		"data.json": `{"key": "value"}`,
	})
	defer cleanup()

	cfg := StaticConfig{
		Filesystem: http.Dir(dir),
	}
	mw := Static(cfg)

	c, rec := newStaticContext(http.MethodGet, "/data.json")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"value"`) {
		t.Errorf("expected JSON content, got %q", rec.Body.String())
	}
}

// ============================================================
// 默认配置
// ============================================================

func TestStaticDefaultConfig(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{
		"index.html": "<h1>Default</h1>",
	})
	defer cleanup()

	// 只设置 Root，其余用默认值
	cfg := StaticConfig{Root: dir}
	mw := Static(cfg)

	c, rec := newStaticContext(http.MethodGet, "/")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 默认 Index 应为 "index.html"
	if !strings.Contains(rec.Body.String(), "<h1>Default</h1>") {
		t.Errorf("expected default index.html, got %q", rec.Body.String())
	}
}

func TestStaticEmptyRootDefaultsToDot(t *testing.T) {
	// 空 Root 默认为 "."，但为了测试隔离，我们直接构造一个自定义 Filesystem
	dir, cleanup := setupTempDir(t, map[string]string{
		"test.txt": "test content",
	})
	defer cleanup()

	cfg := StaticConfig{
		Filesystem: http.Dir(dir),
	}
	mw := Static(cfg)

	c, rec := newStaticContext(http.MethodGet, "/test.txt")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// ============================================================
// 目录请求无 index.html 且未开启 Browse
// ============================================================

func TestStaticDirectoryNoIndexNoBrowse(t *testing.T) {
	dir, cleanup := setupTempDir(t, map[string]string{})
	defer cleanup()
	// 创建空子目录（setupTempDir 只能创建文件，这里手动创建目录）
	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	cfg := StaticConfig{Root: dir}
	mw := Static(cfg)

	c, _ := newStaticContext(http.MethodGet, "/subdir/")
	handler := mw(func(c *zest.Context) error {
		return zest.NewHTTPError(http.StatusNotFound, "not found")
	})

	err := handler(c)
	if err == nil {
		t.Fatal("expected error for directory without index, got nil")
	}
	he, ok := err.(*zest.HTTPError)
	if !ok || he.Code != http.StatusNotFound {
		t.Errorf("expected 404 error, got %v", err)
	}
}

// ============================================================
// formatSize (used by listDir)
// ============================================================

func TestStaticFormatSize(t *testing.T) {
	tests := []struct {
		size     int
		expected string
	}{
		{0, "      0 B "},
		{1, "      1 B "},
		{1023, "   1023 B "},
		{1024, "   1.00 KB"},
		{1536, "   1.50 KB"},
		{1048576, "   1.00 MB"},
		{1073741824, "   1.00 GB"},
	}
	for _, tt := range tests {
		result := formatSize(tt.size)
		if result != tt.expected {
			t.Errorf("formatSize(%d) = %q, want %q", tt.size, result, tt.expected)
		}
	}
}
