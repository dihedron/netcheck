// Package spire is the runner of the spire check: a TLS handshake whose server
// certificate is verified as an X509-SVID, the way SPIFFE workloads do, and
// not by name against the system roots. The core does not import it:
//
//	netcheck.Run(ctx, bundle, netcheck.WithRunner(netcheck.SPIRE, spire.Runner(spire.WithBundleFile(path))))
//
// The chain must be signed by the trust bundle, in date, and carry one URI
// SAN that is a SPIFFE ID; Check.SPIFFEID, when set, is the ID it must be.
// The trust anchors come from the options of the runner, never from a bundle
// file of checks, which could otherwise choose what to trust.
package spire

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

// Option tunes the runner.
type Option func(*runner)

// WithSource sets where the trust bundles come from: a Workload API source
// (workloadapi.BundleSource), a set of bundles, or any x509bundle.Source.
func WithSource(s x509bundle.Source) Option { return func(r *runner) { r.source = s } }

// WithBundleFile reads the trust bundle, the PEM of the CAs of the trust domain,
// from path at each attempt, so that a rotated file is picked up. The trust
// domain of the SVID is not compared with anything: the CAs are the trust.
func WithBundleFile(path string) Option { return func(r *runner) { r.source = fileSource(path) } }

type fileSource string

func (f fileSource) GetX509BundleForTrustDomain(td spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	return x509bundle.Load(td, string(f))
}

// Runner returns the spire runner. Without WithSource or WithBundleFile every
// check fails: there is nothing to trust.
func Runner(opts ...Option) netcheck.Runner {
	r := &runner{}
	for _, o := range opts {
		o(r)
	}
	return r
}

type runner struct{ source x509bundle.Source }

func (r *runner) Run(ctx context.Context, c netcheck.Check, _ netcheck.Defaults) (netcheck.Detail, error) {
	if r.source == nil {
		return netcheck.Detail{}, errors.New("no trust bundle: give the runner a source or a bundle file")
	}
	if c.SPIFFEID != "" {
		if _, err := spiffeid.FromString(c.SPIFFEID); err != nil {
			return netcheck.Detail{}, fmt.Errorf("invalid spiffe_id: %w", err)
		}
	}
	var h netcheck.Hooks
	if c.Hooks != nil {
		h = *c.Hooks
	}
	h.VerifyCertificate = func(chain []*x509.Certificate) error {
		raw := make([][]byte, len(chain))
		for i, cert := range chain {
			raw[i] = cert.Raw
		}
		got, _, err := x509svid.ParseAndVerify(raw, r.source)
		if err != nil {
			return fmt.Errorf("verifying the server's SVID: %w", err)
		}
		if c.SPIFFEID != "" && got.String() != c.SPIFFEID {
			return fmt.Errorf("the server presented %s, expected %s", got, c.SPIFFEID)
		}
		return nil
	}
	c.Hooks = &h
	return netcheck.TLSHandshake(ctx, netcheck.SPIRE, c)
}
