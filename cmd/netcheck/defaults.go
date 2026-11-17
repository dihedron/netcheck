package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	"gopkg.in/yaml.v3"
)

// defaultsPaths are the places the defaults file is looked for, in order.
var defaultsPaths = []string{"./netcheck.conf", "~/netcheck.conf", "/etc/netcheck.conf"}

// conf is the defaults file: every key is optional.
type conf struct {
	Timeout     *netcheck.Duration `yaml:"timeout"`
	Retries     *int               `yaml:"retries"`
	Wait        *netcheck.Duration `yaml:"wait"`
	Concurrency *int               `yaml:"concurrency"`
	Ping        *struct {
		Count      *int               `yaml:"count"`
		Interval   *netcheck.Duration `yaml:"interval"`
		Size       *int               `yaml:"size"`
		Privileged *bool              `yaml:"privileged"`
	} `yaml:"ping"`
}

// loadDefaults returns the library defaults overridden by the first file of
// paths that exists. A file that exists and cannot be read is an error. Raw
// ICMP sockets are the default on Linux and Windows, where the package gives
// the binary the capability for them.
func loadDefaults(paths []string) (netcheck.Defaults, error) {
	d := netcheck.DefaultDefaults()
	d.Ping.Privileged = runtime.GOOS == "linux" || runtime.GOOS == "windows"
	for _, path := range paths {
		path = expandHome(path)
		data, err := os.ReadFile(filepath.Clean(path))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return d, fmt.Errorf("reading %s: %w", path, err)
		}
		var c conf
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(&c); err != nil {
			return d, fmt.Errorf("parsing %s: %w", path, err)
		}
		apply(&d, c)
		return d, nil
	}
	return d, nil
}

func apply(d *netcheck.Defaults, c conf) {
	if c.Timeout != nil {
		d.Timeout = *c.Timeout
	}
	if c.Retries != nil {
		d.Retries = *c.Retries
	}
	if c.Wait != nil {
		d.Wait = *c.Wait
	}
	if c.Concurrency != nil {
		d.Concurrency = *c.Concurrency
	}
	if p := c.Ping; p != nil {
		if p.Count != nil {
			d.Ping.Count = *p.Count
		}
		if p.Interval != nil {
			d.Ping.Interval = *p.Interval
		}
		if p.Size != nil {
			d.Ping.Size = *p.Size
		}
		if p.Privileged != nil {
			d.Ping.Privileged = *p.Privileged
		}
	}
}

// expandHome replaces a leading "~/" with the user's home directory.
func expandHome(path string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	return path
}
