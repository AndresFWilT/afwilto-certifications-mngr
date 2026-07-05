package usecase

import (
	"context"
	"io"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/app/port/out"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

type uploadAttachmentService struct {
	certs   out.CertificationRepository
	storage out.FileStorage
	clock   out.Clock
	ids     out.IDGenerator
}

func NewUploadAttachmentService(certs out.CertificationRepository, storage out.FileStorage, clock out.Clock, ids out.IDGenerator) in.UploadAttachment {
	return &uploadAttachmentService{certs: certs, storage: storage, clock: clock, ids: ids}
}

func (s *uploadAttachmentService) Execute(ctx context.Context, cmd in.UploadAttachmentCommand) (domain.Attachment, error) {
	cert, err := s.certs.FindByID(ctx, cmd.CertificationID)
	if err != nil {
		return domain.Attachment{}, err
	}
	kind, err := domain.ParseAttachmentKind(cmd.Kind)
	if err != nil {
		return domain.Attachment{}, err
	}

	id := s.ids.NewID()
	key := "certifications/" + cert.ID() + "/" + id
	attachment, err := domain.NewAttachment(id, kind, cmd.FileName, cmd.ContentType, cmd.SizeBytes, key, s.clock.Now())
	if err != nil {
		return domain.Attachment{}, err
	}

	// Remember the displaced blob (replace semantics) before the aggregate
	// swaps it out; its bytes are removed only after the metadata commits.
	var displacedKey string
	for _, existing := range cert.Attachments() {
		if existing.Kind == kind {
			displacedKey = existing.StorageKey
		}
	}

	if err := cert.Attach(attachment, s.clock.Now()); err != nil {
		return domain.Attachment{}, err
	}
	if err := s.storage.Put(ctx, key, cmd.Content); err != nil {
		return domain.Attachment{}, err
	}
	if err := s.certs.Save(ctx, cert); err != nil {
		// Roll the orphaned blob back so failed saves leave no litter.
		_ = s.storage.Delete(ctx, key)
		return domain.Attachment{}, err
	}
	if displacedKey != "" {
		_ = s.storage.Delete(ctx, displacedKey)
	}
	return attachment, nil
}

type downloadAttachmentService struct {
	certs   out.CertificationRepository
	storage out.FileStorage
}

func NewDownloadAttachmentService(certs out.CertificationRepository, storage out.FileStorage) in.DownloadAttachment {
	return &downloadAttachmentService{certs: certs, storage: storage}
}

func (s *downloadAttachmentService) Execute(ctx context.Context, attachmentID string) (domain.Attachment, io.ReadCloser, error) {
	cert, err := s.certs.FindByAttachmentID(ctx, attachmentID)
	if err != nil {
		return domain.Attachment{}, nil, err
	}
	attachment, ok := cert.FindAttachment(attachmentID)
	if !ok {
		return domain.Attachment{}, nil, domain.ErrNotFound
	}
	content, err := s.storage.Get(ctx, attachment.StorageKey)
	if err != nil {
		return domain.Attachment{}, nil, err
	}
	return attachment, content, nil
}

type deleteAttachmentService struct {
	certs   out.CertificationRepository
	storage out.FileStorage
	clock   out.Clock
}

func NewDeleteAttachmentService(certs out.CertificationRepository, storage out.FileStorage, clock out.Clock) in.DeleteAttachment {
	return &deleteAttachmentService{certs: certs, storage: storage, clock: clock}
}

func (s *deleteAttachmentService) Execute(ctx context.Context, attachmentID string) error {
	cert, err := s.certs.FindByAttachmentID(ctx, attachmentID)
	if err != nil {
		return err
	}
	attachment, ok := cert.FindAttachment(attachmentID)
	if !ok {
		return domain.ErrNotFound
	}
	if err := cert.DetachByID(attachmentID, s.clock.Now()); err != nil {
		return err
	}
	if err := s.certs.Save(ctx, cert); err != nil {
		return err
	}
	_ = s.storage.Delete(ctx, attachment.StorageKey)
	return nil
}
