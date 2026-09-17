// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/forke/forke-backend/internal/database"
	"github.com/forke/forke-backend/internal/github"
	"github.com/forke/forke-backend/internal/models"
	"github.com/forke/forke-backend/internal/queue"
	"github.com/forke/forke-backend/internal/sandbox"
	"github.com/go-chi/chi/v5"
)

// ReviewHandler handles incoming review triggers, status polling, and baseline creation
type ReviewHandler struct {
	db             *database.DB
	queue          *queue.ReviewQueue
	sandboxManager *sandbox.SandboxManager
	githubClient   *github.Client
}

// NewReviewHandler initializes the HTTP handler
func NewReviewHandler(db *database.DB, q *queue.ReviewQueue, sm *sandbox.SandboxManager) *ReviewHandler {
	return &ReviewHandler{
		db:             db,
		queue:          q,
		sandboxManager: sm,
		githubClient:   github.NewClient(),
	}
}

// TriggerRequest defines the incoming payload for initiating a PR review
type TriggerRequest struct {
	Username    string      `json:"username"`
	SandboxRepo string      `json:"sandboxRepo"`
	PRNumber    interface{} `json:"prNumber"` // can be int or string
}

// TriggerReview enqueues a new automated PR review job
// @Summary Trigger PR Review
// @Description Enqueues an isolated deterministic verification and AI review for a PR ref
// @Tags Reviews
// @Accept json
// @Produce json
// @Success 202 {object} map[string]interface{}
// @Router /api/v1/reviews/trigger [post]
func (h *ReviewHandler) TriggerReview(w http.ResponseWriter, r *http.Request) {
	var req TriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request JSON payload"}`, http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.SandboxRepo == "" || req.PRNumber == nil {
		http.Error(w, `{"error":"Missing required fields: username, sandboxRepo, prNumber"}`, http.StatusBadRequest)
		return
	}

	// Parse PR Number
	var prNumber int
	switch v := req.PRNumber.(type) {
	case float64:
		prNumber = int(v)
	case string:
		var err error
		prNumber, err = strconv.Atoi(v)
		if err != nil {
			http.Error(w, `{"error":"Invalid prNumber parameter"}`, http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, `{"error":"prNumber must be an integer or numeric string"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// 1. Verify Sandbox Repository
	sandboxRepo, err := h.db.GetSandboxRepoBySandbox(ctx, req.SandboxRepo)
	if err != nil || sandboxRepo == nil {
		http.Error(w, fmt.Sprintf(`{"error":"Sandbox repository '%s' not found"}`, req.SandboxRepo), http.StatusNotFound)
		return
	}

	// 2. Fetch Owner OAuth Token
	ownerToken, err := h.db.GetSandboxOwnerToken(ctx, sandboxRepo.OwnerID)
	if err != nil || ownerToken == "" {
		http.Error(w, `{"error":"Sandbox owner token not available"}`, http.StatusUnauthorized)
		return
	}

	// 3. Fetch PR Head SHA from GitHub
	prInfo, err := h.githubClient.FetchPRInfo(ctx, req.SandboxRepo, prNumber, ownerToken)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"GitHub API error: %v"}`, err), http.StatusBadGateway)
		return
	}

	// Update PR URL in developer_forks if changed
	if fork, _ := h.db.GetDeveloperFork(ctx, req.Username, req.SandboxRepo); fork != nil {
		if prInfo.HTMLURL != "" && (fork.PRURL == nil || *fork.PRURL != prInfo.HTMLURL) {
			_ = h.db.UpdateDeveloperForkPRURL(ctx, fork.ID, prInfo.HTMLURL)
		}
	}

	// 4. Enqueue into Review Queue (<40ms response)
	job, err := h.queue.Enqueue(req.SandboxRepo, prNumber, prInfo.HeadSHA, req.Username)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to enqueue job: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Set GitHub Commit Status to Pending immediately
	_ = h.githubClient.UpdateCommitStatus(
		ctx, req.SandboxRepo, prInfo.HeadSHA, "pending",
		"Forke automated review queued...", "", ownerToken,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"triggered": true,
		"jobId":     job.ID,
		"status":    job.Status,
		"headSha":   prInfo.HeadSHA,
		"message":   "Forke automated verification queued in background runner.",
	})
}

