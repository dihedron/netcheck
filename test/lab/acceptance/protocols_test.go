//go:build lab

package acceptance

import (
	"strings"
	"testing"
)

// LB-02: every protocol, pass and fail, against the real servers.

func addr(port string) string { return serversIP() + ":" + port }

func TestTCP(t *testing.T) {
	wantPass(t, one(t, "tcp", addr("9001")))
	wantFail(t, one(t, "tcp", addr("9002")), "refused")
	wantPass(t, one(t, "tcp", "[fd42:77:77:2::10]:9001")) // IPv6
	wantPass(t, one(t, "tcp", "nc-servers.lab:9001"))     // by name
}

// A packet that the router drops is a timeout, and the counter proves that the
// traffic really goes through the router.
func TestTCPDroppedByTheRouter(t *testing.T) {
	hits := fault(t, "ip daddr "+serversIP()+" tcp dport 9003 counter drop")
	wantFail(t, one(t, "tcp", addr("9003")), "timeout")
	if hits() == 0 {
		t.Error("the fault rule matched nothing: the client does not reach the servers through nc-fw")
	}
}

func TestUDP(t *testing.T) {
	wantPass(t, one(t, "udp", addr("9101")))
	wantFail(t, one(t, "udp", addr("9102")), "refused") // the kernel answers with ICMP port unreachable
}

func TestICMP(t *testing.T) {
	r := one(t, "icmp", serversIP())
	wantPass(t, r)
	if r.Detail.PacketsRecv == 0 {
		t.Errorf("no echo reply recorded: %+v", r.Detail)
	}
	r = one(t, "icmp", "fd42:77:77:2::10") // IPv6
	wantPass(t, r)
}

func TestICMPLossIsAFailure(t *testing.T) {
	hits := fault(t, "ip daddr "+serversIP()+" icmp type echo-request counter drop")
	wantFail(t, one(t, "icmp", serversIP()), "loss")
	if hits() == 0 {
		t.Error("the fault rule matched nothing")
	}
}

func TestTLS(t *testing.T) {
	r := one(t, "tls", "nc-servers.lab:8443")
	wantPass(t, r)
	if r.Detail.TLS == nil || !r.Detail.TLS.Verified {
		t.Errorf("the certificate was not reported as verified: %+v", r.Detail.TLS)
	}
	wantPass(t, one(t, "tls", addr("8443")))              // by IP: the certificate has an IP SAN
	wantPass(t, one(t, "tls", "[fd42:77:77:2::10]:8443")) // IPv6
	wantFail(t, one(t, "tls", "nc-servers.lab:8444"), "expired")
	wantFail(t, one(t, "tls", "nc-servers.lab:8445"), "someone-else.lab")
	wantFail(t, one(t, "tls", "nc-servers.lab:8446"), "unknown authority")
	wantFail(t, one(t, "tls", "nc-servers.lab:9001")) // a plain port
}

func TestFailedTLSStillDescribesTheCertificate(t *testing.T) {
	r := one(t, "tls", "nc-servers.lab:8444")
	wantFail(t, r)
	if r.Detail.TLS == nil || r.Detail.TLS.NotAfter.IsZero() {
		t.Errorf("an expired certificate must still be described: %+v", r.Detail.TLS)
	}
}

func TestDTLS(t *testing.T) {
	wantPass(t, one(t, "dtls", "nc-servers.lab:4433"))
	wantFail(t, one(t, "dtls", addr("9101"))) // a plain UDP port
}

func TestSSH(t *testing.T) {
	r := one(t, "ssh", serversIP())
	wantPass(t, r)
	if !strings.HasPrefix(r.Detail.SSHBanner, "SSH-2.0-OpenSSH") {
		t.Errorf("unexpected banner %q", r.Detail.SSHBanner)
	}
	// the fingerprint the check reports is one of the server's host keys
	keys := run(t, "nc-servers", "sh", "-c", "for k in /etc/ssh/ssh_host_*_key.pub; do ssh-keygen -lf $k; done").Stdout
	fingerprint := strings.TrimPrefix(r.Detail.SSHHostKey, "SHA256:")
	if fingerprint == "" || !strings.Contains(keys, fingerprint) {
		t.Errorf("fingerprint %q is none of the server's host keys:\n%s", r.Detail.SSHHostKey, keys)
	}
	wantFail(t, one(t, "ssh", addr("9301"))) // speaks, but not SSH
	wantFail(t, one(t, "ssh", addr("9002")), "refused")
}

func TestHTTP(t *testing.T) {
	r := one(t, "http", "nc-servers.lab/ok")
	wantPass(t, r)
	if r.Detail.HTTPStatus != 200 {
		t.Errorf("status %d", r.Detail.HTTPStatus)
	}
	wantPass(t, one(t, "http", "nc-servers.lab/moved"))  // a redirect to /ok
	wantPass(t, one(t, "http", "nc-servers.lab/big"))    // a body over the 64 KiB read limit
	wantPass(t, one(t, "http", "[fd42:77:77:2::10]/ok")) // IPv6
	r = one(t, "http", "nc-servers.lab/missing")
	wantFail(t, r, "404")
	if r.Detail.HTTPStatus != 404 {
		t.Errorf("status %d", r.Detail.HTTPStatus)
	}
	r = one(t, "http", "nc-servers.lab/error")
	wantFail(t, r, "500")
	wantFail(t, one(t, "http", "nc-servers.lab/loop"), "redirects")
	wantFail(t, one(t, "http", "nc-servers.lab:9201/ok"), "deadline exceeded") // accepts, never answers
}

func TestHTTPS(t *testing.T) {
	wantPass(t, one(t, "https", "nc-servers.lab:8443/ok"))
	wantFail(t, one(t, "https", "nc-servers.lab:8444/"), "expired")
	wantFail(t, one(t, "https", "nc-servers.lab:8446/"), "unknown authority")
	wantFail(t, one(t, "https", "nc-servers.lab:8443/to-http"), "downgrade") // https to http is refused
}
