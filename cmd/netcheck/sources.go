package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	"github.com/dihedron/netcheck/v2/pkg/netcheck/fetch"
	"github.com/dihedron/netcheck/v2/pkg/netcheck/fetch/consul"
	"github.com/dihedron/netcheck/v2/pkg/netcheck/fetch/redis"
	"github.com/dihedron/netcheck/v2/pkg/netcheck/sso"
)

// splitScheme reads the command's scheme forms: a trailing "-" on the scheme
// skips the TLS verification (https-://, rediss-://, consulkvs-://...), and
// "+sso" selects SPNEGO authentication (http+sso://, https+sso-://). It
// returns the location with the plain scheme the library knows.
func splitScheme(location string) (plain string, insecure, withSSO bool) {
	scheme, rest, ok := strings.Cut(location, "://")
	if !ok {
		return location, false, false
	}
	if s, found := strings.CutSuffix(scheme, "-"); found {
		scheme, insecure = s, true
	}
	if s, found := strings.CutSuffix(scheme, "+sso"); found {
		scheme, withSSO = s, true
	}
	return scheme + "://" + rest, insecure, withSSO
}

// skipVerify is the TLS configuration of the "-" schemes: an explicit request
// of the user, never a default.
func skipVerify() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12} //nolint:gosec // the "-" schemes ask for it
}

// loadBundle fetches a bundle from a file or URL, with every source the
// command supports registered.
func loadBundle(ctx context.Context, location string) (*netcheck.Bundle, error) {
	location, insecure, withSSO := splitScheme(location)

	var tlsConfig *tls.Config
	var httpClient *http.Client
	redisOpts := []redis.Option{}
	consulOpts := []consul.Option{}
	if insecure {
		tlsConfig = skipVerify()
		httpClient = &http.Client{Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, TLSClientConfig: tlsConfig}}
		redisOpts = append(redisOpts, redis.WithTLSConfig(tlsConfig))
		consulOpts = append(consulOpts, consul.WithHTTPClient(httpClient))
	}
	if withSSO {
		var o []sso.Option
		if insecure {
			o = append(o, sso.WithTLSConfig(tlsConfig))
		}
		httpClient = sso.Client(o...)
	}
	opts := []fetch.Option{
		fetch.WithSource(redis.Source(redisOpts...), "redis", "rediss"),
		fetch.WithSource(consul.KV(consulOpts...), "consulkv", "consulkvs"),
		fetch.WithSource(consul.ServiceRegistry(consulOpts...), "consulsr", "consulsrs"),
	}
	if httpClient != nil {
		opts = append(opts, fetch.WithHTTPClient(httpClient))
	}
	return fetch.Load(ctx, location, opts...)
}

// parseNetworks reads CIDR prefixes; a bare address is a single-host prefix.
func parseNetworks(values []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, v := range values {
		p, err := netip.ParsePrefix(v)
		if err != nil {
			addr, addrErr := netip.ParseAddr(v)
			if addrErr != nil {
				return nil, fmt.Errorf("%q is not a network (CIDR) or an address", v)
			}
			p = netip.PrefixFrom(addr, addr.BitLen())
		}
		prefixes = append(prefixes, p)
	}
	return prefixes, nil
}
