// Package mock has sample results for developing an output template without
// running checks. They have the shape of the library's Report, Result, Check,
// Detail and TLSFacts, and every field counts how often a template reads it,
// so that --print-diagnostics can show which fields a template uses. The test
// of this package fails when the library gains a field the mock lacks.
package mock

import (
	"fmt"
	"io"
	"time"

	"github.com/dihedron/netcheck/v2/internal/tracked"
	"github.com/dihedron/netcheck/v2/pkg/netcheck"
)

// Marker highlights the name of an accessed field.
type Marker func(format string, a ...any) string

// Report mirrors netcheck.Report.
type Report struct {
	bundleID    tracked.Value[string]
	description tracked.Value[string]
	started     tracked.Value[time.Time]
	duration    tracked.Value[time.Duration]
	results     tracked.Value[[]*Result]
}

// BundleID is read by templates as .BundleID.
func (r *Report) BundleID() string { return r.bundleID.Value() }

// Description is read by templates as .Description.
func (r *Report) Description() string { return r.description.Value() }

// Started is read by templates as .Started.
func (r *Report) Started() time.Time { return r.started.Value() }

// Duration is read by templates as .Duration.
func (r *Report) Duration() time.Duration { return r.duration.Value() }

// Results is read by templates as .Results.
func (r *Report) Results() []*Result { return r.results.Value() }

// Result mirrors netcheck.Result.
type Result struct {
	check    tracked.Value[*Check]
	status   tracked.Value[netcheck.Status]
	attempts tracked.Value[int]
	latency  tracked.Value[time.Duration]
	err      tracked.Value[error]
	detail   tracked.Value[*Detail]
}

// Check is read by templates as .Check.
func (r *Result) Check() *Check { return r.check.Value() }

// Status is read by templates as .Status.
func (r *Result) Status() netcheck.Status { return r.status.Value() }

// Attempts is read by templates as .Attempts.
func (r *Result) Attempts() int { return r.attempts.Value() }

// Latency is read by templates as .Latency.
func (r *Result) Latency() time.Duration { return r.latency.Value() }

// Err is read by templates as .Err.
func (r *Result) Err() error { return r.err.Value() }

// Detail is read by templates as .Detail.
func (r *Result) Detail() *Detail { return r.detail.Value() }

// Check mirrors netcheck.Check.
type Check struct {
	name     tracked.Value[string]
	address  tracked.Value[string]
	protocol tracked.Value[netcheck.Protocol]
	timeout  tracked.Value[netcheck.Duration]
	retries  tracked.Value[int]
	wait     tracked.Value[netcheck.Duration]
	sso      tracked.Value[bool]
}

// Name is read by templates as .Name.
func (c *Check) Name() string { return c.name.Value() }

// Address is read by templates as .Address.
func (c *Check) Address() string { return c.address.Value() }

// Protocol is read by templates as .Protocol.
func (c *Check) Protocol() netcheck.Protocol { return c.protocol.Value() }

// Timeout is read by templates as .Timeout.
func (c *Check) Timeout() netcheck.Duration { return c.timeout.Value() }

// Retries is read by templates as .Retries.
func (c *Check) Retries() int { return c.retries.Value() }

// Wait is read by templates as .Wait.
func (c *Check) Wait() netcheck.Duration { return c.wait.Value() }

// SSO is read by templates as .SSO.
func (c *Check) SSO() bool { return c.sso.Value() }

// Detail mirrors netcheck.Detail.
type Detail struct {
	remote      tracked.Value[string]
	httpStatus  tracked.Value[int]
	packetsSent tracked.Value[int]
	packetsRecv tracked.Value[int]
	tls         tracked.Value[*TLSFacts]
	sshBanner   tracked.Value[string]
	sshHostKey  tracked.Value[string]
}

// Remote is read by templates as .Remote.
func (d *Detail) Remote() string { return d.remote.Value() }

