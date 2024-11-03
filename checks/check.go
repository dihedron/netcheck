package checks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"strings"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

// Check represents a single check to perform.
type Check struct {
	// id is the internal if of the check.
	id int
	// Name is the external name of the check.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Timeout is the timeout before considering the check failed.
	Timeout Timeout `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	// Retries is the number of retries before considering the check failed.
	Retries int `json:"retries,omitempty" yaml:"retries,omitempty"`
	// Wat is the wait time between check attempts.
	Wait Timeout `json:"wait,omitempty" yaml:"wait,omitempty"`
	// Address i sthe address of the endpoint against which to run the check.
	Address string `json:"address,omitempty" yaml:"address,omitempty"`
	// PRotocol is the kind of check to perform.
	Protocol Protocol `json:"protocol" yaml:"protocol"`
	// Result is the result of the check (possibly including the error).
	Result Result `json:"result" yaml:"result"`
}

// ToJSON converts the Check to its JSON pretty representation.
func (c *Check) ToJSON() string {
	data, _ := json.MarshalIndent(c, "  ", "")
	return string(data)
}

// ToYAML converts the Check to its YAML representation.
func (c *Check) ToYAML() string {
	data, _ := yaml.Marshal(c)
	return string(data)
}

// Do performs the actual check.
func (c *Check) Do() error {
	var protocol string
	switch c.Protocol {
	case TCP, UDP:
		var dialer net.Dialer
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.Timeout))
		defer cancel()
		conn, err := dialer.DialContext(ctx, c.Protocol.String(), c.Address)
		if err != nil {
			slog.Error("error dialling", "address", c.Address, "protocol", c.Protocol.String(), "error", err)
			return fmt.Errorf("error dialling %s on protocol %s: %w", c.Address, c.Protocol.String(), err)
		}
		defer conn.Close()
		slog.Info("successfully tested connection", "address", c.Address, "protocol", c.Protocol.String())
	case DTLS:
		protocol = "udp"
		// the rest of the logic is identical to the TLS case, so...
		fallthrough
	case TLS:
		if protocol != "udp" {
			protocol = "tcp"
		}
		dialer := &net.Dialer{
			Timeout: time.Duration(c.Timeout),
		}
		conn, err := tls.DialWithDialer(dialer, protocol, c.Address, nil)
		if err != nil {
			slog.Error("error dialling", "address", c.Address, "protocol", c.Protocol.String(), "error", err)
			return fmt.Errorf("error dialling %s on protocol %s: %w", c.Address, c.Protocol.String(), err)
		}
		defer conn.Close()
		err = conn.VerifyHostname(strings.Split(c.Address, ":")[0])
		if err != nil {
			slog.Error("hostname does not match certificate", "hostname", strings.Split(c.Address, ":")[0], "error", err)
			return fmt.Errorf("hostname mismatch in certificate from host %s on protocol %s: %w", c.Address, c.Protocol.String(), err)
		}
		expiry := conn.ConnectionState().PeerCertificates[0].NotAfter
		issuer := conn.ConnectionState().PeerCertificates[0].Issuer
		if time.Now().After(expiry) {
			slog.Error("certificate has expired", "expiry", expiry.Format(time.RFC3339))
			return fmt.Errorf("certificate from host %s on protocol %s expired on %s", c.Address, c.Protocol.String(), expiry.Format(time.RFC3339))
		}
		slog.Info("successfully tested connection", "address", c.Address, "protocol", c.Protocol.String(), "certificate issuer", issuer, "certificate expiry", expiry.Format(time.RFC3339))
	case ICMP:
		pinger, err := probing.NewPinger(c.Address)
		if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
			// on linux, package post install must run:
			// setcap cap_net_raw=+ep /path/to/your/netcheck
			// for unprivileged ping to work
			pinger.SetPrivileged(true)
		}

		if err != nil {
			slog.Error("error creating ICMP client", "address", c.Address, "protocol", c.Protocol.String(), "error", err)
			return fmt.Errorf("expired creating ICMP client to %s: %w", c.Address, err)
		}
		// TODO: take these parameters from configuration/bundle/CLI
		pinger.Timeout = time.Duration(c.Timeout)
		pinger.Count = *Default.Ping.Count
		pinger.Interval = time.Duration(*Default.Ping.Interval)
		pinger.Size = *Default.Ping.Size

		pinger.OnRecv = func(pkt *probing.Packet) {
			slog.Debug("received ping response", "bytes", pkt.Nbytes, "endpoint", pkt.IPAddr, "sequence", pkt.Seq, "rtt", pkt.Rtt, "ttl", pkt.TTL)
		}

		pinger.OnDuplicateRecv = func(pkt *probing.Packet) {
			slog.Debug("received duplicate ping response", "bytes", pkt.Nbytes, "endpoint", pkt.IPAddr, "sequence", pkt.Seq, "rtt", pkt.Rtt, "ttl", pkt.TTL)
		}

		pinger.OnFinish = func(stats *probing.Statistics) {
			slog.Debug("ping statistics", "destination", stats.Addr, "transmitted", stats.PacketsSent, "received", stats.PacketsRecv, "loss_percent", stats.PacketLoss, "roundtrip_min", stats.MinRtt, "roundtrip_avg", stats.AvgRtt, "roundtrip_max", stats.MaxRtt, "roundtrip_stddev", stats.StdDevRtt)
		}

		err = pinger.Run()
		if err != nil {
			slog.Error("error running ping", "endpoint", c.Address, "protocol", c.Protocol.String(), "error", err)
			return fmt.Errorf("error running ping against %s: %w", c.Address, err)
		}
		slog.Info("successfully tested connection", "address", c.Address, "protocol", c.Protocol.String())
	case SSH:
		config := &ssh.ClientConfig{
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         time.Duration(c.Timeout),
		}
		client, err := ssh.Dial("tcp", c.Address, config)
		if err != nil && !strings.Contains(err.Error(), "ssh: unable to authenticate") {
			slog.Error("error running ssh session", "address", c.Address, "protocol", c.Protocol.String(), "error", err, "type", fmt.Sprintf("%T", errors.Unwrap(err)))
			return fmt.Errorf("error opening SSH session to %s: %w", c.Address, err)
		}
		if client != nil {
			defer client.Close()
		}
	}
	return nil
}
