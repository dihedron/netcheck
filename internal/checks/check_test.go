package checks

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// listener returns a TCP listener of ours and its address; it accepts and
// closes every connection.
func listener(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	return l.Addr().String()
}

// closedPort returns an address where nothing listens.
func closedPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func check(p Protocol, address string) Check {
	return Check{Address: address, Protocol: p, Timeout: Timeout(2 * time.Second), Retries: 1}
}

func TestTCP(t *testing.T) {
	c := check(TCP, listener(t))
	if err := c.Do(); err != nil {
		t.Errorf("listening port: %v", err)
	}
	c = check(TCP, closedPort(t))
	if err := c.Do(); err == nil {
		t.Error("closed port: expected an error")
	}
}

func TestUDP(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pc.Close() }()
	c := check(UDP, pc.LocalAddr().String())
	if err := c.Do(); err != nil {
		t.Errorf("UDP socket: %v", err)
	}
}

// A TLS check against a server whose certificate is signed by a CA that the
// system does not trust fails on verification; so does one against a plain
// TCP service.
func TestTLSFailures(t *testing.T) {
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()
	c := check(TLS, strings.TrimPrefix(server.URL, "https://"))
	if err := c.Do(); err == nil {
		t.Error("untrusted certificate: expected an error")
	}
	c = check(TLS, listener(t))
	if err := c.Do(); err == nil {
		t.Error("plain TCP service: expected an error")
	}
	c = check(TLS, closedPort(t))
	if err := c.Do(); err == nil {
		t.Error("closed port: expected an error")
	}
}

// The DTLS check does not pass against a UDP socket that does not speak DTLS.
func TestDTLSFailsAgainstPlainUDP(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pc.Close() }()
	c := check(DTLS, pc.LocalAddr().String())
	c.Timeout = Timeout(300 * time.Millisecond)
	if err := c.Do(); err == nil {
		t.Error("expected an error")
	}
}

// sshServer starts an SSH server that refuses every authentication.
func sshServer(t *testing.T) string {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) {
			return nil, errors.New("denied")
		},
	}
	config.AddHostKey(signer)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = c.Close() }()
				if sc, _, _, err := ssh.NewServerConn(c, config); err == nil {
					_ = sc.Close()
				}
			}()
		}
	}()
	return l.Addr().String()
}

// An SSH check passes when the server answers, even if it refuses the
// (absent) credentials; it fails against a service that is not SSH.
func TestSSH(t *testing.T) {
	c := check(SSH, sshServer(t))
	if err := c.Do(); err != nil {
		t.Errorf("SSH server: %v", err)
	}
	c = check(SSH, listener(t))
	if err := c.Do(); err == nil {
		t.Error("non-SSH service: expected an error")
	}
	c = check(SSH, closedPort(t))
	if err := c.Do(); err == nil {
		t.Error("closed port: expected an error")
	}
}

func TestHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	address := strings.TrimPrefix(server.URL, "http://")
	c := check(HTTP, address+"/health")
	if err := c.Do(); err != nil {
		t.Errorf("plain: %v", err)
	}
	// with sso the request goes through the SPNEGO transport, which needs a
	// Kerberos configuration (/etc/krb5.conf) that a test host may not have:
	// the outcome depends on the host, the path is only exercised here
	c = check(HTTP, address+"/health")
	c.SSO = true
	_ = c.Do()
	c = check(HTTP, closedPort(t))
	if err := c.Do(); err == nil {
		t.Error("closed port: expected an error")
	}
}

func TestHTTPSUntrustedCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()
	c := check(HTTPS, strings.TrimPrefix(server.URL, "https://"))
	if err := c.Do(); err == nil {
		t.Error("expected a certificate verification error")
	}
}

func TestCheckSerialisation(t *testing.T) {
	c := Check{Name: "n", Address: "a:1", Protocol: TLS, Timeout: Timeout(time.Second)}
	if !strings.Contains(c.ToJSON(), `"protocol": "tls"`) || !strings.Contains(c.ToYAML(), "protocol: tls") {
		t.Errorf("unexpected serialisation:\n%s\n%s", c.ToJSON(), c.ToYAML())
	}
}

// An ICMP check needs a raw socket (cap_net_raw), which a test host may lack:
// the result depends on the host, the check must in any case return.
func TestICMPReturns(t *testing.T) {
	c := check(ICMP, "127.0.0.1")
	c.Timeout = Timeout(time.Second)
	t.Logf("ICMP to localhost: %v", c.Do())
}
