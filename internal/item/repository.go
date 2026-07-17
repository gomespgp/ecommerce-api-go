package item

import (
	"context"
	sql "ecommerce-api/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// Repository interface decouples business logic from DB framework
type Repository interface {
	Create(ctx context.Context, arg sql.CreateItemParams) (sql.Item, error)
	Update(ctx context.Context, arg sql.UpdateItemParams) (sql.Item, error)
	Get(ctx context.Context, id int64) (sql.Item, error)
	List(ctx context.Context) ([]sql.Item, error)
	FilterByCategory(ctx context.Context, category string) ([]sql.Item, error)
	Search(ctx context.Context, term string) ([]sql.Item, error)
	Delete(ctx context.Context, id int64) error
}

type postgresRepo struct {
	queries *sql.Queries
}

func NewRepository(db sql.DBTX) Repository {
	return &postgresRepo{
		queries: sql.New(db),
	}
}

func (r *postgresRepo) Create(ctx context.Context, arg sql.CreateItemParams) (sql.Item, error) {
	return r.queries.CreateItem(ctx, arg)
}

func (r *postgresRepo) Update(ctx context.Context, arg sql.UpdateItemParams) (sql.Item, error) {
	return r.queries.UpdateItem(ctx, arg)
}

func (r *postgresRepo) Get(ctx context.Context, id int64) (sql.Item, error) {
	return r.queries.GetItem(ctx, id)
}

func (r *postgresRepo) List(ctx context.Context) ([]sql.Item, error) {
	return r.queries.ListItems(ctx)
}

func (r *postgresRepo) FilterByCategory(ctx context.Context, category string) ([]sql.Item, error) {
	return r.queries.FilterItemsByCategory(ctx, category)
}

func (r *postgresRepo) Search(ctx context.Context, term string) ([]sql.Item, error) {
	textParam := pgtype.Text{
		String: term,
		Valid:  true, // Tells Postgres this value is NOT NULL
	}
	return r.queries.SearchItems(ctx, textParam)
}

func (r *postgresRepo) Delete(ctx context.Context, id int64) error {
	return r.queries.DeleteItem(ctx, id)
}