// HTTPStatus is read by templates as .HTTPStatus.
func (d *Detail) HTTPStatus() int { return d.httpStatus.Value() }

// PacketsSent is read by templates as .PacketsSent.
func (d *Detail) PacketsSent() int { return d.packetsSent.Value() }

// PacketsRecv is read by templates as .PacketsRecv.
func (d *Detail) PacketsRecv() int { return d.packetsRecv.Value() }

// TLS is read by templates as .TLS; it is nil when there is no TLS session.
func (d *Detail) TLS() *TLSFacts { return d.tls.Value() }

// SSHBanner is read by templates as .SSHBanner.
func (d *Detail) SSHBanner() string { return d.sshBanner.Value() }

// SSHHostKey is read by templates as .SSHHostKey.
func (d *Detail) SSHHostKey() string { return d.sshHostKey.Value() }

// TLSFacts mirrors netcheck.TLSFacts.
type TLSFacts struct {
	version     tracked.Value[string]
	cipherSuite tracked.Value[string]
	subject     tracked.Value[string]
	issuer      tracked.Value[string]
	notAfter    tracked.Value[time.Time]
	sans        tracked.Value[[]string]
	verified    tracked.Value[bool]
}

// Version is read by templates as .Version.
func (f *TLSFacts) Version() string { return f.version.Value() }

// CipherSuite is read by templates as .CipherSuite.
func (f *TLSFacts) CipherSuite() string { return f.cipherSuite.Value() }

// Subject is read by templates as .Subject.
func (f *TLSFacts) Subject() string { return f.subject.Value() }

// Issuer is read by templates as .Issuer.
func (f *TLSFacts) Issuer() string { return f.issuer.Value() }

// NotAfter is read by templates as .NotAfter.
func (f *TLSFacts) NotAfter() time.Time { return f.notAfter.Value() }

// SANs is read by templates as .SANs.
func (f *TLSFacts) SANs() []string { return f.sans.Value() }

// Verified is read by templates as .Verified.
func (f *TLSFacts) Verified() bool { return f.verified.Value() }

// Reports returns fresh sample reports: a passed, a failed and a skipped
// check, with and without TLS facts.
func Reports() []*Report {
	started := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	facts := func() *TLSFacts {
		return &TLSFacts{
			version: tracked.New("TLS 1.3"), cipherSuite: tracked.New("TLS_AES_128_GCM_SHA256"),
			subject: tracked.New("CN=example.com"), issuer: tracked.New("CN=Example CA"),
			notAfter: tracked.New(started.AddDate(0, 3, 0)), sans: tracked.New([]string{"example.com", "www.example.com"}), verified: tracked.New(true),
		}
	}
	check := func(name, address string, p netcheck.Protocol) *Check {
		return &Check{
			name: tracked.New(name), address: tracked.New(address), protocol: tracked.New(p),
			timeout: tracked.New(netcheck.Duration(2 * time.Second)), retries: tracked.New(3), wait: tracked.New(netcheck.Duration(time.Second)), sso: tracked.New(false),
		}
	}
	detail := func(remote string, status int, f *TLSFacts) *Detail {
		return &Detail{
			remote: tracked.New(remote), httpStatus: tracked.New(status), packetsSent: tracked.New(0), packetsRecv: tracked.New(0),
			tls: tracked.New(f), sshBanner: tracked.New(""), sshHostKey: tracked.New(""),
		}
	}
	result := func(c *Check, status netcheck.Status, attempts int, err error, d *Detail) *Result {
		return &Result{
			check: tracked.New(c), status: tracked.New(status), attempts: tracked.New(attempts),
			latency: tracked.New(12 * time.Millisecond), err: tracked.New(err), detail: tracked.New(d),
		}
	}
	report := func(id string, results ...*Result) *Report {
		return &Report{
			bundleID: tracked.New(id), description: tracked.New("description of " + id), started: tracked.New(started),
			duration: tracked.New(1500 * time.Millisecond), results: tracked.New(results),
		}
	}
	return []*Report{
		report("mock-bundle-1",
			result(check("check-1-1", "localhost:80", netcheck.TCP), netcheck.Passed, 1, nil, detail("127.0.0.1:80", 0, nil)),
			result(check("check-1-2", "localhost:80", netcheck.UDP), netcheck.Failed, 3, fmt.Errorf("error type 1"), detail("", 0, nil)),
		),
		report("mock-bundle-2",
			result(check("check-2-1", "localhost:443", netcheck.HTTPS), netcheck.Passed, 1, nil, detail("127.0.0.1:443", 200, facts())),
			result(check("check-2-2", "localhost", netcheck.ICMP), netcheck.Skipped, 0, fmt.Errorf("context canceled"), detail("", 0, nil)),
		),
	}
}

