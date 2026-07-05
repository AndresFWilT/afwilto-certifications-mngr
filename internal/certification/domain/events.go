package domain

import "time"

// Domain events are modeled but not dispatched — no bus exists on the
// platform (module spec §6.6). Aggregate methods return them so future
// consumers (reminders, notifications) get an outbox for free.

type CertificationCreated struct {
	ID string
	At time.Time
}

type CertificationAchieved struct {
	ID string
	At time.Time
}

type CertificationRescheduled struct {
	ID string
	At time.Time
}

type CertificationDeleted struct {
	ID string
	At time.Time
}
