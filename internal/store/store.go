// Package store turns raw API data into ready-to-display artist views and
// keeps them in memory for the HTTP handlers.
package store

import (
	"cmp"
	"fmt"
	"slices"
	"sync"
	"time"

	"tour-track/internal/activity"
	"tour-track/internal/api"
)

// Concert is one location where an artist played, with all its dates.
type Concert struct {
	City    string
	Country string // empty when the API location has no country part
	Dates   []time.Time
}

// ArtistView is an artist enriched with concerts and activity data.
//
// Concerts, TourSpread and ConcertCount are filled by Build. CareerYears,
// Score and Label depend on the current year, so they are filled at request
// time by Store.All and Store.ByID.
type ArtistView struct {
	api.Artist

	Concerts     []Concert
	TourSpread   int // number of unique countries
	ConcertCount int // number of concert dates

	CareerYears int
	Score       int
	Label       string
}

// Fetcher is the data source used by Store.Load. *api.Client implements it;
// tests use a fake.
type Fetcher interface {
	FetchArtists() ([]api.Artist, error)
	FetchRelations() ([]api.Relation, error)
}

// Build joins every artist with its relation and returns one view per
// artist, in the same order as artists.
//
// Assumptions: an artist without a relation has no concerts; a date that
// cannot be parsed is skipped and not counted; a location left without any
// valid date is dropped; a location without a country is shown but does
// not count towards TourSpread.
func Build(artists []api.Artist, relations []api.Relation) []ArtistView {
	relByID := make(map[int]api.Relation, len(relations))
	for _, r := range relations {
		relByID[r.ID] = r
	}

	views := make([]ArtistView, 0, len(artists))
	for _, a := range artists {
		// A missing ID gives a zero Relation whose nil map ranges as empty.
		views = append(views, buildView(a, relByID[a.ID].DatesLocations))
	}
	return views
}

// buildView parses one artist's locations and dates into a view.
func buildView(a api.Artist, datesLocations map[string][]string) ArtistView {
	v := ArtistView{Artist: a}
	countries := make(map[string]struct{})

	for location, rawDates := range datesLocations {
		city, country := ParseLocation(location)
		c := Concert{City: city, Country: country}

		for _, raw := range rawDates {
			d, err := ParseDate(raw)
			if err != nil {
				continue
			}
			c.Dates = append(c.Dates, d)
		}
		if len(c.Dates) == 0 {
			continue
		}

		slices.SortFunc(c.Dates, time.Time.Compare)
		v.Concerts = append(v.Concerts, c)
		v.ConcertCount += len(c.Dates)
		if country != "" {
			countries[country] = struct{}{}
		}
	}

	v.TourSpread = len(countries)
	// Map iteration order is random in Go, so sort for a stable output.
	slices.SortFunc(v.Concerts, compareConcerts)
	return v
}

// compareConcerts orders concerts by their first date, then by city.
func compareConcerts(a, b Concert) int {
	if c := a.Dates[0].Compare(b.Dates[0]); c != 0 {
		return c
	}
	return cmp.Compare(a.City, b.City)
}

// withActivity returns a copy of v with the activity fields computed for
// the given year.
func (v ArtistView) withActivity(year int) ArtistView {
	v.CareerYears = activity.CareerYears(v.CreationDate, year)
	v.Score = activity.Score(v.TourSpread, v.ConcertCount, v.CareerYears)
	v.Label = activity.Classify(v.TourSpread, v.ConcertCount, v.CareerYears)
	return v
}

// Store keeps artist views in memory. It is safe for concurrent use; the
// zero value is an empty store ready to use.
type Store struct {
	mu      sync.RWMutex
	artists []ArtistView
	byID    map[int]int // artist ID -> index in artists
}

// Set replaces the stored artists.
func (s *Store) Set(views []ArtistView) {
	// Build the index before locking to keep the critical section short.
	byID := make(map[int]int, len(views))
	for i, v := range views {
		byID[v.ID] = i
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.artists = views
	s.byID = byID
}

// All returns every artist with activity data computed for year.
func (s *Store) All(year int) []ArtistView {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]ArtistView, len(s.artists))
	for i, v := range s.artists {
		out[i] = v.withActivity(year)
	}
	return out
}

// ByID returns the artist with the given ID and activity data computed for
// year. The boolean is false if there is no such artist.
func (s *Store) ByID(id, year int) (ArtistView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	i, ok := s.byID[id]
	if !ok {
		return ArtistView{}, false
	}
	return s.artists[i].withActivity(year), true
}

// Load fetches fresh data from f and replaces the stored artists. On error
// the previous data is kept untouched.
func (s *Store) Load(f Fetcher) error {
	artists, err := f.FetchArtists()
	if err != nil {
		return fmt.Errorf("load artists: %w", err)
	}
	relations, err := f.FetchRelations()
	if err != nil {
		return fmt.Errorf("load relations: %w", err)
	}
	s.Set(Build(artists, relations))
	return nil
}

// Len returns the number of stored artists. It is 0 until data is loaded.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.artists)
}
