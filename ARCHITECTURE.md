# Deep Dive: Architecture & Implementation of our Go E-Commerce REST API

Welcome to the architectural blueprint of your new Go E-Commerce REST API. This document breaks down **how** the application is structured, **why** certain design choices were made, and **how** every file communicates from the infrastructure layer down to the database.

---

## 1. Architectural Overview: Clean Architecture

This application uses a variation of **Clean Architecture** (often called Ports and Adapters or Three-Tier Architecture). 

```text
       ┌───────────────────────────────────────────────────────────────────────┐
       │                             HTTP Request                              │
       └──────────────────────────────────┬────────────────────────────────────┘
                                          ▼
       ┌───────────────────────────────────────────────────────────────────────┐
       │   Presentation / Transport Layer (internal/item/handler.go)           │
       │   - Listens on HTTP routes, handles JSON serialization                │
       │   - Decodes HTTP JSON into Go Structs (like DTOs / Pydantic models)   │
       │   - Dispatches parsed parameters to the Service Layer                 │
       └──────────────────────────────────┬────────────────────────────────────┘
                                          ▼
       ┌───────────────────────────────────────────────────────────────────────┐
       │   Business Logic Layer (internal/item/service.go)                     │
       │   - Validates data (e.g., price > 0, non-empty names)                 │
       │   - Orchestrates multi-step business transactions                     │
       │   - Knows NOTHING about HTTP, URLs, or JSON response codes            │
       └──────────────────────────────────┬────────────────────────────────────┘
                                          ▼
       ┌───────────────────────────────────────────────────────────────────────┐
       │   Data Access Layer (internal/item/repository.go)                     │
       │   - Interface-driven database operations                              │
       │   - Maps Go objects to SQL query parameters                           │
       │   - Communicates with Postgres via sqlc-generated code                │
       └───────────────────────────────────────────────────────────────────────┘
```

The core principle is the **Dependency Inversion Principle**: Higher-level modules (like the Service) do not depend on lower-level modules (like the specific Postgres driver). Instead, they depend on **Interfaces**. This allows you to completely swap out your database layer in the future without changing a single line of business logic.

---

## 2. A Data Engineer's Perspective: Mental Model (Go vs. Python)

If you are coming from a Python-centric background (using frameworks like FastAPI, Flask, SQLAlchemy, Pandas, or PySpark), here is a translation guide to help you build the right mental model for Go:

| Python Concept | Go Equivalent | Key Architectural & Performance Differences |
| :--- | :--- | :--- |
| **FastAPI / Flask Route** | **Chi Router & Handler Method** | In Go, routing is extremely fast and light. Go's standard library `net/http` combined with `go-chi` is highly performant and uses concurrent threads (Goroutines) for every request natively, without needing `async`/`await` keyword clutter. |
| **Pydantic Model / Marshmallow** | **Go `struct` with tags (e.g. ``json:"name"``)** | Instead of heavy validation libraries running at runtime, Go uses lightweight typed structs. `json.NewDecoder(r.Body).Decode(&req)` parses JSON directly into typed Go structs with compile-time safety. |
| **SQLAlchemy / Django ORM** | **`sqlc` (Compile-time Type-Safe SQL)** | Rather than using heavy runtime ORM engines that translate Python objects to SQL behind the scenes (often introducing hidden N+1 query problems and high CPU overhead), we write **raw SQL**. `sqlc` compiles that SQL into safe, optimized, native Go functions at compile time. |
| **Type Hints / Protocols (`typing.Protocol`)** | **Go `interface`** | Go interfaces are **implicitly** implemented. If a struct has the methods declared in an interface, Go considers it implemented (like static duck typing). No explicit subclassing or inheritance is required. |
| **Dependency Injection Container** | **Manual Dependency Injection in `main.go`** | We don't use heavy frameworks. We construct structs from the bottom up: DB Pool ➔ Repository ➔ Service ➔ Handler ➔ Router. It is fully transparent, making it easy to trace exactly where database handles and configurations are used. |
| **FastAPI `BackgroundTasks` / Threading** | **`context.Context` & Goroutines** | Go uses a `context.Context` parameter across all layers to manage request deadlines, cancellations, and scope-specific metadata. |

---

## 3. End-to-End Walkthrough: How a Request Becomes a Database Record

Let's trace exactly how an HTTP `POST /items` request containing JSON data flows through each layer and is persisted to the database.

