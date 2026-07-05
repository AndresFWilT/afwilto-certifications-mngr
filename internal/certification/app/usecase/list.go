package usecase

import (
	"context"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/app/port/out"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

type listService struct {
	certs out.CertificationRepository
}

func NewListService(certs out.CertificationRepository) in.ListCertifications {
	return &listService{certs: certs}
}

func (s *listService) Execute(ctx context.Context, query domain.ListQuery) (domain.Page, error) {
	if err := query.Validate(); err != nil {
		return domain.Page{}, err
	}
	return s.certs.FindPage(ctx, query)
}

type listCategoriesService struct {
	categories out.CategoryRepository
}

func NewListCategoriesService(categories out.CategoryRepository) in.ListCategories {
	return &listCategoriesService{categories: categories}
}

func (s *listCategoriesService) Execute(ctx context.Context) ([]domain.CategoryWithCount, error) {
	return s.categories.ListInUse(ctx)
}
