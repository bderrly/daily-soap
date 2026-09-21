package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bderrly/daily-soap/internal/apibible"
	"github.com/bderrly/daily-soap/internal/esv"
)

// fetchPassagesWithCache fetches verses from the cache or the respective Bible API (ESV or API.Bible).
func (app *application) fetchPassagesWithCache(ctx context.Context, translation string, references []string) (esv.Response, error) {
	translation = strings.ToUpper(strings.TrimSpace(translation))
	switch {
	case translation == "" || translation == "ESV":
		translation = "ESV"
	case translation == "NLT" || strings.EqualFold(translation, apibible.TranslationNLT):
		translation = "NLT"
	case translation == "MSG" || strings.EqualFold(translation, apibible.TranslationMSG):
		translation = "MSG"
	}
	rawKey := strings.Join(references, ";")
	key := fmt.Sprintf("%s:%s", translation, rawKey)
	var response esv.Response

	// 1. Check cache.
	content, err := app.store.GetCachedScripture(ctx, key)
	if errors.Is(err, sql.ErrNoRows) && strings.EqualFold(translation, "ESV") {
		// Fallback to un-prefixed key for backwards compatibility with legacy ESV cache entries.
		content, err = app.store.GetCachedScripture(ctx, rawKey)
	}

	if err == nil {
		// Cache hit.
		if err := json.Unmarshal([]byte(content), &response); err != nil {
			slog.Error("failed to unmarshal cached scripture response", slog.Any("error", err))
		} else {
			slog.Debug("cache hit for verses", slog.String("reference", key))
			if strings.EqualFold(translation, "ESV") {
				esv.SplitCrossBookPassages(&response)
			}
			return response, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		slog.Error("failed to query scripture_cache", "error", err)
	}

	// 2. Fetch from API.
	switch {
	case strings.EqualFold(translation, "NLT") || translation == apibible.TranslationNLT:
		apiResp, err := apibible.FetchPassages(ctx, apibible.TranslationNLT, references)
		if err != nil {
			return response, fmt.Errorf("fetching passages %v from NLT: %w", references, err)
		}
		response = esv.Response{
			Query:      apiResp.Query,
			Passages:   apiResp.Passages,
			Copyright:  apiResp.Copyright,
			FUMSTokens: apiResp.FUMSTokens,
		}
	case strings.EqualFold(translation, "MSG") || translation == apibible.TranslationMSG:
		apiResp, err := apibible.FetchPassages(ctx, apibible.TranslationMSG, references)
		if err != nil {
			return response, fmt.Errorf("fetching passages %v from The Message: %w", references, err)
		}
		response = esv.Response{
			Query:      apiResp.Query,
			Passages:   apiResp.Passages,
			Copyright:  apiResp.Copyright,
			FUMSTokens: apiResp.FUMSTokens,
		}
	default:
		response, err = esv.FetchPassages(ctx, references)
		if err != nil {
			return response, fmt.Errorf("fetching passages %v from ESV: %w", references, err)
		}
	}

	// 3. Save to cache.
	responseBytes, err := json.Marshal(response)
	if err != nil {
		slog.Error("failed to marshal scripture response for cache", "error", err)
		return response, nil // Return successful fetch even if cache save fails.
	}

	err = app.store.SaveCachedScripture(ctx, key, string(responseBytes))
	if err != nil {
		slog.Error("failed to save to scripture_cache", "error", err)
	} else {
		slog.Debug("saved verses to cache", "reference", key)
	}

	return response, nil
}
