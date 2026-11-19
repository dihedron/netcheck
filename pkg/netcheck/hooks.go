package netcheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/netip"
	"time"
)

// Hooks are the points where a program can step into a check. A nil field is
// the default behaviour. Hooks are code: they are set through the API (WithHooks
// for a run, Check.Hooks for one check) and never read from a bundle file.
//
// For AfterConnect, VerifyCertificate and AfterHandshake the check's field
// replaces the run's. BeforeConnect is a veto: the networks limits, the run's
// hook and the check's hook all run, and any of them can refuse; none can lift
// a limit.
//
// A hook that panics fails the check. Validate rejects a check that sets a
// hook its protocol does not honour.
type Hooks struct {
	// BeforeConnect is called with every address a check is about to connect
	// to, after name resolution; an error refuses it (ErrNetworkNotAllowed
	// wraps it). Every protocol honours it. With an http or https check that
	// goes through a proxy it sees the proxy.
	BeforeConnect func(ctx context.Context, ip netip.Addr) error
	// AfterConnect is called when the TCP connection is up and nothing has
	// been sent; the hook may talk on conn (STARTTLS). An error fails the
	// check. Honoured by tcp, tls, postgresql and spire.
	AfterConnect func(ctx context.Context, conn net.Conn) error
	// VerifyCertificate replaces the verification of the server's chain, leaf
	// first. The facts of the check say Verified only when it accepted, and
	// Verifier "custom". Honoured by tls, postgresql and spire.
	VerifyCertificate func(chain []*x509.Certificate) error
	// AfterHandshake is called when the handshake is done and the chain
	// accepted; an error fails the check. Honoured by tls, postgresql and spire.
	AfterHandshake func(ctx context.Context, state tls.ConnectionState) error
}

type ownHooksKey struct{}

// ownHooks returns the hooks of the check that ctx belongs to.
func ownHooks(ctx context.Context) Hooks {
	if h, _ := ctx.Value(ownHooksKey{}).(*Hooks); h != nil {
		return *h
	}
	return Hooks{}
}

// effectiveHooks returns the hooks of the run, replaced field by field by
// those of the check.
func effectiveHooks(ctx context.Context, c Check) Hooks {
	var h Hooks
	if cfg, ok := ctx.Value(vetKey{}).(*config); ok {
		h = cfg.hooks
	}
	if o := c.Hooks; o != nil {
		if o.AfterConnect != nil {
			h.AfterConnect = o.AfterConnect
		}
		if o.VerifyCertificate != nil {
			h.VerifyCertificate = o.VerifyCertificate
		}
		if o.AfterHandshake != nil {
			h.AfterHandshake = o.AfterHandshake
		}
	}
	return h
}

// callHook runs a hook, turning a panic into an error (T-V4): a hook may run
// where the runner's own guard does not reach, in the dialer.
func callHook(name string, f func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%s hook panic: %v", name, p)
		}
	}()
	return f()
}

// afterConnect runs the AfterConnect hook on a new connection, within the
// attempt's deadline, which it leaves off the connection afterwards.
func afterConnect(ctx context.Context, h Hooks, conn net.Conn) error {
	if h.AfterConnect == nil {
		return nil
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	err := callHook("AfterConnect", func() error { return h.AfterConnect(ctx, conn) })
	_ = conn.SetDeadline(time.Time{})
	return err
}

// hooksHonoured says which hooks each protocol's runner calls.
func hooksHonoured(p Protocol) (connect, tlsHooks bool) {
	switch p {
	case TCP:
		return true, false
	case TLS, PostgreSQL, SPIRE:
		return true, true
	}
	return false, false
}

// validateHooks rejects the hooks a check sets that its protocol ignores.
func (c *Check) validateHooks() []error {
	var errs []error
	if c.SPIFFEID != "" && c.Protocol != SPIRE {
		errs = append(errs, fmt.Errorf("spiffe_id applies to spire checks, not %s", c.Protocol))
	}
	if c.Hooks == nil {
		return errs
	}
	connect, tlsHooks := hooksHonoured(c.Protocol)
	if c.Hooks.AfterConnect != nil && !connect {
		errs = append(errs, fmt.Errorf("hook AfterConnect does not apply to %s checks", c.Protocol))
	}
	if c.Hooks.VerifyCertificate != nil && !tlsHooks {
		errs = append(errs, fmt.Errorf("hook VerifyCertificate does not apply to %s checks", c.Protocol))
	}
	if c.Hooks.AfterHandshake != nil && !tlsHooks {
		errs = append(errs, fmt.Errorf("hook AfterHandshake does not apply to %s checks", c.Protocol))
	}
	return errs
}
