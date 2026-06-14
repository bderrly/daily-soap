package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bderrly/daily-soap/internal/dailytexts"
	"github.com/bderrly/daily-soap/internal/email"
	"github.com/bderrly/daily-soap/internal/esv"
	"github.com/bderrly/daily-soap/internal/export"
	"github.com/bderrly/daily-soap/internal/store"
)

func handleIndex(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*store.User)

	// Only handle root path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get date from query parameter, default to today
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		// Get current date in YYYY-MM-DD format based on user location
		loc, err := time.LoadLocation(user.Timezone)
		if err != nil {
			slog.Error("failed to load user location", "timezone", user.Timezone, "error", err)
			loc = time.UTC
		}
		dateStr = time.Now().In(loc).Format(time.DateOnly)
	}

	// Get data for the requested date (will load year file if needed)
	dailyText, err := dailytexts.GetDailyText(dateStr)
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

	// Fetch verse content from ESV API (using cache)
	verseContents, err := fetchPassagesWithCache(r.Context(), dailyText.Verses)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error loading verses for %s", dateStr), http.StatusInternalServerError)
		return
	}

	// Load existing SOAP data from database
	soapData, err := appStore.GetSOAPData(r.Context(), user.ID, dateStr)
	if err != nil {
		slog.Warn("failed to load SOAP data", "date", dateStr, "error", err)
		// Continue with empty values if there's an error
		soapData = &store.SOAPData{
			Date:           dateStr,
			Observation:    "",
			Application:    "",
			Prayer:         "",
			SelectedVerses: []string{},
		}
	}

	// Prepare template data
	data := map[string]any{
		"esvData":        verseContents,
		"date":           dateStr,
		"observation":    soapData.Observation,
		"application":    soapData.Application,
		"prayer":         soapData.Prayer,
		"selectedVerses": soapData.SelectedVerses,
		"user":           user,
		"CSRFToken":      r.Context().Value(csrfContextKey).(string),
		"Nonce":          r.Context().Value(nonceContextKey).(string),
	}

	// Execute template
	templateName := "index.html"
	if r.Header.Get("HX-Request") == "true" {
		templateName = "content.gotmpl"
	}

	if err := tmpl.ExecuteTemplate(w, templateName, data); err != nil {
		slog.Error("failed to execute template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleReading handles requests for the verses partial template (for HTMX).
// Accepts a "date" query parameter (YYYY-MM-DD format). Defaults to today if not provided.
func handleReading(w http.ResponseWriter, r *http.Request) {
	// Get date from query parameter, default to today
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		// Use user's timezone for default date
		loc := time.UTC
		if user, ok := r.Context().Value(userContextKey).(*store.User); ok {
			if l, err := time.LoadLocation(user.Timezone); err == nil {
				loc = l
			}
		}
		dateStr = time.Now().In(loc).Format(time.DateOnly)
	}

	// Get daily text for the requested date
	dailyText, err := dailytexts.GetDailyText(dateStr)
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

	// Fetch verse content from ESV API (using cache)
	verseContents, err := fetchPassagesWithCache(r.Context(), dailyText.Verses)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching verses for %s", dateStr), http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := map[string]any{
		"esvData": verseContents,
		"date":    dateStr,
	}

	// Execute only the verses template
	if err := tmpl.ExecuteTemplate(w, "verses.gotmpl", data); err != nil {
		slog.Error("failed to execute verses template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleSOAP handles GET and POST requests for SOAP data.
func handleSOAP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleGetSOAP(w, r)
	case http.MethodPost:
		handlePostSOAP(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetSOAP retrieves SOAP data for a given date.
func handleGetSOAP(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*store.User)
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format(time.DateOnly)
	}

	soapData, err := appStore.GetSOAPData(r.Context(), user.ID, dateStr)
	if err != nil {
		slog.Error("failed to get SOAP data", "date", dateStr, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(soapData); err != nil {
		slog.Error("failed to encode SOAP data", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handlePostSOAP saves SOAP data.
func handlePostSOAP(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*store.User)

	var soapData store.SOAPData
	if err := json.NewDecoder(r.Body).Decode(&soapData); err != nil {
		slog.Error("failed to decode SOAP data", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := appStore.SaveSOAPData(r.Context(), user.ID, &soapData); err != nil {
		slog.Error("failed to save SOAP data", "error", err)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save data"}); err != nil {
			slog.Error("failed to encode error response", "error", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
		slog.Error("failed to encode success response", "error", err)
	}
}

type exportRequest struct {
	Date       string   `json:"date"`
	Format     string   `json:"format"`     // html or markdown
	Method     string   `json:"method"`     // download or email
	Recipients []string `json:"recipients"` // only for method=email
}

// handleExport handles SOAP journal export requests.
func handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req exportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode export request", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	user := r.Context().Value(userContextKey).(*store.User)

	// Fetch SOAP data via appStore.GetSOAPData(r.Context(), user.ID, req.Date)
	soapData, err := appStore.GetSOAPData(r.Context(), user.ID, req.Date)
	if err != nil {
		slog.Error("failed to get SOAP data for export", "date", req.Date, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Fetch Scripture content
	// Get daily text via dailytexts.GetDailyText(req.Date)
	dailyText, err := dailytexts.GetDailyText(req.Date)
	if err != nil {
		slog.Error("failed to get daily text for export", "date", req.Date, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if dailyText == nil {
		slog.Warn("no data found for date", "date", req.Date)
		http.Error(w, fmt.Sprintf("No data found for date: %s", req.Date), http.StatusNotFound)
		return
	}

	// Fetch verse content from ESV API (using cache)
	references := dailyText.Verses
	if len(soapData.SelectedVerses) > 0 {
		references = []string{esv.FormatReferences(soapData.SelectedVerses)}
	}
	verseContents, err := fetchPassagesWithCache(r.Context(), references)
	if err != nil {
		slog.Error("failed to fetch verses for export", "date", req.Date, "error", err)
		http.Error(w, fmt.Sprintf("Error loading verses for %s", req.Date), http.StatusInternalServerError)
		return
	}

	scriptureHTML := strings.Join(verseContents.Passages, "\n")

	// Email Logic:
	if req.Method == "email" {
		// Only allow format: html
		if req.Format != "html" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(map[string]string{"error": "Email export only supports HTML format"}); err != nil {
				slog.Error("failed to encode error response", "error", err)
			}
			return
		}

		exporter, err := export.NewHTMLExporter()
		if err != nil {
			slog.Error("failed to create HTML exporter", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		var buf bytes.Buffer
		if err := exporter.Export(r.Context(), &buf, soapData, scriptureHTML); err != nil {
			slog.Error("failed to export HTML for email", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Call email.QueueExportEmail(r.Context(), appStore, user, req.Date, req.Recipients, htmlContent)
		if err := email.QueueExportEmail(r.Context(), appStore, user, req.Date, req.Recipients, buf.String()); err != nil {
			slog.Error("failed to queue export email", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Return 202 Accepted with JSON {"status": "queued"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "queued"}); err != nil {
			slog.Error("failed to encode success response", "error", err)
		}
		return
	}

	// Download Logic:
	var exporter export.Exporter
	var filename string
	if req.Format == "markdown" {
		exporter, err = export.NewMarkdownExporter()
		filename = fmt.Sprintf("soap-%s.md", req.Date)
	} else {
		exporter, err = export.NewHTMLExporter()
		filename = fmt.Sprintf("soap-%s.html", req.Date)
	}

	if err != nil {
		slog.Error("failed to create exporter", "format", req.Format, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set Content-Type and Content-Disposition
	w.Header().Set("Content-Type", exporter.ContentType())
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Write generated content to w
	if err := exporter.Export(r.Context(), w, soapData, scriptureHTML); err != nil {
		slog.Error("failed to export content for download", "error", err)
		// Note: headers already sent, can't change status code easily
	}
}

// HistoryEntry represents a single SOAP journal entry in the history view.
type HistoryEntry struct {
	Date         string
	Observation  string
	Application  string
	Prayer       string
	PassagesHTML []template.HTML
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*store.User)
	loc, err := time.LoadLocation(user.Timezone)
	if err != nil {
		loc = time.UTC
	}

	endDateStr := r.URL.Query().Get("end_date")
	if endDateStr == "" {
		endDateStr = time.Now().In(loc).Format(time.DateOnly)
	}

	endDate, err := time.Parse(time.DateOnly, endDateStr)
	if err != nil {
		endDate = time.Now().In(loc)
		endDateStr = endDate.Format(time.DateOnly)
	}

	daysStr := r.URL.Query().Get("days")
	days := 7
	switch daysStr {
	case "14":
		days = 14
	case "30":
		days = 30
	}

	startDate := endDate.AddDate(0, 0, -(days - 1))
	startDateStr := startDate.Format(time.DateOnly)

	nextEndDate := endDate.AddDate(0, 0, days)
	if nextEndDate.After(time.Now().In(loc)) {
		nextEndDate = time.Now().In(loc)
	}
	prevEndDate := endDate.AddDate(0, 0, -days)

	entries, err := appStore.GetSOAPDataRange(r.Context(), user.ID, startDateStr, endDateStr)
	if err != nil {
		slog.Error("failed to get history data", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var historyEntries []HistoryEntry
	for _, entry := range entries {
		var htmlPassages []template.HTML

		if len(entry.SelectedVerses) > 0 {
			references := []string{esv.FormatReferences(entry.SelectedVerses)}
			esvRes, err := fetchPassagesWithCache(r.Context(), references)
			if err != nil {
				slog.Error("failed to fetch verses for history", "date", entry.Date, "error", err)
			} else {
				for _, p := range esvRes.Passages {
					htmlPassages = append(htmlPassages, template.HTML(p)) // #nosec G203
				}
			}
		}

		historyEntries = append(historyEntries, HistoryEntry{
			Date:         entry.Date,
			Observation:  entry.Observation,
			Application:  entry.Application,
			Prayer:       entry.Prayer,
			PassagesHTML: htmlPassages,
		})
	}

	data := map[string]any{
		"Entries":     historyEntries,
		"Days":        days,
		"EndDate":     endDateStr,
		"StartDate":   startDateStr,
		"PrevEndDate": prevEndDate.Format(time.DateOnly),
		"NextEndDate": nextEndDate.Format(time.DateOnly),
		"ShowNext":    nextEndDate.After(endDate) || endDate.Format(time.DateOnly) != time.Now().In(loc).Format(time.DateOnly),
		"User":        user,
		"CSRFToken":   r.Context().Value(csrfContextKey).(string),
		"Nonce":       r.Context().Value(nonceContextKey).(string),
	}

	if err := tmpl.ExecuteTemplate(w, "history.html", data); err != nil {
		slog.Error("failed to execute history template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
