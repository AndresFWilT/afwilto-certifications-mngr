package out

import (
	"context"

	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

// CategoryRepository is an explicit CRUD surface: categories are an
// admin-managed resource, never auto-created on certification write.
type CategoryRepository interface {
	Create(ctx context.Context, path domain.CategoryPath, color domain.CategoryColor) (domain.Category, error)
	Update(ctx context.Context, id string, path domain.CategoryPath, color domain.CategoryColor) (domain.Category, error)
	Delete(ctx context.Context, id string) error
	GetByPath(ctx context.Context, path domain.CategoryPath) (domain.Category, error)
	List(ctx context.Context) ([]domain.CategoryWithCount, error)
}
