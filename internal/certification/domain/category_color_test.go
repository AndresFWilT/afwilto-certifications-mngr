package domain

import "testing"

func TestCategoryColorNormalizesUppercase(t *testing.T) {
	c, err := NewCategoryColor("#22d3ee")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if c.String() != "#22D3EE" {
		t.Errorf("String() = %q, want #22D3EE", c.String())
	}
}

func TestCategoryColorAcceptsUppercaseAsIs(t *testing.T) {
	c, err := NewCategoryColor("#22D3EE")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if c.String() != "#22D3EE" {
		t.Errorf("String() = %q", c.String())
	}
}

func TestCategoryColorTrimsWhitespace(t *testing.T) {
	c, err := NewCategoryColor("  #22d3ee  ")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if c.String() != "#22D3EE" {
		t.Errorf("String() = %q", c.String())
	}
}

func TestCategoryColorRejectsMissingHash(t *testing.T) {
	_, err := NewCategoryColor("22d3ee")
	wantViolation(t, err, "color")
}

func TestCategoryColorRejectsWrongLength(t *testing.T) {
	for _, raw := range []string{"#22D3E", "#22D3EEE"} {
		_, err := NewCategoryColor(raw)
		wantViolation(t, err, "color")
	}
}

func TestCategoryColorRejectsNonHexChars(t *testing.T) {
	_, err := NewCategoryColor("#22D3EG")
	wantViolation(t, err, "color")
}

func TestCategoryColorRejectsNamedColor(t *testing.T) {
	_, err := NewCategoryColor("blue")
	wantViolation(t, err, "color")
}

func TestCategoryColorRejectsEmpty(t *testing.T) {
	_, err := NewCategoryColor("")
	wantViolation(t, err, "color")
}

func TestRehydrateCategoryColorTrustsInput(t *testing.T) {
	c := RehydrateCategoryColor("#22D3EE")
	if c.String() != "#22D3EE" {
		t.Errorf("String() = %q", c.String())
	}
}
