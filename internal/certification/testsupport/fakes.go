// Package testsupport provides hand-written fakes of the out-ports (never
// mocks of concrete classes) plus deterministic clock/id fixtures, shared by
// use-case and API tests.
package testsupport

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

var FixedNow = time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)

type FixedClock struct{ Instant time.Time }

func (c FixedClock) Now() time.Time { return c.Instant }

type SeqIDGen struct{ N int }

func (g *SeqIDGen) NewID() string {
	g.N++
	return fmt.Sprintf("00000000-0000-7000-8000-%012d", g.N)
}

// FakeCertRepo implements out.CertificationRepository in memory with the same
// visibility rules as the SQL adapter (soft-deleted rows are invisible).
type FakeCertRepo struct {
	Certs map[string]*domain.Certification
	Saves int
}

func NewFakeCertRepo() *FakeCertRepo {
	return &FakeCertRepo{Certs: map[string]*domain.Certification{}}
}

func (r *FakeCertRepo) Save(_ context.Context, cert *domain.Certification) error {
	r.Certs[cert.ID()] = cert
	r.Saves++
	return nil
}

func (r *FakeCertRepo) FindByID(_ context.Context, id string) (*domain.Certification, error) {
	cert, ok := r.Certs[id]
	if !ok || cert.DeletedAt() != nil {
		return nil, domain.ErrNotFound
	}
	return cert, nil
}

func (r *FakeCertRepo) FindPage(_ context.Context, query domain.ListQuery) (domain.Page, error) {
	var alive []*domain.Certification
	for _, c := range r.Certs {
		if c.DeletedAt() == nil {
			alive = append(alive, c)
		}
	}
	sort.Slice(alive, func(i, j int) bool { return alive[i].ID() < alive[j].ID() })

	var filtered []*domain.Certification
	for _, c := range alive {
		if len(query.Statuses) > 0 {
			match := false
			for _, s := range query.Statuses {
				if c.Status() == s {
					match = true
				}
			}
			if !match {
				continue
			}
		}
		if query.Search != "" {
			needle := strings.ToLower(query.Search)
			goal := ""
			if c.Goal() != nil {
				goal = *c.Goal()
			}
			if !strings.Contains(strings.ToLower(c.Name().String()), needle) &&
				!strings.Contains(strings.ToLower(goal), needle) {
				continue
			}
		}
		if query.CategoryPrefix != "" {
			n := c.Category().Path.Normalized()
			if n != query.CategoryPrefix && !strings.HasPrefix(n, query.CategoryPrefix+"/") {
				continue
			}
		}
		if query.Year != 0 && !c.Period().IntersectsYear(query.Year) {
			continue
		}
		filtered = append(filtered, c)
	}

	total := len(filtered)
	start := (query.Page - 1) * query.PageSize
	if start > total {
		start = total
	}
	end := start + query.PageSize
	if end > total {
		end = total
	}
	return domain.Page{Items: filtered[start:end], TotalItems: total}, nil
}

func (r *FakeCertRepo) ExistsByName(_ context.Context, name string, excludeID string) (bool, error) {
	for id, c := range r.Certs {
		if id != excludeID && c.DeletedAt() == nil && strings.EqualFold(c.Name().String(), name) {
			return true, nil
		}
	}
	return false, nil
}

func (r *FakeCertRepo) FindByAttachmentID(_ context.Context, attachmentID string) (*domain.Certification, error) {
	for _, c := range r.Certs {
		if c.DeletedAt() != nil {
			continue
		}
		if _, ok := c.FindAttachment(attachmentID); ok {
			return c, nil
		}
	}
	return nil, domain.ErrNotFound
}

// FakeCategoryRepo implements out.CategoryRepository in memory.
type FakeCategoryRepo struct {
	ByNormalized map[string]domain.Category
	Certs        *FakeCertRepo
	ids          SeqIDGen
}

func NewFakeCategoryRepo(certs *FakeCertRepo) *FakeCategoryRepo {
	return &FakeCategoryRepo{ByNormalized: map[string]domain.Category{}, Certs: certs, ids: SeqIDGen{N: 9000}}
}

func (r *FakeCategoryRepo) GetOrCreate(_ context.Context, path domain.CategoryPath) (domain.Category, error) {
	if c, ok := r.ByNormalized[path.Normalized()]; ok {
		return c, nil
	}
	c := domain.Category{ID: r.ids.NewID(), Path: path}
	r.ByNormalized[path.Normalized()] = c
	return c, nil
}

func (r *FakeCategoryRepo) ListInUse(_ context.Context) ([]domain.CategoryWithCount, error) {
	counts := map[string]int{}
	if r.Certs != nil {
		for _, c := range r.Certs.Certs {
			if c.DeletedAt() == nil {
				counts[c.Category().ID]++
			}
		}
	}
	var out []domain.CategoryWithCount
	for _, cat := range r.ByNormalized {
		if n := counts[cat.ID]; n > 0 {
			out = append(out, domain.CategoryWithCount{Category: cat, Count: n})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path.Display() < out[j].Path.Display() })
	return out, nil
}

// FakeFileStorage implements out.FileStorage in memory.
type FakeFileStorage struct {
	Blobs map[string][]byte
}

func NewFakeFileStorage() *FakeFileStorage {
	return &FakeFileStorage{Blobs: map[string][]byte{}}
}

func (s *FakeFileStorage) Put(_ context.Context, key string, content io.Reader) error {
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}
	s.Blobs[key] = data
	return nil
}

type nopCloser struct{ *strings.Reader }

func (nopCloser) Close() error { return nil }

func (s *FakeFileStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := s.Blobs[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	return nopCloser{strings.NewReader(string(data))}, nil
}

func (s *FakeFileStorage) Delete(_ context.Context, key string) error {
	delete(s.Blobs, key)
	return nil
}
