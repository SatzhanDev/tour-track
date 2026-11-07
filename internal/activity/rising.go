package activity

import "time"

// Rising Star rule from the subject: an artist is rising when their
// concert rate over the latest 2 years is more than 50% above their career
// average rate (concert_count / career_years).
const (
	risingWindowYears = 2
	risingFactor      = 1.5
)

// IsRisingStar applies the rule to two rates in concerts per year. With the
// subject's example, a career average of 4 gives a threshold of 6: a recent
// rate of 7 is rising, 5 is not, and exactly 6 is not ("more than 50%").
func IsRisingStar(recentRate, careerAverage float64) bool {
	return recentRate > risingFactor*careerAverage
}

// Trend holds the two rates the Rising Star rule compares, so they can be
// shown next to the badge.
type Trend struct {
	RecentRate    float64 // concerts per year over the latest 2 years
	CareerAverage float64 // concertCount / careerYears
}

// Rising reports whether the trend earns the Rising Star badge.
func (t Trend) Rising() bool {
	return IsRisingStar(t.RecentRate, t.CareerAverage)
}

// ExplainTrend computes both rates from the artist's numbers: recent is the
// number of concerts in the latest 2 years (see RecentConcerts), concerts
// and careerYears are the same values the classifier uses.
//
// A career shorter than one year counts as one year, so the average is
// always defined.
func ExplainTrend(recent, concerts, careerYears int) Trend {
	return Trend{
		RecentRate:    float64(max(recent, 0)) / risingWindowYears,
		CareerAverage: float64(max(concerts, 0)) / float64(max(careerYears, 1)),
	}
}

// RecentConcerts counts the dates within the latest 2 years: from 2 years
// before now up to now, both included. now is passed in, like currentYear
// in CareerYears, so the function stays pure and testable; callers use the
// time of the request. Upcoming concerts (after now) have not happened yet
// and are not counted.
func RecentConcerts(dates []time.Time, now time.Time) int {
	start := now.AddDate(-risingWindowYears, 0, 0)

	recent := 0
	for _, d := range dates {
		if !d.Before(start) && !d.After(now) {
			recent++
		}
	}
	return recent
}
