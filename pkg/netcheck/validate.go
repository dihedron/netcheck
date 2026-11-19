package netcheck

import (
	"errors"
	"fmt"
	"time"
)

// Limits bound what a bundle may ask for, so that a bundle from an untrusted
// place cannot exhaust the host.
type Limits struct {
	MaxRetries     int
	MaxTimeout     Duration // also bounds the wait between attempts
	MaxConcurrency int
	MaxChecks      int
}

// DefaultLimits returns the limits Validate enforces: 10 attempts, 5
// minutes, 256 workers and 1,000 checks.
func DefaultLimits() Limits {
	return Limits{MaxRetries: 10, MaxTimeout: Duration(5 * time.Minute), MaxConcurrency: 256, MaxChecks: 1000}
}

// Validate reports every defect of the bundle at once, as joined errors.
func (b *Bundle) Validate() error { return b.validate(DefaultLimits()) }

func (b *Bundle) validate(l Limits) error {
	var errs []error
	if len(b.Checks) > l.MaxChecks {
		errs = append(errs, fmt.Errorf("the bundle has %d checks, more than the %d allowed", len(b.Checks), l.MaxChecks))
	}
	errs = append(errs, l.settings("bundle", b.Timeout, b.Wait, b.Retries)...)
	if b.Concurrency > l.MaxConcurrency {
		errs = append(errs, fmt.Errorf("bundle: concurrency %d exceeds the maximum %d", b.Concurrency, l.MaxConcurrency))
	}
	for i := range b.Checks {
		if err := b.Checks[i].validate(l); err != nil {
			errs = append(errs, fmt.Errorf("check %d: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

func (l Limits) settings(what string, timeout, wait Duration, retries int) (errs []error) {
	if timeout > l.MaxTimeout {
		errs = append(errs, fmt.Errorf("%s: timeout %v exceeds the maximum %v", what, timeout, l.MaxTimeout))
	}
	if wait > l.MaxTimeout {
		errs = append(errs, fmt.Errorf("%s: wait %v exceeds the maximum %v", what, wait, l.MaxTimeout))
	}
	if retries > l.MaxRetries {
		errs = append(errs, fmt.Errorf("%s: retries %d exceeds the maximum %d", what, retries, l.MaxRetries))
	}
	return errs
}

// Validate reports every defect of the check at once.
func (c *Check) Validate() error { return c.validate(DefaultLimits()) }

func (c *Check) validate(l Limits) error {
	errs := l.settings("check", c.Timeout, c.Wait, c.Retries)
	switch {
	case c.Protocol == "":
		errs = append(errs, errors.New("protocol is empty"))
	case !protocols[c.Protocol]:
		errs = append(errs, fmt.Errorf("unknown protocol %q", c.Protocol))
	case c.Protocol == ICMP:
		if c.Address == "" {
			errs = append(errs, errors.New("address is empty"))
		}
	default:
		if _, err := dialTarget(c.Protocol, c.Address); err != nil {
			errs = append(errs, fmt.Errorf("address %q: %w", redact(c.Address), err))
		}
	}
	errs = append(errs, c.validateHooks()...)
	return errors.Join(errs...)
}
