//go:build stress

package netcheck

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
)

// The stress test runs thousands of checks against local peers that behave
// badly at random: they accept, refuse, say nothing, reset the connection,
// answer slowly or answer with an error. Every round checks that each result
// matches what its peer was told to do, that Run returns within a bound and
// that, when the run is cancelled at a random moment, every check ends with a
// status and Skipped appears only after the cancellation. goleak (TestMain)
// fails the run when a goroutine outlives it. Run it with the race detector:
//
//	go test -race -tags stress -run TestStress -timeout 0 ./pkg/netcheck
//
// NETCHECK_STRESS_DURATION (default 20s) sets how long it keeps starting
// rounds, NETCHECK_STRESS_CHECKS (default 5000) how many checks a round has,
// NETCHECK_STRESS_SEED the seed of the first round. The soak run of the
// operator's plan is this test with a duration of hours.

func stressEnv(name string, def int64) int64 {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

// peer is one behaviour of a server and the status it must produce.
type peer struct {
	name    string
	address string
	proto   Protocol
	want    Status
}

// silentTLS accepts connections and never speaks; resetTLS accepts and closes.
func listen(t *testing.T, serve func(net.Conn)) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	t.Cleanup(func() { _ = l.Close(); <-done })
	go func() {
		defer close(done)
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go serve(c)
		}
	}()
	return l.Addr().String()
}

func stressPeers(t *testing.T) []peer {
	t.Helper()
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	refused := closed.Addr().String()
	_ = closed.Close()

	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	silent := listen(t, func(c net.Conn) {
		// never speaks; gives up after a while, or a soak would hold every
		// connection of every round until the end
		select {
		case <-stop:
		case <-time.After(3 * time.Second):
		}
		_ = c.Close()
	})
	reset := listen(t, func(c net.Conn) { _ = c.Close() })
	open := listen(t, func(c net.Conn) { _ = c.Close() })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
		case "/slow": // longer than the timeout of the checks
			select {
			case <-r.Context().Done():
			case <-stop:
			case <-time.After(5 * time.Second):
			}
		}
	}))
	t.Cleanup(srv.Close)
	host := srv.Listener.Addr().String()

	return []peer{
		{"tcp open", open, TCP, Passed},
		{"tcp refused", refused, TCP, Failed},
		{"tls silent", silent, TLS, Failed},
		{"tls reset", reset, TLS, Failed},
		{"http ok", host + "/ok", HTTP, Passed},
		{"http error", host + "/error", HTTP, Failed},
		{"http slow", host + "/slow", HTTP, Failed},
	}
}

func TestStress(t *testing.T) {
	peers := stressPeers(t)
	checks := int(stressEnv("NETCHECK_STRESS_CHECKS", 5000))
	duration := time.Duration(stressEnv("NETCHECK_STRESS_DURATION", 20)) * time.Second
	seed := stressEnv("NETCHECK_STRESS_SEED", time.Now().UnixNano())

	const (
		timeout     = 150 * time.Millisecond
		retries     = 2
		wait        = 10 * time.Millisecond
		concurrency = 200
	)
	// the worst case for a round: every check takes the full timeout on every
	// attempt, concurrency checks at a time
	bound := time.Duration((checks+concurrency-1)/concurrency)*(retries*timeout+wait) + 5*time.Second

	// the default limits stop a bundle of this size: lift them
	limits := DefaultLimits()
	limits.MaxChecks = checks
	lift := WithLimits(limits)

	deadline := time.Now().Add(duration)
	for round := 0; time.Now().Before(deadline) || round == 0; round++ {
		rng := rand.New(rand.NewSource(seed + int64(round))) //nolint:gosec // a test, the seed is printed to replay a failure
		bundle := &Bundle{ID: fmt.Sprintf("stress-%d", seed+int64(round)), Concurrency: concurrency, Retries: retries, Wait: Duration(wait), Timeout: Duration(timeout)}
		truth := make([]peer, checks)
		for i := range truth {
			p := peers[rng.Intn(len(peers))]
			truth[i] = p
			bundle.Checks = append(bundle.Checks, Check{Name: fmt.Sprintf("%s #%d", p.name, i), Address: p.address, Protocol: p.proto})
		}

		// a calm round: every result is what its peer was told to produce
		start := time.Now()
		report, err := Run(context.Background(), bundle, lift)
		if err != nil {
			t.Fatalf("round %d (seed %d): %v", round, seed+int64(round), err)
		}
		if took := time.Since(start); took > bound {
			t.Errorf("round %d (seed %d): took %v, bound %v", round, seed+int64(round), took, bound)
		}
		if len(report.Results) != checks {
			t.Fatalf("round %d (seed %d): %d results for %d checks", round, seed+int64(round), len(report.Results), checks)
		}
		for i, r := range report.Results {
			if r.Status != truth[i].want {
				t.Errorf("round %d (seed %d): %s: got %s (%v), want %s", round, seed+int64(round), r.Check.Name, r.Status, r.Err, truth[i].want)
			}
		}

		// a cancelled round: the run stops at a random moment
		cancelAfter := time.Duration(rng.Intn(int(bound/4/time.Millisecond)+1)) * time.Millisecond
		if cancelAfter > 2*time.Second {
			cancelAfter = 2 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), cancelAfter)
		start = time.Now()
		report, err = Run(ctx, bundle, lift)
		cancel()
		if took := time.Since(start); took > cancelAfter+3*time.Second {
			t.Errorf("round %d (seed %d): cancelled after %v but returned after %v", round, seed+int64(round), cancelAfter, took)
		}
		if report == nil || len(report.Results) != checks {
			t.Fatalf("round %d (seed %d): the cancelled run lost results", round, seed+int64(round))
		}
		skipped := 0
		for _, r := range report.Results {
			switch r.Status {
			case Passed, Failed:
			case Skipped:
				skipped++
			default:
				t.Errorf("round %d (seed %d): %s has the status %q", round, seed+int64(round), r.Check.Name, r.Status)
			}
		}
		if skipped > 0 && err == nil {
			t.Errorf("round %d (seed %d): %d checks skipped but Run returned no error", round, seed+int64(round), skipped)
		}
		t.Logf("round %d: %d checks, cancelled after %v, %d skipped", round, checks, cancelAfter, skipped)
	}
}
