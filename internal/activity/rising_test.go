package activity

import (
	"testing"
	"time"
)

func TestIsRisingStar(t *testing.T) {
	tests := []struct {
		name                      string
		recentRate, careerAverage float64
		want                      bool
	}{
		// The example from the subject: career average 4, threshold 6.
		{"subject example, recent 7", 7, 4, true},
		{"subject example, recent 5", 5, 4, false},
		{"exactly 50% more is not enough", 6, 4, false},
		{"no concerts at all", 0, 0, false},
		{"recent concerts, empty career", 1, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRisingStar(tt.recentRate, tt.careerAverage); got != tt.want {
				t.Errorf("IsRisingStar(%v, %v) = %v, want %v",
					tt.recentRate, tt.careerAverage, got, tt.want)
			}
		})
	}
}

func TestExplainTrend(t *testing.T) {
	tests := []struct {
		name                          string
		recent, concerts, careerYears int
		want                          Trend
		wantRising                    bool
	}{
		// 40 concerts in 10 years = 4/year; 14 recent = 7/year.
		{"subject example, rising", 14, 40, 10, Trend{RecentRate: 7, CareerAverage: 4}, true},
		// 10 recent = 5/year.
		{"subject example, not rising", 10, 40, 10, Trend{RecentRate: 5, CareerAverage: 4}, false},
		// Queen today: 8 concerts, none in the latest 2 years, 56 years.
		{"queen real data", 0, 8, 56, Trend{RecentRate: 0, CareerAverage: 8.0 / 56}, false},
		{"career under a year counts as one", 2, 2, 0, Trend{RecentRate: 1, CareerAverage: 2}, false},
		{"negative inputs treated as zero", -1, -1, 5, Trend{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExplainTrend(tt.recent, tt.concerts, tt.careerYears)
			if got != tt.want {
				t.Errorf("ExplainTrend(%d, %d, %d) = %+v, want %+v",
					tt.recent, tt.concerts, tt.careerYears, got, tt.want)
			}
			if got.Rising() != tt.wantRising {
				t.Errorf("Rising() = %v, want %v", got.Rising(), tt.wantRising)
			}
		})
	}
}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestRecentConcerts(t *testing.T) {
	now := day(2026, 11, 7)
	tests := []struct {
		name  string
		dates []time.Time
		want  int
	}{
		{"no concerts", nil, 0},
		{"inside the window", []time.Time{day(2025, 1, 1), day(2026, 6, 1)}, 2},
		{"old concerts are left out", []time.Time{day(2019, 1, 1), day(2020, 1, 1), day(2025, 1, 1)}, 1},
		{"window start is included", []time.Time{day(2024, 11, 7)}, 1},
		{"one day before the window", []time.Time{day(2024, 11, 6)}, 0},
		{"today is included", []time.Time{now}, 1},
		{"upcoming concerts are not counted", []time.Time{day(2026, 11, 8), day(2027, 1, 1)}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RecentConcerts(tt.dates, now); got != tt.want {
				t.Errorf("RecentConcerts = %d, want %d", got, tt.want)
			}
		})
	}
}
