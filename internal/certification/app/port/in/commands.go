package in

import (
	"context"
	"io"
	"time"

	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

// ── Create ──────────────────────────────────────────────────────────────────

type CreateCommand struct {
	Name             string
	Category         string
	StudyStartDate   *time.Time
	ExpectedExamDate *time.Time
	Goal             *string
}

type CreateCertification interface {
	Execute(ctx context.Context, cmd CreateCommand) (*domain.Certification, error)
}

// ── Get ─────────────────────────────────────────────────────────────────────

type GetCertification interface {
	Execute(ctx context.Context, id string) (*domain.Certification, error)
}

// ── Update ──────────────────────────────────────────────────────────────────

// Patch distinguishes "absent" (Set=false, leave unchanged) from "null"
// (Set=true, Value=nil, clear) — JSON merge-patch semantics.
type Patch[T any] struct {
	Set   bool
	Value *T
}

func PatchOf[T any](v T) Patch[T] { return Patch[T]{Set: true, Value: &v} }
func PatchNull[T any]() Patch[T]  { return Patch[T]{Set: true} }

type UpdateCommand struct {
	Name             Patch[string]
	Category         Patch[string]
	Status           Patch[string]
	StudyStartDate   Patch[time.Time]
	ExpectedExamDate Patch[time.Time]
	Goal             Patch[string]
	AchievedDate     Patch[time.Time]
	CredlyURL        Patch[string]
	OfficialURL      Patch[string]
}

type UpdateCertification interface {
	Execute(ctx context.Context, id string, cmd UpdateCommand) (*domain.Certification, error)
}

// ── Delete ──────────────────────────────────────────────────────────────────

type DeleteCertification interface {
	Execute(ctx context.Context, id string) error
}

// ── Attachments ─────────────────────────────────────────────────────────────

type UploadAttachmentCommand struct {
	CertificationID string
	Kind            string
	FileName        string
	ContentType     string // sniffed by the adapter, never the declared header
	SizeBytes       int64
	Content         io.Reader
}

type UploadAttachment interface {
	Execute(ctx context.Context, cmd UploadAttachmentCommand) (domain.Attachment, error)
}

type DownloadAttachment interface {
	// Execute returns the metadata and an open stream the caller must close.
	Execute(ctx context.Context, attachmentID string) (domain.Attachment, io.ReadCloser, error)
}

type DeleteAttachment interface {
	Execute(ctx context.Context, attachmentID string) error
}

// ── List ────────────────────────────────────────────────────────────────────

type ListCertifications interface {
	Execute(ctx context.Context, query domain.ListQuery) (domain.Page, error)
}

type ListCategories interface {
	Execute(ctx context.Context) ([]domain.CategoryWithCount, error)
}
