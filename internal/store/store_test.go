package store

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tour-track/internal/activity"
	"tour-track/internal/api"
)

// Fixtures cover the main cases: several locations in one country, a
// location without a country, an artist missing from every endpoint except
// /artists, and broken dates. /locations and /dates describe the same
// concerts as /relation, the way the real API does.
var (
	testArtists = []api.Artist{
		{ID: 1, Name: "Queen", CreationDate: 1970},
		{ID: 2, Name: "No Concerts", CreationDate: 2020},
		{ID: 3, Name: "Bad Dates", CreationDate: 2010},
	}
	testLocations = []api.Location{
		{ID: 1, Locations: []string{"london-uk", "manchester-uk", "osaka-japan", "kyoto-japan", "nowhere"}},
		{ID: 3, Locations: []string{"paris-france", "berlin-germany"}},
	}
	testDates = []api.Dates{
		{ID: 1, Dates: []string{"*02-01-2020", "01-01-2020", "*03-01-2020", "*28-01-2019", "*28-01-2019", "*05-05-2019"}},
		{ID: 3, Dates: []string{"*not-a-date", "*01-02-2020", "32-01-2020"}},
	}
	testRelations = []api.Relation{
		{ID: 1, DatesLocations: map[string][]string{
			"london-uk":     {"02-01-2020", "01-01-2020"},
			"manchester-uk": {"03-01-2020"},
			"osaka-japan":   {"28-01-2019"},
			"kyoto-japan":   {"28-01-2019"}, // same first date as Osaka
			"nowhere":       {"05-05-2019"},
		}},
		{ID: 3, DatesLocations: map[string][]string{
			"paris-france":   {"not-a-date"},
			"berlin-germany": {"01-02-2020", "32-01-2020"},
		}},
	}
	testData = Data{
		Artists:   testArtists,
		Locations: testLocations,
		Dates:     testDates,
		Relations: testRelations,
	}
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// now is the request time used by the tests.
var now = date(2026, time.January, 1)

func cities(v ArtistView) []string {
	var out []string
	for _, c := range v.Concerts {
		out = append(out, c.City)
	}
	return out
}

func TestBuild(t *testing.T) {
	views := Build(testData)
	if len(views) != 3 {
		t.Fatalf("got %d views, want 3", len(views))
	}

	t.Run("counts and order", func(t *testing.T) {
		v := views[0]
		if v.Name != "Queen" {
			t.Fatalf("views[0].Name = %q, want Queen", v.Name)
		}
		if v.ConcertCount != 6 {
			t.Errorf("ConcertCount = %d, want 6", v.ConcertCount)
		}
		// UK twice, Japan twice and a location without country: UK, Japan.
		if v.TourSpread != 2 {
			t.Errorf("TourSpread = %d, want 2", v.TourSpread)
		}
		// Kyoto and Osaka share a date, so they are ordered by city name.
		want := []string{"Kyoto", "Osaka", "Nowhere", "London", "Manchester"}
		if got := cities(v); !slices.Equal(got, want) {
			t.Errorf("concert order = %v, want %v", got, want)
		}
		london := v.Concerts[3]
		if !london.Dates[0].Equal(date(2020, time.January, 1)) {
			t.Errorf("London dates not sorted: %v", london.Dates)
		}
		if london.Country != "UK" {
			t.Errorf("London country = %q, want UK", london.Country)
		}
	})

	t.Run("artist missing from other endpoints", func(t *testing.T) {
		v := views[1]
		if len(v.Concerts) != 0 || v.ConcertCount != 0 || v.TourSpread != 0 {
			t.Errorf("got %d concerts, count %d, spread %d, want all 0",
				len(v.Concerts), v.ConcertCount, v.TourSpread)
		}
	})

	t.Run("invalid dates are skipped", func(t *testing.T) {
		v := views[2]
		if v.ConcertCount != 1 {
			t.Errorf("ConcertCount = %d, want 1", v.ConcertCount)
		}
		// Countries come from /locations, so Paris counts even though none
		// of its dates is valid.
		if v.TourSpread != 2 {
			t.Errorf("TourSpread = %d, want 2", v.TourSpread)
		}
		if got := cities(v); !slices.Equal(got, []string{"Berlin"}) {
			t.Errorf("concerts = %v, want [Berlin]", got)
		}
	})
}

// TestBuildRepeatedLocation mirrors Maroon 5 in the real API: they played
// in Florida on two tours. /relation keeps only one of the dates because a
// JSON object cannot repeat a key, while /dates keeps both.
func TestBuildRepeatedLocation(t *testing.T) {
	views := Build(Data{
		Artists:   []api.Artist{{ID: 1, Name: "Maroon 5"}},
		Locations: []api.Location{{ID: 1, Locations: []string{"florida-usa", "florida-usa"}}},
		Dates:     []api.Dates{{ID: 1, Dates: []string{"*25-10-2019", "*01-02-2020"}}},
		Relations: []api.Relation{{ID: 1, DatesLocations: map[string][]string{"florida-usa": {"01-02-2020"}}}},
	})

	v := views[0]
	if v.ConcertCount != 2 {
		t.Errorf("ConcertCount = %d, want 2 (both dates from /dates)", v.ConcertCount)
	}
	if v.TourSpread != 1 {
		t.Errorf("TourSpread = %d, want 1", v.TourSpread)
	}
	if len(v.Concerts) != 1 {
		t.Errorf("got %d concerts in the table, want 1", len(v.Concerts))
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	want := cities(Build(testData)[0])
	for range 20 {
		if got := cities(Build(testData)[0]); !slices.Equal(got, want) {
			t.Fatalf("order changed between runs: %v vs %v", got, want)
		}
	}
}

func TestStoreAllAndByID(t *testing.T) {
	var s Store
	s.Set(Build(testData))

	if got := len(s.All(now)); got != 3 {
		t.Errorf("All returned %d artists, want 3", got)
	}
	if s.Len() != 3 {
		t.Errorf("Len = %d, want 3", s.Len())
	}

	v, ok := s.ByID(1, now)
	if !ok {
		t.Fatal("ByID(1) not found")
	}
	// 2 countries*2 + 6 concerts*3 + min(10, 56/5) = 4 + 18 + 10 = 32
	if v.CareerYears != 56 || v.Score != 32 || v.Label != activity.Established {
		t.Errorf("got career %d, score %d, label %q; want 56, 32, %q",
			v.CareerYears, v.Score, v.Label, activity.Established)
	}
	if want := (activity.Breakdown{Countries: 4, Concerts: 18, Career: 10}); v.Points != want {
		t.Errorf("Points = %+v, want %+v", v.Points, want)
	}
	// No concert since 2024, so the recent rate is 0; career: 6 / 56 years.
	if want := (activity.Trend{RecentRate: 0, CareerAverage: 6.0 / 56}); v.Trend != want || v.RisingStar {
		t.Errorf("Trend = %+v, RisingStar = %v; want %+v, false", v.Trend, v.RisingStar, want)
	}

	// Asked in 2021, the same concerts are recent: 6 / 2 = 3 per year
	// against 6 / 51 years, so Queen was a Rising Star then.
	if v, _ := s.ByID(1, date(2021, time.January, 1)); !v.RisingStar {
		t.Errorf("RisingStar in 2021 = false, want true (trend %+v)", v.Trend)
	}

	// The label depends on the year passed at request time.
	// 1980: career 10 -> 2 points -> score 24 -> Active.
	if v, _ := s.ByID(1, date(1980, time.January, 1)); v.Label != activity.Active {
		t.Errorf("label in 1980 = %q, want %q", v.Label, activity.Active)
	}

	if _, ok := s.ByID(99, now); ok {
		t.Error("ByID(99) found, want not found")
	}
}

func TestStoreRisingStar(t *testing.T) {
	var s Store
	s.Set(Build(Data{
		Artists: []api.Artist{
			{ID: 1, Name: "Old band", CreationDate: 1990},
			{ID: 2, Name: "New band", CreationDate: 2025},
			{ID: 3, Name: "Quiet band", CreationDate: 1990},
		},
		Dates: []api.Dates{
			{ID: 1, Dates: []string{"*01-06-2024", "*01-06-2025"}},
			{ID: 2, Dates: []string{"*01-06-2024", "*01-06-2025"}},
			{ID: 3, Dates: []string{"*01-01-2019", "*01-01-2020"}},
		},
	}))

	tests := []struct {
		id   int
		want bool
	}{
		{1, true},  // 2 recent = 1/year against 2 concerts / 36 years
		{2, false}, // 1/year against 2 concerts / 1 year = 2/year
		{3, false}, // no concert in the latest 2 years
	}
	for _, tt := range tests {
		if v, _ := s.ByID(tt.id, now); v.RisingStar != tt.want {
			t.Errorf("artist %d: RisingStar = %v, want %v (trend %+v)", tt.id, v.RisingStar, tt.want, v.Trend)
		}
	}
}

func TestStoreZeroValue(t *testing.T) {
	var s Store
	if got := s.All(now); len(got) != 0 {
		t.Errorf("All on empty store = %d artists, want 0", len(got))
	}
	if _, ok := s.ByID(1, now); ok {
		t.Error("ByID on empty store found an artist")
	}
	if s.Len() != 0 {
		t.Errorf("Len on empty store = %d, want 0", s.Len())
	}
}

// fakeFetcher returns fixed data, or an error per endpoint.
type fakeFetcher struct {
	data                                             Data
	artistsErr, locationsErr, datesErr, relationsErr error
}

func (f fakeFetcher) FetchArtists() ([]api.Artist, error) {
	return f.data.Artists, f.artistsErr
}

func (f fakeFetcher) FetchLocations() ([]api.Location, error) {
	return f.data.Locations, f.locationsErr
}

func (f fakeFetcher) FetchDates() ([]api.Dates, error) {
	return f.data.Dates, f.datesErr
}

func (f fakeFetcher) FetchRelations() ([]api.Relation, error) {
	return f.data.Relations, f.relationsErr
}

func TestLoad(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name    string
		fetcher fakeFetcher
		wantErr bool
	}{
		{"success", fakeFetcher{data: testData}, false},
		{"artists error", fakeFetcher{data: testData, artistsErr: boom}, true},
		{"locations error", fakeFetcher{data: testData, locationsErr: boom}, true},
		{"dates error", fakeFetcher{data: testData, datesErr: boom}, true},
		{"relations error", fakeFetcher{data: testData, relationsErr: boom}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s Store
			s.Set(Build(Data{Artists: testArtists[:1]})) // previous data

			err := s.Load(tt.fetcher)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, boom) {
				t.Errorf("error %v does not wrap the original", err)
			}

			want := 3
			if tt.wantErr {
				want = 1 // previous data must stay untouched
			}
			if got := len(s.All(now)); got != want {
				t.Errorf("store has %d artists, want %d", got, want)
			}
		})
	}
}

