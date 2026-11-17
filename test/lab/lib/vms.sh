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

# route_unit VM DEST VIA: a systemd unit that keeps a route through the router.
route_unit() {
	local vm="$1" dest="$2" via="$3"
	vm_run "$vm" sh -c "cat >/etc/systemd/system/lab-route.service" <<UNIT
[Unit]
Description=Route to the lab's other network through nc-fw
After=network-online.target
Wants=network-online.target
[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/sbin/ip route replace $dest via $via
[Install]
WantedBy=multi-user.target
UNIT
	vm_run "$vm" systemctl enable --now lab-route.service >/dev/null
}

vms_up() {
	section "VMs"
	local fw_ip srv_ip cli_ip vm
	fw_ip="$(vm_ip nc-fw client)"
	srv_ip="$(vm_ip nc-servers servers)"
	cli_ip="$(vm_ip nc-client client)"
	vm_create nc-fw "$LAB_FW_CPUS" "$LAB_FW_MEMORY" "$LAB_NET_CLIENT" "$fw_ip" "$(fw_cloud_init)"
	if [[ -z "$(lxd config device get nc-fw eth1 network 2>/dev/null)" ]]; then
		lxd config device add nc-fw eth1 nic network="$LAB_NET_SERVERS" name=eth1 ipv4.address="$(vm_ip nc-fw servers)" >/dev/null
	fi
	vm_create nc-servers "$LAB_SERVERS_CPUS" "$LAB_SERVERS_MEMORY" "$LAB_NET_SERVERS" "$srv_ip" "$(servers_cloud_init)"
	vm_create nc-client "$LAB_CLIENT_CPUS" "$LAB_CLIENT_MEMORY" "$LAB_NET_CLIENT" "$cli_ip" "$(client_cloud_init)"
	for vm in "${VMS[@]}"; do
		vm_ready "$vm"
		info "$vm ready"
	done
	# the routes through the router, both ways
	route_unit nc-client "$LAB_SUBNET_SERVERS.0/24" "$fw_ip"
	route_unit nc-servers "$LAB_SUBNET_CLIENT.0/24" "$(vm_ip nc-fw servers)"
	# the client has no sudo: the package must not depend on it
	vm_run nc-client sh -c 'if dpkg -s sudo >/dev/null 2>&1; then SUDO_FORCE_REMOVE=yes DEBIAN_FRONTEND=noninteractive apt-get purge -y sudo >/dev/null 2>&1; fi; ! command -v sudo >/dev/null' ||
		die "nc-client still has sudo"
	info "nc-client has no sudo"
}

vms_status() {
	section "Status"
	lxd list -c ns4 2>/dev/null || info "the lab does not exist"
}
