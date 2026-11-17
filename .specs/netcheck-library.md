# netcheck: the library

| | |
| :-- | :-- |
| Status | Draft 0.2 (2026-11-17): open questions L-1 to L-5 decided. Not approved: no implementation code before approval (AGENTS.md rule 1) |
| Basis | `netcheck-reverse-engineering.md` (what the code does, defects D-n, questions Q-n); `netcheck-restructure.md` (repository layout, build and release tooling) |
| Consumer | `ligo`'s client SDK (`pkg/client`, backlog item 12): turn a binding's network flows into checks and report each as reachable or not |
| Decided | Q-1 one package `pkg/netcheck`; Q-2 heavy dependencies in sub-packages; Q-7 this repository; Q-6 HTTP 2xx/3xx, ICMP at least one reply, real DTLS with `pion/dtls`, non-zero exit on failure |

## 1. Goal

Make the connectivity checks of `netcheck` available as a Go library with no global state, no process exit, no hidden I/O, cancellable through `context.Context`, and with results a program can inspect. The command becomes a thin user of the library. Bundle loading (files, HTTP, Redis, Consul) stays available, but outside the core.

Non-goals: a daemon, a scheduler, metrics export, a new bundle format, checks beyond the eight protocols (tcp, udp, icmp, tls, dtls, ssh, http, https).

## 2. Packages

```
pkg/netcheck/              core: Bundle, Check, Protocol, Options, Result, Run
pkg/netcheck/fetch/        the Source interface, file source, schemes registry, HTTP source (stdlib only)
pkg/netcheck/fetch/redis/  Redis source        (go-redis)
pkg/netcheck/fetch/consul/ Consul KV and Service Registry sources (consul/api)
pkg/netcheck/sso/          SPNEGO transport for HTTP checks and HTTP source (go-spnego)
pkg/netcheck/dtls/         DTLS check (pion/dtls)
pkg/metadata/              build metadata (as in ligo)
cmd/netcheck/              the command
internal/...               command-only code: printers, spinner, templates, logging set-up, version command
```

Rules:

- The core imports the standard library, `golang.org/x/crypto/ssh` and `pro-bing` only. A consumer that runs TCP, UDP, TLS, SSH, HTTP and ICMP checks pulls nothing else.
- Heavy protocols and sources plug in by registration, never by default: the core exposes `RegisterProtocol(p Protocol, r Runner)` and `fetch.Register(scheme, Source)`. Importing a sub-package for its side effect is not used; the caller passes the registration explicitly (`netcheck.WithRunner(dtls.Runner())`, `fetch.WithSource("redis", redis.Source())`), so what a binary links is visible in its code.
- No package-level mutable variable, no `init()` that reads the disk, no use of `http.DefaultClient`, `http.DefaultTransport` mutation or the global `slog` default.
- Command-only code (colours, templates, spinner, mock data) is `internal/` and is not part of the library.

## 3. API

Names are proposals; the shape is what is being approved.

```go
package netcheck

// Protocol is one of TCP, UDP, ICMP, TLS, DTLS, SSH, HTTP, HTTPS.
// The zero value is invalid: a Check without a protocol is rejected by Validate.
type Protocol string

type Check struct {
    Name     string
    Address  string   // host:port, a bare host for icmp, host[:port][/path] for http(s)
    Protocol Protocol
    Timeout  time.Duration // 0 = bundle/option default
    Retries  int           // attempts in total; 0 = default
    Wait     time.Duration // between attempts; 0 = default
    SSO      bool
    TLS      *TLSOptions   // optional: ServerName, RootCAs, InsecureSkipVerify
}

type Bundle struct {
    ID, Description string
    Timeout, Wait   time.Duration
    Retries, Concurrency int
    Checks []Check
}

type Defaults struct { Timeout, Wait time.Duration; Retries, Concurrency int; Ping PingOptions }

func DefaultDefaults() Defaults            // the compiled-in values
func (b *Bundle) Validate() error          // every defect found, as errors.Join

type Option func(*runner)
func WithDefaults(Defaults) Option
func WithLogger(*slog.Logger) Option       // default: discard
func WithRunner(p Protocol, r Runner) Option
func WithHTTPClient(*http.Client) Option   // never mutated by the library

// Run executes every check of the bundle and returns one Report. It returns early
// only when ctx is cancelled: the checks not run yet are reported as Skipped.
func Run(ctx context.Context, b *Bundle, opts ...Option) (*Report, error)
func RunCheck(ctx context.Context, c Check, opts ...Option) Result

type Runner interface { Run(ctx context.Context, c Check, d Defaults) (Detail, error) }
```