// TestStoreConcurrent is meant to be run with -race.
func TestStoreConcurrent(t *testing.T) {
	var s Store
	views := Build(testData)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.Set(views)
		}()
		go func() {
			defer wg.Done()
			s.All(now)
			s.ByID(1, now)
		}()
	}
	wg.Wait()
}

// parallelFetcher proves that Load runs the requests at the same time:
// FetchArtists only succeeds if FetchRelations has already started.
type parallelFetcher struct {
	fakeFetcher
	relationsStarted chan struct{}
}

func (f parallelFetcher) FetchArtists() ([]api.Artist, error) {
	select {
	case <-f.relationsStarted:
		return f.data.Artists, nil
	case <-time.After(time.Second):
		return nil, errors.New("relations were not fetched in parallel")
	}
}

func (f parallelFetcher) FetchRelations() ([]api.Relation, error) {
	close(f.relationsStarted)
	return f.data.Relations, nil
}

func TestLoadFetchesInParallel(t *testing.T) {
	var s Store
	if err := s.Load(parallelFetcher{fakeFetcher{data: testData}, make(chan struct{})}); err != nil {
		t.Fatal(err)
	}
	if s.Len() != 3 {
		t.Errorf("Len = %d, want 3", s.Len())
	}
}

// flakyFetcher fails the first failures calls of FetchArtists and then
// succeeds; the other endpoints always work.
type flakyFetcher struct {
	fakeFetcher
	failures int32
	calls    atomic.Int32
}

