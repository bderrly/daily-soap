package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bderrly/daily-soap/internal/esv"
)

// fetchPassagesWithCache fetches verses from the cache or the ESV API.
func fetchPassagesWithCache(ctx context.Context, references []string) (esv.Response, error) {
	key := strings.Join(references, ";")
	var response esv.Response

	// 1. Check cache
	content, err := appStore.GetCachedESV(ctx, key)
	if err == nil {
		// Cache hit
		if err := json.Unmarshal([]byte(content), &response); err != nil {
			// If unmarshal fails, log it and fall back to fetch
			slog.Error("failed to unmarshal cached ESV response", slog.Any("error", err))
		} else {
			slog.Debug("cache hit for verses", slog.String("reference", key))
			// Ensure cross-book passages are split (old cache entries may
			// contain merged passages without the second book heading).
			esv.SplitCrossBookPassages(&response)
			return response, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		// Log DB error but proceed to fetch
		slog.Error("failed to query esv_cache", "error", err)
	}

	// 2. Fetch from API
	response, err = esv.FetchPassages(ctx, references)
	if err != nil {
		return response, fmt.Errorf("fetching passages %v from ESV: %w", references, err)
	}

	// 3. Save to cache
	responseBytes, err := json.Marshal(response)
	if err != nil {
		slog.Error("failed to marshal ESV response for cache", "error", err)
		return response, nil // Return successful fetch even if cache save fails
	}

	err = appStore.SaveCachedESV(ctx, key, string(responseBytes))
	if err != nil {
		slog.Error("failed to save to esv_cache", "error", err)
	} else {
		slog.Debug("saved verses to cache", "reference", key)
	}

	return response, nil
}
