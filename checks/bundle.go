package checks

import (
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/dihedron/netcheck/fetch"
	"github.com/dihedron/netcheck/format"
	"gopkg.in/yaml.v3"
)

const (
	DefaultTimeout     = Timeout(20 * time.Second)
	DefaultRetries     = 3
	DefaultWait        = Timeout(1 * time.Second)
	DefaultConcurrency = 10
)

// Bundle represents a consistent set of checks, with some package-level defaults.
type Bundle struct {
	ID          string  `json:"id,omitempty" yaml:"id,omitempty"`
	Description string  `json:"description,omitempty" yaml:"description,omitempty"`
	Timeout     Timeout `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Retries     int     `json:"retries,omitempty" yaml:"retries,omitempty"`
	Wait        Timeout `json:"wait,omitempty" yaml:"wait,omitempty"`
	Concurrency int     `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
	Checks      []Check `json:"checks,omitempty" yaml:"checks,omitempty"`
}

// New fetches the bundle data from the given path, parses it and returns a Bundle
// object with all checks ready to be run.
func New(path string) (*Bundle, error) {

	var (
		data []byte
		err  error
		f    format.Format
	)

	if strings.HasPrefix("http://", path) || strings.HasPrefix("https://", path) || strings.HasPrefix("https-://", path) {
		// retrieve from URL
		data, f, err = fetch.FromHTTP(path)
		if err != nil {
			slog.Error("error fetching bundle file from HTTP(s) source", "path", path, "error", err)
			return nil, err
		}
	} else if strings.HasPrefix("redis://", path) || strings.HasPrefix("rediss://", path) || strings.HasPrefix("rediss-://", path) {
		// retrieve from a Redis instance
		data, f, err = fetch.FromRedis(path)
		if err != nil {
			slog.Error("error fetching bundle file from Redis source", "path", path, "error", err)
			return nil, err
		}
	} else if strings.HasPrefix(path, "consulkv://") || strings.HasPrefix(path, "consulkvs://") || strings.HasPrefix(path, "consulkvs-://") {
		// retrieve from a Consul K/V store
		data, f, err = fetch.FromConsulKV(path)
		if err != nil {
			slog.Error("error fetching bundle file from Consul KV source", "path", path, "error", err)
			return nil, err
		}
	} else if strings.HasPrefix(path, "consulsr://") || strings.HasPrefix(path, "consulsrs://") || strings.HasPrefix(path, "consulsrs-://") {
		// retrieve from a Consul Service Registry
		data, f, err = fetch.FromConsulSR(path)
		if err != nil {
			slog.Error("error fetching bundle file from Consul Service Registry", "path", path, "error", err)
			return nil, err
		}
	} else {
		// attempt reading from file on disk
		data, f, err = fetch.FromFile(path)
		if err != nil {
			slog.Error("error fetching bundle file from local source", "path", path, "error", err)
			return nil, err
		}
	}

	bundle := &Bundle{
		Timeout:     DefaultTimeout,
		Retries:     DefaultRetries,
		Wait:        DefaultWait,
		Concurrency: DefaultConcurrency,
	}

	switch f {
	case format.YAML:
		err := yaml.Unmarshal(data, bundle)
		if err != nil {
			slog.Error("error parsing checks package", "format", "yaml", "error", err)
			os.Exit(1)
		}
	case format.JSON:
		err := json.Unmarshal(data, bundle)
		if err != nil {
			slog.Error("error parsing checks package", "format", "json", "error", err)
			os.Exit(1)
		}
	}
	// safety checks
	if bundle.Concurrency < 0 {
		bundle.Concurrency = DefaultConcurrency
	}
	if bundle.Timeout <= 0 {
		bundle.Timeout = DefaultTimeout
	}
	if bundle.Wait <= 0 {
		bundle.Wait = DefaultWait
	}
	if bundle.Retries < 1 {
		bundle.Retries = DefaultRetries
	}

	return bundle, nil
}

// ToJSON returns a JSON representation of the Bundle.
func (b *Bundle) ToJSON() string {
	data, _ := json.MarshalIndent(b, "  ", "")
	return string(data)
}

// ToYAML returns a YAML representation of the Bundle.
func (b *Bundle) ToYAML() string {
	data, _ := yaml.Marshal(b)
	return string(data)
}

// Check creates a goroutine pool, enqueues all the Checks to the workers in
// the pool and then waits for the Checks to come back with the actual result.
func (b *Bundle) Check() {
	inputs := make(chan Check, len(b.Checks))
	outputs := make(chan Check, len(b.Checks))

	// launch the thread pool
	for range b.Concurrency {
		go worker(inputs, outputs)
	}

	// submit the checks
	for id, check := range b.Checks {
		check.id = id
		if check.Timeout <= 0 {
			check.Timeout = b.Timeout
		}
		if check.Retries < 1 {
			check.Retries = b.Retries
		}
		if check.Wait <= 0 {
			check.Wait = b.Wait
		}
		inputs <- check
	}
	close(inputs)

	// update the Bundle with the results coming from the channel
	for range len(b.Checks) {
		output := <-outputs
		slog.Debug("received check", "from channel", output.ToJSON(), "original", b.Checks[output.id].ToJSON())
		b.Checks[output.id].Result = output.Result
	}
}

// works is the internal workhorse: itis deployed in multiple instances inside
// a goroutine pool, picks its Check from the inputs channel, runs the check,
// then updates the check's Error field and returns it on the output channel.
func worker(inputs <-chan Check, outputs chan<- Check) {

	for check := range inputs {
		var err error
		slog.Debug("performing check", "id", check.id)
		retries := check.Retries
		if retries <= 0 {
			retries = 1
		}
	attempts:
		for i := range retries {
			err = check.Do()
			if err != nil {
				slog.Warn("check failed", "id", check.id, "attempt", i+1, "error", err)
				time.Sleep(time.Duration(check.Wait))
			} else {
				slog.Debug("check successful", "id", check.id)
				break attempts
			}
		}
		// update the error in the check and return it
		check.Result = Result{
			err: err,
		}
		outputs <- check
	}
}
