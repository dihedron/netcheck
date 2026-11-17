#!/usr/bin/env bash
# The netcheck lab: LXD virtual machines that run the installed package
# against real servers. See .specs/netcheck-lab.md and "lab.sh help".
set -euo pipefail

LAB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$LAB_DIR/../.." && pwd)"

# shellcheck source=lib/common.sh
source "$LAB_DIR/lib/common.sh"
# shellcheck source=lib/lxd.sh
source "$LAB_DIR/lib/lxd.sh"
# shellcheck source=lib/vms.sh
source "$LAB_DIR/lib/vms.sh"
# shellcheck source=lib/deploy.sh
source "$LAB_DIR/lib/deploy.sh"

usage() {
	cat <<'USAGE'
usage: lab.sh <command> [options]

commands:
  preflight          check that this machine can run the lab
  up                 build the lab (or complete a partial one): network, VMs, servers
  deploy             install the package under test on nc-client (dpkg -i)
  test [GO-FLAGS]    run the acceptance scenarios against the installed package
  status             show the lab's VMs
  ssh VM [CMD...]    a shell, or a command, on a VM (as root, through the LXD agent)
  down               destroy the lab (asks first; --yes does not)
  help               this text

settings: test/lab/lab.env, overridden by lab.local.env and the environment.
USAGE
}

preflight() {
	section "Preflight"
	local fails=0
	command -v lxc >/dev/null || { warn "lxc is not installed"; fails=$((fails + 1)); }
	lxc query /1.0 >/dev/null 2>&1 || { warn "cannot talk to the LXD daemon (is your user in the lxd group?)"; fails=$((fails + 1)); }
	[[ -r /dev/kvm && -w /dev/kvm ]] || { warn "/dev/kvm is not usable"; fails=$((fails + 1)); }
	command -v go >/dev/null || { warn "go is not installed"; fails=$((fails + 1)); }
	local net
	for net in "$LAB_SUBNET_CLIENT" "$LAB_SUBNET_SERVERS"; do
		if ip route | grep -q "^$net\.0/24" && ! lxc network show "$LAB_NET_CLIENT" >/dev/null 2>&1; then
			warn "the host already routes $net.0/24"
			fails=$((fails + 1))
		fi
	done
	((fails == 0)) || die "preflight: $fails check(s) failed"
	info "preflight passed"
}

confirm() {
	[[ "${ASSUME_YES:-}" == yes ]] && return 0
	local answer
	read -r -p "$1 [y/N] " answer
	[[ "$answer" == y || "$answer" == Y ]]
}

main() {
	load_settings
	local cmd="${1:-help}"
	shift || true
	case "$cmd" in
	preflight) preflight ;;
	up)
		preflight
		lxd_up
		vms_up
		;;
	deploy) deploy ;;
	test) run_tests "$@" ;;
	status) vms_status ;;
	ssh)
		local vm="${1:?usage: lab.sh ssh VM [CMD...]}"
		shift
		if (($# == 0)); then lxd exec "$vm" -- bash -l; else lxd exec "$vm" -- "$@"; fi
		;;
	down)
		confirm "Destroy the netcheck lab (VMs, networks, project, pool)?" || die "not confirmed"
		lxd_down
		;;
	help | -h | --help) usage ;;
	*)
		usage >&2
		exit 2
		;;
	esac
}

main "$@"
