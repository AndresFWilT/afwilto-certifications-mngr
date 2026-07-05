package domain

import "strings"

// CategoryPath is a free hierarchical path ("TI / Cloud / AWS"). The display
// form preserves the user's casing; identity and matching use the normalized
// form (trimmed segments, lower-cased, single spaces, '/'-joined).
type CategoryPath struct {
	display    string
	normalized string
}

func NewCategoryPath(raw string) (CategoryPath, error) {
	segments := strings.Split(raw, "/")
	cleaned := make([]string, 0, len(segments))
	for _, s := range segments {
		seg := strings.Join(strings.Fields(s), " ")
		if seg == "" {
			return CategoryPath{}, SingleViolation("category", "segment", "category segments must not be empty")
		}
		cleaned = append(cleaned, seg)
	}
	if len(cleaned) == 0 {
		return CategoryPath{}, SingleViolation("category", "required", "category must not be empty")
	}
	display := strings.Join(cleaned, " / ")
	if len([]rune(display)) > 120 {
		return CategoryPath{}, SingleViolation("category", "max_length", "category path must be at most 120 characters")
	}
	lowered := make([]string, len(cleaned))
	for i, s := range cleaned {
		lowered[i] = strings.ToLower(s)
	}
	return CategoryPath{display: display, normalized: strings.Join(lowered, "/")}, nil
}

func (p CategoryPath) Display() string    { return p.display }
func (p CategoryPath) Normalized() string { return p.normalized }

// Category has identity so filters and colors stay stable across renames; it
// is a shared reference datum, not part of the Certification aggregate.
type Category struct {
	ID   string
	Path CategoryPath
}

type CategoryWithCount struct {
	Category
	Count int
}

// RehydrateCategoryPath trusts persisted display/normalized forms.
func RehydrateCategoryPath(display, normalized string) CategoryPath {
	return CategoryPath{display: display, normalized: normalized}
}
