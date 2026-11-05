package handlers

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"tour-track/internal/api"
	"tour-track/internal/store"
)

// TestMain silences request logs so test output stays readable.
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// The real templates and static files are used, so template mistakes are
// caught by these tests too.
var (
	templatesFS = os.DirFS("../../templates")
	staticFS    = os.DirFS("../../static")
)

func newTestHandler(t *testing.T, s *store.Store) *Handler {
	t.Helper()
	h, err := New(s, templatesFS, staticFS)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	return h
}

func loadedStore() *store.Store {
	var s store.Store
	s.Set(store.Build(
		[]api.Artist{{ID: 1, Name: "Queen", CreationDate: 1970, Members: []string{"Freddie Mercury"}}},
		[]api.Relation{{ID: 1, DatesLocations: map[string][]string{
			"london-uk":   {"01-01-2020"},
			"osaka-japan": {"02-01-2020", "03-01-2020"},
		}}},
	))
	return &s
}

func TestRoutes(t *testing.T) {
	h := newTestHandler(t, loadedStore())
	routes := h.Routes()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{"home", http.MethodGet, "/", 200, "Queen"},
		{"home shows badge", http.MethodGet, "/", 200, "badge-active"},
		{"artist page", http.MethodGet, "/artist/1", 200, "Osaka"},
		// 2 countries*2 + 3 concerts*3 + min(10, 56/5) = 23 -> Active
		{"artist score", http.MethodGet, "/artist/1", 200, "Activity score: 23"},
		{"static file", http.MethodGet, "/static/style.css", 200, ".badge"},
		{"head allowed", http.MethodHead, "/", 200, ""},
		{"id not a number", http.MethodGet, "/artist/abc", 400, "positive number"},
		{"id zero", http.MethodGet, "/artist/0", 400, "positive number"},
		{"id negative", http.MethodGet, "/artist/-1", 400, "positive number"},
		{"id overflow", http.MethodGet, "/artist/99999999999999999999", 400, "positive number"},
		{"unknown artist", http.MethodGet, "/artist/999", 404, "no artist"},
		{"extra segment", http.MethodGet, "/artist/1/extra", 404, "does not exist"},
		{"unknown page", http.MethodGet, "/nope", 404, "does not exist"},
		{"post home", http.MethodPost, "/", 405, "not allowed"},
		{"delete artist", http.MethodDelete, "/artist/1", 405, "not allowed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			routes.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body does not contain %q", tt.wantBody)
			}
		})
	}
}

func TestMethodNotAllowedSetsAllowHeader(t *testing.T) {
	h := newTestHandler(t, loadedStore())
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))

	if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
		t.Errorf("Allow = %q, want %q", got, "GET, HEAD")
	}
}

func TestDataNotLoaded(t *testing.T) {
	h := newTestHandler(t, &store.Store{})
	routes := h.Routes()

	for _, path := range []string{"/", "/artist/1"} {
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("GET %s: status = %d, want 503", path, rec.Code)
		}
	}
}

func TestTemplateErrorGives500(t *testing.T) {
	// index.html fails at execution time: a slice has no field Missing.
	broken := fstest.MapFS{
		"index.html": {Data: []byte(`partial {{.Missing}}`)},
		"error.html": {Data: []byte(`{{.Status}} {{.Message}}`)},
	}
	h, err := New(loadedStore(), broken, staticFS)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "partial") {
		t.Error("half-rendered page was sent to the client")
	}
}

func TestErrorTemplateMissingFallsBackToText(t *testing.T) {
	onlyIndex := fstest.MapFS{"index.html": {Data: []byte(`ok`)}}
	h, err := New(loadedStore(), onlyIndex, staticFS)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestNewInvalidTemplate(t *testing.T) {
	bad := fstest.MapFS{"index.html": {Data: []byte(`{{ if }}`)}}
	if _, err := New(&store.Store{}, bad, staticFS); err == nil {
		t.Error("expected error for invalid template syntax")
	}
}

func TestRecoverPanic(t *testing.T) {
	h := newTestHandler(t, loadedStore())
	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	rec := httptest.NewRecorder()
	h.recoverPanic(panicking).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestTemplateFuncs(t *testing.T) {
	date := templateFuncs["date"].(func(time.Time) string)
	if got := date(time.Date(2019, time.August, 23, 0, 0, 0, 0, time.UTC)); got != "23 Aug 2019" {
		t.Errorf("date = %q, want %q", got, "23 Aug 2019")
	}
}