`Report` and `Result`:

```go
type Report struct {
    BundleID string
    Started  time.Time
    Duration time.Duration
    Results  []Result            // same order as Bundle.Checks
}
func (r *Report) Failed() int
func (r *Report) OK() bool

type Result struct {
    Check    Check               // the check as run, defaults applied
    Status   Status              // Passed, Failed, Skipped
    Attempts int
    Latency  time.Duration       // of the last attempt
    Err      error               // last error; nil when Passed; errors.Is/As work
    Detail   Detail              // per protocol facts, see below
}

type Detail struct {
    Remote      string           // resolved address actually dialled
    HTTPStatus  int              // http, https
    PacketsSent, PacketsRecv int // icmp
    TLS         *TLSFacts        // tls, dtls, https: version, cipher, issuer, subject, NotAfter, SANs
    SSHBanner, SSHHostKey string // ssh: server version string and the key's SHA-256 fingerprint
}
```

JSON and YAML marshalling of `Bundle` and `Check` keeps today's field names and the duration-string form (`"2s"`), so existing bundle files load unchanged. `Result` marshals with `status`, `attempts`, `latency`, `error` (string), and `detail`; it unmarshals too.

### 3.1 Behaviour, protocol by protocol

| Protocol | Success | Notes |
| :-- | :-- | :-- |
| tcp | Connection established | Dial through `net.Dialer` with the attempt's deadline |
| udp | Connection "established" **and** no ICMP error back within a short grace window (`Defaults.UDPGrace`, default 200 ms) | A dial alone proves nothing (reverse-engineering §6). The check is still best-effort and says so in `Detail` and the guide |
| tls | Handshake succeeds, chain verified against `TLSOptions.RootCAs` or the system roots, name verified against `ServerName` (default: the host of the address, IPv6 handled with `net.SplitHostPort`), certificate not expired | Certificate facts are returned whether or not the check passed, when a handshake got far enough |
| dtls | As tls, over UDP, with `pion/dtls` | Sub-package `dtls` |
| icmp | At least one echo reply received | `PacketsSent` and `PacketsRecv` reported; privileged or unprivileged mode chosen by the caller through `PingOptions.Privileged` (no OS-based guess); a bad address returns an error, never panics |
| ssh | Transport handshake completes (a refused authentication is a pass) | The host key is **not** verified; its fingerprint is returned in `Detail`. A pinned key (`TLSOptions`-like `SSHOptions.HostKey`) is added later, not in this release |
| http, https | Response with status 200 to 399 | Redirects followed up to 10; `Detail.HTTPStatus` set; the body is discarded after at most 64 KiB; `sso` uses `sso.Transport()` and needs it registered, or the check fails with a clear error |

Default ports (when the address has none): ssh 22, http 80, https 443. IPv6 literals use brackets (`[2001:db8::1]:443`).

### 3.2 Execution

- A bounded pool of `Concurrency` workers, at least 1; a value below 1 means the default (fixes D-8).
- Each attempt runs under `context.WithTimeout(ctx, Timeout)`; the wait between attempts honours `ctx` and is **not** taken after the last attempt (fixes D-10).
- A panic in a runner is recovered, becomes a `Failed` result with an error, and does not kill other checks.
- Results are written by index into a pre-sized slice: no result channel to block on.
- `Run` does not log at info level by default; with `WithLogger` it logs through the given logger only.

### 3.3 Bundle sources (`pkg/netcheck/fetch`)

```go
type Source interface { Fetch(ctx context.Context, location *url.URL) (data []byte, f Format, err error) }
func Load(ctx context.Context, location string, opts ...Option) (*netcheck.Bundle, error)
```

