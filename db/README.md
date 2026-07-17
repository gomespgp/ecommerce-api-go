# Guide: Using sqlc for Type-Safe SQL in Go

`sqlc` compiles raw SQL queries into type-safe, idiomatic Go code. This means you get the performance and flexibility of raw SQL without having to write boilerplate mapping code or use complex, heavy ORMs.

This guide explains how to configure and run `sqlc` in this project using different workflows (as a modern localized Go tool, a global CLI, or via Docker for OS compatibility).

---

## 1. Project Structure for sqlc

Our project is structured as follows:
- `db/migration/`: Contains your raw SQL schema migration files (e.g., `000001_init_items.up.sql`).
- `db/queries/`: Contains your raw SQL query files (e.g., `items.sql`).
- `db/sqlc/`: The output folder where `sqlc` generates type-safe Go code.
- `sqlc.yaml`: The configuration file in the project root.

---

## 2. Configuration (`sqlc.yaml`)

The `sqlc.yaml` file in your root folder coordinates how code is generated. For a PostgreSQL database using the modern `pgx/v5` driver, the configuration looks like this:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "db/migration"
    queries: "db/queries"
    gen:
      go:
        package: "db"
        out: "db/sqlc"
        sql_package: "pgx/v5"
```

---

## 3. How to Generate Code

Depending on your operating system and environment, choose **one** of the following methods to generate your code.

### Method A: As a Localized Go Tool (Recommended)
This is the modern Go way. It locks the version of `sqlc` directly inside your `go.mod` file, ensuring everyone on your team compiles with the exact same version without needing global installations.

1. **Pin the tool to your project (done once):**
   ```bash
   go get -tool github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
   ```

2. **Generate the code:**
   ```bash
   go tool sqlc generate
   ```

---

### Method B: As a Global CLI
If you prefer to have `sqlc` globally installed on your machine's system path:

1. **Install the binary globally:**
   ```bash
   go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
   ```

2. **Generate the code:**
   Make sure your `GOBIN` (e.g., `$HOME/go/bin` or `%USERPROFILE%\go\bin` on Windows) is in your system's `PATH` variable, then run:
   ```bash
   sqlc generate
   ```

---

### Method C: Using Docker (Windows Workaround)
If you are on native Windows and running into Postgres parser engine errors (`unknown engine`), Docker is the cleanest workaround. Since the PostgreSQL parser relies on CGo dependencies that struggle on Windows, running it inside Linux Docker works perfectly.

1. Run this command in **PowerShell** from your project root folder:
   ```powershell
   docker run --rm -v "${PWD}:/src" -w /src sqlc/sqlc generate
   ```

---

## 4. What sqlc Generates

After running the generator, you will see three new files created inside your `db/sqlc/` folder:

1. **`models.go`**: Contains Go struct representations of your database tables (e.g., `Item` struct matching the `items` table).
2. **`db.go`**: Defines interfaces like `DBTX` (which accommodates both single connections and transaction pools) and prepares queries.
3. **`items.sql.go`**: Contains the actual compiled Go functions mapping directly to your SQL queries (e.g., `CreateItem()`, `ListItems()`, `SearchItems()`).

---

## 5. Development Workflow

Whenever you change your schema or write new database operations:
1. Write/edit your migration `.sql` files in `db/migration/`.
2. Write/edit your queries in `db/queries/*.sql`.
3. Re-run your preferred generation command (e.g., `go tool sqlc generate`).
4. Update your repository layer in Go to utilize the newly generated functions.