package item

import (
	"context"
	sql "ecommerce-api/db/sqlc"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidPrice = errors.New("item price must be greater than zero")
	ErrEmptyName    = errors.New("item name cannot be empty")
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, name, description string, price float64, categories []string) (sql.Item, error) {
	if name == "" {
		return sql.Item{}, ErrEmptyName
	}
	if price <= 0 {
		return sql.Item{}, ErrInvalidPrice
	}

	// Format types for sqlc/pgx
	numericPrice := pgtype.Numeric{}
	err := numericPrice.Scan(fmt.Sprintf("%f", price))
	if err != nil {
		return sql.Item{}, err
	}

	arg := sql.CreateItemParams{
		Name:        name,
		Description: description,
		Price:       numericPrice,
		Categories:  categories,
	}

	return s.repo.Create(ctx, arg)
}

func (s *Service) CreateBulk(ctx context.Context, items []struct {
	Name        string
	Description string
	Price       float64
	Categories  []string
}) ([]sql.Item, error) {
	var createdItems []sql.Item
	for _, it := range items {
		created, err := s.Create(ctx, it.Name, it.Description, it.Price, it.Categories)
		if err != nil {
			return nil, fmt.Errorf("failed to create bulk items: error on item '%s': %w", it.Name, err)
		}
		createdItems = append(createdItems, created)
	}
	return createdItems, nil
}

func (s *Service) Update(ctx context.Context, id int64, name, description string, price float64, categories []string) (sql.Item, error) {
	if price <= 0 {
		return sql.Item{}, ErrInvalidPrice
	}

	numericPrice := pgtype.Numeric{}
	if err := numericPrice.Scan(fmt.Sprintf("%f", price)); err != nil {
		return sql.Item{}, err
	}

	arg := sql.UpdateItemParams{
		ID:          id,
		Name:        name,
		Description: description,
		Price:       numericPrice,
		Categories:  categories,
	}

	return s.repo.Update(ctx, arg)
}

func (s *Service) Get(ctx context.Context, id int64) (sql.Item, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]sql.Item, error) {
	return s.repo.List(ctx)
}

func (s *Service) FilterByCategory(ctx context.Context, category string) ([]sql.Item, error) {
	return s.repo.FilterByCategory(ctx, category)
}

func (s *Service) Search(ctx context.Context, term string) ([]sql.Item, error) {
	return s.repo.Search(ctx, term)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
