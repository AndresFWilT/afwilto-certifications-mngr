package http

import (
	"net/http"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
)

// RouterDeps carries everything the HTTP adapter needs, injected from the
// composition root.
type RouterDeps struct {
	Pinger       Pinger
	AllowOrigins []string

	Create         in.CreateCertification
	Get            in.GetCertification
	Update         in.UpdateCertification
	Delete         in.DeleteCertification
	List           in.ListCertifications
	ListCategories in.ListCategories

	Upload           in.UploadAttachment
	Download         in.DownloadAttachment
	RemoveAttachment in.DeleteAttachment
	MaxUploadBytes   int64
}

func NewRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", handleHealthz())
	mux.HandleFunc("GET /readyz", handleReadyz(deps.Pinger))

	mux.HandleFunc("POST /api/v1/certifications", handleCreateCertification(deps.Create))
	mux.HandleFunc("GET /api/v1/certifications", handleListCertifications(deps.List))
	mux.HandleFunc("GET /api/v1/certifications/{id}", handleGetCertification(deps.Get))
	mux.HandleFunc("PATCH /api/v1/certifications/{id}", handleUpdateCertification(deps.Update))
	mux.HandleFunc("DELETE /api/v1/certifications/{id}", handleDeleteCertification(deps.Delete))
	mux.HandleFunc("GET /api/v1/categories", handleListCategories(deps.ListCategories))
	mux.HandleFunc("POST /api/v1/certifications/{id}/attachments", handleUploadAttachment(deps.Upload, deps.MaxUploadBytes))
	mux.HandleFunc("GET /api/v1/attachments/{id}", handleDownloadAttachment(deps.Download))
	mux.HandleFunc("DELETE /api/v1/attachments/{id}", handleDeleteAttachment(deps.RemoveAttachment))

	var handler http.Handler = mux
	handler = corsMiddleware(deps.AllowOrigins, handler)
	handler = recoverMiddleware(handler)
	handler = requestLogMiddleware(handler)
	return handler
}
