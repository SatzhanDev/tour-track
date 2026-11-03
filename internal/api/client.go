package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DefaultBaseURL is the root of the public Groupie Trackers API.
const DefaultBaseURL = "https://groupietrackers.01edu.ai/api"

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

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
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

// FetchRelations returns the concert data of every artist from the
// /relation endpoint. The /locations and /dates endpoints are not used
// because /relation already contains the same data, already linked.
func (c *Client) FetchRelations() ([]Relation, error) {
	var relations relationResponse
	err := c.getJSON("/relation", &relations)
	if err != nil {
		return nil, err
	}
	return relations.Index, nil
}
