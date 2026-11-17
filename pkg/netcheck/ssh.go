package netcheck

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// sshRunner passes when the SSH transport handshake completes: a server that
// refuses the (absent) credentials is a pass. It proves reachability, not
// identity: the host key is recorded in the Detail and never verified (T-6).
type sshRunner struct{}

func (sshRunner) Run(ctx context.Context, c Check, _ Defaults) (Detail, error) {
	target, err := dialTarget(SSH, c.Address)
	if err != nil {
		return Detail{}, err
	}
	var d net.Dialer
	raw, err := d.DialContext(ctx, "tcp", target)
	if err != nil {
		return Detail{}, fmt.Errorf("connecting to %s: %w", target, err)
	}
	defer func() { _ = raw.Close() }()
	if dl, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(dl)
	}
	// the library does not expose the server's version string unless the
	// authentication succeeds, so the first line the server sends is kept
	conn := &bannerConn{Conn: raw}
	detail := Detail{Remote: raw.RemoteAddr().String()}
	config := &ssh.ClientConfig{
		User: "netcheck",
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			detail.SSHHostKey = ssh.FingerprintSHA256(key)
			return nil
		},
		Timeout: time.Until(deadlineOf(ctx)),
	}
	client, chans, reqs, err := ssh.NewClientConn(conn, target, config)
	detail.SSHBanner = conn.banner()
	if err != nil {
		// shortcut: x/crypto has no typed error for a refused authentication, so the text
		// is matched; the host key having been seen means the transport is up
		if detail.SSHHostKey != "" && strings.Contains(err.Error(), "unable to authenticate") {
			return detail, nil
		}
		return detail, fmt.Errorf("SSH handshake with %s: %w", target, err)
	}
	defer func() { _ = client.Close() }()
	go ssh.DiscardRequests(reqs)
	go func() {
		for ch := range chans {
			_ = ch.Reject(ssh.Prohibited, "")
		}
	}()
	return detail, nil
}

func deadlineOf(ctx context.Context) time.Time {
	if dl, ok := ctx.Deadline(); ok {
		return dl
	}
	return time.Now().Add(time.Minute)
}

// bannerConn remembers the first line read from the server, which is its
// "SSH-2.0-..." version string.
type bannerConn struct {
	net.Conn
	line []byte
	done bool
}

func (b *bannerConn) Read(p []byte) (int, error) {
	n, err := b.Conn.Read(p)
	for _, c := range p[:n] {
		if b.done || len(b.line) >= 255 {
			break
		}
		if c == '\n' {
			b.done = true
			break
		}
		b.line = append(b.line, c)
	}
	return n, err
}

func (b *bannerConn) banner() string { return strings.TrimSpace(string(b.line)) }
