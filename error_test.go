package zest

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHTTPError_WithMessage(t *testing.T) {
	he := NewHTTPError(http.StatusNotFound, "custom not found message")

	if he.Code != http.StatusNotFound {
		t.Errorf("expected Code %d, got %d", http.StatusNotFound, he.Code)
	}
	if he.Message != "custom not found message" {
		t.Errorf("expected Message %q, got %q", "custom not found message", he.Message)
	}
	if he.err != nil {
		t.Errorf("expected inner err to be nil, got %v", he.err)
	}
}

func TestNewHTTPError_WithoutMessage(t *testing.T) {
	he := NewHTTPError(http.StatusTeapot)

	if he.Code != http.StatusTeapot {
		t.Errorf("expected Code %d, got %d", http.StatusTeapot, he.Code)
	}
	expectedMsg := http.StatusText(http.StatusTeapot)
	if he.Message != expectedMsg {
		t.Errorf("expected Message %q, got %q", expectedMsg, he.Message)
	}
}

func TestHTTPError_Error(t *testing.T) {
	t.Run("with custom message", func(t *testing.T) {
		he := NewHTTPError(http.StatusBadRequest, "bad input")
		if he.Error() != "bad input" {
			t.Errorf("expected %q, got %q", "bad input", he.Error())
		}
	})

	t.Run("without message falls back to StatusText", func(t *testing.T) {
		he := NewHTTPError(http.StatusNotFound)
		if he.Error() != http.StatusText(http.StatusNotFound) {
			t.Errorf("expected %q, got %q", http.StatusText(http.StatusNotFound), he.Error())
		}
	})

	t.Run("empty message falls back to StatusText", func(t *testing.T) {
		he := &HTTPError{Code: http.StatusInternalServerError}
		if he.Error() != http.StatusText(http.StatusInternalServerError) {
			t.Errorf("expected %q, got %q", http.StatusText(http.StatusInternalServerError), he.Error())
		}
	})
}

func TestHTTPError_Wrap(t *testing.T) {
	inner := errors.New("inner error")
	he := NewHTTPError(http.StatusInternalServerError, "wrapped error")
	result := he.Wrap(inner)

	// Wrap should return self for chaining
	if result != he {
		t.Error("Wrap should return the same *HTTPError for chaining")
	}
	// The inner error should be set
	if he.Unwrap() != inner {
		t.Errorf("expected Unwrap to return the inner error %v, got %v", inner, he.Unwrap())
	}
}

func TestHTTPError_Unwrap(t *testing.T) {
	t.Run("returns wrapped inner error", func(t *testing.T) {
		inner := errors.New("the root cause")
		he := NewHTTPError(http.StatusBadGateway).Wrap(inner)

		unwrapped := he.Unwrap()
		if unwrapped != inner {
			t.Errorf("expected %v, got %v", inner, unwrapped)
		}
	})

	t.Run("errors.Is finds the wrapped error", func(t *testing.T) {
		sentinel := errors.New("sentinel")
		he := NewHTTPError(http.StatusBadGateway).Wrap(sentinel)

		if !errors.Is(he, sentinel) {
			t.Error("errors.Is should find the sentinel error through Unwrap")
		}
	})

	t.Run("errors.Is works with HTTPError itself", func(t *testing.T) {
		inner := errors.New("nested")
		he := NewHTTPError(http.StatusBadGateway).Wrap(inner)

		// errors.Is matching the HTTPError itself
		var target *HTTPError
		if !errors.As(he, &target) {
			t.Error("errors.As should match *HTTPError")
		}
		if target.Code != http.StatusBadGateway {
			t.Errorf("expected Code %d, got %d", http.StatusBadGateway, target.Code)
		}
	})

	t.Run("nil inner error", func(t *testing.T) {
		he := NewHTTPError(http.StatusOK)
		if he.Unwrap() != nil {
			t.Errorf("expected nil, got %v", he.Unwrap())
		}
	})
}

func TestDefaultErrHandlerFunc_HTTPError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	c := NewContext(w, r)

	he := NewHTTPError(http.StatusForbidden, "access denied")
	DefaultErrHandlerFunc(c, he)

	resp := w.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	var body Map
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	if body["error"] != "access denied" {
		t.Errorf("expected error message %q, got %q", "access denied", body["error"])
	}
}

func TestDefaultErrHandlerFunc_PlainError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	c := NewContext(w, r)

	plainErr := errors.New("something went wrong")
	DefaultErrHandlerFunc(c, plainErr)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	var body Map
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	if body["error"] != "something went wrong" {
		t.Errorf("expected error message %q, got %q", "something went wrong", body["error"])
	}
}

func TestDefaultErrHandlerFunc_AlreadyCommitted(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	c := NewContext(w, r)

	// Commit a response first
	c.JSON(http.StatusOK, Map{"ok": true})

	respBefore := w.Result()
	if respBefore.StatusCode != http.StatusOK {
		t.Fatalf("expected initial status %d, got %d", http.StatusOK, respBefore.StatusCode)
	}

	// Now try to trigger error handler — it should be a no-op
	he := NewHTTPError(http.StatusInternalServerError, "should be ignored")
	DefaultErrHandlerFunc(c, he)

	// The response should still be the original 200, not 500
	respAfter := w.Result()
	if respAfter.StatusCode != http.StatusOK {
		t.Errorf("expected status still %d, got %d", http.StatusOK, respAfter.StatusCode)
	}

	// Read the body — it should still contain the original JSON, not the error
	var body Map
	if err := json.NewDecoder(respAfter.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	if body["ok"] != true {
		t.Errorf("expected original body with ok=true, got %v", body)
	}
}

func TestDefaultErrHandlerFunc_HEADRequest(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodHead, "/test", nil)
	c := NewContext(w, r)

	he := NewHTTPError(http.StatusNotFound, "gone")
	DefaultErrHandlerFunc(c, he)

	resp := w.Result()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status %d for HEAD, got %d", http.StatusNoContent, resp.StatusCode)
	}

	// HEAD requests should have no body
	bodyBytes := w.Body.Bytes()
	if len(bodyBytes) != 0 {
		t.Errorf("expected empty body for HEAD request, got %d bytes: %q", len(bodyBytes), string(bodyBytes))
	}
}
