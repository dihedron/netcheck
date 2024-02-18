# netcheck

A tool to automate connectivity checks.

Create one or more bundles, each containing the set of checks to run. It's possible to write bundles in JSON, YAML or TOML format. See directory `_tests` for examples.

Supported protocols include TCP, UDP and ICMP; TCP and UDP tests require an address including hostname or IP address, and port (`host.example.com:80` or `192.168.1.15:443`); ICMP checks only require the hostname or IP address.

It's possible to specify the default timeout for the whole bundle, or more specific timeouts for each check within a bundle.

It's also possible to provide one or more triggers that specify actions to execute after a successful test (`on: success`), a failed one (`on: failure`) or no matter the result (`on: always`).

The action must specify a command and can have optional arguments; the execution captures the standard output, standard error and exit code, which is reported alongside the test result. If a per-trigger timeout is specified, the associated action must terminate before the timeout expires or it will be aborted.

Work is underway to support inline scripts as trigger actions (see **TODO** below).

This is a sample bundle in YAML format:

```yaml
id: my-bundle 
description: a collection of useful checks
timeout: 5s         # this applies by default to all checks
parallelism: 10     # run these many checks in parallel
checks:
  - address: www.google.com:80    # hostname:port
    protocol: tcp                 # TCP is the default: it can be omitted (see below)
    timeout: 1s                   # specify a different timeout
    triggers:
    - on: success                 # trigger the command when the check is successful
      command: echo
      args: ["it was a success"]
      timeout: 1s                 # if the command takes more than 1s, abort
    - on: failure                 # trigger upon failure
      command: echo
      args: ["it was a failure"]
  - address: www.google.com:443   # hostname:port, all the rest is the default
  - address: dns.example.com:53
    protocol: udp                 # use UDP for DNS
  - address: www.google.com       # ping this host
    protocol: icmp
```

The command can run against local bundles or remotely GET-table HTTP/HTTPs resources. The two things can be mixed.

The output can be in text mode (the default), or in one of `json`, `yaml` and `toml` formats.

```bash
$> netcheck --format=json local-1.yaml local-2.json \
        local-3.toml http://remote.example.com?id=1 \
        https://remote.example.com/remote-2.json 
```
When redirected to file, the `text` mode is not colorised.

When exposing remote bundles via HTTP, make sure the `Content-Type` is properly set, as it is used to identify the format (YAML, JSON, TOML).

The following is an example output of running the check against a local bundle, including triggering the `on: failure` action when the TCP connection to an invalid HTTPs port (`445`) failed; the commandlet (`echo "it was a failure"`) only outputs to standard out and exits with `0`, as shown in the output:

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
    actions:
      - command:
          - echo
          - it was a failure
        exitcode: 0
        stdout: |
          it was a failure
```

## How to build

The build requires Golang 1.22+.

In order to build, run `make`.

In order to install to the default location (`/usr/lib/bin`) run `sudo make install`; to remove it, run `sudo make uninstall`.
In order to specify a different install directory use the `PREFIX` environment variable; the same for uninstalling:

```bash
$> make && sudo make install PREFIX=/usr/bin
```

The default target compiles for `linux/amd64`. 

It's possible to cross compile to any other supported GOOS/GOARCH combination (as per `go tool dist list`), e.g. `make windows/amd64` to build for 64-byte Windows on AMD/Intel CPUs.

## How to debug

Run under the `NETCHECK_LOG_LEVEL=debug` environment variable; other acceptable log levels are `info`, `warn` and `error`.

## TODO

- [ ] Move Unix and Windows trigger execution to different compilation units and enable conditional compilation
- [ ] Support inline scriptlets in triggers on Unix platforms (must start with `#!` to be recognised as such)
- [ ] Support specification of custom Golang template on the command line for on-the-fly custom report production