package analyzer

import (
	"testing"

	"dfcleaner/internal/scanner"
)

func TestParseAnalysisResponse(t *testing.T) {
	entries := []scanner.FileEntry{
		{Path: "/tmp/cache/a.log"},
		{Path: "/tmp/config/settings.json"},
	}

	jsonResp := `[{"path":"/tmp/cache/a.log","risk_level":"safe","reason":"Log file, safe to delete","category":"log","confidence":0.9},{"path":"/tmp/config/settings.json","risk_level":"dangerous","reason":"Configuration file, do not delete","category":"config","confidence":0.95}]`

	results, err := parseAnalysisResponse(jsonResp, entries)
	if err != nil {
		t.Fatalf("parseAnalysisResponse: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].RiskLevel != "safe" {
		t.Errorf("result[0].RiskLevel = %q, want 'safe'", results[0].RiskLevel)
	}
	if results[1].RiskLevel != "dangerous" {
		t.Errorf("result[1].RiskLevel = %q, want 'dangerous'", results[1].RiskLevel)
	}
}

func TestParseAnalysisResponseWrapped(t *testing.T) {
	entries := []scanner.FileEntry{
		{Path: "/tmp/test"},
	}
	resp := "```json\n[{\"path\":\"/tmp/test\",\"risk_level\":\"caution\",\"reason\":\"Check first\",\"category\":\"temp\",\"confidence\":0.7}]\n```"

	results, err := parseAnalysisResponse(resp, entries)
	if err != nil {
		t.Fatalf("parseAnalysisResponse: %v", err)
	}
	if results[0].RiskLevel != "caution" {
		t.Errorf("RiskLevel = %q, want 'caution'", results[0].RiskLevel)
	}
}

func TestParseAnalysisResponseInvalid(t *testing.T) {
	entries := []scanner.FileEntry{
		{Path: "/tmp/a"},
		{Path: "/tmp/b"},
	}

	results, err := parseAnalysisResponse("not json at all", entries)
	if err != nil {
		t.Fatalf("parseAnalysisResponse: %v", err)
	}
	for _, r := range results {
		if r.Error != "analysis_error" {
			t.Errorf("expected analysis_error, got %q", r.Error)
		}
	}
}

func TestParseAnalysisResponseUnknownRiskLevel(t *testing.T) {
	entries := []scanner.FileEntry{
		{Path: "/tmp/x"},
	}
	resp := `[{"path":"/tmp/x","risk_level":"weird_value","reason":"Unknown","category":"temp","confidence":0.5}]`

	results, err := parseAnalysisResponse(resp, entries)
	if err != nil {
		t.Fatalf("parseAnalysisResponse: %v", err)
	}
	// Unknown risk level should default to "caution"
	if results[0].RiskLevel != "caution" {
		t.Errorf("RiskLevel = %q, want 'caution' for unknown", results[0].RiskLevel)
	}
}
