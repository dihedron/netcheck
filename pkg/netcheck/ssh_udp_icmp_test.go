package netcheck

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// sshServer starts an SSH server that refuses every authentication and
// returns its address and the fingerprint of its host key.
func sshServer(t *testing.T) (string, string) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { return nil, errors.New("denied") }}
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
	return l.Addr().String(), ssh.FingerprintSHA256(signer.PublicKey())
}

// NC-11
func TestSSH(t *testing.T) {
	addr, fingerprint := sshServer(t)
	r := runOne(t, SSH, addr)
	wantPassed(t, r)
	if r.Detail.SSHHostKey != fingerprint || !strings.HasPrefix(r.Detail.SSHHostKey, "SHA256:") {
		t.Errorf("fingerprint %q, want %q", r.Detail.SSHHostKey, fingerprint)
	}
	if !strings.HasPrefix(r.Detail.SSHBanner, "SSH-2.0-") {
		t.Errorf("banner %q", r.Detail.SSHBanner)
	}
	wantFailed(t, runOne(t, SSH, listener(t)))
	wantFailed(t, runOne(t, SSH, closedPort(t)))
}

// NC-12
func TestUDP(t *testing.T) {
	grace := WithDefaults(func() Defaults { d := DefaultDefaults(); d.UDPGrace = Duration(100 * time.Millisecond); return d }())
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	wantPassed(t, runOne(t, UDP, pc.LocalAddr().String(), grace))

	gone, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := gone.LocalAddr().String()
	_ = gone.Close()
	wantFailed(t, runOne(t, UDP, addr, grace)) // the host answers ICMP port unreachable
}

// NC-09: the result depends on whether the host lets us send ICMP.
func TestICMP(t *testing.T) {
	r := runOne(t, ICMP, "127.0.0.1")
	if r.Status != Passed {
		t.Skipf("no ICMP on this host (%v): needs cap_net_raw or net.ipv4.ping_group_range", r.Err)
	}
	if r.Detail.PacketsRecv < 1 || r.Detail.PacketsSent < r.Detail.PacketsRecv {
		t.Errorf("%+v", r.Detail)
	}
}

func TestICMPFailures(t *testing.T) {
	d := DefaultDefaults()
	d.Ping.Count = 1
	wantFailed(t, RunCheck(t.Context(), Check{Address: "192.0.2.1", Protocol: ICMP, Retries: 1, Timeout: Duration(300 * time.Millisecond)}, WithDefaults(d))) // 100% loss
	wantFailed(t, runOne(t, ICMP, "no-such-host.invalid"))                                                                                                    // a bad address is an error, not a panic
}
