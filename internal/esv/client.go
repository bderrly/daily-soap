package esv

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
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

// FetchPassages fetches verses from the ESV API.
func FetchPassages(references []string) (EsvResponse, error) {
	// See https://api.esv.org/docs/passage-html/ for API documentation.
	apiURL := "https://api.esv.org/v3/passage/html/"
	params := url.Values{}
	params.Add("q", strings.Join(references, ";"))
	params.Add("include-audio-link", "false")
	params.Add("include-footnotes", "false")
	params.Add("include-first-verse-numbers", "false")
	apiURL += "?" + params.Encode()

	var apiResp EsvResponse

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return apiResp, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Token %s", os.Getenv("ESV_API_KEY")))

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	slog.Debug("fetching verses", "references", references, "apiURL", apiURL)
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

	// Post-process the HTML to wrap verses in selectable spans
	for i, p := range apiResp.Passages {
		processed, err := processPassageHTML(p)
		if err != nil {
			// Getting partial functionality (original HTML) is better than breaking everything.
			slog.Error("error processing passage HTML", "error", err)
			continue
		}
		apiResp.Passages[i] = processed
	}

	return apiResp, nil
}
