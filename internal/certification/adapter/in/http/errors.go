package http

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

// translateError is the single point where domain errors become HTTP status
// codes; codes never travel further inward.
func translateError(w http.ResponseWriter, err error) {
	var validation *domain.ValidationError
	switch {
	case errors.As(err, &validation):
		details := make([]errorDetail, len(validation.Violations))
		for i, v := range validation.Violations {
			details[i] = errorDetail{Field: v.Field, Rule: v.Rule, Message: v.Message}
		}
		writeError(w, http.StatusUnprocessableEntity, errorBody{
			Code:    "validation_failed",
			Message: "validation failed",
			Details: details,
		})
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, errorBody{
			Code:    "not_found",
			Message: "resource not found",
		})
	default:
		slog.Error("internal error", "err", err)
		writeError(w, http.StatusInternalServerError, errorBody{
			Code:    "internal",
			Message: "internal server error",
		})
	}
}

func malformedBody(w http.ResponseWriter) {
	writeError(w, http.StatusUnprocessableEntity, errorBody{
		Code:    "validation_failed",
		Message: "validation failed",
		Details: []errorDetail{{Field: "body", Rule: "json", Message: "request body must be valid JSON"}},
	})
}