- `Load` picks the source by scheme from the registry; the file source and the HTTP(S) source are built in (stdlib only); Redis, Consul KV and Consul SR are registered by the caller. An unregistered scheme is an error naming the scheme, never a silent fallback to the file system (fixes D-1).
- The scheme grammar stays as today for the command: a trailing `-` skips TLS verification, `+sso` selects SPNEGO. In the library the same is also available as explicit options on the `Source`, and the `-` forms are parsed by the command's scheme layer only (§6), so the library API does not make insecure verification a string trick.
- Every fetch has a deadline from `ctx`, a body limit (`WithMaxSize`, default 1 MiB), a status check (2xx), and never logs credentials or the body.
- Format detection: the Content-Type or extension hint first, then sniffing; a YAML document without `---` is accepted when a hint says YAML; JSON and YAML are both tried in the ambiguous case.

## 4. The command

`cmd/netcheck` keeps its flags and its three output formats and its templates, now through `internal/`. Changes:

- **Exit status:** 0 all checks passed; 1 usage, load or template error; 2 at least one check failed (D-9). Documented in the guide.
- Schemes with `-` and `+sso` stay in the command, mapped to library options and registrations. The Redis, Consul and SSO registrations are made in `cmd/netcheck`, so the shipped binary supports every source it does today.
- Defaults file (`netcheck.conf`) is read by the command, and passed to the library with `WithDefaults`; it is not read by the library.
- Mock bundles and `--print-diagnostics` stay as command code, over a `internal/mock` copy of the result shape; they follow `netcheck.Result` and need a test that fails when a field is added to `Result` without being added to the mock (D-15).

## 5. Use by ligo (informative)

A flow `{Protocol: "TCP", Destination: "db.lab", Ports: 5432, 6000-6010, TLS: true, ServerName: "db.lab"}` becomes one `tls` check per port (or one `tcp` check when `TLS` is false) with `TLSOptions{ServerName: ...}` and the workload's trust bundle as `RootCAs`. A UDP flow becomes a `udp` check. A range is expanded by ligo's SDK, not by this library; the library accepts `[]Check` and returns a `Report`. ICMP and SSH checks are not used by ligo.

## 6. Threat model

Assets: the host the checks run from (a workload or an operator machine); credentials in source URLs; the trustworthiness of a check's outcome.

| ID | Threat | Mitigation |
| :-- | :-- | :-- |
| T-1 | A hostile or compromised bundle source feeds checks that turn the tool into a scanner or an SSRF probe against internal addresses | The library only runs checks it is handed. A caller that loads bundles from untrusted places can set `WithAllowedNetworks`/`WithDeniedNetworks` (CIDR lists) and `WithMaxChecks` (default 1,000) to bound what a bundle can reach and how big it is. The command documents the risk. |
| T-2 | A bundle with huge `retries`, `timeout` or `concurrency` exhausts the host | Upper bounds in `Validate` (`MaxRetries` 10, `MaxTimeout` 5 min, `MaxConcurrency` 256), each overridable by an explicit option |
| T-3 | Credentials leak through logs or errors (URLs with `user:pass@`, tokens, the fetched body) | URLs are logged without user info; bodies never; errors from sources are wrapped with the redacted URL |
| T-4 | TLS verification disabled by accident | No `InsecureSkipVerify` unless set explicitly in `TLSOptions` or the source option; the result's `Detail.TLS` records `Verified: false` so a report shows it |
| T-5 | A check reports success when nothing was proven (UDP dial, ICMP with loss, HTTP error page) | Behaviour in §3.1; `Detail` always carries the evidence; the guide states each check's guarantee |
| T-6 | SSH check mistaken for host authentication | Documented: it proves reachability and a banner only; fingerprint returned for the caller to pin |
| T-7 | A remote HTTP source returns a gigantic or never-ending body | Body limit and context deadline on every fetch |
| T-8 | Redirects from an HTTP check lead to another host, or from https to http | Redirect limit 10; a downgrade from https to http fails the check unless `AllowDowngrade` is set |
| T-9 | Data race or shared state between concurrent checks and the host application | No globals; no mutation of `http.DefaultClient`; `-race` in CI; `WithHTTPClient` is cloned per check where a transport is changed |
| T-10 | A runner panics or hangs and stalls the whole bundle | Recover per check; every runner receives a context with a deadline |
| T-11 | Supply chain: the library drags large dependency trees into every consumer | Sub-packages (§2); `govulncheck` and a dependency review in CI; a test that fails if the core imports anything outside the allowed list |
| T-12 | ICMP raw sockets need privileges (`cap_net_raw`) | Privilege is the caller's choice (`PingOptions.Privileged`), never guessed from the OS; a missing privilege is an error that says so; the packaging script keeps `setcap` for the command only |

