package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"derrclan.com/moravian-soap/internal/dailytexts"
	"derrclan.com/moravian-soap/internal/esv"
	"golang.org/x/crypto/bcrypt"

	_ "github.com/mattn/go-sqlite3"
)

var (
	tmpl *template.Template
	db   *sql.DB
)

//go:embed web
var web embed.FS

type contextKey string

const userContextKey contextKey = "user"

type User struct {
	ID    int64
	Email string
}

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

	// Public routes
	mux.HandleFunc("/login", handleLogin)
	mux.HandleFunc("/register", handleRegister)
	mux.HandleFunc("/logout", handleLogout)

	// Protected routes
	mux.HandleFunc("/", authMiddleware(handleIndex))
	mux.HandleFunc("/reading", authMiddleware(handleVerses))
	mux.HandleFunc("/soap", authMiddleware(handleSOAP))

	// Create a subdirectory filesystem for the web directory
	webFS, err := fs.Sub(web, "web")
	if err != nil {
		slog.Error("failed to create web subdirectory filesystem", "error", err)
	} else {
		mux.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.FS(webFS))))
	}

	return mux
}

// authMiddleware checks for a valid session cookie and sets the user in the context
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil {
			if r.URL.Path == "/" {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := getUserFromSession(cookie.Value)
		if err != nil {
			// Invalid session
			http.SetCookie(w, &http.Cookie{
				Name:   "session_token",
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			if r.URL.Path == "/" {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if err := tmpl.ExecuteTemplate(w, "login.html", map[string]interface{}{"IsLogin": true}); err != nil {
			slog.Error("failed to execute login template", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		password := r.FormValue("password")

		user, err := authenticateUser(email, password)
		if err != nil {
			data := map[string]interface{}{
				"IsLogin": true,
				"Error":   "Invalid email or password",
				"Email":   email,
			}
			if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
				slog.Error("failed to execute login template", "error", err)
			}
			return
		}

		sessionToken, err := createSession(user.ID)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    sessionToken,
			Path:     "/",
			HttpOnly: true,
			Expires:  time.Now().Add(24 * time.Hour * 30), // 30 days
		})

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if err := tmpl.ExecuteTemplate(w, "login.html", map[string]interface{}{"IsLogin": false}); err != nil {
			slog.Error("failed to execute register template", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		password := r.FormValue("password")

		if email == "" || password == "" {
			data := map[string]interface{}{
				"IsLogin": false,
				"Error":   "Email and password are required",
				"Email":   email,
			}
			if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
				slog.Error("failed to execute register template", "error", err)
			}
			return
		}

		if err := createUser(email, password); err != nil {
			slog.Error("failed to create user", "error", err)
			data := map[string]interface{}{
				"IsLogin": false,
				"Error":   "Failed to create user. Email may already be in use.",
				"Email":   email,
			}
			if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
				slog.Error("failed to execute register template", "error", err)
			}
			return
		}

		// Auto login after registration
		user, err := authenticateUser(email, password)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		sessionToken, err := createSession(user.ID)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    sessionToken,
			Path:     "/",
			HttpOnly: true,
			Expires:  time.Now().Add(24 * time.Hour * 30),
		})

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "session_token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/login", http.StatusFound)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*User)

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
	soapData, err := getSOAPData(user.ID, today)
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
		"user":           user,
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

	// Create users table
	createUsersSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL
	);`
	if _, err := db.Exec(createUsersSQL); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Create sessions table
	createSessionsSQL := `
	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`
	if _, err := db.Exec(createSessionsSQL); err != nil {
		return fmt.Errorf("failed to create sessions table: %w", err)
	}

	// Create the journal table with user_id
	createJournalSQL := `
	CREATE TABLE IF NOT EXISTS journal (
		user_id INTEGER NOT NULL,
		date TEXT NOT NULL,
		observation TEXT NOT NULL,
		application TEXT NOT NULL,
		prayer TEXT NOT NULL,
		selected_verses TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, date),
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`

	if _, err := db.Exec(createJournalSQL); err != nil {
		return fmt.Errorf("failed to create journal table: %w", err)
	}

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
	user := r.Context().Value(userContextKey).(*User)
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	soapData, err := getSOAPData(user.ID, dateStr)
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
	user := r.Context().Value(userContextKey).(*User)

	var soapData SOAPData
	if err := json.NewDecoder(r.Body).Decode(&soapData); err != nil {
		slog.Error("failed to decode SOAP data", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := saveSOAPData(user.ID, &soapData); err != nil {
		slog.Error("failed to save SOAP data", "error", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save data"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// getSOAPData retrieves SOAP data from the database for a given user and date
func getSOAPData(userID int64, dateStr string) (*SOAPData, error) {
	var soapData SOAPData
	var selectedVersesJSON sql.NullString
	soapData.Date = dateStr

	query := `SELECT observation, application, prayer, selected_verses FROM journal WHERE user_id = ? AND date = ?`
	err := db.QueryRow(query, userID, dateStr).Scan(&soapData.Observation, &soapData.Application, &soapData.Prayer, &selectedVersesJSON)
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
func saveSOAPData(userID int64, soapData *SOAPData) error {
	// Encode selected verses as JSON
	selectedVersesJSON, err := json.Marshal(soapData.SelectedVerses)
	if err != nil {
		return fmt.Errorf("failed to marshal selected verses: %w", err)
	}

	query := `
		INSERT INTO journal (user_id, date, observation, application, prayer, selected_verses)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, date) DO UPDATE SET
			observation = excluded.observation,
			application = excluded.application,
			prayer = excluded.prayer,
			selected_verses = excluded.selected_verses,
			timestamp = CURRENT_TIMESTAMP
	`
	_, err = db.Exec(query, userID, soapData.Date, soapData.Observation, soapData.Application, soapData.Prayer, selectedVersesJSON)
	if err != nil {
		return fmt.Errorf("failed to save SOAP data: %w", err)
	}
	return nil
}

// User registration and authentication helpers

func createUser(email, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", email, string(hashedPassword))
	return err
}

func authenticateUser(email, password string) (*User, error) {
	var id int64
	var passwordHash string
	err := db.QueryRow("SELECT id, password_hash FROM users WHERE email = ?", email).Scan(&id, &passwordHash)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, err
	}

	return &User{ID: id, Email: email}, nil
}

func createSession(userID int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.URLEncoding.EncodeToString(b)

	// Clean up expired sessions first (optional, but good practice)
	// In a production app, do this in a background job
	db.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now())

	expiresAt := time.Now().Add(24 * time.Hour * 30) // 30 days
	_, err := db.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)", token, userID, expiresAt)
	return token, err
}

func getUserFromSession(token string) (*User, error) {
	var user User
	var expiresAt time.Time

	query := `
		SELECT u.id, u.email, s.expires_at 
		FROM sessions s 
		JOIN users u ON s.user_id = u.id 
		WHERE s.token = ?`

	err := db.QueryRow(query, token).Scan(&user.ID, &user.Email, &expiresAt)
	if err != nil {
		return nil, err
	}

	if time.Now().After(expiresAt) {
		return nil, fmt.Errorf("session expired")
	}

	return &user, nil
}
