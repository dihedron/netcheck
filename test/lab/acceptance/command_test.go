//go:build lab

package acceptance

import (
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// LB-05: the installed command: exit codes, interruption, formats, defaults, logging.

func TestExitCodes(t *testing.T) {
	_, code := check(t, "id: ok\nchecks:\n  - address: "+addr("9001")+"\n")
	if code != 0 {
		t.Errorf("all passed: exit %d, want 0", code)
	}
	_, code = check(t, "id: bad\nretries: 1\nchecks:\n  - address: "+addr("9002")+"\n")
	if code != 2 {
		t.Errorf("a check failed: exit %d, want 2", code)
	}
	_, code = check(t, "id: [not a bundle")
	if code != 1 {
		t.Errorf("an invalid bundle: exit %d, want 1", code)
	}
	if r := asProbe(t, "netcheck", "--format=json"); r.Code != 1 {
		t.Errorf("no bundle: exit %d, want 1", r.Code)
	}
	if r := asProbe(t, "netcheck", "--no-such-flag"); r.Code != 1 {
		t.Errorf("a bad flag: exit %d, want 1", r.Code)
	}
}

// Ctrl-C and SIGTERM end the run: the unfinished check is reported skipped
// and the status is 1.
func TestInterruption(t *testing.T) {
	bundle := "id: slow\nretries: 1\ntimeout: 30s\nchecks:\n  - protocol: http\n    address: nc-servers.lab:9201/ok\n"
	for _, signal := range []string{"INT", "TERM"} {
		t.Run(signal, func(t *testing.T) {
			runIn(t, "nc-client", bundle, "sh", "-c", "cat >/tmp/slow.yaml; chmod 644 /tmp/slow.yaml")
			script := "netcheck --format=json /tmp/slow.yaml >/tmp/slow.out 2>/tmp/slow.err & pid=$!; sleep 1; kill -" + signal + " $pid; wait $pid; echo $?"
			start := time.Now()
			r := asProbe(t, "sh", "-c", script)
			if strings.TrimSpace(r.Stdout) != "1" || time.Since(start) > 10*time.Second {
				t.Fatalf("exit %q after %v, want 1 promptly", strings.TrimSpace(r.Stdout), time.Since(start))
			}
			out := asProbe(t, "cat", "/tmp/slow.out").Stdout
			if !strings.Contains(out, `"status": "skipped"`) {
				t.Errorf("the unfinished check must be skipped:\n%s", out)
			}
		})
	}
}

func TestOutputFormats(t *testing.T) {
	bundle := "id: formats\nretries: 1\nchecks:\n  - name: up\n    address: " + addr("9001") + "\n  - name: down\n    address: " + addr("9002") + "\n"
	runIn(t, "nc-client", bundle, "sh", "-c", "cat >/tmp/formats.yaml; chmod 644 /tmp/formats.yaml")

	r := asProbe(t, "netcheck", "--format=yaml", "/tmp/formats.yaml")
	var reports []map[string]any
	if err := yaml.Unmarshal([]byte(r.Stdout), &reports); err != nil || len(reports) != 1 || reports[0]["bundle_id"] != "formats" {
		t.Errorf("yaml output: %v\n%s", err, r.Stdout)
	}

	r = asProbe(t, "netcheck", "/tmp/formats.yaml") // text, not a terminal
	if !strings.Contains(r.Stdout, "formats") || !strings.Contains(r.Stdout, "up") || !strings.Contains(r.Stdout, "down") || r.Code != 2 {
		t.Errorf("text output (exit %d):\n%s", r.Code, r.Stdout)
	}

	tpl := `{{ range . }}{{ .BundleID }}:{{ range .Results }} {{ .Check.Name }}={{ .Status }}{{ end }}{{ end }}`
	runIn(t, "nc-client", tpl, "sh", "-c", "cat >/tmp/out.tpl; chmod 644 /tmp/out.tpl")
	r = asProbe(t, "netcheck", "--template=/tmp/out.tpl", "/tmp/formats.yaml")
	if strings.TrimSpace(r.Stdout) != "formats: up=passed down=failed" {
		t.Errorf("template output: %q", r.Stdout)
	}

	// developing a template: no bundle, mock data, and the fields the template read
	r = asProbe(t, "netcheck", "--template=/tmp/out.tpl", "--print-diagnostics")
	if r.Code != 0 || !strings.Contains(r.Stderr, "Status") {
		t.Errorf("diagnostics (exit %d): %s", r.Code, r.Stderr)
	}
}

// The defaults file nearest to the working directory wins; a broken one is an error.
func TestDefaultsFile(t *testing.T) {
	run(t, "nc-client", "sh", "-c", "mkdir -p /tmp/conf && printf 'timeout: 400ms\\nretries: 1\\nwait: 10ms\\n' >/tmp/conf/netcheck.conf && printf 'id: d\\nchecks:\\n  - protocol: http\\n    address: nc-servers.lab:9201/ok\\n' >/tmp/conf/b.yaml && chmod -R a+rX /tmp/conf")
	start := time.Now()
	r := run(t, "nc-client", "sh", "-c", "cd /tmp/conf && runuser -u probe -- netcheck --format=json b.yaml")
	if r.Code != 2 || time.Since(start) > 3*time.Second {
		t.Errorf("with ./netcheck.conf (400 ms, 1 attempt): exit %d after %v, want 2 within 3 s (/etc/netcheck.conf says 2 s and 3 attempts)", r.Code, time.Since(start))
	}
	run(t, "nc-client", "sh", "-c", "printf 'timeout: [broken' >/tmp/conf/netcheck.conf")
	r = run(t, "nc-client", "sh", "-c", "cd /tmp/conf && runuser -u probe -- netcheck --format=json b.yaml")
	if r.Code != 1 {
		t.Errorf("a broken defaults file: exit %d, want 1", r.Code)
	}
	run(t, "nc-client", "rm", "-rf", "/tmp/conf")
}

func TestLoggingAndVersion(t *testing.T) {
	run(t, "nc-client", "sh", "-c", "printf 'id: l\\nretries: 1\\nchecks:\\n  - address: "+addr("9001")+"\\n' >/tmp/l.yaml; chmod 644 /tmp/l.yaml")
	r := run(t, "nc-client", "runuser", "-u", "probe", "--", "env", "NETCHECK_LOG_LEVEL=debug", "netcheck", "--format=json", "/tmp/l.yaml")
	if !strings.Contains(r.Stderr, "level=DEBUG") {
		t.Errorf("no debug log:\n%s", r.Stderr)
	}
	r = run(t, "nc-client", "runuser", "-u", "probe", "--", "netcheck", "--format=json", "/tmp/l.yaml")
	if strings.Contains(r.Stderr, "level=") {
		t.Errorf("logging must be off by default:\n%s", r.Stderr)
	}
	v := asProbe(t, "netcheck", "version", "-v")
	if v.Code != 0 || !strings.Contains(v.Stdout, "1.5.5") && !strings.Contains(v.Stdout, "SNAPSHOT") && !strings.Contains(v.Stdout, "2.") {
		t.Errorf("version -v:\n%s", v.Stdout)
	}
}
