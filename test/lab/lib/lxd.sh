# shellcheck shell=bash
# The lab's LXD objects and VMs. Every object carries user.netcheck-lab=yes, so
# that the lab never takes over or removes what it did not create.

lxd_up() {
	section "LXD"
	if ! lxc storage show "$LAB_POOL" >/dev/null 2>&1; then
		lxd_once 300 storage create "$LAB_POOL" "$LAB_POOL_DRIVER" user.netcheck-lab=yes >/dev/null || die "cannot create the storage pool $LAB_POOL"
		info "storage pool $LAB_POOL created"
	fi
	if ! lxc network show "$LAB_NET_CLIENT" >/dev/null 2>&1; then
		lxc network create "$LAB_NET_CLIENT" ipv4.address="$LAB_SUBNET_CLIENT.1/24" ipv4.nat=true \
			ipv6.address=fd42:77:77:1::1/64 ipv6.nat=true dns.domain=nclab user.netcheck-lab=yes >/dev/null
		info "network $LAB_NET_CLIENT created ($LAB_SUBNET_CLIENT.0/24)"
	fi
	if ! lxc network show "$LAB_NET_SERVERS" >/dev/null 2>&1; then
		lxc network create "$LAB_NET_SERVERS" ipv4.address="$LAB_SUBNET_SERVERS.1/24" ipv4.nat=true \
			ipv6.address=fd42:77:77:2::1/64 ipv6.nat=true dns.domain=ncsrv user.netcheck-lab=yes >/dev/null
		info "network $LAB_NET_SERVERS created ($LAB_SUBNET_SERVERS.0/24)"
	fi
	if ! lxc project show "$LAB_PROJECT" >/dev/null 2>&1; then
		lxc project create "$LAB_PROJECT" -c features.images=false -c features.profiles=true -c user.netcheck-lab=yes >/dev/null
		lxd profile device add default root disk path=/ pool="$LAB_POOL" >/dev/null
		lxd profile device add default eth0 nic network="$LAB_NET_CLIENT" name=eth0 >/dev/null
		info "project $LAB_PROJECT created"
	fi
}

# lxd_down: removes the lab's objects, the VMs first.
lxd_down() {
	local vm
	for vm in "${VMS[@]}"; do
		if lxc project show "$LAB_PROJECT" >/dev/null 2>&1 && vm_exists "$vm"; then
			lxd delete --force "$vm" >/dev/null && info "$vm deleted"
		fi
	done
	if lxc project show "$LAB_PROJECT" >/dev/null 2>&1; then
		[[ "$(lxc project get "$LAB_PROJECT" user.netcheck-lab)" == yes ]] || die "project $LAB_PROJECT is not the lab's: refusing to delete it"
		lxd profile device remove default root >/dev/null 2>&1 || true
		lxd profile device remove default eth0 >/dev/null 2>&1 || true
		lxc project delete "$LAB_PROJECT" && info "project $LAB_PROJECT deleted"
	fi
	local net
	for net in "$LAB_NET_CLIENT" "$LAB_NET_SERVERS"; do
		if [[ "$(lxc network get "$net" user.netcheck-lab 2>/dev/null)" == yes ]]; then
			lxc network delete "$net" && info "network $net deleted"
		fi
	done
	if [[ "$(lxc storage get "$LAB_POOL" user.netcheck-lab 2>/dev/null)" == yes ]]; then
		lxc storage delete "$LAB_POOL" && info "storage pool $LAB_POOL deleted"
	fi
}

# vm_create VM CPUS MEMORY NETWORK HOSTIP CLOUDINIT: creates and starts a VM
# on NETWORK with a fixed address, unless it exists.
vm_create() {
	local vm="$1" cpus="$2" memory="$3" network="$4" ip="$5" userdata="$6"
	if vm_exists "$vm"; then
		vm_running "$vm" || lxd start "$vm" >/dev/null
		return
	fi
	info "creating $vm"
	lxd_once 600 init "$LAB_IMAGE" "$vm" --vm \
		-c limits.cpu="$cpus" -c limits.memory="$memory" -c user.netcheck-lab=yes \
		-c cloud-init.user-data="$userdata" \
		-d eth0,network="$network" -d eth0,ipv4.address="$ip" >/dev/null || die "cannot create $vm"
}

# vm_ready VM: starts the VM if needed and waits for it.
vm_ready() {
	vm_running "$1" || lxd start "$1" >/dev/null
	vm_wait "$1"
}
