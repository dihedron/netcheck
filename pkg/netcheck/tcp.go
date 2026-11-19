package netcheck

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// tcpRunner passes when a TCP connection is established.
type tcpRunner struct{ dialer *net.Dialer }

func (r tcpRunner) Run(ctx context.Context, c Check, _ Defaults) (Detail, error) {
	target, err := dialTarget(TCP, c.Address)
	if err != nil {
		return Detail{}, err
	}
	conn, err := r.dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return Detail{}, fmt.Errorf("connecting to %s: %w", target, err)
	}
	defer func() { _ = conn.Close() }()
	detail := Detail{Remote: conn.RemoteAddr().String()}
	if err := afterConnect(ctx, effectiveHooks(ctx, c), conn); err != nil {
		return detail, fmt.Errorf("after connecting to %s: %w", target, err)
	}
	return detail, nil
}

// udpRunner passes when the peer does not answer with an ICMP error. Dialling
// UDP proves nothing, so a datagram is sent and the socket is watched for
// Defaults.UDPGrace: a closed port makes the host reply "port unreachable",
// which the read reports. A silent peer is a pass; the check is best effort.
type udpRunner struct{ dialer *net.Dialer }

func (r udpRunner) Run(ctx context.Context, c Check, d Defaults) (Detail, error) {
	target, err := dialTarget(UDP, c.Address)
	if err != nil {
		return Detail{}, err
	}
	conn, err := r.dialer.DialContext(ctx, "udp", target)
	if err != nil {
		return Detail{}, fmt.Errorf("connecting to %s: %w", target, err)
	}
	defer func() { _ = conn.Close() }()
	detail := Detail{Remote: conn.RemoteAddr().String()}

	deadline := time.Now().Add(time.Duration(d.UDPGrace))
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return detail, fmt.Errorf("setting the deadline: %w", err)
	}
	if _, err := conn.Write(nil); err != nil {
		return detail, fmt.Errorf("sending to %s: %w", target, err)
	}
	var buf [1]byte
	if _, err := conn.Read(buf[:]); err != nil {
		var ne net.Error
		if errorsAsTimeout(err, &ne) {
			return detail, nil // nothing came back: no ICMP error
		}
		return detail, fmt.Errorf("%s did not accept the datagram: %w", target, err)
	}
	return detail, nil
}

func errorsAsTimeout(err error, ne *net.Error) bool {
	return errors.As(err, ne) && (*ne).Timeout()
}
