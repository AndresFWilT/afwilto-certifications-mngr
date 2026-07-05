package usecase

import (
	"context"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/app/port/out"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

type updateService struct {
	certs      out.CertificationRepository
	categories out.CategoryRepository
	clock      out.Clock
}

func NewUpdateService(certs out.CertificationRepository, categories out.CategoryRepository, clock out.Clock) in.UpdateCertification {
	return &updateService{certs: certs, categories: categories, clock: clock}
}

// Execute applies merge-patch semantics: absent fields stay untouched, null
// clears nullable fields. Status is applied before evidence so a single patch
// can achieve-and-attach-evidence (GH-E4) while BR-EV1 still rejects evidence
// on non-achieved certifications (GH-E5).
func (s *updateService) Execute(ctx context.Context, id string, cmd in.UpdateCommand) (*domain.Certification, error) {
	cert, err := s.certs.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()

	if cmd.Status.Set {
		if cmd.Status.Value == nil {
			return nil, domain.SingleViolation("status", "not_null", "status cannot be null")
		}
		status, err := domain.ParseStatus(*cmd.Status.Value)
		if err != nil {
			return nil, err
		}
		if err := cert.ChangeStatus(status, now); err != nil {
			return nil, err
		}
	}

	if cmd.Name.Set {
		if cmd.Name.Value == nil {
			return nil, domain.SingleViolation("name", "not_null", "name cannot be null")
		}
		name, err := domain.NewCertificationName(*cmd.Name.Value)
		if err != nil {
			return nil, err
		}
		taken, err := s.certs.ExistsByName(ctx, name.String(), cert.ID())
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, domain.SingleViolation("name", "unique", "a certification with this name already exists")
		}
		if err := cert.Rename(name, now); err != nil {
			return nil, err
		}
	}

	if cmd.Category.Set {
		if cmd.Category.Value == nil {
			return nil, domain.SingleViolation("category", "not_null", "category cannot be null")
		}
		path, err := domain.NewCategoryPath(*cmd.Category.Value)
		if err != nil {
			return nil, err
		}
		category, err := s.categories.GetOrCreate(ctx, path)
		if err != nil {
			return nil, err
		}
		if err := cert.Recategorize(category, now); err != nil {
			return nil, err
		}
	}

	if cmd.StudyStartDate.Set || cmd.ExpectedExamDate.Set {
		start := cert.Period().Start()
		if cmd.StudyStartDate.Set {
			start = cmd.StudyStartDate.Value
		}
		exam := cert.Period().Exam()
		if cmd.ExpectedExamDate.Set {
			exam = cmd.ExpectedExamDate.Value
		}
		period, err := domain.NewStudyPeriod(start, exam)
		if err != nil {
			return nil, err
		}
		if _, err := cert.Reschedule(period, now); err != nil {
			return nil, err
		}
	}

	if cmd.Goal.Set {
		if err := cert.DescribeGoal(cmd.Goal.Value, now); err != nil {
			return nil, err
		}
	}

	if cmd.CredlyURL.Set {
		if err := cert.SetCredlyURL(cmd.CredlyURL.Value, now); err != nil {
			return nil, err
		}
	}
	if cmd.OfficialURL.Set {
		if err := cert.SetOfficialURL(cmd.OfficialURL.Value, now); err != nil {
			return nil, err
		}
	}
	if cmd.AchievedDate.Set {
		if err := cert.SetAchievedDate(cmd.AchievedDate.Value, now); err != nil {
			return nil, err
		}
	}

	if err := s.certs.Save(ctx, cert); err != nil {
		return nil, err
	}
	return cert, nil
}
