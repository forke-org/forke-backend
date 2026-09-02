// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

// @title Forke Backend API
// @version 1.0
// @description Forke Core Cloud Platform & Developer Sandbox Go API.
// @termsOfService https://forke.space/terms

// @contact.name Forke Support
// @contact.url https://forke.space
// @contact.email support@forke.space

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host api.forke.space
// @BasePath /api/v1
// @schemes https http

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/forke/forke-backend/docs"
	"github.com/forke/forke-backend/internal/config"
	"github.com/forke/forke-backend/internal/database"
	"github.com/forke/forke-backend/internal/handlers"
	"github.com/forke/forke-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to Database
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Printf("[Warning] Database connection failed (%v). Continuing in standalone mode for local development.", err)
	} else {
		defer db.Close()
	}

	// Router Setup
	r := chi.NewRouter()

	// Standard Middlewares
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))
	r.Use(middleware.CORS(cfg.CORSOrigins))

	// Root & Health Endpoints
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		res := map[string]interface{}{
			"service":   "Forke Core API",
			"status":    "healthy",
			"version":   "1.0.0",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		if cfg.EnableSwagger {
			scheme := "https"
			if req.TLS == nil && req.Header.Get("X-Forwarded-Proto") != "https" && (req.Host == "localhost:8080" || req.Host == "127.0.0.1:8080") {
				scheme = "http"
			}
			host := req.Host
			if host == "" {
				host = "api.forke.space"
			}
			res["docs"] = fmt.Sprintf("%s://%s/swagger/index.html", scheme, host)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	})
	r.Get("/health", handlers.HealthCheck)
	r.Get("/api/v1/health", handlers.HealthCheck)
	r.Get("/system/telemetry", handlers.SystemTelemetry)
	r.Get("/api/v1/system/telemetry", handlers.SystemTelemetry)

	// Swagger Docs Endpoint (only mounted if EnableSwagger is true)
	if cfg.EnableSwagger {
		r.Get("/swagger/*", httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"),
		))
	}

	if db != nil {
		authHandler := handlers.NewAuthHandler(db, cfg)
		workspaceHandler := handlers.NewWorkspaceHandler(db)
		blogHandler := handlers.NewBlogHandler(db)
		adminHandler := handlers.NewAdminHandler(db)

		// Public API Routes
		r.Route("/api/v1", func(r chi.Router) {
			// Auth
			r.Route("/auth", func(r chi.Router) {
				r.Post("/login", authHandler.Login)
				r.Post("/register", authHandler.Register)
				r.Post("/logout", authHandler.Logout)

				// Protected Auth Routes
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireAuth(cfg.JWTSecret))
					r.Get("/me", authHandler.Me)
				})
			})

			// Blogs (Public)
			r.Route("/blogs", func(r chi.Router) {
				r.Get("/", blogHandler.ListPublished)
				r.Get("/{slug}", blogHandler.GetBySlug)
			})

			// Workspaces / Tasks (Protected)
			r.Route("/workspaces", func(r chi.Router) {
				r.Get("/", workspaceHandler.ListTasks)
				r.Get("/{id}", workspaceHandler.GetTask)

				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireAuth(cfg.JWTSecret))
					r.Post("/", workspaceHandler.CreateTask)
				})
			})

			// Admin Portal (Protected with Admin Role)
			r.Route("/admin", func(r chi.Router) {
				r.Use(middleware.RequireAuth(cfg.JWTSecret))
				r.Use(middleware.RequireAdminRole)

				r.Get("/stats", adminHandler.GetStats)
				r.Get("/users", adminHandler.ListUsers)
			})
		})
	}

	// Start Server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("🚀 Forke Go Backend started successfully on port %s", cfg.Port)
		log.Printf("📖 Swagger Docs available at http://localhost:%s/swagger/index.html", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to listen: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Forke Backend gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Forke Backend exited cleanly.")
}
