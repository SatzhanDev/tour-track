package activity

import (
	"slices"
	"testing"
)

func TestCareerYears(t *testing.T) {
	tests := []struct {
		name                      string
		creationYear, currentYear int
		want                      int
	}{
		{"regular", 1970, 2026, 56},
		{"same year", 2026, 2026, 0},
		{"creation in the future", 2030, 2026, 0},
		{"unknown creation year", 0, 2026, 0},
		{"negative creation year", -1, 2026, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CareerYears(tt.creationYear, tt.currentYear)
			if got != tt.want {
				t.Errorf("CareerYears(%d, %d) = %d, want %d",
					tt.creationYear, tt.currentYear, got, tt.want)
			}
		})
	}
}

func TestScore(t *testing.T) {
	tests := []struct {
		name                             string
		tourSpread, concertCount, career int
		want                             int
	}{
		{"all zero", 0, 0, 0, 0},
		{"countries only", 3, 0, 0, 6},
		{"concerts only", 0, 4, 0, 12},
		{"career below 5 years gives no points", 0, 0, 4, 0},
		{"integer division drops remainder", 0, 0, 9, 1},
		{"career 49 years", 0, 0, 49, 9},
		{"career cap reached at 50", 0, 0, 50, 10},
		{"career cap above 50", 0, 0, 68, 10},
		{"queen real data", 3, 8, 56, 40},
		// The example auditors check by hand: 10 + 24 + 6 = 40.
		{"audit example", 5, 8, 30, 40},
		{"negative inputs treated as zero", -5, -2, -10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Score(tt.tourSpread, tt.concertCount, tt.career)
			if got != tt.want {
				t.Errorf("Score(%d, %d, %d) = %d, want %d",
					tt.tourSpread, tt.concertCount, tt.career, got, tt.want)
			}
		})
	}
}

func TestLabelForScore(t *testing.T) {
	tests := []struct {
		score int
		want  string
	}{
		{-1, Emerging},
		{0, Emerging},
		{10, Emerging},
		{11, Active},
		{25, Active},
		{26, Established},
		{40, Established},
		{41, Legendary},
		{1000, Legendary},
	}
	for _, tt := range tests {
		if got := LabelForScore(tt.score); got != tt.want {
			t.Errorf("LabelForScore(%d) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name                             string
		tourSpread, concertCount, career int
		want                             string
	}{
		{"emerging upper bound (10)", 2, 2, 0, Emerging},
		{"active lower bound (11)", 1, 3, 0, Active},
		{"active upper bound (25)", 2, 7, 0, Active},
		{"established lower bound (26)", 1, 8, 0, Established},
		{"queen: established upper bound (40)", 3, 8, 56, Established},
		{"audit example (40)", 5, 8, 30, Established},
		{"legendary lower bound (41)", 1, 13, 0, Legendary},
		{"long career alone stays emerging", 0, 0, 68, Emerging},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.tourSpread, tt.concertCount, tt.career)
			if got != tt.want {
				t.Errorf("Classify(%d, %d, %d) = %q, want %q",
					tt.tourSpread, tt.concertCount, tt.career, got, tt.want)
			}
		})
	}
}

func TestExplain(t *testing.T) {
	tests := []struct {
		name                             string
		tourSpread, concertCount, career int
		want                             Breakdown
	}{
		{"all zero", 0, 0, 0, Breakdown{}},
		{"queen real data", 3, 8, 56, Breakdown{Countries: 6, Concerts: 24, Career: 10}},
		{"career below cap", 1, 1, 23, Breakdown{Countries: 2, Concerts: 3, Career: 4}},
		{"negative inputs treated as zero", -1, -1, -1, Breakdown{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Explain(tt.tourSpread, tt.concertCount, tt.career)
			if got != tt.want {
				t.Errorf("Explain(%d, %d, %d) = %+v, want %+v",
					tt.tourSpread, tt.concertCount, tt.career, got, tt.want)
			}
			if got.Total() != Score(tt.tourSpread, tt.concertCount, tt.career) {
				t.Errorf("Total() = %d, Score = %d, want equal",
					got.Total(), Score(tt.tourSpread, tt.concertCount, tt.career))
			}
		})
	}
}

func TestLabels(t *testing.T) {
	want := []string{Emerging, Active, Established, Legendary}
	got := Labels()
	if !slices.Equal(got, want) {
		t.Fatalf("Labels() = %v, want %v", got, want)
	}

	// Changing the returned slice must not affect later calls.
	got[0] = "changed"
	if Labels()[0] != Emerging {
		t.Error("Labels() returned shared data")
	}
}
