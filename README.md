# Local Development Guide: running the E-Commerce REST API

This guide will walk you through setting up and running your Go e-commerce API. 

Since we are using modern Go tool chaining, you **do not** need to install tools like `sqlc` or `migrate` globally on your system. Everything runs seamlessly, either fully containerized or directly through the Go toolchain!

---

## Prerequisites

Before starting, make sure you have the following installed on your machine:
- **Go** (v1.24 or newer recommended)
- **Docker** & **Docker Desktop** (running)

---

## Configuration (`.env`)

We manage our environment configuration via a `.env` file. Before launching the app locally or building, copy the example environment file:

```bash
cp .env.example .env
```

The default configuration in `.env` is:
```ini
DB_CONN_STR=postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable
```

---

## How to Run the API

There are two primary ways to run this application: **Option A (Fully Containerized)** or **Option B (Local Development)**.

### Option A: Fully Containerized with Docker Compose (Easiest)
This option compiles and runs both the PostgreSQL database and the Go REST API in isolated containers.

1. Open your terminal in the project root directory.
2. Build and start all services:
   ```bash
   docker compose up --build
   ```
3. That's it!
   - PostgreSQL will boot up.
   - The Go API will wait for PostgreSQL to be ready, **automatically apply database migrations**, and start listening on `http://localhost:8080`.
   - *Note: Inside Docker, the API container's connection string is automatically set to use the Docker bridge network (`postgres` instead of `localhost`), overriding the local `.env` values.*

To stop the services:
```bash
docker compose down
```

To nuke the database data completely and start fresh:
```bash
docker compose down -v
```

---

### Option B: Local Go Development (For Active Code Editing)
If you are actively developing and modifying the Go code, it is much faster to run the Go application directly on your host machine while keeping only the database inside Docker.

#### 1. Start the PostgreSQL Database
Start only the Postgres container in the background:
```bash
docker compose up -d postgres
```
*(Verify it's running via `docker ps`—you should see a container named `ecommerce_db` running on port `5432`).*

#### 2. Run the Go Server
With your `.env` file set up, simply run the server! **Migrations are automatically applied on startup**, so you do not need to run any migration tool manually!

```bash
go run cmd/api/main.go
```

You will see output indicating that the `.env` file was loaded, migrations were applied, and the server is running:
```text
Database connection pool established successfully!
Running database migrations...
Database migrations applied successfully!
Starting Server on port :8080...
```

---

## Modifying Queries & Schema (Advanced)

If you are changing the database structure or adding new queries, you will need to regenerate your Go structures.

### 1. Database Migrations
If you create a new SQL migration file inside `db/migration/` (e.g., to add a table or a column), they will automatically be executed on the next application startup (either via Docker Compose or `go run`).

If you ever need to manually rollback migrations:
```powershell
go get -tool github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1
go tool migrate -path db/migration -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable" -verbose down
```

### 2. Generate Type-Safe Go Code (with `sqlc`)
If you modify or add SQL queries in `db/queries/items.sql`, you need to regenerate the Go code:

```powershell
go tool sqlc generate
```
*Note: If you run into any local environment issues generating with `sqlc`, you can use the Docker-backed generator instead:*
```powershell
docker run --rm -v "${PWD}:/src" -w /src sqlc/sqlc generate
```

---

## Testing the API Endpoints

While the server is running, you can test the endpoints using curl:

### 1. Create a Single Item
```bash
curl -X POST http://localhost:8080/items \
  -H "Content-Type: application/json" \
  -d '{"name": "Gaming Laptop", "description": "High-end specs", "price": 1299.99, "categories": ["electronics", "gaming"]}'
```
*(On Windows PowerShell, use backticks `` ` `` instead of backslashes `\` for line continuation).*

### 2. Create Bulk Items
```bash
curl -X POST http://localhost:8080/items/bulk \
  -H "Content-Type: application/json" \
  -d '[
    {"name": "Espresso Machine", "description": "15-bar pump", "price": 149.50, "categories": ["kitchen", "appliances"]},
    {"name": "Coffee Beans", "description": "Organic dark roast", "price": 14.99, "categories": ["food", "coffee"]}
  ]'
```

### 3. Retrieve All Items
```bash
curl http://localhost:8080/items
```

### 4. Search for Items (Matches Name or Description)
```bash
curl "http://localhost:8080/items?search=Laptop"
```

### 5. Filter Items by Category
```bash
curl "http://localhost:8080/items?category=coffee"
```

### 6. Delete an Item
*(Replace `1` with the actual ID of the item you want to delete)*
```bash
curl -X DELETE http://localhost:8080/items/1
```
