package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/pelletier/go-toml/v2"
	probing "github.com/prometheus-community/pro-bing"
	"gopkg.in/yaml.v3"
)

const (
	DefaultTimeout     = 20 * time.Second
	DefaultParallelism = 10
)

func init() {
	options := &slog.HandlerOptions{
		Level:     slog.LevelWarn,
		AddSource: true,
	}

	level, ok := os.LookupEnv("NETCHECK_LOG_LEVEL")
	if ok {
		switch strings.ToLower(level) {
		case "debug", "dbg", "d", "trace", "trc", "t":
			options.Level = slog.LevelDebug
		case "informational", "info", "inf", "i":
			options.Level = slog.LevelInfo
		case "warning", "warn", "wrn", "w":
			options.Level = slog.LevelWarn
		case "error", "err", "e", "fatal", "ftl", "f":
			options.Level = slog.LevelError
		}
	}
	handler := slog.NewTextHandler(os.Stderr, options)
	slog.SetDefault(slog.New(handler))
}

type Endpoint struct {
	Timeout  time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty" toml:"timeout"`
	Address  string        `json:"address,omitempty" yaml:"address,omitempty" toml:"address"`
	Port     uint16        `json:"port,omitempty" yaml:"port,omitempty" toml:"port"`
	Protocol string        `json:"protocol,omitempty" yaml:"protocol,omitempty" toml:"protocol"`
}

func (e *Endpoint) Do() bool {
	switch e.Protocol {
	case "tcp", "udp":
		var dialer net.Dialer
		ctx, cancel := context.WithTimeout(context.Background(), e.Timeout)
		defer cancel()
		conn, err := dialer.DialContext(ctx, e.Protocol, fmt.Sprintf("%s:%d", e.Address, e.Port))
		if err != nil {
			return false
		}
		defer conn.Close()
	case "icmp":
		pinger, err := probing.NewPinger(e.Address)
		if err != nil {
			return false
		}
		pinger.Timeout = e.Timeout
		pinger.Count = 10
		pinger.Interval = 100 * time.Microsecond

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
		ctx, cancel := context.WithTimeout(context.Background(), e.Timeout)
		defer cancel()
		conn, err := dialer.DialContext(ctx, e.Protocol, fmt.Sprintf("%s:%d", e.Address, e.Port))
		if err != nil {
			return false
		}
		defer conn.Close()
	}
	return true
}

type Configuration struct {
	Timeout     time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty" toml:"timeout"`
	Parallelism int           `json:"parallelism,omitempty" yaml:"parallelism,omitempty" toml:"parallelism"`
	Endpoints   []Endpoint    `json:"endpoints,omitempty" yaml:"endpoints,omitempty" toml:"endpoints"`
}

func (c *Configuration) ToJSON() string {
	data, _ := json.MarshalIndent(c, "  ", "")
	return string(data)
}

func (c *Configuration) ToYAML() string {
	data, _ := yaml.Marshal(c)
	return string(data)
}

func (c *Configuration) ToTOML() string {
	data, _ := toml.Marshal(c)
	return string(data)
}

type Result struct {
	Endpoint string
	Success  bool
}

var (
	red   = color.New(color.FgRed).FprintfFunc()
	green = color.New(color.FgGreen).FprintfFunc()
)

func main() {

	if len(os.Args) != 2 {
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		slog.Error("error opening input file", "path", os.Args[1], "error", err)
		os.Exit(1)
	}

	configuration := &Configuration{
		Timeout:     DefaultTimeout,
		Parallelism: DefaultParallelism,
	}

	switch strings.ToLower(filepath.Ext(os.Args[1])) {
	case ".yaml", ".yml":
		err := yaml.Unmarshal(data, configuration)
		if err != nil {
			slog.Error("error parsing endpoints configuration", "format", "yaml", "error", err)
			os.Exit(1)
		}
	case ".json":
		err := json.Unmarshal(data, configuration)
		if err != nil {
			slog.Error("error parsing endpoints configuration", "format", "json", "error", err)
			os.Exit(1)
		}
	case ".toml":
		err := toml.Unmarshal(data, configuration)
		if err != nil {
			slog.Error("error parsing endpoints configuration", "format", "json", "error", err)
			os.Exit(1)
		}
	}

	// fmt.Printf("%s\n", configuration.ToYAML())

	endpoints := make(chan Endpoint, len(configuration.Endpoints))
	results := make(chan Result, len(configuration.Endpoints))

	for id := 1; id <= configuration.Parallelism; id++ {
		go worker(id, endpoints, results)
	}

	for _, endpoint := range configuration.Endpoints {
		if endpoint.Timeout == 0 {
			endpoint.Timeout = configuration.Timeout
		}
		if endpoint.Protocol == "" {
			endpoint.Protocol = "tcp"
		}
		endpoints <- endpoint
	}
	close(endpoints)

	for a := 1; a <= len(configuration.Endpoints); a++ {
		result := <-results
		if result.Success {
			if isatty.IsTerminal(os.Stdout.Fd()) {
				green(os.Stdout, "%s\n", result.Endpoint)
			} else {
				fmt.Printf("%s: ok\n", result.Endpoint)
			}
		} else {
			if isatty.IsTerminal(os.Stdout.Fd()) {
				red(os.Stdout, "%s\n", result.Endpoint)
			} else {
				fmt.Printf("%s: ko\n", result.Endpoint)
			}
		}
	}
}

func worker(id int, endpoints <-chan Endpoint, results chan<- Result) {
	for endpoint := range endpoints {
		ok := endpoint.Do()
		results <- Result{
			Endpoint: fmt.Sprintf("%s/%s:%d", endpoint.Protocol, endpoint.Address, endpoint.Port),
			Success:  ok,
		}
	}
}
