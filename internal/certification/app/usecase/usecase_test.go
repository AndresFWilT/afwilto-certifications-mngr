package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
	"github.com/afwilto/certifications-mngr/internal/certification/testsupport"
)

var fixedNow = testsupport.FixedNow

type env struct {
	certs      *testsupport.FakeCertRepo
	categories *testsupport.FakeCategoryRepo
	clock      testsupport.FixedClock
	ids        *testsupport.SeqIDGen
}

func newEnv() env {
	certs := testsupport.NewFakeCertRepo()
	return env{
		certs:      certs,
		categories: testsupport.NewFakeCategoryRepo(certs),
		clock:      testsupport.FixedClock{Instant: fixedNow},
		ids:        &testsupport.SeqIDGen{},
	}
}

func datePtr(s string) *time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return &d
}

func wantViolationOn(t *testing.T, err error, field string) {
	t.Helper()
	var v *domain.ValidationError
	if !errors.As(err, &v) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	for _, viol := range v.Violations {
		if viol.Field == field {
			return
		}
	}
	t.Fatalf("violations = %+v, want one on %q", v.Violations, field)
}

func createFull(t *testing.T, e env) *domain.Certification {
	t.Helper()
	goal := "Need this before applying to Senior Cloud Architect."
	svc := NewCreateService(e.certs, e.categories, e.clock, e.ids)
	cert, err := svc.Execute(context.Background(), in.CreateCommand{
		Name:             "AWS Solutions Architect Associate",
		Category:         "TI / Cloud / AWS",
		StudyStartDate:   datePtr("2026-08-01"),
		ExpectedExamDate: datePtr("2026-11-15"),
		Goal:             &goal,
	})
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// ── Create (GH-C1..C4) ─────────────────────────────────────────────────────

func TestCreateFullData(t *testing.T) {
	e := newEnv()
	cert := createFull(t, e)

	if cert.ID() == "" {
		t.Error("no id assigned")
	}
	if cert.Status() != domain.StatusPlanned {
		t.Errorf("status = %q, want planned", cert.Status())
	}
	if cert.Category().Path.Display() != "TI / Cloud / AWS" {
		t.Errorf("category = %q", cert.Category().Path.Display())
	}
	if e.certs.Saves != 1 {
		t.Errorf("saves = %d, want 1", e.certs.Saves)
	}
}

func TestCreateReusesNormalizedCategory(t *testing.T) {
	e := newEnv()
	svc := NewCreateService(e.certs, e.categories, e.clock, e.ids)

	a, err := svc.Execute(context.Background(), in.CreateCommand{Name: "One", Category: "  Soft Skills  "})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Execute(context.Background(), in.CreateCommand{Name: "Two", Category: "SOFT SKILLS"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Category().ID != b.Category().ID {
		t.Error("normalized-equal categories got different identities")
	}
	if a.Category().Path.Display() != "Soft Skills" {
		t.Errorf("display = %q, want first-seen casing", a.Category().Path.Display())
	}
}

func TestCreateRejectsDateOrder(t *testing.T) {
	e := newEnv()
	svc := NewCreateService(e.certs, e.categories, e.clock, e.ids)
	_, err := svc.Execute(context.Background(), in.CreateCommand{
		Name:             "X",
		Category:         "TI",
		StudyStartDate:   datePtr("2026-12-01"),
		ExpectedExamDate: datePtr("2026-01-01"),
	})
	wantViolationOn(t, err, "expectedExamDate")
}

func TestCreateRejectsDuplicateNameCaseInsensitive(t *testing.T) {
	e := newEnv()
	svc := NewCreateService(e.certs, e.categories, e.clock, e.ids)
	if _, err := svc.Execute(context.Background(), in.CreateCommand{Name: "CKA", Category: "TI"}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Execute(context.Background(), in.CreateCommand{Name: "cka", Category: "TI"})
	wantViolationOn(t, err, "name")
}

// ── Get ─────────────────────────────────────────────────────────────────────

func TestGetUnknownIDNotFound(t *testing.T) {
	e := newEnv()
	svc := NewGetService(e.certs)
	_, err := svc.Execute(context.Background(), "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ── Update (GH-E1..E5) ─────────────────────────────────────────────────────

func TestUpdatePartialOnlyTouchesSentFields(t *testing.T) {
	e := newEnv()
	cert := createFull(t, e)
	later := testsupport.FixedClock{Instant: fixedNow.Add(time.Hour)}
	svc := NewUpdateService(e.certs, e.categories, later)

	updated, err := svc.Execute(context.Background(), cert.ID(), in.UpdateCommand{
		Goal: in.PatchOf("Updated motivation"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if *updated.Goal() != "Updated motivation" {
		t.Error("goal not updated")
	}
	if updated.Name().String() != "AWS Solutions Architect Associate" {
		t.Error("name changed on partial update")
	}
	if !updated.UpdatedAt().After(fixedNow) {
		t.Error("updatedAt not refreshed")
	}
}

func TestUpdateUnknownIDNotFound(t *testing.T) {
	e := newEnv()
	svc := NewUpdateService(e.certs, e.categories, e.clock)
	_, err := svc.Execute(context.Background(), "00000000-0000-7000-8000-000000000000", in.UpdateCommand{
		Goal: in.PatchOf("x"),
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateStatusTransition(t *testing.T) {
	e := newEnv()
	cert := createFull(t, e)
	svc := NewUpdateService(e.certs, e.categories, e.clock)

	updated, err := svc.Execute(context.Background(), cert.ID(), in.UpdateCommand{
		Status: in.PatchOf("in_progress"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status() != domain.StatusInProgress {
		t.Errorf("status = %q", updated.Status())
	}
}

func TestUpdateAchieveWithEvidenceSamePatch(t *testing.T) {
	e := newEnv()
	cert := createFull(t, e)
	svc := NewUpdateService(e.certs, e.categories, e.clock)

	updated, err := svc.Execute(context.Background(), cert.ID(), in.UpdateCommand{
		Status:       in.PatchOf("achieved"),
		CredlyURL:    in.PatchOf("https://credly.com/x"),
		AchievedDate: in.Patch[time.Time]{Set: true, Value: datePtr("2026-11-20")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Evidence().CredlyURL() == nil {
		t.Error("credlyUrl not stored")
	}
	if updated.Evidence().AchievedDate().Format("2006-01-02") != "2026-11-20" {
		t.Error("explicit achievedDate not honored")
	}
}

func TestUpdateAchievedDateDefaultsToClock(t *testing.T) {
	e := newEnv()
	cert := createFull(t, e)
	svc := NewUpdateService(e.certs, e.categories, e.clock)

	updated, err := svc.Execute(context.Background(), cert.ID(), in.UpdateCommand{
		Status: in.PatchOf("achieved"),
	})
	if err != nil {
		t.Fatal(err)
	}
	got := updated.Evidence().AchievedDate()
	if got == nil || got.Format("2006-01-02") != "2026-07-04" {
		t.Errorf("achievedDate = %v, want clock date", got)
	}
}

func TestUpdateEvidenceRejectedWhileNotAchieved(t *testing.T) {
	e := newEnv()
	cert := createFull(t, e)
	svc := NewUpdateService(e.certs, e.categories, e.clock)

	_, err := svc.Execute(context.Background(), cert.ID(), in.UpdateCommand{
		CredlyURL: in.PatchOf("https://credly.com/x"),
	})
	wantViolationOn(t, err, "credlyUrl")
}

func TestUpdateNullClearsNullableField(t *testing.T) {
	e := newEnv()
	cert := createFull(t, e)
	svc := NewUpdateService(e.certs, e.categories, e.clock)

	updated, err := svc.Execute(context.Background(), cert.ID(), in.UpdateCommand{
		ExpectedExamDate: in.PatchNull[time.Time](),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Period().Exam() != nil {
		t.Error("null did not clear expectedExamDate")
	}
	if updated.Period().Start() == nil {
		t.Error("start date lost")
	}
}

func TestUpdateNullRejectedForRequiredFields(t *testing.T) {
	e := newEnv()
	cert := createFull(t, e)
	svc := NewUpdateService(e.certs, e.categories, e.clock)

	if _, err := svc.Execute(context.Background(), cert.ID(), in.UpdateCommand{Name: in.PatchNull[string]()}); err == nil {
		t.Error("null name accepted")
	}
	if _, err := svc.Execute(context.Background(), cert.ID(), in.UpdateCommand{Status: in.PatchNull[string]()}); err == nil {
		t.Error("null status accepted")
	}
	if _, err := svc.Execute(context.Background(), cert.ID(), in.UpdateCommand{Category: in.PatchNull[string]()}); err == nil {
		t.Error("null category accepted")
	}
}

func TestUpdateMergedDatesValidated(t *testing.T) {
	e := newEnv()
	cert := createFull(t, e) // start 2026-08-01
	svc := NewUpdateService(e.certs, e.categories, e.clock)

	_, err := svc.Execute(context.Background(), cert.ID(), in.UpdateCommand{
		ExpectedExamDate: in.Patch[time.Time]{Set: true, Value: datePtr("2026-01-01")},
	})
	wantViolationOn(t, err, "expectedExamDate")
}

func TestUpdateDuplicateNameExcludesSelf(t *testing.T) {
	e := newEnv()
	cert := createFull(t, e)
	svc := NewUpdateService(e.certs, e.categories, e.clock)

	// Renaming to its own name (different case) must be allowed.
	if _, err := svc.Execute(context.Background(), cert.ID(), in.UpdateCommand{
		Name: in.PatchOf("aws solutions architect associate"),
	}); err != nil {
		t.Fatalf("self-rename rejected: %v", err)
	}

	create := NewCreateService(e.certs, e.categories, e.clock, e.ids)
	if _, err := create.Execute(context.Background(), in.CreateCommand{Name: "CKA", Category: "TI"}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Execute(context.Background(), cert.ID(), in.UpdateCommand{Name: in.PatchOf("CKA")})
	wantViolationOn(t, err, "name")
}

// ── Delete (GH-D1..D2) ─────────────────────────────────────────────────────

func TestDeleteHidesAndSecondDeleteNotFound(t *testing.T) {
	e := newEnv()
	cert := createFull(t, e)
	svc := NewDeleteService(e.certs, e.clock)

	if err := svc.Execute(context.Background(), cert.ID()); err != nil {
		t.Fatal(err)
	}
	get := NewGetService(e.certs)
	if _, err := get.Execute(context.Background(), cert.ID()); !errors.Is(err, domain.ErrNotFound) {
		t.Error("deleted certification still visible")
	}
	if err := svc.Execute(context.Background(), cert.ID()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("second delete = %v, want ErrNotFound", err)
	}
}

// ── List ────────────────────────────────────────────────────────────────────

func TestListValidatesQuery(t *testing.T) {
	e := newEnv()
	svc := NewListService(e.certs)
	_, err := svc.Execute(context.Background(), domain.ListQuery{Page: -1, PageSize: 20}.WithDefaults())
	if err == nil {
		t.Error("negative page accepted")
	}
}

func TestListReturnsPage(t *testing.T) {
	e := newEnv()
	createFull(t, e)
	svc := NewListService(e.certs)
	page, err := svc.Execute(context.Background(), domain.ListQuery{}.WithDefaults())
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalItems != 1 || len(page.Items) != 1 {
		t.Errorf("page = %+v, want single item", page)
	}
}
