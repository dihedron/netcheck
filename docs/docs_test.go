// Package docs holds the tests that keep the guide complete and its samples
// valid (see .specs/netcheck-restructure.md): the guide itself is Markdown,
// built to PDF by build.sh.
package docs

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dihedron/netcheck/v2/internal/checks"
	"gopkg.in/yaml.v3"
)

// chapters returns the guide's chapter files, in order.
func chapters(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("guide/[0-9][0-9]-*.md")
	if err != nil || len(files) == 0 {
		t.Fatalf("no chapters in docs/guide: %v", err)
	}
	return files
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// chapter returns the text of the chapter whose file name starts with prefix.
func chapter(t *testing.T, prefix string) string {
	t.Helper()
	files, _ := filepath.Glob("guide/" + prefix + "-*.md")
	if len(files) != 1 {
		t.Fatalf("expected one chapter %s, found %v", prefix, files)
	}
	return read(t, files[0])
}

func TestChaptersAreNumberedInSequence(t *testing.T) {
	for i, file := range chapters(t) {
		want := fmt.Sprintf("guide/%02d-", i+1)
		if !strings.HasPrefix(file, want) {
			t.Errorf("%s: expected a name starting with %s", file, want)
		}
		if !strings.HasPrefix(read(t, file), "# ") {
			t.Errorf("%s does not start with a level-1 heading", file)
		}
	}
	if _, err := os.Stat("guide/metadata.yaml"); err != nil {
		t.Error(err)
	}
}

// separator matches a pipe table's header separator line.
var separator = regexp.MustCompile(`^\|(\s*:?-+:?\s*\|)+\s*$`)

// TestTablesSetColumnWidths fails on a pipe table whose separator line
// gives every column the same width: with a cell wider than the line, pandoc
// then gives every column the same width. The dashes' relative lengths set
// the columns' relative widths.
func TestTablesSetColumnWidths(t *testing.T) {
	for _, file := range chapters(t) {
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
// section in the chapter on writing bundles.
func TestEveryProtocolIsDocumented(t *testing.T) {
	text := chapter(t, "04")
	for _, name := range []string{"tcp", "udp", "icmp", "tls", "dtls", "ssh", "http", "https"} {
		var p checks.Protocol
		if err := p.FromString(name); err != nil {
			t.Errorf("the code does not accept %q any more: %v", name, err)
			continue
		}
		if !regexp.MustCompile(`(?m)^### .*\b` + name + `\b`).MatchString(text) {
			t.Errorf("chapter 4 has no \"### %s\" section", name)
		}
	}
}

// TestEverySchemeIsDocumented fails when a bundle location scheme is missing
// from the table of chapter 6.
func TestEverySchemeIsDocumented(t *testing.T) {
	text := chapter(t, "06")
	for _, scheme := range []string{
		"http://", "https://", "https-://", "http+sso://", "https+sso://", "https+sso-://",
		"redis://", "rediss://", "rediss-://",
		"consulkv://", "consulkvs://", "consulkvs-://",
		"consulsr://", "consulsrs://", "consulsrs-://",
	} {
		if !strings.Contains(text, scheme) {
			t.Errorf("chapter 6 does not mention the scheme %s", scheme)
		}
	}
}

// show writes a duration the way chapter 2 does: "20 s", "100 ms".
func show(d checks.Timeout) string {
	if time.Duration(d) < time.Second {
		return fmt.Sprintf("%d ms", time.Duration(d).Milliseconds())
	}
	return fmt.Sprintf("%d s", int(time.Duration(d).Seconds()))
}

// TestDefaultsTableMatchesTheCode fails when the compiled-in defaults of
// chapter 2 differ from the constants of the code.
func TestDefaultsTableMatchesTheCode(t *testing.T) {
	text := chapter(t, "02")
	for _, row := range []struct{ key, value string }{
		{"timeout", show(checks.DefaultTimeout)},
		{"retries", fmt.Sprint(checks.DefaultRetries)},
		{"wait", show(checks.DefaultWait)},
		{"concurrency", fmt.Sprint(checks.DefaultConcurrency)},
		{"ping.count", fmt.Sprint(checks.DefaultPingCount)},
		{"ping.interval", show(checks.DefaultPingInterval)},
		{"ping.size", fmt.Sprint(checks.DefaultPingSize)},
	} {
		pattern := regexp.MustCompile("(?m)^\\| `" + regexp.QuoteMeta(row.key) + "` \\|.*\\| " + regexp.QuoteMeta(row.value) + " *\\|$")
		if !pattern.MatchString(text) {
			t.Errorf("chapter 2's defaults table has no row for %s with the default %s", row.key, row.value)
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
// the guide that starts with "id:") or the defaults file does not decode.
func TestSamplesDecode(t *testing.T) {
	for _, file := range []string{"../examples/netcheck.yaml"} {
		var b checks.Bundle
		if err := strict([]byte(read(t, file)), &b); err != nil {
			t.Errorf("%s: %v", file, err)
		}
	}
	var d checks.Defaults
	if err := strict([]byte(read(t, "../examples/netcheck.conf")), &d); err != nil {
		t.Errorf("examples/netcheck.conf: %v", err)
	}
	block := regexp.MustCompile("(?s)```yaml\n(id: .*?)```")
	count := 0
	for _, file := range chapters(t) {
		for _, m := range block.FindAllStringSubmatch(read(t, file), -1) {
			count++
			var b checks.Bundle
			if err := strict([]byte(m[1]), &b); err != nil {
				t.Errorf("%s: a bundle sample does not decode: %v\n%s", file, err, m[1])
			}
			if len(b.Checks) == 0 {
				t.Errorf("%s: a bundle sample has no checks", file)
			}
		}
	}
	if count == 0 {
		t.Error("no bundle sample found in the guide")
	}
}

// TestIncludesExist fails when the guide includes a file that is not there.
func TestIncludesExist(t *testing.T) {
	include := regexp.MustCompile(`include="([^"]+)"`)
	for _, file := range chapters(t) {
		for _, m := range include.FindAllStringSubmatch(read(t, file), -1) {
			if _, err := os.Stat(filepath.Join("..", m[1])); err != nil {
				t.Errorf("%s includes %s: %v", file, m[1], err)
			}
		}
	}
}
