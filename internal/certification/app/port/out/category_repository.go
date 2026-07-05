package out

import (
	"context"

	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

// CategoryRepository realizes the CategoryResolver rule together with the
// CategoryPath VO: categories are created implicitly on first use and listed
// only while non-deleted certifications reference them.
type CategoryRepository interface {
	GetOrCreate(ctx context.Context, path domain.CategoryPath) (domain.Category, error)
	ListInUse(ctx context.Context) ([]domain.CategoryWithCount, error)
}
