package acoustid

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const apiURL = "https://api.acoustid.org/v2/lookup"

// Client is an AcoustID API client.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// LookupResult represents a single match from the AcoustID API.
type LookupResult struct {
	Score      float64     `json:"score"`
	ID         string      `json:"id"`
	Recordings []Recording `json:"recordings"`
}

// Recording represents a MusicBrainz recording.
type Recording struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Artists  []Artist  `json:"artists"`
	Releases []Release `json:"releases"`
}

// Artist represents a MusicBrainz artist.
type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Release represents a MusicBrainz release (album).
type Release struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// NewClient creates a new AcoustID API client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// apiResponse is the raw JSON response from AcoustID.
type apiResponse struct {
	Status  string         `json:"status"`
	Error   *apiError      `json:"error,omitempty"`
	Results []LookupResult `json:"results"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Lookup queries the AcoustID API with a fingerprint and duration.
func (c *Client) Lookup(ctx context.Context, fingerprint string, duration int) ([]LookupResult, error) {
	params := url.Values{}
	params.Set("client", c.apiKey)
	params.Set("fingerprint", fingerprint)
	params.Set("duration", strconv.Itoa(duration))
	params.Set("meta", "recordings releases tracks")
	params.Set("format", "json")

	reqURL := apiURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if apiResp.Status != "ok" {
		msg := "unknown error"
		if apiResp.Error != nil {
			msg = apiResp.Error.Message
		}
		return nil, fmt.Errorf("API error: %s", msg)
	}

	return apiResp.Results, nil
}
