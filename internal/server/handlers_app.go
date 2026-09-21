package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/bderrly/daily-soap/internal/dailytexts"
	"github.com/bderrly/daily-soap/internal/email"
	"github.com/bderrly/daily-soap/internal/esv"
	"github.com/bderrly/daily-soap/internal/export"
	"github.com/bderrly/daily-soap/internal/store"
)

var h2Regex = regexp.MustCompile(`(?i)(<h2 class="extra_text">)(.*?)(</h2>)`)

func normalizeTranslation(t string) string {
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case "NLT", "D6E14A625393B4DA-01":
		return "NLT"
	case "MSG", "6F11A7DE016F942E-01", "THE MESSAGE":
		return "MSG"
	case "ESV":
		return "ESV"
	default:
		if t != "" {
			return t
		}
		return "ESV"
	}
}

func (app *application) home(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*store.User)

	// Only handle root path.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get date from query parameter, default to today.
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		// Get current date in YYYY-MM-DD format based on user location.
		loc, err := time.LoadLocation(user.Timezone)
		if err != nil {
			slog.Error("failed to load user location", "timezone", user.Timezone, "error", err)
			loc = time.UTC
		}
		dateStr = time.Now().In(loc).Format(time.DateOnly)

		if !isHTMXRequest(r) {
			redirURL := "/?date=" + dateStr
			http.Redirect(w, r, redirURL, http.StatusFound)
			return
		}
	}

	// Get data for the requested date (will load year file if needed).
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

	// Load existing SOAP data from database.
	soapData, err := app.store.GetSOAPData(r.Context(), user.ID, dateStr)
	if err != nil {
		slog.Warn("failed to load SOAP data", "date", dateStr, "error", err)
		// Continue with empty values if there's an error.
		soapData = &store.SOAPData{
			Date:           dateStr,
			Observation:    "",
			Application:    "",
			Prayer:         "",
			SelectedVerses: []string{},
			Translation:    normalizeTranslation(user.Translation),
		}
	}

	activeTranslation := soapData.Translation
	if activeTranslation == "" {
		activeTranslation = user.Translation
	}
	activeTranslation = normalizeTranslation(activeTranslation)

	// Fetch verse content using translation and cache.
	verseContents, err := app.fetchPassagesWithCache(r.Context(), activeTranslation, dailyText.Verses)
	if err != nil {
		slog.Error("failed to fetch verses", "date", dateStr, "translation", activeTranslation, "error", err)
		http.Error(w, fmt.Sprintf("Error loading verses for %s", dateStr), http.StatusInternalServerError)
		return
	}

	// Prepare template data.
	data := map[string]any{
		"esvData":        verseContents,
		"fumsTokens":     verseContents.FUMSTokens,
		"date":           dateStr,
		"observation":    soapData.Observation,
		"application":    soapData.Application,
		"prayer":         soapData.Prayer,
		"selectedVerses": soapData.SelectedVerses,
		"translation":    activeTranslation,
		"user":           user,
		"CSRFToken":      r.Context().Value(csrfContextKey).(string),
		"Nonce":          r.Context().Value(nonceContextKey).(string),
	}

	// Render the base template by default.
	templateName := "base"

	// But if the request is coming from HTMX, render the home:content
	// template instead.
	if isHTMXRequest(r) {
		templateName = "home:content"
	}

	if err := app.html.render(w, http.StatusOK, data, templateName, "pages/home.tmpl"); err != nil {
		slog.Error("failed to execute template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getSoap retrieves SOAP data for a given date.
func (app *application) getSoap(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*store.User)
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format(time.DateOnly)
	}

	soapData, err := app.store.GetSOAPData(r.Context(), user.ID, dateStr)
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

// getSoapDates retrieves dates with journal entries for a user within +- 3 months of the requested month.
func (app *application) getSoapDates(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*store.User)

	loc, err := time.LoadLocation(user.Timezone)
	if err != nil {
		slog.Error("failed to load user location", "timezone", user.Timezone, "error", err)
		loc = time.UTC
	}

	var refDate time.Time
	monthParam := r.URL.Query().Get("month")
	dateParam := r.URL.Query().Get("date")

	if monthParam != "" {
		t, parseErr := time.ParseInLocation("2006-01", monthParam, loc)
		if parseErr == nil {
			refDate = t
		}
	} else if dateParam != "" {
		t, parseErr := time.ParseInLocation(time.DateOnly, dateParam, loc)
		if parseErr == nil {
			refDate = t
		}
	}
	if refDate.IsZero() {
		refDate = time.Now().In(loc)
	}

	firstOfMonth := time.Date(refDate.Year(), refDate.Month(), 1, 0, 0, 0, 0, loc)
	startDate := firstOfMonth.AddDate(0, -3, 0).Format(time.DateOnly)
	endDate := firstOfMonth.AddDate(0, 4, 0).AddDate(0, 0, -1).Format(time.DateOnly)

	dates, err := app.store.GetSOAPDatesWithEntries(r.Context(), user.ID, startDate, endDate)
	if err != nil {
		slog.Error("failed to get SOAP dates with entries", "error", err, "userID", user.ID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"dates": dates}); err != nil {
		slog.Error("failed to encode SOAP dates", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// postSoap saves SOAP data.
func (app *application) postSoap(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*store.User)

	var soapData store.SOAPData
	if err := json.NewDecoder(r.Body).Decode(&soapData); err != nil {
		slog.Error("failed to decode SOAP data", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if soapData.Translation == "" {
		soapData.Translation = user.Translation
	}
	soapData.Translation = normalizeTranslation(soapData.Translation)

	if err := app.store.SaveSOAPData(r.Context(), user.ID, &soapData); err != nil {
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

// export handles SOAP journal export requests.
func (app *application) export(w http.ResponseWriter, r *http.Request) {
	var req exportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode export request", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	user := r.Context().Value(userContextKey).(*store.User)

	// Fetch SOAP data.
	soapData, err := app.store.GetSOAPData(r.Context(), user.ID, req.Date)
	if err != nil {
		slog.Error("failed to get SOAP data for export", "date", req.Date, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	translation := soapData.Translation
	if translation == "" {
		translation = user.Translation
	}
	translation = normalizeTranslation(translation)
	soapData.Translation = translation

	// Fetch Scripture content.
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

	// Fetch verse content using translation (using cache).
	references := dailyText.Verses
	if len(soapData.SelectedVerses) > 0 {
		references = []string{esv.FormatReferences(soapData.SelectedVerses)}
	}
	verseContents, err := app.fetchPassagesWithCache(r.Context(), translation, references)
	if err != nil {
		slog.Error("failed to fetch verses for export", "date", req.Date, "error", err)
		http.Error(w, fmt.Sprintf("Error loading verses for %s", req.Date), http.StatusInternalServerError)
		return
	}

	for i, p := range verseContents.Passages {
		if strings.Contains(p, "<h2 class=\"extra_text\">") {
			verseContents.Passages[i] = h2Regex.ReplaceAllStringFunc(p, func(m string) string {
				sub := h2Regex.FindStringSubmatch(m)
				if len(sub) == 4 && !strings.Contains(sub[2], "("+translation+")") {
					return fmt.Sprintf("%s%s (%s)%s", sub[1], sub[2], translation, sub[3])
				}
				return m
			})
		} else if len(references) > i {
			verseContents.Passages[i] = fmt.Sprintf("<h2 class=\"extra_text\">%s (%s)</h2>\n%s", references[i], translation, p)
		}
	}

	scriptureHTML := strings.Join(verseContents.Passages, "\n")

	// Email Logic:
	if req.Method == "email" {
		for _, token := range verseContents.FUMSTokens {
			scriptureHTML += fmt.Sprintf(`<img src="https://fums.api.bible/f3?t=%s" width="1" height="1" style="display:none;" alt="" />`, token)
		}

		// Only allow format: html.
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

		if err := email.QueueExportEmail(r.Context(), app.store, user, req.Date, req.Recipients, buf.String()); err != nil {
			slog.Error("failed to queue export email", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Return 202 Accepted with JSON {"status": "queued"}.
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

	// Set Content-Type and Content-Disposition.
	w.Header().Set("Content-Type", exporter.ContentType())
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Write generated content to w.
	if err := exporter.Export(r.Context(), w, soapData, scriptureHTML); err != nil {
		slog.Error("failed to export content for download", "error", err)
		// Note: headers already sent, can't change status code easily.
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

func (app *application) history(w http.ResponseWriter, r *http.Request) {
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

	entries, err := app.store.GetSOAPDataRange(r.Context(), user.ID, startDateStr, endDateStr)
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
			entryTranslation := entry.Translation
			if entryTranslation == "" {
				entryTranslation = user.Translation
			}
			entryTranslation = normalizeTranslation(entryTranslation)

			esvRes, err := app.fetchPassagesWithCache(r.Context(), entryTranslation, references)
			if err != nil {
				slog.Error("failed to fetch verses for history", "date", entry.Date, "error", err)
			} else {
				for _, p := range esvRes.Passages {
					var formattedP string
					if strings.Contains(p, "<h2 class=\"extra_text\">") {
						formattedP = h2Regex.ReplaceAllStringFunc(p, func(m string) string {
							sub := h2Regex.FindStringSubmatch(m)
							if len(sub) == 4 && !strings.Contains(sub[2], "("+entryTranslation+")") {
								return fmt.Sprintf("%s%s (%s)%s", sub[1], sub[2], entryTranslation, sub[3])
							}
							return m
						})
					} else {
						formattedP = fmt.Sprintf("<h2 class=\"extra_text\">%s (%s)</h2>\n%s", esv.FormatReferences(entry.SelectedVerses), entryTranslation, p)
					}
					htmlPassages = append(htmlPassages, template.HTML(formattedP)) // #nosec G203
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
		"IsHistory":   true,
		"user":        user,
		"CSRFToken":   r.Context().Value(csrfContextKey).(string),
		"Nonce":       r.Context().Value(nonceContextKey).(string),
	}

	if err := app.html.render(w, http.StatusOK, data, "base", "pages/history.tmpl"); err != nil {
		slog.Error("failed to execute history template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// postTranslation updates the user's preferred Bible translation.
func (app *application) postTranslation(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*store.User)

	translation := r.FormValue("translation")
	if translation == "" {
		translation = r.URL.Query().Get("translation")
	}

	switch {
	case strings.EqualFold(translation, "ESV"):
		translation = "ESV"
	case strings.EqualFold(translation, "NLT") || translation == "d6e14a625393b4da-01":
		translation = "NLT"
	case strings.EqualFold(translation, "MSG") || translation == "6f11a7de016f942e-01":
		translation = "MSG"
	default:
		http.Error(w, "Invalid translation", http.StatusBadRequest)
		return
	}

	if err := app.store.UpdateUserTranslation(r.Context(), user.ID, translation); err != nil {
		slog.Error("failed to update user translation", "error", err, "userID", user.ID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	user.Translation = translation

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = r.FormValue("date")
	}
	if dateStr != "" {
		if err := app.store.UpdateJournalTranslation(r.Context(), user.ID, dateStr, translation); err != nil {
			slog.Warn("failed to update journal translation on translation change", "error", err, "userID", user.ID, "date", dateStr)
		}
	}

	if isHTMXRequest(r) {
		if dateStr == "" {
			loc, err := time.LoadLocation(user.Timezone)
			if err != nil {
				loc = time.UTC
			}
			dateStr = time.Now().In(loc).Format(time.DateOnly)
		}
		r.URL.Path = "/"
		r.URL.RawQuery = "date=" + dateStr
		app.home(w, r)
		return
	}

	redirURL := "/"
	if dateStr != "" {
		if t, err := time.Parse(time.DateOnly, dateStr); err == nil {
			redirURL = "/?date=" + t.Format(time.DateOnly)
		}
	}
	// #nosec G710 - redirURL is guaranteed to be a relative path with validated DateOnly parameter.
	http.Redirect(w, r, redirURL, http.StatusFound)
}
