package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/afwilto/certifications-mngr/internal/certification/app/usecase"
	"github.com/afwilto/certifications-mngr/internal/certification/testsupport"
)

type apiEnv struct {
	router http.Handler
	certs  *testsupport.FakeCertRepo
}

func newAPIEnv() apiEnv {
	certs := testsupport.NewFakeCertRepo()
	categories := testsupport.NewFakeCategoryRepo(certs)
	clock := testsupport.FixedClock{Instant: testsupport.FixedNow}
	ids := &testsupport.SeqIDGen{}

	router := NewRouter(RouterDeps{
		Pinger:         stubPinger{},
		AllowOrigins:   []string{"http://localhost:3000"},
		Create:         usecase.NewCreateService(certs, categories, clock, ids),
		Get:            usecase.NewGetService(certs),
		Update:         usecase.NewUpdateService(certs, categories, clock),
		Delete:         usecase.NewDeleteService(certs, clock),
		List:           usecase.NewListService(certs),
		ListCategories: usecase.NewListCategoriesService(categories),
	})
	return apiEnv{router: router, certs: certs}
}

func (e apiEnv) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func decodeData(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body.String(), err)
	}
	return envelope.Data
}

func createCert(t *testing.T, e apiEnv, name string) string {
	t.Helper()
	rec := e.do(t, http.MethodPost, "/api/v1/certifications", fmt.Sprintf(
		`{"name":%q,"category":"TI / Cloud / AWS","studyStartDate":"2026-08-01","expectedExamDate":"2026-11-15","goal":"cloud native"}`, name))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	return decodeData(t, rec)["id"].(string)
}

// ── Create (GH-C1, GH-C3, GH-C4) ────────────────────────────────────────────

