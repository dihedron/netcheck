# Introduction

`netcheck` checks network connectivity. You describe what to test in a **bundle**, a small YAML or JSON document that lists **checks**: an address and a protocol each. `netcheck` fetches the bundle, runs its checks concurrently, retries the failures, and prints the outcome as coloured text, JSON, YAML or through a template of your own.

## What it is for

- **Before an application goes live:** can the host reach its database, its identity provider, its message broker, its certificate authority?
- **After a firewall change:** which of the flows an application needs are still blocked?
- **In a pipeline or a probe:** the same bundle, run from many hosts, shows where the network differs.
- **For certificates:** a TLS check also verifies the server certificate, the host name it carries and its expiry date.

## What it checks

| Protocol | What a check proves |
| ---------- | ---------------------------------------------------------------- |
| `tcp` | A TCP connection to the address can be opened |
| `udp` | A UDP socket to the address can be opened; no reply is expected (see the notes in chapter 4) |
| `icmp` | The host answers pings |
| `tls` | A TLS handshake succeeds, the certificate matches the host name and has not expired |
| `dtls` | As `tls`, over UDP (see the notes in chapter 4) |
| `ssh` | The server speaks SSH: the key exchange completes; no credentials are tried |
| `http`, `https` | A GET request to the address returns a response; with `sso` the request authenticates with the local Kerberos or NTLM identity |

## Where the bundles live

A bundle can be a file on disk, a document on an HTTP or HTTPS server, a value in Redis, a value in the Consul key/value store, or the metadata of a service in the Consul service registry. A single run can mix sources: `netcheck` takes any number of bundle locations on the command line and runs them in order. Chapter 5 shows how to publish a bundle in each place and chapter 6 how to run `netcheck` against it.

## How to read this guide

1. Chapter 2 describes how `netcheck` works inside.
2. Chapter 3 installs it.
3. Chapter 4 explains how to write bundles and describes every check.
4. Chapter 5 explains how to publish bundles.
5. Chapter 6 explains how to run `netcheck`: sources, output formats, templates, logging and profiling.

This guide describes the command. A Go library with the same checks is being prepared: its design is in `.specs/netcheck-library.md` of the source repository, and the guide will cover it when it is released.
