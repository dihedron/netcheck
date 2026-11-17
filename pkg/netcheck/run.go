package netcheck

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// Runner performs one attempt of a check. The Check it receives has its
// defaults applied, and ctx carries the attempt's deadline: a Runner must
// return when ctx ends. The Detail is kept whether or not err is nil.
type Runner interface {
	Run(ctx context.Context, c Check, d Defaults) (Detail, error)
}

// RunnerFunc makes a function a Runner.
type RunnerFunc func(ctx context.Context, c Check, d Defaults) (Detail, error)

// Run calls f.
func (f RunnerFunc) Run(ctx context.Context, c Check, d Defaults) (Detail, error) {
	return f(ctx, c, d)
}

// Option tunes Run and RunCheck.
type Option func(*config)

type config struct {
	defaults   Defaults
	limits     Limits
	logger     *slog.Logger
	runners    map[Protocol]Runner
	httpClient *http.Client
	allowed    []netip.Prefix
	denied     []netip.Prefix
}

// WithDefaults replaces the compiled-in defaults.
func WithDefaults(d Defaults) Option { return func(c *config) { c.defaults = d } }

// WithLimits replaces the bounds Validate enforces (T-2).
func WithLimits(l Limits) Option { return func(c *config) { c.limits = l } }

// WithLogger sets where the library logs; by default it logs nothing.
func WithLogger(l *slog.Logger) Option { return func(c *config) { c.logger = l } }

// WithRunner registers the Runner of a protocol, replacing a built-in one.
// It is how the dtls and sso sub-packages plug in.
func WithRunner(p Protocol, r Runner) Option { return func(c *config) { c.runners[p] = r } }

// WithHTTPClient sets the client of the http and https checks. The library
// never changes it: a check that needs its own transport gets a copy.
func WithHTTPClient(h *http.Client) Option { return func(c *config) { c.httpClient = h } }

// WithAllowedNetworks restricts the checks to the given networks (T-1): a
// connection to an address outside them fails with ErrNetworkNotAllowed. The
// test is made on the resolved address, at dial time, so redirects and DNS
// answers cannot lead out. With any network limit the http checks do not use
// a proxy, which would hide the real destination.
func WithAllowedNetworks(p ...netip.Prefix) Option { return func(c *config) { c.allowed = p } }

// WithDeniedNetworks forbids the given networks; it wins over WithAllowedNetworks.
func WithDeniedNetworks(p ...netip.Prefix) Option { return func(c *config) { c.denied = p } }

func newConfig(opts []Option) *config {
	c := &config{
		defaults: DefaultDefaults(),
		limits:   DefaultLimits(),
		logger:   slog.New(slog.DiscardHandler),
		runners:  map[Protocol]Runner{},
	}
	for _, o := range opts {
		o(c)
	}
	for p, r := range c.builtins() {
		if _, ok := c.runners[p]; !ok {
			c.runners[p] = r
		}
	}
	return c
}

// Run executes every check of the bundle, at most Concurrency at a time, and
// returns one Report. The bundle is not modified. It returns an error and no
// Report when the bundle is invalid; when ctx ends early it returns the
// Report, with the checks that did not finish marked Skipped, and ctx's error.
func Run(ctx context.Context, b *Bundle, opts ...Option) (*Report, error) {
	cfg := newConfig(opts)
	if err := b.validate(cfg.limits); err != nil {
		return nil, fmt.Errorf("invalid bundle: %w", err)
	}
	checks := make([]Check, len(b.Checks))
	for i, c := range b.Checks {
		checks[i] = cfg.resolve(c, b)
	}
	rep := &Report{BundleID: b.ID, Started: time.Now(), Results: make([]Result, len(checks))}
	for i := range checks {
		rep.Results[i] = Result{Check: redactCheck(checks[i]), Status: Skipped}
	}

	workers := b.Concurrency
	if workers < 1 {
		workers = cfg.defaults.Concurrency
	}
	workers = max(1, min(workers, len(checks)))

	jobs := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range jobs {
				rep.Results[i] = cfg.execute(ctx, checks[i])
			}
		})
	}
