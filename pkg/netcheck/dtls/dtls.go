// Package dtls is the runner of the dtls check, on pion/dtls. The core does
// not import it, so that a program that needs no DTLS does not link it:
//
//	netcheck.Run(ctx, bundle, netcheck.WithRunner(netcheck.DTLS, dtls.Runner()))
//
// The check passes when the DTLS handshake succeeds with a verified chain
// and a matching name (TLSOptions as for tls); the certificate and session
// are described in Detail.TLS. A server that does not answer fails the
// check by timeout.
package dtls

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
	pion "github.com/pion/dtls/v3"
)

// Runner returns the DTLS runner.
func Runner() netcheck.Runner { return runner{} }

type runner struct{}

func (runner) Run(ctx context.Context, c netcheck.Check, _ netcheck.Defaults) (netcheck.Detail, error) {
	target, err := netcheck.Target(netcheck.DTLS, c.Address)
	if err != nil {
		return netcheck.Detail{}, err
	}
	host, portText, _ := net.SplitHostPort(target)
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return netcheck.Detail{}, fmt.Errorf("invalid port %q", portText)
	}
	ip, err := resolve(ctx, host)
	if err != nil {
		return netcheck.Detail{}, err
	}
	raddr := &net.UDPAddr{IP: ip.AsSlice(), Port: port}

	var o netcheck.TLSOptions
	if c.TLS != nil {
		o = *c.TLS
	}
	if o.ServerName == "" {
		o.ServerName = host
	}
	// pion writes with WriteTo, so the socket is unconnected
	pc, err := net.ListenUDP("udp", nil)
	if err != nil {
		return netcheck.Detail{}, fmt.Errorf("opening a UDP socket: %w", err)
	}
	conn, err := pion.ClientWithOptions(pc, raddr,
		pion.WithRootCAs(o.RootCAs), pion.WithServerName(o.ServerName), pion.WithInsecureSkipVerify(o.InsecureSkipVerify)) //nolint:gosec // only when the caller asks, and the result says so
	if err != nil {
		_ = pc.Close()
		return netcheck.Detail{}, fmt.Errorf("creating the DTLS client: %w", err)
	}
	defer func() { _ = conn.Close() }() // closes the socket too

	if err := conn.HandshakeContext(ctx); err != nil {
		return netcheck.Detail{}, fmt.Errorf("DTLS handshake with %s: %w", target, err)
	}
	detail := netcheck.Detail{Remote: raddr.String()}
	if state, ok := conn.ConnectionState(); ok {
		f := &netcheck.TLSFacts{Version: "DTLS 1.2", CipherSuite: pion.CipherSuiteName(state.CipherSuiteID), Verified: !o.InsecureSkipVerify}
		if len(state.PeerCertificates) > 0 {
			if cert, err := x509.ParseCertificate(state.PeerCertificates[0]); err == nil {
				f.Describe(cert)
			}
		}
		detail.TLS = f
	}
	return detail, nil
}

// resolve picks the first address of host that the networks limits allow.
func resolve(ctx context.Context, host string) (netip.Addr, error) {
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("resolving %s: %w", host, err)
	}
	var vetErr error
	for _, ip := range ips {
		if vetErr = netcheck.VetAddr(ctx, ip); vetErr == nil {
			return ip, nil
		}
	}
	if vetErr == nil {
		vetErr = errors.New("no address")
	}
	return netip.Addr{}, vetErr
}
