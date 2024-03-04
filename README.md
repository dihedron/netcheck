# netcheck

A tool to automate connectivity checks.

Create one or more bundles, each containing the set of checks to run. It's possible to write bundles in JSON or YAML format. See directory `_tests` for examples.

Supported protocols include TCP, UDP, ICMP, TLS over streams (TLS) and TLS over datagrams (DTLS), the latter two including certificate verification; TCP, UDP, TLS and DTLS checks require an address including hostname/IP address and port (`host.example.com:80` or `192.168.1.15:443`); ICMP checks only require the hostname or IP address.

It's possible to specify the default timeout for the whole bundle, or more specific timeouts for each check within a bundle. Moreover it's possible to specify how many times to retry in case of failure and a wait time between attempts.

This is a sample bundle in YAML format:

```yaml
id: my-bundle 
description: a collection of useful checks
timeout: 5s         # this applies by default to all checks
parallelism: 10     # run these many checks in parallel
retries: 3          # in case of failure, try these many times...
wait: 5s            # ... waiting this long between attempts
checks:
  - address: www.google.com:80    # hostname:port
    protocol: tcp                 # TCP is the default: it can be omitted (see below)
    timeout: 1s                   # specify a different timeout
  - address: www.google.com:443   # hostname:port, all the rest is the default
  - address: dns.example.com:53
    protocol: udp                 # use UDP for DNS
  - address: www.google.com       # ping this host
    protocol: icmp
```

The command can run against:

1. local bundles
1. remotely GET-table HTTP/HTTPs resources, 
1. values in Consul key/value stores
1. values in Consul Service Registry metadata
1. values in Redis values

These things can be mixed, so you can call `netcheck` on multiple bundles at once, mixing them at will. All checks will be performed bundle by bindle, in the same order that was specified on the command line.

The output can be in text mode (the default), or in one of `json` and `yaml` formats. A future version will allow to specify a Golang template in order to produce the output in an arbitrary format.

```bash
$> netcheck --format=json local-1.yaml local-2.json \
        local-3.toml http://remote.example.com?id=1 \
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

## How to build

Compilation requires Golang 1.22+.

In order to build, run `make`.

In order to install to the default location (`/usr/local/bin`) run `sudo make install`; to remove it, run `sudo make uninstall`.
In order to specify a different install directory use the `PREFIX` environment variable; the same for uninstalling:

```bash
$> make && sudo make install PREFIX=/usr/bin
```

The default target compiles for `linux/amd64`. 

It's possible to cross compile to any other supported GOOS/GOARCH combination (as per `go tool dist list`), e.g. `make windows/amd64` to build for 64-byte Windows on AMD/Intel CPUs.

## How to debug

Run under the `NETCHECK_LOG_LEVEL=debug` environment variable; other acceptable log levels are `info`, `warn` and `error`.

## TODO

- [ ] Support bundle download from Hashicorp Consul (both KV and Service Registry)
- [ ] Support specification of custom Golang template on the command line for on-the-fly custom report production