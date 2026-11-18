# shellcheck shell=bash
# The three VMs: their cloud-init and their network wiring. The client reaches
# the servers' network only through nc-fw, whose second leg is on that network.

# base_cloud_init PACKAGES...: the user data every VM gets.
base_cloud_init() {
	local pkg
	echo '#cloud-config'
	echo 'package_update: true'
	echo 'packages:'
	for pkg in "$@"; do echo "  - $pkg"; done
}

fw_cloud_init() {
	base_cloud_init nftables
	cat <<'YAML'
write_files:
  - path: /etc/sysctl.d/90-lab-forward.conf
    content: |
      net.ipv4.ip_forward=1
      net.ipv6.conf.all.forwarding=1
runcmd:
  - [ sysctl, --system ]
YAML
}

servers_cloud_init() {
	base_cloud_init nginx openssh-server socat openssl redis-server dnsmasq krb5-kdc krb5-admin-server krb5-user ncat nftables jq
}

client_cloud_init() {
	base_cloud_init libcap2-bin iputils-ping iproute2 jq
	cat <<'YAML'
users:
  - default
  - name: probe
    shell: /bin/bash
    lock_passwd: true
YAML
}

# lab_routes VM DEST VIA DEST6 VIA6: keeps a route through the router in the
# VM's netplan, so that a DHCP renewal (networkd) never drops it: a route added
# by hand with ip is dropped on renewal, which cut the servers' way back to the
# client in the middle of a soak.
lab_routes() {
	local vm="$1" dest="$2" via="$3" dest6="$4" via6="$5"
	vm_run "$vm" sh -c "systemctl disable --now lab-route.service >/dev/null 2>&1; rm -f /etc/systemd/system/lab-route.service; cat >/etc/netplan/61-lab-routes.yaml; chmod 600 /etc/netplan/61-lab-routes.yaml" <<NETPLAN
network:
  version: 2
  ethernets:
    enp5s0:
      routes:
        - {to: $dest, via: $via}
        - {to: $dest6, via: "$via6"}
NETPLAN
	vm_run "$vm" netplan apply 2>/dev/null
}

vms_up() {
	section "VMs"
	local fw_ip srv_ip cli_ip vm
	fw_ip="$(vm_ip nc-fw client)"
	srv_ip="$(vm_ip nc-servers servers)"
	cli_ip="$(vm_ip nc-client client)"
	vm_create nc-fw "$LAB_FW_CPUS" "$LAB_FW_MEMORY" "$LAB_NET_CLIENT" "$fw_ip" "$(vm_ip6 nc-fw client)" "$(fw_cloud_init)"
	if [[ -z "$(lxd config device get nc-fw eth1 network 2>/dev/null)" ]]; then
		lxd config device add nc-fw eth1 nic network="$LAB_NET_SERVERS" name=eth1 ipv4.address="$(vm_ip nc-fw servers)" ipv6.address="$(vm_ip6 nc-fw servers)" >/dev/null
		lxd restart nc-fw >/dev/null
	fi
	vm_create nc-servers "$LAB_SERVERS_CPUS" "$LAB_SERVERS_MEMORY" "$LAB_NET_SERVERS" "$srv_ip" "$(vm_ip6 nc-servers servers)" "$(servers_cloud_init)"
	vm_create nc-client "$LAB_CLIENT_CPUS" "$LAB_CLIENT_MEMORY" "$LAB_NET_CLIENT" "$cli_ip" "$(vm_ip6 nc-client client)" "$(client_cloud_init)"
	for vm in "${VMS[@]}"; do
		vm_ready "$vm"
		info "$vm ready"
	done
	# the router's second leg: cloud-init configures the first NIC only
	vm_run nc-fw sh -c "cat >/etc/netplan/60-lab.yaml; chmod 600 /etc/netplan/60-lab.yaml" <<NETPLAN
network:
  version: 2
  ethernets:
    enp6s0:
      dhcp4: false
      dhcp6: false
      accept-ra: false
      addresses: [$(vm_ip nc-fw servers)/24, $(vm_ip6 nc-fw servers)/64]
NETPLAN
	vm_run nc-fw netplan apply 2>/dev/null
	# the hosts must follow their static routes: no ICMP redirects either way (the
	# host's bridge would otherwise short-cut the router)
	vm_run nc-fw sh -c "printf 'net.ipv4.conf.all.send_redirects=0\nnet.ipv4.conf.default.send_redirects=0\n' >/etc/sysctl.d/91-lab-redirects.conf && sysctl --system >/dev/null"
	for vm in nc-client nc-servers; do
		vm_run "$vm" sh -c "printf 'net.ipv4.conf.all.accept_redirects=0\nnet.ipv4.conf.default.accept_redirects=0\nnet.ipv4.conf.all.secure_redirects=0\nnet.ipv4.conf.default.secure_redirects=0\n' >/etc/sysctl.d/91-lab-redirects.conf && sysctl --system >/dev/null && ip route flush cache"
	done
	# the routes through the router, both ways
	lab_routes nc-client "$LAB_SUBNET_SERVERS.0/24" "$fw_ip" fd42:77:77:2::/64 "$(vm_ip6 nc-fw client)"
	lab_routes nc-servers "$LAB_SUBNET_CLIENT.0/24" "$(vm_ip nc-fw servers)" fd42:77:77:1::/64 "$(vm_ip6 nc-fw servers)"
	# the client has no sudo: the package must not depend on it
	vm_run nc-client sh -c 'if dpkg -s sudo >/dev/null 2>&1; then SUDO_FORCE_REMOVE=yes DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=600 purge -y sudo >/dev/null 2>&1; fi; ! command -v sudo >/dev/null' ||
		die "nc-client still has sudo"
	info "nc-client has no sudo"
}

vms_status() {
	section "Status"
	lxd list -c ns4 2>/dev/null || info "the lab does not exist"
}
