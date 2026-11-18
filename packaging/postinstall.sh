#!/bin/sh
# Lets netcheck send ICMP echo requests on raw sockets without being root: the
# binary gets the capability cap_net_raw. The package manager runs this script
# as root, so no sudo is involved (a minimal system has none). A failure must
# not fail the installation, but it must be heard.

BINARY=/usr/bin/netcheck

[ -f "$BINARY" ] || exit 0

if ! command -v setcap >/dev/null 2>&1; then
	echo "netcheck: setcap is not installed, so ICMP checks need root or ping.privileged: false." >&2
	echo "netcheck: install libcap2-bin (libcap on Red Hat, libcap-utils on Alpine) and run: setcap cap_net_raw=+ep $BINARY" >&2
	exit 0
fi

if ERROR="$(setcap cap_net_raw=+ep "$BINARY" 2>&1)"; then
	echo "netcheck: ICMP checks can use raw sockets (cap_net_raw set on $BINARY)"
else
	echo "netcheck: could not set cap_net_raw on $BINARY: $ERROR" >&2
	echo "netcheck: ICMP checks need root or ping.privileged: false" >&2
fi
exit 0
