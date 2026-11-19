package netcheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
)

// tlsRunner passes when the handshake succeeds with a verified chain, a
// matching name and an unexpired certificate.
type tlsRunner struct{ dialer *net.Dialer }

func (r tlsRunner) Run(ctx context.Context, c Check, _ Defaults) (Detail, error) {
	return tlsHandshake(ctx, r.dialer, TLS, c)
}

// TLSHandshake is the TLS check for the runners of sub-packages that start
// from it (postgresql, spire): it connects with the run's dialer, so the
// networks limits and BeforeConnect hooks bind it, calls the AfterConnect,
// VerifyCertificate and AfterHandshake hooks of the check (those of the run
// where the check has none) and describes the session. p names the protocol
// for the default port and the errors.
func TLSHandshake(ctx context.Context, p Protocol, c Check) (Detail, error) {
	cfg, ok := ctx.Value(vetKey{}).(*config)
	if !ok {
		cfg = &config{}
	}
	return tlsHandshake(ctx, cfg.dialer(), p, c)
}

func tlsHandshake(ctx context.Context, dialer *net.Dialer, p Protocol, c Check) (Detail, error) {
	target, err := dialTarget(p, c.Address)
	if err != nil {
		return Detail{}, err
	}
	host, _, _ := net.SplitHostPort(target)
	h := effectiveHooks(ctx, c)
	cfg := tlsConfig(c.TLS, host)
	var leaf *x509.Certificate
	custom := h.VerifyCertificate != nil
	if custom {
		cfg.InsecureSkipVerify = true //nolint:gosec // the VerifyCertificate hook verifies instead, and the result says so
		// VerifyConnection, not VerifyPeerCertificate: it also runs on a resumed session
		cfg.VerifyConnection = func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("the server presented no certificate")
			}
			leaf = cs.PeerCertificates[0]
			return callHook("VerifyCertificate", func() error { return h.VerifyCertificate(cs.PeerCertificates) })
		}
	}

	raw, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return Detail{}, fmt.Errorf("TLS handshake with %s: %w", target, err)
	}
	defer func() { _ = raw.Close() }()
	detail := Detail{Remote: raw.RemoteAddr().String()}
	if err := afterConnect(ctx, h, raw); err != nil {
		return detail, fmt.Errorf("after connecting to %s: %w", target, err)
	}
	conn := tls.Client(raw, cfg)
	if err := conn.HandshakeContext(ctx); err != nil {
		detail.TLS = factsFromError(err)
		if detail.TLS == nil && leaf != nil {
			detail.TLS = &TLSFacts{Verifier: "custom"}
			detail.TLS.Describe(leaf)
		}
		return detail, fmt.Errorf("TLS handshake with %s: %w", target, err)
	}
	detail.TLS = factsFromState(conn.ConnectionState(), !cfg.InsecureSkipVerify || custom)
	if custom {
		detail.TLS.Verifier = "custom"
	}
	if h.AfterHandshake != nil {
		if err := callHook("AfterHandshake", func() error { return h.AfterHandshake(ctx, conn.ConnectionState()) }); err != nil {
			return detail, fmt.Errorf("after the handshake with %s: %w", target, err)
		}
	}
	return detail, nil
}

// tlsConfig builds the client configuration: verification is on unless the
// caller asked otherwise (T-4).
func tlsConfig(o *TLSOptions, host string) *tls.Config {
	cfg := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	if o != nil {
		if o.ServerName != "" {
			cfg.ServerName = o.ServerName
		}
		cfg.RootCAs = o.RootCAs
		cfg.InsecureSkipVerify = o.InsecureSkipVerify //nolint:gosec // only when the caller asks, and the result says so
	}
	return cfg
}

func factsFromState(s tls.ConnectionState, verified bool) *TLSFacts {
	f := &TLSFacts{Version: tls.VersionName(s.Version), CipherSuite: tls.CipherSuiteName(s.CipherSuite), Verified: verified}
	if len(s.PeerCertificates) > 0 {
		f.Describe(s.PeerCertificates[0])
	}
	return f
}

// factsFromError describes the certificate a failed verification rejected,
// so that a report can say what was wrong with it.
func factsFromError(err error) *TLSFacts {
	var cve *tls.CertificateVerificationError
	if errors.As(err, &cve) && len(cve.UnverifiedCertificates) > 0 {
		f := &TLSFacts{}
		f.Describe(cve.UnverifiedCertificates[0])
		return f
	}
	return nil
}

// Describe fills in the certificate facts: subject, issuer, expiry and names.
func (f *TLSFacts) Describe(c *x509.Certificate) {
	f.Subject, f.Issuer, f.NotAfter = c.Subject.String(), c.Issuer.String(), c.NotAfter
	f.SANs = append(append(append([]string{}, c.DNSNames...), ipStrings(c.IPAddresses)...), c.EmailAddresses...)
	for _, u := range c.URIs {
		f.SANs = append(f.SANs, u.String())
	}
}

func ipStrings(ips []net.IP) []string {
	out := make([]string, len(ips))
	for i, ip := range ips {
		out[i] = ip.String()
	}
	return out
}
