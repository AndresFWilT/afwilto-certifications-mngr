package http

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var pdfBytes = []byte("%PDF-1.4 fake but sniffable pdf content")
var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)

func multipartBody(t *testing.T, kind, filename string, content []byte) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("kind", kind); err != nil {
		t.Fatal(err)
	}
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	w.Close()
	return &buf, w.FormDataContentType()
}

func (e apiEnv) upload(t *testing.T, certID, kind, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, contentType := multipartBody(t, kind, filename, content)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/certifications/"+certID+"/attachments", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func achievedCertID(t *testing.T, e apiEnv, name string) string {
	t.Helper()
	id := createCert(t, e, name)
	rec := e.do(t, http.MethodPatch, "/api/v1/certifications/"+id, `{"status":"achieved"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("achieve failed: %d", rec.Code)
	}
	return id
}

func TestAPIUploadCertificate201(t *testing.T) {
	e := newAPIEnv()
	id := achievedCertID(t, e, "AWS SAA")

	rec := e.upload(t, id, "certificate", "cert.pdf", pdfBytes)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeData(t, rec)
	if data["kind"] != "certificate" || data["fileName"] != "cert.pdf" || data["contentType"] != "application/pdf" {
		t.Errorf("data = %v", data)
	}
	assertConformsToContract(t, http.MethodPost, "/api/v1/certifications/"+id+"/attachments", rec)
}

func TestAPIUploadRejectedOnPlanned422(t *testing.T) {
	e := newAPIEnv()
	id := createCert(t, e, "Planned Cert")
	rec := e.upload(t, id, "certificate", "cert.pdf", pdfBytes)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (BR-EV1)", rec.Code)
	}
}

func TestAPIUploadSniffsContentType415(t *testing.T) {
	e := newAPIEnv()
	id := achievedCertID(t, e, "AWS SAA")

	// Declared as png by filename, but the bytes are an executable stub.
	exe := append([]byte("MZ\x90\x00\x03"), bytes.Repeat([]byte{0x90}, 64)...)
	rec := e.upload(t, id, "certificate", "innocent.png", exe)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "unsupported_media_type") {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestAPIUploadOversize413(t *testing.T) {
	e := newAPIEnv()
	id := achievedCertID(t, e, "AWS SAA")

	big := append([]byte("%PDF-1.4 "), bytes.Repeat([]byte{'a'}, maxTestUploadBytes+1)...)
	rec := e.upload(t, id, "certificate", "big.pdf", big)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "payload_too_large") {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestAPIUploadBadKind422(t *testing.T) {
	e := newAPIEnv()
	id := achievedCertID(t, e, "AWS SAA")
	rec := e.upload(t, id, "selfie", "cert.pdf", pdfBytes)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
}

func TestAPIUploadReplaceAndDownload(t *testing.T) {
	e := newAPIEnv()
	id := achievedCertID(t, e, "AWS SAA")

	first := e.upload(t, id, "diploma", "old.pdf", pdfBytes)
	if first.Code != http.StatusCreated {
		t.Fatal(first.Body.String())
	}
	second := e.upload(t, id, "diploma", "new.png", pngBytes)
	if second.Code != http.StatusCreated {
		t.Fatal(second.Body.String())
	}
	newID := decodeData(t, second)["id"].(string)

	// The certification exposes exactly one diploma — the replacement.
	rec := e.do(t, http.MethodGet, "/api/v1/certifications/"+id, "")
	body := rec.Body.String()
	if strings.Contains(body, "old.pdf") || !strings.Contains(body, "new.png") {
		t.Errorf("detail = %s", body)
	}

	dl := e.do(t, http.MethodGet, "/api/v1/attachments/"+newID, "")
	if dl.Code != http.StatusOK {
		t.Fatalf("download = %d", dl.Code)
	}
	if got := dl.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="new.png"`) {
		t.Errorf("Content-Disposition = %q", got)
	}
	if got := dl.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q", got)
	}
	if !bytes.Equal(dl.Body.Bytes(), pngBytes) {
		t.Error("downloaded bytes differ from upload")
	}
}

func TestAPIDeleteAttachment(t *testing.T) {
	e := newAPIEnv()
	id := achievedCertID(t, e, "AWS SAA")
	rec := e.upload(t, id, "certificate", "cert.pdf", pdfBytes)
	attID := decodeData(t, rec)["id"].(string)

	del := e.do(t, http.MethodDelete, "/api/v1/attachments/"+attID, "")
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", del.Code)
	}
	if again := e.do(t, http.MethodDelete, "/api/v1/attachments/"+attID, ""); again.Code != http.StatusNotFound {
		t.Fatalf("double delete = %d, want 404", again.Code)
	}
	if dl := e.do(t, http.MethodGet, "/api/v1/attachments/"+attID, ""); dl.Code != http.StatusNotFound {
		t.Fatalf("download after delete = %d, want 404", dl.Code)
	}
}

func TestAPIAttachmentOfDeletedCertNotDownloadable(t *testing.T) {
	e := newAPIEnv()
	id := achievedCertID(t, e, "AWS SAA")
	rec := e.upload(t, id, "certificate", "cert.pdf", pdfBytes)
	attID := decodeData(t, rec)["id"].(string)

	e.do(t, http.MethodDelete, "/api/v1/certifications/"+id, "")
	if dl := e.do(t, http.MethodGet, "/api/v1/attachments/"+attID, ""); dl.Code != http.StatusNotFound {
		t.Fatalf("download on deleted cert = %d, want 404", dl.Code)
	}
}
