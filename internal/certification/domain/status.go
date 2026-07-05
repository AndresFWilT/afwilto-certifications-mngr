package domain

type AchievementStatus string

const (
	StatusPlanned    AchievementStatus = "planned"
	StatusInProgress AchievementStatus = "in_progress"
	StatusAchieved   AchievementStatus = "achieved"
)

// ParseStatus accepts the closed set; every transition between valid statuses
// is legal (FR-2), so no transition table exists.
func ParseStatus(raw string) (AchievementStatus, error) {
	switch AchievementStatus(raw) {
	case StatusPlanned, StatusInProgress, StatusAchieved:
		return AchievementStatus(raw), nil
	default:
		return "", SingleViolation("status", "enum", "status must be one of planned, in_progress, achieved")
	}
}
