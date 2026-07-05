package domain

import "time"

// StudyPeriod owns the date-order invariant: two optional civil dates with
// start ≤ exam whenever both are present.
type StudyPeriod struct {
	start *time.Time
	exam  *time.Time
}

func NewStudyPeriod(start, exam *time.Time) (StudyPeriod, error) {
	if start != nil && exam != nil && exam.Before(*start) {
		return StudyPeriod{}, SingleViolation(
			"expectedExamDate", "date_order",
			"expected exam date must be on or after study start date")
	}
	return StudyPeriod{start: start, exam: exam}, nil
}

func (p StudyPeriod) Start() *time.Time { return p.start }
func (p StudyPeriod) Exam() *time.Time  { return p.exam }

// IntersectsYear reports whether the [start, exam] interval — or the single
// known date — touches the given year. Dateless periods never intersect.
func (p StudyPeriod) IntersectsYear(year int) bool {
	if p.start == nil && p.exam == nil {
		return false
	}
	from := p.start
	if from == nil {
		from = p.exam
	}
	to := p.exam
	if to == nil {
		to = p.start
	}
	yearStart := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	yearEnd := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
	return !from.After(yearEnd) && !to.Before(yearStart)
}
