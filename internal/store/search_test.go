package store

import (
	"slices"
	"testing"

	"tour-track/internal/activity"
	"tour-track/internal/api"
)

// searchViews already carry a label, like the result of Store.All.
var searchViews = []ArtistView{
	{
		Artist:   api.Artist{ID: 1, Name: "Queen", Members: []string{"Freddie Mercury"}},
		Concerts: []Concert{{City: "Osaka", Country: "Japan"}},
		Label:    activity.Established,
	},
	{
		Artist:     api.Artist{ID: 2, Name: "Pink Floyd", Members: []string{"Roger Waters"}},
		Concerts:   []Concert{{City: "London", Country: "UK"}},
		Label:      activity.Legendary,
		RisingStar: true,
	},
	{
		Artist: api.Artist{ID: 3, Name: "SOJA"},
		Label:  activity.Established,
	},
}

func ids(views []ArtistView) []int {
	out := []int{}
	for _, v := range views {
		out = append(out, v.ID)
	}
	return out
}

func TestSearch(t *testing.T) {
	tests := []struct {
		name   string
		filter Filter
		want   []int
	}{
		{"no filters", Filter{}, []int{1, 2, 3}},
		{"name ignores case", Filter{Query: "queen"}, []int{1}},
		{"part of a name", Filter{Query: "floy"}, []int{2}},
		{"member", Filter{Query: "freddie"}, []int{1}},
		{"city", Filter{Query: "osaka"}, []int{1}},
		{"country", Filter{Query: "uk"}, []int{2}},
		{"spaces around query", Filter{Query: "  soja  "}, []int{3}},
		{"no match", Filter{Query: "nobody"}, []int{}},
		{"label only", Filter{Label: activity.Established}, []int{1, 3}},
		{"label ignores case", Filter{Label: "LEGENDARY"}, []int{2}},
		{"query and label", Filter{Query: "queen", Label: activity.Established}, []int{1}},
		{"query and other label", Filter{Query: "queen", Label: activity.Legendary}, []int{}},
		{"rising only", Filter{RisingOnly: true}, []int{2}},
		{"rising and query", Filter{Query: "queen", RisingOnly: true}, []int{}},
		{"rising and label", Filter{Label: activity.Legendary, RisingOnly: true}, []int{2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ids(Search(searchViews, tt.filter))
			if !slices.Equal(got, tt.want) {
				t.Errorf("Search(%+v) = %v, want %v", tt.filter, got, tt.want)
			}
		})
	}
}

func TestSearchEmptyResultIsNotNil(t *testing.T) {
	// A nil slice would be encoded as JSON null instead of [].
	if got := Search(searchViews, Filter{Query: "nobody"}); got == nil {
		t.Error("Search returned nil, want an empty slice")
	}
}
