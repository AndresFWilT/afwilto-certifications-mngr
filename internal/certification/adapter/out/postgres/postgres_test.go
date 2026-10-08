package postgres

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/afwilto/certifications-mngr/internal/certification/domain"
	"github.com/afwilto/certifications-mngr/internal/certification/testsupport"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("certifications"),
		tcpostgres.WithUsername("certifications"),
		tcpostgres.WithPassword("certifications"),
		tcpostgres.BasicWaitStrategies(),
		tcpostgres.WithSQLDriver("pgx"),
	)
	if err != nil {
		panic(err)
	}
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic(err)
	}
	if err := migrateUp(dsn); err != nil {
		panic(err)
	}
	pool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		panic(err)
	}
	code := m.Run()
	pool.Close()
	_ = container.Terminate(ctx)
	os.Exit(code)
}

func migrateUp(dsn string) error {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "..", "migrations")
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	m, err := migrate.New("file://"+abs, "pgx5://"+dsn[len("postgres://"):])
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func skipShort(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires Docker")
	}
}

func cleanTables(t *testing.T) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `TRUNCATE attachments, certifications, categories CASCADE`); err != nil {
		t.Fatal(err)
	}
}

type repoEnv struct {
	certs      *CertificationRepository
	categories *CategoryRepository
	ids        *testsupport.SeqIDGen
	clock      testsupport.FixedClock
}

func newRepoEnv(t *testing.T) repoEnv {
	skipShort(t)
	cleanTables(t)
	ids := &testsupport.SeqIDGen{}
	return repoEnv{
		certs:      NewCertificationRepository(pool),
		categories: NewCategoryRepository(pool, ids),
		ids:        ids,
		clock:      testsupport.FixedClock{Instant: testsupport.FixedNow},
	}
}

func datePtr(s string) *time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return &d
}

