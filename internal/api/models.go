package api

// Artist describes a band or artist returned by the /artists endpoint.
// The API also returns URLs to locations, dates and relations; they are
// intentionally omitted because concert data is loaded from /relation.
type Artist struct {
	ID           int      `json:"id"`
	Image        string   `json:"image"`
	Name         string   `json:"name"`
	Members      []string `json:"members"`
	CreationDate int      `json:"creationDate"`
	FirstAlbum   string   `json:"firstAlbum"`
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
