package checks

import (
	"log/slog"
	"os"
	"time"

	"github.com/dihedron/netcheck/pointer"
	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v3"
)

const (
	DefaultTimeout      = Timeout(20 * time.Second)
	DefaultRetries      = 3
	DefaultWait         = Timeout(1 * time.Second)
	DefaultConcurrency  = 10
	DefaultPingTimeout  = Timeout(1 * time.Second)
	DefaultPingCount    = 10
	DefaultPingInterval = Timeout(100 * time.Millisecond)
	DefaultPingSize     = 64
)

type Defaults struct {
	Timeout     *Timeout `yaml:"timeout"`
	Retries     *int     `yaml:"retries"`
	Wait        *Timeout `yaml:"wait"`
	Concurrency *int     `yaml:"concurrency"`
	Ping        *struct {
		Count    *int     `yaml:"count"`
		Interval *Timeout `yaml:"interval"`
		Size     *int     `yaml:"size"`
	} `yaml:"ping"`
}

var Default *Defaults

func loadDefaultsFrom(path string) error {
	path, err := homedir.Expand(path) // (string, error)
	if err != nil {
		slog.Error("error resolving user's home directory", "path", path, "error", err)
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("file does not exist", "path", path, "error", err)
			return err
		} else {
			slog.Error("error reading defaults file", "path", path, "error", err)
			return err
		}
	}

	d := &Defaults{}
	err = yaml.Unmarshal(data, d)
	if err != nil {
		slog.Error("error unmarshalling defaults", "path", path, "data", data, "error", err)
		return err
	}
	// store into the current Defaults, then check
	// that all values are provided and fill in
	Default = d
	if Default.Timeout == nil {
		Default.Timeout = pointer.To(DefaultTimeout)
	}
	if Default.Retries == nil {
		Default.Retries = pointer.To(DefaultRetries)
	}
	if Default.Wait == nil {
		Default.Wait = pointer.To(DefaultWait)
	}
	if Default.Concurrency == nil {
		Default.Concurrency = pointer.To(DefaultConcurrency)
	}
	if Default.Ping == nil {
		Default.Ping = &struct {
			Count    *int     `yaml:"count"`
			Interval *Timeout `yaml:"interval"`
			Size     *int     `yaml:"size"`
		}{}
	}
	if Default.Ping.Count == nil {
		Default.Ping.Count = pointer.To(DefaultPingCount)
	}
	if Default.Ping.Interval == nil {
		Default.Ping.Interval = pointer.To(DefaultPingInterval)
	}
	if Default.Ping.Size == nil {
		Default.Ping.Size = pointer.To(DefaultPingSize)
	}
	return nil
}
