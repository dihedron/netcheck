package netcheck

import (
	"encoding/json"
	"errors"
	"time"

	"gopkg.in/yaml.v3"
)

// Status is the outcome of a check.
type Status string

// The outcomes: Skipped means the check did not finish because the context ended.
const (
	Passed  Status = "passed"
	Failed  Status = "failed"
	Skipped Status = "skipped"
)

// TLSFacts describes the certificate and the session of a tls, dtls or https check.
type TLSFacts struct {
	Version     string    `json:"version,omitempty" yaml:"version,omitempty"`
	CipherSuite string    `json:"cipher_suite,omitempty" yaml:"cipher_suite,omitempty"`
	Subject     string    `json:"subject,omitempty" yaml:"subject,omitempty"`
	Issuer      string    `json:"issuer,omitempty" yaml:"issuer,omitempty"`
	NotAfter    time.Time `json:"not_after,omitempty" yaml:"not_after,omitempty"`
	SANs        []string  `json:"sans,omitempty" yaml:"sans,omitempty"`
	Verified    bool      `json:"verified" yaml:"verified"` // false when verification was skipped on request
}

// Detail carries the evidence behind a result, per protocol.
type Detail struct {
	Remote      string    `json:"remote,omitempty" yaml:"remote,omitempty"`             // the address actually dialled
	HTTPStatus  int       `json:"http_status,omitempty" yaml:"http_status,omitempty"`   // http, https
	PacketsSent int       `json:"packets_sent,omitempty" yaml:"packets_sent,omitempty"` // icmp
	PacketsRecv int       `json:"packets_recv,omitempty" yaml:"packets_recv,omitempty"` // icmp
	TLS         *TLSFacts `json:"tls,omitempty" yaml:"tls,omitempty"`                   // tls, dtls, https
	SSHBanner   string    `json:"ssh_banner,omitempty" yaml:"ssh_banner,omitempty"`     // ssh: the server's version string
	SSHHostKey  string    `json:"ssh_host_key,omitempty" yaml:"ssh_host_key,omitempty"` // ssh: SHA-256 fingerprint, not verified
}

// Result is what happened to one check.
type Result struct {
	Check    Check // as run, with the defaults applied
	Status   Status
	Attempts int
	Latency  time.Duration // of the last attempt
	Err      error         // the last error; nil when Passed; errors.Is and errors.As work
	Detail   Detail
}

// resultWire is the serialised form of a Result: the error is a string.
type resultWire struct {
	Check    Check    `json:"check" yaml:"check"`
	Status   Status   `json:"status" yaml:"status"`
	Attempts int      `json:"attempts" yaml:"attempts"`
	Latency  Duration `json:"latency" yaml:"latency"`
	Error    string   `json:"error,omitempty" yaml:"error,omitempty"`
	Detail   Detail   `json:"detail" yaml:"detail"`
}

func (r Result) wire() resultWire {
	w := resultWire{Check: r.Check, Status: r.Status, Attempts: r.Attempts, Latency: Duration(r.Latency), Detail: r.Detail}
	if r.Err != nil {
		w.Error = r.Err.Error()
	}
	return w
}

func (r *Result) fromWire(w resultWire) {
	*r = Result{Check: w.Check, Status: w.Status, Attempts: w.Attempts, Latency: time.Duration(w.Latency), Detail: w.Detail}
	if w.Error != "" {
		r.Err = errors.New(w.Error)
	}
}

// MarshalJSON writes the Result with its error as a string.
func (r Result) MarshalJSON() ([]byte, error) { return json.Marshal(r.wire()) }

// UnmarshalJSON reads a Result; its error becomes a plain error value.
func (r *Result) UnmarshalJSON(data []byte) error {
	var w resultWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	r.fromWire(w)
	return nil
}

// MarshalYAML writes the Result with its error as a string.
func (r Result) MarshalYAML() (any, error) { return r.wire(), nil }

// UnmarshalYAML reads a Result; its error becomes a plain error value.
func (r *Result) UnmarshalYAML(node *yaml.Node) error {
	var w resultWire
	if err := node.Decode(&w); err != nil {
		return err
	}
	r.fromWire(w)
	return nil
}

// Report is the outcome of a bundle.
type Report struct {
	BundleID    string
	Description string
	Started     time.Time
	Duration    time.Duration
	Results     []Result // in the order of Bundle.Checks
}

// reportWire is the serialised form of a Report: the duration is a string.
type reportWire struct {
	BundleID    string    `json:"bundle_id" yaml:"bundle_id"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	Started     time.Time `json:"started" yaml:"started"`
	Duration    Duration  `json:"duration" yaml:"duration"`
	Results     []Result  `json:"results" yaml:"results"`
}

func (r Report) wire() reportWire {
	return reportWire{r.BundleID, r.Description, r.Started, Duration(r.Duration), r.Results}
}

func (r *Report) fromWire(w reportWire) {
	*r = Report{w.BundleID, w.Description, w.Started, time.Duration(w.Duration), w.Results}
}

// MarshalJSON writes the Report with its duration as a string.
func (r Report) MarshalJSON() ([]byte, error) { return json.Marshal(r.wire()) }

// UnmarshalJSON reads a Report.
func (r *Report) UnmarshalJSON(data []byte) error {
	var w reportWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	r.fromWire(w)
	return nil
}

// MarshalYAML writes the Report with its duration as a string.
func (r Report) MarshalYAML() (any, error) { return r.wire(), nil }

// UnmarshalYAML reads a Report.
func (r *Report) UnmarshalYAML(node *yaml.Node) error {
	var w reportWire
	if err := node.Decode(&w); err != nil {
		return err
	}
	r.fromWire(w)
	return nil
}

// Failed counts the checks that failed.
func (r *Report) Failed() int { return r.count(Failed) }

// OK reports whether every check passed: a skipped check is not a pass.
func (r *Report) OK() bool { return r.count(Passed) == len(r.Results) }

func (r *Report) count(s Status) (n int) {
	for i := range r.Results {
		if r.Results[i].Status == s {
			n++
		}
	}
	return n
}
