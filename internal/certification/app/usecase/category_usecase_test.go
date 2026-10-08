package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

// ── Create category (AC 1-4) ────────────────────────────────────────────────

func TestCreateCategorySucceeds(t *testing.T) {
	e := newEnv()
	svc := NewCreateCategoryService(e.categories)
	cat, err := svc.Execute(context.Background(), in.CreateCategoryCommand{Path: "Cloud / AWS", Color: "#22D3EE"})
	if err != nil {
		t.Fatal(err)
	}
	if cat.Path.Display() != "Cloud / AWS" || cat.Color.String() != "#22D3EE" {
		t.Errorf("cat = %+v", cat)
	}
}

func TestCreateCategoryRejectsDuplicatePath(t *testing.T) {
	e := newEnv()
	svc := NewCreateCategoryService(e.categories)
	if _, err := svc.Execute(context.Background(), in.CreateCategoryCommand{Path: "Cloud / AWS", Color: "#22D3EE"}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Execute(context.Background(), in.CreateCategoryCommand{Path: "cloud / aws", Color: "#34D399"})
	wantViolationOn(t, err, "path")
}

func TestCreateCategoryRejectsBadColor(t *testing.T) {
	e := newEnv()
	svc := NewCreateCategoryService(e.categories)
	_, err := svc.Execute(context.Background(), in.CreateCategoryCommand{Path: "Cloud", Color: "blue"})
	wantViolationOn(t, err, "color")
}

func TestCreateCategoryRejectsEmptyPath(t *testing.T) {
	e := newEnv()
	svc := NewCreateCategoryService(e.categories)
	_, err := svc.Execute(context.Background(), in.CreateCategoryCommand{Path: "", Color: "#22D3EE"})
	wantViolationOn(t, err, "category")
}

// ── Update category (AC 6-9) ────────────────────────────────────────────────

func TestUpdateCategoryColorOnlyLeavesPathUnchanged(t *testing.T) {
	e := newEnv()
	create := NewCreateCategoryService(e.categories)
	cat, err := create.Execute(context.Background(), in.CreateCategoryCommand{Path: "Cloud", Color: "#22D3EE"})
	if err != nil {
		t.Fatal(err)
	}

	update := NewUpdateCategoryService(e.categories)
	color := "#34D399"
	updated, err := update.Execute(context.Background(), cat.ID, in.UpdateCategoryCommand{Color: &color})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Path.Display() != "Cloud" || updated.Color.String() != "#34D399" {
		t.Errorf("updated = %+v", updated)
	}
}

func TestUpdateCategoryPathRename(t *testing.T) {
	e := newEnv()
	create := NewCreateCategoryService(e.categories)
	cat, err := create.Execute(context.Background(), in.CreateCategoryCommand{Path: "Cloud / AWS", Color: "#22D3EE"})
	if err != nil {
		t.Fatal(err)
	}

	update := NewUpdateCategoryService(e.categories)
	newPath := "Cloud / GCP"
	updated, err := update.Execute(context.Background(), cat.ID, in.UpdateCategoryCommand{Path: &newPath})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Path.Display() != "Cloud / GCP" {
		t.Errorf("path = %q", updated.Path.Display())
	}
}

func TestUpdateCategorySelfRenameNotACollision(t *testing.T) {
	e := newEnv()
	create := NewCreateCategoryService(e.categories)
	cat, err := create.Execute(context.Background(), in.CreateCategoryCommand{Path: "Cloud", Color: "#22D3EE"})
	if err != nil {
		t.Fatal(err)
	}

	update := NewUpdateCategoryService(e.categories)
	samePath := "Cloud"
	if _, err := update.Execute(context.Background(), cat.ID, in.UpdateCategoryCommand{Path: &samePath}); err != nil {
		t.Fatalf("self-rename rejected: %v", err)
	}
}

func TestUpdateCategoryRejectsCollisionWithDifferentCategory(t *testing.T) {
	e := newEnv()
	create := NewCreateCategoryService(e.categories)
	if _, err := create.Execute(context.Background(), in.CreateCategoryCommand{Path: "Cloud", Color: "#22D3EE"}); err != nil {
		t.Fatal(err)
	}
	other, err := create.Execute(context.Background(), in.CreateCategoryCommand{Path: "Soft Skills", Color: "#A78BFA"})
	if err != nil {
		t.Fatal(err)
	}

	update := NewUpdateCategoryService(e.categories)
	collidingPath := "cloud"
	_, err = update.Execute(context.Background(), other.ID, in.UpdateCategoryCommand{Path: &collidingPath})
	wantViolationOn(t, err, "path")
}

func TestUpdateCategoryUnknownIDNotFound(t *testing.T) {
	e := newEnv()
	update := NewUpdateCategoryService(e.categories)
	color := "#22D3EE"
	_, err := update.Execute(context.Background(), "missing", in.UpdateCategoryCommand{Color: &color})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ── Delete category (AC 10-12) ──────────────────────────────────────────────

func TestDeleteCategoryUnusedSucceeds(t *testing.T) {
	e := newEnv()
	create := NewCreateCategoryService(e.categories)
	cat, err := create.Execute(context.Background(), in.CreateCategoryCommand{Path: "Cloud", Color: "#22D3EE"})
	if err != nil {
		t.Fatal(err)
	}

	del := NewDeleteCategoryService(e.categories)
	if err := del.Execute(context.Background(), cat.ID); err != nil {
		t.Fatal(err)
	}
	list, err := e.categories.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("categories = %+v, want empty after delete", list)
	}
}

func TestDeleteCategoryInUseRejected(t *testing.T) {
	e := newEnv()
	create := NewCreateCategoryService(e.categories)
	cat, err := create.Execute(context.Background(), in.CreateCategoryCommand{Path: "Cloud / AWS", Color: "#22D3EE"})
	if err != nil {
		t.Fatal(err)
	}
	certSvc := NewCreateService(e.certs, e.categories, e.clock, e.ids)
	if _, err := certSvc.Execute(context.Background(), in.CreateCommand{Name: "AWS SAA", Category: "Cloud / AWS"}); err != nil {
		t.Fatal(err)
	}

	del := NewDeleteCategoryService(e.categories)
	err = del.Execute(context.Background(), cat.ID)
	wantViolationOn(t, err, "category")
}

func TestDeleteCategoryUnknownIDNotFound(t *testing.T) {
	e := newEnv()
	del := NewDeleteCategoryService(e.categories)
	if err := del.Execute(context.Background(), "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ── Certification create/update now reject unknown categories (AC 13, 15) ──

func TestCreateCertificationRejectsUnknownCategory(t *testing.T) {
	e := newEnv()
	svc := NewCreateService(e.certs, e.categories, e.clock, e.ids)
	_, err := svc.Execute(context.Background(), in.CreateCommand{Name: "X", Category: "Never Created Before"})
	wantViolationOn(t, err, "category")
}

func TestUpdateCertificationRejectsUnknownCategory(t *testing.T) {
	e := newEnv()
	create := NewCreateCategoryService(e.categories)
	if _, err := create.Execute(context.Background(), in.CreateCategoryCommand{Path: "Cloud / AWS", Color: "#22D3EE"}); err != nil {
		t.Fatal(err)
	}
	certSvc := NewCreateService(e.certs, e.categories, e.clock, e.ids)
	cert, err := certSvc.Execute(context.Background(), in.CreateCommand{Name: "AWS SAA", Category: "Cloud / AWS"})
	if err != nil {
		t.Fatal(err)
	}

	updateSvc := NewUpdateService(e.certs, e.categories, e.clock)
	_, err = updateSvc.Execute(context.Background(), cert.ID(), in.UpdateCommand{Category: in.PatchOf("Also Never Created")})
	wantViolationOn(t, err, "category")
}
