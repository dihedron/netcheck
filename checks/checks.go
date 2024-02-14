package checks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	probing "github.com/prometheus-community/pro-bing"
	"gopkg.in/yaml.v3"
)

const (
	DefaultTimeout     = 20 * time.Second
	DefaultParallelism = 10
)

type Package struct {
	ID          string        `json:"id,omitempty" yaml:"id,omitempty" toml:"id"`
	Description string        `json:"description,omitempty" yaml:"description,omitempty" toml:"description"`
	Timeout     time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty" toml:"timeout"`
	Parallelism int           `json:"parallelism,omitempty" yaml:"parallelism,omitempty" toml:"parallelism"`
	Checks      []Check       `json:"checks,omitempty" yaml:"checks,omitempty" toml:"checks"`
}

type Result struct {
	Protocol string
	Endpoint string
	Success  bool
}

type DataType int8

const (
	YAML DataType = iota
	JSON
	TOML
)

func New(path string) (*Package, error) {

	var (
		data     []byte
		err      error
		datatype DataType
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
			datatype = JSON
		case "application/x-yaml", "text/yaml":
			datatype = YAML
		case "application/toml":
			datatype = TOML
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
			datatype = YAML
		case ".json":
			datatype = JSON
		case ".toml":
			datatype = TOML
		}
	}

	pkg := &Package{
		Timeout:     DefaultTimeout,
		Parallelism: DefaultParallelism,
	}

	switch datatype {
	case YAML:
		err := yaml.Unmarshal(data, pkg)
		if err != nil {
			slog.Error("error parsing checks package", "format", "yaml", "error", err)
			os.Exit(1)
		}
	case JSON:
		err := json.Unmarshal(data, pkg)
		if err != nil {
			slog.Error("error parsing checks package", "format", "json", "error", err)
			os.Exit(1)
		}
	case TOML:
		err := toml.Unmarshal(data, pkg)
		if err != nil {
			slog.Error("error parsing checks package", "format", "json", "error", err)
			os.Exit(1)
		}
	}

	// // fmt.Printf("%s\n", pkg.ToYAML())

	return pkg, nil
}

func (p *Package) ToJSON() string {
	data, _ := json.MarshalIndent(p, "  ", "")
	return string(data)
}

func (p *Package) ToYAML() string {
	data, _ := yaml.Marshal(p)
	return string(data)
}

func (p *Package) ToTOML() string {
	data, _ := toml.Marshal(p)
	return string(data)
}

func (p *Package) Check() []Result {
	checks := make(chan Check, len(p.Checks))
	results := make(chan Result, len(p.Checks))

	// launch the thread pool
	for id := 1; id <= p.Parallelism; id++ {
		go worker(id, checks, results)
	}

	// submit the checks
	for _, check := range p.Checks {
		if check.Timeout == 0 {
			check.Timeout = p.Timeout
		}
		if check.Protocol == "" {
			check.Protocol = "tcp"
		}
		checks <- check
	}
	close(checks)

	// collect the results
	array := []Result{}
	for range len(p.Checks) {
		result := <-results
		array = append(array, result)
	}

	return array
}

type Check struct {
	Name     string        `json:"name,omitempty" yaml:"name,omitempty" toml:"name"`
	Timeout  time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty" toml:"timeout"`
	Address  string        `json:"address,omitempty" yaml:"address,omitempty" toml:"address"`
	Port     uint16        `json:"port,omitempty" yaml:"port,omitempty" toml:"port"`
	Protocol string        `json:"protocol,omitempty" yaml:"protocol,omitempty" toml:"protocol"`
}

func (c *Check) Do() bool {
	switch c.Protocol {
	case "tcp", "udp":
		var dialer net.Dialer
		ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
		defer cancel()
		conn, err := dialer.DialContext(ctx, c.Protocol, fmt.Sprintf("%s:%d", c.Address, c.Port))
		if err != nil {
			return false
		}
		defer conn.Close()
	case "icmp":
		pinger, err := probing.NewPinger(c.Address)
		if err != nil {
			return false
		}
		pinger.Timeout = c.Timeout
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
			return false
		}
	default:
		// same as tcp
		var dialer net.Dialer
		ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
		defer cancel()
		conn, err := dialer.DialContext(ctx, c.Protocol, fmt.Sprintf("%s:%d", c.Address, c.Port))
		if err != nil {
			return false
		}
		defer conn.Close()
	}
	return true
}

func worker(id int, check <-chan Check, results chan<- Result) {
	for check := range check {
		result := Result{
			Protocol: check.Protocol,
			Success:  check.Do(),
		}
		switch check.Protocol {
		case "icmp":
			result.Endpoint = check.Address
		default:
			result.Endpoint = fmt.Sprintf("%s:%d", check.Address, check.Port)
		}
		results <- result
	}
}
