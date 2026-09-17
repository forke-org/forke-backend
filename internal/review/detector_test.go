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

func TestDetectStack(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "stack-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Test Node / TypeScript detection
	_ = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"dependencies": {"next": "^14.0.0"}}`), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), []byte(`{}`), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "bun.lockb"), []byte(``), 0644)

	stack := DetectStack(tempDir)
	if stack.Language != "TypeScript" {
		t.Errorf("expected Language=TypeScript, got %s", stack.Language)
	}
	if stack.Framework != "Next.js" {
		t.Errorf("expected Framework=Next.js, got %s", stack.Framework)
	}
	if stack.PackageManager != "bun" {
		t.Errorf("expected PackageManager=bun, got %s", stack.PackageManager)
	}

	// 2. Test Python FastAPI detection
	pyDir, _ := os.MkdirTemp("", "py-test-*")
	defer os.RemoveAll(pyDir)
	_ = os.WriteFile(filepath.Join(pyDir, "requirements.txt"), []byte("fastapi==0.109.0\nuvicorn==0.27.0"), 0644)

	pyStack := DetectStack(pyDir)
	if pyStack.Language != "Python" {
		t.Errorf("expected Language=Python, got %s", pyStack.Language)
	}
	if pyStack.Framework != "FastAPI" {
		t.Errorf("expected Framework=FastAPI, got %s", pyStack.Framework)
	}
	if pyStack.PackageManager != "pip" {
		t.Errorf("expected PackageManager=pip, got %s", pyStack.PackageManager)
	}

	// 3. Test Go detection
	goDir, _ := os.MkdirTemp("", "go-test-*")
	defer os.RemoveAll(goDir)
	_ = os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module myapp\n\ngo 1.22\n\nrequire github.com/go-chi/chi v5.0.0"), 0644)

	goStack := DetectStack(goDir)
	if goStack.Language != "Go" {
		t.Errorf("expected Language=Go, got %s", goStack.Language)
	}
	if goStack.Framework != "Chi" {
		t.Errorf("expected Framework=Chi, got %s", goStack.Framework)
	}
}
