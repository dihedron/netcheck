# netcheck

[![Go Report Card](https://goreportcard.com/badge/github.com/dihedron/netcheck)](https://goreportcard.com/report/github.com/dihedron/netcheck)

A tool to automate connectivity checks.

Create one or more **bundles**, each containing the set of checks to run.
It's possible to write bundles in JSON or YAML format. See directory `_tests` for examples.

Supported protocols include TCP, UDP, ICMP, SSH, HTTP, HTTPs, TLS over streams (TLS) and TLS over datagrams (DTLS), the latter three including certificate verification; TCP, UDP, SSH, TLS and DTLS checks require an address including hostname/IP address and port (`host.example.com:80` or `192.168.1.15:443`); ICMP checks only require the hostname or IP address; HTTP, HTTPS and SSH checks will use the default protocol ports (80, 443 and 22 respectively) if none is specified.

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
```

Bundles can be:

1. local (i.e. a file on disk)
2. remotely GET-table HTTP/HTTPs resources (i.e., specified as an HTTP URL),
3. values in Consul key/value stores (i.e. stored as a Consul value and pointed at through its key)
4. values in Consul Service Registry services' metadata (i.e. stored in the service's metadata)
5. values in Redis key/value stores (i.e. stored ina Redis value and pointed at through the key)

These things can be mixed, so you can call `netcheck` on multiple bundles at once, mixing them at will.
All checks will be performed bundle by bundle, in the same order that was specified on the command line.

The output can be in `text` mode (the default), in one of `json` and `yaml` formats, or generated dynamically in an arbitrary format based on a user-provided Golang template.

```bash
$> netcheck --format=json local-1.yaml local-2.json \
        http://remote.example.com?id=1 \
        https://remote.example.com/remote-2.json
```
When redirected to file, the `text` mode is not colorised.

When exposing remote bundles via HTTP, make sure the `Content-Type` is properly set, as it is used to identify the format of the checks bundle (YAML, JSON).

The following is an example output of running the check against a local bundle:

```bash
$> netcheck --format=yaml test.yaml
test-bundle:
  - protocol: tcp
    endpoint: www.google.com:443
    success: true
  - protocol: tcp
    endpoint: www.repubblica.it:443
    success: true
  - protocol: tcp
    endpoint: www.repubblica.it:443
    success: false
```

## URL formats

The application assumes that any argument that is not recognised as a command line parameter is the URL of a bundle.

Bundles can be retrieved from multiple sources: a local file, an HTTP server, a Consul Key/Value store, a Consul Service Registry, a Redis instance. If the argument is not parsed as a valid URL, it is assumed to point to a local file.

### Retrieving a bundle from an HTTP server

The application supports downloading a bundle from an HTTP or HTTPs server. The URL is usually an ordinary HTTP address, with the exception that in order to skip the TLS certificate verification, the `https-://` custom scheme is supported. The `-` is the same as specifying `-k` with cURL or `--insecure-skip-verify` on many other applications.

### Retrieving a bundle from a Redis server

The application supports downloading a bundle from a Redis server, in plaintext or with a TLS-protected protocol. The URL is prefixed with the `redis://` scheme for plaintext, `rediss://` for secure-Redis, and `rediss-://` for secure-Redis with skipped verification of the TLS certificate. The URL must also contain the `key` query parameter to specify the key under which the bundle is stored, and can optionally have the `db` query parameter if the key is on a non-default (`!= 0`) database.

### Retrieving a bundle from a Consul Key/Value store

The application supports downloading a bundle from a Consul Key/Value store, in plaintext or with a TLS-protected protocol. The URL is prefixed with the `consulkv://` scheme for plaintext, `consulkvs://` for secure-Consul, and `consulkvs-://` for secure-Consul with skipped verification of the TLS certificate. The URL must also contain the `key` query parameter to specify the key under which the bundle is stored, and can optionally have the `dc` query parameter if the key is not in the default datacentre.

### Retrieving a bundle from a Consul Service Registry

The application supports downloading a bundle from a Consul Service Registry, in plaintext or with a TLS-protected protocol. The URL is prefixed with the `consulsr://` scheme for plaintext, `consulsrs://` for secure-Consul, and `consulsrs-://` for secure-Consul with skipped verification of the TLS certificate. The URL must also contain the `service` query parameter to specify the name of the service, an optional `tag` to help filtering on the list of services in the registry, and a compulsory `meta` value which represents the name of the service metadata map under which the bundle is stored in JSON or YAML format; moreover it can optionally have the `dc` query parameter if the service is not in the default datacentre.

## Using templates for output

When the `--template=<mytemplate.tpl>` command line parameter is specified, it overrides the `--format` parameter setting it to `template`; the application will then proceed to compile the provided template and use it on the following data structure:

```golang
[]struct {
  ID              string  // the id of the bundle
  Description     string  // a description of the bundle
  Timeout         Timeout // the connection timeout
  Retries         int     // how many attempts before declaring failure...
  Wait            Timeout // and how long to wait between those successive attempts
  Concurrency     int     // how many checks to run concurrently
  Checks          []struct {
    Description   string   // the description of the check
    Timeout       Timeout  // the connection timeout (to override the bundle-global one)
    Retries       int      // how many attempts before declaring failure...
    Wait          Timeout  // and how long to wait between those successive attempts
    Address       string   // the address to connect to, possibly including the port
    Protocol      int      // to translate this to "icmp", "tls"... use the .String method
    SSO           bool     // whether to use single-sign-on with SPNEGO authentication
    Result        Result   // the check's result, see below for details
  } // the array of checks in the bundle
}
```

The `Result` structure (inside each of the `Check`s in the `Bundle`) provides two utility methods:

1. `String()`, which either returns the string `"success"` or the string representation of the error, and
1. `IsError()` that provides a way to check if the result represents a failure.

They can be used in the output template too, as shown in the `_tests/output.tpl` file, which provides an extensive example:

```golang
{{ range . }}
Bundle      : {{ .ID | cyan }}{{ if .Description }}
Description : {{ .Description | cyan }}{{ end }}
Timeout     : {{ .Timeout.String | cyan }}
Retries     : {{ .Retries | cyan }} attempts before failing
Wait time   : {{ .Wait | cyan }} between successive attempts
Concurrency : {{ .Concurrency | cyan }} concurrent goroutines
--------------------------------------------------------------------------------{{ range .Checks }}
  - Protocol: {{ .Protocol.String | purple }}
    Host: {{ $a := splitList ":" .Address }}{{ index $a 0 | yellow }}{{ $l := len $a }}{{ if eq $l 2 }}
    Port: {{ index $a 1 | yellow }}{{ end }}
    {{ if .Result.IsError }}Result: {{ .Result.String | red }}{{ else }}Result: {{ .Result.String | green }}{{ end }}{{ end }}
{{ end }}--------------------------------------------------------------------------------
```

The first `range` loop goes over the array of bundles, `Bundle` by bundle; `.` will refer to the current bundle; some information about the bundle is printed out.
The second `range` loop runs over the array of `Check`s within the bundle and prints out:

1. the protocol: see the use of `.Protocol.String` to print the textual representation of the protocol,
1. the host: see how the `splitList` Sprig function is used to split hostname/IP and port apart
1. the port: only if the `splitList` operation returned more than one item (ICMP does not have a port!)
1. the error: only if it is not nil

**Note**: The template engine includes the excellent [Sprig](http://masterminds.github.io/sprig/) library functions to help with values manipulation, plus some additional colouring functions (`blue`, `cyan`, `green`, `magenta`, `purple`, `red`, `yellow`, `white` and their "highlighted" version: `hiblue`, `hicyan`...); the usage is shown in the `_test/output.tpl` template and in the previous example.

### Developing and debugging a template

If you want to try your template without having to wait for real checks to be performed, call the application with the `--template=<mytemplate.tpl>` parameter and **no** bundle: the application will generate a mock result that includes a couple of bundles with two checks each, and will use it to apply the provided template.

Moreover, if you pass the `--print-diagnostics` flag, the application will also print out a representation of the mock result where the fields that were accessed by the template are highlighted in magenta. This can help you understand the data structure.

```bash
$> netcheck --template=_test/output.tpl --print-diagnostics
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
```

## Getting started

The application is pre-built for a multiplicity of platforms (Linux, Windows, Mac) and architectures (AMD64, ARM64), thanks to Golang support for a lot of architectures. Moreover, thanks to nFPM, it comes packaged in many installable formats including DEB, RPM and APK.
To download the binary file, go to the project's GitHub page at https://github.com/dihedron/netcheck and then refer to your OS package manager for installation instructions.

## How to build

Compilation requires Golang 1.25+, `make` and `goreleaser`.

### Building with `make` and `goreleaser`

In order to build, run `make compile`. Running `make help` provides list of all the choices. By default `make` and the default target `make compile` build for `linux/amd64`.

In order to release a new version, commit all outtanding changes, then create a tag with the new version:

```bash
$> git tag -a v1.2.3 -m "v1.2.3 - bug fixes and updated dependencies"
```

and then run `make release`.

To run HTTPs unit tests, run `make self-signed-cert` to generate the `fetch/server.key` and `fetch/server.crt` that will be used by the local HTTPs server.

## How to debug

Run under the `NETCHECK_LOG_LEVEL=debug` environment variable; other acceptable log levels are `info`, `warn`, `error` and `off` (the default).

## TODO

- [ ] Evaluate whether/how to allow loading of custom trust anchors for TLS validation.
- [ ] Implement support for single-sign-on with SPNEGO authentication in HTTP(s) bundle retrieval