```text
 [ HTTP Client ]
      │
      │ 1. POST /items {"name": "Laptop", "price": 999.99, ...}
      ▼
 [ Router (cmd/api/main.go) ]
      │
      │ 2. Routes request to h.Create() inside Handler
      ▼
 [ Presentation Layer: Handler (internal/item/handler.go) ]
      │
      │ 3. Decodes JSON into `createItemRequest` struct
      │ 4. Extracts parameters & calls h.service.Create()
      ▼
 [ Business Logic Layer: Service (internal/item/service.go) ]
      │
      │ 5. Validates parameters (e.g. Price > 0)
      │ 6. Converts float64 to pgtype.Numeric (Postgres compatible)
      │ 7. Assembles `sql.CreateItemParams` and calls s.repo.Create()
      ▼
 [ Data Access Layer: Repository (internal/item/repository.go) ]
      │
      │ 8. Executes `r.queries.CreateItem(ctx, arg)`
      ▼
 [ Autogenerated Database Client (db/sqlc/items.sql.go) ]
      │
      │ 9. Queries Postgres using safe parameterized SQL inserts
      ▼
 [ PostgreSQL Database ]
```

### Step 1: Receiving & Routing (`cmd/api/main.go`)
An HTTP request is sent by the client. The router, configured in `main.go`, matches the route path `/items` and the HTTP verb `POST` to the corresponding handler method:
```go
r.Route("/items", func(r chi.Router) {
    r.Post("/", handler.Create) // Maps to handler.Create method
})
```

### Step 2: Parsing & Parsing Validation (`internal/item/handler.go`)
The `handler.Create` method handles HTTP-specific concerns:
1. **JSON Decoding:** It reads the HTTP request stream (`r.Body`) and decodes it into a request-specific Transfer Object (DTO) struct (`createItemRequest`).
2. **Error Isolation:** If the payload is malformed JSON, it responds immediately with an `HTTP 400 Bad Request` without stressing the database or business layers.
3. **Delegation:** It calls the service layer with the context and individual parameters:
   ```go
   item, err := h.service.Create(r.Context(), req.Name, req.Description, req.Price, req.Categories)
   ```

### Step 3: Business Validation & Formatting (`internal/item/service.go`)
The `Service` receives the parameters. It does not know or care that they originated from an HTTP request.
1. **Business Rules Enforcement:** It checks business-critical constraints (e.g., item price cannot be negative, item name cannot be empty).
2. **Data-Type Conversion:** It translates domain representations into database-optimized types. For example, it safely converts a Go float64 into a Postgres-compatible arbitrary-precision type `pgtype.Numeric` using string scaling.
3. **Data Delegation:** It packs the database query parameters structure (`sql.CreateItemParams`) and forwards it down to the Repository:
   ```go
   return s.repo.Create(ctx, arg)
   ```

### Step 4: Interfacing the Database (`internal/item/repository.go`)
The Repository is an interface that isolates the business logic from direct database technologies.
1. Our concrete repository `postgresRepo` wraps the database queries generated by `sqlc`:
   ```go
   func (r *postgresRepo) Create(ctx context.Context, arg sql.CreateItemParams) (sql.Item, error) {
       return r.queries.CreateItem(ctx, arg)
   }
   ```
2. The `sqlc` library's generated code `r.queries.CreateItem` prepares the SQL statement, passes the bound variables to PostgreSQL, executes the query, maps the resulting row back into a compiled Go `sql.Item` struct, and returns it.

---

## 4. Component-by-Component Walkthrough

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
Instead of using a bulky Object-Relational Mapper (ORM), we use **`sqlc`** to write raw, high-performance SQL.

