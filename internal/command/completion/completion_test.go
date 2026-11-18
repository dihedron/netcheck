package completion

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	flags "github.com/jessevdk/go-flags"
)

type opts struct {
	Format   string   `short:"f" long:"format" choice:"json" choice:"yaml" choice:"text" default:"text" description:"how to write the report"`
	Template *string  `short:"t" long:"template" value-name:"FILE" description:"a Go template"`
	AllowNet []string `long:"allow-net" description:"only connect to this network"`
	Quiet    bool     `long:"print-diagnostics" description:"it's a \"quoted\" $(touch pwned) description"`
}

func parser() *flags.Parser { return flags.NewParser(&opts{}, flags.Default) }

func generate(t testing.TB, shell string) string {
	t.Helper()
	var out, errs bytes.Buffer
	if code := Run([]string{shell}, parser(), &out, &errs); code != 0 {
		t.Fatalf("%s: exit %d: %s", shell, code, errs.String())
	}
	return out.String()
}

var allShells = []string{"bash", "zsh", "fish", "powershell"}

// CX-1: every option, long and short, and the choices of --format.
func TestEveryOptionIsListed(t *testing.T) {
	for _, sh := range allShells {
		t.Run(sh, func(t *testing.T) {
			script := generate(t, sh)
			for _, want := range []string{"format", "template", "allow-net", "print-diagnostics", "help", "version", "completion", "json", "yaml", "text"} {
				if !strings.Contains(script, want) {
					t.Errorf("%s script lacks %q", sh, want)
				}
			}
			if sh == "bash" && !strings.Contains(script, "compgen -f") {
				t.Error("bash must complete file names")
			}
		})
	}
}

// the output does not depend on the run
func TestDeterministic(t *testing.T) {
	for _, sh := range allShells {
		if generate(t, sh) != generate(t, sh) {
			t.Errorf("%s output differs between runs", sh)
		}
	}
}

// CX-2: an unknown shell, or none, is a usage error.
func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"tcsh"}, {"bash", "extra"}} {
		var out, errs bytes.Buffer
		if code := Run(args, parser(), &out, &errs); code != 1 {
			t.Errorf("%v: exit %d, want 1", args, code)
		}
		if out.Len() != 0 || !strings.Contains(errs.String(), "usage") {
			t.Errorf("%v: stdout %q, stderr %q", args, out.String(), errs.String())
		}
	}
}

func have(t *testing.T, name string) string {
	t.Helper()
	p, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s is not installed", name)
	}
	return p
}

// CX-3 and T-K1: bash loads the script, completes, and runs nothing it was not asked to.
func TestBashLoadsAndCompletes(t *testing.T) {
	bash := have(t, "bash")
	dir := t.TempDir()
	script := filepath.Join(dir, "netcheck.bash")
	if err := os.WriteFile(script, []byte(generate(t, "bash")), 0o600); err != nil {
		t.Fatal(err)
	}
	probe := `source "$1"
complete -p netcheck >/dev/null || { echo "no completion registered"; exit 3; }
COMP_WORDS=(netcheck --fo); COMP_CWORD=1; _netcheck; echo "${COMPREPLY[*]}"
COMP_WORDS=(netcheck --format ""); COMP_CWORD=2; _netcheck; echo "${COMPREPLY[*]}"
`
	cmd := exec.Command(bash, "-c", probe, "probe", script)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "--format") || lines[1] != "json yaml text" {
		t.Errorf("got %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
		t.Fatal("the description ran as a command")
	}
}

// CX-3: zsh and fish parse the scripts when they are installed.
func TestOtherShellsParse(t *testing.T) {
	for _, tc := range []struct {
		shell, bin string
		args       func(string) []string
	}{
		{"zsh", "zsh", func(f string) []string { return []string{"-n", f} }},
		{"fish", "fish", func(f string) []string { return []string{"--no-execute", f} }},
		{"powershell", "pwsh", nil},
	} {
		t.Run(tc.shell, func(t *testing.T) {
			bin := have(t, tc.bin)
			if tc.args == nil {
				t.Skip("PowerShell has no syntax-only mode here")
			}
			f := filepath.Join(t.TempDir(), "script")
			if err := os.WriteFile(f, []byte(generate(t, tc.shell)), 0o600); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(bin, tc.args(f)...).CombinedOutput(); err != nil {
				t.Fatalf("%v: %s", err, out)
			}
		})
	}
}

// T-K1: no script runs a command to complete.
func TestScriptsRunNothing(t *testing.T) {
	for _, sh := range allShells {
		script := generate(t, sh)
		for _, bad := range []string{"eval ", "$(netcheck", "`netcheck", "netcheck --"} {
			if strings.Contains(script, bad) {
				t.Errorf("%s script contains %q", sh, bad)
			}
		}
	}
}

// CX-5 and T-K2: a hostile description is quoted for every shell.
func FuzzQuote(f *testing.F) {
	for _, s := range []string{"plain", "it's", `a"b`, "$(touch x)", "`id`", `\`, "a\nb", "]:[", "''"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for name, q := range map[string]func(string) string{"bash": quoteSh, "zsh": quoteZsh, "fish": quoteFish, "powershell": quotePS} {
			out := q(s)
			if strings.ContainsAny(out, "\n\r") {
				t.Errorf("%s: a newline survives in %q", name, out)
			}
			if !strings.HasPrefix(out, "'") || !strings.HasSuffix(out, "'") {
				t.Errorf("%s: %q is not a single quoted word", name, out)
			}
		}
		if bash, err := exec.LookPath("bash"); err == nil && len(s) < 64 {
			out, err := exec.Command(bash, "-c", "printf %s "+quoteSh(s)).Output()
			if err != nil || string(out) != strings.NewReplacer("\n", " ", "\r", " ").Replace(s) {
				t.Errorf("bash read %q back as %q (%v)", s, out, err)
			}
		}
	})
}
