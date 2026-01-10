package server

import (
	"database/sql"
	"encoding/json"
	"reflect"
	"testing"

	"derrclan.com/moravian-soap/internal/esv"
	_ "github.com/mattn/go-sqlite3"
)

func TestFetchPassagesWithCache_Hit(t *testing.T) {
	// 1. Setup in-memory DB
	var err error
	db, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	// 2. Create table
	createCacheSQL := `
	CREATE TABLE esv_cache (
		reference TEXT PRIMARY KEY,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(createCacheSQL); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// 3. Insert fake cache entry
	fakeRef := "Test 1:1"
	fakeResponse := esv.EsvResponse{
		Query:    fakeRef,
		Passages: []string{"<p>This is a cached response</p>"},
	}
	responseBytes, _ := json.Marshal(fakeResponse)

	_, err = db.Exec("INSERT INTO esv_cache (reference, content) VALUES (?, ?)", fakeRef, string(responseBytes))
	if err != nil {
		t.Fatalf("failed to insert fake cache: %v", err)
	}

	// 4. Call function under test
	// Note: fetchPassagesWithCache uses the global 'db' variable which we set above
	result, err := fetchPassagesWithCache([]string{fakeRef})
	if err != nil {
		t.Fatalf("fetchPassagesWithCache failed: %v", err)
	}

	// 5. Verify result matches cache
	if !reflect.DeepEqual(result, fakeResponse) {
		t.Errorf("expected %v, got %v", fakeResponse, result)
	}
}