## 7. Acceptance criteria

| ID | Criterion |
| :-- | :-- |
| NC-01 | `go list -deps ./pkg/netcheck` contains only the standard library, `x/crypto`, `pro-bing` and their transitive dependencies; a test enforces an allow-list |
| NC-02 | No `init()` with side effects, no exported mutable variable, no `os.Exit`, `log.Fatal` or `panic` for input errors in `pkg/` (checked by a lint rule or test) |
| NC-03 | `Run` returns promptly when `ctx` is cancelled; unfinished checks are `Skipped`; no goroutine leaks (`goleak`) |
| NC-04 | `concurrency` 0 or negative uses the default; no deadlock |
| NC-05 | No wait after the last failed attempt; total time for a failing check with retries R, wait W, timeout T is at most `R*T + (R-1)*W` plus a small margin |
| NC-06 | TCP: pass against a listener, fail against a closed port, fail on timeout against a black-holed address |
| NC-07 | TLS: pass with a trusted certificate; fail for an expired, wrong-name, self-signed (unless `RootCAs` contain it) certificate; IPv6 host names handled; facts returned |
| NC-08 | DTLS: pass against a `pion/dtls` server; fail against a plain UDP port |
| NC-09 | ICMP: fails with 100% loss; passes with one reply; invalid address returns an error, no panic |
| NC-10 | HTTP: 200 and 3xx pass; 404 and 500 fail with `HTTPStatus` set; redirect loop fails; https to http downgrade fails; response body capped |
| NC-11 | SSH: pass against an `x/crypto/ssh` test server that refuses authentication; fingerprint returned; fail against a non-SSH port |
| NC-12 | UDP: fail when the peer answers with ICMP port unreachable; pass against a listening socket |
| NC-13 | Default ports applied for ssh, http, https; IPv6 literals work for tcp, tls, http |
| NC-14 | `Bundle` JSON and YAML round-trip; the sample bundles of `_test/` load unchanged; unknown protocol is a validation error |
| NC-15 | `Validate` reports every problem at once and enforces the bounds of T-2 |
| NC-16 | `Load`: file, HTTP and HTTPS (ephemeral certificate), Redis and Consul (testcontainers, build-tagged) each return the bundle; an unregistered scheme is an error; a 404, an oversize body and a hanging server fail within the deadline |
| NC-17 | Credentials never appear in logs or errors (test with a capturing handler) |
| NC-18 | Command: exit 0, 1, 2 as in §4; output formats unchanged for the same input; template and `--print-diagnostics` still work; the mock-shape test passes |
| NC-19 | `go test -race ./...` passes on a clean checkout with no external network and no files outside the repository (the Redis and Consul tests are build-tagged and run in CI with containers) |
| NC-20 | The guide documents every check's guarantee, every scheme, the exit codes, and the library API with a runnable example (`Example` tests) |

## 8. Test plan

Tests are written before the code they cover (AGENTS.md rule 4).

1. **Unit:** `Timeout` and `Protocol` marshalling, `Validate`, defaults merge, address parsing (IPv6, default ports), format detection, redaction.
2. **Per-protocol, hermetic:** local listeners (`net.Listen`), `httptest` servers (HTTP, HTTPS with an ephemeral CA), an in-process `x/crypto/ssh` server, a `pion/dtls` server, ICMP against `127.0.0.1` where raw sockets are allowed (skipped otherwise, with the reason printed).
3. **Concurrency:** `-race`, `goleak`, cancellation mid-run, a runner that panics, a runner that never returns until `ctx` ends.
4. **Sources:** file and HTTP hermetic; Redis and Consul behind a build tag with testcontainers.
5. **Command:** table tests over the exit codes and output formats using a built binary and fixtures in `test/fixtures/`.
6. **Fuzz:** `FuzzParseAddress`, `FuzzDetectFormat`, `FuzzBundleUnmarshal` (no panics; round-trip where parsing succeeds).
7. **Benchmark:** `Run` over 1,000 TCP checks against a local listener at concurrency 1, 10, 100; the number goes in the guide as a reference, not a guarantee.
8. **Lab (proposed):** one run from `ligo`'s lab: the SDK's flow check against the real flows of a lab binding, with a firewall rule blocking one port, to see the report name the blocked flow. Planned with ligo's item 12, not part of this repository's gate.

