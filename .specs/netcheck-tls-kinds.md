# Check hooks, and the postgresql and spire checks

- **Version:** 0.1 (2026-11-19), approved by the user in plan mode; built 2026-11-19
- **Backlog:** `.specs/netcheck-backlog.md`, item 7
- **Branch:** `feature/tls-kinds`, from `main`
- **Release:** v2.1.0 (additive)

## 1. Why

`ligo` checks the network flows of a binding with netcheck (`pkg/client/flowcheck`). Its lab run showed two real flows that a plain `tls` check reports unreachable although the port is open: PostgreSQL negotiates TLS inside its own protocol (an `SSLRequest`, then the handshake), and ligo's workload API presents a SPIFFE SVID, which has a URI SAN and no DNS name. Both are handshakes worth checking for themselves, so they become checks of netcheck, on a small set of hooks into the check's life that anyone can use.

## 2. Decisions (the user, 2026-11-19)

| Question | Decision |
| :-- | :-- |
| Where | In netcheck, as two new protocols, `postgresql` and `spire`, not in `ligo`. |
| How | Hooks stay in the core; the two checks are thin runners that set them. |
| Names | After the event, `Before<Event>` and `After<Event>`; more can be added later without renaming. |
| Networks | `--allow-net` and `--deny-net` are the default implementation of `BeforeConnect`; the test stays on the resolved address at dial time. Per-check networks in a bundle file: a later item (backlog 8), narrowing only. |
| Socket | The CLI takes a trust bundle file (`--spiffe-bundle`) only. The Workload API socket pulls in gRPC; a program that has it gives the source through `spire.WithSource`. |

## 3. Hooks (`pkg/netcheck/hooks.go`)

```go
type Hooks struct {
	BeforeConnect     func(ctx context.Context, ip netip.Addr) error
	AfterConnect      func(ctx context.Context, conn net.Conn) error
	VerifyCertificate func(chain []*x509.Certificate) error
	AfterHandshake    func(ctx context.Context, state tls.ConnectionState) error
}
```

A nil field is the default behaviour. Hooks are code: they are set by the API (`Check.Hooks`, `netcheck.WithHooks`) and never read from a bundle file (T-4).

| Hook | When | Effect | Honoured by |
| :-- | :-- | :-- | :-- |
| `BeforeConnect` | every resolved address, before the socket connects | an error vetoes the address, wrapping `ErrNetworkNotAllowed` | every protocol, `VetAddr` included |
| `AfterConnect` | the TCP connection is up, nothing sent | an error fails the check; the hook may exchange bytes (STARTTLS) | `tcp`, `tls`, `postgresql`, `spire` |
| `VerifyCertificate` | during the handshake, instead of the default verification | accepts or rejects the chain; `TLSFacts.Verified` is true only when it accepted, `TLSFacts.Verifier` is `custom` | `tls`, `postgresql`, `spire` |
| `AfterHandshake` | the handshake is done and verified | an error fails the check (minimum version, ALPN, pinning) | `tls`, `postgresql`, `spire` |

**Layering.** `WithHooks` sets the hooks of a run; `Check.Hooks` those of one check. For `AfterConnect`, `VerifyCertificate` and `AfterHandshake` the check's field replaces the run's (the more specific wins). `BeforeConnect` is a veto: the networks limits, the run's hook and the check's hook all run, and any of them refuses (a bundle can never widen the operator's `--deny-net`).

**Honoured or refused.** `Validate` rejects a check whose hook its protocol does not honour (`hook AfterConnect does not apply to ssh checks`), so a security hook is never dropped in silence.

`netcheck.TLSHandshake(ctx, p, c)` is the TLS runner for the protocols of sub-packages: it dials with the run's dialer (so the networks limits bind it), runs the hooks, performs the handshake and describes the session.

## 4. The checks

**`postgresql`** (`pkg/netcheck/postgres`, no new dependency). Address `host[:port]`, port 5432 by default. `AfterConnect` sends the 8-byte `SSLRequest` (`00 00 00 08 04 d2 16 2f`) and reads one byte: `S` continues with the TLS handshake and the usual verification (the system roots or `TLSOptions.RootCAs`, the name of the host: PostgreSQL's `sslmode=verify-full`); `N` fails with "the server does not support TLS"; `E` or anything else fails with that cause. No startup packet or credential is ever sent, and the check never falls back to plain text.

**`spire`** (`pkg/netcheck/spire`, on `go-spiffe` without gRPC). Address `host:port`. The server's chain is verified as an X509-SVID: signed by the trust bundle, in date, with exactly one URI SAN that is a SPIFFE ID. `Check.SPIFFEID` (`spiffe_id:`) is the expected ID; absent, any ID the bundle vouches for passes. The trust bundle never comes from a bundle file (T-V1): `spire.Runner(spire.WithBundleFile(path))` reads the PEM of the trust domain's CAs at each attempt (a rotated file is picked up), `spire.WithSource(src)` takes any `x509bundle.Source`. With neither, the check fails with "no trust bundle". The report says which ID was presented.

## 5. Threat model

| ID | Threat | Mitigation |
| :-- | :-- | :-- |
| T-V1 | A bundle weakens the verification | Hooks, trust anchors and verifiers come from the API and the command line only; the one thing a bundle gives, `spiffe_id`, narrows what passes. |
| T-V2 | Downgrade to plain text | `postgresql` fails on `N`, and sends nothing after it. |
| T-V3 | A probe leaks a credential | Neither check sends one: `SSLRequest` comes before authentication, and no startup packet follows. |
| T-V4 | A hook that panics or hangs | The runner's panic guard (T-10) and the attempt's deadline apply to hooks as to runners; `AfterConnect` gets the attempt's context and the connection's deadline is the attempt's. |
| T-V5 | A hook widens the network limits | `BeforeConnect` can only veto; the limits run first and cannot be overridden by a hook. |
| T-V6 | The `go-spiffe` dependency | It is imported only by `pkg/netcheck/spire`; `make vuln` on every change. |

## 6. Tests first

- **NV-1** each hook runs at its moment and its error fails the check; `BeforeConnect` of the run and of the check both run, and the limits cannot be widened.
- **NV-2** `VerifyCertificate`: accepted gives `Verified` true and `Verifier` `custom`, rejected fails; `AfterHandshake` sees the negotiated version.
- **NV-3** a hook set on a protocol that does not honour it, and `spiffe_id` on another protocol, are validation errors.
- **NV-4** `postgresql` against a fake server: `S` then TLS passes, `N`, `E`, a garbage byte, EOF and a bad certificate fail with their reasons; a fuzz target for the answer.
- **NV-5** `spire` against an in-process SPIFFE CA (`spiffetest`): the right ID passes, a wrong ID, a wrong trust domain, an expired SVID and a certificate with no URI SAN fail; a rotated bundle file is read again; no bundle fails clearly.
- **NV-6** the command: `--spiffe-bundle`, the protocols documented in the manual and the man page, the completions list the option.
- **NV-7** real servers: both checks run against the real PostgreSQL and the real SPIFFE-authenticated workload API of `ligo`'s lab, through `flowcheck` (`ligo` `.specs/ligo-network-checks.md`, chunk 3); netcheck's own lab gets no PostgreSQL for it.

## 7. Out of scope

`AfterVerifyCertificate`, `AfterResponse` for http, a host key hook for ssh, `AfterAttempt` for metrics, per-check networks, the Workload API socket in the CLI, and `sslmode` variants (`require`, `verify-ca`) of `postgresql`.
