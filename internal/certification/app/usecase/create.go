package usecase

import (
	"context"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/app/port/out"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

type createService struct {
	certs      out.CertificationRepository
	categories out.CategoryRepository
	clock      out.Clock
	ids        out.IDGenerator
}

func NewCreateService(certs out.CertificationRepository, categories out.CategoryRepository, clock out.Clock, ids out.IDGenerator) in.CreateCertification {
	return &createService{certs: certs, categories: categories, clock: clock, ids: ids}
}

func (s *createService) Execute(ctx context.Context, cmd in.CreateCommand) (*domain.Certification, error) {
	name, err := domain.NewCertificationName(cmd.Name)
	if err != nil {
		return nil, err
	}
	path, err := domain.NewCategoryPath(cmd.Category)
	if err != nil {
		return nil, err
	}
	period, err := domain.NewStudyPeriod(cmd.StudyStartDate, cmd.ExpectedExamDate)
	if err != nil {
		return nil, err
	}

	taken, err := s.certs.ExistsByName(ctx, name.String(), "")
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, domain.SingleViolation("name", "unique", "a certification with this name already exists")
	}

	category, err := s.categories.GetOrCreate(ctx, path)
	if err != nil {
		return nil, err
	}

	cert, _, err := domain.NewCertification(s.ids.NewID(), name, category, period, cmd.Goal, s.clock.Now())
	if err != nil {
		return nil, err
	}
	if err := s.certs.Save(ctx, cert); err != nil {
		return nil, err
	}
	return cert, nil
}
