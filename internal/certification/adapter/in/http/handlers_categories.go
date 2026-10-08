package http

import (
	"encoding/json"
	"net/http"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

func handleCreateCategory(create in.CreateCategory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			malformedBody(w)
			return
		}
		cat, err := create.Execute(r.Context(), req.toCommand())
		if err != nil {
			translateError(w, err)
			return
		}
		count := 0
		dto := toCategoryDTO(cat)
		dto.Count = &count
		writeData(w, http.StatusCreated, dto)
	}
}

func handleUpdateCategory(update in.UpdateCategory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req updateCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			malformedBody(w)
			return
		}
		if req.Path == nil && req.Color == nil {
			translateError(w, domain.SingleViolation("body", "required", "at least one of path or color must be provided"))
			return
		}
		cat, err := update.Execute(r.Context(), r.PathValue("id"), req.toCommand())
		if err != nil {
			translateError(w, err)
			return
		}
		writeData(w, http.StatusOK, toCategoryDTO(cat))
	}
}

func handleDeleteCategory(del in.DeleteCategory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := del.Execute(r.Context(), r.PathValue("id")); err != nil {
			translateError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
