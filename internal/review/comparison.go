// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package review

import (
	"fmt"
	"strings"

	"github.com/forke/forke-backend/internal/models"
)

// CompareToBaseline computes the delta between PR results and baseline snapshot
func CompareToBaseline(baselineResults, prResults map[string]models.CategoryResult) *models.BaselineComparison {
	regressions := make(map[string]string)
	baselineDebt := make(map[string]string)
	improvements := make(map[string]string)

	for cat, prRes := range prResults {
		baseRes, inBase := baselineResults[cat]

		// Category failed or warned in PR
		if prRes.Status == "fail" || prRes.Status == "warn" {
			if !inBase {
				// Not present in baseline: New failure introduced
				regressions[cat] = fmt.Sprintf("Introduced new %s failure (%d issues).", prRes.Status, prRes.IssuesCount)
			} else if baseRes.Status == "pass" || baseRes.Status == "skip" {
				// Was passing in baseline, now failing
				regressions[cat] = fmt.Sprintf("Regressed from %s to %s in PR (%d issues).", baseRes.Status, prRes.Status, prRes.IssuesCount)
			} else if prRes.IssuesCount > baseRes.IssuesCount {
				// Was already failing, but PR increased issue count
				delta := prRes.IssuesCount - baseRes.IssuesCount
				regressions[cat] = fmt.Sprintf("Increased issue count by %d (Baseline had %d, PR has %d).", delta, baseRes.IssuesCount, prRes.IssuesCount)
			} else {
				// Pre-existing debt: Baseline was already broken and PR didn't make it worse
				baselineDebt[cat] = fmt.Sprintf("Pre-existing baseline debt (%d issues in baseline). Not penalized.", baseRes.IssuesCount)
			}
		} else if prRes.Status == "pass" && inBase && (baseRes.Status == "fail" || baseRes.Status == "warn") {
			// PR fixed an issue that was broken in baseline!
			improvements[cat] = fmt.Sprintf("Resolved pre-existing baseline %s issue!", baseRes.Status)
		}
	}

	netVerdict := "pass"
	if len(regressions) > 0 {
		netVerdict = "fail"
	} else if len(baselineDebt) > 0 {
		netVerdict = "warn"
	}

	// Generate clean HTML summary
	summaryHTML := generateSummaryHTML(regressions, baselineDebt, improvements, netVerdict)

	return &models.BaselineComparison{
		Regressions:  regressions,
		BaselineDebt: baselineDebt,
		Improvements: improvements,
		NetVerdict:   netVerdict,
		SummaryHTML:  summaryHTML,
	}
}

func generateSummaryHTML(regressions, debt, improvements map[string]string, verdict string) string {
	var sb strings.Builder
	sb.WriteString("<div class=\"forke-baseline-comparison\">\n")

	if len(regressions) > 0 {
		sb.WriteString("  <div class=\"comparison-section regressions\">\n")
		sb.WriteString("    <h4>⚠️ Net Regressions Detected</h4>\n    <ul>\n")
		for cat, msg := range regressions {
			sb.WriteString(fmt.Sprintf("      <li><strong>%s:</strong> %s</li>\n", cat, msg))
		}
		sb.WriteString("    </ul>\n  </div>\n")
	} else {
		sb.WriteString("  <div class=\"comparison-section no-regressions\">\n")
		sb.WriteString("    <p>✅ <strong>Zero regressions:</strong> All tests conform to or improve upon repository baseline.</p>\n")
		sb.WriteString("  </div>\n")
	}

	if len(improvements) > 0 {
		sb.WriteString("  <div class=\"comparison-section improvements\">\n")
		sb.WriteString("    <h4>🎉 Improvements Over Baseline</h4>\n    <ul>\n")
		for cat, msg := range improvements {
			sb.WriteString(fmt.Sprintf("      <li><strong>%s:</strong> %s</li>\n", cat, msg))
		}
		sb.WriteString("    </ul>\n  </div>\n")
	}

	if len(debt) > 0 {
		sb.WriteString("  <div class=\"comparison-section debt\">\n")
		sb.WriteString("    <p>ℹ️ <em>Pre-existing baseline debt in " + fmt.Sprintf("%d", len(debt)) + " category(s) was discounted to ensure fair scoring.</em></p>\n")
		sb.WriteString("  </div>\n")
	}

	sb.WriteString("</div>")
	return sb.String()
}
