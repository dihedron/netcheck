package main

import (
	"crypto/tls"
	"net"
	"os"
	"sync"
	"time"

	"github.com/pion/dtls/v3"
	dtlsnet "github.com/pion/dtls/v3/pkg/net"
)

// pion's own listener hands every datagram of a source address to the
// connection that address had first, until that connection closes. A check
// that gave up leaves a half-done handshake behind, and the next check that
// is given the same ephemeral port is then ignored for as long as the server
// keeps the old one (the soak found it: .specs/netcheck-lab.md). This
// listener tells the two apart by the random of the ClientHello (a retry of
// the same handshake repeats it) and gives a new handshake a new connection.

const (
	recordHeader    = 13 // type, version, epoch, sequence, length
	handshakeHeader = 12 // type, length, message sequence, fragment offset and length
	helloRandomAt   = recordHeader + handshakeHeader + 2
	maxDatagram     = 8192
	queue           = 128 // datagrams and connections that wait
)

// clientHello returns the random of a datagram that starts with the first
// fragment of a ClientHello, and whether it is one.
func clientHello(p []byte) (random [32]byte, ok bool) {
	if len(p) < helloRandomAt+len(random) {
		return random, false
	}
	const handshake, clientHello = 22, 1
	if p[0] != handshake || p[3] != 0 || p[4] != 0 || p[recordHeader] != clientHello { // epoch 0
		return random, false
	}
	if p[recordHeader+6] != 0 || p[recordHeader+7] != 0 || p[recordHeader+8] != 0 { // a later fragment
		return random, false
	}
	copy(random[:], p[helloRandomAt:])
	return random, true
}

// listen starts the server's DTLS listener on addr.
func listen(addr *net.UDPAddr, cert tls.Certificate) (net.Listener, error) {
	pc, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	l := &packetListener{pc: pc, conns: map[string]*packetConn{}, accept: make(chan *packetConn, queue), closed: make(chan struct{})}
	go l.read()
	d, err := dtls.NewListenerWithOptions(l, dtls.WithCertificates(cert))
	if err != nil {
		_ = l.Close()
		return nil, err
	}
	return d, nil
}

type packetListener struct {
	pc     *net.UDPConn
	accept chan *packetConn
	closed chan struct{}
	once   sync.Once

	mu    sync.Mutex
	conns map[string]*packetConn
}

var _ dtlsnet.PacketListener = (*packetListener)(nil)

func (l *packetListener) read() {
	buf := make([]byte, maxDatagram)
	for {
		n, raddr, err := l.pc.ReadFrom(buf)
		if err != nil {
			_ = l.Close()
			return
		}
		l.route(raddr, append([]byte(nil), buf[:n]...))
	}
}

// route hands a datagram to the connection of its source, a new one when it
// starts a handshake that the current one is not part of.
func (l *packetListener) route(raddr net.Addr, p []byte) {
	random, hello := clientHello(p)
	key := raddr.String()
	l.mu.Lock()
	c := l.conns[key]
	if c != nil && hello && c.random != random {
		delete(l.conns, key) // the old handshake is dead: its client has a new one
		go func(old *packetConn) { _ = old.Close() }(c)
		c = nil
	}
	if c == nil {
		if !hello {
			l.mu.Unlock()
			return
		}
		c = &packetConn{l: l, raddr: raddr, random: random, in: make(chan []byte, queue), done: make(chan struct{}), wake: make(chan struct{})}
		select {
		case l.accept <- c:
			l.conns[key] = c
		default: // too many handshakes waiting: the client retries
			l.mu.Unlock()
			return
		}
	}
	l.mu.Unlock()
	select {
	case c.in <- p:
	default: // a full queue drops, as a network does
	}
}

func (l *packetListener) Accept() (net.PacketConn, net.Addr, error) {
	select {
	case c := <-l.accept:
		return c, c.raddr, nil
	case <-l.closed:
		return nil, nil, net.ErrClosed
	}
}

func (l *packetListener) Close() error {
	var err error
	l.once.Do(func() {
		close(l.closed)
		err = l.pc.Close()
	})
	return err
}

func (l *packetListener) Addr() net.Addr { return l.pc.LocalAddr() }

// packetConn is one handshake's view of the socket.
type packetConn struct {
	l      *packetListener
	raddr  net.Addr
	random [32]byte
	in     chan []byte
	done   chan struct{}
	once   sync.Once

	mu       sync.Mutex
	deadline time.Time
	wake     chan struct{} // closed when the deadline changes
}

func (c *packetConn) ReadFrom(b []byte) (int, net.Addr, error) {
	for {
		c.mu.Lock()
		deadline, wake := c.deadline, c.wake
		c.mu.Unlock()
		var expired <-chan time.Time
		if !deadline.IsZero() {
			d := time.Until(deadline)
			if d <= 0 {
				return 0, nil, os.ErrDeadlineExceeded
			}
			t := time.NewTimer(d)
			defer t.Stop()
			expired = t.C
		}
		select {
		case p := <-c.in:
			return copy(b, p), c.raddr, nil
		case <-c.done:
			return 0, nil, net.ErrClosed
		case <-expired:
			return 0, nil, os.ErrDeadlineExceeded
		case <-wake:
		}
	}
}

func (c *packetConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	select {
	case <-c.done:
		return 0, net.ErrClosed
	default:
	}
	return c.l.pc.WriteTo(p, addr)
}

func (c *packetConn) Close() error {
	c.once.Do(func() {
		close(c.done)
		c.l.mu.Lock()
		if c.l.conns[c.raddr.String()] == c {
			delete(c.l.conns, c.raddr.String())
		}
		c.l.mu.Unlock()
	})
	return nil
}

func (c *packetConn) LocalAddr() net.Addr { return c.l.pc.LocalAddr() }

func (c *packetConn) SetDeadline(t time.Time) error { return c.SetReadDeadline(t) }

func (c *packetConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	c.deadline = t
	close(c.wake)
	c.wake = make(chan struct{})
	c.mu.Unlock()
	return nil
}

func (c *packetConn) SetWriteDeadline(time.Time) error { return nil }
