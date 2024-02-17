package checks

import (
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

type Timeout time.Duration

func (t Timeout) String() string {
	return time.Duration(t).String()
}

func (t Timeout) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

func (t *Timeout) UnmarshalJSON(data []byte) (err error) {
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

func (t Timeout) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(t.String())
}

func (t *Timeout) UnmarshalYAML(node *yaml.Node) (err error) {
	d, err := time.ParseDuration(node.Value)
	if err != nil {
		return err
	}
	*t = Timeout(d)
	return nil
}

func (t Timeout) MarshalText() (text []byte, err error) {
	return []byte(t.String()), nil
}

func (t *Timeout) UnmarshalText(text []byte) error {
	d, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*t = Timeout(d)
	return nil
}

type Protocol uint8

const (
	TCP Protocol = iota
	UDP
	ICMP
)

func (p Protocol) String() string {
	return []string{"tcp", "udp", "icmp"}[p]
}

func (p *Protocol) FromString(value string) error {
	switch value {
	case "tcp":
		*p = TCP
	case "udp":
		*p = UDP
	case "icmp":
		*p = ICMP
	default:
		return fmt.Errorf("unsupported value: '%s'", value)
	}
	return nil
}

func (p Protocol) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

func (p *Protocol) UnmarshalJSON(data []byte) (err error) {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	return p.FromString(value)
}

func (p Protocol) MarshalYAML() ([]byte, error) {
	return []byte(p.String()), nil
}

func (p *Protocol) UnmarshalYAML(node *yaml.Node) error {
	return p.FromString(node.Value)
}

func (p Protocol) MarshalText() (text []byte, err error) {
	return []byte(p.String()), nil
}

func (p *Protocol) UnmarshalText(text []byte) error {
	return p.FromString(string(text))
}

type Event uint8

const (
	Success Event = iota
	Failure
	Always
)

func (e Event) String() string {
	return []string{"success", "failure"}[e]
}

func (e *Event) FromString(value string) error {
	switch value {
	case "success":
		*e = Success
	case "failure":
		*e = Failure
	default:
		return fmt.Errorf("unsupported value: '%s'", value)
	}
	return nil
}

func (e Event) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

func (e *Event) UnmarshalJSON(data []byte) (err error) {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	return e.FromString(value)
}

func (e Event) MarshalYAML() ([]byte, error) {
	return []byte(e.String()), nil
}

func (e *Event) UnmarshalYAML(node *yaml.Node) error {
	return e.FromString(node.Value)
}

func (e Event) MarshalText() (text []byte, err error) {
	return []byte(e.String()), nil
}

func (e *Event) UnmarshalText(text []byte) error {
	return e.FromString(string(text))
}
