package netcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"
	"gopkg.in/yaml.v3"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// fake registers fn as the runner of every protocol a test uses.
func fake(fn func(ctx context.Context, c Check, d Defaults) (Detail, error)) Option {
	return WithRunner(TCP, RunnerFunc(fn))
}

func tcpChecks(n int) []Check {
	cs := make([]Check, n)
	for i := range cs {
		cs[i] = Check{Name: string(rune('a' + i)), Address: "h:1", Protocol: TCP}
	}
	return cs
}

func TestRunKeepsOrderAndAppliesDefaults(t *testing.T) {
	b := &Bundle{ID: "b", Timeout: Duration(3 * time.Second), Retries: 2, Wait: Duration(time.Millisecond), Checks: tcpChecks(5)}
	b.Checks[1].Retries = 4
	rep, err := Run(context.Background(), b, fake(func(_ context.Context, c Check, _ Defaults) (Detail, error) {
		return Detail{Remote: c.Name}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if rep.BundleID != "b" || len(rep.Results) != 5 || !rep.OK() || rep.Failed() != 0 || rep.Started.IsZero() {
		t.Fatalf("report %+v", rep)
	}
	for i, r := range rep.Results {
		if r.Detail.Remote != b.Checks[i].Name || r.Status != Passed || r.Attempts != 1 || r.Err != nil {
			t.Errorf("result %d out of order or wrong: %+v", i, r)
		}
		if r.Check.Timeout != b.Timeout {
			t.Errorf("result %d: timeout %v, want the bundle's", i, r.Check.Timeout)
		}
	}
	if rep.Results[0].Check.Retries != 2 || rep.Results[1].Check.Retries != 4 {
		t.Error("retries: the check's value wins over the bundle's")
	}
	if b.Checks[0].Timeout != 0 {
		t.Error("Run must not modify the bundle")
	}
}

func TestRunRetries(t *testing.T) {
	var calls atomic.Int32
	b := &Bundle{Retries: 3, Wait: Duration(time.Millisecond), Checks: tcpChecks(1)}
	rep, _ := Run(context.Background(), b, fake(func(context.Context, Check, Defaults) (Detail, error) {
		if calls.Add(1) < 3 {
			return Detail{}, errors.New("not yet")
		}
		return Detail{}, nil
	}))
	if r := rep.Results[0]; r.Status != Passed || r.Attempts != 3 {
		t.Fatalf("%+v", r)
	}
}

// NC-05: no wait after the last attempt.
func TestRunNoWaitAfterLastAttempt(t *testing.T) {
	var calls atomic.Int32
	wait := 150 * time.Millisecond
	b := &Bundle{Retries: 3, Wait: Duration(wait), Checks: tcpChecks(1)}
	start := time.Now()
	rep, _ := Run(context.Background(), b, fake(func(context.Context, Check, Defaults) (Detail, error) {
		calls.Add(1)
		return Detail{}, errors.New("no")
	}))
	elapsed := time.Since(start)
	r := rep.Results[0]
	if r.Status != Failed || r.Attempts != 3 || calls.Load() != 3 || r.Err == nil || rep.OK() || rep.Failed() != 1 {
		t.Fatalf("%+v", r)
	}
	if elapsed < 2*wait || elapsed >= 3*wait {
		t.Fatalf("elapsed %v: want 2 waits (%v), not 3", elapsed, 2*wait)
	}
}

// NC-04 and the pool: the concurrency is respected, and 0 or less means the default.
func TestRunConcurrency(t *testing.T) {
	for _, tc := range []struct{ concurrency, wantMax int }{{2, 2}, {1, 1}, {0, 10}, {-5, 10}} {
		var cur, max atomic.Int32
		b := &Bundle{Concurrency: tc.concurrency, Checks: tcpChecks(30)}
		rep, err := Run(context.Background(), b, fake(func(context.Context, Check, Defaults) (Detail, error) {
			n := cur.Add(1)
			for m := max.Load(); n > m && !max.CompareAndSwap(m, n); m = max.Load() {
			}
			time.Sleep(5 * time.Millisecond)
			cur.Add(-1)
			return Detail{}, nil
		}))
		if err != nil || !rep.OK() {
			t.Fatalf("concurrency %d: %v", tc.concurrency, err)
		}
		if got := max.Load(); got > int32(tc.wantMax) || (tc.wantMax <= 2 && got != int32(tc.wantMax)) {
			t.Errorf("concurrency %d: peak %d, want at most %d", tc.concurrency, got, tc.wantMax)
		}
	}
}

// NC-03: cancellation returns promptly, marks the rest Skipped, leaks nothing.
func TestRunCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	b := &Bundle{Concurrency: 2, Checks: tcpChecks(10)}
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	start := time.Now()
	rep, err := Run(ctx, b, fake(func(ctx context.Context, _ Check, _ Defaults) (Detail, error) {
		<-ctx.Done()
		return Detail{}, ctx.Err()
	}))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("Run did not return promptly")
	}
	if rep == nil || len(rep.Results) != 10 || rep.OK() {
		t.Fatalf("report %+v", rep)
	}
	for i, r := range rep.Results {
		if r.Status != Skipped {
			t.Errorf("result %d: %s, want skipped", i, r.Status)
		}
	}
}

func TestRunAttemptTimeout(t *testing.T) {
	b := &Bundle{Timeout: Duration(50 * time.Millisecond), Retries: 1, Checks: tcpChecks(1)}
	rep, _ := Run(context.Background(), b, fake(func(ctx context.Context, _ Check, _ Defaults) (Detail, error) {
		<-ctx.Done()
		return Detail{}, ctx.Err()
	}))
	if r := rep.Results[0]; r.Status != Failed || !errors.Is(r.Err, context.DeadlineExceeded) {
		t.Fatalf("a timed-out attempt is a failure, not a skip: %+v", r)
	}
}

// T-10: a panicking runner fails its check, not the process.
func TestRunRecoversFromPanic(t *testing.T) {
	b := &Bundle{Retries: 1, Checks: tcpChecks(3)}
	rep, _ := Run(context.Background(), b, fake(func(_ context.Context, c Check, _ Defaults) (Detail, error) {
		if c.Name == "b" {
			panic("boom")
		}
		return Detail{}, nil
	}))
	if rep.Results[0].Status != Passed || rep.Results[2].Status != Passed {
		t.Error("the other checks must run")
	}
	if r := rep.Results[1]; r.Status != Failed || r.Err == nil || !strings.Contains(r.Err.Error(), "boom") {
		t.Fatalf("%+v", r)
	}
}

func TestRunWithoutRunner(t *testing.T) {
	b := &Bundle{Retries: 1, Checks: []Check{{Address: "h:1", Protocol: DTLS}}}
	rep, _ := Run(context.Background(), b)
	if r := rep.Results[0]; r.Status != Failed || !strings.Contains(r.Err.Error(), "dtls") {
		t.Fatalf("want an error naming the protocol: %+v", r)
	}
}

func TestRunRejectsInvalidBundle(t *testing.T) {
	rep, err := Run(context.Background(), &Bundle{Checks: []Check{{Protocol: "x"}}})
	if err == nil || rep != nil {
		t.Fatalf("got %v %v", rep, err)
	}
	if _, err := Run(context.Background(), &Bundle{Checks: tcpChecks(3)}, WithLimits(Limits{MaxChecks: 2, MaxRetries: 1, MaxTimeout: 1, MaxConcurrency: 1})); err == nil {
		t.Fatal("WithLimits must apply")
	}
}

func TestRunCheck(t *testing.T) {
	r := RunCheck(context.Background(), Check{Address: "h:1", Protocol: TCP, Retries: 1}, fake(func(context.Context, Check, Defaults) (Detail, error) {
		return Detail{Remote: "x"}, nil
	}))
	if r.Status != Passed || r.Detail.Remote != "x" || r.Check.Timeout == 0 {
		t.Fatalf("%+v", r)
	}
	if r := RunCheck(context.Background(), Check{Protocol: TCP}); r.Status != Failed || r.Err == nil {
		t.Fatalf("an invalid check fails: %+v", r)
	}
}

func TestWithDefaults(t *testing.T) {
	d := DefaultDefaults()
	d.Timeout = Duration(7 * time.Second)
	var got Duration
	RunCheck(context.Background(), Check{Address: "h:1", Protocol: TCP}, WithDefaults(d), fake(func(_ context.Context, c Check, _ Defaults) (Detail, error) {
		got = c.Timeout
		return Detail{}, nil
	}))
	if got != d.Timeout {
		t.Fatalf("timeout %v", got)
	}
}

// Nothing is logged unless a logger is given.
func TestWithLogger(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	b := &Bundle{Retries: 1, Checks: tcpChecks(1)}
	Run(context.Background(), b, WithLogger(log), fake(func(context.Context, Check, Defaults) (Detail, error) { return Detail{}, errors.New("bad") }))
	if !strings.Contains(buf.String(), "bad") {
		t.Fatalf("expected the failure in the log: %q", buf.String())
	}
}

// NC-17: credentials in an address never reach the log.
func TestLogsAreRedacted(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	log := slog.New(slog.NewTextHandler(writerFunc(func(p []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return buf.Write(p) }), &slog.HandlerOptions{Level: slog.LevelDebug}))
	b := &Bundle{Retries: 1, Checks: []Check{{Address: "user:s3cret@host:80/x", Protocol: HTTP}}}
	rep, _ := Run(context.Background(), b, WithLogger(log), WithRunner(HTTP, RunnerFunc(func(context.Context, Check, Defaults) (Detail, error) {
		return Detail{}, errors.New("failed")
	})))
	if strings.Contains(buf.String(), "s3cret") {
		t.Fatalf("credentials in the log: %s", buf.String())
	}
	if strings.Contains(rep.Results[0].Check.Address, "s3cret") {
		t.Fatalf("credentials in the report: %+v", rep.Results[0].Check)
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func TestResultMarshalling(t *testing.T) {
	in := Result{Check: Check{Address: "h:1", Protocol: TCP}, Status: Failed, Attempts: 2, Latency: 15 * time.Millisecond,
		Err: errors.New("refused"), Detail: Detail{HTTPStatus: 503, TLS: &TLSFacts{Verified: true}}}
	data, err := json.Marshal(in)
	if err != nil || !strings.Contains(string(data), `"error":"refused"`) || !strings.Contains(string(data), `"latency":"15ms"`) {
		t.Fatalf("%s %v", data, err)
	}
	var out Result
	if err := json.Unmarshal(data, &out); err != nil || out.Err.Error() != "refused" || out.Detail.HTTPStatus != 503 || out.Latency != in.Latency || out.Status != Failed {
		t.Fatalf("%+v %v", out, err)
	}
	data, err = yaml.Marshal(in)
	if err != nil || !strings.Contains(string(data), "status: failed") {
		t.Fatalf("%s %v", data, err)
	}
	out = Result{}
	if err := yaml.Unmarshal(data, &out); err != nil || out.Err.Error() != "refused" || out.Attempts != 2 {
		t.Fatalf("%+v %v", out, err)
	}
	ok := Result{Status: Passed}
	if data, _ := json.Marshal(ok); strings.Contains(string(data), "error") {
		t.Errorf("no error field when passed: %s", data)
	}
}
