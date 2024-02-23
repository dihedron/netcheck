package checks

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	probing "github.com/prometheus-community/pro-bing"
	"gopkg.in/yaml.v3"
)

const (
	DefaultTimeout     = Timeout(20 * time.Second)
	DefaultParallelism = 10
)

type Bundle struct {
	ID          string  `json:"id,omitempty" yaml:"id,omitempty" toml:"id"`
	Description string  `json:"description,omitempty" yaml:"description,omitempty" toml:"description"`
	Timeout     Timeout `json:"timeout,omitempty" yaml:"timeout,omitempty" toml:"timeout"`
	Parallelism int     `json:"parallelism,omitempty" yaml:"parallelism,omitempty" toml:"parallelism"`
	Checks      []Check `json:"checks,omitempty" yaml:"checks,omitempty" toml:"checks"`
}

type Result struct {
	Protocol Protocol `json:"protocol" yaml:"protocol" toml:"protocol"`
	Endpoint string   `json:"endpoint,omitempty" yaml:"endpoint,omitempty" toml:"endpoint"`
	Code     Code     `json:"code" yaml:"code" toml:"code"`
	Actions  []Action `json:"actions,omitempty" yaml:"actions,omitempty" toml:"actions"`
}

type Code int

const (
	ConnectionOK Code = iota
	ConnectionFailed
	HostnameMismatch
	CertificateExpired
)

type Format int8

const (
	YAML Format = iota
	JSON
	TOML
)

func New(path string) (*Bundle, error) {

	var (
		data   []byte
		err    error
		format Format
	)

	if strings.HasPrefix("http://", path) || strings.HasPrefix("https://", path) {
		// retrieve from URL
		resp, err := http.Get(path)
		if err != nil {
			slog.Error("error downloading package from URL", "url", path, "error", err)
			return nil, err
		}
		defer resp.Body.Close()

		var buffer bytes.Buffer
		_, err = io.Copy(&buffer, resp.Body)
		if err != nil {
			slog.Error("error reading package body from URL", "url", path, "error", err)
			return nil, err
		}

		data = buffer.Bytes()

		switch resp.Header.Get("Content-Type") {
		case "application/json":
			format = JSON
		case "application/x-yaml", "text/yaml":
			format = YAML
		case "application/toml":
			format = TOML
		}
	} else {
		// read from file on disk
		data, err = os.ReadFile(path)
		if err != nil {
			slog.Error("error reading package from file", "path", path, "error", err)
			return nil, err
		}

		switch strings.ToLower(filepath.Ext(path)) {
		case ".yaml", ".yml":
			format = YAML
		case ".json":
			format = JSON
		case ".toml":
			format = TOML
		}
	}

	bundle := &Bundle{
		Timeout:     DefaultTimeout,
		Parallelism: DefaultParallelism,
	}

	switch format {
	case YAML:
		err := yaml.Unmarshal(data, bundle)
		if err != nil {
			slog.Error("error parsing checks package", "format", "yaml", "error", err)
			os.Exit(1)
		}
	case JSON:
		err := json.Unmarshal(data, bundle)
		if err != nil {
			slog.Error("error parsing checks package", "format", "json", "error", err)
			os.Exit(1)
		}
	case TOML:
		err := toml.Unmarshal(data, bundle)
		if err != nil {
			slog.Error("error parsing checks package", "format", "toml", "error", err)
			os.Exit(1)
		}
	}

	// // fmt.Printf("%s\n", bundle.ToYAML())

	return bundle, nil
}

func (b *Bundle) ToJSON() string {
	data, _ := json.MarshalIndent(b, "  ", "")
	return string(data)
}

func (b *Bundle) ToYAML() string {
	data, _ := yaml.Marshal(b)
	return string(data)
}

func (b *Bundle) ToTOML() string {
	data, _ := toml.Marshal(b)
	return string(data)
}

func (b *Bundle) Check(withTriggers bool) []Result {
	checks := make(chan Check, len(b.Checks))
	results := make(chan Result, len(b.Checks))

	// launch the thread pool
	for id := 1; id <= b.Parallelism; id++ {
		go worker(id, withTriggers, checks, results)
	}

	// submit the checks
	for _, check := range b.Checks {
		if check.Timeout == 0 {
			check.Timeout = b.Timeout
		}
		checks <- check
	}
	close(checks)

	// collect the results
	array := []Result{}
	for range len(b.Checks) {
		result := <-results
		array = append(array, result)
	}

	return array
}

type Check struct {
	Name     string    `json:"name,omitempty" yaml:"name,omitempty" toml:"name"`
	Timeout  Timeout   `json:"timeout,omitempty" yaml:"timeout,omitempty" toml:"timeout"`
	Retries  int       `json:"retries,omitempty" yaml:"retries,omitempty" toml:"retries"`
	Wait     Timeout   `json:"wait,omitempty" yaml:"wait,omitempty" toml:"wait"`
	Address  string    `json:"address,omitempty" yaml:"address,omitempty" toml:"address"`
	Protocol Protocol  `json:"protocol,omitempty" yaml:"protocol,omitempty" toml:"protocol"`
	Triggers []Trigger `json:"triggers,omitempty" yaml:"triggers,omitempty" toml:"triggers"`
}

