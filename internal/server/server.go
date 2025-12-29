package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var (
	// Cache of loaded year data, keyed by year (e.g., "2025", "2026")
	yearDataCache = make(map[string]Year)
	cacheMutex    sync.RWMutex
	tmpl          *template.Template

	// TODO: Paste your ESV API token here
	esvAPIKey = "YOUR_KEY"
)

// esvAPIResponse represents the response structure from the ESV API
type esvAPIResponse struct {
	Passages  []string `json:"passages"`
	Copyright string   `json:"copyright"`
}

func init() {
	// Load current year data
	currentYear := time.Now().Format("2006")
	if err := loadYearData(currentYear); err != nil {
		slog.Error("failed to load year data", "year", currentYear, "error", err)
	}

	// Parse templates with function map for safe HTML rendering
	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
	}
	var err error
	tmpl, err = template.New("").Funcs(funcMap).ParseFS(web, "web/*.html", "web/*.gotmpl")
	if err != nil {
		slog.Error("failed to parse template", "error", err)
		// Create a minimal template to prevent nil pointer errors
		tmpl = template.Must(template.New("error").Parse("<html><body><h1>Template Error</h1></body></html>"))
	}
}

func Muxer() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/verses", handleVerses)

	// Create a subdirectory filesystem for the web directory
	webFS, err := fs.Sub(web, "web")
	if err != nil {
		slog.Error("failed to create web subdirectory filesystem", "error", err)
	} else {
		mux.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.FS(webFS))))
	}

	return mux
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	// Only handle root path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get current date in YYYY-MM-DD format
	today := time.Now().Format("2006-01-02")

	// Get today's data (will load year file if needed)
	dailyText, err := getDailyText(today)
	if err != nil {
		slog.Error("failed to get daily text", "date", today, "error", err)
		http.Error(w, fmt.Sprintf("Error loading data for date: %s", today), http.StatusInternalServerError)
		return
	}

	if dailyText == nil {
		slog.Warn("no data found for date", "date", today)
		http.Error(w, fmt.Sprintf("No data found for date: %s", today), http.StatusNotFound)
		return
	}

	// Fetch verse content from ESV API
	verseContents := fetchVersesContent(dailyText.Verses)

	// Prepare template data
	data := map[string]any{
		"verses": verseContents,
		"date":   today,
	}

	// Execute template
	if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		slog.Error("failed to execute template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleVerses handles requests for the verses partial template (for HTMX).
// Accepts a "date" query parameter (YYYY-MM-DD format). Defaults to today if not provided.
func handleVerses(w http.ResponseWriter, r *http.Request) {
	// Get date from query parameter, default to today
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	// Get daily text for the requested date
	dailyText, err := getDailyText(dateStr)
	if err != nil {
		slog.Error("failed to get daily text", "date", dateStr, "error", err)
		http.Error(w, fmt.Sprintf("Error loading data for date: %s", dateStr), http.StatusInternalServerError)
		return
	}

	if dailyText == nil {
		slog.Warn("no data found for date", "date", dateStr)
		http.Error(w, fmt.Sprintf("No data found for date: %s", dateStr), http.StatusNotFound)
		return
	}

	// Fetch verse content from ESV API
	verseContents := fetchVersesContent(dailyText.Verses)

	// Prepare template data
	data := map[string]any{
		"verses": verseContents,
		"date":   dateStr,
	}

	// Execute only the verses template
	if err := tmpl.ExecuteTemplate(w, "verses.gotmpl", data); err != nil {
		slog.Error("failed to execute verses template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getDailyText retrieves the daily text for a given date (YYYY-MM-DD format).
// It will automatically load the year file if it hasn't been loaded yet.
func getDailyText(dateStr string) (*DailyText, error) {
	// Extract year from date string (first 4 characters)
	if len(dateStr) < 4 {
		return nil, fmt.Errorf("invalid date format: %s", dateStr)
	}
	year := dateStr[:4]

	// Check if year data is already loaded
	cacheMutex.RLock()
	yearData, ok := yearDataCache[year]
	cacheMutex.RUnlock()

	if !ok {
		// Load year data if not in cache
		if err := loadYearData(year); err != nil {
			return nil, fmt.Errorf("failed to load year data for %s: %w", year, err)
		}
		cacheMutex.RLock()
		yearData = yearDataCache[year]
		cacheMutex.RUnlock()
	}

	// Get the daily text for the date
	dailyText, ok := yearData[dateStr]
	if !ok {
		return nil, nil // Date not found, but not an error
	}

	return &dailyText, nil
}

// loadYearData loads the JSON data for the specified year.
// The year should be in format "YYYY" (e.g., "2025", "2026").
func loadYearData(year string) error {
	// Check if already loaded
	cacheMutex.RLock()
	if _, ok := yearDataCache[year]; ok {
		cacheMutex.RUnlock()
		return nil // Already loaded
	}
	cacheMutex.RUnlock()

	// Read the year file
	filename := fmt.Sprintf("web/%s.json", year)
	data, err := web.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filename, err)
	}

	// Unmarshal JSON
	var yearData Year
	if err := json.Unmarshal(data, &yearData); err != nil {
		return fmt.Errorf("failed to unmarshal JSON from %s: %w", filename, err)
	}

	// Store in cache
	cacheMutex.Lock()
	yearDataCache[year] = yearData
	cacheMutex.Unlock()

	slog.Info("loaded year data", "year", year)
	return nil
}

// fetchVerseFromESV fetches verse HTML content and copyright from the ESV API
func fetchVerseFromESV(reference string) (*VerseContent, error) {
	if esvAPIKey == "" {
		return nil, fmt.Errorf("ESV API key not configured")
	}

	// Build the API URL
	apiURL := "https://api.esv.org/v3/passage/html/"
	params := url.Values{}
	params.Add("q", reference)
	params.Add("include-audio-link", "false")
	apiURL += "?" + params.Encode()

	// Create the request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add the Authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Token %s", esvAPIKey))

	// Make the request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch verse: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ESV API returned status %d", resp.StatusCode)
	}

	// Decode the JSON response
	var apiResp esvAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract the HTML content (usually the first passage)
	var htmlContent string
	if len(apiResp.Passages) > 0 {
		htmlContent = apiResp.Passages[0]
	}

	return &VerseContent{
		Reference: reference,
		HTML:      htmlContent,
		Copyright: apiResp.Copyright,
	}, nil
}

// fetchVersesContent fetches verse content for all verse references
func fetchVersesContent(references []string) []VerseContent {
	var verses []VerseContent
	var copyright string // We'll use the copyright from the last verse (they should all be the same)

	for _, ref := range references {
		verse, err := fetchVerseFromESV(ref)
		if err != nil {
			slog.Error("failed to fetch verse", "reference", ref, "error", err)
			// Continue with other verses even if one fails
			verses = append(verses, VerseContent{
				Reference: ref,
				HTML:      fmt.Sprintf("<p>Error loading verse: %s</p>", err.Error()),
				Copyright: "",
			})
			continue
		}
		verses = append(verses, *verse)
		if verse.Copyright != "" {
			copyright = verse.Copyright
		}
	}

	// Set copyright for all verses (in case some were missing)
	for i := range verses {
		if verses[i].Copyright == "" {
			verses[i].Copyright = copyright
		}
	}

	return verses
}
