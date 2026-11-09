// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package database

import (
	"context"
	"errors"
	"fmt"
	"github.com/forke/forke-backend/internal/tokencrypt"

	"github.com/forke/forke-backend/internal/models"
	"github.com/jackc/pgx/v5"
)

// GetSandboxRepoBySandbox finds a sandbox repository record by its mirrored full name
func (db *DB) GetSandboxRepoBySandbox(ctx context.Context, sandboxRepo string) (*models.SandboxRepo, error) {
	query := `
		SELECT id, owner_id, source_repo, sandbox_repo,
		       COALESCE(task_title, ''), COALESCE(task_description, ''),
		       COALESCE(frontend_stack, ''), COALESCE(backend_stack, ''),
		       COALESCE(allowed_paths, ''), COALESCE(restricted_paths, ''),
		       COALESCE(acceptance_criteria, ''), COALESCE(verification_status, 'verifying'),
		       created_at
		FROM sandbox_repos
		WHERE sandbox_repo = $1
		LIMIT 1;
	`
	var repo models.SandboxRepo
	err := db.Pool.QueryRow(ctx, query, sandboxRepo).Scan(
		&repo.ID, &repo.OwnerID, &repo.SourceRepo, &repo.SandboxRepo,
		&repo.TaskTitle, &repo.TaskDescription,
		&repo.FrontendStack, &repo.BackendStack,
		&repo.AllowedPaths, &repo.RestrictedPaths,
		&repo.AcceptanceCriteria, &repo.VerificationStatus,
		&repo.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query sandbox_repos: %w", err)
	}
	return &repo, nil
}

// GetSandboxRepoByID finds a sandbox repository record by its primary UUID
func (db *DB) GetSandboxRepoByID(ctx context.Context, id string) (*models.SandboxRepo, error) {
	query := `
		SELECT id, owner_id, source_repo, sandbox_repo,
		       COALESCE(task_title, ''), COALESCE(task_description, ''),
		       COALESCE(frontend_stack, ''), COALESCE(backend_stack, ''),
		       COALESCE(allowed_paths, ''), COALESCE(restricted_paths, ''),
		       COALESCE(acceptance_criteria, ''), COALESCE(verification_status, 'verifying'),
		       created_at
		FROM sandbox_repos
		WHERE id = $1
		LIMIT 1;
	`
	var repo models.SandboxRepo
	err := db.Pool.QueryRow(ctx, query, id).Scan(
		&repo.ID, &repo.OwnerID, &repo.SourceRepo, &repo.SandboxRepo,
		&repo.TaskTitle, &repo.TaskDescription,
		&repo.FrontendStack, &repo.BackendStack,
		&repo.AllowedPaths, &repo.RestrictedPaths,
		&repo.AcceptanceCriteria, &repo.VerificationStatus,
		&repo.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query sandbox_repos by id: %w", err)
	}
	return &repo, nil
}

// GetDeveloperFork finds the developer's registered fork record
func (db *DB) GetDeveloperFork(ctx context.Context, username, sandboxRepo string) (*models.DeveloperFork, error) {
	query := `
		SELECT id, github_username, sandbox_repo, fork_url, pr_url, created_at
		FROM developer_forks
		WHERE github_username = $1 AND sandbox_repo = $2
		LIMIT 1;
	`
	var fork models.DeveloperFork
	err := db.Pool.QueryRow(ctx, query, username, sandboxRepo).Scan(
		&fork.ID, &fork.GitHubUsername, &fork.SandboxRepo, &fork.ForkURL, &fork.PRURL, &fork.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query developer_forks: %w", err)
	}
	return &fork, nil
}

// GetSandboxOwnerToken retrieves the owner's GitHub OAuth access token
func (db *DB) GetSandboxOwnerToken(ctx context.Context, ownerID string) (string, error) {
	query := `
		SELECT access_token
		FROM sandbox_users
		WHERE id = $1 AND role = 'owner'
		LIMIT 1;
	`
	var token string
	err := db.Pool.QueryRow(ctx, query, ownerID).Scan(&token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("owner with id %s not found in sandbox_users", ownerID)
		}
		return "", fmt.Errorf("failed to query sandbox_users: %w", err)
	}
	return tokencrypt.Decrypt(token)
}

// GetLatestBaselineSnapshot retrieves the most recent baseline snapshot for a sandbox
func (db *DB) GetLatestBaselineSnapshot(ctx context.Context, sandboxRepoID string) (*models.BaselineSnapshot, error) {
	query := `
		SELECT id, sandbox_repo_id, branch, commit_sha,
		       COALESCE(tech_stack, '{}'), COALESCE(results, '{}'), COALESCE(ai_summary, ''),
		       created_at
		FROM baseline_snapshots
		WHERE sandbox_repo_id = $1
		ORDER BY created_at DESC
		LIMIT 1;
	`
	var snapshot models.BaselineSnapshot
	err := db.Pool.QueryRow(ctx, query, sandboxRepoID).Scan(
		&snapshot.ID, &snapshot.SandboxRepoID, &snapshot.Branch, &snapshot.CommitSHA,
		&snapshot.TechStack, &snapshot.Results, &snapshot.AISummary, &snapshot.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query baseline_snapshots: %w", err)
	}
	return &snapshot, nil
}

