package postgres

import (
	"time"

	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

type certRow struct {
	ID               string
	Name             string
	Goal             *string
	Status           string
	StudyStartDate   *time.Time
	ExpectedExamDate *time.Time
	AchievedDate     *time.Time
	CredlyURL        *string
	OfficialURL      *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
	CategoryID       string
	CategoryPath     string
	CategoryNorm     string
	CategoryColor    string
}

type attachmentRow struct {
	ID              string
	CertificationID string
	Kind            string
	FileName        string
	ContentType     string
	SizeBytes       int64
	StorageKey      string
	CreatedAt       time.Time
}

func (r certRow) toDomain(attachments []attachmentRow) (*domain.Certification, error) {
	name, err := domain.NewCertificationName(r.Name)
	if err != nil {
		return nil, err
	}
	period, err := domain.NewStudyPeriod(r.StudyStartDate, r.ExpectedExamDate)
	if err != nil {
		return nil, err
	}
	atts := make([]domain.Attachment, 0, len(attachments))
	for _, a := range attachments {
		atts = append(atts, domain.Attachment{
			ID:          a.ID,
			Kind:        domain.AttachmentKind(a.Kind),
			FileName:    a.FileName,
			ContentType: a.ContentType,
			SizeBytes:   a.SizeBytes,
			StorageKey:  a.StorageKey,
			CreatedAt:   a.CreatedAt,
		})
	}
	return domain.Rehydrate(
		r.ID,
		name,
		domain.Category{ID: r.CategoryID, Path: domain.RehydrateCategoryPath(r.CategoryPath, r.CategoryNorm), Color: domain.RehydrateCategoryColor(r.CategoryColor)},
		period,
		r.Goal,
		domain.AchievementStatus(r.Status),
		domain.RehydrateEvidence(r.CredlyURL, r.OfficialURL, r.AchievedDate),
		atts,
		r.CreatedAt,
		r.UpdatedAt,
		r.DeletedAt,
	), nil
}
