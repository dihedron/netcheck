# Protocols

Each check proves one thing. This chapter states what, why it was defined so, and what it leaves unproven (T-5: a check must not report success when nothing was proven).

::: landscape

| Protocol | Pass means | Deliberately not proven | Design decision |
| ---------- | ---------------------------- | ----------------------------- | ---------------------------------------- |
| `tcp` | A connection was established | What speaks on the port | Dial with the attempt's deadline |
| `udp` | A datagram was sent and no ICMP error returned within 200 ms | That anything listens: a silent peer passes | UDP has no handshake. Dialling alone proves nothing, so the check sends a datagram and waits a short grace window (`Defaults.UDPGrace`) for "port unreachable" |
| `icmp` | At least one echo reply | The absence of loss: 9 lost out of 10 is a pass | The loss is reported in `detail`; privilege (`Ping.Privileged`) is the caller's choice and never guessed from the OS |
| `tls` | Handshake done, chain trusted, name matches, not expired | Application behaviour | Verification is on by default and can only be switched off through the API (`TLSOptions`), recording `verified: false` in the result |
| `dtls` | As `tls`, over UDP (DTLS 1.2) | As `tls` | A plug-in, so that the core does not link `pion/dtls` |
| `ssh` | The transport handshake completed | The identity of the host and any credential | A refused authentication is a pass. The host key's SHA-256 fingerprint is returned so the caller can pin it; pinning itself is not in this version |
| `http`, `https` | The final status is 200 to 399 | The body | At most 10 redirects; https to http is refused; the body is read up to 64 KiB and discarded; a rejected certificate is described in `detail` |

:::

## Addresses

Default ports apply when the address has none (ssh 22, http 80, https 443). IPv6 literals are written in brackets and handled with `net.SplitHostPort`, never by counting colons. The path of an http address is dropped when the connection target is needed (`Target`).

## What a failure carries

A failed attempt keeps its evidence: a TLS check that the certificate failed still reports the certificate's subject, issuer, expiry and names, because that is what the operator needs to fix it. The `error` of a result wraps the cause, so a Go caller can ask `errors.Is(err, context.DeadlineExceeded)` or `errors.Is(err, netcheck.ErrNetworkNotAllowed)`.

## Known limits

- The real Kerberos exchange needs a KDC. The unit tests cover the wiring and the failure path; the exchange is part of the lab plan (chapter 16).
- ICMP without a privilege uses unprivileged datagram sockets, which the kernel allows only for the groups in `net.ipv4.ping_group_range`.
- A UDP check cannot tell a firewall that drops from a host that is silent.
