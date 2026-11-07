package store

import (
	"slices"
	"strings"
)

// Filter describes which artists Search returns. The zero value matches
// every artist.
type Filter struct {
	// Query is matched case-insensitively against the artist name,
	// members, concert cities and countries. Empty matches everyone.
	Query string
	// Label is compared with the activity label ignoring case. Empty
	// matches every label.
	Label string
	// RisingOnly keeps only Rising Stars.
	RisingOnly bool
}

// Search returns the artists that match every part of f, keeping their
// order. Activity data must already be filled in, so pass the result of
// Store.All.
func Search(views []ArtistView, f Filter) []ArtistView {
	query := strings.ToLower(strings.TrimSpace(f.Query))

	out := make([]ArtistView, 0, len(views))
	for _, v := range views {
		if f.Label != "" && !strings.EqualFold(v.Label, f.Label) {
			continue
		}
		if f.RisingOnly && !v.RisingStar {
			continue
		}
		if query == "" || v.matches(query) {
			out = append(out, v)
		}
	}
	return out
}

// matches reports whether query appears in the artist's name, members or
// concert places. query must already be lower case.
func (v ArtistView) matches(query string) bool {
	contains := func(s string) bool {
		return strings.Contains(strings.ToLower(s), query)
	}

	if contains(v.Name) || slices.ContainsFunc(v.Members, contains) {
		return true
	}
	return slices.ContainsFunc(v.Concerts, func(c Concert) bool {
		return contains(c.City) || contains(c.Country)
	})
}
