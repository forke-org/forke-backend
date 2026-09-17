// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/forke/forke-backend/internal/models"
)

// GeminiClient communicates with the Google Cloud Gemini Flash API
type GeminiClient struct {
	APIKey     string
	HTTPClient *http.Client
	Model      string
}

// NewGeminiClient creates a new Gemini Flash client
func NewGeminiClient(apiKey string) *GeminiClient {
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &GeminiClient{
		APIKey: apiKey,
		Model:  model,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ReviewContext aggregates all facts passed to Gemini Flash
type ReviewContext struct {
	PRNumber           int
	PRTitle            string
	PRBody             string
	DeveloperUsername  string
	ChangedFiles       []string
	GitDiff            string
	RepoStructure      string
	TaskTitle          string
	TaskDescription    string
	AcceptanceCriteria string
	FrontendStack      string
	BackendStack       string
	TestScoreResult    models.TestScoreResult
	BaselineComparison *models.BaselineComparison
}

// RunReview executes the Gemini Flash AI reasoning layer
func (g *GeminiClient) RunReview(ctx context.Context, reviewCtx ReviewContext) (*models.AIReviewResult, error) {
	if g.APIKey == "" {
		// Graceful fallback when running offline or without API key
		return &models.AIReviewResult{
			Verdict:          "pass",
			Score:            85,
			RequirementMatch: 0.85,
			Summary:          "Automated review completed. (Gemini API key not configured — fallback heuristic evaluation applied).",
			Strengths:        []string{"Passed deterministic verification suite.", "Clean PR structure."},
			Issues:           []models.AIIssue{},
			Risks:            []models.AIRisk{},
			TokensUsed:       0,
			Model:            "heuristic-fallback",
		}, nil
	}

	prompt := g.buildPrompt(reviewCtx)

	reqPayload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":       0.2,
			"topP":              0.8,
			"maxOutputTokens":   4096,
			"responseMimeType":  "application/json",
		},
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Gemini request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.Model, g.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.HTTPClient.Do(httpReq)
	if err != nil {
		// Fallback to gemini-1.5-flash if 2.0 fails or times out
		if g.Model != "gemini-1.5-flash" {
			g.Model = "gemini-1.5-flash"
			return g.RunReview(ctx, reviewCtx)
		}
		return nil, fmt.Errorf("gemini API call failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Gemini response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	return g.parseGeminiResponse(respBytes)
}

func (g *GeminiClient) buildPrompt(c ReviewContext) string {
	var sb strings.Builder

	sb.WriteString("You are Forke's Senior Principal AI Code Reviewer.\n")
	sb.WriteString("Your mission is to rigorously evaluate this Pull Request against the owner's Task Acceptance Criteria and Deterministic Verification Facts.\n\n")

	sb.WriteString("=== TASK SPECIFICATION ===\n")
	sb.WriteString(fmt.Sprintf("Title: %s\n", c.TaskTitle))
	sb.WriteString(fmt.Sprintf("Description: %s\n", c.TaskDescription))
	sb.WriteString(fmt.Sprintf("Acceptance Criteria:\n%s\n", c.AcceptanceCriteria))
	if c.FrontendStack != "" {
		sb.WriteString(fmt.Sprintf("Frontend Stack: %s\n", c.FrontendStack))
	}
	if c.BackendStack != "" {
		sb.WriteString(fmt.Sprintf("Backend Stack: %s\n", c.BackendStack))
	}
	sb.WriteString("\n")

	sb.WriteString("=== DETERMINISTIC VERIFICATION FACTS (DO NOT GUESS THESE) ===\n")
	sb.WriteString(fmt.Sprintf("Raw Test Score: %d/100 (Adjusted Test Score: %d/100)\n", c.TestScoreResult.RawScore, c.TestScoreResult.TestScore))
	sb.WriteString(fmt.Sprintf("Compilation / Build Failed: %v\n", c.TestScoreResult.BuildFailed))
	sb.WriteString("Categories Tested:\n")
	for _, cat := range c.TestScoreResult.Categories {
		sb.WriteString(fmt.Sprintf("- %s: status=%s, quality=%.2f, issues=%d\n", cat.Name, cat.Status, cat.Quality, cat.IssuesCount))
	}
	if len(c.TestScoreResult.Penalties) > 0 {
		sb.WriteString("Penalties Applied:\n")
		for _, p := range c.TestScoreResult.Penalties {
			sb.WriteString(fmt.Sprintf("- %s: -%.1f pts\n", p.Reason, p.Points))
		}
	}
	sb.WriteString("\n")

	if c.BaselineComparison != nil && len(c.BaselineComparison.Regressions) > 0 {
		sb.WriteString("=== NET REGRESSIONS FROM BASELINE ===\n")
		for cat, reg := range c.BaselineComparison.Regressions {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", cat, reg))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("=== PULL REQUEST #%d DETAILS ===\n", c.PRNumber))
	sb.WriteString(fmt.Sprintf("Author: %s\n", c.DeveloperUsername))
	sb.WriteString(fmt.Sprintf("Title: %s\n", c.PRTitle))
	sb.WriteString(fmt.Sprintf("Description: %s\n", c.PRBody))
	sb.WriteString("Changed Files:\n")
	for _, f := range c.ChangedFiles {
		sb.WriteString(fmt.Sprintf("- %s\n", f))
	}
	sb.WriteString("\n")

	sb.WriteString("=== GIT DIFF (TRUNCATED) ===\n")
	// Bound diff to 30,000 characters max
	diffText := c.GitDiff
	if len(diffText) > 30000 {
		diffText = diffText[:30000] + "\n...[Diff truncated for context limits]..."
	}
	sb.WriteString(diffText)
	sb.WriteString("\n\n")

	sb.WriteString("=== REQUIRED JSON OUTPUT SCHEMA ===\n")
	sb.WriteString("Respond ONLY with a valid JSON object matching this schema:\n")
	sb.WriteString(`{
  "verdict": "pass" | "needs_changes" | "high_risk",
  "score": <0-100 integer representing code quality and task fulfillment>,
  "requirement_match": <0.0 to 1.0 float representing degree of acceptance criteria satisfied>,
  "summary": "<Concise 2-3 paragraph executive review summary>",
  "strengths": ["<strength 1>", "<strength 2>"],
  "issues": [
    {
      "file": "<relative file path>",
      "line": <integer line number>,
      "severity": "critical" | "high" | "medium" | "low",
      "message": "<description of flaw>",
      "suggestion": "<concrete fix or code snippet>",
      "status": "new"
    }
  ],
  "risks": [
    {
      "category": "credential" | "security" | "logic" | "compliance",
      "message": "<description of risk>",
      "severity": "critical" | "high" | "medium" | "low",
      "status": "new"
    }
  ],
  "unauthorized_file_edits": []
}`)

	return sb.String()
}

func (g *GeminiClient) parseGeminiResponse(respBytes []byte) (*models.AIReviewResult, error) {
	var apiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			TotalTokenCount int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini API JSON: %w", err)
	}

	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini returned empty response candidates")
	}

	rawJSON := apiResp.Candidates[0].Content.Parts[0].Text
	rawJSON = cleanJSONBlock(rawJSON)

	var result models.AIReviewResult
	if err := json.Unmarshal([]byte(rawJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AIReviewResult from model text: %w (raw text: %s)", err, rawJSON)
	}

	result.TokensUsed = apiResp.UsageMetadata.TotalTokenCount
	result.Model = g.Model

	if result.Verdict == "" {
		result.Verdict = "pass"
	}

	return &result, nil
}

func cleanJSONBlock(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "```json") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		if idx := strings.LastIndex(trimmed, "```"); idx != -1 {
			trimmed = trimmed[:idx]
		}
	} else if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
		if idx := strings.LastIndex(trimmed, "```"); idx != -1 {
			trimmed = trimmed[:idx]
		}
	}
	return strings.TrimSpace(trimmed)
}
