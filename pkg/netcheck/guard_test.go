package netcheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// NC-01: the core drags in the standard library, x/crypto, pro-bing and
// yaml (the bundle format) and what they need, nothing else (T-11).
func TestImportAllowList(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Skipf("cannot run go list: %v", err)
	}
	allowed := []string{
		"golang.org/x/", "github.com/prometheus-community/pro-bing", "github.com/google/uuid", "gopkg.in/yaml.v3",
	}
	for _, pkg := range strings.Fields(string(out)) {
		if first, _, _ := strings.Cut(pkg, "/"); !strings.Contains(first, ".") {
			continue // standard library
		}
		ok := pkg == "github.com/dihedron/netcheck/v2/pkg/netcheck" // itself, not its sub-packages
		for _, a := range allowed {
			ok = ok || strings.HasPrefix(pkg, a)
		}
		if !ok {
			t.Errorf("the core imports %s, which is not on the allow-list", pkg)
		}
	}
}

// NC-02: nothing in pkg/ (but the linker-set build metadata) has an init function, exits the process, panics or
// exports a mutable variable.
func TestNoHiddenBehaviourInPkg(t *testing.T) {
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && d.Name() == "metadata" {
			return filepath.SkipDir // build metadata is set by the linker: exported variables by design
		}
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.Name == "init" {
					t.Errorf("%s: init function", path)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if v, ok := spec.(*ast.ValueSpec); ok && d.Tok == token.VAR {
						for _, n := range v.Names {
							if n.IsExported() && !strings.HasPrefix(n.Name, "Err") { // sentinel errors
								t.Errorf("%s: exported variable %s", path, n.Name)
							}
						}
					}
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				if fn.Name == "panic" {
					t.Errorf("%s: panic", path)
				}
			case *ast.SelectorExpr:
				if x, ok := fn.X.(*ast.Ident); ok && (x.Name == "os" && fn.Sel.Name == "Exit" || x.Name == "log" && strings.HasPrefix(fn.Sel.Name, "Fatal")) {
					t.Errorf("%s: %s.%s", path, x.Name, fn.Sel.Name)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
