package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/out"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

type CategoryRepository struct {
	pool *pgxpool.Pool
	ids  out.IDGenerator
}

func NewCategoryRepository(pool *pgxpool.Pool, ids out.IDGenerator) *CategoryRepository {
	return &CategoryRepository{pool: pool, ids: ids}
}

// GetOrCreate is race-safe: the insert is a no-op when the normalized path
// already exists, and the follow-up select wins either way.
func (r *CategoryRepository) GetOrCreate(ctx context.Context, path domain.CategoryPath) (domain.Category, error) {
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO categories (id, path, normalized_path)
		VALUES ($1, $2, $3)
		ON CONFLICT (normalized_path) DO NOTHING`,
		r.ids.NewID(), path.Display(), path.Normalized()); err != nil {
		return domain.Category{}, err
	}

	var id, display, normalized string
	if err := r.pool.QueryRow(ctx,
		`SELECT id, path, normalized_path FROM categories WHERE normalized_path = $1`,
		path.Normalized()).Scan(&id, &display, &normalized); err != nil {
		return domain.Category{}, err
	}
	return domain.Category{ID: id, Path: domain.RehydrateCategoryPath(display, normalized)}, nil
}

func (r *CategoryRepository) ListInUse(ctx context.Context) ([]domain.CategoryWithCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT k.id, k.path, k.normalized_path, count(*) AS in_use
		FROM categories k
		JOIN certifications c ON c.category_id = k.id AND c.deleted_at IS NULL
		GROUP BY k.id, k.path, k.normalized_path
		ORDER BY k.path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.CategoryWithCount
	for rows.Next() {
		var id, display, normalized string
		var count int
		if err := rows.Scan(&id, &display, &normalized, &count); err != nil {
			return nil, err
		}
		out = append(out, domain.CategoryWithCount{
			Category: domain.Category{ID: id, Path: domain.RehydrateCategoryPath(display, normalized)},
			Count:    count,
		})
	}
	return out, rows.Err()
}
