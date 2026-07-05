package domain

import "time"

type AttachmentKind string

const (
	KindCertificate AttachmentKind = "certificate"
	KindDiploma     AttachmentKind = "diploma"
)

func ParseAttachmentKind(raw string) (AttachmentKind, error) {
	switch AttachmentKind(raw) {
	case KindCertificate, KindDiploma:
		return AttachmentKind(raw), nil
	default:
		return "", SingleViolation("kind", "enum", "kind must be certificate or diploma")
	}
}

const MaxAttachmentBytes = 10 * 1024 * 1024

var allowedContentTypes = map[string]bool{
	"application/pdf": true,
	"image/png":       true,
	"image/jpeg":      true,
}

func AllowedContentType(contentType string) bool {
	return allowedContentTypes[contentType]
}

// Attachment has identity (its download URL) but no life outside its
// certification — an entity inside the aggregate.
type Attachment struct {
	ID          string
	Kind        AttachmentKind
	FileName    string
	ContentType string
	SizeBytes   int64
	StorageKey  string
	CreatedAt   time.Time
}

func NewAttachment(id string, kind AttachmentKind, fileName, contentType string, sizeBytes int64, storageKey string, now time.Time) (Attachment, error) {
	if !AllowedContentType(contentType) {
		return Attachment{}, SingleViolation("file", "content_type", "file must be application/pdf, image/png or image/jpeg")
	}
	if sizeBytes <= 0 || sizeBytes > MaxAttachmentBytes {
		return Attachment{}, SingleViolation("file", "size", "file must be between 1 byte and 10 MB")
	}
	return Attachment{
		ID:          id,
		Kind:        kind,
		FileName:    fileName,
		ContentType: contentType,
		SizeBytes:   sizeBytes,
		StorageKey:  storageKey,
		CreatedAt:   now,
	}, nil
}
