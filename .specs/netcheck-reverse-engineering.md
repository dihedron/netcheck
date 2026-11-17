# netcheck: reverse-engineered specification of the current code

| | |
| :-- | :-- |
| Status | Draft 0.1, descriptive only: it records what the code does on `main` at `86ad300` (2026-11-02), not what it should do |
| Written | 2026-11-17, on `feature/library-restructure` |
| Purpose | Basis for turning `netcheck` into a library that `ligo`'s client SDK can use (ligo backlog item 12), and for the restructure in `netcheck-restructure.md` |
| Rule | Nothing here is a requirement. Each defect (D-n) and open question (Q-n) is settled in the library specification, which comes next |

## 1. What netcheck is

A command-line tool that checks connectivity. The user writes one or more **bundles**, each a list of **checks**; `netcheck` fetches each bundle, runs its checks concurrently, and prints the outcome as text, JSON, YAML or through a Go template.

Size on `main`: about 1,500 lines of Go in the root package and `checks/`, about 600 in `fetch/`, plus small helpers. Tests exist only in `fetch/`. `go vet ./...` is clean; `go test ./fetch` fails on a clean checkout: `TestFromFile` and `TestFromHTTP` pass, then `TestFromHTTPs` aborts the whole test process because `server.crt` and `server.key` are missing.

## 2. Layout

| Path | Role |
| :-- | :-- |
| `main.go`, `init.go`, `print.go` | The command (package `main`): flags, orchestration, logging set-up, printers |
| `checks/` | The domain: `Bundle`, `Check`, `Protocol`, `Timeout`, `Result`, defaults, the worker pool, the mock/tracked types for template development |
| `fetch/` | Bundle sources: file, HTTP(S), Redis, Consul KV, Consul Service Registry |
| `format/` | `Format` (YAML, JSON) and content sniffing |
| `extensions/` | Colour functions added to templates |
| `spinner/`, `tracked/`, `pointer/`, `logging/`, `metadata/` | Terminal spinner, access-tracking value wrapper, generic pointer helper, JSON/YAML dump helpers, build metadata |
| `_test/` | Sample bundles and a sample template; the `fetch` tests read them |
| `netcheck.conf`, `postinstall.sh`, `Dockerfile` | Defaults file shipped under `/etc`, `setcap` post-install script, an old Dockerfile |

The library-worthy part is `checks/` plus `fetch/` and `format/`. Everything in the root package is command code, and part of it (`os.Exit`, printing) leaks into the library today (see §10).

## 3. Data model

### 3.1 Bundle (`checks.Bundle`)

`id`, `description`, `timeout`, `retries`, `wait`, `concurrency`, `checks[]`. JSON and YAML tags are identical. `timeout` and `wait` are `Timeout` values: a Go duration string (`"2s"`, `"100ms"`) in JSON, YAML and text.

### 3.2 Check (`checks.Check`)

`name`, `timeout`, `retries`, `wait`, `address`, `protocol`, `sso`, `result`, plus an unexported `id` (index in the bundle). A zero `timeout`, `retries` or `wait` means "inherit the bundle's".

### 3.3 Protocol

Eight values, serialised as lower-case strings, in this order: `tcp` (the zero value, hence the default when `protocol` is omitted), `udp`, `icmp`, `tls`, `dtls`, `ssh`, `http`, `https`. An unknown string is an error at parse time. `Protocol.String()` indexes a fixed slice and would panic on a value outside 0 to 7.

### 3.4 Result

Wraps an `error`; unexported field, so callers outside the package can read it only through `IsError()` and `String()` (`"success"` or the error text). It marshals to JSON and YAML as that string. It cannot be unmarshalled, and the error value (for `errors.Is`/`As`) is not reachable.

### 3.5 Defaults (`checks.Defaults`, global `checks.Default`)

