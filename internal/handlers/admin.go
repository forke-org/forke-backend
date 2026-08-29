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
)

type AdminHandler struct {
	db *database.DB
}

func NewAdminHandler(db *database.DB) *AdminHandler {
	return &AdminHandler{db: db}
}

type AdminStatsResponse struct {
	TotalUsers     int `json:"total_users" example:"1240"`
	TotalOwners    int `json:"total_owners" example:"85"`
	TotalTasks     int `json:"total_tasks" example:"320"`
	PendingReports int `json:"pending_reports" example:"4"`
}

type AdminUserSummary struct {
	ID         string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name       string    `json:"name" example:"Ayushman"`
	Username   string    `json:"username" example:"ayushman"`
	Email      string    `json:"email" example:"user@example.com"`
	Role       string    `json:"role" example:"developer"`
	Level      int       `json:"level" example:"4"`
	XP         int       `json:"xp" example:"1250"`
	IsBanned   bool      `json:"is_banned" example:"false"`
	IsApproved bool      `json:"is_approved" example:"true"`
	CreatedAt  time.Time `json:"created_at"`
}

// GetStats godoc
// @Summary Admin Platform Statistics
// @Description Get platform total users, tasks, and enquiries count
// @Tags Admin
// @Produce json
// @Success 200 {object} AdminStatsResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /admin/stats [get]
func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var totalUsers, totalOwners, totalTasks, pendingEnquiries int
	_ = h.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&totalUsers)
	_ = h.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM owners").Scan(&totalOwners)
	_ = h.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM tasks").Scan(&totalTasks)
	_ = h.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM support_enquiries WHERE status = 'pending'").Scan(&pendingEnquiries)

	resp := AdminStatsResponse{
		TotalUsers:     totalUsers,
		TotalOwners:    totalOwners,
		TotalTasks:     totalTasks,
		PendingReports: pendingEnquiries,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ListUsers godoc
// @Summary Admin Users List
// @Description List latest 100 users for moderation
// @Tags Admin
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /admin/users [get]
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.db.Pool.Query(ctx, `
		SELECT id, name, COALESCE(username, ''), email, role, level, xp, is_banned, is_approved, created_at
		FROM users
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		http.Error(w, `{"error":"server_error","message":"failed to query users"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	userList := make([]AdminUserSummary, 0)
	for rows.Next() {
		var u AdminUserSummary
		if err := rows.Scan(&u.ID, &u.Name, &u.Username, &u.Email, &u.Role, &u.Level, &u.XP, &u.IsBanned, &u.IsApproved, &u.CreatedAt); err == nil {
			userList = append(userList, u)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"users": userList,
		"count": len(userList),
	})
}
