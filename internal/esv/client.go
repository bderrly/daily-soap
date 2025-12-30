package esv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	esvAPIKey = "YOUR_KEY"
)

// PassageMeta represents the metadata for a passage.
type PassageMeta struct {
	Canonical    string `json:"canonical"`
	ChapterStart []int  `json:"chapter_start"`
	ChapterEnd   []int  `json:"chapter_end"`
	PrevVerse    int    `json:"prev_verse"`
	NextVerse    int    `json:"next_verse"`
	PrevChapter  []int  `json:"prev_chapter"`
	NextChapter  []int  `json:"next_chapter"`
}

// EsvResponse represents the response structure from the ESV API.
type EsvResponse struct {
	Query       string        `json:"query"`
	PassageMeta []PassageMeta `json:"passage_meta"`
	Passages    []string      `json:"passages"`
	Copyright   string        `json:"copyright"`
}

// FetchVerses fetches verses from the ESV API.
func FetchVerses(references []string) (EsvResponse, error) {
	apiURL := "https://api.esv.org/v3/passage/html/"
	params := url.Values{}
	params.Add("q", strings.Join(references, ";"))
	params.Add("include-audio-link", "false")
	params.Add("wrapping-div", "true")
	apiURL += "?" + params.Encode()

	var apiResp EsvResponse

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return apiResp, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Token %s", esvAPIKey))

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return apiResp, fmt.Errorf("failed to fetch verse: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return apiResp, fmt.Errorf("ESV API returned status %d", resp.StatusCode)
	}

	// Decode the JSON response
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return apiResp, fmt.Errorf("failed to decode response: %w", err)
	}

	return apiResp, nil
}
