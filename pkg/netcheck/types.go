package netcheck

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that reads and writes as a string ("2s",
// "150ms") in JSON and YAML, as the bundle files always did.
type Duration time.Duration

// String returns the duration in the form time.Duration prints.
func (d Duration) String() string { return time.Duration(d).String() }

// MarshalJSON writes the duration as a JSON string.
func (d Duration) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

// UnmarshalJSON reads a duration from a JSON string such as "2s".
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("duration: %w", err)
	}
	return d.parse(s)
}

// MarshalYAML writes the duration as a YAML string.
func (d Duration) MarshalYAML() (any, error) { return d.String(), nil }

// UnmarshalYAML reads a duration from a YAML string such as 2s.
func (d *Duration) UnmarshalYAML(node *yaml.Node) error { return d.parse(node.Value) }

func (d *Duration) parse(s string) error {
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("duration: %w", err)
	}
	*d = Duration(v)
	return nil
}

// Protocol names a kind of check. The zero value is invalid.
type Protocol string

// The protocols a check can use.
const (
	TCP   Protocol = "tcp"
	UDP   Protocol = "udp"
	ICMP  Protocol = "icmp"
	TLS   Protocol = "tls"
	DTLS  Protocol = "dtls" // needs a runner from the dtls sub-package
	SSH   Protocol = "ssh"
	HTTP  Protocol = "http"
	HTTPS Protocol = "https"
)

var protocols = map[Protocol]bool{TCP: true, UDP: true, ICMP: true, TLS: true, DTLS: true, SSH: true, HTTP: true, HTTPS: true}

// TLSOptions tunes the certificate verification of the tls, dtls and https
// checks. It is set through the API only, never read from a bundle file.
type TLSOptions struct {
	ServerName         string         // name to verify; default: the host of the address
	RootCAs            *x509.CertPool // trust anchors; default: the system roots
	InsecureSkipVerify bool           // skip verification; the result records Verified: false
}

// Check is one thing to test.
type Check struct {
	Name     string   `json:"name,omitempty" yaml:"name,omitempty"`
	Address  string   `json:"address,omitempty" yaml:"address,omitempty"` // host:port; a bare host for icmp; host[:port][/path] for http and https
	Protocol Protocol `json:"protocol" yaml:"protocol"`
	Timeout  Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"` // per attempt; 0 means the bundle's or the default
	Retries  int      `json:"retries,omitempty" yaml:"retries,omitempty"` // attempts in total; 0 means the bundle's or the default
	Wait     Duration `json:"wait,omitempty" yaml:"wait,omitempty"`       // between attempts; 0 means the bundle's or the default
	SSO      bool     `json:"sso" yaml:"sso"`                             // http and https: SPNEGO authentication, needs the sso sub-package

	TLS            *TLSOptions `json:"-" yaml:"-"`
	AllowDowngrade bool        `json:"-" yaml:"-"` // http and https: let a redirect leave https for http
}

// A check read from a file without a protocol is a tcp check, as it always was.
// Checks built in Go have no such default: Validate rejects them.

// UnmarshalJSON reads a Check, defaulting the protocol to tcp.
func (c *Check) UnmarshalJSON(data []byte) error {
	type plain Check
	v := plain{Protocol: TCP}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*c = Check(v)
	return nil
}

// UnmarshalYAML reads a Check, defaulting the protocol to tcp.
func (c *Check) UnmarshalYAML(node *yaml.Node) error {
	type plain Check
	v := plain{Protocol: TCP}
	if err := node.Decode(&v); err != nil {
		return err
	}
	*c = Check(v)
	return nil
}

// Bundle is a set of checks with the settings they share.
type Bundle struct {
	ID          string   `json:"id,omitempty" yaml:"id,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Timeout     Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Retries     int      `json:"retries,omitempty" yaml:"retries,omitempty"`
	Wait        Duration `json:"wait,omitempty" yaml:"wait,omitempty"`
	Concurrency int      `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
	Checks      []Check  `json:"checks,omitempty" yaml:"checks,omitempty"`
}

// PingOptions tunes the icmp check.
type PingOptions struct {
	Count      int      // echo requests per attempt
	Interval   Duration // between requests
	Size       int      // payload bytes
	Privileged bool     // raw sockets (needs cap_net_raw or root); otherwise unprivileged ICMP datagram sockets
}

// Defaults holds the settings a Check or a Bundle leaves at zero.
type Defaults struct {
	Timeout     Duration
	Retries     int
	Wait        Duration
	Concurrency int
	UDPGrace    Duration // how long a udp check waits for an ICMP error to come back
	Ping        PingOptions
}

// DefaultDefaults returns the compiled-in defaults.
func DefaultDefaults() Defaults {
	return Defaults{
		Timeout:     Duration(20 * time.Second),
		Retries:     3,
		Wait:        Duration(time.Second),
		Concurrency: 10,
		UDPGrace:    Duration(200 * time.Millisecond),
		Ping:        PingOptions{Count: 10, Interval: Duration(100 * time.Millisecond), Size: 64},
	}
}
