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

// Labels returns all activity labels, from least to most active.
// It returns a new slice on every call, so callers may modify it.
func Labels() []string {
	return []string{Emerging, Active, Established, Legendary}
}

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

// Breakdown shows how many points each feature adds to the score, so the
// result can be explained to the user.
type Breakdown struct {
	Countries int // tourSpread*2
	Concerts  int // concertCount*3
	Career    int // min(10, careerYears/5)
}

// Total returns the activity score: the sum of all points.
func (b Breakdown) Total() int {
	return b.Countries + b.Concerts + b.Career
}

// Explain computes the points of each part of the formula:
//
//	score = tourSpread*2 + concertCount*3 + min(10, careerYears/5)
//
// careerYears/5 is integer division on purpose: one point per full
// five years, so 9 years give 1 point, not 1.8.
// Negative inputs are treated as 0 so no part is ever negative.
func Explain(tourSpread, concertCount, careerYears int) Breakdown {
	tourSpread = max(tourSpread, 0)
	concertCount = max(concertCount, 0)
	careerYears = max(careerYears, 0)

	return Breakdown{
		Countries: tourSpread * countryWeight,
		Concerts:  concertCount * concertWeight,
		Career:    min(maxCareerPoints, careerYears/yearsPerPoint),
	}
}

// Score computes the activity score. See Explain for the formula.
func Score(tourSpread, concertCount, careerYears int) int {
	return Explain(tourSpread, concertCount, careerYears).Total()
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
