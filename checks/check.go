package checks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"gopkg.in/yaml.v3"
)

// Check represents a single check to perform.
type Check struct {
	id       int
	Name     string   `json:"name,omitempty" yaml:"name,omitempty"`
	Timeout  Timeout  `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Retries  int      `json:"retries,omitempty" yaml:"retries,omitempty"`
	Wait     Timeout  `json:"wait,omitempty" yaml:"wait,omitempty"`
	Address  string   `json:"address,omitempty" yaml:"address,omitempty"`
	Protocol Protocol `json:"protocol" yaml:"protocol"`
	Result   Result   `json:"result" yaml:"result"`
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
			// t, _ := time.Parse("2006-Jan-02", "2014-Feb-23")
			// if t.Before(expiry) {
			slog.Error("certificate has expired", "expiry", expiry.Format(time.RFC3339))
			return fmt.Errorf("certificate from host %s on protocol %s expired on %s", c.Address, c.Protocol.String(), expiry.Format(time.RFC3339))
		}
		slog.Info("successfully tested connection", "address", c.Address, "protocol", c.Protocol.String(), "certificate issuer", issuer, "certificate expiry", expiry.Format(time.RFC3339))
	case ICMP:
		pinger, err := probing.NewPinger(c.Address)
		if err != nil {
			slog.Error("error creating ICMP client", "address", c.Address, "protocol", c.Protocol.String(), "error", err)
			return fmt.Errorf("expired creating ICMP client to %s: %w", c.Address, err)
		}
		// TODO: take these parameters from configuration/bundle/CLI
		pinger.Timeout = time.Duration(c.Timeout)
		pinger.Count = 10
		pinger.Interval = 100 * time.Microsecond
		pinger.Size = 64

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
	}
	return nil
}
