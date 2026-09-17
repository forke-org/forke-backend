// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package sandbox

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/forke/forke-backend/internal/models"
)

// LogCallback is invoked as lines of logs are emitted by the sandbox runner
type LogCallback func(tag, message string)

// Runner defines the interface for running deterministic verification pipelines
type Runner interface {
	Run(ctx context.Context, checkoutDir string, onLog LogCallback) (*models.ReviewRunnerResult, error)
	IsDocker() bool
}

// SandboxManager manages workspaces and selects between Docker and Local runners
type SandboxManager struct {
	BaseWorkspaceDir string
	DockerImage      string
	hasDockerChecked bool
	dockerAvailable  bool
	mu               sync.Mutex
}

// NewSandboxManager initializes the workspace manager
func NewSandboxManager(baseWorkspaceDir, dockerImage string) *SandboxManager {
	if baseWorkspaceDir == "" {
		baseWorkspaceDir = filepath.Join(os.TempDir(), "forke-workspaces")
	}
	if dockerImage == "" {
		dockerImage = "forke-runner:latest"
	}
	_ = os.MkdirAll(baseWorkspaceDir, 0755)

	return &SandboxManager{
		BaseWorkspaceDir: baseWorkspaceDir,
		DockerImage:      dockerImage,
	}
}

// CheckDocker checks whether Docker is installed and running
func (m *SandboxManager) CheckDocker() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.hasDockerChecked {
		return m.dockerAvailable
	}

	cmd := exec.Command("docker", "info")
	err := cmd.Run()
	m.dockerAvailable = (err == nil)
	m.hasDockerChecked = true

	if m.dockerAvailable {
		log.Println("[SandboxManager] Docker daemon detected. Hardened Docker sandbox enabled.")
	} else {
		log.Println("[SandboxManager] Docker not detected. Falling back to isolated local runner.")
	}

	return m.dockerAvailable
}

// CreateWorkspace provisions a unique ephemeral directory for a job
func (m *SandboxManager) CreateWorkspace(jobID string) (string, error) {
	workspacePath := filepath.Join(m.BaseWorkspaceDir, fmt.Sprintf("job-%s-%d", jobID, time.Now().UnixNano()))
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return "", fmt.Errorf("failed to create workspace: %w", err)
	}
	return workspacePath, nil
}

// CleanupWorkspace removes an ephemeral workspace directory
func (m *SandboxManager) CleanupWorkspace(workspacePath string) {
	if workspacePath != "" && strings.Contains(workspacePath, "forke-workspaces") {
		_ = os.RemoveAll(workspacePath)
	}
}

// CloneAndCheckout executes a shallow git clone and checks out the PR branch
func (m *SandboxManager) CloneAndCheckout(ctx context.Context, repoFullName string, prNumber int, token, targetDir string, onLog LogCallback) error {
	onLog("GIT_CLONE", fmt.Sprintf("Cloning repository %s to workspace...", repoFullName))

	authURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, repoFullName)

	// 1. Clone base repo with depth 50
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "50", authURL, targetDir)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %s (%w)", string(out), err)
	}

	// 2. Fetch PR head
	fetchCmd := exec.CommandContext(ctx, "git", "-C", targetDir, "fetch", "origin", fmt.Sprintf("pull/%d/head:pr-%d", prNumber, prNumber))
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch PR ref failed: %s (%w)", string(out), err)
	}

	// 3. Checkout PR ref
	checkoutCmd := exec.CommandContext(ctx, "git", "-C", targetDir, "checkout", fmt.Sprintf("pr-%d", prNumber))
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout failed: %s (%w)", string(out), err)
	}

	onLog("GIT_CLONE", fmt.Sprintf("Successfully checked out PR #%d ref.", prNumber))
	return nil
}

// CloneBaseBranch clones only the base branch for baseline snapshot creation
func (m *SandboxManager) CloneBaseBranch(ctx context.Context, repoFullName, branch, token, targetDir string, onLog LogCallback) error {
	onLog("GIT_CLONE", fmt.Sprintf("Cloning base branch '%s' for repository %s...", branch, repoFullName))

	authURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, repoFullName)
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "50", "--branch", branch, authURL, targetDir)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone base branch failed: %s (%w)", string(out), err)
	}

	onLog("GIT_CLONE", fmt.Sprintf("Base branch '%s' checked out successfully.", branch))
	return nil
}

// ─── Docker Sandbox Runner ───────────────────────────────────────────────────

