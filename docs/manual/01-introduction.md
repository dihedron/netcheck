# Introduction

`netcheck` checks network connectivity. You describe what to test in a **bundle**, a small YAML or JSON document that lists **checks**: an address and a protocol each. `netcheck` fetches the bundle, runs its checks concurrently, retries the failures, and prints the outcome as coloured text, JSON, YAML or through a template of your own. The same checks are available to Go programs as a library.

## What it is for

- **Before an application goes live:** can the host reach its database, its identity provider, its message broker, its certificate authority?
- **After a firewall change:** which of the flows an application needs are still blocked?
- **In a pipeline or a probe:** the same bundle, run from many hosts, shows where the network differs.
- **For certificates:** a TLS check also verifies the server certificate, the host name it carries and its expiry date.
- **Inside a Go program:** a workload can check its own flows at start-up and report what is blocked (chapter 14).

## What it checks

| Protocol | What a check proves |
| ---------- | ---------------------------------------------------------------- |
| `tcp` | A TCP connection to the address can be opened |
| `udp` | A datagram can be sent and no ICMP error came back; a silent peer passes, so the check is best effort |
| `icmp` | At least one echo request is answered |
| `tls` | A TLS handshake succeeds, the certificate chain is trusted, the name matches and the certificate has not expired |
| `dtls` | As `tls`, over UDP (DTLS 1.2) |
| `ssh` | The server speaks SSH: the key exchange completes; no credentials are tried, a refused authentication is a pass |
| `http`, `https` | The final response, after at most 10 redirects, has a status from 200 to 399; with `sso` the request authenticates with the local Kerberos or NTLM identity |

Chapter 4 describes each check and chapter 11 states its exact guarantee and what it does not prove.

## Where the bundles live

A bundle can be a file on disk, a document on an HTTP or HTTPS server, a value in Redis, a value in the Consul key/value store, or the metadata of a service in the Consul service registry. A single run can mix sources: `netcheck` takes any number of bundle locations on the command line and runs them in order.

## How to read this manual

| Chapters | For | Content |
| ---------- | ------------------------- | ---------------------------------------------------------- |
| 1 to 3 | Whoever installs it | What `netcheck` is, installation and verification, a first run |
| 4 to 8 | Whoever runs it | Writing, publishing and running bundles; defaults and results; limits; troubleshooting |
| 9 to 13 | Reviewers and maintainers | The architecture, the packages, the protocols, the execution model, the threat model |
| 14 to 17 | Go programmers | The library, loading bundles, plug-ins, testing and the decisions behind the design |
