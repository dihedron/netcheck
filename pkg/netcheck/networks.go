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

func (cfg *config) limited() bool { return len(cfg.allowed)+len(cfg.denied) > 0 }

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

// dialer returns the dialer every runner connects with: when networks are
// limited, its Control hook vets the resolved address before the connection.
func (cfg *config) dialer() *net.Dialer {
	d := &net.Dialer{}
	if cfg.limited() {
		d.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip, err := netip.ParseAddr(host)
			if err != nil {
				return err
			}
			if !cfg.permits(ip) {
				return fmt.Errorf("%w: %s", ErrNetworkNotAllowed, ip)
			}
			return nil
		}
	}
	return d
}

// resolve returns the address an icmp check pings: the literal IP, vetted
// against the networks, so that the pinger cannot resolve it differently.
func (cfg *config) pingTarget(ctx context.Context, host string) (string, error) {
	if !cfg.limited() {
		return host, nil
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", host, err)
	}
	for _, ip := range ips {
		if cfg.permits(ip) {
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrNetworkNotAllowed, host)
}
