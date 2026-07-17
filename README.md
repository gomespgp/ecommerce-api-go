# Local Development Guide: running the E-Commerce REST API

This guide will walk you through setting up and running your Go e-commerce API locally on your machine. 

Since we are using modern Go tool chaining, you **do not** need to install `sqlc` or `migrate` globally on your system. Everything runs directly through the Go toolchain!

---

## Prerequisites

Before starting, make sure you have the following installed on your machine:
- **Go** (v1.22 or newer recommended)
- **Docker** & **Docker Desktop** (running)

---

## Step-by-Step Launch Guide

### Step 1: Start the PostgreSQL Database
We run PostgreSQL in a Docker container to ensure a clean, isolated database instance.

1. Open your terminal/PowerShell in the project root directory (`ecommerce-api`).
2. Run the database container in the background:
   ```powershell
   docker compose up -d
   ```
3. Verify that the container is running:
   ```powershell
   docker ps
   ```
   *(You should see a container named `ecommerce_db` running on port `5432`).*

---

### Step 2: Apply Database Migrations
This step creates the database tables and indexes using the tool pinned in our project.

1. Ensure you have pinned the migrate tool to your `go.mod` (run this once):
   ```powershell
   go get -tool github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.0
   ```
2. Apply the migration schema:
   ```powershell
   go tool migrate -path db/migration -database "postgres://root:secretpassword@localhost:5432/ecommerce?sslmode=disable" -verbose up
   ```
   *(This reads the files in `db/migration` and builds your schema. If it succeeds, you'll see a series of log lines ending with "Finished").*

---

### Step 3: Generate Type-Safe Go Code
Generate your database query methods from `db/queries/items.sql`.

1. Run the `sqlc` compiler using the pinned project tool:
   ```powershell
   go tool sqlc generate
   ```
   *Note: If you are on native Windows and run into Postgres parsing/engine issues, use the Docker-backed workaround instead:*
   ```powershell
   docker run --rm -v "${PWD}:/src" -w /src sqlc/sqlc generate
   ```

---

### Step 4: Run the Go Server
With your database ready and your database code compiled, start the REST API:

```powershell
go run cmd/api/main.go
```

You should see the following output in your terminal:
```text
Database connection pool established successfully!
Starting Server on port :8080...
```

---

## Step 5: Test the API Endpoints

While the server is running, open a new terminal window or tab to test the endpoints.

### 1. Create a Single Item
```powershell
curl -X POST http://localhost:8080/items `
  -H "Content-Type: application/json" `
  -d '{"name": "Gaming Laptop", "description": "High-end specs", "price": 1299.99, "categories": ["electronics", "gaming"]}'
```

### 2. Create Bulk Items
```powershell
curl -X POST http://localhost:8080/items/bulk `
  -H "Content-Type: application/json" `
  -d '[
    {"name": "Espresso Machine", "description": "15-bar pump", "price": 149.50, "categories": ["kitchen", "appliances"]},
    {"name": "Coffee Beans", "description": "Organic dark roast", "price": 14.99, "categories": ["food", "coffee"]}
  ]'
```

### 3. Retrieve All Items
```powershell
curl http://localhost:8080/items
```

### 4. Search for Items (Matches Name or Description)
```powershell
curl "http://localhost:8080/items?search=Laptop"
```

### 5. Filter Items by Category
```powershell
curl "http://localhost:8080/items?category=coffee"
```

### 6. Delete an Item
*(Replace `1` with the actual ID of the item you want to delete)*
```powershell
curl -X DELETE http://localhost:8080/items/1
```

---

## Troubleshooting & Cleanup

* **Stop the Database:**
  To stop the Postgres database and keep its data:
  ```powershell
  docker compose down
  ```
* **Nuke Database Data:**
  If you ever need to completely wipe out the database data and start from scratch:
  ```powershell
  docker compose down -v
  ```
* **Reverting Migrations:**
  To rollback the database schema:
  ```powershell
  go tool migrate -path db/migration -database "postgres://root:secretpassword@localhost:5432/ecommerce?sslmode=disable" -verbose down