`timeout` 20 s, `retries` 3, `wait` 1 s, `concurrency` 10, `ping.count` 10, `ping.interval` 100 ms, `ping.size` 64. Overridable by a YAML file, first match wins and the file is not merged with the next:

- Unix: `./netcheck.conf`, `~/netcheck.conf`, `/etc/netcheck.conf`;
- Windows: `./netcheck.conf`, `~/netcheck.conf`.

Loading happens in the package `init()`, so merely importing `checks` reads the disk. The shipped `netcheck.conf` sets `timeout: 2s` and `wait: 100ms`, so an installed package does not use the compiled defaults.

## 4. Bundle retrieval (`checks.New(path)`)

Dispatch by prefix of `path`:

| Scheme | Source | Format decided by |
| :-- | :-- | :-- |
| `http://`, `https://`, `https-://`, `http+sso://`, `https+sso://`, `https+sso-://` | `fetch.FromHTTP` | response `Content-Type`: `application/json` is JSON; `application/x-yaml` or `text/yaml` is YAML; anything else leaves the zero value, YAML |
| `redis://`, `rediss://`, `rediss-://` | `fetch.FromRedis` | content sniffing |
| `consulkv://`, `consulkvs://`, `consulkvs-://` | `fetch.FromConsulKV` | content sniffing |
| `consulsr://`, `consulsrs://`, `consulsrs-://` | `fetch.FromConsulSR` | content sniffing |
| anything else | `fetch.FromFile` | extension: `.yaml`, `.yml`, `.json`; otherwise the zero value, YAML |

Conventions in the schemes: a trailing `-` before `://` skips TLS verification; `+sso` selects SPNEGO (Kerberos/NTLM) authentication on HTTP.

Content sniffing (`format.Detect`): after trimming leading newline, carriage return and tab, text starting with `---` is YAML, text starting with `{` or `[` is JSON, anything else is an error. A YAML document without `---` is therefore refused.

After parsing, a bundle starts from the defaults and the parsed fields overwrite them. Then: a negative `concurrency` becomes the default (zero is kept and means **no workers**, see D-8); `timeout` or `wait` of zero or less becomes the default; `retries` below 1 becomes the default.

### 4.1 Source parameters

- **HTTP:** `GET`, no status-code check, body read in full (no size limit). `https-` and `https+sso-` set `InsecureSkipVerify`. Basic credentials in the URL are handled by Go's HTTP client.
- **Redis:** query `key` (required) and `db` (default 0); user and password from the URL. `GET key`.
- **Consul KV:** query `key` (required), `dc`; basic authentication if user and password are both present, token if only a password is present (`consulkv://:token@host`).
- **Consul Service Registry:** query `service` (required), `tag` (required in the code, although the README calls it optional), `meta` (required, the metadata key), `dc`. The service and tag must match exactly one catalog entry; the bundle is the value of that entry's metadata key.
- **File:** `os.ReadFile(filepath.Clean(path))`.

## 5. Execution (`Bundle.Check`)

1. A pool of `Concurrency` goroutines reads checks from a buffered channel.
2. Each check is copied, its index recorded, and zero `timeout`, `retries`, `wait` filled from the bundle.
3. A worker runs `check.Do()` up to `retries` times; after each failure it sleeps `wait`, including after the last one. The first success stops the loop.
4. The result (`nil` or the last error) goes back on an output channel; the caller writes it into `Checks[i].Result` by index.

There is no context, no cancellation and no overall deadline: a bundle takes at most `ceil(checks / concurrency) x retries x (timeout + wait)` in the worst case, and nothing can stop it. `Check()` has no return value; results are read from the bundle afterwards.

## 6. The checks (`Check.Do`)

