package ui

import (
	"bytes"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIconValidPNG(t *testing.T) {
	b := Icon()
	if _, err := png.Decode(bytes.NewReader(b)); err != nil {
		t.Fatalf("icon not valid PNG: %v", err)
	}
	t.Logf("icon bytes=%d", len(b))
}

func TestHandlerServesLogo(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/logo.svg", nil)
	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logo status=%d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct == "" {
		t.Fatalf("missing content-type")
	}
	t.Logf("logo content-type=%q body=%d bytes", w.Header().Get("Content-Type"), w.Body.Len())
}
