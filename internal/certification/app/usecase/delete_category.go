package usecase

import (
	"context"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/app/port/out"
)

type deleteCategoryService struct {
	categories out.CategoryRepository
}

func NewDeleteCategoryService(categories out.CategoryRepository) in.DeleteCategory {
	return &deleteCategoryService{categories: categories}
}

func (s *deleteCategoryService) Execute(ctx context.Context, id string) error {
	return s.categories.Delete(ctx, id)
}