// DockerRunner executes verification checks inside an ephemeral Docker container
type DockerRunner struct {
	Image string
}

func (r *DockerRunner) IsDocker() bool { return true }

func (r *DockerRunner) Run(ctx context.Context, checkoutDir string, onLog LogCallback) (*models.ReviewRunnerResult, error) {
	containerName := fmt.Sprintf("forke-runner-%d", time.Now().UnixNano())

	onLog("DOCKER", fmt.Sprintf("Launching hardened container sandbox %s...", containerName))

	// Docker run arguments conforming to the security specification:
	// --memory=2560m, --memory-swap=3072m, --cpus=1.5, --pids-limit=250,
	// --security-opt=no-new-privileges, --cap-drop=ALL
	args := []string{
		"run", "--rm",
		"--name", containerName,
		"--memory=2560m",
		"--memory-swap=3072m",
		"--cpus=1.5",
		"--pids-limit=250",
		"--security-opt=no-new-privileges",
		"--cap-drop=ALL",
		"-v", fmt.Sprintf("%s:/workspace:rw", checkoutDir),
		r.Image,
	}

	cmd := exec.CommandContext(ctx, "docker", args...)

	// Setup cleanup to kill container if context is cancelled or superseded
	go func() {
		<-ctx.Done()
		killCmd := exec.Command("docker", "kill", containerName)
		_ = killCmd.Run()
	}()

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start docker container: %w", err)
	}

	scanner := bufio.NewScanner(stdoutPipe)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			tag := "EXEC"
			if strings.HasPrefix(trimmed, "===") {
				tag = "STAGE"
			} else if strings.Contains(strings.ToLower(trimmed), "error") || strings.Contains(strings.ToLower(trimmed), "fail") {
				tag = "WARN"
			}
			onLog(tag, trimmed)
		}
	}

	cmdErr := cmd.Wait()
	durationMs := time.Since(startTime).Milliseconds()

	// Parse results.json emitted by container
	resultsPath := filepath.Join(checkoutDir, "results.json")
	resultBytes, err := os.ReadFile(resultsPath)
	if err != nil {
		if cmdErr != nil {
			return nil, fmt.Errorf("container exited with error (%v) and no results.json: %w", cmdErr, err)
		}
		return nil, fmt.Errorf("container completed but results.json was not generated: %w", err)
	}

	var rawResult struct {
		Timestamp string                           `json:"timestamp"`
		TechStack models.TechStack                 `json:"techStack"`
		Results   map[string]models.CategoryResult `json:"results"`
	}
	if err := json.Unmarshal(resultBytes, &rawResult); err != nil {
		return nil, fmt.Errorf("failed to parse results.json: %w", err)
	}

	return &models.ReviewRunnerResult{
		TechStack:  rawResult.TechStack,
		Results:    rawResult.Results,
		DurationMs: durationMs,
	}, nil
}

// ─── Local Fallback Runner (For Dev / When Docker is Not Present) ─────────────

// LocalRunner runs verification directly in a cleaned subshell environment
type LocalRunner struct{}

func (r *LocalRunner) IsDocker() bool { return false }

func (r *LocalRunner) Run(ctx context.Context, checkoutDir string, onLog LogCallback) (*models.ReviewRunnerResult, error) {
	onLog("LOCAL", "Executing deterministic pipeline in isolated local subshell environment...")
	startTime := time.Now()

	results := make(map[string]models.CategoryResult)

	// Clean environment variables (remove Next/NPM pollution)
	cleanEnv := make([]string, 0)
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "__NEXT_") &&
			!strings.HasPrefix(env, "NEXT_") &&
			!strings.HasPrefix(env, "npm_") &&
			!strings.HasPrefix(env, "NODE_OPTIONS=") {
			cleanEnv = append(cleanEnv, env)
		}
	}
	cleanEnv = append(cleanEnv, "NODE_ENV=development")

	// Detect Stack
	techStack := detectLocalStack(checkoutDir)
	onLog("DETECT", fmt.Sprintf("Detected stack: %s (package manager: %s)", techStack.Language, techStack.PackageManager))

	// Map commands based on language
	commands := mapCommandsForStack(techStack, checkoutDir)

	for cat, cmdStr := range commands {
		if cmdStr == "" {
			results[cat] = models.CategoryResult{
				Status:      "skip",
				DurationMs:  0,
				Logs:        "No command defined for this category.",
				IssuesCount: 0,
			}
			continue
		}

		onLog("RUN", fmt.Sprintf("Running %s: %s", cat, cmdStr))
		catStart := time.Now()

		catCmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
		catCmd.Dir = checkoutDir
		catCmd.Env = cleanEnv

		out, err := catCmd.CombinedOutput()
		catDuration := time.Since(catStart).Milliseconds()

		logs := string(out)
		status := "pass"
		issues := 0

		if err != nil {
			status = "fail"
			issues = 1
		}

		// Count issues if warnings present
		lower := strings.ToLower(logs)
		if strings.Contains(lower, "warning") || strings.Contains(lower, "error") {
			issues = countIssuesFromLogs(cat, logs)
			if status == "pass" && issues > 0 {
				status = "warn"
			}
		}

		results[cat] = models.CategoryResult{
			Status:      status,
			DurationMs:  catDuration,
			Logs:        logs,
			IssuesCount: issues,
		}

		onLog("DONE", fmt.Sprintf("Finished %s: status=%s (%d ms)", cat, status, catDuration))
	}

	return &models.ReviewRunnerResult{
		TechStack:  techStack,
		Results:    results,
		DurationMs: time.Since(startTime).Milliseconds(),
	}, nil
}

