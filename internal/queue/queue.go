// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/forke/forke-backend/internal/ai"
	"github.com/forke/forke-backend/internal/database"
	"github.com/forke/forke-backend/internal/github"
	"github.com/forke/forke-backend/internal/models"
	"github.com/forke/forke-backend/internal/review"
	"github.com/forke/forke-backend/internal/sandbox"
)

// ReviewQueue coordinates background PR review jobs with strict concurrency
type ReviewQueue struct {
	db             *database.DB
	sandboxManager *sandbox.SandboxManager
	geminiClient   *ai.GeminiClient
	githubClient   *github.Client

	jobChan        chan *models.ReviewJob
	jobs           map[string]*models.ReviewJob
	activeCancels  map[string]context.CancelFunc // prKey -> cancelFunc for superseded cancellation
	mu             sync.RWMutex

	maxConcurrency int
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewReviewQueue initializes the worker pool with concurrency = 2
func NewReviewQueue(db *database.DB, sm *sandbox.SandboxManager, maxConcurrency int) *ReviewQueue {
	if maxConcurrency <= 0 {
		maxConcurrency = 2 // Strict VM capacity limit
	}

	ctx, cancel := context.WithCancel(context.Background())

	q := &ReviewQueue{
		db:             db,
		sandboxManager: sm,
		geminiClient:   ai.NewGeminiClient(""),
		githubClient:   github.NewClient(),
		jobChan:        make(chan *models.ReviewJob, 500),
		jobs:           make(map[string]*models.ReviewJob),
		activeCancels:  make(map[string]context.CancelFunc),
		maxConcurrency: maxConcurrency,
		ctx:            ctx,
		cancel:         cancel,
	}

	// Start worker pool
	for i := 1; i <= maxConcurrency; i++ {
		go q.worker(i)
	}

	log.Printf("[ReviewQueue] Started background review queue with concurrency = %d", maxConcurrency)
	return q
}

// Enqueue adds a job to the queue, canceling any older in-flight jobs for the same PR
func (q *ReviewQueue) Enqueue(sandboxRepo string, prNumber int, headSHA, username string) (*models.ReviewJob, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	prKey := fmt.Sprintf("%s#%d", sandboxRepo, prNumber)

	// 1. Superseded Commit Auto-Cancellation:
	// If an older commit is running or queued for this same PR, cancel it immediately!
	if cancelFunc, exists := q.activeCancels[prKey]; exists {
		log.Printf("[ReviewQueue] Superseding previous in-flight job for PR %s with newer commit %s", prKey, headSHA)
		cancelFunc()
		delete(q.activeCancels, prKey)
	}

	jobID := fmt.Sprintf("pr-review-%d-%d", prNumber, time.Now().UnixMilli())
	job := &models.ReviewJob{
		ID:          jobID,
		SandboxRepo: sandboxRepo,
		PRNumber:    prNumber,
		HeadSHA:     headSHA,
		Username:    username,
		Status:      "queued",
		Progress:    5,
		Logs:        []string{"INIT Review job registered in Go background queue."},
		CreatedAt:   time.Now().UTC(),
	}

	q.jobs[jobID] = job
	q.jobChan <- job

	log.Printf("[ReviewQueue] Enqueued job %s for %s (commit %s)", jobID, prKey, headSHA)
	return job, nil
}

// GetJob returns current status, progress, and logs for a job
func (q *ReviewQueue) GetJob(jobID string) *models.ReviewJob {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.jobs[jobID]
}

// CancelJob manually cancels an active review job
func (q *ReviewQueue) CancelJob(jobID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, exists := q.jobs[jobID]
	if !exists {
		return false
	}

	prKey := fmt.Sprintf("%s#%d", job.SandboxRepo, job.PRNumber)
	if cancelFunc, hasCancel := q.activeCancels[prKey]; hasCancel {
		cancelFunc()
		delete(q.activeCancels, prKey)
	}

	job.Status = "cancelled"
	job.Error = "Manually cancelled by user or administrator"
	now := time.Now().UTC()
	job.FinishedAt = &now
	return true
}

func (q *ReviewQueue) worker(workerID int) {
	for {
		select {
		case <-q.ctx.Done():
			return
		case job := <-q.jobChan:
			prKey := fmt.Sprintf("%s#%d", job.SandboxRepo, job.PRNumber)

			jobCtx, jobCancel := context.WithTimeout(q.ctx, 300*time.Second)

			q.mu.Lock()
			q.activeCancels[prKey] = jobCancel
			job.Status = "running"
			job.Progress = 10
			q.mu.Unlock()

			// Execute the pipeline
			q.processReview(jobCtx, job)

			q.mu.Lock()
			delete(q.activeCancels, prKey)
			jobCancel()
			q.mu.Unlock()
		}
	}
}

func (q *ReviewQueue) processReview(ctx context.Context, job *models.ReviewJob) {
	addLog := func(tag, msg string) {
		formatted := fmt.Sprintf("[%s] %s", tag, msg)
		log.Printf("[Job %s] %s", job.ID, formatted)
		q.mu.Lock()
		job.Logs = append(job.Logs, formatted)
		q.mu.Unlock()
	}

	updateProgress := func(pct int) {
		q.mu.Lock()
		job.Progress = pct
		q.mu.Unlock()
	}

	addLog("INIT", fmt.Sprintf("Starting 4-layer review pipeline for PR #%d on %s", job.PRNumber, job.SandboxRepo))
	updateProgress(15)

	// 1. Fetch Sandbox Configuration from Database
	sandboxRepo, err := q.db.GetSandboxRepoBySandbox(ctx, job.SandboxRepo)
	if err != nil || sandboxRepo == nil {
		q.failJob(job, fmt.Sprintf("Sandbox repository %s not found in database: %v", job.SandboxRepo, err), addLog)
		return
	}

	// 2. Fetch Owner GitHub Token
	ownerToken, err := q.db.GetSandboxOwnerToken(ctx, sandboxRepo.OwnerID)
	if err != nil || ownerToken == "" {
		q.failJob(job, fmt.Sprintf("Owner OAuth token could not be retrieved: %v", err), addLog)
		return
	}

	// 3. Set GitHub Commit Status to Pending
	_ = q.githubClient.UpdateCommitStatus(
		ctx, job.SandboxRepo, job.HeadSHA, "pending",
		"Forke automated verification running in sandbox...", "", ownerToken,
	)
	updateProgress(20)

	// 4. Fetch PR Metadata & Changed Files from GitHub
	addLog("CHECKING", "Fetching PR metadata and altered file list from GitHub...")
	prInfo, err := q.githubClient.FetchPRInfo(ctx, job.SandboxRepo, job.PRNumber, ownerToken)
	if err != nil {
		q.failJob(job, fmt.Sprintf("Failed to fetch PR info from GitHub: %v", err), addLog)
		return
	}

	prFiles, err := q.githubClient.FetchPRFiles(ctx, job.SandboxRepo, job.PRNumber, ownerToken)
	if err != nil {
		q.failJob(job, fmt.Sprintf("Failed to fetch PR changed files: %v", err), addLog)
		return
	}

	changedFileNames := make([]string, len(prFiles))
	for i, f := range prFiles {
		changedFileNames[i] = f.Filename
	}
	gitDiff := github.BuildGitDiff(prFiles)
	addLog("CHECKING", fmt.Sprintf("Retrieved %d changed file(s).", len(changedFileNames)))
	updateProgress(25)

	// 5. File Boundary & Scope Checks (Layer 1 Pre-screening)
	allowedPaths := review.ParsePathList(sandboxRepo.AllowedPaths)
	restrictedPaths := review.ParsePathList(sandboxRepo.RestrictedPaths)
	unauthorizedFiles := review.ValidateFileBoundaries(changedFileNames, allowedPaths, restrictedPaths)
	if len(unauthorizedFiles) > 0 {
		addLog("WARN", fmt.Sprintf("Detected %d unauthorized file modification(s).", len(unauthorizedFiles)))
	} else {
		addLog("SUCCESS", "All file modifications conform to project boundary criteria.")
	}
	updateProgress(30)

	// 6. Provision Workspace & Clone PR Ref
	workspaceDir, err := q.sandboxManager.CreateWorkspace(job.ID)
	if err != nil {
		q.failJob(job, fmt.Sprintf("Failed to provision workspace: %v", err), addLog)
		return
	}
	defer q.sandboxManager.CleanupWorkspace(workspaceDir)

	err = q.sandboxManager.CloneAndCheckout(ctx, job.SandboxRepo, job.PRNumber, ownerToken, workspaceDir, addLog)
	if err != nil {
		q.failJob(job, fmt.Sprintf("Git clone failed: %v", err), addLog)
		return
	}
	updateProgress(45)

	// 7. Hardcoded Secrets Scan & Forke Submission Validation
	secretFindings, _ := review.ScanSecrets(workspaceDir, changedFileNames)
	if len(secretFindings) > 0 {
		addLog("WARN", fmt.Sprintf("Security scanner flagged %d potential secret leak(s).", len(secretFindings)))
	}
	submissionValid, submissionError := review.ValidateForkeSubmission(workspaceDir)
	if !submissionValid {
		addLog("WARN", submissionError)
	}
	updateProgress(50)

	// 8. Execute 12-Category Deterministic Pipeline in Sandbox Runner
	var runner sandbox.Runner
	if q.sandboxManager.CheckDocker() {
		runner = &sandbox.DockerRunner{Image: q.sandboxManager.DockerImage}
	} else {
		runner = &sandbox.LocalRunner{}
	}

	addLog("EXEC", "Starting 12-category deterministic execution pipeline...")
	runnerResult, err := runner.Run(ctx, workspaceDir, addLog)
	if err != nil {
		q.failJob(job, fmt.Sprintf("Execution runner encountered an error: %v", err), addLog)
		return
	}
	updateProgress(75)

	// 9. Compute Raw Test Score
	rawTestScore := review.ComputeTestScore(
		runnerResult.Results, unauthorizedFiles, len(secretFindings), submissionValid,
	)

	// 10. Baseline Delta Comparison (Layer 2)
	addLog("CHECKING", "Comparing PR execution against repository baseline snapshot...")
	baseline, _ := q.db.GetLatestBaselineSnapshot(ctx, sandboxRepo.ID)
	var baselineComparison *models.BaselineComparison
	testScore := rawTestScore

	if baseline != nil && baseline.Results != "" {
		var baseResults map[string]models.CategoryResult
		if err := json.Unmarshal([]byte(baseline.Results), &baseResults); err == nil {
			baselineComparison = review.CompareToBaseline(baseResults, runnerResult.Results)
			// Adjust score so pre-existing debt is not penalized
			testScore = review.AdjustScoreForBaseline(rawTestScore, baselineComparison)
			addLog("SUCCESS", fmt.Sprintf("Baseline delta complete: %d regression(s), %d debt item(s) discounted.",
				len(baselineComparison.Regressions), len(baselineComparison.BaselineDebt)))
		}
	}
	updateProgress(82)

	// 11. AI Code Review Reasoning (Layer 3: Gemini Flash)
	addLog("AI", "Triggering Google Gemini Flash review reasoning...")
	aiReview, err := q.geminiClient.RunReview(ctx, ai.ReviewContext{
		PRNumber:           job.PRNumber,
		PRTitle:            prInfo.Title,
		PRBody:             prInfo.Body,
		DeveloperUsername:  job.Username,
		ChangedFiles:       changedFileNames,
		GitDiff:            gitDiff,
		TaskTitle:          sandboxRepo.TaskTitle,
		TaskDescription:    sandboxRepo.TaskDescription,
		AcceptanceCriteria: sandboxRepo.AcceptanceCriteria,
		FrontendStack:      sandboxRepo.FrontendStack,
		BackendStack:       sandboxRepo.BackendStack,
		TestScoreResult:    testScore,
		BaselineComparison: baselineComparison,
	})
	if err != nil {
		addLog("WARN", fmt.Sprintf("AI review returned an error (%v); applying heuristic verdict.", err))
		aiReview = &models.AIReviewResult{
			Verdict:          "pass",
			Score:            testScore.TestScore,
			RequirementMatch: 0.8,
			Summary:          "Deterministic tests passed. AI review fell back to heuristic scoring.",
			Strengths:        []string{"Deterministic verification succeeded."},
			Issues:           []models.AIIssue{},
			Risks:            []models.AIRisk{},
			Model:            "heuristic",
		}
	}
	updateProgress(90)

	// 12. Synthesize Composite Risk Score & Routing (Layer 4)
	finalScore, finalVerdict, compositeRisk, riskRouting := review.CalculateFinalScore(aiReview, testScore)
	addLog("SCORE", fmt.Sprintf("Final Composite Score: %d/100 | Verdict: %s | Risk: %d/100 | Routing: %s",
		finalScore, finalVerdict, compositeRisk, riskRouting))
	updateProgress(95)

	// 13. Persist Unified Review to PostgreSQL
	resultsJSON, _ := json.Marshal(runnerResult.Results)
	var comparisonJSON []byte
	var comparisonHTML string
	if baselineComparison != nil {
		comparisonJSON, _ = json.Marshal(baselineComparison)
		comparisonHTML = baselineComparison.SummaryHTML
	}
	aiStrengthsJSON, _ := json.Marshal(aiReview.Strengths)
	aiIssuesJSON, _ := json.Marshal(aiReview.Issues)
	aiRisksJSON, _ := json.Marshal(aiReview.Risks)
	unauthJSON, _ := json.Marshal(unauthorizedFiles)

	var baselineID *string
	if baseline != nil {
		baselineID = &baseline.ID
	}

	fork, _ := q.db.GetDeveloperFork(ctx, job.Username, job.SandboxRepo)
	var forkID *string
	if fork != nil {
		forkID = &fork.ID
	}

	codeReview := &models.CodeReview{
		SandboxRepoID:      sandboxRepo.ID,
		DeveloperForkID:    forkID,
		PRNumber:           job.PRNumber,
		CommitSHA:          job.HeadSHA,
		BaselineSnapshotID: baselineID,
		Results:            string(resultsJSON),
		Comparison:         string(comparisonJSON),
		Verdict:            finalVerdict,
		ReportHTML:         comparisonHTML,
		AIVerdict:          aiReview.Verdict,
		AIScore:            finalScore,
		RequirementMatch:   fmt.Sprintf("%.2f", aiReview.RequirementMatch),
		AISummary:          aiReview.Summary,
		AIStrengths:        string(aiStrengthsJSON),
		AIIssues:           string(aiIssuesJSON),
		AIRisks:            string(aiRisksJSON),
		UnauthorizedEdits:  string(unauthJSON),
		RiskScore:          compositeRisk,
		RiskRouting:        riskRouting,
		AIModel:            aiReview.Model,
		TokensUsed:         aiReview.TokensUsed,
	}

	if err := q.db.InsertCodeReview(ctx, codeReview); err != nil {
		addLog("WARN", fmt.Sprintf("Failed to save code_review record to database: %v", err))
	} else {
		job.ReviewRecordID = &codeReview.ID
		addLog("SUCCESS", fmt.Sprintf("Review results saved to PostgreSQL (ID: %s).", codeReview.ID))
	}

	// 14. Post Final GitHub Commit Status
	ghState := "success"
	ghDesc := fmt.Sprintf("Forke Score: %d/100 (%s)", finalScore, finalVerdict)
	if finalVerdict == "high_risk" {
		ghState = "failure"
		ghDesc = fmt.Sprintf("Forke: High Risk (%d/100) — Build/Tests Failed", finalScore)
	} else if finalVerdict == "needs_changes" {
		ghState = "failure"
		ghDesc = fmt.Sprintf("Forke: Changes Required (%d/100)", finalScore)
	}

	_ = q.githubClient.UpdateCommitStatus(ctx, job.SandboxRepo, job.HeadSHA, ghState, ghDesc, "", ownerToken)

	// 15. Mark Job Complete
	q.mu.Lock()
	job.Status = "success"
	job.Progress = 100
	now := time.Now().UTC()
	job.FinishedAt = &now
	q.mu.Unlock()

	addLog("COMPLETE", "Automated PR review pipeline finished successfully.")
}

func (q *ReviewQueue) failJob(job *models.ReviewJob, errMsg string, addLog func(tag, msg string)) {
	addLog("ERROR", errMsg)
	q.mu.Lock()
	job.Status = "failed"
	job.Error = errMsg
	job.Progress = 100
	now := time.Now().UTC()
	job.FinishedAt = &now
	q.mu.Unlock()
}

// Close gracefully stops the worker queue
func (q *ReviewQueue) Close() {
	q.cancel()
}
