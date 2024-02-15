# netcheck

A tool to automate connectivity checks.

Create one or more bundles, each containing the set of checks to run. It's possible to write bundles in JSON, YAML or TOML format. See directory `_tests` for examples.

Supported protocols include TCP, UDP and ICMP; TCP and UDP tests require an address including hostname/IP address and port (`host.example.com:80` or `192.168.1.15:443`); ICMP checks only require the hostname or IP address part.

It's possible to specify the default timeout for the whole bundle, or more specific timeouts for each check within a bundle.

This is a sample configuration in YAML format:

```yaml
id: my-bundle 
description: a collection of useful checks
timeout: 5s         # this applies by default to all checks
parallelism: 10     # run these many checks in parallel
checks:
    - address: www.google.com:80    # hostname:port
      protocol: tcp                 # TCP is the default: it can be omitted (see below)
      timeout: 1s      
    - address: www.google.com:443   # hostname:port, all the rest is the default
    - address: dns.example.com:53
      protocol: udp                 # use UDP for DNS
    - address: www.google.com       # ping this host
      protocol: icmp
```

