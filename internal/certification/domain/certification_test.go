package domain

import (
	"errors"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)

func newTestCert(t *testing.T) *Certification {
	t.Helper()
	name, err := NewCertificationName("AWS Solutions Architect Associate")
	if err != nil {
		t.Fatal(err)
	}
	path, err := NewCategoryPath("TI / Cloud / AWS")
	if err != nil {
		t.Fatal(err)
	}
	period, err := NewStudyPeriod(datePtr("2026-08-01"), datePtr("2026-11-15"))
	if err != nil {
		t.Fatal(err)
	}
	goal := "Need this before applying to Senior Cloud Architect."
	cert, _, err := NewCertification("id-1", name, Category{ID: "cat-1", Path: path}, period, &goal, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func TestNewCertificationDefaults(t *testing.T) {
	cert := newTestCert(t)
	if cert.Status() != StatusPlanned {
		t.Errorf("status = %q, want planned", cert.Status())
	}
	if !cert.CreatedAt().Equal(fixedNow) || !cert.UpdatedAt().Equal(fixedNow) {
		t.Error("timestamps not set from clock")
	}
	if cert.DeletedAt() != nil {
		t.Error("new certification must not be deleted")
	}
	if len(cert.Attachments()) != 0 {
		t.Error("new certification must have no attachments")
	}
}

func TestNewCertificationEmitsCreatedEvent(t *testing.T) {
	name, _ := NewCertificationName("CKA")
	path, _ := NewCategoryPath("TI / Cloud")
	period, _ := NewStudyPeriod(nil, nil)
	_, event, err := NewCertification("id-2", name, Category{ID: "c", Path: path}, period, nil, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if event.ID != "id-2" {
		t.Errorf("event.ID = %q", event.ID)
	}
}

func TestNewCertificationRejectsLongGoal(t *testing.T) {
	name, _ := NewCertificationName("CKA")
	path, _ := NewCategoryPath("TI")
	period, _ := NewStudyPeriod(nil, nil)
	long := string(make([]byte, 2001))
	_, _, err := NewCertification("id", name, Category{ID: "c", Path: path}, period, &long, fixedNow)
	wantViolation(t, err, "goal")
}

// ── Status lifecycle + evidence (BR-EV1 / BR-EV2) ──────────────────────────

func TestAllStatusTransitionsAllowed(t *testing.T) {
	cert := newTestCert(t)
	for _, s := range []AchievementStatus{StatusInProgress, StatusAchieved, StatusPlanned, StatusAchieved, StatusInProgress} {
		if err := cert.ChangeStatus(s, fixedNow); err != nil {
			t.Fatalf("transition to %q rejected: %v", s, err)
		}
	}
}

func TestAchieveDefaultsAchievedDate(t *testing.T) {
	cert := newTestCert(t)
	if err := cert.ChangeStatus(StatusAchieved, fixedNow); err != nil {
		t.Fatal(err)
	}
	got := cert.Evidence().AchievedDate()
	if got == nil {
		t.Fatal("achievedDate not defaulted on transition to achieved")
	}
	if got.Format("2006-01-02") != "2026-07-04" {
		t.Errorf("achievedDate = %s, want clock date", got.Format("2006-01-02"))
	}
}

func TestEvidenceRejectedWhileNotAchieved(t *testing.T) {
	cert := newTestCert(t)
	url := "https://credly.com/x"
	err := cert.SetCredlyURL(&url, fixedNow)
	wantViolation(t, err, "credlyUrl")

	err = cert.SetOfficialURL(&url, fixedNow)
	wantViolation(t, err, "officialUrl")

	err = cert.SetAchievedDate(datePtr("2026-11-20"), fixedNow)
	wantViolation(t, err, "achievedDate")
}

func TestEvidenceAcceptedWhenAchieved(t *testing.T) {
	cert := newTestCert(t)
	if err := cert.ChangeStatus(StatusAchieved, fixedNow); err != nil {
		t.Fatal(err)
	}
	url := "https://credly.com/x"
	if err := cert.SetCredlyURL(&url, fixedNow); err != nil {
		t.Fatalf("SetCredlyURL: %v", err)
	}
	if err := cert.SetAchievedDate(datePtr("2026-11-20"), fixedNow); err != nil {
		t.Fatalf("SetAchievedDate: %v", err)
	}
	if got := cert.Evidence().CredlyURL(); got == nil || *got != url {
		t.Error("credlyUrl not stored")
	}
}

func TestMovingAwayFromAchievedKeepsEvidence(t *testing.T) {
	cert := newTestCert(t)
	_ = cert.ChangeStatus(StatusAchieved, fixedNow)
	url := "https://credly.com/x"
	_ = cert.SetCredlyURL(&url, fixedNow)

	if err := cert.ChangeStatus(StatusInProgress, fixedNow); err != nil {
		t.Fatal(err)
	}
	if cert.Evidence().CredlyURL() == nil {
		t.Error("evidence lost on regression from achieved (BR-EV2)")
	}
}

// ── Attachments ─────────────────────────────────────────────────────────────

func testAttachment(t *testing.T, id string, kind AttachmentKind) Attachment {
	t.Helper()
	a, err := NewAttachment(id, kind, "cert.pdf", "application/pdf", 2048, "key-"+id, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAttachRejectedWhileNotAchieved(t *testing.T) {
	cert := newTestCert(t)
	err := cert.Attach(testAttachment(t, "a1", KindCertificate), fixedNow)
	if err == nil {
		t.Fatal("attach on planned cert accepted (BR-EV1)")
	}
}

func TestAttachReplacesSameKind(t *testing.T) {
	cert := newTestCert(t)
	_ = cert.ChangeStatus(StatusAchieved, fixedNow)

	if err := cert.Attach(testAttachment(t, "a1", KindDiploma), fixedNow); err != nil {
		t.Fatal(err)
	}
	if err := cert.Attach(testAttachment(t, "a2", KindDiploma), fixedNow); err != nil {
		t.Fatal(err)
	}
	atts := cert.Attachments()
	if len(atts) != 1 {
		t.Fatalf("attachments = %d, want 1 (replace semantics)", len(atts))
	}
	if atts[0].ID != "a2" {
		t.Errorf("kept %q, want the replacement a2", atts[0].ID)
	}
}

func TestAttachAllowsOnePerKind(t *testing.T) {
	cert := newTestCert(t)
	_ = cert.ChangeStatus(StatusAchieved, fixedNow)
	_ = cert.Attach(testAttachment(t, "a1", KindCertificate), fixedNow)
	_ = cert.Attach(testAttachment(t, "a2", KindDiploma), fixedNow)
	if len(cert.Attachments()) != 2 {
		t.Errorf("attachments = %d, want 2", len(cert.Attachments()))
	}
}

func TestDetachByID(t *testing.T) {
	cert := newTestCert(t)
	_ = cert.ChangeStatus(StatusAchieved, fixedNow)
	_ = cert.Attach(testAttachment(t, "a1", KindCertificate), fixedNow)

	if err := cert.DetachByID("a1", fixedNow); err != nil {
		t.Fatal(err)
	}
	if len(cert.Attachments()) != 0 {
		t.Error("attachment not removed")
	}
	if err := cert.DetachByID("missing", fixedNow); !errors.Is(err, ErrNotFound) {
		t.Errorf("detach missing = %v, want ErrNotFound", err)
	}
}

func TestNewAttachmentValidatesTypeAndSize(t *testing.T) {
	if _, err := NewAttachment("a", KindCertificate, "x.exe", "application/octet-stream", 10, "k", fixedNow); err == nil {
		t.Error("disallowed content type accepted")
	}
	if _, err := NewAttachment("a", KindCertificate, "x.pdf", "application/pdf", 10485761, "k", fixedNow); err == nil {
		t.Error("oversize accepted")
	}
	if _, err := NewAttachment("a", KindCertificate, "x.pdf", "application/pdf", 0, "k", fixedNow); err == nil {
		t.Error("zero size accepted")
	}
}

// ── Soft delete ─────────────────────────────────────────────────────────────

func TestSoftDelete(t *testing.T) {
	cert := newTestCert(t)
	if _, err := cert.SoftDelete(fixedNow); err != nil {
		t.Fatal(err)
	}
	if cert.DeletedAt() == nil {
		t.Fatal("deletedAt not set")
	}
	if _, err := cert.SoftDelete(fixedNow); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete = %v, want ErrNotFound", err)
	}
}

func TestMutationsRejectedOnDeletedAggregate(t *testing.T) {
	cert := newTestCert(t)
	_, _ = cert.SoftDelete(fixedNow)

	if err := cert.ChangeStatus(StatusAchieved, fixedNow); !errors.Is(err, ErrNotFound) {
		t.Errorf("ChangeStatus on deleted = %v, want ErrNotFound", err)
	}
	name, _ := NewCertificationName("New Name")
	if err := cert.Rename(name, fixedNow); !errors.Is(err, ErrNotFound) {
		t.Errorf("Rename on deleted = %v, want ErrNotFound", err)
	}
}

// ── Mutators refresh updatedAt ──────────────────────────────────────────────

func TestMutatorsRefreshUpdatedAt(t *testing.T) {
	cert := newTestCert(t)
	later := fixedNow.Add(time.Hour)
	goal := "Updated motivation"
	if err := cert.DescribeGoal(&goal, later); err != nil {
		t.Fatal(err)
	}
	if !cert.UpdatedAt().Equal(later) {
		t.Error("updatedAt not refreshed")
	}
	if cert.Goal() == nil || *cert.Goal() != goal {
		t.Error("goal not updated")
	}
}

func TestRescheduleValidatesMergedPeriod(t *testing.T) {
	cert := newTestCert(t)
	bad, err := NewStudyPeriod(datePtr("2026-12-01"), datePtr("2026-01-01"))
	if err == nil {
		t.Fatal("invalid period constructed")
	}
	_ = bad
	good, _ := NewStudyPeriod(datePtr("2027-01-01"), datePtr("2027-06-01"))
	if _, err := cert.Reschedule(good, fixedNow); err != nil {
		t.Fatal(err)
	}
	if cert.Period().Start().Format("2006-01-02") != "2027-01-01" {
		t.Error("period not updated")
	}
}