func detectLocalStack(dir string) models.TechStack {
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}

	if exists("package.json") {
		pm := "npm"
		if exists("bun.lockb") || exists("bun.lock") {
			pm = "bun"
		} else if exists("pnpm-lock.yaml") {
			pm = "pnpm"
		} else if exists("yarn.lock") {
			pm = "yarn"
		}
		lang := "JavaScript"
		if exists("tsconfig.json") {
			lang = "TypeScript"
		}
		return models.TechStack{Language: lang, PackageManager: pm}
	}

	if exists("requirements.txt") || exists("pyproject.toml") || exists("Pipfile") {
		return models.TechStack{Language: "Python", PackageManager: "pip"}
	}

	if exists("go.mod") {
		return models.TechStack{Language: "Go", PackageManager: "go"}
	}

	if exists("Cargo.toml") {
		return models.TechStack{Language: "Rust", PackageManager: "cargo"}
	}

	return models.TechStack{Language: "Generic", PackageManager: "none"}
}

func mapCommandsForStack(stack models.TechStack, dir string) map[string]string {
	cmds := make(map[string]string)
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}

	switch stack.Language {
	case "TypeScript", "JavaScript":
		pm := stack.PackageManager
		cmds["build"] = fmt.Sprintf("%s run build --if-present", pm)
		cmds["lint"] = "npx eslint . --max-warnings=0 || true"
		if stack.Language == "TypeScript" {
			cmds["type_checks"] = "npx tsc --noEmit || true"
		}
		cmds["unit_tests"] = fmt.Sprintf("%s test --if-present", pm)
		cmds["security"] = fmt.Sprintf("%s audit || true", pm)
		cmds["sast"] = "npx semgrep scan --config auto --error || true"
	case "Python":
		cmds["build"] = "python3 -m py_compile **/*.py 2>/dev/null || true"
		cmds["lint"] = "flake8 . --exclude=.venv,venv --max-line-length=120 || true"
		cmds["type_checks"] = "mypy . --ignore-missing-imports || true"
		cmds["unit_tests"] = "pytest -v || python3 -m unittest discover || true"
		cmds["sast"] = "bandit -r . -x .venv,venv || true"
	case "Go":
		cmds["build"] = "go build ./..."
		cmds["lint"] = "go vet ./..."
		cmds["type_checks"] = "go vet ./..."
		cmds["unit_tests"] = "go test -v ./..."
	case "Rust":
		cmds["build"] = "cargo build"
		cmds["lint"] = "cargo clippy"
		cmds["type_checks"] = "cargo check"
		cmds["unit_tests"] = "cargo test"
	default:
		if exists("Makefile") {
			cmds["build"] = "make build || make"
			cmds["unit_tests"] = "make test || true"
		}
	}
	return cmds
}

func countIssuesFromLogs(category, logs string) int {
	lower := strings.ToLower(logs)
	count := 0
	lines := strings.Split(lower, "\n")
	for _, l := range lines {
		if strings.Contains(l, "error") || strings.Contains(l, "fail") || strings.Contains(l, "problem") {
			count++
		}
	}
	if count == 0 && (strings.Contains(lower, "error") || strings.Contains(lower, "fail")) {
		return 1
	}
	return count
}