| Protocol | What it does | What counts as success | Notes |
| :-- | :-- | :-- | :-- |
| `tcp`, `udp` | `net.Dialer.DialContext` with the timeout | The dial returns | For UDP a dial almost always succeeds: nothing is sent, so it proves only that a route exists |
| `tls` | `tls.DialWithDialer("tcp", addr)`, `VerifyHostname(host)`, expiry of the first peer certificate | Handshake succeeds, hostname matches, certificate not expired | Host taken as `strings.Split(addr, ":")[0]`: wrong for IPv6 literals. Verification uses the system roots, no way to pass others |
| `dtls` | The same call with `"udp"` | See D-5 | |
| `icmp` | `pro-bing` pinger, privileged on Linux and Windows, count, interval and size from the defaults | `Run()` returns no error | Success does not require any reply received (D-6). Needs `cap_net_raw` (`postinstall.sh`) |
| `ssh` | `ssh.Dial` with no credentials and an insecure host-key callback | Dial succeeds, or fails only with "unable to authenticate" | Proves a working SSH banner and key exchange; the host key is neither checked nor reported |
| `http`, `https` | `GET <protocol>://<address>` with a cloned default transport (proxy variables honoured); with `sso` a SPNEGO transport | The request returns a response | Status code ignored: a 500 is a success (D-4). README says a 200 is needed |

`address` is `host:port` for tcp, udp, tls, dtls, ssh; a bare host for icmp; `host[:port][/path]` for http and https. SSH with no port is not defaulted by the code, although the README says port 22 is used.

## 7. Command line

```
netcheck [-f json|yaml|text|template] [-t FILE] [--print-diagnostics] [BUNDLE...]
netcheck version | --version
netcheck version -v | --verbose
```

- `go-flags` parsing; positional arguments are bundle locations.
- `--template` forces `template` format. `template` without a readable file is an error (exit 1).
- No bundle given: the command runs on **mock data** (`MockBundles`), which exists to develop templates; with `--print-diagnostics` it prints, on stderr, every field of the mock, highlighting those the template read (`tracked.Value`).
- Per bundle, in `text` mode, a spinner (on a terminal) or a `►` line (not a terminal) is shown, the bundle runs, and results print at once. In the other formats results are collected and printed together at the end.
- Text output: one line per check, `▲` success or `▼` failure, port, protocol, host, name (cut at 32 characters), and the error in blue. Colour only on a terminal. Port defaults shown only on a terminal.
- **Exit status:** 0 in every case where the program ran to the end, **including when checks failed**. Only usage, load, template and marshalling errors exit 1 (D-9).
- Template functions: the Sprig set plus `blue`, `cyan`, `green`, `magenta` (also `purple`), `red`, `yellow`, `white` and their `hi` variants.
- `init()` configures logging from `<BINARY>_LOG_LEVEL` and `<BINARY>_LOG_STREAM`, and profiling from `<BINARY>_CPU_PROFILE` and `<BINARY>_MEM_PROFILE`, with `<BINARY>` the upper-cased binary name. Logging is off unless the level is set. `metadata.DotEnvVarName` can name a `.env` file.

## 8. Dependencies

`go-spnego` and `x/crypto/ssh`, `pro-bing`, `consul/api`, `go-redis`, `go-pretty` (colours), `sprig`, `go-flags`, `yaml.v3`, `go-homedir`, `godotenv`, `fatih/color`, `go-isatty`. `testcontainers-go` is in `go.mod` for tests of Redis and Consul. A library built on this code would pull the whole set into every consumer: the split between a lean core and optional fetchers is a design question (Q-2).

## 9. Tests

- `fetch/file_test.go`: reads `../_test/*`.
- `fetch/http_test.go`: starts a local server and fetches over HTTP and HTTPS; the HTTPS part expects key and certificate files that are not in the repository.
- `fetch/redis_test.go`: Redis through testcontainers.
- Nothing tests `checks/` (no test for protocols, defaults, the pool, `Timeout`, `Protocol` marshalling), `format/`, `tracked/`, or the printers.

## 10. Defects and risks found

