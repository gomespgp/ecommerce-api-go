package item

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterItemRoutes initialize Layers and registers all item routes with the given router
func RegisterItemRoutes(dbPool *pgxpool.Pool, r chi.Router) {
	// 1. Internal wiring
	repo := NewRepository(dbPool)
	service := NewService(repo)
	handler := NewHandler(service)

	// 2. Route definitions
	r.Route("/items", func(r chi.Router) {
		r.Post("/", handler.Create)
		r.Post("/bulk", handler.CreateBulk)
		r.Get("/", handler.List)
		r.Get("/{id}", handler.Get)
		r.Put("/{id}", handler.Update)
		r.Delete("/{id}", handler.Delete)
	})
}
