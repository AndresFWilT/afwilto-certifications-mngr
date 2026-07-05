package http

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

// handleUploadAttachment enforces the deterministic upload guarantees: the
// request body is hard-capped (413) and the content type is decided by
// sniffing the first bytes (415), never by trusting the declared header.
func handleUploadAttachment(upload in.UploadAttachment, maxUploadBytes int64) http.HandlerFunc {
	const formOverhead = 64 * 1024
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+formOverhead)
		if err := r.ParseMultipartForm(maxUploadBytes + formOverhead); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				payloadTooLarge(w)
				return
			}
			translateError(w, domain.SingleViolation("body", "multipart", "request must be multipart/form-data with kind and file"))
			return
		}
		defer func() { _ = r.MultipartForm.RemoveAll() }()

		kind := r.FormValue("kind")
		file, header, err := r.FormFile("file")
		if err != nil {
			translateError(w, domain.SingleViolation("file", "required", "a file part is required"))
			return
		}
		defer file.Close()

		if header.Size > maxUploadBytes {
			payloadTooLarge(w)
			return
		}
		sniffed, reader, err := sniffContentType(file)
		if err != nil {
			translateError(w, err)
			return
		}
		if !domain.AllowedContentType(sniffed) {
			writeError(w, http.StatusUnsupportedMediaType, errorBody{
				Code:    "unsupported_media_type",
				Message: "file content must be application/pdf, image/png or image/jpeg",
			})
			return
		}

		attachment, err := upload.Execute(r.Context(), in.UploadAttachmentCommand{
			CertificationID: r.PathValue("id"),
			Kind:            kind,
			FileName:        header.Filename,
			ContentType:     sniffed,
			SizeBytes:       header.Size,
			Content:         reader,
		})
		if err != nil {
			translateError(w, err)
			return
		}
		writeData(w, http.StatusCreated, attachmentDTO{
			ID:          attachment.ID,
			Kind:        string(attachment.Kind),
			FileName:    attachment.FileName,
			ContentType: attachment.ContentType,
			SizeBytes:   attachment.SizeBytes,
		})
	}
}

func sniffContentType(file multipart.File) (string, io.Reader, error) {
	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", nil, err
	}
	head = head[:n]
	sniffed := http.DetectContentType(head)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", nil, err
	}
	return sniffed, file, nil
}

func payloadTooLarge(w http.ResponseWriter) {
	writeError(w, http.StatusRequestEntityTooLarge, errorBody{
		Code:    "payload_too_large",
		Message: "file exceeds the 10 MB limit",
	})
}

func handleDownloadAttachment(download in.DownloadAttachment) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		attachment, content, err := download.Execute(r.Context(), r.PathValue("id"))
		if err != nil {
			translateError(w, err)
			return
		}
		defer content.Close()
		w.Header().Set("Content-Type", attachment.ContentType)
		w.Header().Set("Content-Length", strconv.FormatInt(attachment.SizeBytes, 10))
		w.Header().Set("Content-Disposition", `attachment; filename="`+attachment.FileName+`"`)
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, content)
	}
}

func handleDeleteAttachment(remove in.DeleteAttachment) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := remove.Execute(r.Context(), r.PathValue("id")); err != nil {
			translateError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