| ID | Where | Observation | Severity for a library |
| :-- | :-- | :-- | :-- |
| D-1 | `checks/bundle.go` `New` | `strings.HasPrefix("http://", path)` has its arguments reversed, for all HTTP and the Redis schemes: those paths fall through to `FromFile`. Only Consul schemes dispatch correctly | Blocker |
| D-2 | `checks/bundle.go` `New` | A parse error calls `os.Exit(1)` inside the library | Blocker |
| D-3 | `checks/check.go`, `fetch/http.go` | `http.DefaultClient` is modified (its `Transport` is replaced) on every HTTP check and fetch: a data race with the worker pool, and a side effect on the host application | Blocker |
| D-4 | `checks/check.go` | An HTTP check ignores the status code; the README says a 200 is required | Major, behaviour to decide |
| D-5 | `checks/check.go` | `dtls` calls `tls.DialWithDialer` on `"udp"`; Go's `crypto/tls` does not speak DTLS, so the check cannot work as named | Major |
| D-6 | `checks/check.go` | An ICMP check succeeds even if every packet was lost; the code does not look at `Statistics.PacketsRecv`. `SetPrivileged` is called before the error from `NewPinger` is checked (nil dereference on a bad address) | Major |
| D-7 | `checks/check.go` | TLS host extraction by `Split(":")` breaks on IPv6; `ssh` and `http` default ports are not applied; `ssh` accepts any host key | Major |
| D-8 | `checks/bundle.go` | `concurrency: 0` in a bundle starts no worker and `Check()` then blocks forever on the output channel | Major |
| D-9 | `main.go` | Exit status 0 when checks fail: not usable in scripts and pipelines | Major for the command |
| D-10 | `checks/bundle.go` | The worker sleeps `wait` after the last failed attempt, delaying the result | Minor |
| D-11 | `checks/defaults*.go` | Global `Default`, loaded in `init()`; the `Defaults` struct is repeated anonymously three times; Unix and Windows files differ only in a path and a log line | Design |
| D-12 | `fetch/*.go` | Secrets reach the log: Redis and Consul URLs at debug level (the Redis address includes the password), the fetched bundle body, and a masked token that still reveals first and last characters | Security |
| D-13 | `fetch/*.go` | No size limit on the fetched body; no timeout on HTTP, Redis and Consul fetches (the default client has none); `InsecureSkipVerify` is selectable per URL only, which is intended, but the library API must not make it the easy default | Security, robustness |
| D-14 | `checks/types.go` | `Result` is not unmarshalable and hides its error; `Timeout`'s doc comments mention TOML, which is not implemented | Minor |
| D-15 | `print.go` | Three copies of the field-print logic; a duplicated `Wait` block in the diagnostics; `printAsText` logic repeated for the terminal and non-terminal cases | Minor, command only |
| D-16 | `fetch` tests | Not self-contained: they need files and a network the repository does not provide | Blocker for CI |
| D-18 | `checks/check.go` | HTTP and HTTPS checks ignore `timeout`: the shared `http.DefaultClient` has none, so a slow server holds a worker until the retries are spent or the connection fails (found by a test that set a 1 ns timeout and still got a success). The library applies the attempt's context to every request | Major |
| D-17 | `checks/check.go` | Success logging and error logging at `slog` on the global logger; library code writes through a global handler | Design |

## 11. Open questions for the library design