* **`db/migration/`**: Contains raw SQL schema definitions (e.g., creating the `items` table and indexing categories).
* **`db/queries/items.sql`**: Your raw SQL queries. Notice the annotations:
  ```sql
  -- name: CreateItem :one
  INSERT INTO items (name, description, price, categories)
  VALUES ($1, $2, $3, $4)
  RETURNING *;
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

## 5. The "Infra" Setup (Docker & Compose)

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

## 6. Key Design Patterns to Remember

As a software engineer learning Go, keep these patterns in mind as you review the codebase:

1. **Constructor Functions:** Go doesn't have classes or automatic object initialization. We use constructor functions by convention (e.g., `NewRepository()`, `NewService()`, `NewHandler()`) to safely instantiate and configure our structures.
2. **Implicit Interface Implementation:** Notice that the `postgresRepo` struct doesn't have any keyword declaring that it implements `Repository`. Go detects that it matches the interface methods and compiles it cleanly. This is called "duck typing" but with compile-time type-safety.
3. **Explicit Error Handling:** In Go, errors are returned as normal values (`return value, err`). This forces you to handle failure cases immediately at the source, leading to exceptionally robust backend systems.

---

## 7. Developer Guides: Extending the API

These step-by-step instructions detail exactly how to introduce changes to the API, end-to-end, showing you all the layers that need to be touched.

---

### Guide 7.1: How to Add a New Endpoint to an Existing Route
*Scenario: We want to add an endpoint `PATCH /items/{id}/price` to update only the price of an item.*

To accomplish this, we proceed from the database layer upwards to the presentation layer.

#### Step 1: Define or verify the SQL Query
In this scenario, we can reuse the existing `UpdateItem` or write a specific SQL query in `db/queries/items.sql`. Let's assume we want a lightweight, dedicated price update query.
Add the query to `db/queries/items.sql`:
```sql
-- name: UpdateItemPrice :one
UPDATE items
SET price = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;
```

#### Step 2: Compile the SQL Query with `sqlc`
Regenerate Go structures and methods matching the SQL files:
```bash
sqlc generate
```
This updates files inside `db/sqlc/` (specifically `items.sql.go` will now contain a `UpdateItemPrice` method, and `models.go` or `db.go` might have corresponding structural updates).

#### Step 3: Update the Repository Layer (`internal/item/repository.go`)
1. Add the method signature to the `Repository` interface:
   ```go
   type Repository interface {
       // ... existing methods ...
       UpdatePrice(ctx context.Context, id int64, price pgtype.Numeric) (sql.Item, error)
   }
   ```
2. Implement the method on the concrete `postgresRepo` struct:
   ```go
   func (r *postgresRepo) UpdatePrice(ctx context.Context, id int64, price pgtype.Numeric) (sql.Item, error) {
       return r.queries.UpdateItemPrice(ctx, sql.UpdateItemPriceParams{
           ID:    id,
           Price: price,
       })
   }
   ```

#### Step 4: Update the Service Layer (`internal/item/service.go`)
Add the business logic method to enforce domain validation rules and map input types to database types:
```go
func (s *Service) UpdatePrice(ctx context.Context, id int64, price float64) (sql.Item, error) {
    if price <= 0 {
        return sql.Item{}, ErrInvalidPrice
    }

    numericPrice := pgtype.Numeric{}
    if err := numericPrice.Scan(fmt.Sprintf("%f", price)); err != nil {
        return sql.Item{}, err
    }

    return s.repo.UpdatePrice(ctx, id, numericPrice)
}
```

#### Step 5: Update the Presentation/Transport Layer (`internal/item/handler.go`)
1. Create a request struct (DTO) at the top or inside the handler:
   ```go
   type updatePriceRequest struct {
       Price float64 `json:"price"`
   }
   ```
2. Write a handler function on the `Handler` struct:
   ```go
   func (h *Handler) UpdatePrice(w http.ResponseWriter, r *http.Request) {
       idStr := chi.URLParam(r, "id")
       id, err := strconv.ParseInt(idStr, 10, 64)
       if err != nil {
           http.Error(w, "Invalid ID parameter", http.StatusBadRequest)
           return
       }

       var req updatePriceRequest
       if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
           http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
           return
       }

       item, err := h.service.UpdatePrice(r.Context(), id, req.Price)
       if err != nil {
           http.Error(w, err.Error(), http.StatusBadRequest)
           return
       }

       w.Header().Set("Content-Type", "application/json")
       json.NewEncoder(w).Encode(item)
   }
   ```

#### Step 6: Map Route to Handler in router (`cmd/api/main.go`)
Register the endpoint inside the `/items` route group:
```go
r.Route("/items", func(r chi.Router) {
    // ... existing endpoints ...
    r.Patch("/{id}/price", handler.UpdatePrice)
})
```

---

### Guide 7.2: How to Add a Completely New Route (End-to-End)
*Scenario: We want to add a completely new `orders` domain to keep track of purchases, starting with `POST /orders` and `GET /orders/{id}`.*

We will create a clean separation of concerns in a brand new module: `/internal/order`.

#### Step 1: Create a Database Migration
Generate new migration files or write a raw SQL migration to create the `orders` database schema.
Create `db/migration/000002_create_orders_table.up.sql`:
```sql
CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    item_id BIGINT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    quantity INT NOT NULL,
    total_price NUMERIC(10, 2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);
