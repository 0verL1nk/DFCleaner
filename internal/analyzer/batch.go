package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"dfcleaner/internal/scanner"

	"github.com/cloudwego/eino/schema"
)

type AnalysisResult struct {
	Path       string  `json:"path"`
	RiskLevel  string  `json:"riskLevel"`
	Reason     string  `json:"reason"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Error      string  `json:"error,omitempty"`
}

// BatchAnalyze analyzes a batch of file entries using the LLM.
func (a *Analyzer) BatchAnalyze(ctx context.Context, entries []scanner.FileEntry) ([]AnalysisResult, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	agent, err := a.getOrCreateAgent()
	if err != nil {
		return nil, err
	}

	// Build the batch prompt
	var sb strings.Builder
	sb.WriteString("Analyze the following files/directories and determine if they are safe to delete. ")
	sb.WriteString("For each item, respond with a JSON array where each element has: path, risk_level (safe/caution/dangerous), reason, category (cache/temp/log/config/document/media), confidence (0-1).\n\n")

	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("- Path: %s, Name: %s, Size: %d, Type: %s, IsDir: %v, ModTime: %s",
			e.Path, e.Name, e.Size, e.Extension, e.IsDir, e.ModTime.Format("2006-01-02")))

		if !e.AccessTime.IsZero() {
			sb.WriteString(fmt.Sprintf(", LastAccess: %s", e.AccessTime.Format("2006-01-02")))
		}

		sb.WriteString("\n")
	}

	sb.WriteString("\nRespond ONLY with the JSON array, no other text.")

	input := []*schema.Message{
		schema.UserMessage(sb.String()),
	}

	resp, err := agent.Generate(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("LLM generate: %w", err)
	}

	return parseAnalysisResponse(resp.Content, entries)
}

func parseAnalysisResponse(content string, entries []scanner.FileEntry) ([]AnalysisResult, error) {
	// Extract JSON array from response (may be wrapped in markdown code block)
	jsonStr := content
	if idx := strings.Index(content, "["); idx >= 0 {
		jsonStr = content[idx:]
		if endIdx := strings.LastIndex(jsonStr, "]"); endIdx > 0 {
			jsonStr = jsonStr[:endIdx+1]
		}
	}

	var rawResults []struct {
		Path       string  `json:"path"`
		RiskLevel  string  `json:"risk_level"`
		Reason     string  `json:"reason"`
		Category   string  `json:"category"`
		Confidence float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rawResults); err != nil {
		// If parsing fails, return analysis_error for all entries
		results := make([]AnalysisResult, len(entries))
		for i, e := range entries {
			results[i] = AnalysisResult{
				Path:  e.Path,
				Error: "analysis_error",
			}
		}
		return results, nil
	}

	// Map raw results by path
	resultMap := make(map[string]AnalysisResult, len(rawResults))
	for _, r := range rawResults {
		riskLevel := strings.ToLower(r.RiskLevel)
		if riskLevel != "safe" && riskLevel != "caution" && riskLevel != "dangerous" {
			riskLevel = "caution" // default to caution for unknown values
		}
		resultMap[r.Path] = AnalysisResult{
			Path:       r.Path,
			RiskLevel:  riskLevel,
			Reason:     r.Reason,
			Category:   r.Category,
			Confidence: r.Confidence,
		}
	}

	// Build results for all entries, marking missing ones as analysis_error
	results := make([]AnalysisResult, len(entries))
	for i, e := range entries {
		if r, ok := resultMap[e.Path]; ok {
			results[i] = r
		} else {
			results[i] = AnalysisResult{
				Path:  e.Path,
				Error: "analysis_error",
			}
		}
	}

	return results, nil
}