- **Q-1. Decided 2026-11-17:** one package, `pkg/netcheck`. Asked as: public surface: one package (`pkg/netcheck`) with `Bundle`, `Check`, `Result`, `Run(ctx, ...)`, or the split `pkg/checks` and `pkg/fetch`? What must a client SDK see: only "run these checks and give me the results", or also the bundle format?
- **Q-2. Decided 2026-11-17:** separate fetch sub-packages; the core imports only the standard library, `x/crypto/ssh` and `pro-bing`; Redis, Consul and SPNEGO are pulled in only by importing their package. Asked as: dependency weight: should Redis, Consul and SPNEGO be optional (separate modules or build tags), so that an SDK needing only TCP and TLS stays small?
- **Q-3.** Context and cancellation: `Run(ctx)` with the timeout and retry policy per check; cancellation of the pool; no global state.
- **Q-4.** Result type: a structured result (status, latency, attempts, error, certificate facts) instead of an opaque error string; stable JSON.
- **Q-5.** What ligo needs from it: a ligo `Flow` has destination, ports, protocol and `tls`/`serverName` (`GET /v1/flows`, AC-73). Which checks map to them (TCP, UDP, TLS), and whether an ICMP check and the text of the check need to exist in the SDK.
- **Q-6. Decided 2026-11-17 (part):** HTTP success is a 2xx or 3xx status (a 4xx or 5xx fails); ICMP success is at least one reply received; DTLS is implemented for real with `pion/dtls`, in its own sub-package like the other heavy dependencies; the command exits non-zero when any check failed, with a code distinct from usage errors. IPv6, default ports and the SSH host key: decided in `netcheck-library.md` (L-4). Asked as: behaviour changes (not to slip in during the move): HTTP status handling, ICMP loss, exit status, IPv6, DTLS (drop, or implement with a DTLS library), SSH host key.
- **Q-7. Decided 2026-11-17:** this repository, command under `cmd/`; ligo pins a tag. Asked as: where the library lives: this repository (`github.com/dihedron/netcheck`, with the command under `cmd/`) or a new module; the version tag the SDK pins.
- **Q-8.** Compatibility: AGENTS.md rule 8 says none is needed before 1.0.0; the bundle file format and the command line are the only user-visible surfaces, and the question is which of them to keep.

## 14. Status of the defects (2026-11-18, end of Phase 2)

| ID | Status | Where |
| :-- | :-- | :-- |
| D-1 | Fixed (Phase 0); the dispatch no longer exists: an unregistered scheme is an error | `fetch.Load` |
| D-2 | Fixed: `Parse` and `Load` return errors; nothing in `pkg/` exits (guard test) | `pkg/netcheck/fetch` |
| D-3 | Fixed: own client per check, never `http.DefaultClient`, transports cloned | `pkg/netcheck/http.go` |
| D-4 | Fixed: 200 to 399 passes, the status is reported | `pkg/netcheck/http.go` |
| D-5 | Fixed: a real DTLS handshake with pion | `pkg/netcheck/dtls` |
| D-6 | Fixed: at least one reply is needed, the address error is checked first | `pkg/netcheck/icmp.go` |
| D-7 | Fixed: IPv6 and default ports. The SSH host key is still not verified, by decision (L-4a): its fingerprint is reported | `pkg/netcheck/address.go`, `ssh.go` |
| D-8 | Fixed: a concurrency below 1 means the default | `pkg/netcheck/run.go` |
| D-9 | Fixed: exit codes 0, 1, 2 | `cmd/netcheck/main.go` |
| D-10 | Fixed: no wait after the last attempt | `pkg/netcheck/run.go` |
| D-11 | Fixed: no global; the command reads the defaults file, outside `init()` | `cmd/netcheck/defaults.go` |
| D-12 | Fixed: credentials are redacted from logs and errors; bodies and the password's first and last characters are no longer logged | `fetch`, `run.go` |
| D-13 | Fixed: size limit and context deadline on every fetch; verification is skipped only through explicit options | `pkg/netcheck/fetch` |
| D-14 | Fixed: `Result` and `Report` marshal and unmarshal | `pkg/netcheck/result.go` |
| D-15 | Fixed: one printer per format; the mock follows the library (reflection test) | `cmd/netcheck`, `internal/mock` |
| D-16 | Fixed (Phase 0): hermetic tests | all |
| D-17 | Fixed: logging goes through the logger given with `WithLogger`; none by default | `pkg/netcheck` |
| D-18 | Fixed: every attempt runs under the context's deadline | `pkg/netcheck/run.go` |
