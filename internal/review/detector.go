// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/forke/forke-backend/internal/models"
)

// DetectStack examines repository root files to classify language, package manager, and framework
func DetectStack(checkoutDir string) models.TechStack {
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(checkoutDir, name))
		return err == nil
	}

	readFile := func(name string) string {
		content, err := os.ReadFile(filepath.Join(checkoutDir, name))
		if err != nil {
			return ""
		}
		return string(content)
	}

	// 1. Node / TypeScript / JavaScript
	if exists("package.json") {
		pkgContent := readFile("package.json")
		var pkgJSON struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
			Workspaces      interface{}       `json:"workspaces"`
		}
		_ = json.Unmarshal([]byte(pkgContent), &pkgJSON)

		// Determine language
		lang := "JavaScript"
		if exists("tsconfig.json") || strings.Contains(pkgContent, "\"typescript\"") {
			lang = "TypeScript"
		}

		// Determine package manager
		pm := "npm"
		if exists("bun.lockb") || exists("bun.lock") {
			pm = "bun"
		} else if exists("pnpm-lock.yaml") {
			pm = "pnpm"
		} else if exists("yarn.lock") {
			pm = "yarn"
		}

		// Detect Framework
		framework := "Node.js"
		if hasDep(pkgJSON.Dependencies, pkgJSON.DevDependencies, "next") {
			framework = "Next.js"
		} else if hasDep(pkgJSON.Dependencies, pkgJSON.DevDependencies, "react") {
			framework = "React"
		} else if hasDep(pkgJSON.Dependencies, pkgJSON.DevDependencies, "vue") {
			framework = "Vue"
		} else if hasDep(pkgJSON.Dependencies, pkgJSON.DevDependencies, "@angular/core") {
			framework = "Angular"
		} else if hasDep(pkgJSON.Dependencies, pkgJSON.DevDependencies, "express") {
			framework = "Express"
		} else if hasDep(pkgJSON.Dependencies, pkgJSON.DevDependencies, "fastify") {
			framework = "Fastify"
		}

		// Detect Monorepo
		isMonorepo := false
		monorepoType := ""
		if exists("turbo.json") {
			isMonorepo = true
			monorepoType = "turborepo"
		} else if exists("nx.json") {
			isMonorepo = true
			monorepoType = "nx"
		} else if exists("pnpm-workspace.yaml") || pkgJSON.Workspaces != nil {
			isMonorepo = true
			monorepoType = "npm-workspaces"
		}

		return models.TechStack{
			Language:       lang,
			Framework:      framework,
			PackageManager: pm,
			IsMonorepo:     isMonorepo,
			MonorepoType:   monorepoType,
		}
	}

	// 2. Python
	if exists("requirements.txt") || exists("pyproject.toml") || exists("Pipfile") || exists("setup.py") {
		framework := "Python"
		pyContent := readFile("requirements.txt") + readFile("pyproject.toml")
		if strings.Contains(pyContent, "fastapi") {
			framework = "FastAPI"
		} else if strings.Contains(pyContent, "django") {
			framework = "Django"
		} else if strings.Contains(pyContent, "flask") {
			framework = "Flask"
		}

		pm := "pip"
		if exists("poetry.lock") {
			pm = "poetry"
		} else if exists("Pipfile") {
			pm = "pipenv"
		}

		return models.TechStack{
			Language:       "Python",
			Framework:      framework,
			PackageManager: pm,
		}
	}

	// 3. Go
	if exists("go.mod") {
		goMod := readFile("go.mod")
		framework := "Standard Library"
		if strings.Contains(goMod, "github.com/gin-gonic/gin") {
			framework = "Gin"
		} else if strings.Contains(goMod, "github.com/go-chi/chi") {
			framework = "Chi"
		} else if strings.Contains(goMod, "github.com/labstack/echo") {
			framework = "Echo"
		} else if strings.Contains(goMod, "github.com/gofiber/fiber") {
			framework = "Fiber"
		}

		return models.TechStack{
			Language:       "Go",
			Framework:      framework,
			PackageManager: "go",
		}
	}

	// 4. Rust
	if exists("Cargo.toml") {
		return models.TechStack{
			Language:       "Rust",
			PackageManager: "cargo",
		}
	}

	// 5. Java
	if exists("pom.xml") {
		return models.TechStack{
			Language:       "Java",
			BuildTool:      "Maven",
			PackageManager: "mvn",
		}
	}
	if exists("build.gradle") || exists("build.gradle.kts") {
		return models.TechStack{
			Language:       "Java",
			BuildTool:      "Gradle",
			PackageManager: "gradle",
		}
	}

	return models.TechStack{
		Language:       "Generic",
		PackageManager: "none",
	}
}

func hasDep(deps, devDeps map[string]string, key string) bool {
	if deps != nil {
		if _, ok := deps[key]; ok {
			return true
		}
	}
	if devDeps != nil {
		if _, ok := devDeps[key]; ok {
			return true
		}
	}
	return false
}
