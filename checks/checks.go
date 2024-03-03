package checks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/dihedron/netcheck/fetch"
	"github.com/dihedron/netcheck/format"
	"github.com/dihedron/netcheck/logging"
	capi "github.com/hashicorp/consul/api"
	"github.com/pelletier/go-toml/v2"
	probing "github.com/prometheus-community/pro-bing"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

const (
	DefaultTimeout     = Timeout(20 * time.Second)
	DefaultRetries     = 3
	DefaultWait        = Timeout(1 * time.Second)
	DefaultParallelism = 10
)

type Bundle struct {
	ID          string  `json:"id,omitempty" yaml:"id,omitempty" toml:"id"`
	Description string  `json:"description,omitempty" yaml:"description,omitempty" toml:"description"`
	Timeout     Timeout `json:"timeout,omitempty" yaml:"timeout,omitempty" toml:"timeout"`
	Retries     int     `json:"retries,omitempty" yaml:"retries,omitempty" toml:"retries"`
	Wait        Timeout `json:"wait,omitempty" yaml:"wait,omitempty" toml:"wait"`
	Parallelism int     `json:"parallelism,omitempty" yaml:"parallelism,omitempty" toml:"parallelism"`
	Checks      []Check `json:"checks,omitempty" yaml:"checks,omitempty" toml:"checks"`
}

type Result struct {
	Protocol Protocol `json:"protocol" yaml:"protocol" toml:"protocol"`
	Endpoint string   `json:"endpoint,omitempty" yaml:"endpoint,omitempty" toml:"endpoint"`
	Error    error    `json:"error,omitempty" yaml:"error,omitempty" toml:"error"`
}

func New(path string) (*Bundle, error) {

	var (
		data []byte
		err  error
		f    format.Format
	)

	if strings.HasPrefix("http://", path) || strings.HasPrefix("https://", path) {
		// retrieve from URL
		data, f, err = fetch.FromHTTP(path)
		if err != nil {
			slog.Error("error fetching bundle file from HTTP(s) source", "path", path, "error", err)
			return nil, err
		}

	} else if strings.HasPrefix("redis://", path) || strings.HasPrefix("rediss://", path) {

		u, err := url.Parse(path)
		if err != nil {
			slog.Error("error parsing Redis URL", "url", path, "error", err)
			return nil, err
		}

		slog.Debug("parsed URL", "value", logging.ToJSON(u))

		// the URL is like redis://<user>:<password>@<host>:<port>/<db_number>/<path/to/key>
		// see regex101.com to check how I came up with the following regular expression:
		pattern := regexp.MustCompile(`redis[s]{0,1}://(?:(?:(?:(.*):(.*)))@)*((?:(?:[a-zA-Z]|[a-zA-Z][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*(?:[A-Za-z]|[A-Za-z][A-Za-z0-9\-]*[A-Za-z0-9]))(?::(\d+))*/(?:(\d*)/)*(.*)`)
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
			f = format.YAML
		} else if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			f = format.JSON
		} else {
			f = format.TOML
		}
		data = []byte(value)
	} else if strings.HasPrefix(path, "consulkv://") { // consulkv://username:password@hostname:port/path/to/key or consul://:token@hostname:port/path/to/key

		u, err := url.Parse(path)
		if err != nil {
			slog.Error("error parsing Consul URL", "url", path, "error", err)
			return nil, err
		}

		password, _ := u.User.Password()
		slog.Debug("Consul URL parsed", "parsed", logging.ToJSON(u), "username", u.User.Username(), "password", password)

		cfg := capi.DefaultConfig()
		cfg.Address = u.Host
		if len(u.User.Username()) > 0 {
			cfg.HttpAuth = &capi.HttpBasicAuth{
				Username: u.User.Username(),
				Password: password,
			}
		} else {
			cfg.Token = password
		}
		// cfg.Address := u.Host
		// Get a new client
		client, err := capi.NewClient(cfg)
		if err != nil {
			panic(err)
		}

		// Get a handle to the KV API
		kv := client.KV()

		// PUT a new KV pair
		p := &capi.KVPair{Key: "REDIS_MAXCLIENTS", Value: []byte("1000")}
		_, err = kv.Put(p, nil)
		if err != nil {
			panic(err)
		}

	} else {
		// read from file on disk
		data, f, err = fetch.FromFile(path)
		if err != nil {
			slog.Error("error fetching bundle file from local source", "path", path, "error", err)
			return nil, err
		}
	}

	bundle := &Bundle{
		Timeout:     DefaultTimeout,
		Retries:     DefaultRetries,
		Wait:        DefaultWait,
		Parallelism: DefaultParallelism,
	}

	switch f {
	case format.YAML:
		err := yaml.Unmarshal(data, bundle)
		if err != nil {
			slog.Error("error parsing checks package", "format", "yaml", "error", err)
			os.Exit(1)
		}
	case format.JSON:
		err := json.Unmarshal(data, bundle)
		if err != nil {
			slog.Error("error parsing checks package", "format", "json", "error", err)
			os.Exit(1)
		}
	case format.TOML:
		err := toml.Unmarshal(data, bundle)
		if err != nil {
			slog.Error("error parsing checks package", "format", "toml", "error", err)
			os.Exit(1)
		}
	}
	// safety checks
	if bundle.Parallelism < 0 {
		bundle.Parallelism = DefaultParallelism
	}
	if bundle.Timeout <= 0 {
		bundle.Timeout = DefaultTimeout
	}
	if bundle.Wait <= 0 {
		bundle.Wait = DefaultWait
	}
	if bundle.Retries < 1 {
		bundle.Retries = DefaultRetries
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
		if check.Timeout <= 0 {
			check.Timeout = b.Timeout
		}
		if check.Retries < 1 {
			check.Retries = b.Retries
		}
		if check.Wait <= 0 {
			check.Wait = b.Wait
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
