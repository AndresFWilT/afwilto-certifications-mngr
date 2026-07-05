package domain

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The manifesto's grep rule made executable: the domain package must not
// import any technology vocabulary — only a small stdlib allowlist.
func TestDomainImportsArePure(t *testing.T) {
	allowed := map[string]bool{
		"errors": true, "fmt": true, "strings": true, "time": true,
		// test-only imports
		"testing": true, "go/parser": true, "go/token": true, "os": true, "path/filepath": true,
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(".", e.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if !allowed[path] {
				t.Errorf("%s imports %q — the domain must stay technology-free", e.Name(), path)
			}
		}
	}
}
