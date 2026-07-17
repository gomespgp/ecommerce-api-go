-- name: CreateItem :one
INSERT INTO items (name, description, price, categories)
VALUES ($1, $2, $3, $4)
RETURNING id, name, description, price, categories, created_at, updated_at;

-- name: UpdateItem :one
UPDATE items
SET name = $2, description = $3, price = $4, categories = $5, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, name, description, price, categories, created_at, updated_at;

-- name: GetItem :one
SELECT id, name, description, price, categories, created_at, updated_at
FROM items
WHERE id = $1 LIMIT 1;

-- name: ListItems :many
SELECT id, name, description, price, categories, created_at, updated_at
FROM items
ORDER BY id;

-- name: FilterItemsByCategory :many
SELECT id, name, description, price, categories, created_at, updated_at
FROM items
WHERE $1::text = ANY(categories)
ORDER BY id;

-- name: SearchItems :many
SELECT id, name, description, price, categories, created_at, updated_at
FROM items
WHERE name ILIKE '%' || $1 || '%' OR description ILIKE '%' || $1 || '%'
ORDER BY id;

-- name: DeleteItem :exec
DELETE FROM items
WHERE id = $1;