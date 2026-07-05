package usecase

import (
	"context"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/app/port/out"
)

type deleteService struct {
	certs out.CertificationRepository
	clock out.Clock
}

func NewDeleteService(certs out.CertificationRepository, clock out.Clock) in.DeleteCertification {
	return &deleteService{certs: certs, clock: clock}
}

func (s *deleteService) Execute(ctx context.Context, id string) error {
	cert, err := s.certs.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if _, err := cert.SoftDelete(s.clock.Now()); err != nil {
		return err
	}
	return s.certs.Save(ctx, cert)
}
