// Package consul has two fetch.Sources that read a bundle from Consul.
//
// KV reads a key: consulkv://[user:password@|:token@]host[:port]?key=<key>[&dc=<dc>],
// or consulkvs:// for HTTPS. ServiceRegistry reads a metadata value of the
// one service instance with the given name and tag:
// consulsr://host[:port]?service=<name>&tag=<tag>&meta=<key>[&dc=<dc>], or
// consulsrs://. A user and a password are HTTP basic authentication; a
// password alone is an ACL token. Register them with
//
//	fetch.WithSource(consul.KV(), "consulkv", "consulkvs")
//	fetch.WithSource(consul.ServiceRegistry(), "consulsr", "consulsrs")
//
// Nothing is read from the CONSUL_* environment variables.
package consul

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/dihedron/netcheck/v2/pkg/netcheck/fetch"
	capi "github.com/hashicorp/consul/api"
)

// Option tunes the sources.
type Option func(*config)

type config struct{ client *http.Client }

// WithHTTPClient sets the client of the Consul API calls, for custom trust (a
// private CA, or skipping verification); the library never modifies it.
func WithHTTPClient(c *http.Client) Option { return func(cfg *config) { cfg.client = c } }

// KV returns the key/value source.
func KV(opts ...Option) fetch.Source { return source{opts, false} }

// ServiceRegistry returns the service registry source.
func ServiceRegistry(opts ...Option) fetch.Source { return source{opts, true} }

type source struct {
	opts     []Option
	registry bool
}

func (s source) Fetch(ctx context.Context, u *url.URL, _ int64) ([]byte, fetch.Format, error) {
	q := u.Query()
	// required parameters are checked before any connection
	var key, name, tag, meta string
	if s.registry {
		if name, tag, meta = q.Get("service"), q.Get("tag"), q.Get("meta"); name == "" || tag == "" || meta == "" {
			return nil, fetch.Unknown, errors.New("the location needs service, tag and meta parameters")
		}
	} else if key = q.Get("key"); key == "" {
		return nil, fetch.Unknown, errors.New("the location has no key: add ?key=<name>")
	}
	client, err := s.client(u)
	if err != nil {
		return nil, fetch.Unknown, err
	}
	qo := (&capi.QueryOptions{Datacenter: q.Get("dc")}).WithContext(ctx)

	if !s.registry {
		pair, _, err := client.KV().Get(key, qo)
		if err != nil {
			return nil, fetch.Unknown, fmt.Errorf("reading key %q: %w", key, err)
		}
		if pair == nil {
			return nil, fetch.Unknown, fmt.Errorf("no key found under %s", key)
		}
		return pair.Value, fetch.Unknown, nil
	}
	services, _, err := client.Catalog().Service(name, tag, qo)
	if err != nil {
		return nil, fetch.Unknown, fmt.Errorf("looking up service %q with tag %q: %w", name, tag, err)
	}
	if len(services) != 1 {
		return nil, fetch.Unknown, fmt.Errorf("%d services found under name %s with tag %s, expected exactly 1", len(services), name, tag)
	}
	data, ok := services[0].ServiceMeta[meta]
	if !ok {
		return nil, fetch.Unknown, fmt.Errorf("no bundle found under name %s with tag %s and metadata key %s", name, tag, meta)
	}
	return []byte(data), fetch.Unknown, nil
}

// client builds the API client for a location; it reads no environment.
func (s source) client(u *url.URL) (*capi.Client, error) {
	var cfg config
	for _, o := range s.opts {
		o(&cfg)
	}
	c := &capi.Config{Address: u.Host, Scheme: "http", HttpClient: cfg.client}
	if strings.HasSuffix(u.Scheme, "s") { // consulkvs, consulsrs
		c.Scheme = "https"
	}
	if c.HttpClient == nil {
		c.HttpClient = &http.Client{Transport: http.DefaultTransport}
	}
	username, password := u.User.Username(), ""
	password, _ = u.User.Password()
	switch {
	case username != "" && password != "":
		c.HttpAuth = &capi.HttpBasicAuth{Username: username, Password: password}
	case password != "":
		c.Token = password
	}
	return capi.NewClient(c)
}
