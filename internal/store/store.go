// Package store turns raw API data into ready-to-display artist views and
// keeps them in memory for the HTTP handlers.
package store

import (
	"cmp"
	"context"
	"fmt"
	"log"
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
// Points, Score, Label, Trend and RisingStar depend on the current date, so
// they are filled at request time by Store.All and Store.ByID.
type ArtistView struct {
	api.Artist

	Concerts     []Concert   // from /relation
	TourSpread   int         // unique countries, from /locations
	ConcertCount int         // concert dates, from /dates
	dates        []time.Time // every date from /dates, for the Rising Star window

	CareerYears int
	Points      activity.Breakdown // how the score was computed
	Score       int
	Label       string
	Trend       activity.Trend // the two rates the Rising Star rule compares
	RisingStar  bool
}

// Fetcher is the data source used by Store.Load. *api.Client implements it;
// tests use a fake.
type Fetcher interface {
	FetchArtists() ([]api.Artist, error)
	FetchLocations() ([]api.Location, error)
	FetchDates() ([]api.Dates, error)
	FetchRelations() ([]api.Relation, error)
}

// Data holds the answers of the four API endpoints.
type Data struct {
	Artists   []api.Artist
	Locations []api.Location
	Dates     []api.Dates
	Relations []api.Relation
}

// Build links the four API parts by artist ID and returns one view per
// artist, in the same order as d.Artists. Each part is used for what it
// describes:
//
//   - artists: name, image, members, creation year and first album;
//   - locations: TourSpread, the unique countries of the concert locations;
//   - dates: ConcertCount and the Rising Star recent rate, from every date;
//   - relation: the concerts table, linking each location to its dates.
//
// /relation is a JSON object keyed by location, so when an artist played
// in the same place on two tours only one group of dates survives there
// (3 artists in the current data). /dates keeps every date, which is why
// the counts come from it.
//
// Assumptions: an artist missing from an endpoint has no data from it; a
// date that cannot be parsed is skipped and not counted; a location left
// without any valid date is not shown in the table; a location without a
// country does not count towards TourSpread.
func Build(d Data) []ArtistView {
	locByID := make(map[int][]string, len(d.Locations))
	for _, l := range d.Locations {
		locByID[l.ID] = l.Locations
	}
	datesByID := make(map[int][]string, len(d.Dates))
	for _, dt := range d.Dates {
		datesByID[dt.ID] = dt.Dates
	}
	relByID := make(map[int]map[string][]string, len(d.Relations))
	for _, r := range d.Relations {
		relByID[r.ID] = r.DatesLocations
	}

	views := make([]ArtistView, 0, len(d.Artists))
	for _, a := range d.Artists {
		// A missing ID gives nil, which ranges as empty.
		v := ArtistView{Artist: a}
		v.TourSpread = countCountries(locByID[a.ID])
		dates := parseDates(datesByID[a.ID])
		v.ConcertCount = len(dates)
		v.dates = dates
		v.Concerts = buildConcerts(relByID[a.ID])
		views = append(views, v)
	}
	return views
}

// countCountries returns the number of different countries in locations.
func countCountries(locations []string) int {
	countries := make(map[string]struct{})
	for _, l := range locations {
		if _, country := ParseLocation(l); country != "" {
			countries[country] = struct{}{}
		}
	}
	return len(countries)
}

// parseDates parses the dates of the /dates endpoint, skipping invalid ones.
func parseDates(raw []string) []time.Time {
	var dates []time.Time
	for _, r := range raw {
		if d, err := ParseDate(r); err == nil {
			dates = append(dates, d)
		}
	}
	return dates
}

// buildConcerts turns one artist's relation into concerts sorted by date.
func buildConcerts(datesLocations map[string][]string) []Concert {
	var concerts []Concert
	for location, rawDates := range datesLocations {
		dates := parseDates(rawDates)
		if len(dates) == 0 {
			continue
		}
		slices.SortFunc(dates, time.Time.Compare)
		city, country := ParseLocation(location)
		concerts = append(concerts, Concert{City: city, Country: country, Dates: dates})
	}
	// Map iteration order is random in Go, so sort for a stable output.
	slices.SortFunc(concerts, compareConcerts)
	return concerts
}

// compareConcerts orders concerts by their first date, then by city.
func compareConcerts(a, b Concert) int {
	if c := a.Dates[0].Compare(b.Dates[0]); c != 0 {
		return c
	}
	return cmp.Compare(a.City, b.City)
}

// withActivity returns a copy of v with the activity fields computed at the
// time now.
func (v ArtistView) withActivity(now time.Time) ArtistView {
	v.CareerYears = activity.CareerYears(v.CreationDate, now.Year())
	v.Points = activity.Explain(v.TourSpread, v.ConcertCount, v.CareerYears)
	v.Score = v.Points.Total()
	v.Label = activity.LabelForScore(v.Score)
	recent := activity.RecentConcerts(v.dates, now)
	v.Trend = activity.ExplainTrend(recent, v.ConcertCount, v.CareerYears)
	v.RisingStar = v.Trend.Rising()
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

// All returns every artist with activity data computed at the time now.
func (s *Store) All(now time.Time) []ArtistView {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]ArtistView, len(s.artists))
	for i, v := range s.artists {
		out[i] = v.withActivity(now)
	}
	return out
}

