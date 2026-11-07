package api

// Artist describes a band or artist returned by the /artists endpoint.
// The API also returns URLs to the artist's locations, dates and relation;
// they are omitted because all four endpoints are loaded in full instead.
type Artist struct {
	ID           int      `json:"id"`
	Image        string   `json:"image"`
	Name         string   `json:"name"`
	Members      []string `json:"members"`
	CreationDate int      `json:"creationDate"`
	FirstAlbum   string   `json:"firstAlbum"`
}

// Location lists the places where an artist (by ID) played, from the
// /locations endpoint. A place may appear twice when the artist played
// there on two separate tours.
type Location struct {
	ID        int      `json:"id"`
	Locations []string `json:"locations"`
}

// locationsResponse mirrors the /locations payload: {"index": [...]}.
type locationsResponse struct {
	Index []Location `json:"index"`
}

// Dates lists the concert dates of an artist (by ID), from the /dates
// endpoint. A date starting with "*" opens the group of dates that belongs
// to the next entry of Location.Locations.
type Dates struct {
	ID    int      `json:"id"`
	Dates []string `json:"dates"`
}

// datesResponse mirrors the /dates payload: {"index": [...]}.
type datesResponse struct {
	Index []Dates `json:"index"`
}

// Relation links an artist (by ID) to their concerts.
// DatesLocations maps a location key such as "london-uk" to the list of
// concert dates in "DD-MM-YYYY" format.
type Relation struct {
	ID             int                 `json:"id"`
	DatesLocations map[string][]string `json:"datesLocations"`
}

// relationResponse mirrors the /relation payload, which wraps the list
// of relations in an "index" field: {"index": [...]}.
type relationResponse struct {
	Index []Relation `json:"index"`
}
