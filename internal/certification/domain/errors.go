package domain

import (
	"errors"
	"strings"
)

// ErrNotFound covers unknown ids and soft-deleted aggregates alike — a
// deleted certification is invisible to every operation.
var ErrNotFound = errors.New("not found")

type Violation struct {
	Field   string
	Rule    string
	Message string
}

// ValidationError carries field-level violations; the HTTP adapter translates
// it to the 422 error model. Both shape and business-rule failures share it.
type ValidationError struct {
	Violations []Violation
}

func (e *ValidationError) Error() string {
	msgs := make([]string, len(e.Violations))
	for i, v := range e.Violations {
		msgs[i] = v.Field + ": " + v.Message
	}
	return "validation failed: " + strings.Join(msgs, "; ")
}

func NewValidation(violations ...Violation) *ValidationError {
	return &ValidationError{Violations: violations}
}

func SingleViolation(field, rule, message string) *ValidationError {
	return NewValidation(Violation{Field: field, Rule: rule, Message: message})
}
