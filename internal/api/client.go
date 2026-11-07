package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultBaseURL is the root of the public Groupie Trackers API.
const DefaultBaseURL = "https://groupietrackers.01edu.ai/api"

// maxBodySize limits how much of a response is read, so a broken or
// malicious API cannot make the server use unlimited memory. Real answers
// are well below 1 MB.
const maxBodySize = 10 << 20 // 10 MB

// Client fetches artist and concert data from the API.
// Create it with NewClient; the zero value is not usable.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a Client that sends requests to baseURL.
// An empty baseURL falls back to DefaultBaseURL. Every request is limited
// by a timeout so a slow or hanging API can never block the server forever.
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// getJSON sends a GET request to baseURL+path and decodes the JSON body
// into target, which must be a pointer. It returns an error if the request
// fails, the status is not 200 OK, or the body is not valid JSON.
func (c *Client) getJSON(path string, target any) error {
	url := c.baseURL + path
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: unexpected status %d", path, resp.StatusCode)

	}

	body := io.LimitReader(resp.Body, maxBodySize)
	if err := json.NewDecoder(body).Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}

// FetchArtists returns all artists from the /artists endpoint.
func (c *Client) FetchArtists() ([]Artist, error) {
	var artists []Artist
	err := c.getJSON("/artists", &artists)
	if err != nil {
		return nil, err
	}
	return artists, nil
}

// FetchLocations returns the concert locations of every artist from the
// /locations endpoint.
func (c *Client) FetchLocations() ([]Location, error) {
	var resp locationsResponse
	if err := c.getJSON("/locations", &resp); err != nil {
		return nil, err
	}
	return resp.Index, nil
}

// FetchDates returns the concert dates of every artist from the /dates
// endpoint.
func (c *Client) FetchDates() ([]Dates, error) {
	var resp datesResponse
	if err := c.getJSON("/dates", &resp); err != nil {
		return nil, err
	}
	return resp.Index, nil
}

// FetchRelations returns, for every artist, the link between each location
// and its dates, from the /relation endpoint.
func (c *Client) FetchRelations() ([]Relation, error) {
	var relations relationResponse
	err := c.getJSON("/relation", &relations)
	if err != nil {
		return nil, err
	}
	return relations.Index, nil
}
