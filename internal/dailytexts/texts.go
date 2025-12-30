package dailytexts

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

//go:embed texts
var texts embed.FS

var (
	// Cache of loaded year data, keyed by year (e.g., "2025", "2026")
	yearDataCache = make(map[string]Year)
	cacheMutex    sync.RWMutex
)

type Year map[string]DailyText

type DailyText struct {
	Verses          []string `json:"verses"`
	Prayer          string   `json:"prayer"`
	DailyWatchWord  string   `json:"daily_watchword"`
	Doctrinal       string   `json:"doctrinal"`
	WeeklyWatchword string   `json:"weekly_watchword,omitempty"`
	SpecialRemarks  []string `json:"special_remarks,omitempty"`
}

// getDailyText retrieves the daily text for a given date (YYYY-MM-DD format).
// It will automatically load the year file if it hasn't been loaded yet.
func GetDailyText(dateStr string) (*DailyText, error) {
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
	filename := fmt.Sprintf("texts/%s.json", year)
	data, err := texts.ReadFile(filename)
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

func init() {
	// Load current year data
	currentYear := time.Now().Format("2006")
	if err := loadYearData(currentYear); err != nil {
		slog.Error("failed to load year data", "year", currentYear, "error", err)
	}
}
