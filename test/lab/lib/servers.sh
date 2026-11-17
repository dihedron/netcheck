# shellcheck shell=bash
# The real servers on nc-servers, the router's fault chain, and what the client
# needs to use them (trust, names, Kerberos).

readonly CONSUL_VERSION_DEFAULT=1.22.7
readonly CONSUL_SHA256_DEFAULT=fe25cecd8dd3552a8e5b0941cde1d79bb6004eac384aa45679dd1398f947201d

# secret NAME: a random value kept under the state directory (git-ignored),
# created on first use (LT-1).
secret() {
	local file="$LAB_STATE_DIR/secret-$1"
	[[ -f "$file" ]] || (umask 077 && openssl rand -hex 16 >"$file")
	cat "$file"
}

# vm_push VM SRC DEST: copies a file into a VM.
vm_push() { lxd file push "$2" "$1$3" >/dev/null; }

pki_up() {
	if [[ ! -f "$LAB_STATE_DIR/pki/server.pem" ]]; then
		info "generating the lab's certificates"
		(cd "$REPO_DIR" && go run ./test/lab/pki "$LAB_STATE_DIR/pki")
	fi
}

fw_up() {
	section "Router"
	vm_run nc-fw sh -c 'cat >/etc/nftables.conf' <<'NFT'
#!/usr/sbin/nft -f
flush ruleset
# the tests put rules into "faults" to drop, reject or delay what crosses the router
table inet lab {
	chain faults {
	}
	chain forward {
		type filter hook forward priority 0; policy accept;
		jump faults
	}
}
NFT
	vm_run nc-fw systemctl enable nftables >/dev/null 2>&1
	vm_run nc-fw systemctl restart nftables
	info "the fault chain is in place"
}

servers_up() {
	section "Servers"
	pki_up
	local pki="$LAB_STATE_DIR/pki" f
	vm_run nc-servers mkdir -p /etc/lab/pki /root/lab-servers
	for f in "$pki"/*.pem; do
		vm_push nc-servers "$f" /etc/lab/pki/"$(basename "$f")"
	done
	for f in "$LAB_DIR"/servers/*; do
		vm_push nc-servers "$f" /root/lab-servers/"$(basename "$f")"
	done
	info "provisioning nc-servers (packages, Consul download: a few minutes the first time)"
	vm_run nc-servers env \
		CONSUL_VERSION="${LAB_CONSUL_VERSION:-$CONSUL_VERSION_DEFAULT}" \
		CONSUL_SHA256="${LAB_CONSUL_SHA256:-$CONSUL_SHA256_DEFAULT}" \
		KRB_MASTER_PW="$(secret krb-master)" KRB_USER_PW="$(secret krb-user)" \
		bash /root/lab-servers/provision.sh | tail -3
}

client_up() {
	section "Client"
	pki_up
	local pki="$LAB_STATE_DIR/pki"
	# names, trust (the lab CA only: the system roots stay as they are) and Kerberos
	vm_run nc-client sh -c "grep -q nc-servers.lab /etc/hosts || echo '$(vm_ip nc-servers servers) nc-servers.lab' >>/etc/hosts"
	vm_run nc-client sh -c "grep -q denied.lab /etc/hosts || echo '$LAB_SUBNET_SERVERS.20 denied.lab' >>/etc/hosts"
	vm_push nc-client "$pki/lab-ca.pem" /usr/local/share/ca-certificates/lab-ca.crt
	vm_run nc-client update-ca-certificates >/dev/null
	vm_push nc-client "$LAB_DIR/servers/krb5.conf" /etc/krb5.conf
	vm_run nc-client sh -c 'DEBIAN_FRONTEND=noninteractive apt-get install -y -qq krb5-user >/dev/null'
	vm_run nc-client sh -c "printf '%s' '$(secret krb-user)' >/home/probe/.krb5pw 2>/dev/null || { mkdir -p /home/probe; printf '%s' '$(secret krb-user)' >/home/probe/.krb5pw; chown probe: /home/probe; }; chown probe: /home/probe/.krb5pw; chmod 600 /home/probe/.krb5pw"
}