// mustGetOrCreateCategory mirrors the pre-explicit-CRUD GetOrCreate
// convenience for test setup: the category repository itself now requires
// callers to create a category before referencing it, but many table tests
// below only care about certification behavior and reuse the same category
// path across many certs.
func (e repoEnv) mustGetOrCreateCategory(t *testing.T, path domain.CategoryPath) domain.Category {
	t.Helper()
	ctx := context.Background()
	cat, err := e.categories.GetByPath(ctx, path)
	if err == nil {
		return cat
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	color, _ := domain.NewCategoryColor("#22D3EE")
	cat, err = e.categories.Create(ctx, path, color)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func (e repoEnv) newCert(t *testing.T, name, category string, start, exam *time.Time) *domain.Certification {
	t.Helper()
	ctx := context.Background()
	n, err := domain.NewCertificationName(name)
	if err != nil {
		t.Fatal(err)
	}
	path, err := domain.NewCategoryPath(category)
	if err != nil {
		t.Fatal(err)
	}
	cat := e.mustGetOrCreateCategory(t, path)
	period, err := domain.NewStudyPeriod(start, exam)
	if err != nil {
		t.Fatal(err)
	}
	cert, _, err := domain.NewCertification(e.ids.NewID(), n, cat, period, nil, e.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := e.certs.Save(ctx, cert); err != nil {
		t.Fatal(err)
	}
	return cert
}

// ── Round trip ──────────────────────────────────────────────────────────────

func TestSaveFindRoundTrip(t *testing.T) {
	e := newRepoEnv(t)
	ctx := context.Background()
	cert := e.newCert(t, "AWS SAA", "TI / Cloud / AWS", datePtr("2026-08-01"), datePtr("2026-11-15"))

	found, err := e.certs.FindByID(ctx, cert.ID())
	if err != nil {
		t.Fatal(err)
	}
	if found.Name().String() != "AWS SAA" || found.Category().Path.Display() != "TI / Cloud / AWS" {
		t.Errorf("round trip lost data: %s / %s", found.Name().String(), found.Category().Path.Display())
	}
	if found.Period().Start().Format("2006-01-02") != "2026-08-01" {
		t.Errorf("start = %v", found.Period().Start())
	}
}

func TestSavePersistsAttachments(t *testing.T) {
	e := newRepoEnv(t)
	ctx := context.Background()
	cert := e.newCert(t, "CKA", "TI / Cloud", nil, nil)

	if err := cert.ChangeStatus(domain.StatusAchieved, e.clock.Now()); err != nil {
		t.Fatal(err)
	}
	att, err := domain.NewAttachment(e.ids.NewID(), domain.KindDiploma, "diploma.pdf", "application/pdf", 2048, "certs/x/diploma", e.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := cert.Attach(att, e.clock.Now()); err != nil {
		t.Fatal(err)
	}
	if err := e.certs.Save(ctx, cert); err != nil {
		t.Fatal(err)
	}

	found, err := e.certs.FindByID(ctx, cert.ID())
	if err != nil {
		t.Fatal(err)
	}
	atts := found.Attachments()
	if len(atts) != 1 || atts[0].FileName != "diploma.pdf" || atts[0].StorageKey != "certs/x/diploma" {
		t.Errorf("attachments = %+v", atts)
	}

	byAtt, err := e.certs.FindByAttachmentID(ctx, atts[0].ID)
	if err != nil || byAtt.ID() != cert.ID() {
		t.Errorf("FindByAttachmentID = %v, %v", byAtt, err)
	}
}

// ── Name uniqueness (partial index) ─────────────────────────────────────────

func TestNameUniqueAmongLivingOnly(t *testing.T) {
	e := newRepoEnv(t)
	ctx := context.Background()
	cert := e.newCert(t, "CKA", "TI", nil, nil)

	taken, err := e.certs.ExistsByName(ctx, "cka", "")
	if err != nil || !taken {
		t.Errorf("ExistsByName(cka) = %v, %v — want true (case-insensitive)", taken, err)
	}

	if _, err := cert.SoftDelete(e.clock.Now()); err != nil {
		t.Fatal(err)
	}
	if err := e.certs.Save(ctx, cert); err != nil {
		t.Fatal(err)
	}

	taken, err = e.certs.ExistsByName(ctx, "CKA", "")
	if err != nil || taken {
		t.Errorf("name still taken after soft delete")
	}
	// Reusing the name must not trip the partial unique index.
	e.newCert(t, "CKA", "TI", nil, nil)
}

func TestUniqueIndexBackstop(t *testing.T) {
	e := newRepoEnv(t)
	e.newCert(t, "Same Name", "TI", nil, nil)

	ctx := context.Background()
	n, _ := domain.NewCertificationName("same name")
	path, _ := domain.NewCategoryPath("TI")
	cat := e.mustGetOrCreateCategory(t, path)
	period, _ := domain.NewStudyPeriod(nil, nil)
	dup, _, _ := domain.NewCertification(e.ids.NewID(), n, cat, period, nil, e.clock.Now())

	err := e.certs.Save(ctx, dup)
	var v *domain.ValidationError
	if !errors.As(err, &v) {
		t.Errorf("duplicate save = %v, want ValidationError from unique index", err)
	}
}

// ── Soft delete visibility ──────────────────────────────────────────────────

func TestSoftDeletedInvisible(t *testing.T) {
	e := newRepoEnv(t)
	ctx := context.Background()
	cert := e.newCert(t, "Gone", "TI", nil, nil)
	_, _ = cert.SoftDelete(e.clock.Now())
	if err := e.certs.Save(ctx, cert); err != nil {
		t.Fatal(err)
	}

	if _, err := e.certs.FindByID(ctx, cert.ID()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("FindByID on deleted = %v, want ErrNotFound", err)
	}
	page, err := e.certs.FindPage(ctx, domain.ListQuery{}.WithDefaults())
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalItems != 0 {
		t.Errorf("deleted cert still listed")
	}
	categories, err := e.categories.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 1 || categories[0].Count != 0 {
		t.Errorf("categories = %+v, want the one category listed with count 0", categories)
	}
}

// ── Categories (CRUD) ────────────────────────────────────────────────────────

func TestCategoryCreateRejectsDuplicateNormalizedPath(t *testing.T) {
	e := newRepoEnv(t)
	ctx := context.Background()
	color, _ := domain.NewCategoryColor("#22D3EE")

	p1, _ := domain.NewCategoryPath("  Soft Skills  ")
	if _, err := e.categories.Create(ctx, p1, color); err != nil {
		t.Fatal(err)
	}
	p2, _ := domain.NewCategoryPath("SOFT SKILLS")
	_, err := e.categories.Create(ctx, p2, color)
	var v *domain.ValidationError
	if !errors.As(err, &v) {
		t.Errorf("err = %v, want ValidationError (unique path)", err)
	}
}

func TestCategoryGetByPathNormalizesCaseInsensitive(t *testing.T) {
	e := newRepoEnv(t)
	ctx := context.Background()
	color, _ := domain.NewCategoryColor("#22D3EE")
	p, _ := domain.NewCategoryPath("  Soft Skills  ")
	created, err := e.categories.Create(ctx, p, color)
	if err != nil {
		t.Fatal(err)
	}

	lookup, _ := domain.NewCategoryPath("SOFT SKILLS")
	found, err := e.categories.GetByPath(ctx, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != created.ID || found.Path.Display() != "Soft Skills" {
		t.Errorf("found = %+v, want first-seen casing under the same id", found)
	}
}

func TestCategoryGetByPathUnknownNotFound(t *testing.T) {
	e := newRepoEnv(t)
	p, _ := domain.NewCategoryPath("Nope")
	if _, err := e.categories.GetByPath(context.Background(), p); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// TestListIncludesUnusedCategories covers backend AC-5: List must return
// categories nobody has used yet (the new "create category" form populates
// exactly this), not only in-use ones like the old ListInUse did.
func TestListIncludesUnusedCategories(t *testing.T) {
	e := newRepoEnv(t)
	e.newCert(t, "A", "TI / Cloud / AWS", nil, nil)
	e.newCert(t, "B", "TI / Cloud / AWS", nil, nil)
	e.newCert(t, "C", "Soft Skills", nil, nil)
	color, _ := domain.NewCategoryColor("#A78BFA")
	unusedPath, _ := domain.NewCategoryPath("Never Used")
	if _, err := e.categories.Create(context.Background(), unusedPath, color); err != nil {
		t.Fatal(err)
	}

	categories, err := e.categories.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 3 {
		t.Fatalf("categories = %d, want 3 (including unused)", len(categories))
	}
	for _, c := range categories {
		want := 1
		switch c.Path.Display() {
		case "TI / Cloud / AWS":
			want = 2
		case "Never Used":
			want = 0
		}
		if c.Count != want {
			t.Errorf("%s count = %d, want %d", c.Path.Display(), c.Count, want)
		}
	}
}

func TestCategoryUpdateRenamePropagatesToCertificationsWithoutMigration(t *testing.T) {
	e := newRepoEnv(t)
	ctx := context.Background()
	cert := e.newCert(t, "AWS SAA", "Cloud / AWS", nil, nil)
	categoryID := cert.Category().ID

	newPath, _ := domain.NewCategoryPath("Cloud / GCP")
	color, _ := domain.NewCategoryColor("#22D3EE")
	if _, err := e.categories.Update(ctx, categoryID, newPath, color); err != nil {
		t.Fatal(err)
	}

	found, err := e.certs.FindByID(ctx, cert.ID())
	if err != nil {
		t.Fatal(err)
	}
	if found.Category().Path.Display() != "Cloud / GCP" {
		t.Errorf("category path = %q, want renamed path with no data migration", found.Category().Path.Display())
	}
}

func TestCategoryUpdateUnknownIDNotFound(t *testing.T) {
	e := newRepoEnv(t)
	path, _ := domain.NewCategoryPath("X")
	color, _ := domain.NewCategoryColor("#22D3EE")
	_, err := e.categories.Update(context.Background(), "00000000-0000-7000-8000-000000000000", path, color)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestCategoryDeleteUnusedSucceeds(t *testing.T) {
	e := newRepoEnv(t)
	ctx := context.Background()
	color, _ := domain.NewCategoryColor("#22D3EE")
	path, _ := domain.NewCategoryPath("Unused")
	cat, err := e.categories.Create(ctx, path, color)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.categories.Delete(ctx, cat.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.categories.GetByPath(ctx, path); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound after delete", err)
	}
}

func TestCategoryDeleteInUseRejected(t *testing.T) {
	e := newRepoEnv(t)
	ctx := context.Background()
	cert := e.newCert(t, "AWS SAA", "Cloud / AWS", nil, nil)

	err := e.categories.Delete(ctx, cert.Category().ID)
	var v *domain.ValidationError
	if !errors.As(err, &v) {
		t.Errorf("err = %v, want ValidationError (in_use)", err)
	}
}

func TestCategoryDeleteUnknownIDNotFound(t *testing.T) {
	e := newRepoEnv(t)
	err := e.categories.Delete(context.Background(), "00000000-0000-7000-8000-000000000000")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ── FindPage filters ────────────────────────────────────────────────────────

func TestFindPagePaginationAndSort(t *testing.T) {
	e := newRepoEnv(t)
	e.newCert(t, "Alpha", "TI", datePtr("2026-01-01"), datePtr("2026-03-01"))
	e.newCert(t, "Bravo", "TI", datePtr("2026-02-01"), datePtr("2026-04-01"))
	e.newCert(t, "Charlie", "TI", nil, nil) // null dates → last on asc sort

	page, err := e.certs.FindPage(context.Background(), domain.ListQuery{
		Sort: domain.SortExpectedExamDate, Order: domain.OrderAsc,
	}.WithDefaults())
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalItems != 3 {
		t.Fatalf("total = %d", page.TotalItems)
	}
	names := []string{}
	for _, c := range page.Items {
		names = append(names, c.Name().String())
	}
	want := []string{"Alpha", "Bravo", "Charlie"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("order = %v, want %v (nulls last)", names, want)
		}
	}

	small, err := e.certs.FindPage(context.Background(), domain.ListQuery{Page: 2, PageSize: 2}.WithDefaults())
	if err != nil {
		t.Fatal(err)
	}
	if len(small.Items) != 1 || small.TotalItems != 3 {
		t.Errorf("page 2 of 2: items=%d total=%d", len(small.Items), small.TotalItems)
	}
}

func TestFindPageSearchNameAndGoal(t *testing.T) {
	e := newRepoEnv(t)
	ctx := context.Background()
	cka := e.newCert(t, "Kubernetes CKA", "TI", nil, nil)
	goal := "cloud native"
	if err := cka.DescribeGoal(&goal, e.clock.Now()); err != nil {
		t.Fatal(err)
	}
	if err := e.certs.Save(ctx, cka); err != nil {
		t.Fatal(err)
	}
	e.newCert(t, "Oracle Java", "TI", nil, nil)

	page, err := e.certs.FindPage(ctx, domain.ListQuery{Search: "CLOUD"}.WithDefaults())
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalItems != 1 || page.Items[0].Name().String() != "Kubernetes CKA" {
		t.Errorf("search hit = %+v", page)
	}
}

func TestFindPageCategoryPrefixBoundary(t *testing.T) {
	e := newRepoEnv(t)
	e.newCert(t, "AWS", "TI / Cloud / AWS", nil, nil)
	e.newCert(t, "Azure", "TI / Cloud / Azure", nil, nil)
	e.newCert(t, "Cloudy", "TI / Cloudy", nil, nil)

	page, err := e.certs.FindPage(context.Background(), domain.ListQuery{CategoryPrefix: "ti/cloud"}.WithDefaults())
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalItems != 2 {
		names := []string{}
		for _, c := range page.Items {
			names = append(names, c.Name().String())
		}
		t.Errorf("prefix matched %v, want exactly AWS+Azure (segment boundary)", names)
	}
}

func TestFindPageYearIntersection(t *testing.T) {
	e := newRepoEnv(t)
	e.newCert(t, "In 2026", "TI", datePtr("2026-08-01"), datePtr("2026-11-15"))
	e.newCert(t, "Crosses into 2027", "TI", datePtr("2026-10-01"), datePtr("2027-01-15"))
	e.newCert(t, "Only 2025", "TI", datePtr("2025-01-01"), datePtr("2025-06-01"))
	e.newCert(t, "Dateless", "TI", nil, nil)

	page, err := e.certs.FindPage(context.Background(), domain.ListQuery{Year: 2027}.WithDefaults())
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalItems != 1 || page.Items[0].Name().String() != "Crosses into 2027" {
		t.Errorf("year filter = %+v", page.TotalItems)
	}

	page, err = e.certs.FindPage(context.Background(), domain.ListQuery{Year: 2026}.WithDefaults())
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalItems != 2 {
		t.Errorf("2026 matched %d, want 2 (dateless excluded)", page.TotalItems)
	}
}

func TestFindPageStatusAndComposition(t *testing.T) {
	e := newRepoEnv(t)
	ctx := context.Background()
	cka := e.newCert(t, "CKA", "TI / Cloud", datePtr("2026-08-01"), datePtr("2026-11-15"))
	if err := cka.ChangeStatus(domain.StatusInProgress, e.clock.Now()); err != nil {
		t.Fatal(err)
	}
	if err := e.certs.Save(ctx, cka); err != nil {
		t.Fatal(err)
	}
	e.newCert(t, "AWS planned same category", "TI / Cloud", datePtr("2026-01-01"), datePtr("2026-03-01"))

	page, err := e.certs.FindPage(ctx, domain.ListQuery{
		Statuses:       []domain.AchievementStatus{domain.StatusInProgress},
		CategoryPrefix: "ti/cloud",
		Year:           2026,
	}.WithDefaults())
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalItems != 1 || page.Items[0].Name().String() != "CKA" {
		t.Errorf("AND composition = %d items", page.TotalItems)
	}
}
