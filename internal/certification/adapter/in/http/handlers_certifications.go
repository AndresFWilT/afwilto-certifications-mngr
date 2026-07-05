package http

import (
	"encoding/json"
	"net/http"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
)

func handleCreateCertification(create in.CreateCertification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			malformedBody(w)
			return
		}
		cmd, err := req.toCommand()
		if err != nil {
			translateError(w, err)
			return
		}
		cert, err := create.Execute(r.Context(), cmd)
		if err != nil {
			translateError(w, err)
			return
		}
		w.Header().Set("Location", "/api/v1/certifications/"+cert.ID())
		writeData(w, http.StatusCreated, toCertificationDTO(cert))
	}
}

func handleGetCertification(get in.GetCertification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cert, err := get.Execute(r.Context(), r.PathValue("id"))
		if err != nil {
			translateError(w, err)
			return
		}
		writeData(w, http.StatusOK, toCertificationDTO(cert))
	}
}

func handleUpdateCertification(update in.UpdateCertification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req updateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			malformedBody(w)
			return
		}
		cmd, err := req.toCommand()
		if err != nil {
			translateError(w, err)
			return
		}
		cert, err := update.Execute(r.Context(), r.PathValue("id"), cmd)
		if err != nil {
			translateError(w, err)
			return
		}
		writeData(w, http.StatusOK, toCertificationDTO(cert))
	}
}

func handleDeleteCertification(del in.DeleteCertification) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := del.Execute(r.Context(), r.PathValue("id")); err != nil {
			translateError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
