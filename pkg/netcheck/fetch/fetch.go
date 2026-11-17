// Package fetch loads a netcheck.Bundle from a location: a file, an HTTP(S)
// URL, or any other place a Source is registered for (Redis, Consul: see the
// redis and consul sub-packages). Sources are registered explicitly by the
// caller, so what a binary links is visible in its code.
//
// A location with no "://" is a file name. Any other scheme must have a
// Source registered, or Load fails naming the scheme: there is no fallback to
// the file system. Every fetch is bounded by the context and by a size limit,
// and credentials in a location never reach a log or an error.
package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	"gopkg.in/yaml.v3"
)

// Format is the serialisation of a bundle.
type Format string

// The formats; Unknown means the source gave no hint and the data is sniffed.
const (
	Unknown Format = ""
	YAML    Format = "yaml"
	JSON    Format = "json"
)

// DefaultMaxSize is the largest bundle Load accepts unless told otherwise.
const DefaultMaxSize = 1 << 20

// ErrUnknownScheme is wrapped by the error Load returns for a scheme with no Source.
var ErrUnknownScheme = errors.New("no source registered for scheme")

// Source retrieves the raw bundle at a location. It must honour ctx and return
// at most maxSize bytes (Load rejects more); the Format is a hint, Unknown when
// it has none. A Source does not log and need not redact: Load does.
type Source interface {
	Fetch(ctx context.Context, location *url.URL, maxSize int64) (data []byte, hint Format, err error)
}

// Option tunes Load.
type Option func(*config)

type config struct {
	sources map[string]Source
	maxSize int64
	client  *http.Client
	logger  *slog.Logger
}

// WithSource registers a Source for the given schemes (lower case).
func WithSource(s Source, schemes ...string) Option {
	return func(c *config) {
		for _, scheme := range schemes {
			c.sources[strings.ToLower(scheme)] = s
		}
	}
}

// WithMaxSize sets the size limit in bytes; the default is DefaultMaxSize.
func WithMaxSize(n int64) Option { return func(c *config) { c.maxSize = n } }

// WithHTTPClient sets the client of the http and https sources, for custom
// trust, proxies or SSO; the library never modifies it.
func WithHTTPClient(h *http.Client) Option { return func(c *config) { c.client = h } }

// WithLogger sets where Load logs; by default it logs nothing.
func WithLogger(l *slog.Logger) Option { return func(c *config) { c.logger = l } }

func newConfig(opts []Option) *config {
	c := &config{sources: map[string]Source{}, maxSize: DefaultMaxSize, logger: slog.New(slog.DiscardHandler)}
	for _, o := range opts {
		o(c)
	}
	built := map[string]Source{"file": fileSource{}, "http": httpSource{c.client}, "https": httpSource{c.client}}
	for scheme, s := range built {
		if _, ok := c.sources[scheme]; !ok {
			c.sources[scheme] = s
		}
	}
	return c
}

// Load fetches and parses the bundle at location.
func Load(ctx context.Context, location string, opts ...Option) (*netcheck.Bundle, error) {
	cfg := newConfig(opts)
	u, err := parseLocation(location)
	if err != nil {
		return nil, err
	}
	log := cfg.logger.With("location", u.Redacted())
	src, ok := cfg.sources[u.Scheme]
	if !ok {
		return nil, fmt.Errorf("%w %q", ErrUnknownScheme, u.Scheme)
	}
	log.Debug("fetching bundle")
	data, hint, err := src.Fetch(ctx, u, cfg.maxSize)
	if err != nil {
		err = fmt.Errorf("fetching %s: %w", u.Redacted(), scrub(err, u))
		log.Error("fetching bundle failed", "error", err.Error())
		return nil, err
	}
	if int64(len(data)) > cfg.maxSize {
		err = fmt.Errorf("fetching %s: the bundle is larger than the %d bytes allowed", u.Redacted(), cfg.maxSize)
		log.Error("fetching bundle failed", "error", err.Error())
		return nil, err
	}
	bundle, err := Parse(data, hint)
	if err != nil {
		err = fmt.Errorf("reading the bundle from %s: %w", u.Redacted(), err)
		log.Error("parsing bundle failed", "error", err.Error())
		return nil, err
	}
	log.Debug("bundle loaded", "id", bundle.ID, "checks", len(bundle.Checks))
	return bundle, nil
}

// parseLocation turns a location into a URL: a name with no "://" is a file.
func parseLocation(location string) (*url.URL, error) {
	if !strings.Contains(location, "://") {
		return &url.URL{Scheme: "file", Path: location}, nil
	}
	u, err := url.Parse(location)
	if err != nil {
		// the error would quote the location, credentials included
		return nil, errors.New("the location is not a valid URL")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	return u, nil
}

// Parse decodes a bundle; hint says whether it is JSON or YAML, and with no
// hint the data is sniffed, then tried as JSON and as YAML.
func Parse(data []byte, hint Format) (*netcheck.Bundle, error) {
	trimmed := bytes.TrimLeft(data, "\xef\xbb\xbf \t\r\n")
	if len(trimmed) == 0 {
		return nil, errors.New("the bundle is empty")
	}
	if hint == Unknown {
		switch {
		case bytes.HasPrefix(trimmed, []byte("---")):
			hint = YAML
		case trimmed[0] == '{' || trimmed[0] == '[':
			hint = JSON
		}
	}
	var b netcheck.Bundle
	switch hint {
	case JSON:
		if err := json.Unmarshal(data, &b); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
	case YAML:
		if err := yaml.Unmarshal(data, &b); err != nil {
			return nil, fmt.Errorf("invalid YAML: %w", err)
		}
	default:
		if json.Unmarshal(data, &b) == nil {
			return &b, nil
		}
		b = netcheck.Bundle{}
		if yaml.Unmarshal(data, &b) != nil {
			return nil, errors.New("unsupported or undetected format")
		}
	}
	return &b, nil
}

// scrubbed is an error whose text has lost the password of the location.
type scrubbed struct {
	error
	msg string
}

func (e scrubbed) Error() string { return e.msg }
func (e scrubbed) Unwrap() error { return e.error }

// scrub removes the password of the location from the error's text.
func scrub(err error, u *url.URL) error {
	pw, _ := u.User.Password()
	if pw == "" || !strings.Contains(err.Error(), pw) {
		return err
	}
	return scrubbed{err, strings.ReplaceAll(err.Error(), pw, "***")}
}
