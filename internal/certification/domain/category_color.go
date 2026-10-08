package domain

import "strings"

// CategoryColor is a persisted, user-chosen 6-digit hex color. Identity and
// display both use the normalized uppercase form.
type CategoryColor struct {
	hex string
}

func NewCategoryColor(raw string) (CategoryColor, error) {
	trimmed := strings.TrimSpace(raw)
	if !isHexColor(trimmed) {
		return CategoryColor{}, SingleViolation("color", "format", "color must be a 6-digit hex value like #22D3EE")
	}
	return CategoryColor{hex: strings.ToUpper(trimmed)}, nil
}

func isHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, r := range s[1:] {
		if (r < '0' || r > '9') && (r < 'A' || r > 'F') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// RehydrateCategoryColor trusts persisted data, mirrors RehydrateCategoryPath.
func RehydrateCategoryColor(hex string) CategoryColor {
	return CategoryColor{hex: hex}
}

func (c CategoryColor) String() string { return c.hex }
