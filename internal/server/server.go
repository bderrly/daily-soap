package server

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"derrclan.com/moravian-soap/internal/dailytexts"
	"derrclan.com/moravian-soap/internal/esv"

	_ "github.com/mattn/go-sqlite3"
)

var (
	tmpl *template.Template
	db   *sql.DB
)

//go:embed web
var web embed.FS

func init() {
	// Initialize database
	if err := initDB(); err != nil {
		slog.Error("failed to initialize database", "error", err)
	}

	// Parse templates with function map for safe HTML rendering
	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"toJSON": func(v interface{}) (template.JS, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return template.JS(b), nil
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
	mux.HandleFunc("/api/soap", handleSOAP)

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
	dailyText, err := dailytexts.GetDailyText(today)
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
	verseContents, err := esv.FetchVerses(dailyText.Verses)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error loading verses for %s", today), http.StatusInternalServerError)
		return
	}

	// Load existing SOAP data from database
	soapData, err := getSOAPData(today)
	if err != nil {
		slog.Warn("failed to load SOAP data", "date", today, "error", err)
		// Continue with empty values if there's an error
		soapData = &SOAPData{
			Date:           today,
			Observation:    "",
			Application:    "",
			Prayer:         "",
			SelectedVerses: []string{},
		}
	}

	// Prepare template data
	data := map[string]any{
		"esvData":        verseContents,
		"date":           today,
		"observation":    soapData.Observation,
		"application":    soapData.Application,
		"prayer":         soapData.Prayer,
		"selectedVerses": soapData.SelectedVerses,
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

	// Fetch verse content from ESV API
	verseContents, err := esv.FetchVerses(dailyText.Verses)
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

// initDB initializes the SQLite database and creates the necessary table.
// The database file will be created at "journal.db" in the current directory.
func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "journal.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Create the table with date as primary key
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS journal (
		date TEXT PRIMARY KEY,
		observation TEXT NOT NULL,
		application TEXT NOT NULL,
		prayer TEXT NOT NULL,
		selected_verses TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Add selected_verses column if it doesn't exist (for existing databases)
	// SQLite doesn't support IF NOT EXISTS for ALTER TABLE ADD COLUMN,
	// so we'll just try to add it and ignore the error if it already exists
	alterTableSQL := `ALTER TABLE journal ADD COLUMN selected_verses TEXT;`
	db.Exec(alterTableSQL) // Ignore error if column already exists

	slog.Info("database initialized successfully")
	return nil
}

// SOAPData represents the SOAP journal entry
type SOAPData struct {
	Date           string   `json:"date"`
	Observation    string   `json:"observation"`
	Application    string   `json:"application"`
	Prayer         string   `json:"prayer"`
	SelectedVerses []string `json:"selectedVerses"`
}

// handleSOAP handles GET and POST requests for SOAP data
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

// handleGetSOAP retrieves SOAP data for a given date
func handleGetSOAP(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	soapData, err := getSOAPData(dateStr)
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

// handlePostSOAP saves SOAP data
func handlePostSOAP(w http.ResponseWriter, r *http.Request) {
	var soapData SOAPData
	if err := json.NewDecoder(r.Body).Decode(&soapData); err != nil {
		slog.Error("failed to decode SOAP data", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := saveSOAPData(&soapData); err != nil {
		slog.Error("failed to save SOAP data", "error", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save data"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// getSOAPData retrieves SOAP data from the database for a given date
func getSOAPData(dateStr string) (*SOAPData, error) {
	var soapData SOAPData
	var selectedVersesJSON sql.NullString
	soapData.Date = dateStr

	query := `SELECT observation, application, prayer, selected_verses FROM journal WHERE date = ?`
	err := db.QueryRow(query, dateStr).Scan(&soapData.Observation, &soapData.Application, &soapData.Prayer, &selectedVersesJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			// No data found, return empty values
			soapData.SelectedVerses = []string{}
			return &soapData, nil
		}
		return nil, fmt.Errorf("failed to query SOAP data: %w", err)
	}

	// Parse selected verses from JSON
	if selectedVersesJSON.Valid && selectedVersesJSON.String != "" {
		if err := json.Unmarshal([]byte(selectedVersesJSON.String), &soapData.SelectedVerses); err != nil {
			slog.Warn("failed to unmarshal selected verses", "error", err)
			soapData.SelectedVerses = []string{}
		}
	} else {
		soapData.SelectedVerses = []string{}
	}

	return &soapData, nil
}

// saveSOAPData saves SOAP data to the database
func saveSOAPData(soapData *SOAPData) error {
	// Encode selected verses as JSON
	selectedVersesJSON, err := json.Marshal(soapData.SelectedVerses)
	if err != nil {
		return fmt.Errorf("failed to marshal selected verses: %w", err)
	}

	query := `
		INSERT INTO journal (date, observation, application, prayer, selected_verses)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET
			observation = excluded.observation,
			application = excluded.application,
			prayer = excluded.prayer,
			selected_verses = excluded.selected_verses,
			timestamp = CURRENT_TIMESTAMP
	`
	_, err = db.Exec(query, soapData.Date, soapData.Observation, soapData.Application, soapData.Prayer, selectedVersesJSON)
	if err != nil {
		return fmt.Errorf("failed to save SOAP data: %w", err)
	}
	return nil
}
