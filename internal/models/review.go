// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package models

import "time"

// ─── Database Entity Models ──────────────────────────────────────────────────

// SandboxRepo represents a task sandbox mirrored on Forke
type SandboxRepo struct {
	ID                 string    `json:"id"`
	OwnerID            string    `json:"owner_id"`
	SourceRepo         string    `json:"source_repo"`
	SandboxRepo        string    `json:"sandbox_repo"`
	TaskTitle          string    `json:"task_title"`
	TaskDescription    string    `json:"task_description"`
	FrontendStack      string    `json:"frontend_stack"`
	BackendStack       string    `json:"backend_stack"`
	AllowedPaths       string    `json:"allowed_paths"`
	RestrictedPaths    string    `json:"restricted_paths"`
	AcceptanceCriteria string    `json:"acceptance_criteria"`
	VerificationStatus string    `json:"verification_status"`
	CreatedAt          time.Time `json:"created_at"`
}

// SandboxUser represents an owner or developer with GitHub token
type SandboxUser struct {
	ID          string    `json:"id"`
	GitHubID    int64     `json:"github_id"`
	Username    string    `json:"username"`
	AccessToken string    `json:"access_token"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

// DeveloperFork tracks a developer's fork of a sandbox repository
type DeveloperFork struct {
	ID             string    `json:"id"`
	GitHubUsername string    `json:"github_username"`
	SandboxRepo    string    `json:"sandbox_repo"`
	ForkURL        string    `json:"fork_url"`
	PRURL          *string   `json:"pr_url,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// BaselineSnapshot stores the target branch's baseline health state
type BaselineSnapshot struct {
	ID            string    `json:"id"`
	SandboxRepoID string    `json:"sandbox_repo_id"`
	Branch        string    `json:"branch"`
	CommitSHA     string    `json:"commit_sha"`
	TechStack     string    `json:"tech_stack"`  // JSON string of TechStack
	Results       string    `json:"results"`     // JSON string of 12 category results
	AISummary     string    `json:"ai_summary"`  // Summary text or JSON
	CreatedAt     time.Time `json:"created_at"`
}

// CodeReview stores unified deterministic verification and AI review findings
type CodeReview struct {
	ID                 string    `json:"id"`
	SandboxRepoID      string    `json:"sandbox_repo_id"`
	DeveloperForkID    *string   `json:"developer_fork_id,omitempty"`
	PRNumber           int       `json:"pr_number"`
	CommitSHA          string    `json:"commit_sha"`
	BaselineSnapshotID *string   `json:"baseline_snapshot_id,omitempty"`
	Results            string    `json:"results"` // JSON of 12 categories
	Comparison         string    `json:"comparison,omitempty"`
	Verdict            string    `json:"verdict"` // pass, warn, fail
	ReportHTML         string    `json:"report_html,omitempty"`
	AIVerdict          string    `json:"ai_verdict"` // pass, needs_changes, high_risk
	AIScore            int       `json:"ai_score"`
	RequirementMatch   string    `json:"requirement_match,omitempty"`
	AISummary          string    `json:"ai_summary,omitempty"`
	AIStrengths        string    `json:"ai_strengths,omitempty"` // JSON array
	AIIssues           string    `json:"ai_issues,omitempty"`    // JSON array
	AIRisks            string    `json:"ai_risks,omitempty"`     // JSON array
	UnauthorizedEdits  string    `json:"unauthorized_edits,omitempty"`
	ResolvedIssues     string    `json:"resolved_issues,omitempty"`
	ResolvedRisks      string    `json:"resolved_risks,omitempty"`
	RiskScore          int       `json:"risk_score"`
	RiskRouting        string    `json:"risk_routing"` // auto_approve, owner_review, reviewer_queue
	AIModel            string    `json:"ai_model,omitempty"`
	TokensUsed         int       `json:"tokens_used,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

// ─── Deterministic Engine Types ──────────────────────────────────────────────

// CategoryResult models execution findings for a single deterministic category
type CategoryResult struct {
	Status      string      `json:"status"` // pass, fail, warn, skip
	DurationMs  int64       `json:"durationMs"`
	Logs        string      `json:"logs"`
	IssuesCount int         `json:"issuesCount"`
	Details     interface{} `json:"details,omitempty"`
}

// TechStack describes detected language, tools, and package managers
type TechStack struct {
	Language      string `json:"language"`
	Framework     string `json:"framework,omitempty"`
	PackageManager string `json:"packageManager"`
	BuildTool     string `json:"buildTool,omitempty"`
	TestFramework string `json:"testFramework,omitempty"`
	IsMonorepo    bool   `json:"isMonorepo"`
	MonorepoType  string `json:"monorepoType,omitempty"`
}

// ReviewRunnerResult captures all 12 category executions
type ReviewRunnerResult struct {
	CommitSHA  string                    `json:"commitSha"`
	TechStack  TechStack                 `json:"techStack"`
	Results    map[string]CategoryResult `json:"results"`
	DurationMs int64                     `json:"durationMs"`
}

// CategoryScore tracks individual category weighting and earned quality
type CategoryScore struct {
	Name         string  `json:"name"`
	Weight       float64 `json:"weight"`
	Quality      float64 `json:"quality"`
	Contribution float64 `json:"contribution"`
	Status       string  `json:"status"`
	IssuesCount  int     `json:"issuesCount"`
}

// ScorePenalty represents a deduction for security/boundary violations
type ScorePenalty struct {
	Reason string  `json:"reason"`
	Points float64 `json:"points"`
}

// TestScoreResult aggregates the deterministic scoring algorithm
type TestScoreResult struct {
	RawScore    int             `json:"rawScore"`
	TestScore   int             `json:"testScore"`
	BuildFailed bool            `json:"buildFailed"`
	Penalties   []ScorePenalty  `json:"penalties"`
	Categories  []CategoryScore `json:"categories"`
}

// BaselineComparison represents delta findings between PR and baseline snapshot
type BaselineComparison struct {
	Regressions   map[string]string `json:"regressions"`   // Category -> regression reason
	BaselineDebt  map[string]string `json:"baselineDebt"`  // Category -> pre-existing issues
	Improvements  map[string]string `json:"improvements"`  // Category -> resolved issues
	NetVerdict    string            `json:"netVerdict"`    // pass, warn, fail
	SummaryHTML   string            `json:"summaryHtml"`
}

// SecretFinding models hardcoded secrets flagged during scan
type SecretFinding struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Type string `json:"type"`
}

// ─── AI Reasoning Types ──────────────────────────────────────────────────────

// AIIssue models a specific code issue found by the LLM
type AIIssue struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Severity   string `json:"severity"` // critical, high, medium, low
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
	Status     string `json:"status"` // new, existing, resolved
}

// AIRisk models an architectural or security risk flagged by the LLM
type AIRisk struct {
	Category string `json:"category"` // credential, security, logic, compliance
	Message  string `json:"message"`
	Severity string `json:"severity"` // critical, high, medium, low
	Status   string `json:"status"`   // new, resolved
}

// AIReviewResult is the structured output from Gemini Flash
type AIReviewResult struct {
	Verdict           string    `json:"verdict"` // pass, needs_changes, high_risk
	Score             int       `json:"score"`   // 0 - 100
	RequirementMatch  float64   `json:"requirement_match"`
	Summary           string    `json:"summary"`
	Strengths         []string  `json:"strengths"`
	Issues            []AIIssue `json:"issues"`
	Risks             []AIRisk  `json:"risks"`
	UnauthorizedEdits []string  `json:"unauthorized_file_edits"`
	TokensUsed        int       `json:"tokens_used"`
	Model             string    `json:"model"`
}

// ─── Background Job Queue Types ──────────────────────────────────────────────

// ReviewJob represents a review task queued or running in the Go worker pool
type ReviewJob struct {
	ID             string     `json:"id"`
	SandboxRepo    string     `json:"sandbox_repo"`
	PRNumber       int        `json:"pr_number"`
	HeadSHA        string     `json:"head_sha"`
	Username       string     `json:"username"`
	Status         string     `json:"status"` // queued, running, success, failed, cancelled
	Progress       int        `json:"progress"`
	Logs           []string   `json:"logs"`
	CreatedAt      time.Time  `json:"created_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	Error          string     `json:"error,omitempty"`
	ReviewRecordID *string    `json:"review_record_id,omitempty"`
}
