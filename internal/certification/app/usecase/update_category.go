package usecase

import (
	"context"
	"errors"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/app/port/out"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

type updateCategoryService struct {
	categories out.CategoryRepository
}

func NewUpdateCategoryService(categories out.CategoryRepository) in.UpdateCategory {
	return &updateCategoryService{categories: categories}
}

func (s *updateCategoryService) Execute(ctx context.Context, id string, cmd in.UpdateCategoryCommand) (domain.Category, error) {
	all, err := s.categories.List(ctx)
	if err != nil {
		return domain.Category{}, err
	}
	var current *domain.Category
	for i := range all {
		if all[i].ID == id {
			current = &all[i].Category
			break
		}
	}
	if current == nil {
		return domain.Category{}, domain.ErrNotFound
	}

	path := current.Path
	if cmd.Path != nil {
		p, err := domain.NewCategoryPath(*cmd.Path)
		if err != nil {
			return domain.Category{}, err
		}
		path = p
	}
	color := current.Color
	if cmd.Color != nil {
		c, err := domain.NewCategoryColor(*cmd.Color)
		if err != nil {
			return domain.Category{}, err
		}
		color = c
	}

	if path.Normalized() != current.Path.Normalized() {
		existing, err := s.categories.GetByPath(ctx, path)
		if err == nil && existing.ID != id {
			return domain.Category{}, domain.SingleViolation("path", "unique", "a category with this path already exists")
		}
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return domain.Category{}, err
		}
	}

	return s.categories.Update(ctx, id, path, color)
}
