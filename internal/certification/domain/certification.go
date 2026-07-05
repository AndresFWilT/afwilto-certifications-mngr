package domain

import "time"

// Certification is the aggregate root and transactional consistency boundary:
// the certification and its attachments' metadata change together.
type Certification struct {
	id          string
	name        CertificationName
	category    Category
	period      StudyPeriod
	goal        *string
	status      AchievementStatus
	evidence    AchievementEvidence
	attachments []Attachment
	createdAt   time.Time
	updatedAt   time.Time
	deletedAt   *time.Time
}

func NewCertification(id string, name CertificationName, category Category, period StudyPeriod, goal *string, now time.Time) (*Certification, CertificationCreated, error) {
	if err := validateGoal(goal); err != nil {
		return nil, CertificationCreated{}, err
	}
	cert := &Certification{
		id:        id,
		name:      name,
		category:  category,
		period:    period,
		goal:      goal,
		status:    StatusPlanned,
		createdAt: now,
		updatedAt: now,
	}
	return cert, CertificationCreated{ID: id, At: now}, nil
}

// Rehydrate rebuilds an aggregate from persistence, trusting stored values.
func Rehydrate(id string, name CertificationName, category Category, period StudyPeriod, goal *string,
	status AchievementStatus, evidence AchievementEvidence, attachments []Attachment,
	createdAt, updatedAt time.Time, deletedAt *time.Time) *Certification {
	return &Certification{
		id: id, name: name, category: category, period: period, goal: goal,
		status: status, evidence: evidence, attachments: attachments,
		createdAt: createdAt, updatedAt: updatedAt, deletedAt: deletedAt,
	}
}

func (c *Certification) ID() string                    { return c.id }
func (c *Certification) Name() CertificationName       { return c.name }
func (c *Certification) Category() Category            { return c.category }
func (c *Certification) Period() StudyPeriod           { return c.period }
func (c *Certification) Goal() *string                 { return c.goal }
func (c *Certification) Status() AchievementStatus     { return c.status }
func (c *Certification) Evidence() AchievementEvidence { return c.evidence }
func (c *Certification) CreatedAt() time.Time          { return c.createdAt }
func (c *Certification) UpdatedAt() time.Time          { return c.updatedAt }
func (c *Certification) DeletedAt() *time.Time         { return c.deletedAt }

func (c *Certification) Attachments() []Attachment {
	out := make([]Attachment, len(c.attachments))
	copy(out, c.attachments)
	return out
}

func (c *Certification) ensureAlive() error {
	if c.deletedAt != nil {
		return ErrNotFound
	}
	return nil
}

func (c *Certification) touch(now time.Time) { c.updatedAt = now }

func (c *Certification) Rename(name CertificationName, now time.Time) error {
	if err := c.ensureAlive(); err != nil {
		return err
	}
	c.name = name
	c.touch(now)
	return nil
}

func (c *Certification) Recategorize(category Category, now time.Time) error {
	if err := c.ensureAlive(); err != nil {
		return err
	}
	c.category = category
	c.touch(now)
	return nil
}

func (c *Certification) Reschedule(period StudyPeriod, now time.Time) (CertificationRescheduled, error) {
	if err := c.ensureAlive(); err != nil {
		return CertificationRescheduled{}, err
	}
	c.period = period
	c.touch(now)
	return CertificationRescheduled{ID: c.id, At: now}, nil
}

func (c *Certification) DescribeGoal(goal *string, now time.Time) error {
	if err := c.ensureAlive(); err != nil {
		return err
	}
	if err := validateGoal(goal); err != nil {
		return err
	}
	c.goal = goal
	c.touch(now)
	return nil
}

// ChangeStatus allows every transition (FR-2). Transitioning to achieved with
// no achievedDate on record defaults it to the clock's civil date; moving away
// keeps stored evidence as history (BR-EV2).
func (c *Certification) ChangeStatus(status AchievementStatus, now time.Time) error {
	if err := c.ensureAlive(); err != nil {
		return err
	}
	c.status = status
	if status == StatusAchieved && c.evidence.achievedDate == nil {
		day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		c.evidence.achievedDate = &day
	}
	c.touch(now)
	return nil
}

func (c *Certification) ensureEvidenceWritable(field string) error {
	if err := c.ensureAlive(); err != nil {
		return err
	}
	if c.status != StatusAchieved {
		return SingleViolation(field, "not_achieved", field+" is writable only when the certification is achieved")
	}
	return nil
}

func (c *Certification) SetCredlyURL(url *string, now time.Time) error {
	if err := c.ensureEvidenceWritable("credlyUrl"); err != nil {
		return err
	}
	if url != nil {
		if err := ValidateEvidenceURL("credlyUrl", *url); err != nil {
			return err
		}
	}
	c.evidence.credlyURL = url
	c.touch(now)
	return nil
}

func (c *Certification) SetOfficialURL(url *string, now time.Time) error {
	if err := c.ensureEvidenceWritable("officialUrl"); err != nil {
		return err
	}
	if url != nil {
		if err := ValidateEvidenceURL("officialUrl", *url); err != nil {
			return err
		}
	}
	c.evidence.officialURL = url
	c.touch(now)
	return nil
}

func (c *Certification) SetAchievedDate(d *time.Time, now time.Time) error {
	if err := c.ensureEvidenceWritable("achievedDate"); err != nil {
		return err
	}
	c.evidence.achievedDate = d
	c.touch(now)
	return nil
}

// Attach enforces BR-EV1 and the at-most-one-per-kind invariant with replace
// semantics: a new attachment of an existing kind displaces the old one.
func (c *Certification) Attach(a Attachment, now time.Time) error {
	if err := c.ensureEvidenceWritable("file"); err != nil {
		return err
	}
	kept := c.attachments[:0]
	for _, existing := range c.attachments {
		if existing.Kind != a.Kind {
			kept = append(kept, existing)
		}
	}
	c.attachments = append(kept, a)
	c.touch(now)
	return nil
}

func (c *Certification) DetachByID(attachmentID string, now time.Time) error {
	if err := c.ensureAlive(); err != nil {
		return err
	}
	for i, a := range c.attachments {
		if a.ID == attachmentID {
			c.attachments = append(c.attachments[:i], c.attachments[i+1:]...)
			c.touch(now)
			return nil
		}
	}
	return ErrNotFound
}

func (c *Certification) FindAttachment(attachmentID string) (Attachment, bool) {
	for _, a := range c.attachments {
		if a.ID == attachmentID {
			return a, true
		}
	}
	return Attachment{}, false
}

func (c *Certification) SoftDelete(now time.Time) (CertificationDeleted, error) {
	if err := c.ensureAlive(); err != nil {
		return CertificationDeleted{}, err
	}
	c.deletedAt = &now
	c.touch(now)
	return CertificationDeleted{ID: c.id, At: now}, nil
}

func validateGoal(goal *string) error {
	if goal != nil && len([]rune(*goal)) > 2000 {
		return SingleViolation("goal", "max_length", "goal must be at most 2000 characters")
	}
	return nil
}
