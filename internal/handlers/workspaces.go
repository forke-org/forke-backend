// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/forke/forke-backend/internal/database"
	"github.com/forke/forke-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

type WorkspaceHandler struct {
	db *database.DB
}

func NewWorkspaceHandler(db *database.DB) *WorkspaceHandler {
	return &WorkspaceHandler{db: db}
}

type TaskItem struct {
	ID          string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Title       string    `json:"title" example:"Fix TypeScript types"`
	Description string    `json:"description" example:"Refactor all any types to strict types"`
	Budget      int       `json:"budget" example:"5000"`
	Currency    string    `json:"currency" example:"INR"`
	Status      string    `json:"status" example:"open"`
	ClientID    string    `json:"client_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	ClaimantID  *string   `json:"claimant_id" example:"null"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateTaskRequest struct {
	Title       string `json:"title" example:"Build landing page component"`
	Description string `json:"description" example:"Implement hero section with Tailwind CSS"`
	Budget      int    `json:"budget" example:"8000"`
	Currency    string `json:"currency" example:"INR"`
}

// ListTasks godoc
// @Summary List Tasks & Workspaces
// @Description Retrieve latest available tasks
// @Tags Workspaces
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /workspaces [get]
func (h *WorkspaceHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.db.Pool.Query(ctx, `
		SELECT id, title, description, budget, currency, status, client_id, claimant_id, created_at
		FROM tasks
		ORDER BY created_at DESC
		LIMIT 50
	`)
	if err != nil {
		http.Error(w, `{"error":"server_error","message":"failed to fetch tasks"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tasks := make([]TaskItem, 0)
	for rows.Next() {
		var t TaskItem
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Budget, &t.Currency, &t.Status, &t.ClientID, &t.ClaimantID, &t.CreatedAt); err == nil {
			tasks = append(tasks, t)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
}

// GetTask godoc
// @Summary Get Task by ID
// @Description Get details of a single task
// @Tags Workspaces
// @Produce json
// @Param id path string true "Task UUID"
// @Success 200 {object} TaskItem
// @Failure 404 {object} map[string]string
// @Router /workspaces/{id} [get]
func (h *WorkspaceHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	ctx := r.Context()

	var t TaskItem
	err := h.db.Pool.QueryRow(ctx, `
		SELECT id, title, description, budget, currency, status, client_id, claimant_id, created_at
		FROM tasks
		WHERE id = $1
	`, taskID).Scan(&t.ID, &t.Title, &t.Description, &t.Budget, &t.Currency, &t.Status, &t.ClientID, &t.ClaimantID, &t.CreatedAt)

	if err != nil {
		http.Error(w, `{"error":"not_found","message":"task not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(t)
}

// CreateTask godoc
// @Summary Create Task
// @Description Create a new bounty task
// @Tags Workspaces
// @Accept json
// @Produce json
// @Param request body CreateTaskRequest true "Task payload"
// @Success 201 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /workspaces [post]
func (h *WorkspaceHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(middleware.UserContextKey).(*middleware.JWTClaims)
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	if req.Currency == "" {
		req.Currency = "INR"
	}

	ctx := r.Context()
	var taskID string
	err := h.db.Pool.QueryRow(ctx, `
		INSERT INTO tasks (title, description, budget, currency, client_id, status)
		VALUES ($1, $2, $3, $4, $5, 'open')
		RETURNING id
	`, req.Title, req.Description, req.Budget, req.Currency, claims.UserID).Scan(&taskID)

	if err != nil {
		http.Error(w, `{"error":"server_error","message":"failed to create task"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": taskID, "status": "created"})
}
