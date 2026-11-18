// Command dtlsprobe is the lab's DTLS stress client: it abandons handshakes
// half way and then handshakes again from the same UDP source port, which
// is what a client does when the ephemeral port it was given has just been
// used by a check that gave up (the soak found the lab's server ignoring such
// a client; see .specs/netcheck-lab.md). It runs on nc-client, beside the
// package under test, and does not verify the server's certificate: it tests
// the server, not the check.
//
//	dtlsprobe -addr nc-servers.lab:4433 -rounds 200 -workers 16
//
// It prints one line per failed round and a summary, and exits 1 when any
// round failed.
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/dtls/v3"
)

func main() {
	addr := flag.String("addr", "nc-servers.lab:4433", "the DTLS server")
	rounds := flag.Int("rounds", 200, "rounds to run")
	workers := flag.Int("workers", 16, "rounds at once")
	timeout := flag.Duration("timeout", 4*time.Second, "the time a handshake has, as a check's timeout")
	reuse := flag.Bool("reuse", true, "abandon a handshake first, then reuse its source port; false: plain handshakes")
	flag.Parse()

	raddr, err := net.ResolveUDPAddr("udp", *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dtlsprobe:", err)
		os.Exit(2)
	}
	var failed, done atomic.Int64
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range *workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range jobs {
				if err := round(raddr, *timeout, *reuse); err != nil {
					failed.Add(1)
					fmt.Printf("round %d: %v\n", n, err)
				}
				done.Add(1)
			}
		}()
	}
	for n := range *rounds {
		jobs <- n
	}
	close(jobs)
	wg.Wait()
	fmt.Printf("%d rounds, %d failed\n", done.Load(), failed.Load())
	if failed.Load() > 0 {
		os.Exit(1)
	}
}

// round abandons a handshake (when reuse is set), waits a little, and
// handshakes again from the same source port.
func round(raddr *net.UDPAddr, timeout time.Duration, reuse bool) error {
	port := 0
	if reuse {
		pc, err := net.ListenUDP("udp", nil)
		if err != nil {
			return fmt.Errorf("opening a socket: %w", err)
		}
		port = pc.LocalAddr().(*net.UDPAddr).Port
		// the ClientHello goes out, and the answer is never read
		if c, err := dtls.ClientWithOptions(pc, raddr, dtls.WithInsecureSkipVerify(true)); err == nil { //nolint:gosec // lab probe of the server
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(rand.IntN(4))*time.Millisecond)
			_ = c.HandshakeContext(ctx)
			cancel()
			_ = c.Close()
		} else {
			_ = pc.Close()
		}
		// the new check comes within the server's handshake timeout
		time.Sleep(time.Duration(rand.IntN(3000)) * time.Millisecond)
	}
	pc, err := net.ListenUDP("udp", &net.UDPAddr{Port: port})
	if err != nil {
		return fmt.Errorf("reopening port %d: %w", port, err)
	}
	c, err := dtls.ClientWithOptions(pc, raddr, dtls.WithInsecureSkipVerify(true)) //nolint:gosec // lab probe of the server
	if err != nil {
		_ = pc.Close()
		return fmt.Errorf("creating the client: %w", err)
	}
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := c.HandshakeContext(ctx); err != nil {
		return fmt.Errorf("handshake from port %d: %w", port, err)
	}
	return nil
}
