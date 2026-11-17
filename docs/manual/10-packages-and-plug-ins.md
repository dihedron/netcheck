# Packages and plug-ins

![The packages and their dependencies](packages.pdf){width=100%}

## The packages

| Package | Role | External dependencies |
| ---------------------------- | ---------------------------------------------------- | ---------------------------- |
| `pkg/netcheck` | The core: `Bundle`, `Check`, `Run`, `RunCheck`, the runners of tcp, udp, icmp, tls, ssh, http, https | `x/crypto/ssh`, `pro-bing`, `yaml.v3` |
| `pkg/netcheck/fetch` | `Load` and `Parse`: bundles from a file or an HTTP(S) server, the `Source` interface | none |
| `pkg/netcheck/fetch/redis` | A bundle in a Redis value | `go-redis` |
| `pkg/netcheck/fetch/consul` | A bundle in a Consul key, or in a service's metadata | `consul/api` |
| `pkg/netcheck/dtls` | The runner of `dtls` checks | `pion/dtls` |
| `pkg/netcheck/sso` | An HTTP client that authenticates with Kerberos or NTLM | `go-spnego` |
| `pkg/metadata` | Build metadata (version, commit, licence) set at link time | none |
| `cmd/netcheck` | The command | `go-flags`, `fatih/color` and the packages above |
| `internal/` | Command-only code: printers, spinner, version command, mock data, logging set-up | |

## Rules

1. **A consumer pays for what it uses.** The core imports the standard library, `x/crypto`, `pro-bing` and `yaml.v3`. A program that runs tcp, tls, http and ssh checks links neither Redis, Consul, DTLS nor Kerberos. A test (`TestImportAllowList`) fails when the core's dependency list grows, and another when the core imports a sub-package (T-11).
2. **Plug-ins are registered by the caller, never by import.** There is no `init()` that registers anything: the caller passes `WithRunner(DTLS, dtls.Runner())`, `WithSSOClient(sso.Client())` or `fetch.WithSource(redis.Source(), "redis", "rediss")`, so what a binary can do is visible in its `main`.
3. **No global state in `pkg/`.** No exported mutable variable (only the `Err...` sentinels), no `init()`, no `os.Exit`, no `panic` for input errors, no use of `http.DefaultClient` and no mutation of `http.DefaultTransport`; `TestNoHiddenBehaviourInPkg` enforces it. Two programs, or two goroutines, can run checks with different settings without seeing each other.
4. **Logging is injected.** The library logs nothing unless `WithLogger` gives it a logger; it never uses the global `slog` default.
5. **The command owns the machine.** The defaults file, environment variables, signals, colours and the `-` and `+sso` scheme suffixes are in `cmd/netcheck`; the library has no string trick that weakens TLS verification.

## The plug-in points

| Point | Type | Replaces or adds |
| ------------------------- | ---------------------------------------------- | ---------------------------------- |
| `WithRunner(p, r)` | `Runner` (one attempt of a check) | The runner of protocol `p`, built-in or not; `dtls` is only a plug-in |
| `WithSSOClient(h)` | `*http.Client` | The client used by checks with `sso: true` |
| `WithHTTPClient(h)` | `*http.Client` | The client of http and https checks; copied when a check needs a transport of its own |
| `fetch.WithSource(s, schemes...)` | `fetch.Source` (location to bytes) | A source for the given URL schemes |
| `fetch.WithHTTPClient(h)` | `*http.Client` | The client of the HTTP(S) source |

A plug-in that opens its own sockets, as `dtls` does, must honour the networks limits: the core exports `VetAddr`, which a plug-in calls on each address it is about to use, and `Target`, which gives the `host:port` a check connects to.
