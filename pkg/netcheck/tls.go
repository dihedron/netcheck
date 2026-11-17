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
	target, err := dialTarget(TLS, c.Address)
	if err != nil {
		return Detail{}, err
	}
	host, _, _ := net.SplitHostPort(target)
	cfg := tlsConfig(c.TLS, host)
	dialer := tls.Dialer{NetDialer: r.dialer, Config: cfg}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return Detail{TLS: factsFromError(err)}, fmt.Errorf("TLS handshake with %s: %w", target, err)
	}
	defer func() { _ = conn.Close() }()
	state := conn.(*tls.Conn).ConnectionState()
	return Detail{Remote: conn.RemoteAddr().String(), TLS: factsFromState(state, !cfg.InsecureSkipVerify)}, nil
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
}

func ipStrings(ips []net.IP) []string {
	out := make([]string, len(ips))
	for i, ip := range ips {
		out[i] = ip.String()
	}
	return out
}
