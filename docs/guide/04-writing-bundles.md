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
| `concurrency` | How many checks run at the same time; a negative value means the default |
| `checks` | The list of checks |

A field the bundle leaves out takes the default of chapter 2. Durations are written as Go durations: `100ms`, `2s`, `1m30s`.

Set `concurrency` to at least 1. A bundle with `concurrency: 0` starts no worker, so its run never ends.

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
| `tcp`, `udp` | `host:port` | `db.example.com:5432` |
| `tls`, `dtls` | `host:port` | `www.example.com:443` |
| `ssh` | `host:port`: always give the port | `git.example.com:22` |
| `icmp` | `host` only, no port | `gateway.example.com` |
| `http`, `https` | `host[:port][/path]`; the port defaults to 80 or 443 | `www.example.com/health` |

## The checks, one by one

### tcp

Opens a TCP connection to `host:port` within the timeout and closes it. It proves that the address is routable and that something accepts connections on that port. It says nothing about what speaks there.

### udp

Opens a UDP socket to `host:port`. UDP has no handshake, so the check succeeds whenever the local system can open the socket and send; it does not wait for a reply, and a firewall that drops the packets is not detected. Treat it as a weak check.

### icmp

Sends `ping.count` echo requests of `ping.size` bytes, `ping.interval` apart, and waits at most the timeout. It needs the privilege described in chapter 3. In this version the check fails if the pings cannot be sent, not when some or all replies are lost.

### tls

Opens a TCP connection, performs a TLS handshake and then verifies three things: the certificate chain is trusted by the system's root certificates, the certificate is valid for the host name of the address, and it has not expired. The error names the failed condition. The log (chapter 6) shows the issuer and the expiry date on success. Use the host name, not the IP address, so that the name can be checked.

### dtls

Meant as `tls` over UDP. In this version it dials with the TLS client over a UDP socket, which does not speak the DTLS protocol, so it does not work against real DTLS servers. Do not rely on it; a proper implementation is planned.

### ssh

Opens a TCP connection and performs the SSH key exchange. The server's refusal of the (absent) credentials counts as success: the check proves that an SSH server answers. The host key is not verified, so the check does not prove the identity of the server.

### http and https

Sends `GET <protocol>://<address>` and succeeds when a response comes back. The status code is not examined: a `404` or a `500` is a response. HTTPS verifies the server certificate with the system's roots. The proxy environment variables (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`) are honoured. With `sso: true` the request answers a `Negotiate` challenge with the local Kerberos or NTLM identity.

## Timeouts, retries and the time a bundle takes

One attempt takes at most `timeout`. A failed attempt is followed by a pause of `wait`, and a check is attempted at most `retries` times. A check that always fails therefore takes about `retries x (timeout + wait)`. A bundle of N such checks, run `concurrency` at a time, takes about `ceil(N / concurrency)` times as long. Set the timeout to what a healthy answer needs, not to what you would be willing to wait.

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
