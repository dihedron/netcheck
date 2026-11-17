# shellcheck shell=bash
# Logging, settings, state and the LXD helpers of the lab.

if [[ -t 2 ]]; then
	C_RED=$'\033[31m' C_GREEN=$'\033[32m' C_YELLOW=$'\033[33m' C_OFF=$'\033[0m'
else
	C_RED='' C_GREEN='' C_YELLOW='' C_OFF=''
fi
section() { printf '\n== %s\n' "$*" >&2; }
info() { printf '   %s\n' "$*" >&2; }
warn() { printf '%swarning:%s %s\n' "$C_YELLOW" "$C_OFF" "$*" >&2; }
die() {
	printf '%serror:%s %s\n' "$C_RED" "$C_OFF" "$*" >&2
	exit 1
}

# load_settings: lab.env, then lab.local.env, then the environment (which wins).
load_settings() {
	local name saved=()
	for name in $(compgen -A variable LAB_); do
		saved+=("$name=${!name}")
	done
	# shellcheck source=../lab.env
	source "$LAB_DIR/lab.env"
	# shellcheck source=/dev/null
	[[ -f "$LAB_DIR/lab.local.env" ]] && source "$LAB_DIR/lab.local.env"
	local pair
	for pair in "${saved[@]}"; do
		printf -v "${pair%%=*}" '%s' "${pair#*=}"
	done
	LAB_STATE_DIR="$REPO_DIR/build/lab"
	mkdir -p "$LAB_STATE_DIR"
}

# lxd ARGS: lxc in the lab's project.
lxd() { lxc --project "$LAB_PROJECT" "$@"; }
# lxd_once SECONDS ARGS: lxc in the lab's project with an empty stdin and a timeout.
lxd_once() {
	local seconds="$1"
	shift
	timeout "$seconds" lxc --project "$LAB_PROJECT" "$@" </dev/null
}

readonly VMS=(nc-fw nc-servers nc-client)

# vm_ip VM NETWORK: the address of the VM on the client's or the servers' network.
vm_ip() {
	case "$1/$2" in
	nc-client/client) echo "$LAB_SUBNET_CLIENT.$LAB_HOST_CLIENT" ;;
	nc-fw/client) echo "$LAB_SUBNET_CLIENT.$LAB_HOST_FW" ;;
	nc-fw/servers) echo "$LAB_SUBNET_SERVERS.$LAB_HOST_FW" ;;
	nc-servers/servers) echo "$LAB_SUBNET_SERVERS.$LAB_HOST_SERVERS" ;;
	*) die "vm_ip: $1 has no address on $2" ;;
	esac
}

vm_exists() { lxd info "$1" >/dev/null 2>&1; }
vm_running() { [[ "$(lxd list "$1" -c s -f csv 2>/dev/null)" == RUNNING ]]; }

# vm_run VM COMMAND...: runs a command in the VM as root.
vm_run() {
	local vm="$1"
	shift
	lxd exec "$vm" -- "$@"
}

# vm_wait VM: waits for the agent and for cloud-init.
vm_wait() {
	local i out rc=0
	for i in $(seq 1 120); do
		lxd exec "$1" -- true >/dev/null 2>&1 && break
		sleep 2
	done
	lxd exec "$1" -- true >/dev/null 2>&1 || die "$1: the LXD agent does not answer"
	out="$(lxd exec "$1" -- cloud-init status --wait 2>&1)" || rc=$?
	((rc == 0)) || die "$1: cloud-init failed ($rc): $out"
}
