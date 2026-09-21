package apibible

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Supported translation IDs for API.Bible.
const (
	TranslationNLT = "d6e14a625393b4da-01"
	TranslationMSG = "6f11a7de016f942e-01"
)

// Shorthand returns the short-hand translation abbreviation for a Bible ID or translation name.
func Shorthand(bibleID string) string {
	switch strings.ToLower(strings.TrimSpace(bibleID)) {
	case TranslationNLT, "nlt":
		return "NLT"
	case TranslationMSG, "msg":
		return "MSG"
	case "esv":
		return "ESV"
	default:
		return strings.ToUpper(strings.TrimSpace(bibleID))
	}
}

// Common API.Bible error codes.
var (
	ErrBadRequest   = errors.New("bad request (invalid bible or passage ID)")
	ErrUnauthorized = errors.New("unauthorized (missing or invalid API token)")
	ErrForbidden    = errors.New("forbidden (not authorized to access this Bible)")
	ErrNotFound     = errors.New("not found (unable to find Bible or passage)")
)

// APIError represents an error returned by the API.Bible REST API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API.Bible returned status %d: %s", e.StatusCode, e.Message)
}

// Response represents the combined passages and analytics data from API.Bible.
type Response struct {
	Query      string   `json:"query"`
	Passages   []string `json:"passages"`
	Copyright  string   `json:"copyright"`
	FUMSTokens []string `json:"fums_tokens"`
}

// Client is an HTTP client for API.Bible.
type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new API.Bible client.
func NewClient(apiKey string) *Client {
	if apiKey == "" {
		apiKey = os.Getenv("API_BIBLE_KEY")
	}
	return &Client{
		APIKey:     apiKey,
		BaseURL:    "https://api.scripture.api.bible/v1",
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type passageAPIResponse struct {
	Data struct {
		ID         string `json:"id"`
		BibleID    string `json:"bibleId"`
		Reference  string `json:"reference"`
		Content    string `json:"content"`
		VerseCount int    `json:"verseCount"`
		Copyright  string `json:"copyright"`
	} `json:"data"`
	Meta struct {
		FUMSToken string `json:"fumsToken"`
	} `json:"meta"`
	StatusCode int    `json:"statusCode"`
	Error      string `json:"error"`
	Message    string `json:"message"`
}

// FetchPassages retrieves and transforms scripture for the given references from API.Bible.
func (c *Client) FetchPassages(ctx context.Context, bibleID string, references []string) (Response, error) {
	var resp Response
	resp.Query = strings.Join(references, ";")

	if c.APIKey == "" {
		c.APIKey = os.Getenv("API_BIBLE_KEY")
	}

	for _, ref := range references {
		passageIDs, err := ParseReference(ref)
		if err != nil {
			return resp, fmt.Errorf("parsing reference %q: %w", ref, err)
		}

		for _, pid := range passageIDs {
			var endpointPath string
			switch {
			case strings.Contains(pid, "-"):
				endpointPath = "passages/" + pid
			case strings.Count(pid, ".") == 1:
				endpointPath = "chapters/" + pid
			case strings.Count(pid, ".") == 2:
				endpointPath = "verses/" + pid
			default:
				endpointPath = "passages/" + pid
			}

			apiURL := fmt.Sprintf("%s/bibles/%s/%s?fums-version=3&content-type=html&include-notes=false&include-titles=true&include-chapter-numbers=false&include-verse-numbers=true",
				c.BaseURL, bibleID, endpointPath)

			passageData, err := func() (*passageAPIResponse, error) {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
				if err != nil {
					return nil, fmt.Errorf("creating request: %w", err)
				}

				// Authentication header per API.Bible specification
				req.Header.Set("api-key", c.APIKey)

				slog.Debug("fetching scripture from API.Bible", "endpoint", endpointPath, "bibleID", bibleID)
				httpResp, err := c.HTTPClient.Do(req)
				if err != nil {
					return nil, fmt.Errorf("calling API.Bible: %w", err)
				}
				defer func() {
					_ = httpResp.Body.Close()
				}()

				if httpResp.StatusCode != http.StatusOK {
					switch httpResp.StatusCode {
					case http.StatusBadRequest:
						return nil, ErrBadRequest
					case http.StatusUnauthorized:
						return nil, ErrUnauthorized
					case http.StatusForbidden:
						return nil, ErrForbidden
					case http.StatusNotFound:
						return nil, ErrNotFound
					default:
						return nil, &APIError{StatusCode: httpResp.StatusCode, Message: httpResp.Status}
					}
				}

				var data passageAPIResponse
				if err := json.NewDecoder(httpResp.Body).Decode(&data); err != nil {
					return nil, fmt.Errorf("decoding API.Bible response: %w", err)
				}
				return &data, nil
			}()
			if err != nil {
				return resp, err
			}

			bookNum, chapNum := extractBookAndChapter(pid)
			processedHTML, err := ProcessPassageHTML(passageData.Data.Content, bookNum, chapNum)
			if err != nil {
				slog.Error("error processing API.Bible HTML", "error", err)
				processedHTML = passageData.Data.Content
			}

			if passageData.Data.Reference != "" && !strings.Contains(processedHTML, "<h2") {
				shorthand := Shorthand(bibleID)
				processedHTML = fmt.Sprintf("<h2 class=\"extra_text\">%s (%s)</h2>\n%s", passageData.Data.Reference, shorthand, processedHTML)
			}

			resp.Passages = append(resp.Passages, processedHTML)
			if passageData.Data.Copyright != "" {
				resp.Copyright = passageData.Data.Copyright
			}
			if passageData.Meta.FUMSToken != "" {
				resp.FUMSTokens = append(resp.FUMSTokens, passageData.Meta.FUMSToken)
			}
		}
	}

	return resp, nil
}

// FetchPassages is a package-level helper that uses the default client.
func FetchPassages(ctx context.Context, bibleID string, references []string) (Response, error) {
	client := NewClient("")
	return client.FetchPassages(ctx, bibleID, references)
}

func extractBookAndChapter(passageID string) (int, int) {
	parts := strings.Split(passageID, "-")
	start := parts[0]
	sub := strings.Split(start, ".")
	if len(sub) >= 2 {
		if b, ok := USFMToBookInfo(sub[0]); ok {
			chap, _ := strconv.Atoi(sub[1])
			return b.Number, chap
		}
	}
	return 0, 0
}
