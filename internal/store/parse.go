package store

import (
	"strings"
	"time"
	"unicode"
)

// dateLayout is the API date format DD-MM-YYYY written in Go's reference
// time notation (Mon Jan 2 15:04:05 MST 2006): 02 = day, 01 = month,
// 2006 = year.
const dateLayout = "02-01-2006"

// countryAliases maps alternative spellings found in the API to a single
// canonical key, so one country is never counted twice in tour_spread.
// The list was built by inspecting the API data: for example, artist 25
// has both "new_york-us" and "boston-usa".
var countryAliases = map[string]string{
	"us":             "usa",
	"brasil":         "brazil",
	"philippine":     "philippines",
	"czech_republic": "czechia",
	"korea":          "south_korea",
}

// acronyms are country keys shown in upper case instead of title case.
var acronyms = map[string]bool{
	"usa": true,
	"uk":  true,
}

// ParseLocation splits a raw API location such as "playa_del_carmen-mexico"
// into a display-ready city and country ("Playa Del Carmen", "Mexico").
//
// The country is the part after the last "-"; if there is no "-", the whole
// string is treated as the city and country is empty. Alias spellings are
// normalized (see countryAliases). Entries that look wrong, such as
// "glasgow-australia", are kept as is: guessing would hide data issues.
func ParseLocation(raw string) (city, country string) {
	raw = strings.ToLower(strings.TrimSpace(raw))

	i := strings.LastIndex(raw, "-")
	if i < 0 {
		return formatName(raw), ""
	}

	cityKey, countryKey := raw[:i], raw[i+1:]
	if canonical, ok := countryAliases[countryKey]; ok {
		countryKey = canonical
	}
	if acronyms[countryKey] {
		return formatName(cityKey), strings.ToUpper(countryKey)
	}
	return formatName(cityKey), formatName(countryKey)
}

// formatName turns an API key such as "new_zealand" into "New Zealand".
func formatName(key string) string {
	words := strings.Fields(strings.ReplaceAll(key, "_", " "))
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// ParseDate parses a concert date in DD-MM-YYYY format. A leading "*",
// which the /dates endpoint uses as a group marker, is ignored.
func ParseDate(raw string) (time.Time, error) {
	return time.Parse(dateLayout, strings.TrimPrefix(strings.TrimSpace(raw), "*"))
}
