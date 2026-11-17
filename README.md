# netcheck

[![Go Report Card](https://goreportcard.com/badge/github.com/dihedron/netcheck)](https://goreportcard.com/report/github.com/dihedron/netcheck)

A tool to automate connectivity checks.

Create one or more **bundles**, each containing the set of checks to run.
It's possible to write bundles in JSON or YAML format. See directory `examples/` for examples.

Supported protocols include TCP, UDP, ICMP, SSH, HTTP, HTTPs, TLS over streams (TLS) and TLS over datagrams (DTLS), the latter three including certificate verification; TCP, UDP, SSH, TLS and DTLS checks require an address including hostname/IP address and port (`host.example.com:80` or `192.168.1.15:443`); ICMP checks only require the hostname or IP address; HTTP, HTTPS and SSH checks will use the default protocol ports (80, 443 and 22 respectively) if none is specified. 

HTTP/HTTPS and SSH checks behave differently from TCP and TLS checks in that they also try to establish a valid connection at the application level of the network stack. This means that while a TCP check to `example.com:80` will connect to the web server `example.com` on port `80` and report whether the packet flow succeeded, an HTTP test against the same address will also check if the response is a valid (HTTP 200 status code) response; the HTTPS check (typically run against port `443`) will additionally perform a TLS handshake and check that the server certificate is valid. When the `sso` flag is set to true, the check will also try to use the local Kerberos/NTLM identity to authenticate the request on the web server.

It's possible to specify the default timeout for the whole bundle, or more specific timeouts for each check within a bundle.
Moreover it's possible to specify how many times to retry in case of failure and a wait time between attempts.

This is a sample bundle in YAML format:

```yaml
id: my-bundle
description: a collection of useful checks
timeout: 5s         # this applies by default to all checks
concurrency: 10     # run these many checks concurrently
retries: 3          # in case of failure, try these many times...
wait: 5s            # ... waiting this long between attempts
checks:
  - address: www.google.com:80     # hostname:port
    protocol: tcp                  # TCP is the default: it can be omitted (see below)
    timeout: 1s                    # specify a different timeout
  - address: www.google.com:443    # hostname:port, all the rest is the default
  - address: dns.example.com:53
    protocol: udp                  # use UDP for DNS
  - address: www.google.com        # ping this host
    protocol: icmp
  - address: github.com:22         # try to SSH to this host
    protocol: ssh
  - name: Google (HTTPs with page) # ... with its own check name
    address: www.google.com/imghp?hl=en&authuser=0&ogbl
    protocol: https                # this is HTTPs, you can test a specific resource!
    sso: true
```

Bundles can be:

1. local (i.e. a file on disk)
2. remotely GET-table HTTP/HTTPs resources (i.e., specified as an HTTP URL),
3. values in Consul key/value stores (i.e. stored as a Consul value and pointed at through its key)
4. values in Consul Service Registry services' metadata (i.e. stored in the service's metadata)
5. values in Redis key/value stores (i.e. stored ina Redis value and pointed at through the key)

These things can be mixed, so you can call `netcheck` on multiple bundles at once, mixing them at will.
All checks will be performed bundle by bundle, in the same order that was specified on the command line.

The application supports retrieving bundles from authenticated HTTP/HTTPS server. To use basic authentication, use the ordinary syntax (e.g. `https-://<username>:<password>@example.com:443`); to retrieve it from a Kerberos/NTLM protected server, you have to specify a custom schema with an additional `+sso`( e.g. `https+sso-://example.com/` for HTTPS with single sign on and without certificate). 

The output can be in `text` mode (the default), in one of `json` and `yaml` formats, or generated dynamically in an arbitrary format based on a user-provided Golang template.

```bash
$> netcheck --format=json local-1.yaml local-2.json \
        http://remote.example.com?id=1 \
        https://remote.example.com/remote-2.json
```
When redirected to file, the `text` mode is not colorised.

When exposing remote bundles via HTTP, make sure the `Content-Type` is properly set, as it is used to identify the format of the checks bundle (YAML, JSON).

The following is an example output of running the check against a local bundle (`▲` passed, `▼` failed):

```bash
$> netcheck test.yaml
► test-bundle
▲   443 tcp   → www.google.com                 : Google
▼   445 tcp   → www.repubblica.it               : wrong port        dial tcp 18.66.1.1:445: i/o timeout
```

The exit status is `0` when every check passed, `1` for a usage, load or template error (or an interruption with Ctrl-C), and `2` when at least one check failed, so that `netcheck bundle.yaml || alert` works as expected. The JSON and YAML outputs give, for each check, the status, the attempts, the latency, the error and the evidence gathered (HTTP status, certificate, SSH fingerprint...).

A bundle is a list of connections the host makes on its author's behalf: when bundles come from places you do not fully trust, bound them with `--allow-net=<CIDR>` and `--deny-net=<CIDR>` (repeatable).

## Using it as a library

The checks are available as the Go package `github.com/dihedron/netcheck/v2/pkg/netcheck`: no global state, cancellable through a `context.Context`, with structured results. Bundle loading (`pkg/netcheck/fetch`, with `redis` and `consul` sub-packages), DTLS (`pkg/netcheck/dtls`) and SSO (`pkg/netcheck/sso`) are plug-ins the caller registers explicitly, so that a program links only what it uses (the manual, built by `make docs`, describes the library in chapters 14 and 15):

```go
report, err := netcheck.Run(ctx, &netcheck.Bundle{
    Checks: []netcheck.Check{{Address: "db.example.com:5432", Protocol: netcheck.TCP}},
})
if err == nil && !report.OK() { /* report.Results has one Result per check */ }
```

## URL formats

The application assumes that any argument that is not recognised as a command line parameter is the URL of a bundle.

Bundles can be retrieved from multiple sources: a local file, an HTTP server, a Consul Key/Value store, a Consul Service Registry, a Redis instance. If the argument is not parsed as a valid URL, it is assumed to point to a local file.

### Retrieving a bundle from an HTTP server

The application supports downloading a bundle from an HTTP or HTTPs server. The URL is usually an ordinary HTTP address, with the exception that in order to skip the TLS certificate verification, the `https-://` custom scheme is supported. The `-` is the same as specifying `-k` with cURL or `--insecure-skip-verify` on many other applications.

The application requires a custom schema to identify HTTP/HTTPS resources that require Kerberos/NTLM authetication: 

* `http+sso://` for HTTP with Kerberos/NTLM authentication, 
* `https+sso://` for HTTPS with Kerberos/NTLM authentication, and 
* `https+sso-://` for HTTPS with Kerberos/NTLM authentication and without certificate verification.

### Retrieving a bundle from a Redis server

The application supports downloading a bundle from a Redis server, in plaintext or with a TLS-protected protocol. The URL is prefixed with the `redis://` scheme for plaintext, `rediss://` for secure-Redis, and `rediss-://` for secure-Redis with skipped verification of the TLS certificate. The URL must also contain the `key` query parameter to specify the key under which the bundle is stored, and can optionally have the `db` query parameter if the key is on a non-default (`!= 0`) database.

### Retrieving a bundle from a Consul Key/Value store

The application supports downloading a bundle from a Consul Key/Value store, in plaintext or with a TLS-protected protocol. The URL is prefixed with the `consulkv://` scheme for plaintext, `consulkvs://` for secure-Consul, and `consulkvs-://` for secure-Consul with skipped verification of the TLS certificate. The URL must also contain the `key` query parameter to specify the key under which the bundle is stored, and can optionally have the `dc` query parameter if the key is not in the default datacentre.

### Retrieving a bundle from a Consul Service Registry

The application supports downloading a bundle from a Consul Service Registry, in plaintext or with a TLS-protected protocol. The URL is prefixed with the `consulsr://` scheme for plaintext, `consulsrs://` for secure-Consul, and `consulsrs-://` for secure-Consul with skipped verification of the TLS certificate. The URL must also contain the `service` query parameter to specify the name of the service, an optional `tag` to help filtering on the list of services in the registry, and a compulsory `meta` value which represents the name of the service metadata map under which the bundle is stored in JSON or YAML format; moreover it can optionally have the `dc` query parameter if the service is not in the default datacentre.

## Using templates for output

When the `--template=<mytemplate.tpl>` command line parameter is specified, it overrides the `--format` parameter setting it to `template`; the application will then proceed to compile the provided template and use it on a list of reports, one per bundle (the same shape as the JSON output, see the Setup Guide and the library package `pkg/netcheck`):

```golang
[]struct {                   // netcheck.Report
  BundleID    string         // the id of the bundle
  Description string         // a description of the bundle
  Started     time.Time      // when the run started
  Duration    time.Duration  // how long it took
  Results     []struct {     // netcheck.Result, one per check, in order
    Status    string         // "passed", "failed" or "skipped"
    Attempts  int            // how many attempts were made
    Latency   time.Duration  // the last attempt's latency
    Err       error          // the last error; nil when passed
    Check     struct {       // the check as it ran, defaults applied
      Name, Address string
      Protocol      string   // "tcp", "udp", "icmp", "tls", "dtls", "ssh", "http", "https"
      Timeout, Wait time.Duration
      Retries       int
      SSO           bool
    }
    Detail    struct {       // the evidence the protocol gathered
      Remote, SSHBanner, SSHHostKey string
      HTTPStatus, PacketsSent, PacketsRecv int
      TLS *struct { Version, CipherSuite, Subject, Issuer string; NotAfter time.Time; SANs []string; Verified bool } // nil without a TLS session
    }
  }
}
```

A template written for version 1 must be rewritten (`.Result.IsError` and `.Result.String` are gone); the extensive example in `examples/output.tpl` shows the new shape:

```golang
{{ range . }}
Bundle       : {{ .BundleID | cyan }}{{ if .Description }}
Description  : {{ .Description | cyan }}{{ end }}
Duration     : {{ .Duration | cyan }}
Checks       :{{ range .Results }}
  - Protocol : {{ .Check.Protocol | purple }}
    Attempts : {{ .Attempts }}
    {{ if eq (print .Status) "passed" }}Result   : {{ "passed" | green }}{{ else }}Result   : {{ print .Err | red }}{{ end }}{{ end }}
{{ end }}
```

The `range` loops go over the reports and over each report's results; `.` is the current item.

**Note**: The template engine includes the excellent [Sprig](http://masterminds.github.io/sprig/) library functions to help with values manipulation, plus some additional colouring functions (`blue`, `cyan`, `green`, `magenta`, `purple`, `red`, `yellow`, `white` and their "highlighted" version: `hiblue`, `hicyan`...); the usage is shown in the `examples/output.tpl` template and in the previous example.

### Developing and debugging a template

If you want to try your template without having to wait for real checks to be performed, call the application with the `--template=<mytemplate.tpl>` parameter and **no** bundle: the application will generate a mock result that includes a couple of reports with two checks each, and will use it to apply the provided template.

Moreover, if you pass the `--print-diagnostics` flag, the application will also print out a representation of the mock result where the fields that were accessed by the template are highlighted in magenta. This can help you understand the data structure.

```bash
$> netcheck --template=examples/output.tpl --print-diagnostics
```

### Configuring the defaults

The application can optionally load the default values to use when not provided in the bundle.

On Unix systems it tries to load the defaults from a file called `netcheck.conf`, in YAML format, in the following paths: `./netcheck.conf`, `~/netcheck.conf`, `/etc/netcheck.conf`.

On Windows systems it tries to load the defaults from a file called `netcheck.conf`, in YAML format, in the following paths: `./netcheck.conf`, `~/netcheck.conf`.

The file containing the default values, that is installed alongsite the application under `/etc`, is in the sources root directory:

```yaml
timeout: 2s           # fail a check after 2 seconds without response
retries: 3            # try up to 3 times if check fails
wait: 100ms           # wait 100 millisendons between attempts
concurrency: 10       # run 10 checks concurrently
ping:
    count: 10         # send 10 packets
    interval: 100ms   # send an ICMP packet every 100 microseconds
    size: 64          # 64 bytes
    privileged: true  # raw ICMP sockets (cap_net_raw or root): the default on Linux and Windows
```

## Getting started

The application is pre-built for a multiplicity of platforms (Linux, Windows, Mac) and architectures (AMD64, ARM64), thanks to Golang support for a lot of architectures. Moreover, thanks to nFPM, it comes packaged in many installable formats including DEB, RPM and APK.
To download the binary file, go to the project's GitHub page at https://github.com/dihedron/netcheck and then refer to your OS package manager for installation instructions.

## How to build

Compilation requires Golang 1.27+, `make`, `goreleaser` and `golangci-lint` (see `make help`).

In order to build, run `make compile` (it checks the code first: `gofmt`, `go vet`, `golangci-lint`); the binary for `linux/amd64` is in `dist/`. Running `make help` provides the list of all the choices. `make vuln` checks the dependencies for known vulnerabilities and `make secrets` scans for secrets: run both before committing.

`make docs` builds the manual (setup, operation, architecture and the library, one PDF) into `build/docs/` (it needs Docker or Podman; the sources are in `docs/manual/`). `make stress` runs the stress and chaos test for 20 seconds with the race detector; `make soak` runs it for hours and is meant for an overnight run (`SOAK_SECONDS` sets the length).

`go test ./...` runs the tests, which need neither network nor Docker; the Redis source test runs in a container and is built only with `go test -tags integration ./...`.

In order to release a new version, run `make release-major`, `make release-minor` or `make release-patch`: it creates and pushes the tag, and the GitHub release workflow builds, signs and publishes the release. The module path is `github.com/dihedron/netcheck/v2` from version 2.

The design of the tool and of its library form is in `.specs/`: `netcheck-reverse-engineering.md` (what the code does today), `netcheck-library.md` (the library) and `netcheck-restructure.md` (the repository layout).

## How to debug

Run under the `NETCHECK_LOG_LEVEL=debug` environment variable; other acceptable log levels are `info`, `warn`, `error` and `off` (the default).

## TODO

- [ ] Evaluate whether/how to allow loading of custom trust anchors for TLS validation.
- [ ] Implement support for single-sign-on with SPNEGO authentication in HTTP(s) bundle retrieval
