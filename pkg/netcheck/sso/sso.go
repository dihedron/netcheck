// Package sso is the HTTP client of the checks and bundle sources that use
// single sign-on (SPNEGO/Kerberos, on go-spnego). The core does not import
// it:
//
//	netcheck.Run(ctx, bundle, netcheck.WithSSOClient(sso.Client()))
//	fetch.Load(ctx, location, fetch.WithHTTPClient(sso.Client()))
//
// The client authenticates every request with the Kerberos configuration of
// the host (krb5.conf, a ticket cache or keytab; SSPI on Windows) and honours
// the HTTP_PROXY variables. It is used as given: network limits and TLS
// options of the check do not apply to it, so set the TLS configuration here
// with WithTLSConfig.
package sso

import (
	"crypto/tls"
	"net/http"

	"github.com/dpotapov/go-spnego"
)

// Option tunes the client.
type Option func(*spnego.Transport)

// WithTLSConfig sets the TLS configuration of the client, for a private CA or
// to skip verification.
func WithTLSConfig(c *tls.Config) Option {
	return func(t *spnego.Transport) { t.TLSClientConfig = c }
}

// Client returns a new SPNEGO-authenticated HTTP client; the process-wide
// default transport and client are not touched.
func Client(opts ...Option) *http.Client {
	t := &spnego.Transport{}
	t.Transport = *http.DefaultTransport.(*http.Transport).Clone() //nolint:govet // copylocks: the clone is fresh, nothing else holds it
	for _, o := range opts {
		o(t)
	}
	return &http.Client{Transport: t}
}
