package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	"tour-track/internal/store"
)

// artistJSON is an artist as sent by the JSON API. It carries only what a
// card needs, not the full list of concerts.
type artistJSON struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Image        string `json:"image"`
	CreationDate int    `json:"creationDate"`
	ConcertCount int    `json:"concertCount"`
	Score        int    `json:"score"`
	Label        string `json:"label"`
	RisingStar   bool   `json:"risingStar"`
}

// searchResponse is the body of GET /api/artists.
type searchResponse struct {
	Total   int          `json:"total"` // number of artists before filtering
	Artists []artistJSON `json:"artists"`
}

// errorResponse is the body of every JSON error.
type errorResponse struct {
	Error string `json:"error"`
}

// apiArtists answers GET /api/artists?q=...&label=...&rising=1 with the matching
// artists as JSON. It powers the live search on the home page and accepts
// the same filters as the page itself.
func (h *Handler) apiArtists(w http.ResponseWriter, r *http.Request) {
	if !isReadMethod(r) {
		w.Header().Set("Allow", "GET, HEAD")
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{"method not allowed"})
		return
	}
	if h.store.Len() == 0 {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{"artist data is not loaded yet"})
		return
	}
	filter, ok := parseFilter(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, errorResponse{"unknown activity label"})
		return
	}

	all := h.store.All(h.now())
	found := store.Search(all, filter)

	resp := searchResponse{Total: len(all), Artists: make([]artistJSON, len(found))}
	for i, a := range found {
		resp.Artists[i] = artistJSON{
			ID:           a.ID,
			Name:         a.Name,
			Image:        a.Image,
			CreationDate: a.CreationDate,
			ConcertCount: a.ConcertCount,
			Score:        a.Score,
			Label:        a.Label,
			RisingStar:   a.RisingStar,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// writeJSON encodes v into a buffer first, like render does for HTML, so an
// encoding error never sends a half-written body with status 200.
func writeJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		log.Printf("encode json: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	send(w, status, "application/json", &buf)
}