dispatch:
	for i := range checks {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break dispatch
		}
	}
	close(jobs)
	wg.Wait()
	rep.Duration = time.Since(rep.Started)

	if rep.count(Skipped) > 0 {
		return rep, ctx.Err()
	}
	return rep, nil
}

// RunCheck executes a single check, with the same semantics as Run.
func RunCheck(ctx context.Context, c Check, opts ...Option) Result {
	cfg := newConfig(opts)
	c = cfg.resolve(c, &Bundle{})
	if err := c.validate(cfg.limits); err != nil {
		return Result{Check: redactCheck(c), Status: Failed, Err: fmt.Errorf("invalid check: %w", err)}
	}
	return cfg.execute(ctx, c)
}

// resolve fills the check's zero settings from the bundle, then the defaults.
func (cfg *config) resolve(c Check, b *Bundle) Check {
	d := cfg.defaults
	c.Timeout = first(c.Timeout, b.Timeout, d.Timeout)
	c.Wait = first(c.Wait, b.Wait, d.Wait)
	c.Retries = first(c.Retries, b.Retries, d.Retries)
	return c
}

func first[T int | Duration](values ...T) T {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

// redactCheck returns the check as it appears in a Result: without credentials.
func redactCheck(c Check) Check {
	c.Address = redact(c.Address)
	return c
}

// execute runs the attempts of one check.
func (cfg *config) execute(ctx context.Context, c Check) Result {
	res := Result{Check: redactCheck(c), Status: Failed}
	runner, ok := cfg.runners[c.Protocol]
	if !ok {
		res.Err = fmt.Errorf("no runner registered for protocol %q", c.Protocol)
		return res
	}
	log := cfg.logger.With("check", c.Name, "protocol", string(c.Protocol), "address", redact(c.Address))
	for attempt := 1; attempt <= c.Retries && ctx.Err() == nil; attempt++ {
		actx, cancel := context.WithTimeout(ctx, time.Duration(c.Timeout))
		start := time.Now()
		detail, err := call(actx, runner, c, cfg.defaults)
		res.Latency = time.Since(start)
		cancel()
		res.Attempts, res.Detail, res.Err = attempt, detail, scrub(err, c.Address)
		if err == nil {
			log.Debug("check passed", "attempt", attempt, "latency", res.Latency)
			res.Status = Passed
			return res
		}
		log.Debug("check attempt failed", "attempt", attempt, "latency", res.Latency, "error", res.Err.Error())
		if attempt < c.Retries && !sleep(ctx, time.Duration(c.Wait)) {
			break
		}
	}
	if ctx.Err() != nil {
		res.Status, res.Err = Skipped, ctx.Err()
	}
	return res
}

// call runs one attempt, turning a panic into an error (T-10).
func call(ctx context.Context, r Runner, c Check, d Defaults) (detail Detail, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("runner panic: %v", p)
		}
	}()
	return r.Run(ctx, c, d)
}

// sleep waits for d or for ctx, and reports whether the full time passed.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// scrubbed is an error whose text has lost the credentials of the address.
type scrubbed struct {
	error
	msg string
}

func (e scrubbed) Error() string { return e.msg }
func (e scrubbed) Unwrap() error { return e.error }

// scrub removes the user information of the address from the error's text.
func scrub(err error, address string) error {
	if err == nil {
		return nil
	}
	i := strings.LastIndex(address[:authorityEnd(address)], "@")
	if i < 0 || !strings.Contains(err.Error(), address[:i+1]) {
		return err
	}
	return scrubbed{err, strings.ReplaceAll(err.Error(), address[:i+1], "")}
}

// builtins returns the runners the core provides.
func (cfg *config) builtins() map[Protocol]Runner {
	d := cfg.dialer()
	return map[Protocol]Runner{
		TCP: tcpRunner{d}, UDP: udpRunner{d}, ICMP: icmpRunner{cfg}, TLS: tlsRunner{d}, SSH: sshRunner{d},
		HTTP: httpRunner{cfg: cfg}, HTTPS: httpRunner{cfg: cfg},
	}
}
