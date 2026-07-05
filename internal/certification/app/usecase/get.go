package usecase

import (
	"context"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/app/port/out"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

type getService struct {
	certs out.CertificationRepository
}

func NewGetService(certs out.CertificationRepository) in.GetCertification {
	return &getService{certs: certs}
}

func (s *getService) Execute(ctx context.Context, id string) (*domain.Certification, error) {
	return s.certs.FindByID(ctx, id)
}
