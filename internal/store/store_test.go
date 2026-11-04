package store

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"tour-track/internal/activity"
	"tour-track/internal/api"
)

// Fixtures cover the main cases: several locations in one country, a
// location without a country, an artist without a relation and broken dates.
var (
	testArtists = []api.Artist{
		{ID: 1, Name: "Queen", CreationDate: 1970},
		{ID: 2, Name: "No Concerts", CreationDate: 2020},
		{ID: 3, Name: "Bad Dates", CreationDate: 2010},
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
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func cities(v ArtistView) []string {
	var out []string
	for _, c := range v.Concerts {
		out = append(out, c.City)
	}
	return out
}

func TestBuild(t *testing.T) {
	views := Build(testArtists, testRelations)
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

	t.Run("artist without relation", func(t *testing.T) {
		v := views[1]
		if len(v.Concerts) != 0 || v.ConcertCount != 0 || v.TourSpread != 0 {
			t.Errorf("got %d concerts, count %d, spread %d, want all 0",
				len(v.Concerts), v.ConcertCount, v.TourSpread)
		}
	})

	t.Run("invalid dates are skipped", func(t *testing.T) {
		v := views[2]
		if v.ConcertCount != 1 || v.TourSpread != 1 {
			t.Errorf("ConcertCount = %d, TourSpread = %d, want 1 and 1",
				v.ConcertCount, v.TourSpread)
		}
		if got := cities(v); !slices.Equal(got, []string{"Berlin"}) {
			t.Errorf("concerts = %v, want [Berlin]", got)
		}
	})
}

func TestBuildIsDeterministic(t *testing.T) {
	want := cities(Build(testArtists, testRelations)[0])
	for range 20 {
		if got := cities(Build(testArtists, testRelations)[0]); !slices.Equal(got, want) {
			t.Fatalf("order changed between runs: %v vs %v", got, want)
		}
	}
}

func TestStoreAllAndByID(t *testing.T) {
	var s Store
	s.Set(Build(testArtists, testRelations))

	if got := len(s.All(2026)); got != 3 {
		t.Errorf("All returned %d artists, want 3", got)
	}
	if s.Len() != 3 {
		t.Errorf("Len = %d, want 3", s.Len())
	}

	v, ok := s.ByID(1, 2026)
	if !ok {
		t.Fatal("ByID(1) not found")
	}
	// 2 countries*2 + 6 concerts*3 + min(10, 56/5) = 4 + 18 + 10 = 32
	if v.CareerYears != 56 || v.Score != 32 || v.Label != activity.Established {
		t.Errorf("got career %d, score %d, label %q; want 56, 32, %q",
			v.CareerYears, v.Score, v.Label, activity.Established)
	}

	// The label depends on the year passed at request time.
	// 1980: career 10 -> 2 points -> score 24 -> Active.
	if v, _ := s.ByID(1, 1980); v.Label != activity.Active {
		t.Errorf("label in 1980 = %q, want %q", v.Label, activity.Active)
	}

	if _, ok := s.ByID(99, 2026); ok {
		t.Error("ByID(99) found, want not found")
	}
}

func TestStoreZeroValue(t *testing.T) {
	var s Store
	if got := s.All(2026); len(got) != 0 {
		t.Errorf("All on empty store = %d artists, want 0", len(got))
	}
	if _, ok := s.ByID(1, 2026); ok {
		t.Error("ByID on empty store found an artist")
	}
	if s.Len() != 0 {
		t.Errorf("Len on empty store = %d, want 0", s.Len())
	}

}

type fakeFetcher struct {
	artists      []api.Artist
	relations    []api.Relation
	artistsErr   error
	relationsErr error
}

func (f fakeFetcher) FetchArtists() ([]api.Artist, error) {
	return f.artists, f.artistsErr
}

func (f fakeFetcher) FetchRelations() ([]api.Relation, error) {
	return f.relations, f.relationsErr
}

func TestLoad(t *testing.T) {
	ok := fakeFetcher{artists: testArtists, relations: testRelations}
	boom := errors.New("boom")

	tests := []struct {
		name    string
		fetcher fakeFetcher
		wantErr bool
	}{
		{"success", ok, false},
		{"artists error", fakeFetcher{artistsErr: boom}, true},
		{"relations error", fakeFetcher{artists: testArtists, relationsErr: boom}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s Store
			s.Set(Build(testArtists[:1], testRelations)) // previous data

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
			if got := len(s.All(2026)); got != want {
				t.Errorf("store has %d artists, want %d", got, want)
			}
		})
	}
}

// TestStoreConcurrent is meant to be run with -race.
func TestStoreConcurrent(t *testing.T) {
	var s Store
	views := Build(testArtists, testRelations)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.Set(views)
		}()
		go func() {
			defer wg.Done()
			s.All(2026)
			s.ByID(1, 2026)
		}()
	}
	wg.Wait()
}