func newFlakyFetcher(failures int32) *flakyFetcher {
	return &flakyFetcher{fakeFetcher: fakeFetcher{data: testData}, failures: failures}
}

func (f *flakyFetcher) FetchArtists() ([]api.Artist, error) {
	if f.calls.Add(1) <= f.failures {
		return nil, errors.New("api down")
	}
	return f.data.Artists, nil
}

// silenceLogs hides the log lines written by KeepFresh during a test.
func silenceLogs(t *testing.T) {
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
}

func TestKeepFreshRetriesUntilLoaded(t *testing.T) {
	silenceLogs(t)
	var s Store
	f := newFlakyFetcher(2)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.KeepFresh(ctx, f, time.Millisecond, time.Hour)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for s.Len() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done // KeepFresh must stop when ctx is cancelled

	if s.Len() != 3 {
		t.Fatalf("Len = %d, want 3", s.Len())
	}
	// 2 failures + 1 success, then it waits an hour for the next refresh.
	if got := f.calls.Load(); got != 3 {
		t.Errorf("FetchArtists called %d times, want 3", got)
	}
}

func TestKeepFreshStopsOnCancel(t *testing.T) {
	var s Store
	f := newFlakyFetcher(0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		s.KeepFresh(ctx, f, time.Hour, time.Hour)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("KeepFresh did not stop after cancel")
	}
	if f.calls.Load() != 0 {
		t.Error("KeepFresh loaded data after cancel")
	}
}

func TestKeepFreshRefreshesLoadedStore(t *testing.T) {
	silenceLogs(t)
	var s Store
	s.Set(Build(Data{Artists: testArtists[:1]})) // already loaded
	f := newFlakyFetcher(0)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.KeepFresh(ctx, f, time.Hour, time.Millisecond)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for s.Len() != 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done

	if s.Len() != 3 {
		t.Errorf("Len = %d, want 3 after refresh", s.Len())
	}
}
