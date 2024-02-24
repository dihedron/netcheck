package checks

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	probing "github.com/prometheus-community/pro-bing"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

const (
	DefaultTimeout     = Timeout(20 * time.Second)
	DefaultParallelism = 10
)

type Bundle struct {
	ID          string  `json:"id,omitempty" yaml:"id,omitempty" toml:"id"`
	Description string  `json:"description,omitempty" yaml:"description,omitempty" toml:"description"`
	Timeout     Timeout `json:"timeout,omitempty" yaml:"timeout,omitempty" toml:"timeout"`
	Parallelism int     `json:"parallelism,omitempty" yaml:"parallelism,omitempty" toml:"parallelism"`
	Checks      []Check `json:"checks,omitempty" yaml:"checks,omitempty" toml:"checks"`
}

type Result struct {
	Protocol Protocol `json:"protocol" yaml:"protocol" toml:"protocol"`
	Endpoint string   `json:"endpoint,omitempty" yaml:"endpoint,omitempty" toml:"endpoint"`
	Error    error    `json:"error,omitempty" yaml:"error,omitempty" toml:"error"`
}

type Format int8

const (
	YAML Format = iota
	JSON
	TOML
)

func New(path string) (*Bundle, error) {

	var (
		data   []byte
		err    error
		format Format
	)

	if strings.HasPrefix("http://", path) || strings.HasPrefix("https://", path) {
		// retrieve from URL
		resp, err := http.Get(path)
		if err != nil {
			slog.Error("error downloading package from URL", "url", path, "error", err)
			return nil, err
		}
		defer resp.Body.Close()

		var buffer bytes.Buffer
		_, err = io.Copy(&buffer, resp.Body)
		if err != nil {
			slog.Error("error reading package body from URL", "url", path, "error", err)
			return nil, err
		}

		data = buffer.Bytes()

		switch resp.Header.Get("Content-Type") {
		case "application/json":
			format = JSON
		case "application/x-yaml", "text/yaml":
			format = YAML
		case "application/toml":
			format = TOML
		}
	} else if strings.HasPrefix("redis://", path) {

		// the URL is like redis://<user>:<password>@<host>:<port>/<db_number>/<path/to/key>
		// see regex101.com to check how I came up with the following regular expression:
		pattern := regexp.MustCompile(`redis://(?:(?:(?:(.*):(.*)))@)*((?:(?:[a-zA-Z]|[a-zA-Z][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*(?:[A-Za-z]|[A-Za-z][A-Za-z0-9\-]*[A-Za-z0-9]))(?::(\d+))*/(?:(\d*)/)*(.*)`)
		matches := pattern.FindAllStringSubmatch(path, -1)
		var key string
		if len(matches) > 0 {
			username := matches[0][0]
			password := matches[0][1]
			hostname := matches[0][2]
			port := matches[0][3]
			db := matches[0][4]
			key = matches[0][5]
			slog.Debug("address parsed", "username", username, "password", password, "hostname", hostname, "port", port, "db", db, "key", key)
		}

		opts, err := redis.ParseURL(path)
		if err != nil {
			slog.Error("error reading package from redis", "url", path, "error", err)
			return nil, err
		}

		client := redis.NewClient(opts)
		value, err := client.Get(context.Background(), key).Result()
		if err != nil {
			slog.Error("error getting key from Redis", "key", key, "error", err)
			return nil, err
		}
		slog.Debug("data read from Redis", "key", key, "value", value)
		trimmed := strings.TrimLeft(value, "\n\r\t")
		if strings.HasPrefix(trimmed, "---") {
			format = YAML
		} else if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			format = JSON
		} else {
			format = TOML
		}
		data = []byte(value)
	} else {
		// read from file on disk
		data, err = os.ReadFile(path)
		if err != nil {
			slog.Error("error reading package from file", "path", path, "error", err)
			return nil, err
		}

		switch strings.ToLower(filepath.Ext(path)) {
		case ".yaml", ".yml":
			format = YAML
		case ".json":
			format = JSON
		case ".toml":
			format = TOML
		}
	}

	bundle := &Bundle{
		Timeout:     DefaultTimeout,
		Parallelism: DefaultParallelism,
	}

	switch format {
	case YAML:
		err := yaml.Unmarshal(data, bundle)
		if err != nil {
			slog.Error("error parsing checks package", "format", "yaml", "error", err)
			os.Exit(1)
		}
	case JSON:
		err := json.Unmarshal(data, bundle)
		if err != nil {
			slog.Error("error parsing checks package", "format", "json", "error", err)
			os.Exit(1)
		}
	case TOML:
		err := toml.Unmarshal(data, bundle)
		if err != nil {
			slog.Error("error parsing checks package", "format", "toml", "error", err)
			os.Exit(1)
		}
	}
	return bundle, nil
}

func (b *Bundle) ToJSON() string {
	data, _ := json.MarshalIndent(b, "  ", "")
	return string(data)
}

func (b *Bundle) ToYAML() string {
	data, _ := yaml.Marshal(b)
	return string(data)
}

func (b *Bundle) ToTOML() string {
	data, _ := toml.Marshal(b)
	return string(data)
}

func (b *Bundle) Check() []Result {
	checks := make(chan Check, len(b.Checks))
	results := make(chan Result, len(b.Checks))

	// launch the thread pool
	for id := 1; id <= b.Parallelism; id++ {
		go worker(id, checks, results)
	}

	// submit the checks
	for _, check := range b.Checks {
		if check.Timeout == 0 {
			check.Timeout = b.Timeout
		}
		checks <- check
	}
	close(checks)

	// collect the results
	array := []Result{}
	for range len(b.Checks) {
		result := <-results
		array = append(array, result)
	}

	return array
}

type Check struct {
	Name     string   `json:"name,omitempty" yaml:"name,omitempty" toml:"name"`
	Timeout  Timeout  `json:"timeout,omitempty" yaml:"timeout,omitempty" toml:"timeout"`
	Retries  int      `json:"retries,omitempty" yaml:"retries,omitempty" toml:"retries"`
	Wait     Timeout  `json:"wait,omitempty" yaml:"wait,omitempty" toml:"wait"`
	Address  string   `json:"address,omitempty" yaml:"address,omitempty" toml:"address"`
	Protocol Protocol `json:"protocol,omitempty" yaml:"protocol,omitempty" toml:"protocol"`
}

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

func worker(id int, check <-chan Check, results chan<- Result) {

	for check := range check {
		var err error

		retries := check.Retries
		if retries <= 0 {
			retries = 1
		}
	attempts:
		for i := range retries {
			err = check.Do()
			if err != nil {
				slog.Error("error trying check", "attempt", i+1)
				time.Sleep(time.Duration(check.Wait))
			} else {
				break attempts
			}
		}

		result := Result{
			Endpoint: check.Address,
			Protocol: check.Protocol,
			Error:    err,
		}

		results <- result
	}
}
