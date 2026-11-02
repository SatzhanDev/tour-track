// Package activity implements the Artist Activity Classifier.
//
// It is a rule-based "AI": a weighted sum of touring features (countries,
// concerts, career length) is compared against fixed thresholds to produce
// a label. There is no machine learning and no external service involved.
package activity

// Activity labels, from least to most active.
const (
	Emerging    = "Emerging"    // score 0..10
	Active      = "Active"      // score 11..25
	Established = "Established" // score 26..40
	Legendary   = "Legendary"   // score 41 and above
)

// Upper bounds (inclusive) of each label's score range.
const (
	maxEmerging    = 10
	maxActive      = 25
	maxEstablished = 40
)

// Formula weights and the cap on career points.
const (
	countryWeight   = 2
	concertWeight   = 3
	yearsPerPoint   = 5
	maxCareerPoints = 10
)

// CareerYears returns how many years an artist has been active.
// currentYear is passed in instead of calling time.Now() here, so the
// function stays pure and tests do not break when the year changes;
// callers use time.Now().Year() at request time, as the subject requires.
//
// Assumptions: a non-positive creationYear means "unknown" and a creation
// year in the future is a data error; both yield 0.
func CareerYears(creationYear, currentYear int) int {
	if creationYear <= 0 {
		return 0
	}
	return max(currentYear-creationYear, 0)
}

// Score computes the activity score:
//
//	score = tourSpread*2 + concertCount*3 + min(10, careerYears/5)
//
// careerYears/5 is integer division on purpose: one point per full
// five years, so 9 years give 1 point, not 1.8.
// Negative inputs are treated as 0 so the score is never negative.
func Score(tourSpread, concertCount, careerYears int) int {
	tourSpread = max(tourSpread, 0)
	concertCount = max(concertCount, 0)
	careerYears = max(careerYears, 0)

	careerPoints := min(maxCareerPoints, careerYears/yearsPerPoint)
	return tourSpread*countryWeight + concertCount*concertWeight + careerPoints
}

// LabelForScore maps a score to its activity label using the thresholds
// from the subject. Scores below 0 cannot come from Score and are
// reported as Emerging.
func LabelForScore(score int) string {
	switch {
	case score <= maxEmerging:
		return Emerging
	case score <= maxActive:
		return Active
	case score <= maxEstablished:
		return Established
	default:
		return Legendary
	}
}

// Classify returns the activity label for an artist's touring data.
func Classify(tourSpread, concertCount, careerYears int) string {
	return LabelForScore(Score(tourSpread, concertCount, careerYears))
}
