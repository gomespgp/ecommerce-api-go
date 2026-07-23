package main

import (
	"context"
	"ecommerce-api/internal/item"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists (useful for local development outside docker)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	ctx := context.Background()

	dbConnStr := os.Getenv("DB_CONN_STR")

	// 1. Robust Connection: Retry connecting to Postgres until it's ready
	var dbPool *pgxpool.Pool
	var err error

	log.Println("Waiting for database to be ready...")
	for i := 0; i < 10; i++ {
		dbPool, err = pgxpool.New(ctx, dbConnStr)
		if err == nil {
			err = dbPool.Ping(ctx)
		}

		if err == nil {
			break
		}

		log.Printf("Database not ready yet (attempt %d/10): %v. Retrying in 2 seconds...\n", i+1, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatalf("Could not connect to database after retries: %v\n", err)
	}
	log.Println("Database connection pool established successfully!")
	defer dbPool.Close()

	// 2. Run Database Migrations Automatically
	log.Println("Running database migrations...")

	// We need a standard sql.DB connection just for the migrator library
	migrationConnStr := os.Getenv("DB_CONN_STR")

	m, err := migrate.New("file://db/migration", migrationConnStr)
	if err != nil {
		log.Fatalf("Failed to initialize migration engine: %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("Failed to run database migrations: %v", err)
	}
	log.Println("Database migrations applied successfully!")

	// 3. Initialize Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// 4. Initialize Layers and register route groups
	item.RegisterItemRoutes(dbPool, r)

	// 5. Start Server
	port := ":8080"
	log.Printf("Starting Server on port %s...", port)
	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
