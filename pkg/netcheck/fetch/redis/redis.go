// Package redis is a fetch.Source that reads a bundle from a Redis key.
//
// The location is redis://[user:password@]host[:port]?key=<key>[&db=<n>], or
// rediss:// for TLS. Register it with
// fetch.WithSource(redis.Source(), "redis", "rediss").
package redis

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/dihedron/netcheck/v2/pkg/netcheck/fetch"
	goredis "github.com/redis/go-redis/v9"
)

// Option tunes the Source.
type Option func(*config)

type config struct{ tls *tls.Config }

// WithTLSConfig sets the TLS configuration of rediss locations; by default the
// certificate is verified against the system roots and the host name.
func WithTLSConfig(c *tls.Config) Option { return func(cfg *config) { cfg.tls = c } }

// Source returns the Redis source.
func Source(opts ...Option) fetch.Source { return source{opts} }

type source struct{ opts []Option }

func (s source) Fetch(ctx context.Context, u *url.URL, _ int64) ([]byte, fetch.Format, error) {
	opts, key, err := options(u, s.opts)
	if err != nil {
		return nil, fetch.Unknown, err
	}
	client := goredis.NewClient(opts)
	defer func() { _ = client.Close() }()
	value, err := client.Get(ctx, key).Result()
	if err != nil {
		return nil, fetch.Unknown, fmt.Errorf("reading key %q: %w", key, err)
	}
	return []byte(value), fetch.Unknown, nil
}

// options builds the client options and the key from a location.
func options(u *url.URL, opts []Option) (*goredis.Options, string, error) {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	key := u.Query().Get("key")
	if key == "" {
		return nil, "", errors.New("the location has no key: add ?key=<name>")
	}
	if u.Hostname() == "" {
		return nil, "", errors.New("the location has no host")
	}
	o := &goredis.Options{Addr: u.Host, Username: u.User.Username(), MaxRetries: -1, ContextTimeoutEnabled: true} // no retries: the caller decides; the context bounds every read
	o.Password, _ = u.User.Password()
	if db := u.Query().Get("db"); db != "" {
		n, err := strconv.Atoi(db)
		if err != nil || n < 0 || n > 65535 {
			return nil, "", fmt.Errorf("invalid db %q", db)
		}
		o.DB = n
	}
	switch u.Scheme {
	case "redis":
	case "rediss":
		o.TLSConfig = cfg.tls
		if o.TLSConfig == nil {
			o.TLSConfig = &tls.Config{ServerName: u.Hostname(), MinVersion: tls.VersionTLS12}
		}
	default:
		return nil, "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	return o, key, nil
}
