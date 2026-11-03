// Package archtest enforces the module rules of spec §3.3 (constraint 7: module-wise code with
// dependency rules checked automatically): which module may import which, and that platform code
// never depends on business modules. It runs as an ordinary test (`make arch-lint`).
package archtest

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	modPrefix      = "gatepass/internal/modules/"
	platformPrefix = "gatepass/internal/platform/"
)

// allowed lists, per module, the modules it may import. Anything else is a violation.
// Reverse dependencies (e.g. users -> auth, visits -> notifications) go through interfaces
// declared in the caller and injected by internal/app, so they never appear here.
var allowed = map[string][]string{
	"audit":         {},
	"settings":      {"audit"},
	"visitors":      {},
	"users":         {"audit"},
	"auth":          {"users"},
	"visits":        {"audit", "settings", "users", "visitors"},
	"photos":        {"audit", "settings", "visits"},
	"notifications": {"users", "visits"},
	"reports":       {"visits"},
}

func imports(t *testing.T, file string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
	require.NoError(t, err)
	var out []string
	for _, i := range f.Imports {
		out = append(out, strings.Trim(i.Path.Value, `"`))
	}
	return out
}

func goFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	require.NoError(t, filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			files = append(files, p)
		}
		return nil
	}))
	return files
}

// violations checks every non-test file under root (the internal/ directory) and returns messages.
func violations(t *testing.T, root string, allow map[string][]string) []string {
	t.Helper()
	var out []string
	mods, _ := os.ReadDir(filepath.Join(root, "modules"))
	for _, m := range mods {
		if !m.IsDir() {
			continue
		}
		ok := map[string]bool{m.Name(): true}
		for _, a := range allow[m.Name()] {
			ok[a] = true
		}
		for _, f := range goFiles(t, filepath.Join(root, "modules", m.Name())) {
			for _, imp := range imports(t, f) {
				switch {
				case strings.HasPrefix(imp, modPrefix):
					dep := strings.SplitN(strings.TrimPrefix(imp, modPrefix), "/", 2)[0]
					if !ok[dep] {
						out = append(out, "module "+m.Name()+" must not import module "+dep+" ("+filepath.Base(f)+")")
					}
				case imp == "gatepass/internal/app" || strings.HasPrefix(imp, "gatepass/internal/testutil"):
					out = append(out, "module "+m.Name()+" must not import "+imp+" ("+filepath.Base(f)+")")
				}
			}
		}
	}
	for _, f := range goFiles(t, filepath.Join(root, "platform")) {
		for _, imp := range imports(t, f) {
			if strings.HasPrefix(imp, modPrefix) || imp == "gatepass/internal/app" {
				out = append(out, "platform code must not import business code: "+imp+" ("+f+")")
			}
		}
	}
	sort.Strings(out)
	return out
}

// TC-ARCH-001: the real tree follows the rules.
func TestModuleBoundaries_TC_ARCH_001(t *testing.T) {
	assert.Empty(t, violations(t, "..", allowed))
}

// A rule checker that never fails would be worthless, so prove it catches violations.
func TestCheckerCatchesViolations_TC_ARCH_001(t *testing.T) {
	root := t.TempDir()
	write := func(rel, src string) {
		p := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(src), 0o644))
	}
	write("modules/visits/a.go", "package visits\nimport _ \"gatepass/internal/modules/notifications\"\n") // not allowed
	write("modules/reports/b.go", "package reports\nimport _ \"gatepass/internal/modules/photos\"\n")      // not allowed
	write("modules/users/c.go", "package users\nimport _ \"gatepass/internal/modules/audit\"\n")           // allowed
	write("modules/auth/d.go", "package auth\nimport _ \"gatepass/internal/testutil\"\n")                  // never
	write("platform/clock/e.go", "package clock\nimport _ \"gatepass/internal/modules/users\"\n")          // platform -> business

	got := violations(t, root, allowed)
	assert.Equal(t, []string{
		"module auth must not import gatepass/internal/testutil (d.go)",
		"module reports must not import module photos (b.go)",
		"module visits must not import module notifications (a.go)",
	}, got[:3])
	require.Len(t, got, 4)
	assert.Contains(t, got[3], "platform code must not import business code")
}
