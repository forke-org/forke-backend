// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package review

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePathList(t *testing.T) {
	raw := "src/features/**, src/components/*\npackage.json,  .env"
	paths := ParsePathList(raw)

	if len(paths) != 4 {
		t.Fatalf("expected 4 paths, got %d", len(paths))
	}
	if paths[0] != "src/features/**" || paths[1] != "src/components/*" {
		t.Errorf("unexpected parsed paths: %v", paths)
	}
}

func TestValidateFileBoundaries(t *testing.T) {
	allowed := []string{"src/features/**", "README.md"}
	restricted := []string{"package.json", ".env*"}

	changed := []string{
		"src/features/auth/login.tsx",
		"README.md",
		"package.json",
		"src/utils/hack.ts",
	}

	unauthorized := ValidateFileBoundaries(changed, allowed, restricted)

	// package.json is restricted, src/utils/hack.ts is not in allowed
	if len(unauthorized) != 2 {
		t.Fatalf("expected 2 unauthorized files, got %d: %v", len(unauthorized), unauthorized)
	}

	hasPkg := false
	hasHack := false
	for _, u := range unauthorized {
		if u == "package.json" {
			hasPkg = true
		}
		if u == "src/utils/hack.ts" {
			hasHack = true
		}
	}

	if !hasPkg || !hasHack {
		t.Errorf("expected package.json and hack.ts to be unauthorized: %v", unauthorized)
	}
}

func TestScanSecrets(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "secret-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	secretFile := filepath.Join(tempDir, "config.ts")
	_ = os.WriteFile(secretFile, []byte("const key = \"AKIA1234567890ABCDEF\";\nconst ghp = \"ghp_123456789012345678901234567890123456\";"), 0644)

	cleanFile := filepath.Join(tempDir, "clean.ts")
	_ = os.WriteFile(cleanFile, []byte("export const name = 'Forke';"), 0644)

	findings, err := ScanSecrets(tempDir, []string{"config.ts", "clean.ts"})
	if err != nil {
		t.Fatalf("unexpected error scanning secrets: %v", err)
	}

	if len(findings) < 2 {
		t.Errorf("expected at least 2 secret findings, got %d", len(findings))
	}
}

func TestValidateForkeSubmission(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "submission-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Missing file
	valid, _ := ValidateForkeSubmission(tempDir)
	if valid {
		t.Errorf("expected invalid for missing FORKE_SUBMISSION.md")
	}

	// 2. Empty file
	subPath := filepath.Join(tempDir, "FORKE_SUBMISSION.md")
	_ = os.WriteFile(subPath, []byte("tiny"), 0644)
	valid, _ = ValidateForkeSubmission(tempDir)
	if valid {
		t.Errorf("expected invalid for tiny file under 50 bytes")
	}

	// 3. Valid file
	content := "# Forke Submission Report\n\nImplemented auth workflow, unit tests, and security validation."
	_ = os.WriteFile(subPath, []byte(content), 0644)
	valid, errStr := ValidateForkeSubmission(tempDir)
	if !valid {
		t.Errorf("expected valid submission report, got error: %s", errStr)
	}
}
