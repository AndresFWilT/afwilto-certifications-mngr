package http

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

var (
	contractOnce   sync.Once
	contractRouter routers.Router
	contractErr    error
)

func loadContract(t *testing.T) routers.Router {
	t.Helper()
	contractOnce.Do(func() {
		_, thisFile, _, _ := runtime.Caller(0)
		specPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "..", "api", "openapi.yaml")
		loader := openapi3.NewLoader()
		doc, err := loader.LoadFromFile(specPath)
		if err != nil {
			contractErr = err
			return
		}
		if err := doc.Validate(loader.Context); err != nil {
			contractErr = err
			return
		}
		contractRouter, contractErr = gorillamux.NewRouter(doc)
	})
	if contractErr != nil {
		t.Fatalf("loading openapi.yaml: %v", contractErr)
	}
	return contractRouter
}

// assertConformsToContract validates a recorded response against the OpenAPI
// schema for its route — the executable contract-drift guard.
func assertConformsToContract(t *testing.T, method, path string, rec *httptest.ResponseRecorder) {
	t.Helper()
	router := loadContract(t)

	req, _ := http.NewRequest(method, "http://localhost:8082"+path, nil)
	route, pathParams, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("route %s %s not in contract: %v", method, path, err)
	}

	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: pathParams,
			Route:      route,
			Options:    &openapi3filter.Options{ExcludeRequestBody: true},
		},
		Status: rec.Code,
		Header: rec.Header(),
		Body:   io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
	}
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		t.Errorf("%s %s (%d) violates the contract: %v\nbody: %s", method, path, rec.Code, err, rec.Body.String())
	}
}

func TestContractConformance(t *testing.T) {
	e := newAPIEnv()

	rec := e.do(t, http.MethodPost, "/api/v1/categories", `{"path":"TI / Cloud","color":"#22D3EE"}`)
	assertConformsToContract(t, http.MethodPost, "/api/v1/categories", rec)
	categoryID := decodeData(t, rec)["id"].(string)

	rec = e.do(t, http.MethodPatch, "/api/v1/categories/"+categoryID, `{"color":"#34D399"}`)
	assertConformsToContract(t, http.MethodPatch, "/api/v1/categories/"+categoryID, rec)

	rec = e.do(t, http.MethodPost, "/api/v1/certifications",
		`{"name":"Contract Cert","category":"TI / Cloud","studyStartDate":"2026-08-01","goal":"conformance"}`)
	assertConformsToContract(t, http.MethodPost, "/api/v1/certifications", rec)
	id := decodeData(t, rec)["id"].(string)

	rec = e.do(t, http.MethodGet, "/api/v1/certifications/"+id, "")
	assertConformsToContract(t, http.MethodGet, "/api/v1/certifications/"+id, rec)

	rec = e.do(t, http.MethodPatch, "/api/v1/certifications/"+id, `{"status":"achieved","credlyUrl":"https://credly.com/x"}`)
	assertConformsToContract(t, http.MethodPatch, "/api/v1/certifications/"+id, rec)

	rec = e.do(t, http.MethodGet, "/api/v1/certifications?page=1&pageSize=20", "")
	assertConformsToContract(t, http.MethodGet, "/api/v1/certifications", rec)

	rec = e.do(t, http.MethodGet, "/api/v1/categories", "")
	assertConformsToContract(t, http.MethodGet, "/api/v1/categories", rec)

	rec = e.do(t, http.MethodPost, "/api/v1/categories", `{"path":"Unused","color":"#A78BFA"}`)
	assertConformsToContract(t, http.MethodPost, "/api/v1/categories", rec)
	unusedID := decodeData(t, rec)["id"].(string)
	rec = e.do(t, http.MethodDelete, "/api/v1/categories/"+unusedID, "")
	assertConformsToContract(t, http.MethodDelete, "/api/v1/categories/"+unusedID, rec)
	rec = e.do(t, http.MethodPatch, "/api/v1/categories/00000000-0000-7000-8000-000000000000", `{"color":"#A78BFA"}`)
	assertConformsToContract(t, http.MethodPatch, "/api/v1/categories/00000000-0000-7000-8000-000000000000", rec)

	rec = e.do(t, http.MethodGet, "/api/v1/certifications/00000000-0000-7000-8000-000000000000", "")
	assertConformsToContract(t, http.MethodGet, "/api/v1/certifications/00000000-0000-7000-8000-000000000000", rec)

	rec = e.do(t, http.MethodPost, "/api/v1/certifications", `{"name":"","category":""}`)
	assertConformsToContract(t, http.MethodPost, "/api/v1/certifications", rec)

	rec = e.do(t, http.MethodGet, "/healthz", "")
	assertConformsToContract(t, http.MethodGet, "/healthz", rec)

	if !strings.Contains(rec.Body.String(), "ok") {
		t.Error("healthz body unexpected")
	}
}
