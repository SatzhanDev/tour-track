package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tour-track/internal/store"
)

// getJSON sends a request to the router and decodes the JSON answer.
func getJSON(t *testing.T, h *Handler, method, path string, target any) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(method, path, nil))

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		t.Fatalf("invalid JSON %q: %v", rec.Body.String(), err)
	}
	return rec
}

func TestAPIArtists(t *testing.T) {
	h := newTestHandler(t, loadedStore())

	tests := []struct {
		name      string
		path      string
		wantCount int
	}{
		{"no filters", "/api/artists", 1},
		{"name", "/api/artists?q=queen", 1},
		{"city", "/api/artists?q=Osaka", 1},
		{"no match", "/api/artists?q=nobody", 0},
		{"label", "/api/artists?label=Active", 1},
		{"label any case", "/api/artists?label=active", 1},
		{"other label", "/api/artists?label=Legendary", 0},
		{"empty label means any", "/api/artists?label=", 1},
		{"rising only", "/api/artists?rising=1", 0},
		{"rising other value ignored", "/api/artists?rising=yes", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp searchResponse
			rec := getJSON(t, h, http.MethodGet, tt.path, &resp)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if resp.Total != 1 {
				t.Errorf("total = %d, want 1", resp.Total)
			}
			if len(resp.Artists) != tt.wantCount {
				t.Errorf("got %d artists, want %d", len(resp.Artists), tt.wantCount)
			}
		})
	}
}

func TestAPIArtistFields(t *testing.T) {
	h := newTestHandler(t, loadedStore())
	var resp searchResponse
	getJSON(t, h, http.MethodGet, "/api/artists", &resp)

	want := artistJSON{ID: 1, Name: "Queen", CreationDate: 1970, ConcertCount: 3, Score: 23, Label: "Active"}
	if resp.Artists[0] != want {
		t.Errorf("artist = %+v, want %+v", resp.Artists[0], want)
	}
}

func TestAPIEmptyResultIsArray(t *testing.T) {
	h := newTestHandler(t, loadedStore())
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/artists?q=nobody", nil))

	// JavaScript expects an array; null would break artists.map(...).
	if !strings.Contains(rec.Body.String(), `"artists":[]`) {
		t.Errorf("body = %s, want an empty array", rec.Body.String())
	}
}

func TestAPIErrors(t *testing.T) {
	tests := []struct {
		name       string
		store      *store.Store
		method     string
		path       string
		wantStatus int
	}{
		{"unknown label", loadedStore(), http.MethodGet, "/api/artists?label=famous", 400},
		{"wrong method", loadedStore(), http.MethodPost, "/api/artists", 405},
		{"data not loaded", &store.Store{}, http.MethodGet, "/api/artists", 503},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp errorResponse
			rec := getJSON(t, newTestHandler(t, tt.store), tt.method, tt.path, &resp)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if resp.Error == "" {
				t.Error("error message is empty")
			}
		})
	}
}

func TestAPIWrongMethodSetsAllowHeader(t *testing.T) {
	h := newTestHandler(t, loadedStore())
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/artists", nil))

	if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
		t.Errorf("Allow = %q, want %q", got, "GET, HEAD")
	}
}

func TestWriteJSONEncodeError(t *testing.T) {
	// A channel cannot be encoded as JSON.
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, make(chan int))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}
