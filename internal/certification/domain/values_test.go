package domain

import (
	"errors"
	"testing"
	"time"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func datePtr(s string) *time.Time {
	d := date(s)
	return &d
}

func violationFields(err error) []string {
	var v *ValidationError
	if !errors.As(err, &v) {
		return nil
	}
	fields := make([]string, len(v.Violations))
	for i, viol := range v.Violations {
		fields[i] = viol.Field
	}
	return fields
}

func wantViolation(t *testing.T, err error, field string) {
	t.Helper()
	fields := violationFields(err)
	if fields == nil {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	for _, f := range fields {
		if f == field {
			return
		}
	}
	t.Fatalf("violations %v, want one on %q", fields, field)
}

// ── CertificationName ───────────────────────────────────────────────────────

func TestCertificationNameTrimsAndAccepts(t *testing.T) {
	n, err := NewCertificationName("  AWS SAA  ")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if n.String() != "AWS SAA" {
		t.Errorf("name = %q, want trimmed", n.String())
	}
}

func TestCertificationNameRejectsEmpty(t *testing.T) {
	_, err := NewCertificationName("   ")
	wantViolation(t, err, "name")
}

func TestCertificationNameRejectsOver200(t *testing.T) {
	long := make([]byte, 201)
	for i := range long {
		long[i] = 'a'
	}
	_, err := NewCertificationName(string(long))
	wantViolation(t, err, "name")
}

// ── CategoryPath ────────────────────────────────────────────────────────────

func TestCategoryPathNormalizes(t *testing.T) {
	tests := []struct {
		raw, display, normalized string
	}{
		{"TI / Cloud / AWS", "TI / Cloud / AWS", "ti/cloud/aws"},
		{"  Soft Skills  ", "Soft Skills", "soft skills"},
		{"TI/Cloud/AWS", "TI / Cloud / AWS", "ti/cloud/aws"},
		{"Languages /  English", "Languages / English", "languages/english"},
		{"SOFT   SKILLS", "SOFT SKILLS", "soft skills"},
	}
	for _, tc := range tests {
		p, err := NewCategoryPath(tc.raw)
		if err != nil {
			t.Fatalf("NewCategoryPath(%q) err = %v", tc.raw, err)
		}
		if p.Display() != tc.display {
			t.Errorf("Display(%q) = %q, want %q", tc.raw, p.Display(), tc.display)
		}
		if p.Normalized() != tc.normalized {
			t.Errorf("Normalized(%q) = %q, want %q", tc.raw, p.Normalized(), tc.normalized)
		}
	}
}

func TestCategoryPathCaseInsensitiveIdentity(t *testing.T) {
	a, _ := NewCategoryPath("Soft Skills")
	b, _ := NewCategoryPath("SOFT SKILLS")
	if a.Normalized() != b.Normalized() {
		t.Errorf("normalized forms differ: %q vs %q", a.Normalized(), b.Normalized())
	}
}

func TestCategoryPathRejectsEmptyAndEmptySegment(t *testing.T) {
	for _, raw := range []string{"", "   ", "TI //  AWS", "/"} {
		if _, err := NewCategoryPath(raw); err == nil {
			t.Errorf("NewCategoryPath(%q) accepted, want error", raw)
		}
	}
}

func TestCategoryPathRejectsOver120(t *testing.T) {
	long := make([]byte, 121)
	for i := range long {
		long[i] = 'a'
	}
	_, err := NewCategoryPath(string(long))
	wantViolation(t, err, "category")
}

// ── StudyPeriod ─────────────────────────────────────────────────────────────

func TestStudyPeriodOrderInvariant(t *testing.T) {
	_, err := NewStudyPeriod(datePtr("2026-12-01"), datePtr("2026-01-01"))
	wantViolation(t, err, "expectedExamDate")
}

func TestStudyPeriodAcceptsPartialAndEmpty(t *testing.T) {
	cases := []struct{ start, exam *time.Time }{
		{nil, nil},
		{datePtr("2026-08-01"), nil},
		{nil, datePtr("2026-11-15")},
		{datePtr("2026-08-01"), datePtr("2026-08-01")},
	}
	for i, tc := range cases {
		if _, err := NewStudyPeriod(tc.start, tc.exam); err != nil {
			t.Errorf("case %d: err = %v", i, err)
		}
	}
}

func TestStudyPeriodIntersectsYear(t *testing.T) {
	tests := []struct {
		name        string
		start, exam *time.Time
		year        int
		want        bool
	}{
		{"range inside year", datePtr("2026-08-01"), datePtr("2026-11-15"), 2026, true},
		{"range crosses year end", datePtr("2026-10-01"), datePtr("2027-01-15"), 2027, true},
		{"range before year", datePtr("2025-01-01"), datePtr("2025-06-01"), 2026, false},
		{"start only in year", datePtr("2026-03-01"), nil, 2026, true},
		{"start only other year", datePtr("2025-03-01"), nil, 2026, false},
		{"exam only in year", nil, datePtr("2026-06-01"), 2026, true},
		{"no dates", nil, nil, 2026, false},
	}
	for _, tc := range tests {
		p, err := NewStudyPeriod(tc.start, tc.exam)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := p.IntersectsYear(tc.year); got != tc.want {
			t.Errorf("%s: IntersectsYear(%d) = %v, want %v", tc.name, tc.year, got, tc.want)
		}
	}
}

// ── Evidence URLs ───────────────────────────────────────────────────────────

func TestEvidenceURLMustBeHTTPS(t *testing.T) {
	if err := ValidateEvidenceURL("credlyUrl", "http://credly.com/x"); err == nil {
		t.Error("http URL accepted, want violation")
	}
	if err := ValidateEvidenceURL("credlyUrl", "not a url"); err == nil {
		t.Error("garbage accepted, want violation")
	}
	if err := ValidateEvidenceURL("credlyUrl", "https://credly.com/x"); err != nil {
		t.Errorf("https URL rejected: %v", err)
	}
}

// ── AchievementStatus / AttachmentKind ─────────────────────────────────────

func TestParseStatus(t *testing.T) {
	for _, s := range []string{"planned", "in_progress", "achieved"} {
		if _, err := ParseStatus(s); err != nil {
			t.Errorf("ParseStatus(%q) err = %v", s, err)
		}
	}
	if _, err := ParseStatus("bogus"); err == nil {
		t.Error("bogus status accepted")
	}
}

func TestParseAttachmentKind(t *testing.T) {
	for _, k := range []string{"certificate", "diploma"} {
		if _, err := ParseAttachmentKind(k); err != nil {
			t.Errorf("ParseAttachmentKind(%q) err = %v", k, err)
		}
	}
	if _, err := ParseAttachmentKind("selfie"); err == nil {
		t.Error("bogus kind accepted")
	}
}
