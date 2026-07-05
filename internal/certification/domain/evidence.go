package domain

import (
	"strings"
	"time"
)

// AchievementEvidence is an immutable snapshot of achievement proof. BR-EV1
// (writable only when achieved) is enforced by the aggregate, not here; BR-EV2
// (kept as history after a regression) follows from never clearing it on
// status change.
type AchievementEvidence struct {
	credlyURL    *string
	officialURL  *string
	achievedDate *time.Time
}

func (e AchievementEvidence) CredlyURL() *string       { return e.credlyURL }
func (e AchievementEvidence) OfficialURL() *string     { return e.officialURL }
func (e AchievementEvidence) AchievedDate() *time.Time { return e.achievedDate }

// RehydrateEvidence trusts persisted values.
func RehydrateEvidence(credlyURL, officialURL *string, achievedDate *time.Time) AchievementEvidence {
	return AchievementEvidence{credlyURL: credlyURL, officialURL: officialURL, achievedDate: achievedDate}
}

func ValidateEvidenceURL(field, raw string) error {
	if !strings.HasPrefix(raw, "https://") || len(raw) <= len("https://") {
		return SingleViolation(field, "https_url", field+" must be an https URL")
	}
	return nil
}
