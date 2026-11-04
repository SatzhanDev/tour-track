package store

import (
	"testing"
	"time"
)

func TestParseLocation(t *testing.T) {
	tests := []struct {
		raw         string
		wantCity    string
		wantCountry string
	}{
		{"london-uk", "London", "UK"},
		{"playa_del_carmen-mexico", "Playa Del Carmen", "Mexico"},
		{"north_carolina-usa", "North Carolina", "USA"},
		{"dunedin-new_zealand", "Dunedin", "New Zealand"},
		// Alias spellings from the API are normalized.
		{"new_york-us", "New York", "USA"},
		{"sao_paulo-brasil", "Sao Paulo", "Brazil"},
		{"manila-philippine", "Manila", "Philippines"},
		{"prague-czech_republic", "Prague", "Czechia"},
		{"seoul-korea", "Seoul", "South Korea"},
		// Robustness: odd input never panics.
		{"  Osaka-JAPAN ", "Osaka", "Japan"},
		{"nowhere", "Nowhere", ""},
		{"", "", ""},
		{"-uk", "", "UK"},
		{"paris-", "Paris", ""},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			city, country := ParseLocation(tt.raw)
			if city != tt.wantCity || country != tt.wantCountry {
				t.Errorf("ParseLocation(%q) = (%q, %q), want (%q, %q)",
					tt.raw, city, country, tt.wantCity, tt.wantCountry)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	valid := []struct {
		raw  string
		want time.Time
	}{
		{"23-08-2019", time.Date(2019, time.August, 23, 0, 0, 0, 0, time.UTC)},
		{"*05-12-2019", time.Date(2019, time.December, 5, 0, 0, 0, 0, time.UTC)},
		{" 01-01-2020 ", time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range valid {
		got, err := ParseDate(tt.raw)
		if err != nil {
			t.Errorf("ParseDate(%q) unexpected error: %v", tt.raw, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("ParseDate(%q) = %v, want %v", tt.raw, got, tt.want)
		}
	}

	invalid := []string{
		"",
		"32-01-2020", // no such day
		"23-13-2019", // no such month
		"2019-08-23", // wrong order
		"23/08/2019", // wrong separator
		"*",
	}
	for _, raw := range invalid {
		if _, err := ParseDate(raw); err == nil {
			t.Errorf("ParseDate(%q) expected error, got nil", raw)
		}
	}
}
