<p align="center">
  <img src="./public/forke-assets/email-banners/main-banner.png" width="100%" alt="Forke Banner" />
</p>

# ⚙️ Forke Go Backend Engine

<p align="center">
  <i>High-performance, low-latency Go microservices API powering Forke's authentication, task orchestration, webhooks, and developer sandboxes.</i>
</p>

<p align="center">
  <a href="https://www.forke.space/?source=github"><strong>Official Website</strong></a> ·
  <a href="https://github.com/forke-org/.github"><strong>Org Profile</strong></a> ·
  <a href="https://github.com/forke-org/forke-marketing"><strong>Marketing Repo</strong></a> ·
  <a href="https://github.com/forke-org/forke-dashboard"><strong>Dashboard Repo</strong></a> ·
  <a href="https://github.com/forke-org/forke-admin"><strong>Admin Repo</strong></a>
</p>

---

## 📖 Overview

`forke-backend` is the core backend engine for **Forke**, written in Go. Built with high concurrency and raw speed in mind, it handles task queues, JWT session verification across subdomains, escrow settlement hooks, automated test runs, and third-party webhooks.

### ✨ Key Features
* ⚡ **High Concurrency Go Architecture:** Powered by `go-chi/chi/v5` and `pgx/v5` connection pooling for lightning-fast database interactions.
* 🔐 **Cross-Domain JWT Auth:** Validates and decodes sessions shared across platform services.
* 📚 **Interactive Swagger API Docs:** Built-in OpenAPI / Swagger specification and interactive explorer available for local development.
* 🐳 **Containerized & Production Ready:** Multi-stage minimal Alpine Docker container.
* 🛡️ **CORS & Rate Limiting:** Configurable multi-origin CORS middleware and security headers.

---

## 🛠️ Tech Stack

* **Language:** [Go 1.24+](https://go.dev/)
* **HTTP Router:** [Chi v5](https://github.com/go-chi/chi) (`github.com/go-chi/chi/v5`)
* **Database Driver:** [pgx v5](https://github.com/jackc/pgx) (`github.com/jackc/pgx/v5`)
* **Authentication:** [golang-jwt](https://github.com/golang-jwt/jwt) (`github.com/golang-jwt/jwt/v5`)
* **API Documentation:** [Swagger / Swaggo](https://github.com/swaggo/http-swagger) (`http-swagger/v2`)
* **Environment Configuration:** [godotenv](https://github.com/joho/godotenv)

---

## 🚀 Getting Started Locally

### Prerequisites
* **Go:** `v1.24.x` or higher installed
* **PostgreSQL:** Running PostgreSQL database
* **Docker** *(Optional)*: For containerized testing

### 1. Clone the repository
```bash
git clone https://github.com/forke-org/forke-backend.git
cd forke-backend
```

### 2. Download Go dependencies
```bash
go mod download
```

### 3. Configure environment variables
Create a `.env` file from the provided example:
```bash
cp .env.example .env
```

Ensure your `.env` contains:
```env
# Server Port and Environment
PORT=8080
APP_ENV=development

# Database Connection (pgx connection string)
DATABASE_URL=postgres://forke:forke_secret@localhost:5433/forke_dev?sslmode=disable

# JWT Secret for cross-domain auth
JWT_SECRET=forke-jwt-super-secret-key-replace-in-production

# Cookie Domain (localhost for development)
COOKIE_DOMAIN=localhost

# Allowed CORS Origins
CORS_ORIGINS=http://localhost:3000,http://localhost:3001,http://localhost:3002
```

### 4. Run the Go Server

#### Option A: Direct Go Run
```bash
go run ./cmd/api
```

#### Option B: Build and Execute Binary
```bash
go build -o bin/server ./cmd/api
./bin/server
```

#### Option C: Run with Docker
```bash
docker build -t forke-backend .
docker run -p 8080:8080 --env-file .env forke-backend
```

---

## 📖 API Documentation & Swagger

Once the server is running locally, access the interactive Swagger documentation at:
👉 **`http://localhost:8080/swagger/index.html`**

To regenerate Swagger documentation after modifying endpoint annotations:
```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/api/main.go -o docs
```

---

## 📂 Project Structure

```
forke-backend/
├── cmd/
│   └── api/          # Application entrypoint (main.go, router setup, server boot)
├── docs/             # Swagger generated documentation (docs.go, swagger.json, swagger.yaml)
├── internal/         # Private application packages (handlers, middleware, database, models)
├── public/           # Static branding assets and images
├── Dockerfile        # Production multi-stage Docker build
├── go.mod            # Go dependencies and version declaration
├── go.sum            # Checksums for Go modules
└── ...
```

---

## 🍊 Meet Forky!

<p align="center">
  <img src="./public/forke-assets/forky-reactions/locked_in_forky.png" width="160" alt="Locked In Forky" /> &nbsp;&nbsp;&nbsp;&nbsp;
  <img src="./public/forke-assets/forky-reactions/grind_mode_forky.png" width="160" alt="Grind Mode Forky" /> &nbsp;&nbsp;&nbsp;&nbsp;
  <img src="./public/forke-assets/forky-reactions/loot_goblin_forky.png" width="160" alt="Loot Goblin Forky" /> &nbsp;&nbsp;&nbsp;&nbsp;
  <img src="./public/forke-assets/forky-reactions/confused_forky.png" width="160" alt="Confused Forky" />
</p>

---

## 📄 License

This repository is **source-available, not open-source**. The code is public for
transparency and reference, but **all rights are reserved** — you may read and fork
it on GitHub, but you may **not** use, deploy, copy, or commercialize it without
prior written permission. See [LICENSE](./LICENSE) for the full terms.
