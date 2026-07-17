# Deep Dive: Architecture & Implementation of our Go E-Commerce REST API

Welcome to the architectural blueprint of your new Go E-Commerce REST API. This document breaks down **how** the application is structured, **why** certain design choices were made, and **how** every file communicates from the infrastructure layer down to the database.

---

## 1. Architectural Overview: Clean Architecture

This application uses a variation of **Clean Architecture** (often called Ports and Adapters or Three-Tier Architecture). 

```text
       ┌────────────────────────────────────────────────────────┐
       │                   HTTP Request                         │
       └───────────────────────────┬────────────────────────────┘
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │   Presentation Layer (internal/item/handler.go)        │
       │   - Decodes HTTP JSON into Go Structs                  │
       │   - Dispatches requests to the Service                 │
       └───────────────────────────┬────────────────────────────┘
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │   Business Logic Layer (internal/item/service.go)      │
       │   - Validates data (e.g., price > 0, non-empty names)  │
       │   - Implements transactional logic                     │
       └───────────────────────────┬────────────────────────────┘
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │   Data Access Layer (internal/item/repository.go)     │
       │   - Interface-driven database operations               │
       │   - Communicates with Postgres via sqlc-generated code │
       └────────────────────────────────────────────────────────┘
```

The core principle is the **Dependency Inversion Principle**: Higher-level modules (like the Service) do not depend on lower-level modules (like the specific Postgres driver). Instead, they depend on **Interfaces**. This allows you to completely swap out your database layer in the future without changing a single line of business logic.

---

## 2. Component-by-Component Walkthrough

Let's trace the responsibility of every file in the directory structure.

### ── `cmd/api/main.go` (The Orchestrator)
This is the entry point of your application. It has three critical responsibilities:
1. **Environment Setup & Connection Resilience:** It reads environment variables (like `DB_CONN_STR`) and implements a retry loop to wait for PostgreSQL to boot up safely.
2. **Auto-Migrations:** It imports the `golang-migrate` package as a library to automatically detect and apply database changes on startup before accepting web traffic.
3. **Dependency Injection (Wiring):** It instantiates each component in order:
   ```go
   repo := item.NewRepository(dbPool) // Postgres Repo implements the Repository interface
   service := item.NewService(repo)   // Service accepts any Repository interface
   handler := item.NewHandler(service) // Handler takes the Service
   ```
4. **Routing:** It configures the **Chi** router, applies standard middleware (logging, timeout, panic recovery), and registers the endpoints.

---

### ── `db/` (Database Management & `sqlc`)
Instead of using a bulky Object-Relational Mapper (ORM) like Hibernate or SQLAlchemy which introduces hidden runtime magic, we use **`sqlc`**.

* **`db/migration/`**: Contains raw SQL schema definitions (e.g., creating the `items` table and indexing categories).
* **`db/queries/items.sql`**: Your raw SQL queries. Notice the annotations:
  ```sql
  -- name: CreateItem :one
  ```
  `sqlc` parses this, identifies the parameter positions (`$1`, `$2`), and compiles compile-time safe Go code inside `db/sqlc/`.
* **`db/sqlc/models.go`**: Contains a Go `Item` struct that matches the columns of your Postgres table perfectly.
* **`db/sqlc/items.sql.go`**: Contains auto-generated Go methods like `CreateItem(ctx, arg)` that execute the raw SQL query under the hood using safe parameterized inputs.

---

### ── `internal/item/` (The Core Feature Domain)
All code belonging to the "Item" domain is isolated here.

#### 1. `repository.go` (Data Access)
This file defines how we interact with Postgres. 
* It declares the `Repository` **interface**. Any struct that implements these methods can act as a repository.
* It defines a concrete `postgresRepo` struct that wraps `sqlc`’s compiled query handler:
  ```go
  type postgresRepo struct {
      queries *sql.Queries
  }
  ```
* Notice how it maps native Go strings to `pgtype.Text` to safely support full-text matches without breaking database nullability.

#### 2. `service.go` (Business Logic)
This is where rules live. 
* It ensures items cannot have negative prices (`ErrInvalidPrice`) or empty names (`ErrEmptyName`).
* It exposes business-facing functions like `CreateBulk()`. It coordinates transaction loops and passes formatted domain parameters back down to the Repository.
* **Key Point:** The Service knows *nothing* about HTTP. It doesn't know what a JSON payload is, nor does it write HTTP headers.

#### 3. `handler.go` (Transport Layer)
This is the HTTP adapter.
* It implements the endpoints registered in `main.go`.
* It decodes request JSON into DTOs (Data Transfer Objects) using `json.NewDecoder()`.
* It handles API errors gracefully (e.g., if the service returns `ErrInvalidPrice`, the handler turns it into a `400 Bad Request` JSON response).
* It serializes the final structs back into JSON arrays and writes the correct HTTP headers (e.g., `201 Created` or `204 No Content`).

---

## 3. The "Infra" Setup (Docker & Compose)

To run this seamlessly across any operating system (including Windows, macOS, and Linux production servers in the cloud), we containerized the stack using **Docker**.

### The Multi-Stage `Dockerfile`
Go compiles to a single, standalone binary file. We leverage a **Multi-Stage Build** to keep our final image incredibly lightweight:

1. **Stage 1 (Builder):** Uses a heavy Alpine Linux image loaded with the Go compiler (`golang:1.24-alpine`). We copy all source code, download the packages, and run:
   ```bash
   CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main ./cmd/api
   ```
   * `CGO_ENABLED=0`: Compiles a completely static binary. It doesn't look for dynamic C-runtime libraries on the host system.
   * `-ldflags="-w -s"`: Strips out debugging symbols to shrink the executable size.
2. **Stage 2 (Final Run Stage):** Starts from a completely fresh, minimal `alpine:latest` image. We copy **only** two things from Stage 1:
   * The single, compiled `main` binary (~15MB).
   * The `db/migration/` SQL files (so the binary can execute migrations on startup).
   
The result? A tiny, secure, 25MB Docker image ready for production.

---

### Orchestration with `docker-compose.yml`
Docker Compose creates an isolated virtual bridge network called `ecommerce_network` and mounts our components:

* **PostgreSQL Service:** Instantiates a Postgres 16 database. It creates a named volume `postgres_data` mapping to `/var/lib/postgresql/data` so your data **survives** when the container is stopped or restarted.
* **API Service:** Tells Docker to compile the `Dockerfile` in the current folder. It injects the connection string dynamically via environment variables:
  ```yaml
  DB_CONN_STR=postgres://postgres:postgres@postgres:5432/ecommerce?sslmode=disable
  ```
  *Notice that the hostname in the connection string is `postgres` (the name of our service) instead of `localhost`. Docker’s internal DNS automatically resolves `postgres` to the container's private IP address.*

---

## 4. Key Design Patterns to Remember

As a software engineer learning Go, keep these patterns in mind as you review the codebase:

1. **Constructor Functions:** Go doesn't have classes or automatic object initialization. We use constructor functions by convention (e.g., `NewRepository()`, `NewService()`, `NewHandler()`) to safely instantiate and configure our structures.
2. **Implicit Interface Implementation:** Notice that the `postgresRepo` struct doesn't have any keyword declaring that it implements `Repository`. Go detects that it matches the interface methods and compiles it cleanly. This is called "duck typing" but with compile-time type-safety.
3. **Explicit Error Handling:** In Go, errors are returned as normal values (`return value, err`). This forces you to handle failure cases immediately at the source, leading to exceptionally robust backend systems.