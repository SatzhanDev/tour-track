package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Fixtures: a trimmed copy of real API responses. Extra fields such as
// "locations" are kept on purpose to check that they are ignored.
const artistsJSON = `[
	{"id": 1, "image": "https://example.com/queen.jpeg", "name": "Queen",
	 "members": ["Freddie Mercury", "Brian May"], "creationDate": 1970,
	 "firstAlbum": "14-12-1973", "locations": "https://example.com/locations/1"},
	{"id": 2, "image": "https://example.com/soja.jpeg", "name": "SOJA",
	 "members": ["Jacob Hemphill"], "creationDate": 1997, "firstAlbum": "05-06-2002"}
]`

const relationJSON = `{"index": [
	{"id": 1, "datesLocations": {
		"london-uk": ["01-01-2020", "02-01-2020"],
		"osaka-japan": ["28-01-2020"]
	}}
]}`

// newFakeAPI starts a local server that answers every request with the given
// status and body. The server is closed automatically when the test ends.
func newFakeAPI(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNewClientDefaultURL(t *testing.T) {
	c := NewClient("")
	if c.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, DefaultBaseURL)
	}
	if c.httpClient.Timeout == 0 {
		t.Error("http client has no timeout")
	}
}

func TestFetchArtists(t *testing.T) {
	srv := newFakeAPI(t, http.StatusOK, artistsJSON)

	artists, err := NewClient(srv.URL).FetchArtists()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(artists) != 2 {
		t.Fatalf("got %d artists, want 2", len(artists))
	}

	queen := artists[0]
	if queen.ID != 1 {
		t.Errorf("ID = %d, want 1", queen.ID)
	}
	if queen.Name != "Queen" {
		t.Errorf("Name = %q, want %q", queen.Name, "Queen")
	}
	if queen.CreationDate != 1970 {
		t.Errorf("CreationDate = %d, want 1970", queen.CreationDate)
	}
	if queen.FirstAlbum != "14-12-1973" {
		t.Errorf("FirstAlbum = %q, want %q", queen.FirstAlbum, "14-12-1973")
	}
	if len(queen.Members) != 2 || queen.Members[0] != "Freddie Mercury" {
		t.Errorf("Members = %v, want [Freddie Mercury Brian May]", queen.Members)
	}
}

func TestFetchRelations(t *testing.T) {
	srv := newFakeAPI(t, http.StatusOK, relationJSON)

	relations, err := NewClient(srv.URL).FetchRelations()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// An empty slice here usually means a wrong json tag on relationResponse.
	if len(relations) != 1 {
		t.Fatalf("got %d relations, want 1", len(relations))
	}

	rel := relations[0]
	if rel.ID != 1 {
		t.Errorf("ID = %d, want 1", rel.ID)
	}
	if len(rel.DatesLocations) != 2 {
		t.Errorf("got %d locations, want 2", len(rel.DatesLocations))
	}
	if dates := rel.DatesLocations["london-uk"]; len(dates) != 2 {
		t.Errorf("london-uk dates = %v, want 2 dates", dates)
	}
}

func TestFetchServerError(t *testing.T) {
	srv := newFakeAPI(t, http.StatusInternalServerError, `{"error": "boom"}`)
	c := NewClient(srv.URL)

	if artists, err := c.FetchArtists(); err == nil {
		t.Errorf("FetchArtists: expected error, got %d artists", len(artists))
	}
	if relations, err := c.FetchRelations(); err == nil {
		t.Errorf("FetchRelations: expected error, got %d relations", len(relations))
	}
}

func TestFetchInvalidJSON(t *testing.T) {
	srv := newFakeAPI(t, http.StatusOK, `{not valid json`)
	c := NewClient(srv.URL)

	if _, err := c.FetchArtists(); err == nil {
		t.Error("FetchArtists: expected error for invalid JSON")
	}
	if _, err := c.FetchRelations(); err == nil {
		t.Error("FetchRelations: expected error for invalid JSON")
	}
}

func TestFetchUnreachable(t *testing.T) {
	// Start a server and close it right away: its address is now guaranteed
	// to refuse connections.
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	if _, err := NewClient(url).FetchArtists(); err == nil {
		t.Error("expected error when API is unreachable")
	}
}
