// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package review

import (
	"testing"

	"github.com/forke/forke-backend/internal/models"
)

func TestComputeTestScore(t *testing.T) {
	// Perfect run
	perfectResults := map[string]models.CategoryResult{
		"build":             {Status: "pass"},
		"lint":              {Status: "pass"},
		"type_checks":       {Status: "pass"},
		"unit_tests":        {Status: "pass"},
		"integration_tests": {Status: "pass"},
		"security":          {Status: "pass"},
		"sast":              {Status: "pass"},
	}

	res := ComputeTestScore(perfectResults, nil, 0, true)
	if res.TestScore != 100 {
		t.Errorf("expected TestScore=100 for perfect run, got %d", res.TestScore)
	}

	// Compilation failed run
	failedBuild := map[string]models.CategoryResult{
		"build":       {Status: "fail"},
		"lint":        {Status: "pass"},
		"type_checks": {Status: "fail"},
	}
	resFailed := ComputeTestScore(failedBuild, nil, 0, true)
	if !resFailed.BuildFailed {
		t.Errorf("expected BuildFailed=true")
	}
	if resFailed.TestScore > 40 {
		t.Errorf("expected TestScore <= 40 when build fails, got %d", resFailed.TestScore)
	}

	// Penalties applied for unauthorized files and secret leaks
	unauth := []string{"package.json"}
	resPenalty := ComputeTestScore(perfectResults, unauth, 1, false)
	if len(resPenalty.Penalties) != 3 {
		t.Errorf("expected 3 penalties, got %d", len(resPenalty.Penalties))
	}
	if resPenalty.TestScore >= 100 {
		t.Errorf("expected penalized score < 100, got %d", resPenalty.TestScore)
	}
}

func TestCalculateFinalScore(t *testing.T) {
	aiReview := &models.AIReviewResult{
		Score:   90,
		Verdict: "pass",
	}
	testScore := models.TestScoreResult{
		TestScore: 90,
	}

	finalScore, verdict, risk, routing := CalculateFinalScore(aiReview, testScore)
	if finalScore != 90 {
		t.Errorf("expected finalScore=90, got %d", finalScore)
	}
	if verdict != "pass" {
		t.Errorf("expected verdict=pass, got %s", verdict)
	}
	if risk > 30 {
		t.Errorf("expected risk <= 30, got %d", risk)
	}
	if routing != "auto_approve" {
		t.Errorf("expected routing=auto_approve, got %s", routing)
	}
}
