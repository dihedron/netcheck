// Package docs holds the tests that keep the manual complete and their
// samples valid (see .specs/netcheck-restructure.md): the manual is Markdown,
// built to PDF by build.sh.
package docs

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	"gopkg.in/yaml.v3"
)

// documents are the documents built by build.sh, one directory each: the
// manual is the only one.
var documents = []string{"manual"}

// chapters returns the chapter files of one document, in order.
func chapters(t *testing.T, doc string) []string {
	t.Helper()
	files, err := filepath.Glob(doc + "/[0-9][0-9]-*.md")
	if err != nil || len(files) == 0 {
		t.Fatalf("no chapters in docs/%s: %v", doc, err)
	}
	return files
}

// allChapters returns the chapter files of every document.
func allChapters(t *testing.T) []string {
	t.Helper()
	var all []string
	for _, doc := range documents {
		all = append(all, chapters(t, doc)...)
	}
	return all
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// chapter returns the text of the chapter of a document whose file name
// starts with prefix.
func chapter(t *testing.T, doc, prefix string) string {
	t.Helper()
	files, _ := filepath.Glob(doc + "/" + prefix + "-*.md")
	if len(files) != 1 {
		t.Fatalf("expected one chapter %s/%s, found %v", doc, prefix, files)
	}
	return read(t, files[0])
}

func TestChaptersAreNumberedInSequence(t *testing.T) {
	for _, doc := range documents {
		for i, file := range chapters(t, doc) {
			want := fmt.Sprintf("%s/%02d-", doc, i+1)
			if !strings.HasPrefix(file, want) {
				t.Errorf("%s: expected a name starting with %s", file, want)
			}
			if !strings.HasPrefix(read(t, file), "# ") {
				t.Errorf("%s does not start with a level-1 heading", file)
			}
		}
		if _, err := os.Stat(doc + "/metadata.yaml"); err != nil {
			t.Error(err)
		}
	}
}

// TestBuildScriptListsEveryDocument fails when a document directory is not
// built by build.sh, or build.sh names one that is not there.
func TestBuildScriptListsEveryDocument(t *testing.T) {
	script := read(t, "build.sh")
	m := regexp.MustCompile(`readonly DOCUMENTS=\(([^)]*)\)`).FindStringSubmatch(script)
	if m == nil {
		t.Fatal("build.sh has no DOCUMENTS list")
	}
	if got, want := strings.Fields(m[1]), documents; strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("build.sh builds %v, the tests expect %v", got, want)
	}
	for _, doc := range documents {
		if !strings.Contains(script, "["+doc+"]=") {
			t.Errorf("build.sh has no file name for %s", doc)
		}
	}
}

// separator matches a pipe table's header separator line.
var separator = regexp.MustCompile(`^\|(\s*:?-+:?\s*\|)+\s*$`)

// TestTablesSetColumnWidths fails on a pipe table whose separator line
// gives every column the same width: with a cell wider than the line, pandoc
// then gives every column the same width. The dashes' relative lengths set
// the columns' relative widths.
func TestTablesSetColumnWidths(t *testing.T) {
	for _, file := range allChapters(t) {
		for i, line := range strings.Split(read(t, file), "\n") {
			if !separator.MatchString(strings.TrimSpace(line)) {
				continue
			}
			lengths := map[int]bool{}
			for _, cell := range strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|") {
				lengths[len(strings.Trim(strings.TrimSpace(cell), ":"))] = true
			}
			if len(lengths) == 1 {
				t.Errorf("%s:%d: the table's columns all have the same width: set their proportions with the dashes' lengths", file, i+1)
			}
		}
	}
}

// TestEveryProtocolIsDocumented fails when a protocol the code accepts has no
// section in the chapter on writing bundles (manual chapter 4).
func TestEveryProtocolIsDocumented(t *testing.T) {
	text := chapter(t, "manual", "04")
	for _, name := range []string{"tcp", "udp", "icmp", "tls", "dtls", "ssh", "http", "https"} {
		check := netcheck.Check{Address: "host:1", Protocol: netcheck.Protocol(name)}
		if err := check.Validate(); err != nil {
			t.Errorf("the code does not accept %q any more: %v", name, err)
			continue
		}
		if !regexp.MustCompile(`(?m)^### .*\b` + name + `\b`).MatchString(text) {
			t.Errorf("manual chapter 4 has no \"### %s\" section", name)
		}
	}
}

// TestEverySchemeIsDocumented fails when a bundle location scheme is missing
// from the table of manual chapter 6.
func TestEverySchemeIsDocumented(t *testing.T) {
	text := chapter(t, "manual", "06")
	for _, scheme := range []string{
		"http://", "https://", "https-://", "http+sso://", "https+sso://", "https+sso-://",
		"redis://", "rediss://", "rediss-://",
		"consulkv://", "consulkvs://", "consulkvs-://",
		"consulsr://", "consulsrs://", "consulsrs-://",
	} {
		if !strings.Contains(text, scheme) {
			t.Errorf("manual chapter 6 does not mention the scheme %s", scheme)
		}
	}
}

// show writes a duration the way the defaults table does: "20 s", "100 ms".
func show(d netcheck.Duration) string {
	if time.Duration(d) < time.Second {
		return fmt.Sprintf("%d ms", time.Duration(d).Milliseconds())
	}
	return fmt.Sprintf("%d s", int(time.Duration(d).Seconds()))
}