func TestAPICreateFull(t *testing.T) {
	e := newAPIEnv()
	rec := e.do(t, http.MethodPost, "/api/v1/certifications",
		`{"name":"AWS Solutions Architect Associate","category":"TI / Cloud / AWS","studyStartDate":"2026-08-01","expectedExamDate":"2026-11-15","goal":"Need this."}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeData(t, rec)
	if data["status"] != "planned" {
		t.Errorf("status = %v, want planned", data["status"])
	}
	id := data["id"].(string)
	if loc := rec.Header().Get("Location"); loc != "/api/v1/certifications/"+id {
		t.Errorf("Location = %q", loc)
	}
	if _, ok := data["attachments"].([]any); !ok {
		t.Errorf("attachments = %v, want JSON array", data["attachments"])
	}
	category := data["category"].(map[string]any)
	if category["path"] != "TI / Cloud / AWS" {
		t.Errorf("category = %v", category)
	}
}

func TestAPICreateDateOrder422(t *testing.T) {
	e := newAPIEnv()
	rec := e.do(t, http.MethodPost, "/api/v1/certifications",
		`{"name":"X","category":"TI","studyStartDate":"2026-12-01","expectedExamDate":"2026-01-01"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"validation_failed"`) || !strings.Contains(body, `"expectedExamDate"`) {
		t.Errorf("body = %s", body)
	}
}

func TestAPICreateDuplicateName422(t *testing.T) {
	e := newAPIEnv()
	createCert(t, e, "CKA")
	rec := e.do(t, http.MethodPost, "/api/v1/certifications", `{"name":"cka","category":"TI"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"field":"name"`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestAPICreateMalformedJSON422(t *testing.T) {
	e := newAPIEnv()
	rec := e.do(t, http.MethodPost, "/api/v1/certifications", `{not json`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAPICreateBadDateFormat422(t *testing.T) {
	e := newAPIEnv()
	rec := e.do(t, http.MethodPost, "/api/v1/certifications",
		`{"name":"X","category":"TI","studyStartDate":"01/08/2026"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

// ── Get ─────────────────────────────────────────────────────────────────────

func TestAPIGetAndNotFound(t *testing.T) {
	e := newAPIEnv()
	id := createCert(t, e, "AWS SAA")

	rec := e.do(t, http.MethodGet, "/api/v1/certifications/"+id, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	rec = e.do(t, http.MethodGet, "/api/v1/certifications/00000000-0000-7000-8000-000000000000", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}

// ── Update (GH-E1..E5) ──────────────────────────────────────────────────────

func TestAPIPatchPartial(t *testing.T) {
	e := newAPIEnv()
	id := createCert(t, e, "AWS SAA")

	rec := e.do(t, http.MethodPatch, "/api/v1/certifications/"+id, `{"goal":"Updated motivation"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeData(t, rec)
	if data["goal"] != "Updated motivation" {
		t.Errorf("goal = %v", data["goal"])
	}
	if data["name"] != "AWS SAA" {
		t.Errorf("name changed: %v", data["name"])
	}
}

func TestAPIPatchUnknownID404(t *testing.T) {
	e := newAPIEnv()
	rec := e.do(t, http.MethodPatch, "/api/v1/certifications/00000000-0000-7000-8000-000000000000", `{"goal":"x"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAPIPatchAchieveWithEvidence(t *testing.T) {
	e := newAPIEnv()
	id := createCert(t, e, "AWS SAA")

	rec := e.do(t, http.MethodPatch, "/api/v1/certifications/"+id,
		`{"status":"achieved","credlyUrl":"https://credly.com/x","achievedDate":"2026-11-20"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeData(t, rec)
	if data["status"] != "achieved" || data["credlyUrl"] != "https://credly.com/x" || data["achievedDate"] != "2026-11-20" {
		t.Errorf("data = %v", data)
	}
}

func TestAPIPatchEvidenceWhilePlanned422(t *testing.T) {
	e := newAPIEnv()
	id := createCert(t, e, "AWS SAA")

	rec := e.do(t, http.MethodPatch, "/api/v1/certifications/"+id, `{"credlyUrl":"https://credly.com/x"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"field":"credlyUrl"`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestAPIPatchNullClearsDate(t *testing.T) {
	e := newAPIEnv()
	id := createCert(t, e, "AWS SAA")

	rec := e.do(t, http.MethodPatch, "/api/v1/certifications/"+id, `{"expectedExamDate":null}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	data := decodeData(t, rec)
	if data["expectedExamDate"] != nil {
		t.Errorf("expectedExamDate = %v, want null", data["expectedExamDate"])
	}
	if data["studyStartDate"] != "2026-08-01" {
		t.Errorf("studyStartDate = %v, must be untouched", data["studyStartDate"])
	}
}

// ── Delete (GH-D1..D2) ──────────────────────────────────────────────────────

func TestAPIDeleteLifecycle(t *testing.T) {
	e := newAPIEnv()
	id := createCert(t, e, "AWS SAA")

	rec := e.do(t, http.MethodDelete, "/api/v1/certifications/"+id, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", rec.Code)
	}
	rec = e.do(t, http.MethodGet, "/api/v1/certifications/"+id, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", rec.Code)
	}
	rec = e.do(t, http.MethodDelete, "/api/v1/certifications/"+id, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("double delete = %d, want 404", rec.Code)
	}
}

// ── List (GH-L1..L5) ────────────────────────────────────────────────────────

func TestAPIListMeta(t *testing.T) {
	e := newAPIEnv()
	for i := 0; i < 45; i++ {
		createCert(t, e, fmt.Sprintf("Cert %02d", i))
	}
	rec := e.do(t, http.MethodGet, "/api/v1/certifications?page=2&pageSize=20", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data []any `json:"data"`
		Meta struct {
			Page, PageSize, TotalItems, TotalPages int
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 20 {
		t.Errorf("items = %d, want 20", len(envelope.Data))
	}
	if envelope.Meta.Page != 2 || envelope.Meta.PageSize != 20 || envelope.Meta.TotalItems != 45 || envelope.Meta.TotalPages != 3 {
		t.Errorf("meta = %+v", envelope.Meta)
	}
}

func TestAPIListInvalidParams422(t *testing.T) {
	e := newAPIEnv()
	for _, q := range []string{"page=0", "pageSize=101", "sort=bogus", "order=sideways", "page=abc", "year=abc", "status=bogus"} {
		rec := e.do(t, http.MethodGet, "/api/v1/certifications?"+q, "")
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s → %d, want 422", q, rec.Code)
		}
	}
}

func TestAPIListFilters(t *testing.T) {
	e := newAPIEnv()
	id := createCert(t, e, "Kubernetes CKA") // goal "cloud native"
	e.do(t, http.MethodPatch, "/api/v1/certifications/"+id, `{"status":"in_progress"}`)
	e.do(t, http.MethodPost, "/api/v1/certifications", `{"name":"Oracle Java","category":"TI / Development","goal":"backend"}`)

	rec := e.do(t, http.MethodGet, "/api/v1/certifications?search=cloud", "")
	var envelope struct{ Data []map[string]any }
	_ = json.Unmarshal(rec.Body.Bytes(), &envelope)
	if len(envelope.Data) != 1 || envelope.Data[0]["name"] != "Kubernetes CKA" {
		t.Errorf("search result = %v", envelope.Data)
	}

	rec = e.do(t, http.MethodGet, "/api/v1/certifications?status=in_progress&category=TI+%2F+Cloud&year=2026", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &envelope)
	if len(envelope.Data) != 1 || envelope.Data[0]["name"] != "Kubernetes CKA" {
		t.Errorf("AND filter result = %v", envelope.Data)
	}
}

// ── Categories (GH-K1) ──────────────────────────────────────────────────────

func TestAPICategoriesInUse(t *testing.T) {
	e := newAPIEnv()
	id := createCert(t, e, "Solo") // TI / Cloud / AWS
	rec := e.do(t, http.MethodGet, "/api/v1/categories", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"count":1`) {
		t.Errorf("body = %s", rec.Body.String())
	}

	e.do(t, http.MethodDelete, "/api/v1/certifications/"+id, "")
	rec = e.do(t, http.MethodGet, "/api/v1/categories", "")
	var envelope struct{ Data []any }
	_ = json.Unmarshal(rec.Body.Bytes(), &envelope)
	if len(envelope.Data) != 0 {
		t.Errorf("categories after delete = %v, want empty", envelope.Data)
	}
}
