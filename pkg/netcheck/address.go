package netcheck

import (
	"errors"
	"net"
	"net/netip"
	"strings"
)

// splitHostPort splits "host:port", "[v6]:port", a bare host or a bare IPv6
// literal; def is the port to use when the address has none ("" means none
// is available and the address is rejected).
func splitHostPort(addr, def string) (host, port string, err error) {
	if addr == "" {
		return "", "", errors.New("address is empty")
	}
	switch {
	case isIP(addr):
		host, port = addr, def
	case strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]"):
		host, port = addr[1:len(addr)-1], def
	case !strings.Contains(addr, ":"):
		host, port = addr, def
	default:
		if host, port, err = net.SplitHostPort(addr); err != nil {
			return "", "", err
		}
	}
	if host == "" {
		return "", "", errors.New("address has no host")
	}
	if port == "" {
		return "", "", errors.New("address has no port")
	}
	return host, port, nil
}

func isIP(s string) bool {
	_, err := netip.ParseAddr(s)
	return err == nil
}

// dialTarget returns the "host:port" to connect to for a check's address,
// applying the default ports (ssh 22, http 80, https 443) and dropping the
// path of http and https addresses.
func dialTarget(p Protocol, addr string) (string, error) {
	def := ""
	switch p {
	case SSH:
		def = "22"
	case HTTP:
		def = "80"
	case HTTPS:
		def = "443"
	}
	if p == HTTP || p == HTTPS {
		addr = redact(addr)
		addr = addr[:authorityEnd(addr)]
	}
	host, port, err := splitHostPort(addr, def)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(host, port), nil
}

func authorityEnd(s string) int {
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		return i
	}
	return len(s)
}

// redact drops the user information ("user:password@") of an address, so
// that credentials never reach a log or an error.
func redact(addr string) string {
	if i := strings.LastIndex(addr[:authorityEnd(addr)], "@"); i >= 0 {
		return addr[i+1:]
	}
	return addr
}