// GetJobStatus returns real-time progress and logs for an in-flight or completed review job
// @Summary Get Review Job Status
// @Tags Reviews
// @Produce json
// @Param jobId path string true "Job ID"
// @Router /api/v1/reviews/{jobId}/status [get]
func (h *ReviewHandler) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	job := h.queue.GetJob(jobID)
	if job == nil {
		http.Error(w, `{"error":"Job not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(job)
}

// GetPRReview returns the latest saved unified review for a sandbox repository and PR
// @Summary Get PR Review Report
// @Tags Reviews
// @Produce json
// @Router /api/v1/reviews/pr [get]
func (h *ReviewHandler) GetPRReview(w http.ResponseWriter, r *http.Request) {
	sandboxRepo := r.URL.Query().Get("sandboxRepo")
	prNumberStr := r.URL.Query().Get("prNumber")
	username := r.URL.Query().Get("username")

	if sandboxRepo == "" || prNumberStr == "" {
		http.Error(w, `{"error":"Missing query parameters: sandboxRepo, prNumber"}`, http.StatusBadRequest)
		return
	}

	prNumber, err := strconv.Atoi(prNumberStr)
	if err != nil {
		http.Error(w, `{"error":"Invalid prNumber"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	sandboxRecord, err := h.db.GetSandboxRepoBySandbox(ctx, sandboxRepo)
	if err != nil || sandboxRecord == nil {
		http.Error(w, `{"error":"Sandbox not found"}`, http.StatusNotFound)
		return
	}

	reviewRecord, err := h.db.GetLatestCodeReviewForPR(ctx, sandboxRecord.ID, prNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Database query error: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if reviewRecord == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"review": nil})
		return
	}

	// Format response exactly as expected by forke-dashboard UI
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"review": map[string]interface{}{
			"id":                reviewRecord.ID,
			"prNumber":          reviewRecord.PRNumber,
			"commitSha":         reviewRecord.CommitSHA,
			"verdict":           reviewRecord.Verdict,
			"score":             reviewRecord.AIScore,
			"requirementMatch":  reviewRecord.RequirementMatch,
			"summary":           reviewRecord.AISummary,
			"strengths":         parseJSONString(reviewRecord.AIStrengths, []string{}),
			"issues":            parseJSONString(reviewRecord.AIIssues, []interface{}{}),
			"risks":             parseJSONString(reviewRecord.AIRisks, []interface{}{}),
			"unauthorizedEdits": parseJSONString(reviewRecord.UnauthorizedEdits, []string{}),
			"resolvedIssues":    parseJSONString(reviewRecord.ResolvedIssues, []interface{}{}),
			"resolvedRisks":     parseJSONString(reviewRecord.ResolvedRisks, []interface{}{}),
			"results":           parseJSONString(reviewRecord.Results, map[string]interface{}{}),
			"comparison":        parseJSONString(reviewRecord.Comparison, map[string]interface{}{}),
			"reportHtml":        reviewRecord.ReportHTML,
			"riskScore":         reviewRecord.RiskScore,
			"riskRouting":       reviewRecord.RiskRouting,
			"createdAt":         reviewRecord.CreatedAt,
			"username":          username,
		},
	})
}

// CancelReview cancels an active review job
// @Summary Cancel Review Job
// @Tags Reviews
// @Router /api/v1/reviews/cancel [post]
func (h *ReviewHandler) CancelReview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JobID string `json:"jobId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.JobID == "" {
		http.Error(w, `{"error":"Invalid request, jobId required"}`, http.StatusBadRequest)
		return
	}

	cancelled := h.queue.CancelJob(req.JobID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"cancelled": cancelled,
		"jobId":     req.JobID,
	})
}

// CreateBaseline triggers a baseline health check snapshot on the owner's default branch
// @Summary Create Baseline Snapshot
// @Tags Reviews
// @Router /api/v1/reviews/baseline [post]
func (h *ReviewHandler) CreateBaseline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SandboxRepo string `json:"sandboxRepo"`
		Branch      string `json:"branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SandboxRepo == "" {
		http.Error(w, `{"error":"Missing sandboxRepo"}`, http.StatusBadRequest)
		return
	}
	if req.Branch == "" {
		req.Branch = "main"
	}

	ctx := r.Context()
	sandboxRecord, err := h.db.GetSandboxRepoBySandbox(ctx, req.SandboxRepo)
	if err != nil || sandboxRecord == nil {
		http.Error(w, `{"error":"Sandbox not found"}`, http.StatusNotFound)
		return
	}

	ownerToken, err := h.db.GetSandboxOwnerToken(ctx, sandboxRecord.OwnerID)
	if err != nil || ownerToken == "" {
		http.Error(w, `{"error":"Owner token not found"}`, http.StatusUnauthorized)
		return
	}

	// Run baseline in background
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		workspaceDir, err := h.sandboxManager.CreateWorkspace("baseline-" + strconv.FormatInt(time.Now().Unix(), 10))
		if err != nil {
			return
		}
		defer h.sandboxManager.CleanupWorkspace(workspaceDir)

		onLog := func(tag, msg string) {}
		if err := h.sandboxManager.CloneBaseBranch(bgCtx, req.SandboxRepo, req.Branch, ownerToken, workspaceDir, onLog); err != nil {
			return
		}

		var runner sandbox.Runner
		if h.sandboxManager.CheckDocker() {
			runner = &sandbox.DockerRunner{Image: h.sandboxManager.DockerImage}
		} else {
			runner = &sandbox.LocalRunner{}
		}

		res, err := runner.Run(bgCtx, workspaceDir, onLog)
		if err != nil {
			return
		}

		resultsJSON, _ := json.Marshal(res.Results)
		techStackJSON, _ := json.Marshal(res.TechStack)

		snapshot := &models.BaselineSnapshot{
			SandboxRepoID: sandboxRecord.ID,
			Branch:        req.Branch,
			CommitSHA:     "head-" + strconv.FormatInt(time.Now().Unix(), 10),
			TechStack:     string(techStackJSON),
			Results:       string(resultsJSON),
			AISummary:     "Automated baseline snapshot generated by Forke Go Engine.",
		}
		_ = h.db.InsertBaselineSnapshot(bgCtx, snapshot)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Baseline snapshot execution queued.",
		"branch":  req.Branch,
	})
}

func parseJSONString(raw string, fallback interface{}) interface{} {
	if raw == "" {
		return fallback
	}
	var res interface{}
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return fallback
	}
	return res
}
