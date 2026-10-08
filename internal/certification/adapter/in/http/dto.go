package http

import (
	"encoding/json"
	"time"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

// Opt distinguishes absent (Set=false) from explicit null (Set=true,
// Value=nil) so PATCH keeps merge-patch semantics.
type Opt[T any] struct {
	Set   bool
	Value *T
}

func (o *Opt[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

const dateLayout = "2006-01-02"

func parseDate(field string, raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	d, err := time.Parse(dateLayout, *raw)
	if err != nil {
		return nil, domain.SingleViolation(field, "format", field+" must be an ISO date (YYYY-MM-DD)")
	}
	return &d, nil
}

func formatDate(d *time.Time) *string {
	if d == nil {
		return nil
	}
	s := d.Format(dateLayout)
	return &s
}

// ── Requests ────────────────────────────────────────────────────────────────

type createRequest struct {
	Name             string  `json:"name"`
	Category         string  `json:"category"`
	StudyStartDate   *string `json:"studyStartDate"`
	ExpectedExamDate *string `json:"expectedExamDate"`
	Goal             *string `json:"goal"`
}

func (r createRequest) toCommand() (in.CreateCommand, error) {
	start, err := parseDate("studyStartDate", r.StudyStartDate)
	if err != nil {
		return in.CreateCommand{}, err
	}
	exam, err := parseDate("expectedExamDate", r.ExpectedExamDate)
	if err != nil {
		return in.CreateCommand{}, err
	}
	return in.CreateCommand{
		Name:             r.Name,
		Category:         r.Category,
		StudyStartDate:   start,
		ExpectedExamDate: exam,
		Goal:             r.Goal,
	}, nil
}

type updateRequest struct {
	Name             Opt[string] `json:"name"`
	Category         Opt[string] `json:"category"`
	Status           Opt[string] `json:"status"`
	StudyStartDate   Opt[string] `json:"studyStartDate"`
	ExpectedExamDate Opt[string] `json:"expectedExamDate"`
	Goal             Opt[string] `json:"goal"`
	AchievedDate     Opt[string] `json:"achievedDate"`
	CredlyURL        Opt[string] `json:"credlyUrl"`
	OfficialURL      Opt[string] `json:"officialUrl"`
}

func stringPatch(o Opt[string]) in.Patch[string] {
	return in.Patch[string]{Set: o.Set, Value: o.Value}
}

func datePatch(field string, o Opt[string]) (in.Patch[time.Time], error) {
	if !o.Set {
		return in.Patch[time.Time]{}, nil
	}
	d, err := parseDate(field, o.Value)
	if err != nil {
		return in.Patch[time.Time]{}, err
	}
	return in.Patch[time.Time]{Set: true, Value: d}, nil
}

func (r updateRequest) toCommand() (in.UpdateCommand, error) {
	start, err := datePatch("studyStartDate", r.StudyStartDate)
	if err != nil {
		return in.UpdateCommand{}, err
	}
	exam, err := datePatch("expectedExamDate", r.ExpectedExamDate)
	if err != nil {
		return in.UpdateCommand{}, err
	}
	achieved, err := datePatch("achievedDate", r.AchievedDate)
	if err != nil {
		return in.UpdateCommand{}, err
	}
	return in.UpdateCommand{
		Name:             stringPatch(r.Name),
		Category:         stringPatch(r.Category),
		Status:           stringPatch(r.Status),
		StudyStartDate:   start,
		ExpectedExamDate: exam,
		Goal:             stringPatch(r.Goal),
		AchievedDate:     achieved,
		CredlyURL:        stringPatch(r.CredlyURL),
		OfficialURL:      stringPatch(r.OfficialURL),
	}, nil
}

// ── Category requests ───────────────────────────────────────────────────────

type createCategoryRequest struct {
	Path  string `json:"path"`
	Color string `json:"color"`
}

func (r createCategoryRequest) toCommand() in.CreateCategoryCommand {
	return in.CreateCategoryCommand{Path: r.Path, Color: r.Color}
}

type updateCategoryRequest struct {
	Path  *string `json:"path,omitempty"`
	Color *string `json:"color,omitempty"`
}

func (r updateCategoryRequest) toCommand() in.UpdateCategoryCommand {
	return in.UpdateCategoryCommand{Path: r.Path, Color: r.Color}
}

// ── Responses ───────────────────────────────────────────────────────────────

type categoryDTO struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Color string `json:"color"`
	Count *int   `json:"count,omitempty"`
}

func toCategoryDTO(cat domain.Category) categoryDTO {
	return categoryDTO{ID: cat.ID, Path: cat.Path.Display(), Color: cat.Color.String()}
}

type attachmentDTO struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}

type certificationDTO struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Category         categoryDTO     `json:"category"`
	Status           string          `json:"status"`
	StudyStartDate   *string         `json:"studyStartDate"`
	ExpectedExamDate *string         `json:"expectedExamDate"`
	Goal             *string         `json:"goal"`
	AchievedDate     *string         `json:"achievedDate"`
	CredlyURL        *string         `json:"credlyUrl"`
	OfficialURL      *string         `json:"officialUrl"`
	Attachments      []attachmentDTO `json:"attachments"`
	CreatedAt        string          `json:"createdAt"`
	UpdatedAt        string          `json:"updatedAt"`
}

func toCertificationDTO(cert *domain.Certification) certificationDTO {
	attachments := make([]attachmentDTO, 0, 2)
	for _, a := range cert.Attachments() {
		attachments = append(attachments, attachmentDTO{
			ID:          a.ID,
			Kind:        string(a.Kind),
			FileName:    a.FileName,
			ContentType: a.ContentType,
			SizeBytes:   a.SizeBytes,
		})
	}
	return certificationDTO{
		ID:               cert.ID(),
		Name:             cert.Name().String(),
		Category:         toCategoryDTO(cert.Category()),
		Status:           string(cert.Status()),
		StudyStartDate:   formatDate(cert.Period().Start()),
		ExpectedExamDate: formatDate(cert.Period().Exam()),
		Goal:             cert.Goal(),
		AchievedDate:     formatDate(cert.Evidence().AchievedDate()),
		CredlyURL:        cert.Evidence().CredlyURL(),
		OfficialURL:      cert.Evidence().OfficialURL(),
		Attachments:      attachments,
		CreatedAt:        cert.CreatedAt().UTC().Format(time.RFC3339),
		UpdatedAt:        cert.UpdatedAt().UTC().Format(time.RFC3339),
	}
}

type listMetaDTO struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}
