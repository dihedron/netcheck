// Package netcheck runs network connectivity checks (tcp, udp, icmp, tls,
// ssh, http, https; dtls through a plug-in) and reports what it found.
//
// A Bundle is a list of Checks with shared settings; Run executes them with a
// bounded worker pool and returns a Report, one Result per check, each with
// its Status, the number of attempts, the latency and the evidence in Detail.
// RunCheck runs a single check. Both honour the context: when it ends, the
// checks that did not finish are reported Skipped.
//
// The package keeps no global state, reads no file, logs nothing unless given
// a logger (WithLogger) and never exits the process. What each check proves:
//
//   - tcp: a connection was established.
//   - udp: a datagram was sent and no ICMP error came back within
//     Defaults.UDPGrace. A silent peer passes: the check is best effort.
//   - icmp: at least one echo reply came back. Privilege is the caller's
//     choice (PingOptions.Privileged).
//   - tls: the handshake succeeded, the chain verified against TLSOptions.RootCAs
//     (or the system roots), the name matched, the certificate had not expired.
//   - ssh: the transport handshake completed; a refused authentication is a
//     pass. The host key's fingerprint is returned and not verified.
//   - http, https: the final response, after at most 10 redirects, had a
//     status from 200 to 399. A redirect from https to http fails unless
//     Check.AllowDowngrade is set.
//
// Bundles from untrusted places should be bounded with WithLimits: a check is
// a connection the host makes on the bundle's behalf.
package netcheck
