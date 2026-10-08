package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

func (r *CategoryRepository) Create(ctx context.Context, path domain.CategoryPath, color domain.CategoryColor) (domain.Category, error) {
	id := r.ids.NewID()
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO categories (id, path, normalized_path, color)
		VALUES ($1, $2, $3, $4)`,
		id, path.Display(), path.Normalized(), color.String()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return domain.Category{}, domain.SingleViolation("path", "unique", "a category with this path already exists")
		}
		return domain.Category{}, err
	}
	return domain.Category{ID: id, Path: path, Color: color}, nil
}

func (r *CategoryRepository) Update(ctx context.Context, id string, path domain.CategoryPath, color domain.CategoryColor) (domain.Category, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE categories SET path = $2, normalized_path = $3, color = $4 WHERE id = $1`,
		id, path.Display(), path.Normalized(), color.String())
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return domain.Category{}, domain.SingleViolation("path", "unique", "a category with this path already exists")
		}
		return domain.Category{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Category{}, domain.ErrNotFound
	}
	return domain.Category{ID: id, Path: path, Color: color}, nil
}

func (r *CategoryRepository) Delete(ctx context.Context, id string) error {
	var count int
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM certifications WHERE category_id = $1 AND deleted_at IS NULL`,
		id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return domain.SingleViolation("category", "in_use", fmt.Sprintf("category is used by %d certification(s)", count))
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *CategoryRepository) GetByPath(ctx context.Context, path domain.CategoryPath) (domain.Category, error) {
	var id, display, normalized, color string
	err := r.pool.QueryRow(ctx,
		`SELECT id, path, normalized_path, color FROM categories WHERE normalized_path = $1`,
		path.Normalized()).Scan(&id, &display, &normalized, &color)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Category{}, domain.ErrNotFound
		}
		return domain.Category{}, err
	}
	return domain.Category{ID: id, Path: domain.RehydrateCategoryPath(display, normalized), Color: domain.RehydrateCategoryColor(color)}, nil
}

func (r *CategoryRepository) List(ctx context.Context) ([]domain.CategoryWithCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT k.id, k.path, k.normalized_path, k.color, count(c.id) AS in_use
		FROM categories k
		LEFT JOIN certifications c ON c.category_id = k.id AND c.deleted_at IS NULL
		GROUP BY k.id, k.path, k.normalized_path, k.color
		ORDER BY k.path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.CategoryWithCount
	for rows.Next() {
		var id, display, normalized, color string
		var count int
		if err := rows.Scan(&id, &display, &normalized, &color, &count); err != nil {
			return nil, err
		}
		out = append(out, domain.CategoryWithCount{
			Category: domain.Category{ID: id, Path: domain.RehydrateCategoryPath(display, normalized), Color: domain.RehydrateCategoryColor(color)},
			Count:    count,
		})
	}
	return out, rows.Err()
}