func (c *Check) Do() Code {
	var protocol string
	switch c.Protocol {
	case TCP, UDP:
		var dialer net.Dialer
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.Timeout))
		defer cancel()
		conn, err := dialer.DialContext(ctx, c.Protocol.String(), c.Address)
		if err != nil {
			slog.Error("error dialling", "address", c.Address, "protocol", c.Protocol.String(), "error", err)
			return ConnectionFailed
		}
		defer conn.Close()
		slog.Info("successfully tested connection", "address", c.Address, "protocol", c.Protocol.String())
	case DTLS:
		protocol = "udp"
		fallthrough
	case TLS:
		if protocol != "udp" {
			protocol = "tcp"
		}
		dialer := &net.Dialer{
			Timeout: time.Duration(c.Timeout),
		}
		conn, err := tls.DialWithDialer(dialer, protocol, c.Address, nil)
		if err != nil {
			slog.Error("error dialling", "address", c.Address, "protocol", c.Protocol.String(), "error", err)
			return ConnectionFailed
		}
		defer conn.Close()
		err = conn.VerifyHostname(strings.Split(c.Address, ":")[0])
		if err != nil {
			slog.Error("hostname does not match certificate", "hostname", strings.Split(c.Address, ":")[0], "error", err)
			return HostnameMismatch
		}
		expiry := conn.ConnectionState().PeerCertificates[0].NotAfter
		issuer := conn.ConnectionState().PeerCertificates[0].Issuer
		if time.Now().After(expiry) {
			slog.Error("certificate has expired", "expiry", expiry.Format(time.RFC3339))
			return CertificateExpired
		}
		slog.Info("successfully tested connection", "address", c.Address, "protocol", c.Protocol.String(), "certificate issuer", issuer, "certificate expiry", expiry.Format(time.RFC3339))
	case ICMP:
		pinger, err := probing.NewPinger(c.Address)
		if err != nil {
			slog.Error("error creating ICMP client", "address", c.Address, "protocol", c.Protocol.String(), "error", err)
			return ConnectionFailed
		}
		pinger.Timeout = time.Duration(c.Timeout)
		pinger.Count = 10
		pinger.Interval = 100 * time.Microsecond
		pinger.Size = 64

		pinger.OnRecv = func(pkt *probing.Packet) {
			slog.Debug("received ping response", "bytes", pkt.Nbytes, "endpoint", pkt.IPAddr, "sequence", pkt.Seq, "rtt", pkt.Rtt, "ttl", pkt.TTL)
		}

		pinger.OnDuplicateRecv = func(pkt *probing.Packet) {
			slog.Debug("received duplicate ping response", "bytes", pkt.Nbytes, "endpoint", pkt.IPAddr, "sequence", pkt.Seq, "rtt", pkt.Rtt, "ttl", pkt.TTL)
		}

		pinger.OnFinish = func(stats *probing.Statistics) {
			slog.Debug("ping statistics", "destination", stats.Addr, "transmitted", stats.PacketsSent, "received", stats.PacketsRecv, "loss_percent", stats.PacketLoss, "roundtrip_min", stats.MinRtt, "roundtrip_avg", stats.AvgRtt, "roundtrip_max", stats.MaxRtt, "roundtrip_stddev", stats.StdDevRtt)
		}

		err = pinger.Run()
		if err != nil {
			slog.Error("error running ping", "endpoint", c.Address, "protocol", c.Protocol.String(), "error", err)
			return ConnectionFailed
		}
		slog.Info("successfully tested connection", "address", c.Address, "protocol", c.Protocol.String())
	}
	return ConnectionOK
}

type Trigger struct {
	On      Event    `json:"on" yaml:"on" toml:"on"`
	Command string   `json:"command,omitempty" yaml:"command,omitempty" toml:"command"`
	Args    []string `json:"args,omitempty" yaml:"args,omitempty" toml:"args"`
	Timeout Timeout  `json:"timeout,omitempty" yaml:"timeout,omitempty" toml:"timeout"`
}

type Action struct {
	Command  []string `json:"command,omitempty" yaml:"command,omitempty" toml:"command"`
	ExitCode int      `json:"exitcode" yaml:"exitcode" toml:"exitcode"`
	Stdout   string   `json:"stdout,omitempty" yaml:"stdout,omitempty" toml:"stdout"`
	Stderr   string   `json:"stderr,omitempty" yaml:"stderr,omitempty" toml:"stderr"`
}

func (t Trigger) Execute() (*Action, error) {
	var cmd *exec.Cmd

	if strings.HasPrefix(strings.TrimLeft(t.Command, " \t\n\r"), "#!") {
		slog.Debug("running a script")
		_, ok := os.LookupEnv("SHELL")
		if !ok {
			slog.Error("no valid SHELL in environment")
			return nil, fmt.Errorf("no valid SHELL value in environment")
		}
		// TODO: write script to temp file, then call SHELL on it, defer remove temp file
	}

	if t.Timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(t.Timeout))
		defer cancel()
		cmd = exec.CommandContext(ctx, t.Command, t.Args...)
	} else {
		cmd = exec.Command(t.Command, t.Args...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		slog.Error("error running command", "command", t.Command, "args", t.Args, "error", err)
		return nil, err
	}
	return &Action{
		Command:  append([]string{t.Command}, t.Args...),
		ExitCode: cmd.ProcessState.ExitCode(),
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}

func worker(id int, withTriggers bool, check <-chan Check, results chan<- Result) {
	for check := range check {
		result := Result{
			Endpoint: check.Address,
			Protocol: check.Protocol,
			Code:     check.Do(),
		}
		if withTriggers {
			result.Actions = []Action{}
			for _, trigger := range check.Triggers {
				if (trigger.On == Success && result.Code == ConnectionOK) || (trigger.On == Failure && !(result.Code == ConnectionOK)) || (trigger.On == Always) {
					action, err := trigger.Execute()
					if err != nil {
						slog.Error("error executing trigger", "command", trigger.Command, "args", trigger.Args, "error", err)
						continue
					}
					result.Actions = append(result.Actions, *action)
				}
			}
		}
		results <- result
	}
}
