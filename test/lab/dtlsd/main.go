// Command dtlsd is the lab's DTLS 1.2 server: it completes the handshake of
// every client that connects, concurrently, and closes. (openssl s_server
// serves one session at a time, which a soak with many DTLS checks outgrows.)
//
//	dtlsd -addr :4433 -cert server.pem -key server-key.pem
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net"
	"time"

	"github.com/pion/dtls/v3"
)

func main() {
	addr := flag.String("addr", ":4433", "UDP address to listen on")
	certFile := flag.String("cert", "", "certificate file")
	keyFile := flag.String("key", "", "private key file")
	flag.Parse()

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatal(err)
	}
	udp, err := net.ResolveUDPAddr("udp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	l, err := dtls.ListenWithOptions("udp", udp, dtls.WithCertificates(cert))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	log.Printf("dtlsd listening on %s", *addr)
	for {
		c, err := l.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			return
		}
		go func() {
			defer func() { _ = c.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if dc, ok := c.(*dtls.Conn); ok {
				if err := dc.HandshakeContext(ctx); err != nil {
					log.Printf("handshake: %v", err)
				}
			}
		}()
	}
}
