package server

import (
	"embed"
)

//go:embed web
var web embed.FS

type Year map[string]DailyText

type DailyText struct {
	Verses          []string `json:"verses"`
	Prayer          string   `json:"prayer"`
	DailyWatchWord  string   `json:"daily_watchword"`
	Doctrinal       string   `json:"doctrinal"`
	WeeklyWatchword string   `json:"weekly_watchword,omitempty"`
	SpecialRemarks  []string `json:"special_remarks,omitempty"`
}

// VerseContent represents the HTML content and copyright for a verse reference
type VerseContent struct {
	Reference string
	HTML      string
	Copyright string
}
