package domain

import "strings"

type CertificationName struct {
	value string
}

func NewCertificationName(raw string) (CertificationName, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return CertificationName{}, SingleViolation("name", "required", "name must not be empty")
	}
	if len([]rune(trimmed)) > 200 {
		return CertificationName{}, SingleViolation("name", "max_length", "name must be at most 200 characters")
	}
	return CertificationName{value: trimmed}, nil
}

func (n CertificationName) String() string { return n.value }
