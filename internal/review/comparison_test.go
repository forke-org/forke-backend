// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package review

import (
	"testing"

	"github.com/forke/forke-backend/internal/models"
)

func TestCompareToBaseline(t *testing.T) {
	// Baseline: lint had 2 errors, build passed
	baseline := map[string]models.CategoryResult{
		"build":      {Status: "pass", IssuesCount: 0},
		"lint":       {Status: "warn", IssuesCount: 2},
		"unit_tests": {Status: "pass", IssuesCount: 0},
	}

	// PR 1: Net regression (unit_tests failed)
	pr1 := map[string]models.CategoryResult{
		"build":      {Status: "pass", IssuesCount: 0},
		"lint":       {Status: "warn", IssuesCount: 2},
		"unit_tests": {Status: "fail", IssuesCount: 1},
	}

	comp1 := CompareToBaseline(baseline, pr1)
	if comp1.NetVerdict != "fail" {
		t.Errorf("expected NetVerdict=fail, got %s", comp1.NetVerdict)
	}
	if _, ok := comp1.Regressions["unit_tests"]; !ok {
		t.Errorf("expected unit_tests to be marked as regression")
	}
	if _, ok := comp1.BaselineDebt["lint"]; !ok {
		t.Errorf("expected lint to be marked as baseline debt")
	}

	// PR 2: Zero regressions (lint has 2 errors just like baseline, tests pass)
	pr2 := map[string]models.CategoryResult{
		"build":      {Status: "pass", IssuesCount: 0},
		"lint":       {Status: "warn", IssuesCount: 2},
		"unit_tests": {Status: "pass", IssuesCount: 0},
	}

	comp2 := CompareToBaseline(baseline, pr2)
	if comp2.NetVerdict != "warn" { // "warn" due to baseline debt, but zero regressions
		t.Errorf("expected NetVerdict=warn, got %s", comp2.NetVerdict)
	}
	if len(comp2.Regressions) != 0 {
		t.Errorf("expected 0 regressions, got %d", len(comp2.Regressions))
	}

	// PR 3: Improvement (resolved the 2 lint errors)
	pr3 := map[string]models.CategoryResult{
		"build":      {Status: "pass", IssuesCount: 0},
		"lint":       {Status: "pass", IssuesCount: 0},
		"unit_tests": {Status: "pass", IssuesCount: 0},
	}

	comp3 := CompareToBaseline(baseline, pr3)
	if comp3.NetVerdict != "pass" {
		t.Errorf("expected NetVerdict=pass, got %s", comp3.NetVerdict)
	}
	if _, ok := comp3.Improvements["lint"]; !ok {
		t.Errorf("expected lint to be marked as improvement")
	}
}
