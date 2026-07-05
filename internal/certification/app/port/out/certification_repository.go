package out

import (
	"context"

	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

// CertificationRepository persists the aggregate (certification + attachment
// metadata together). Soft-deleted aggregates are invisible to FindByID and
// FindPage; Save persists the deletedAt marker.
type CertificationRepository interface {
	Save(ctx context.Context, cert *domain.Certification) error
	FindByID(ctx context.Context, id string) (*domain.Certification, error)
	FindPage(ctx context.Context, query domain.ListQuery) (domain.Page, error)
	ExistsByName(ctx context.Context, name string, excludeID string) (bool, error)
	FindByAttachmentID(ctx context.Context, attachmentID string) (*domain.Certification, error)
}
