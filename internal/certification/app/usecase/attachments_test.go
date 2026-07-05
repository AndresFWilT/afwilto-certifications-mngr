package usecase

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
	"github.com/afwilto/certifications-mngr/internal/certification/testsupport"
)

func achievedCert(t *testing.T, e env) *domain.Certification {
	t.Helper()
	cert := createFull(t, e)
	svc := NewUpdateService(e.certs, e.categories, e.clock)
	updated, err := svc.Execute(context.Background(), cert.ID(), in.UpdateCommand{Status: in.PatchOf("achieved")})
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

func uploadCmd(certID, kind string) in.UploadAttachmentCommand {
	return in.UploadAttachmentCommand{
		CertificationID: certID,
		Kind:            kind,
		FileName:        "diploma.pdf",
		ContentType:     "application/pdf",
		SizeBytes:       2048,
		Content:         strings.NewReader("pdf-bytes"),
	}
}

func TestUploadOnAchieved(t *testing.T) {
	e := newEnv()
	storage := testsupport.NewFakeFileStorage()
	cert := achievedCert(t, e)
	svc := NewUploadAttachmentService(e.certs, storage, e.clock, e.ids)

	att, err := svc.Execute(context.Background(), uploadCmd(cert.ID(), "certificate"))
	if err != nil {
		t.Fatal(err)
	}
	if att.Kind != domain.KindCertificate || att.FileName != "diploma.pdf" {
		t.Errorf("attachment = %+v", att)
	}
	if string(storage.Blobs[att.StorageKey]) != "pdf-bytes" {
		t.Error("bytes not stored")
	}
}

func TestUploadRejectedOnPlanned(t *testing.T) {
	e := newEnv()
	storage := testsupport.NewFakeFileStorage()
	cert := createFull(t, e)
	svc := NewUploadAttachmentService(e.certs, storage, e.clock, e.ids)

	_, err := svc.Execute(context.Background(), uploadCmd(cert.ID(), "certificate"))
	wantViolationOn(t, err, "file")
	if len(storage.Blobs) != 0 {
		t.Error("blob stored despite BR-EV1 rejection")
	}
}

func TestUploadReplaceDeletesDisplacedBlob(t *testing.T) {
	e := newEnv()
	storage := testsupport.NewFakeFileStorage()
	cert := achievedCert(t, e)
	svc := NewUploadAttachmentService(e.certs, storage, e.clock, e.ids)

	first, err := svc.Execute(context.Background(), uploadCmd(cert.ID(), "diploma"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Execute(context.Background(), uploadCmd(cert.ID(), "diploma"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := storage.Blobs[first.StorageKey]; ok {
		t.Error("displaced blob not deleted")
	}
	if _, ok := storage.Blobs[second.StorageKey]; !ok {
		t.Error("replacement blob missing")
	}
	stored, _ := e.certs.FindByID(context.Background(), cert.ID())
	if len(stored.Attachments()) != 1 {
		t.Errorf("attachments = %d, want 1", len(stored.Attachments()))
	}
}

func TestDownloadStreamsMetadataAndBytes(t *testing.T) {
	e := newEnv()
	storage := testsupport.NewFakeFileStorage()
	cert := achievedCert(t, e)
	up := NewUploadAttachmentService(e.certs, storage, e.clock, e.ids)
	att, err := up.Execute(context.Background(), uploadCmd(cert.ID(), "diploma"))
	if err != nil {
		t.Fatal(err)
	}

	down := NewDownloadAttachmentService(e.certs, storage)
	meta, content, err := down.Execute(context.Background(), att.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer content.Close()
	bytes, _ := io.ReadAll(content)
	if meta.FileName != "diploma.pdf" || string(bytes) != "pdf-bytes" {
		t.Errorf("download = %+v / %q", meta, bytes)
	}
}

func TestDownloadUnknownAttachment404(t *testing.T) {
	e := newEnv()
	down := NewDownloadAttachmentService(e.certs, testsupport.NewFakeFileStorage())
	_, _, err := down.Execute(context.Background(), "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteAttachmentDetachesAndRemovesBlob(t *testing.T) {
	e := newEnv()
	storage := testsupport.NewFakeFileStorage()
	cert := achievedCert(t, e)
	up := NewUploadAttachmentService(e.certs, storage, e.clock, e.ids)
	att, err := up.Execute(context.Background(), uploadCmd(cert.ID(), "certificate"))
	if err != nil {
		t.Fatal(err)
	}

	del := NewDeleteAttachmentService(e.certs, storage, e.clock)
	if err := del.Execute(context.Background(), att.ID); err != nil {
		t.Fatal(err)
	}
	if len(storage.Blobs) != 0 {
		t.Error("blob survived attachment delete")
	}
	if err := del.Execute(context.Background(), att.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("second delete = %v, want ErrNotFound", err)
	}
}
