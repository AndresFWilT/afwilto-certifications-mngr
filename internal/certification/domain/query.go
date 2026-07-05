package domain

type SortField string

const (
	SortName             SortField = "name"
	SortStudyStartDate   SortField = "studyStartDate"
	SortExpectedExamDate SortField = "expectedExamDate"
	SortAchievedDate     SortField = "achievedDate"
	SortCreatedAt        SortField = "createdAt"
)

func ParseSortField(raw string) (SortField, error) {
	switch SortField(raw) {
	case SortName, SortStudyStartDate, SortExpectedExamDate, SortAchievedDate, SortCreatedAt:
		return SortField(raw), nil
	default:
		return "", SingleViolation("sort", "enum",
			"sort must be one of name, studyStartDate, expectedExamDate, achievedDate, createdAt")
	}
}

type SortOrder string

const (
	OrderAsc  SortOrder = "asc"
	OrderDesc SortOrder = "desc"
)

func ParseSortOrder(raw string) (SortOrder, error) {
	switch SortOrder(raw) {
	case OrderAsc, OrderDesc:
		return SortOrder(raw), nil
	default:
		return "", SingleViolation("order", "enum", "order must be asc or desc")
	}
}

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// ListQuery composes every FR-4 filter with AND semantics. CategoryPrefix is
// the normalized path form; Year 0 means "no year filter".
type ListQuery struct {
	Page           int
	PageSize       int
	Search         string
	Statuses       []AchievementStatus
	CategoryPrefix string
	Year           int
	Sort           SortField
	Order          SortOrder
}

func (q ListQuery) WithDefaults() ListQuery {
	if q.Page == 0 {
		q.Page = 1
	}
	if q.PageSize == 0 {
		q.PageSize = DefaultPageSize
	}
	if q.Sort == "" {
		q.Sort = SortCreatedAt
	}
	if q.Order == "" {
		q.Order = OrderDesc
	}
	return q
}

func (q ListQuery) Validate() error {
	var violations []Violation
	if q.Page < 1 {
		violations = append(violations, Violation{Field: "page", Rule: "min", Message: "page must be at least 1"})
	}
	if q.PageSize < 1 || q.PageSize > MaxPageSize {
		violations = append(violations, Violation{Field: "pageSize", Rule: "range", Message: "pageSize must be between 1 and 100"})
	}
	if len(violations) > 0 {
		return NewValidation(violations...)
	}
	return nil
}

type Page struct {
	Items      []*Certification
	TotalItems int
}
