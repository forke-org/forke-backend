// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package review

import (
	"math"

	"github.com/forke/forke-backend/internal/models"
)

var defaultWeights = map[string]float64{
	"build":             25.0,
	"lint":              10.0,
	"type_checks":       15.0,
	"unit_tests":        20.0,
	"integration_tests": 5.0,
	"e2e_tests":         5.0,
	"security":          10.0,
	"code_quality":      5.0,
	"dependencies":      5.0,
	"performance":       5.0,
	"format":            5.0,
	"sast":              10.0,
}

// ComputeTestScore computes raw deterministic scores and applies penalties
func ComputeTestScore(results map[string]models.CategoryResult, unauthorizedFiles []string, secretCount int, submissionValid bool) models.TestScoreResult {
	categories := make([]models.CategoryScore, 0, len(defaultWeights))
	var earned, possible float64
	buildFailed := false

	for cat, weight := range defaultWeights {
		res, exists := results[cat]
		if !exists || res.Status == "skip" {
			continue
		}

		quality := 1.0
		switch res.Status {
		case "pass":
			quality = 1.0
		case "warn":
			quality = 0.6
		case "fail":
			quality = 0.0
			if cat == "build" {
				buildFailed = true
			}
		}

		contrib := math.Round(weight*quality*100) / 100
		earned += contrib
		possible += weight

		categories = append(categories, models.CategoryScore{
			Name:         cat,
			Weight:       weight,
			Quality:      quality,
			Contribution: contrib,
			Status:       res.Status,
			IssuesCount:  res.IssuesCount,
		})
	}

	rawScore := 100
	if possible > 0 {
		rawScore = int(math.Round((earned / possible) * 100))
	}
	if buildFailed && rawScore > 40 {
		rawScore = 40 // Hard ceiling if compilation failed
	}

	// Calculate penalties
	var penalties []models.ScorePenalty
	totalPenalty := 0.0

	if len(unauthorizedFiles) > 0 {
		deduction := math.Min(30.0, float64(len(unauthorizedFiles))*10.0)
		penalties = append(penalties, models.ScorePenalty{
			Reason: "Files modified outside allowed boundaries",
			Points: deduction,
		})
		totalPenalty += deduction
	}

	if secretCount > 0 {
		deduction := math.Min(40.0, float64(secretCount)*20.0)
		penalties = append(penalties, models.ScorePenalty{
			Reason: "Hardcoded credentials or API keys flagged",
			Points: deduction,
		})
		totalPenalty += deduction
	}

	if !submissionValid {
		penalties = append(penalties, models.ScorePenalty{
			Reason: "FORKE_SUBMISSION.md missing or incomplete",
			Points: 15.0,
		})
		totalPenalty += 15.0
	}

	testScore := int(math.Max(0, math.Min(100, float64(rawScore)-totalPenalty)))

	return models.TestScoreResult{
		RawScore:    rawScore,
		TestScore:   testScore,
		BuildFailed: buildFailed,
		Penalties:   penalties,
		Categories:  categories,
	}
}

// AdjustScoreForBaseline recalibrates category scores so pre-existing debt is not penalized
func AdjustScoreForBaseline(raw models.TestScoreResult, comparison *models.BaselineComparison) models.TestScoreResult {
	if comparison == nil {
		return raw
	}

	adjustedCategories := make([]models.CategoryScore, 0, len(raw.Categories))
	var earned, possible float64

	for _, cat := range raw.Categories {
		// If category was flagged as pre-existing debt without regression, grant full quality
		if _, isDebt := comparison.BaselineDebt[cat.Name]; isDebt {
			if _, isReg := comparison.Regressions[cat.Name]; !isReg {
				cat.Quality = 1.0
				cat.Contribution = cat.Weight
				cat.Status = "pass"
			}
		}

		earned += cat.Contribution
		possible += cat.Weight
		adjustedCategories = append(adjustedCategories, cat)
	}

	adjustedRaw := 100
	if possible > 0 {
		adjustedRaw = int(math.Round((earned / possible) * 100))
	}
	if raw.BuildFailed && adjustedRaw > 40 {
		adjustedRaw = 40
	}

	totalPenalty := 0.0
	for _, p := range raw.Penalties {
		totalPenalty += p.Points
	}
	adjustedTestScore := int(math.Max(0, math.Min(100, float64(adjustedRaw)-totalPenalty)))

	return models.TestScoreResult{
		RawScore:    adjustedRaw,
		TestScore:   adjustedTestScore,
		BuildFailed: raw.BuildFailed,
		Penalties:   raw.Penalties,
		Categories:  adjustedCategories,
	}
}

// CalculateFinalScore synthesizes deterministic score + AI requirement match and routes risk
func CalculateFinalScore(aiResult *models.AIReviewResult, testResult models.TestScoreResult) (int, string, int, string) {
	// Weighted composite score: 60% deterministic test score + 40% AI score
	testScore := float64(testResult.TestScore)
	aiScore := float64(aiResult.Score)
	if aiScore <= 0 {
		aiScore = 80.0
	}

	finalScore := int(math.Round((testScore * 0.6) + (aiScore * 0.4)))

	// Determine final verdict
	finalVerdict := "pass"
	if testResult.BuildFailed || aiResult.Verdict == "high_risk" || finalScore < 50 {
		finalVerdict = "high_risk"
	} else if finalScore < 75 || aiResult.Verdict == "needs_changes" {
		finalVerdict = "needs_changes"
	}

	// Calculate composite risk score (0 - 100 where higher is riskier)
	compositeRisk := 0
	if finalVerdict == "high_risk" {
		compositeRisk = int(math.Min(100, 85+float64(100-finalScore)*0.3))
	} else if finalVerdict == "needs_changes" {
		compositeRisk = 40 + int(float64(100-finalScore)*0.4)
	} else {
		compositeRisk = int(math.Max(0, float64(100-finalScore)*0.3))
	}

	// Autonomous Routing decision
	riskRouting := "owner_review"
	if compositeRisk <= 30 && finalVerdict == "pass" {
		riskRouting = "auto_approve"
	} else if compositeRisk > 70 || finalVerdict == "high_risk" {
		riskRouting = "reviewer_queue"
	}

	return finalScore, finalVerdict, compositeRisk, riskRouting
}