```
*(Also create a corresponding `.down.sql` file containing `DROP TABLE IF EXISTS orders;` if you wish to support migrations rollback).*

#### Step 2: Define SQL Queries in a dedicated file
Create a new SQL file at `db/queries/orders.sql`:
```sql
-- name: CreateOrder :one
INSERT INTO orders (item_id, quantity, total_price)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetOrder :one
SELECT * FROM orders WHERE id = $1;
```

#### Step 3: Run `sqlc generate`
Execute the code generation tool:
```bash
sqlc generate
```
This compiles the queries into type-safe Go structs and functions in `db/sqlc/orders.sql.go` and `db/sqlc/models.go`.

#### Step 4: Scaffold the new domain package (`internal/order/`)
Create a new folder `internal/order` containing three standard clean architecture files:

##### 1. `internal/order/repository.go`
```go
package order

import (
    "context"
    sql "ecommerce-api/db/sqlc"
)

type Repository interface {
    Create(ctx context.Context, arg sql.CreateOrderParams) (sql.Order, error)
    Get(ctx context.Context, id int64) (sql.Order, error)
}

type postgresRepo struct {
    queries *sql.Queries
}

func NewRepository(db sql.DBTX) Repository {
    return &postgresRepo{
        queries: sql.New(db),
    }
}

func (r *postgresRepo) Create(ctx context.Context, arg sql.CreateOrderParams) (sql.Order, error) {
    return r.queries.CreateOrder(ctx, arg)
}

func (r *postgresRepo) Get(ctx context.Context, id int64) (sql.Order, error) {
    return r.queries.GetOrder(ctx, id)
}
```

##### 2. `internal/order/service.go`
```go
package order

import (
    "context"
    sql "ecommerce-api/db/sqlc"
    "errors"
    "fmt"
    "github.com/jackc/pgx/v5/pgtype"
)

var (
    ErrInvalidQuantity = errors.New("quantity must be greater than zero")
)

type Service struct {
    repo Repository
}

func NewService(repo Repository) *Service {
    return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, itemID int64, quantity int32, totalPrice float64) (sql.Order, error) {
    if quantity <= 0 {
        return sql.Order{}, ErrInvalidQuantity
    }

    numericPrice := pgtype.Numeric{}
    if err := numericPrice.Scan(fmt.Sprintf("%f", totalPrice)); err != nil {
        return sql.Order{}, err
    }

    arg := sql.CreateOrderParams{
        ItemID:     itemID,
        Quantity:   quantity,
        TotalPrice: numericPrice,
    }

    return s.repo.Create(ctx, arg)
}

func (s *Service) Get(ctx context.Context, id int64) (sql.Order, error) {
    return s.repo.Get(ctx, id)
}
```

##### 3. `internal/order/handler.go`
```go
package order

import (
    "encoding/json"
    "net/http"
    "strconv"
    "github.com/go-chi/chi/v5"
)

type Handler struct {
    service *Service
}

func NewHandler(service *Service) *Handler {
    return &Handler{service: service}
}

type createOrderRequest struct {
    ItemID     int64   `json:"item_id"`
    Quantity   int32   `json:"quantity"`
    TotalPrice float64 `json:"total_price"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
    var req createOrderRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
        return
    }

    order, err := h.service.Create(r.Context(), req.ItemID, req.Quantity, req.TotalPrice)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(order)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
    idStr := chi.URLParam(r, "id")
    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid ID parameter", http.StatusBadRequest)
        return
    }

    order, err := h.service.Get(r.Context(), id)
    if err != nil {
        http.Error(w, "Order not found", http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(order)
}
```

#### Step 5: Inject Dependencies and Register routes in the Server Orchestrator (`cmd/api/main.go`)
1. Import the new order domain:
   ```go
   import (
       // ... existing imports ...
       "ecommerce-api/internal/order"
   )
   ```
2. Wire up the database pool to the repository, the repository to the service, and the service to the handler:
   ```go
   // --- Existing wires ---
   repo := item.NewRepository(dbPool)
   service := item.NewService(repo)
   handler := item.NewHandler(service)

   // --- New Order wires ---
   orderRepo := order.NewRepository(dbPool)
   orderService := order.NewService(orderRepo)
   orderHandler := order.NewHandler(orderService)
   ```
3. Map the base path `/orders` to the new order route handlers in the Chi router configuration:
   ```go
   // Route Group for Orders
   r.Route("/orders", func(r chi.Router) {
       r.Post("/", orderHandler.Create)
       r.Get("/{id}", orderHandler.Get)
   })
   ```

With these 5 simple steps, you have successfully set up a completely isolated, modular, type-safe route in our Clean Architecture!
