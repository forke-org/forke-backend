// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package review

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/forke/forke-backend/internal/models"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret|token|password|auth[_-]?token)["']?\s*[:=]\s*["']([a-zA-Z0-9_\-\.]{16,})["']`),
	regexp.MustCompile(`ghp_[0-9a-zA-Z]{36}`),                           // GitHub Personal Access Token
	regexp.MustCompile(`gho_[0-9a-zA-Z]{36}`),                           // GitHub OAuth Token
	regexp.MustCompile(`xox[baprs]-[0-9a-zA-Z]{10,48}`),                 // Slack Token
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),                              // AWS Access Key ID
	regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`),                        // Google API Key
	regexp.MustCompile(`-----BEGIN (RSA|EC|OPENSSH|PRIVATE) KEY-----`), // Private Keys
}

// ParsePathList splits comma or newline-separated globs into clean slices
func ParsePathList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			clean = append(clean, trimmed)
		}
	}
	return clean
}

// ValidateFileBoundaries checks if any changed files violate allowed or restricted paths
func ValidateFileBoundaries(changedFiles []string, allowedPaths, restrictedPaths []string) []string {
	var unauthorized []string

	for _, file := range changedFiles {
		// 1. Check explicit restricted paths first
		if len(restrictedPaths) > 0 {
			for _, pattern := range restrictedPaths {
				if matchGlob(pattern, file) {
					unauthorized = append(unauthorized, file)
					break
				}
			}
		}

		// 2. Check allowed paths (if defined, file must match at least one)
		if len(allowedPaths) > 0 {
			matched := false
			for _, pattern := range allowedPaths {
				if matchGlob(pattern, file) {
					matched = true
					break
				}
			}
			if !matched {
				unauthorized = append(unauthorized, file)
			}
		}
	}

	return deduplicateStrings(unauthorized)
}

// ScanSecrets inspects changed files within the checkout directory for credential leaks
func ScanSecrets(checkoutDir string, changedFiles []string) ([]models.SecretFinding, error) {
	var findings []models.SecretFinding

	for _, relPath := range changedFiles {
		fullPath := filepath.Join(checkoutDir, relPath)
		fileInfo, err := os.Stat(fullPath)
		if err != nil || fileInfo.IsDir() {
			continue
		}

		// Skip binaries and lock files
		if isIgnoredFile(relPath) {
			continue
		}

		f, err := os.Open(fullPath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		lineNum := 1
		for scanner.Scan() {
			lineText := scanner.Text()
			for _, pat := range secretPatterns {
				if pat.MatchString(lineText) {
					findings = append(findings, models.SecretFinding{
						File: relPath,
						Line: lineNum,
						Type: "potential_secret_leak",
					})
					break
				}
			}
			lineNum++
		}
		_ = f.Close()
	}

	return findings, nil
}

// ValidateForkeSubmission checks whether FORKE_SUBMISSION.md is present and non-empty
func ValidateForkeSubmission(checkoutDir string) (bool, string) {
	submissionPath := filepath.Join(checkoutDir, "FORKE_SUBMISSION.md")
	info, err := os.Stat(submissionPath)
	if err != nil {
		// Try lowercase
		submissionPath = filepath.Join(checkoutDir, "forke_submission.md")
		info, err = os.Stat(submissionPath)
		if err != nil {
			return false, "Missing FORKE_SUBMISSION.md in repository root."
		}
	}

	if info.Size() < 50 {
		return false, "FORKE_SUBMISSION.md is empty or incomplete (under 50 bytes)."
	}

	return true, ""
}

func matchGlob(pattern, path string) bool {
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// Direct prefix match for directories (e.g. src/ or src/**)
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return strings.HasPrefix(path, prefix+"/") || path == prefix
	}

	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}

	// Also test filename alone
	matched, _ = filepath.Match(pattern, filepath.Base(path))
	return matched
}

func isIgnoredFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".lock") ||
		strings.HasSuffix(lower, "lockb") ||
		strings.HasSuffix(lower, ".png") ||
		strings.HasSuffix(lower, ".jpg") ||
		strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".svg") ||
		strings.HasSuffix(lower, ".ico") ||
		strings.HasSuffix(lower, ".pdf") ||
		strings.HasSuffix(lower, ".min.js") ||
		strings.Contains(lower, "node_modules/")
}

func deduplicateStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