// InsertBaselineSnapshot records a new baseline snapshot in the database
func (db *DB) InsertBaselineSnapshot(ctx context.Context, s *models.BaselineSnapshot) error {
	query := `
		INSERT INTO baseline_snapshots (
			sandbox_repo_id, branch, commit_sha, tech_stack, results, ai_summary, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING id, created_at;
	`
	err := db.Pool.QueryRow(ctx, query,
		s.SandboxRepoID, s.Branch, s.CommitSHA, s.TechStack, s.Results, s.AISummary,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert baseline_snapshot: %w", err)
	}
	return nil
}

// InsertCodeReview persists a full unified PR review to PostgreSQL
func (db *DB) InsertCodeReview(ctx context.Context, r *models.CodeReview) error {
	query := `
		INSERT INTO code_reviews (
			sandbox_repo_id, developer_fork_id, pr_number, commit_sha, baseline_snapshot_id,
			results, comparison, verdict, report_html,
			ai_verdict, ai_score, requirement_match, ai_summary,
			ai_strengths, ai_issues, ai_risks, unauthorized_edits,
			resolved_issues, resolved_risks, risk_score, risk_routing,
			ai_model, tokens_used, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $17,
			$18, $19, $20, $21,
			$22, $23, NOW()
		)
		RETURNING id, created_at;
	`
	err := db.Pool.QueryRow(ctx, query,
		r.SandboxRepoID, r.DeveloperForkID, r.PRNumber, r.CommitSHA, r.BaselineSnapshotID,
		r.Results, r.Comparison, r.Verdict, r.ReportHTML,
		r.AIVerdict, r.AIScore, r.RequirementMatch, r.AISummary,
		r.AIStrengths, r.AIIssues, r.AIRisks, r.UnauthorizedEdits,
		r.ResolvedIssues, r.ResolvedRisks, r.RiskScore, r.RiskRouting,
		r.AIModel, r.TokensUsed,
	).Scan(&r.ID, &r.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert code_review: %w", err)
	}
	return nil
}

// GetLatestCodeReviewForPR retrieves the latest review for a PR within a sandbox
func (db *DB) GetLatestCodeReviewForPR(ctx context.Context, sandboxRepoID string, prNumber int) (*models.CodeReview, error) {
	query := `
		SELECT id, sandbox_repo_id, developer_fork_id, pr_number, commit_sha, baseline_snapshot_id,
		       COALESCE(results, '{}'), COALESCE(comparison, '{}'), COALESCE(verdict, 'pass'), COALESCE(report_html, ''),
		       COALESCE(ai_verdict, 'pass'), COALESCE(ai_score, 100), COALESCE(requirement_match, '1.0'), COALESCE(ai_summary, ''),
		       COALESCE(ai_strengths, '[]'), COALESCE(ai_issues, '[]'), COALESCE(ai_risks, '[]'),
		       COALESCE(unauthorized_edits, '[]'), COALESCE(resolved_issues, '[]'), COALESCE(resolved_risks, '[]'),
		       COALESCE(risk_score, 0), COALESCE(risk_routing, 'auto_approve'),
		       COALESCE(ai_model, ''), COALESCE(tokens_used, 0), created_at
		FROM code_reviews
		WHERE sandbox_repo_id = $1 AND pr_number = $2
		ORDER BY created_at DESC
		LIMIT 1;
	`
	var r models.CodeReview
	err := db.Pool.QueryRow(ctx, query, sandboxRepoID, prNumber).Scan(
		&r.ID, &r.SandboxRepoID, &r.DeveloperForkID, &r.PRNumber, &r.CommitSHA, &r.BaselineSnapshotID,
		&r.Results, &r.Comparison, &r.Verdict, &r.ReportHTML,
		&r.AIVerdict, &r.AIScore, &r.RequirementMatch, &r.AISummary,
		&r.AIStrengths, &r.AIIssues, &r.AIRisks,
		&r.UnauthorizedEdits, &r.ResolvedIssues, &r.ResolvedRisks,
		&r.RiskScore, &r.RiskRouting,
		&r.AIModel, &r.TokensUsed, &r.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query code_reviews: %w", err)
	}
	return &r, nil
}

// UpdateDeveloperForkPRURL updates the PR URL associated with a developer fork
func (db *DB) UpdateDeveloperForkPRURL(ctx context.Context, forkID, prURL string) error {
	query := `UPDATE developer_forks SET pr_url = $1 WHERE id = $2;`
	_, err := db.Pool.Exec(ctx, query, prURL, forkID)
	return err
}
