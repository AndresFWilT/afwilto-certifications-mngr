package usecase

import (
	"context"
	"errors"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/app/port/out"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

type createCategoryService struct {
	categories out.CategoryRepository
}

func NewCreateCategoryService(categories out.CategoryRepository) in.CreateCategory {
	return &createCategoryService{categories: categories}
}

func (s *createCategoryService) Execute(ctx context.Context, cmd in.CreateCategoryCommand) (domain.Category, error) {
	path, err := domain.NewCategoryPath(cmd.Path)
	if err != nil {
		return domain.Category{}, err
	}
	color, err := domain.NewCategoryColor(cmd.Color)
	if err != nil {
		return domain.Category{}, err
	}

	if _, err := s.categories.GetByPath(ctx, path); err == nil {
		return domain.Category{}, domain.SingleViolation("path", "unique", "a category with this path already exists")
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.Category{}, err
	}

	return s.categories.Create(ctx, path, color)
}