// Diagnostics writes the sample reports as a tree in which the fields the
// template read are written with mark.
func Diagnostics(w io.Writer, reports []*Report, mark Marker) {
	field := func(indent, name string, accessed bool) {
		if accessed {
			_, _ = fmt.Fprintf(w, "%s%s\n", indent, mark("%s", name))
		} else {
			_, _ = fmt.Fprintf(w, "%s%s\n", indent, name)
		}
	}
	_, _ = fmt.Fprintf(w, "ACCESSED FIELDS:\n")
	for _, r := range reports {
		_, _ = fmt.Fprintf(w, "Report . {\n")
		field("  ", ".BundleID", r.bundleID.Accessed())
		field("  ", ".Description", r.description.Accessed())
		field("  ", ".Started", r.started.Accessed())
		field("  ", ".Duration", r.duration.Accessed())
		field("  ", ".Results [", r.results.Accessed())
		for _, res := range r.results.Peek() {
			_, _ = fmt.Fprintf(w, "    {\n")
			field("      ", ".Status", res.status.Accessed())
			field("      ", ".Attempts", res.attempts.Accessed())
			field("      ", ".Latency", res.latency.Accessed())
			field("      ", ".Err", res.err.Accessed())
			field("      ", ".Check {", res.check.Accessed())
			c := res.check.Peek()
			field("        ", ".Name", c.name.Accessed())
			field("        ", ".Address", c.address.Accessed())
			field("        ", ".Protocol", c.protocol.Accessed())
			field("        ", ".Timeout", c.timeout.Accessed())
			field("        ", ".Retries", c.retries.Accessed())
			field("        ", ".Wait", c.wait.Accessed())
			field("        ", ".SSO", c.sso.Accessed())
			_, _ = fmt.Fprintf(w, "      }\n")
			field("      ", ".Detail {", res.detail.Accessed())
			d := res.detail.Peek()
			field("        ", ".Remote", d.remote.Accessed())
			field("        ", ".HTTPStatus", d.httpStatus.Accessed())
			field("        ", ".PacketsSent", d.packetsSent.Accessed())
			field("        ", ".PacketsRecv", d.packetsRecv.Accessed())
			field("        ", ".SSHBanner", d.sshBanner.Accessed())
			field("        ", ".SSHHostKey", d.sshHostKey.Accessed())
			field("        ", ".TLS {", d.tls.Accessed())
			if f := d.tls.Peek(); f != nil {
				field("          ", ".Version", f.version.Accessed())
				field("          ", ".CipherSuite", f.cipherSuite.Accessed())
				field("          ", ".Subject", f.subject.Accessed())
				field("          ", ".Issuer", f.issuer.Accessed())
				field("          ", ".NotAfter", f.notAfter.Accessed())
				field("          ", ".SANs", f.sans.Accessed())
				field("          ", ".Verified", f.verified.Accessed())
			}
			_, _ = fmt.Fprintf(w, "        }\n      }\n    }\n")
		}
		_, _ = fmt.Fprintf(w, "  ]\n}\n")
	}
}
