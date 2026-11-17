package netcheck

import (
	"context"
	"fmt"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

// icmpRunner passes when at least one echo reply comes back. Privilege is the
// caller's choice (Defaults.Ping.Privileged): raw sockets need cap_net_raw or
// root, unprivileged ones need net.ipv4.ping_group_range to include the user.
type icmpRunner struct{ cfg *config }

func (r icmpRunner) Run(ctx context.Context, c Check, d Defaults) (Detail, error) {
	target, err := r.cfg.pingTarget(ctx, c.Address)
	if err != nil {
		return Detail{}, err
	}
	pinger, err := probing.NewPinger(target)
	if err != nil {
		return Detail{}, fmt.Errorf("creating the ICMP client for %s: %w", redact(c.Address), err)
	}
	pinger.SetPrivileged(d.Ping.Privileged)
	pinger.Count = max(1, d.Ping.Count)
	pinger.Interval = time.Duration(d.Ping.Interval)
	pinger.Size = d.Ping.Size
	if dl, ok := ctx.Deadline(); ok {
		pinger.Timeout = time.Until(dl)
	}
	err = pinger.RunWithContext(ctx)
	stats := pinger.Statistics()
	detail := Detail{Remote: stats.IPAddr.String(), PacketsSent: stats.PacketsSent, PacketsRecv: stats.PacketsRecv}
	if err != nil {
		return detail, fmt.Errorf("pinging %s (is ICMP permitted for this user? see Defaults.Ping.Privileged): %w", redact(c.Address), err)
	}
	if stats.PacketsRecv == 0 {
		return detail, fmt.Errorf("no reply from %s: %d packets sent, 100%% loss", redact(c.Address), stats.PacketsSent)
	}
	return detail, nil
}