// TestDefaultsTableMatchesTheCode fails when the compiled-in defaults of
// manual chapter 7 differ from the constants of the code.
func TestDefaultsTableMatchesTheCode(t *testing.T) {
	text := chapter(t, "manual", "07")
	d := netcheck.DefaultDefaults()
	for _, row := range []struct{ key, value string }{
		{"timeout", show(d.Timeout)},
		{"retries", fmt.Sprint(d.Retries)},
		{"wait", show(d.Wait)},
		{"concurrency", fmt.Sprint(d.Concurrency)},
		{"ping.count", fmt.Sprint(d.Ping.Count)},
		{"ping.interval", show(d.Ping.Interval)},
		{"ping.size", fmt.Sprint(d.Ping.Size)},
	} {
		pattern := regexp.MustCompile("(?m)^\\| `" + regexp.QuoteMeta(row.key) + "` \\|.*\\| " + regexp.QuoteMeta(row.value) + " *\\|$")
		if !pattern.MatchString(text) {
			t.Errorf("manual chapter 7's defaults table has no row for %s with the default %s", row.key, row.value)
		}
	}
}

// strict decodes YAML refusing unknown keys.
func strict(data []byte, into any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	return dec.Decode(into)
}

// TestSamplesDecode fails when a sample bundle (a file, or a YAML block of
// the manual that starts with "id:") does not decode. The defaults file is
// tested with the command, which reads it.
func TestSamplesDecode(t *testing.T) {
	for _, file := range []string{"../examples/netcheck.yaml"} {
		var b netcheck.Bundle
		if err := strict([]byte(read(t, file)), &b); err != nil {
			t.Errorf("%s: %v", file, err)
		}
	}
	block := regexp.MustCompile("(?s)```yaml\n(id: .*?)```")
	count := 0
	for _, file := range allChapters(t) {
		for _, m := range block.FindAllStringSubmatch(read(t, file), -1) {
			count++
			var b netcheck.Bundle
			if err := strict([]byte(m[1]), &b); err != nil {
				t.Errorf("%s: a bundle sample does not decode: %v\n%s", file, err, m[1])
			}
			if len(b.Checks) == 0 {
				t.Errorf("%s: a bundle sample has no checks", file)
			}
		}
	}
	if count == 0 {
		t.Error("no bundle sample found in the manual")
	}
}

// TestIncludesExist fails when the manual includes a file that is not there.
func TestIncludesExist(t *testing.T) {
	include := regexp.MustCompile(`include="([^"]+)"`)
	for _, file := range allChapters(t) {
		for _, m := range include.FindAllStringSubmatch(read(t, file), -1) {
			if _, err := os.Stat(filepath.Join("..", m[1])); err != nil {
				t.Errorf("%s includes %s: %v", file, m[1], err)
			}
		}
	}
}

// TestExitCodesAreDocumented fails when the table of exit statuses is missing
// a status the command returns.
func TestExitCodesAreDocumented(t *testing.T) {
	text := chapter(t, "manual", "06")
	for _, code := range []string{"0", "1", "2"} {
		if !regexp.MustCompile(`(?m)^\| ` + code + ` \|`).MatchString(text) {
			t.Errorf("manual chapter 6 has no exit status %s row", code)
		}
	}
}

// TestThreatsAreDiscussed fails when chapter 13 does not cover a
// threat of the spec's threat model, T-1 to T-12.
func TestThreatsAreDiscussed(t *testing.T) {
	text := chapter(t, "manual", "13")
	for i := 1; i <= 12; i++ {
		if !regexp.MustCompile(fmt.Sprintf(`\bT-%d\b`, i)).MatchString(text) {
			t.Errorf("manual chapter 13 does not discuss T-%d", i)
		}
	}
}

// exported lists the exported top-level function names of a package
// directory, tests excluded, whose name starts with prefix.
func exported(t *testing.T, dir, prefix string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && ast.IsExported(fn.Name.Name) && strings.HasPrefix(fn.Name.Name, prefix) {
				names = append(names, fn.Name.Name)
			}
		}
	}
	if len(names) == 0 {
		t.Fatalf("no function %s* found in %s", prefix, dir)
	}
	return names
}

// TestEveryLibraryEntryPointIsDocumented fails when the library chapters (14 and 15)
// does not mention Run, RunCheck, or an option of the library or of a plug-in.
func TestEveryLibraryEntryPointIsDocumented(t *testing.T) {
	var text string
	for _, prefix := range []string{"14", "15"} {
		text += chapter(t, "manual", prefix)
	}
	names := []string{"Run", "RunCheck", "Load", "Parse"}
	names = append(names, exported(t, "../pkg/netcheck", "With")...)
	names = append(names, exported(t, "../pkg/netcheck/fetch", "With")...)
	names = append(names, "Runner", "Client", "KV", "ServiceRegistry", "Source")
	for _, name := range names {
		if !regexp.MustCompile(`\b` + name + `\b`).MatchString(text) {
			t.Errorf("the library chapters (14 and 15) does not mention %s", name)
		}
	}
}

// TestExamplesNamedInTheGuideExist fails when the library chapters (14 and 15) point to
// an Example function that no test file defines.
func TestExamplesNamedInTheGuideExist(t *testing.T) {
	var defined string
	files, _ := filepath.Glob("../pkg/netcheck/*/*_test.go")
	more, _ := filepath.Glob("../pkg/netcheck/*_test.go")
	for _, file := range append(files, more...) {
		defined += read(t, file)
	}
	var text string
	for _, prefix := range []string{"14", "15"} {
		text += chapter(t, "manual", prefix)
	}
	names := regexp.MustCompile("`(Example\\w*)`").FindAllStringSubmatch(text, -1)
	if len(names) < 5 {
		t.Errorf("the library chapters (14 and 15) points to %d examples, want at least 5", len(names))
	}
	for _, m := range names {
		if !strings.Contains(defined, "func "+m[1]+"()") {
			t.Errorf("the library chapters (14 and 15) mentions %s, which no test file defines", m[1])
		}
	}
}
