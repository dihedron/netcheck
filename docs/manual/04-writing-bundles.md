# Writing bundles

A bundle is a YAML or JSON document. The file extension (`.yaml`, `.yml`, `.json`) tells `netcheck` which one it is; for other sources see chapter 5. A YAML document read from Redis or Consul must start with `---`.

## The bundle

```{.yaml include="examples/netcheck.yaml"}
```

| Field | Meaning |
| ------------- | ------------------------------------------------------------------ |
| `id` | The name of the bundle, shown in the output |
| `description` | A free text |
| `timeout` | Default timeout of one attempt of a check, such as `2s` or `500ms` |
| `retries` | Default number of attempts per check; a value below 1 means the default |
| `wait` | Default pause between attempts, such as `1s` |
| `concurrency` | How many checks run at the same time; a value below 1 means the default |
| `checks` | The list of checks |

A field the bundle leaves out takes the default of chapter 7. Durations are written as Go durations: `100ms`, `2s`, `1m30s`.

## The check

| Field | Meaning |
| ---------- | ------------------------------------------------------------------- |
| `name` | A label shown in the text output (cut to 32 characters) |
| `address` | Where to connect; its form depends on the protocol (below) |
| `protocol` | One of `tcp`, `udp`, `icmp`, `tls`, `dtls`, `ssh`, `http`, `https`; `tcp` when omitted |
| `timeout` | Overrides the bundle's timeout for this check |
| `retries` | Overrides the bundle's retries for this check |
| `wait` | Overrides the bundle's wait for this check |
| `sso` | `true` to authenticate HTTP and HTTPS checks with the local Kerberos or NTLM identity |

## Addresses

| Protocol | Address | Example |
| ---------------- | ------------------------------------------ | ------------------------ |
| `tcp`, `udp` | `host:port`; an IPv6 literal goes in brackets | `db.example.com:5432`, `[2001:db8::1]:443` |
| `tls`, `dtls` | `host:port` | `www.example.com:443` |
| `ssh` | `host[:port]`; the port defaults to 22 | `git.example.com` |
| `icmp` | `host` only, no port | `gateway.example.com` |
| `http`, `https` | `host[:port][/path]`; the port defaults to 80 or 443 | `www.example.com/health` |

## The checks, one by one

### tcp

Opens a TCP connection to `host:port` within the timeout and closes it. It proves that the address is routable and that something accepts connections on that port. It says nothing about what speaks there.

### udp

Sends an empty datagram to `host:port` and watches the socket for a short while (200 ms). A host with nothing listening answers with an ICMP "port unreachable", which fails the check. A peer that stays silent passes: UDP has no handshake, so a firewall that silently drops the packets is not detected. Treat it as a best-effort check.

### icmp

Sends `ping.count` echo requests of `ping.size` bytes, `ping.interval` apart, and waits at most the timeout. The check passes when **at least one** reply comes back and fails on a total loss; the JSON output reports the packets sent and received. It needs the privilege described in chapter 3 (raw sockets, `ping.privileged: true`, the default on Linux and Windows); with `ping.privileged: false` it uses unprivileged ICMP sockets, which need `net.ipv4.ping_group_range` to include your group.

### tls

Opens a TCP connection, performs a TLS handshake and then verifies three things: the certificate chain is trusted by the system's root certificates, the certificate is valid for the host name of the address, and it has not expired. The error names the failed condition. The JSON and YAML output (chapter 6) describe the certificate: subject, issuer, expiry date, names, protocol version and cipher suite, also for a certificate that was rejected. Use the host name, not the IP address, so that the name can be checked.

### dtls

`tls` over UDP: a real DTLS handshake (DTLS 1.2) with the same verification as `tls`. A UDP port that does not speak DTLS fails the check by timeout.

### ssh

Opens a TCP connection and performs the SSH key exchange. The server's refusal of the (absent) credentials counts as success: the check proves that an SSH server answers. The host key is not verified, so the check does not prove the identity of the server; its SHA-256 fingerprint and the server's banner are reported in the JSON and YAML output, so that you can compare the fingerprint with the one you expect.

### http and https

Sends `GET <protocol>://<address>` and passes when the final response has a status from 200 to 399: a `404` or a `500` fails the check, and the status is reported. Up to 10 redirects are followed; a redirect from HTTPS to HTTP fails the check. Only the first 64 KiB of the body are read. HTTPS verifies the server certificate with the system's roots. The proxy environment variables (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`) are honoured, unless `--allow-net` or `--deny-net` is given (chapter 6). With `sso: true` the request is authenticated with the local Kerberos identity (SPNEGO).

## Timeouts, retries and the time a bundle takes

One attempt takes at most `timeout`. A failed attempt is followed by a pause of `wait`, except after the last one, and a check is attempted at most `retries` times. A check that always fails therefore takes at most `retries x timeout + (retries - 1) x wait`. A bundle of N such checks, run `concurrency` at a time, takes about `ceil(N / concurrency)` times as long. Set the timeout to what a healthy answer needs, not to what you would be willing to wait.

## A complete example

```yaml
id: payments-prerequisites
description: what the payments service needs
timeout: 3s
retries: 2
wait: 500ms
concurrency: 5
checks:
  - name: database
    address: db.internal.example.com:5432
  - name: identity provider (TLS and certificate)
    address: idp.example.com:443
    protocol: tls
  - name: gateway reachable
    address: gw.internal.example.com
    protocol: icmp
  - name: bastion SSH
    address: bastion.example.com:22
    protocol: ssh
  - name: internal API health
    address: api.internal.example.com/health
    protocol: https
    sso: true
```
