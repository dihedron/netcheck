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

func (p Protocol) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

func (p *Protocol) UnmarshalJSON(data []byte) (err error) {
	var proto string
	if err := json.Unmarshal(data, &proto); err != nil {
		return err
	}
	switch proto {
	case "tcp":
		*p = TCP
	case "udp":
		*p = UDP
	case "icmp":
		*p = ICMP
	default:
		return fmt.Errorf("unsupported value: '%s'", string(data))
	}
	return nil
}

func (p Protocol) MarshalYAML() ([]byte, error) {
	return []byte(p.String()), nil
}

func (p *Protocol) UnmarshalYAML(node *yaml.Node) error {
	switch node.Value {
	case "tcp":
		*p = TCP
	case "udp":
		*p = UDP
	case "icmp":
		*p = ICMP
	default:
		return fmt.Errorf("unsupported value: '%s'", node.Value)
	}
	return nil
}

func (p Protocol) MarshalText() (text []byte, err error) {
	return []byte(p.String()), nil
}

func (p *Protocol) UnmarshalText(text []byte) error {
	switch string(text) {
	case "tcp":
		*p = TCP
	case "udp":
		*p = UDP
	case "icmp":
		*p = ICMP
	default:
		return fmt.Errorf("unsupported value: '%s'", text)
	}
	return nil
}
