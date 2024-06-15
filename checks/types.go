package checks

import (
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Timeout wraps the native time.Duration time, adding JSON, YAML and TOML
// marshalling/unmarshalling capabilities.
type Timeout time.Duration

// String returns a string representation of the Timeout.
func (t Timeout) String() string {
	return time.Duration(t).String()
}

// MarshalJSON marshals the Timeout struct to JSON.
func (t Timeout) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// UnmarshalJSON unmarshals the Timeout struct from JSON.
func (t *Timeout) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	*t = Timeout(d)
	return nil
}

// MarshalYAML marshals the Timeout struct to YAML.
func (t Timeout) MarshalYAML() (any, error) {
	return t.String(), nil
}

// UnmarshalYAML unmarshals the Timeout struct from YAML.
func (t *Timeout) UnmarshalYAML(node *yaml.Node) (err error) {
	d, err := time.ParseDuration(node.Value)
	if err != nil {
		return err
	}
	*t = Timeout(d)
	return nil
}

// MarshalText marshals the Timeout struct to text.
func (t Timeout) MarshalText() (text []byte, err error) {
	return []byte(t.String()), nil
}

// UnmarshalText unmarshals the Timeout struct from text.
func (t *Timeout) UnmarshalText(text []byte) error {
	d, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*t = Timeout(d)
	return nil
}

// Protocol represents the supported protocols.
type Protocol uint8

const (
	TCP Protocol = iota
	UDP
	ICMP
	TLS
	DTLS // TLS over UDP
	SSH
)

// String returns a string representation of the Protocol.
func (p Protocol) String() string {
	return []string{"tcp", "udp", "icmp", "tls", "dtls", "ssh"}[p]
}

// FromString returns the Protocol value corresponding to the given string representation.
func (p *Protocol) FromString(value string) error {
	switch value {
	case "tcp":
		*p = TCP
	case "udp":
		*p = UDP
	case "icmp":
		*p = ICMP
	case "tls":
		*p = TLS
	case "dtls":
		*p = DTLS
	case "ssh":
		*p = SSH
	default:
		return fmt.Errorf("unsupported value: '%s'", value)
	}
	return nil
}

// MarshalJSON marshals the Protocol struct to JSON.
func (p Protocol) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

// UnmarshalJSON marshals the Timeout struct from JSON.
func (p *Protocol) UnmarshalJSON(data []byte) (err error) {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	return p.FromString(value)
}

// MarshalYAML marshals the Timeout struct to YAML.
func (p Protocol) MarshalYAML() (any, error) {
	return p.String(), nil
}

// UnmarshalYAML marshals the Timeout struct from YAML.
func (p *Protocol) UnmarshalYAML(node *yaml.Node) error {
	return p.FromString(node.Value)
}

// MarshalText marshals the Timeout struct to text.
func (p Protocol) MarshalText() (text []byte, err error) {
	return []byte(p.String()), nil
}

// UnmarshalText marshals the Timeout struct from text.
func (p *Protocol) UnmarshalText(text []byte) error {
	return p.FromString(string(text))
}

// Result represents the result of a check.
type Result struct {
	err error
}

// IsError returns whether the Result represents an error.
func (r Result) IsError() bool {
	return r.err != nil
}

// String returns a string representation of the Result.
func (r Result) String() string {
	if r.IsError() {
		return r.err.Error()
	}
	return "success"
}

// MarshalJSON produces the JSON value for the Result.
func (r Result) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String())
}

// MarshalYAML returns the YAML value for the Result.
func (r Result) MarshalYAML() (any, error) {
	return r.String(), nil
}