## 9. Delivery

1. **Phase 0, the restructure** (`netcheck-restructure.md`, revised for this layout): repository layout, tooling, CI, docs pipeline; the code moves as it is; D-1, D-16 and the build go green. One PR.
2. **Phase 1, the core:** `pkg/netcheck` with TCP, UDP, TLS, SSH, HTTP, ICMP and the pool; tests first. One PR.
3. **Phase 2, sources and plug-ins:** `fetch`, `fetch/redis`, `fetch/consul`, `sso`, `dtls`; the command rewired; exit codes. One PR.
4. **Phase 3, guide and release:** the guide (PDF), `Example` tests, README, `v0.x` tag, so ligo can depend on it. Spec acceptance audit (rule 5).

Each phase: `go test -race ./...`, lint, `govulncheck`, secret scan before commit.

## 10. Open questions

- **L-1. Decided 2026-11-17:** API only; the bundle file format gains no TLS fields. Asked as: per-check TLS options in the bundle file (`serverName`, `caFile`), or only through the API? The file format would grow; ligo does not need it.
- **L-2. Decided 2026-11-17:** both `Run` and `RunCheck`. Asked as: does the library expose `RunCheck` for a single check as public API (ligo's SDK probably wants it), or only `Run`? Proposed: both, as above.
- **L-3. Decided 2026-11-17:** `Defaults.UDPGrace`, 200 ms, no per-check field. Asked as: UDP grace window: a fixed default, or a per-check field? Proposed: default in `Defaults`, no per-check field for now.
- **L-4. Decided 2026-11-17:** the SSH check returns the host-key fingerprint and does not verify it (pinning later); IPv6 literals in brackets and the default ports 22, 80 and 443 as in §3.1.
- **L-5. Decided 2026-11-17:** the release is `v2.0.0` and the module path becomes `github.com/dihedron/netcheck/v2` (Phase 0 changes `go.mod` and every import). The import paths in this spec are shown without `/v2` for readability.

## 12. Phase 2a as built: sources and network limits

`pkg/netcheck/fetch`, `fetch/redis`, `fetch/consul` and the T-1 options; the command and `internal/` are untouched until Phase 2b.

- **`Source` takes the size limit:** `Fetch(ctx, *url.URL, maxSize int64) ([]byte, Format, error)`, so a source can stop reading early; `Load` rejects anything larger. `Format` is a string type (`Unknown`, `YAML`, `JSON`).
- **Registration:** `fetch.WithSource(src, schemes...)` registers one source for several schemes, instead of the single-scheme sketch of §3.3. The built-in schemes are `file` (also a bare path), `http` and `https`.
- **Parsing:** `fetch.Parse(data, hint)`; JSON and YAML decode leniently (unknown fields are ignored, as the old code did). Without a hint the data is sniffed (`---`, `{`), then tried as JSON, then YAML.
- **Errors and logs:** `Load` is the only place that logs, and it redacts the location (`url.Redacted`) and scrubs the password from a source's error text. A location that does not parse is reported without quoting it.
- **TLS and trust are options of the source**, not scheme tricks: `redis.WithTLSConfig`, `consul.WithHTTPClient`, `fetch.WithHTTPClient`. The `-` (skip verification) and `+sso` forms are parsed by the command in 2b and mapped to these options.
- **Redis:** one attempt (`MaxRetries: -1`), `ContextTimeoutEnabled`, so the context bounds every read. The container test is behind `-tags integration`.
- **Consul:** the API client is built from the location only; nothing is read from the `CONSUL_*` environment variables (which `DefaultConfig` would). The password is no longer logged, not even its first and last characters.
- **T-1:** `WithAllowedNetworks` / `WithDeniedNetworks` (denied wins) vet the resolved address in the dialer's `Control` hook for tcp, udp, tls, ssh and http (redirects included); icmp pings the vetted literal address. With any limit set, http checks use no proxy, since a proxy would hide the real destination. `ErrNetworkNotAllowed` is the error to match; the guard test (NC-02) allows exported `Err…` sentinels.
- **Guards:** the core must not import its sub-packages (NC-01 tightened); `fetch` must not import go-redis, consul, pion or go-spnego (T-11).
