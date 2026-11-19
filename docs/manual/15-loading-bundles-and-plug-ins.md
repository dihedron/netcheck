# Loading bundles and plug-ins

## Loading a bundle

`fetch.Load(ctx, location, opts...)` fetches the text at a location and parses it into a `netcheck.Bundle`. The scheme of the location picks the source; the file source and the HTTP and HTTPS sources are built in, and a location with no scheme is a file name. An unregistered scheme is an error (`ErrUnknownScheme`), never a silent fallback to the file system. Every fetch has a size limit (`WithMaxSize`, 1 MiB by default), a deadline from the context, and never logs credentials or the content.

`fetch.Parse(data, hint)` parses bytes you already have; the `Format` hint (JSON or YAML, `Unknown` to let the content decide) comes from a file extension or a `Content-Type`. A check read without a protocol is a `tcp` check. See `ExampleParse`.

| Option | Effect |
| ------------------------------- | -------------------------------------------------- |
| `WithSource(s, schemes...)` | Registers a `Source` for URL schemes, see `ExampleWithSource` |
| `WithHTTPClient(h)` | The client of the HTTP and HTTPS sources |
| `WithMaxSize(n)` | The largest bundle accepted, in bytes |
| `WithLogger(l)` | Where the fetch logs |

## Redis and Consul

They live in their own packages, so a program that does not use them does not link their clients:

```go
bundle, err := fetch.Load(ctx, "rediss://redis.example.com/checks/payments",
    fetch.WithSource(redis.Source(), "redis", "rediss"),
    fetch.WithSource(consul.KV(), "consulkv", "consulkvs"),
    fetch.WithSource(consul.ServiceRegistry(), "consulsr", "consulsrs"))
```

`redis.Source(opts...)` takes `redis.WithTLSConfig`; `consul.KV` and `consul.ServiceRegistry` take `consul.WithHTTPClient`. The location forms are those of chapter 5.

## Writing a source

A `Source` returns the bytes at a location and, when it knows it, the format:

```go
type Source interface {
    Fetch(ctx context.Context, location *url.URL, maxSize int64) ([]byte, fetch.Format, error)
}
```

It must honour the context and `maxSize`, and must not put credentials in an error: wrap errors with the location as the library does (the redacted form).

## DTLS

`dtls.Runner()` is the runner of `dtls` checks, on `pion/dtls`. It is not in the core, so you register it: `netcheck.WithRunner(netcheck.DTLS, dtls.Runner())`. It resolves the address itself and asks the core, through `netcheck.VetAddr`, whether the networks limits allow it. See the `Example` of the package.

## PostgreSQL and SPIRE

`postgres.Runner()` and `spire.Runner(opts...)` are the runners of the `postgresql` and `spire` checks: `netcheck.WithRunner(netcheck.PostgreSQL, postgres.Runner())`, `netcheck.WithRunner(netcheck.SPIRE, spire.Runner(spire.WithBundleFile(path)))`. `spire.WithSource` takes any `x509bundle.Source`, such as a Workload API source (`workloadapi.NewBundleSource`), which the package does not import so that it needs no gRPC. Both are thin: they set hooks on the check and call `netcheck.TLSHandshake`.

## Hooks

`netcheck.Hooks` are the points where a program steps into a check; a nil field is the default. They are code, so they are set through the API only: `netcheck.WithHooks(h)` for the whole run, `Check.Hooks` for one check, never from a bundle file.

| Hook | When | Effect |
| ---------------------- | ------------------------------------------------ | ------------------------------------------------ |
| `BeforeConnect(ctx, ip)` | for every resolved address, before the socket connects | an error refuses the address (`ErrNetworkNotAllowed` wraps it); every protocol honours it |
| `AfterConnect(ctx, conn)` | the TCP connection is up, nothing sent | the hook may talk on the connection (STARTTLS); an error fails the check; `tcp`, `tls`, `postgresql`, `spire` |
| `VerifyCertificate(chain)` | during the handshake | replaces the verification; `verified` is true only when it accepted, and `verifier` says `custom`; `tls`, `postgresql`, `spire` |
| `AfterHandshake(ctx, state)` | the handshake is done and the chain accepted | a policy on the result (minimum version, ALPN, pinning); an error fails the check |

The check's own `AfterConnect`, `VerifyCertificate` and `AfterHandshake` replace the run's. `BeforeConnect` is a veto: the networks limits (`--allow-net`, `--deny-net`), the run's hook and the check's hook all run and any one can refuse, so no hook can lift a limit. `Validate` rejects a check that sets a hook its protocol does not honour, so that a security hook is never dropped in silence. A panic in a hook fails the check.

## SSO

`sso.Client(opts...)` returns an `*http.Client` that authenticates with the local Kerberos or NTLM identity (`go-spnego`); `sso.WithTLSConfig` configures its TLS. Pass it to `netcheck.WithSSOClient` for the checks with `sso: true` and to `fetch.WithHTTPClient` to load bundles from a server that needs it. A check with `sso: true` cannot be combined with TLS options or networks limits, because the client is used as given: it fails with an error that says so. See the `Example` of the package.

## Your own runner

```go
runner := netcheck.RunnerFunc(func(ctx context.Context, c netcheck.Check, d netcheck.Defaults) (netcheck.Detail, error) {
    addr, err := netcheck.Target(c.Protocol, c.Address) // host:port with the default port applied
    ...
})
```

A runner performs one attempt: the check it receives has its defaults applied, and its context carries the attempt's deadline. The `Detail` is kept whether or not the error is nil. A runner that opens its own sockets must call `netcheck.VetAddr(ctx, ip)` on each address first, and `TLSFacts.Describe` fills the certificate facts from an `x509.Certificate`.

## A client SDK

The ligo client SDK turns a flow such as `{Protocol: TCP, Destination: db.lab, Ports: 5432, TLS: true}` into one `tls` check per port with `TLSOptions{ServerName, RootCAs}` (a `tcp` check when TLS is off, a `udp` check for UDP flows), runs them with `Run` under the workload's context, and reports the failed ones by flow. The range expansion is the SDK's job; the library takes a `[]Check` and returns a `Report`.
