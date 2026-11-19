package netcheck

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"
)

// ErrNetworkNotAllowed is wrapped by the error of a check that would connect
// to an address outside the allowed networks or inside a denied one.
var ErrNetworkNotAllowed = errors.New("network not allowed")

func (cfg *config) limited() bool {
	return len(cfg.allowed)+len(cfg.denied) > 0 || cfg.hooks.BeforeConnect != nil
}

// permits reports whether the networks of the configuration allow ip.
func (cfg *config) permits(ip netip.Addr) bool {
	ip = ip.Unmap().WithZone("")
	for _, p := range cfg.denied {
		if p.Contains(ip) {
			return false
		}
	}
	if len(cfg.allowed) == 0 {
		return true
	}
	for _, p := range cfg.allowed {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// vet decides whether a check may connect to ip: the networks limits first,
// then the BeforeConnect hooks of the run and of the check, each of which can
// only refuse (T-V5).
func (cfg *config) vet(ctx context.Context, ip netip.Addr) error {
	ip = ip.Unmap().WithZone("")
	if !cfg.permits(ip) {
		return fmt.Errorf("%w: %s", ErrNetworkNotAllowed, ip)
	}
	for _, f := range []func(context.Context, netip.Addr) error{cfg.hooks.BeforeConnect, ownHooks(ctx).BeforeConnect} {
		if f == nil {
			continue
		}
		if err := callHook("BeforeConnect", func() error { return f(ctx, ip) }); err != nil {
			return fmt.Errorf("%w: %s: %w", ErrNetworkNotAllowed, ip, err)
		}
	}
	return nil
}

// dialer returns the dialer every runner connects with: its Control hook
// vets the resolved address before the connection.
func (cfg *config) dialer() *net.Dialer {
	return &net.Dialer{ControlContext: func(ctx context.Context, _, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		ip, err := netip.ParseAddr(host)
		if err != nil {
			return err
		}
		return cfg.vet(ctx, ip)
	}}
}

// resolve returns the address an icmp check pings: the literal IP, vetted
// against the networks, so that the pinger cannot resolve it differently.
func (cfg *config) pingTarget(ctx context.Context, host string) (string, error) {
	if !cfg.limited() && ownHooks(ctx).BeforeConnect == nil {
		return host, nil
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", host, err)
	}
	for _, ip := range ips {
		if cfg.vet(ctx, ip) == nil {
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrNetworkNotAllowed, host)
}

type vetKey struct{}

// VetAddr reports whether a check may connect to ip: it is for plug-in
// runners that open their own sockets (see the dtls sub-package), so that the
// networks limits (WithAllowedNetworks, WithDeniedNetworks) bind them too. It
// returns an error wrapping ErrNetworkNotAllowed, or nil. Outside a Run it
// allows everything.
func VetAddr(ctx context.Context, ip netip.Addr) error {
	cfg, ok := ctx.Value(vetKey{}).(*config)
	if !ok {
		return nil
	}
	return cfg.vet(ctx, ip)
}

// Target returns the "host:port" a check of the protocol connects to, with
// the default ports applied (ssh 22, http 80, https 443) and the path of
// http and https addresses dropped.
func Target(p Protocol, address string) (string, error) { return dialTarget(p, address) }
