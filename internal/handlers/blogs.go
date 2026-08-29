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
	"github.com/go-chi/chi/v5"
)

type BlogHandler struct {
	db *database.DB
}

func NewBlogHandler(db *database.DB) *BlogHandler {
	return &BlogHandler{db: db}
}

type BlogItem struct {
	ID             string     `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Title          string     `json:"title" example:"Introducing Forke Developer Cloud"`
	Slug           string     `json:"slug" example:"introducing-forke"`
	Excerpt        *string    `json:"excerpt" example:"A developer marketplace reimagined"`
	CoverImage     *string    `json:"cover_image" example:"https://cdn.forke.space/covers/hero.png"`
	ContentHTML    *string    `json:"content_html,omitempty"`
	AuthorName     *string    `json:"author_name" example:"Ayushman"`
	ReadingMinutes int        `json:"reading_minutes" example:"3"`
	Views          int        `json:"views" example:"142"`
	PublishedAt    *time.Time `json:"published_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

// ListPublished godoc
// @Summary List Published Blogs
// @Description Retrieve latest published blogs for marketing site
// @Tags Blogs
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /blogs [get]
func (h *BlogHandler) ListPublished(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.db.Pool.Query(ctx, `
		SELECT id, title, slug, excerpt, cover_image, author_name, reading_minutes, views, published_at, created_at
		FROM blogs
		WHERE status = 'published'
		ORDER BY published_at DESC NULLS LAST
		LIMIT 20
	`)
	if err != nil {
		http.Error(w, `{"error":"server_error","message":"failed to fetch blogs"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	blogs := make([]BlogItem, 0)
	for rows.Next() {
		var b BlogItem
		if err := rows.Scan(&b.ID, &b.Title, &b.Slug, &b.Excerpt, &b.CoverImage, &b.AuthorName, &b.ReadingMinutes, &b.Views, &b.PublishedAt, &b.CreatedAt); err == nil {
			blogs = append(blogs, b)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"blogs": blogs,
		"total": len(blogs),
	})
}

// GetBySlug godoc
// @Summary Get Blog by Slug
// @Description Get blog article content by URL slug
// @Tags Blogs
// @Produce json
// @Param slug path string true "Blog slug"
// @Success 200 {object} BlogItem
// @Failure 404 {object} map[string]string
// @Router /blogs/{slug} [get]
func (h *BlogHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	ctx := r.Context()

	var b BlogItem
	err := h.db.Pool.QueryRow(ctx, `
		UPDATE blogs SET views = views + 1 WHERE slug = $1 AND status = 'published'
		RETURNING id, title, slug, excerpt, cover_image, content_html, author_name, reading_minutes, views, published_at, created_at
	`, slug).Scan(&b.ID, &b.Title, &b.Slug, &b.Excerpt, &b.CoverImage, &b.ContentHTML, &b.AuthorName, &b.ReadingMinutes, &b.Views, &b.PublishedAt, &b.CreatedAt)

	if err != nil {
		http.Error(w, `{"error":"not_found","message":"blog post not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(b)
}
