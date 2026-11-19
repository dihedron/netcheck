// Package postgres is the runner of the postgresql check. PostgreSQL does not
// start TLS on connect: the client sends an SSLRequest and the server answers
// one byte, and only then does the handshake begin. The check does the same
// and the handshake with the verification of the tls check, as sslmode=verify-full
// does. The core does not import this package:
//
//	netcheck.Run(ctx, bundle, netcheck.WithRunner(netcheck.PostgreSQL, postgres.Runner()))
//
// No startup packet and no credential is ever sent, and a server that answers
// that it has no TLS fails the check: it never falls back to plain text.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
)

// sslRequest is the message that asks the server for TLS: its length, 8, and
// the request code 80877103.
var sslRequest = []byte{0, 0, 0, 8, 0x04, 0xd2, 0x16, 0x2f}

// Runner returns the postgresql runner.
func Runner() netcheck.Runner { return runner{} }

type runner struct{}

func (runner) Run(ctx context.Context, c netcheck.Check, _ netcheck.Defaults) (netcheck.Detail, error) {
	var h netcheck.Hooks
	if c.Hooks != nil {
		h = *c.Hooks
	}
	own := h.AfterConnect
	h.AfterConnect = func(ctx context.Context, conn net.Conn) error {
		if err := negotiate(conn); err != nil {
			return err
		}
		if own != nil {
			return own(ctx, conn)
		}
		return nil
	}
	c.Hooks = &h
	return netcheck.TLSHandshake(ctx, netcheck.PostgreSQL, c)
}

// negotiate asks the server for TLS and reads its answer.
func negotiate(conn net.Conn) error {
	if _, err := conn.Write(sslRequest); err != nil {
		return fmt.Errorf("sending the SSLRequest: %w", err)
	}
	var b [1]byte
	if _, err := io.ReadFull(conn, b[:]); err != nil {
		return fmt.Errorf("reading the answer to the SSLRequest: %w", err)
	}
	return answer(b[0])
}

// answer says what the server's byte means: S, the handshake may begin.
func answer(b byte) error {
	switch b {
	case 'S':
		return nil
	case 'N':
		return errors.New("the server does not support TLS (it answered N)")
	case 'E':
		return errors.New("the server answered the SSLRequest with an error")
	}
	return fmt.Errorf("unexpected answer %q to the SSLRequest: not a PostgreSQL server?", b)
}