// ByID returns the artist with the given ID and activity data computed at
// the time now. The boolean is false if there is no such artist.
func (s *Store) ByID(id int, now time.Time) (ArtistView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	i, ok := s.byID[id]
	if !ok {
		return ArtistView{}, false
	}
	return s.artists[i].withActivity(now), true
}

// fetchResult carries one API answer through a channel.
type fetchResult[T any] struct {
	value T
	err   error
}

// fetchAsync runs fetch in its own goroutine and returns a channel that
// receives the answer. The channel is buffered, so the goroutine can always
// send and finish, even when nobody receives because Load returned early.
func fetchAsync[T any](fetch func() (T, error)) <-chan fetchResult[T] {
	ch := make(chan fetchResult[T], 1)
	go func() {
		value, err := fetch()
		ch <- fetchResult[T]{value, err}
	}()
	return ch
}

// Load fetches fresh data from f and replaces the stored artists. The four
// endpoints are requested at the same time, so a load takes as long as the
// slowest request instead of all four added up. On error the previous data
// is kept untouched.
func (s *Store) Load(f Fetcher) error {
	artistsCh := fetchAsync(f.FetchArtists)
	locationsCh := fetchAsync(f.FetchLocations)
	datesCh := fetchAsync(f.FetchDates)
	relationsCh := fetchAsync(f.FetchRelations)

	artists := <-artistsCh
	if artists.err != nil {
		return fmt.Errorf("load artists: %w", artists.err)
	}
	locations := <-locationsCh
	if locations.err != nil {
		return fmt.Errorf("load locations: %w", locations.err)
	}
	dates := <-datesCh
	if dates.err != nil {
		return fmt.Errorf("load dates: %w", dates.err)
	}
	relations := <-relationsCh
	if relations.err != nil {
		return fmt.Errorf("load relations: %w", relations.err)
	}

	s.Set(Build(Data{
		Artists:   artists.value,
		Locations: locations.value,
		Dates:     dates.value,
		Relations: relations.value,
	}))
	return nil
}

// KeepFresh reloads the data from f in the background until ctx is
// cancelled. While the store is empty or the last load failed it tries
// again after retry; after a successful load it waits refresh, so the site
// follows changes in the API. It blocks, so run it in its own goroutine.
func (s *Store) KeepFresh(ctx context.Context, f Fetcher, retry, refresh time.Duration) {
	wait := refresh
	if s.Len() == 0 {
		wait = retry
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		if err := s.Load(f); err != nil {
			log.Printf("reload failed: %v; retrying in %s", err, retry)
			timer.Reset(retry)
			continue
		}
		log.Printf("loaded %d artists", s.Len())
		timer.Reset(refresh)
	}
}

// Len returns the number of stored artists. It is 0 until data is loaded.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.artists)
}
