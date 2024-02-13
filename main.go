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

	level, ok := os.LookupEnv("NETPROBE_LOG_LEVEL")
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
	Timeout  *time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty" toml:"timeout"`
	Address  string         `json:"address,omitempty" yaml:"address,omitempty" toml:"address"`
	Port     uint16         `json:"port,omitempty" yaml:"port,omitempty" toml:"port"`
	Protocol *string        `json:"protocol,omitempty" yaml:"protocol,omitempty" toml:"protocol"`
	ok       bool
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
	results := make(chan Endpoint, len(configuration.Endpoints))

	for w := 1; w <= configuration.Parallelism; w++ {
		go worker(w, configuration.Timeout, endpoints, results)
	}

	for _, endpoint := range configuration.Endpoints {
		endpoints <- endpoint
	}
	close(endpoints)

	for a := 1; a <= len(configuration.Endpoints); a++ {
		endpoint := <-results
		protocol := "tcp"
		if endpoint.Protocol != nil {
			protocol = *endpoint.Protocol
		}
		if endpoint.ok {
			if isatty.IsTerminal(os.Stdout.Fd()) {
				green(os.Stdout, "%s: %s:%d\n", strings.ToUpper(protocol), endpoint.Address, endpoint.Port)
			} else {
				fmt.Printf("%s: %s:%d -> ok\n", strings.ToUpper(protocol), endpoint.Address, endpoint.Port)
			}
		} else {
			if isatty.IsTerminal(os.Stdout.Fd()) {
				red(os.Stdout, "%s: %s:%d\n", strings.ToUpper(protocol), endpoint.Address, endpoint.Port)
			} else {
				fmt.Printf("%s: %s:%d -> ko\n", strings.ToUpper(protocol), endpoint.Address, endpoint.Port)
			}
		}
	}
}

func worker(id int, timeout time.Duration, endpoints <-chan Endpoint, results chan<- Endpoint) {
	for endpoint := range endpoints {
		endpoint.ok = func(endpoint Endpoint) bool {
			if endpoint.Timeout != nil {
				timeout = *endpoint.Timeout
			}
			var d net.Dialer
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			protocol := "tcp"
			if endpoint.Protocol != nil {
				protocol = *endpoint.Protocol
			}

			conn, err := d.DialContext(ctx, protocol, fmt.Sprintf("%s:%d", endpoint.Address, endpoint.Port))
			if err != nil {
				// slog.Error("error connecting to remote address", "address", endpoint.Address, "port", endpoint.Port, "protocol", endpoint.Protocol)
				return false
			}
			defer conn.Close()
			return true
		}(endpoint)
		results <- endpoint
	}
}
