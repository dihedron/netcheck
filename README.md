# netcheck

A tool to automate connectivity checks.

Create one or more bundles, each containing the set of checks to run. It's possible to write bundles in JSON, YAML or TOML format. See directory `_tests` for examples.

Supported protocols include TCP, UDP and ICMP; TCP and UDP tests require an address including hostname or IP address, and port (`host.example.com:80` or `192.168.1.15:443`); ICMP checks only require the hostname or IP address .

It's possible to specify the default timeout for the whole bundle, or more specific timeouts for each check within a bundle.

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
    - address: www.google.com:443   # hostname:port, all the rest is the default
    - address: dns.example.com:53
      protocol: udp                 # use UDP for DNS
    - address: www.google.com       # ping this host
      protocol: icmp
```

The command can run against local bundles or remotely GET-table HTTP/HTTPs resources. The two things can be mixed.

The output can be in text mode (the default), or in one of `json`, `yaml` and `toml` formats.

```bash
$> netcheck --format=json local-1.yaml local-2.json local-3.toml http://remote.example.com?id=1 https://remote.example.com?id=1 
```
When redirected to file, the "text" mode is not colorised.

When exposing remote bundles via HTTP, make sure the Content-Type is properly set, as it is used to identify the format (YAML, JSON, TOML).

## How to build

The build requires an installation of Golang 1.22+.

In order to build, run `make`; in order to install to `/usr/lib/bin` run `sudo make install`; to remove it, run `sudo make uninstall`.
