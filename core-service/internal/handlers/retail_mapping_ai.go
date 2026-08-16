package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/lease-management-system/core-service/internal/services/retailingest"
)

// RetailMappingAI adapts the ai-service suggest-mapping endpoint to the
// retailingest.MappingSuggester seam — Assist Mode: it only returns
// proposals (headers + masked profiles; raw values never leave Go).
type RetailMappingAI struct {
	baseURL string
	client  *http.Client
}

func NewRetailMappingAI() *RetailMappingAI {
	baseURL := os.Getenv("AI_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://ai-service:8000"
	}
	return &RetailMappingAI{baseURL: baseURL, client: &http.Client{Timeout: 30 * time.Second}}
}

type aiMappingProfile struct {
	Header       string `json:"header"`
	NonEmpty     int    `json:"non_empty"`
	NumericLike  int    `json:"numeric_like"`
	DateLike     int    `json:"date_like"`
	MaskedSample string `json:"masked_sample,omitempty"`
}

func (a *RetailMappingAI) SuggestMapping(ctx context.Context, headers []string, columnProfiles []retailingest.ColumnProfile) (retailingest.Mapping, error) {
	profiles := make([]aiMappingProfile, 0, len(columnProfiles))
	for _, profile := range columnProfiles {
		profiles = append(profiles, aiMappingProfile{Header: profile.Header, NonEmpty: profile.NonEmpty, NumericLike: profile.Numeric, DateLike: profile.DateLike, MaskedSample: profile.MaskedSample})
	}
	body, err := json.Marshal(map[string]any{"headers": headers, "column_profiles": profiles})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/v1/suggest-mapping", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := a.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI mapping service returned %d", response.StatusCode)
	}
	var payload struct {
		Suggestions map[string]*string `json:"suggestions"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	mapping := retailingest.Mapping{}
	for header, field := range payload.Suggestions {
		if field != nil {
			mapping[header] = *field
		}
	}
	return mapping, nil
}
