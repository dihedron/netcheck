#!/bin/bash
# Runs on nc-servers as root: turns the VM into the lab's real servers.
# Idempotent. Needs CONSUL_VERSION, CONSUL_SHA256, KRB_MASTER_PW, KRB_USER_PW.
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
SRC=/root/lab-servers

grep -q 'nc-servers.lab' /etc/hosts || echo '10.78.0.10 nc-servers.lab' >>/etc/hosts

apt-get install -y -qq apache2 libapache2-mod-auth-gssapi unzip curl >/dev/null

# --- web: nginx ---------------------------------------------------------------
mkdir -p /var/www/lab
[[ -f /var/www/lab/big.bin ]] || head -c 262144 /dev/zero | tr '\0' 'x' >/var/www/lab/big.bin
cat >/var/www/lab/bundle.yaml <<'YAML'
id: from-http
description: a bundle served by the lab's web server
checks:
  - name: web
    protocol: http
    address: nc-servers.lab/ok
YAML
cat >/var/www/lab/bundle.json <<'JSON'
{"id": "from-http-json", "checks": [{"name": "web", "address": "nc-servers.lab:80", "protocol": "tcp"}]}
JSON
[[ -f /var/www/lab/huge.bin ]] || head -c 2097152 /dev/zero | tr '\0' 'x' >/var/www/lab/huge.bin # over the 1 MiB bundle limit
mkdir -p /var/www/lab/secure
cp /var/www/lab/bundle.yaml /var/www/lab/secure/bundle.yaml
echo ok >/var/www/lab/ok
rm -f /etc/nginx/sites-enabled/default
cp "$SRC/nginx-lab.conf" /etc/nginx/conf.d/lab.conf
nginx -t
systemctl enable nginx >/dev/null 2>&1
systemctl restart nginx

# --- the small servers: socat and openssl --------------------------------------
unit() { # unit NAME DESCRIPTION EXEC...
	local name="$1" desc="$2"
	shift 2
	cat >"/etc/systemd/system/lab-$name.service" <<UNIT
[Unit]
Description=Lab: $desc
After=network-online.target
[Service]
ExecStart=$*
Restart=always
RestartSec=1
[Install]
WantedBy=multi-user.target
UNIT
	systemctl enable "lab-$name.service" >/dev/null 2>&1
	systemctl restart "lab-$name.service"
}
# 9001 accepts and closes; 9002 has nothing; 9201 accepts and never answers; 9301 speaks, but not SSH
unit tcp-open "tcp accepts" /usr/bin/socat 'TCP6-LISTEN:9001,fork,reuseaddr,ipv6-v6only=0' SYSTEM:true
unit tcp-silent "tcp accepts and says nothing" /usr/bin/socat 'TCP6-LISTEN:9201,fork,reuseaddr,ipv6-v6only=0' 'SYSTEM:sleep\ 60'
unit tcp-hello "tcp that is not ssh" /usr/bin/socat 'TCP6-LISTEN:9301,fork,reuseaddr,ipv6-v6only=0' 'SYSTEM:echo\ hello'
# 9101 listens on UDP; 9102 has nothing, so the kernel answers with ICMP port unreachable
unit udp-sink "udp listens" /usr/bin/socat -u 'UDP6-RECV:9101,reuseaddr,ipv6-v6only=0' /dev/null
# DTLS 1.2 on 4433: our own server, which serves many clients at once
chmod +x /usr/local/bin/dtlsd
unit dtls "dtls server" /usr/local/bin/dtlsd -addr :4433 -cert /etc/lab/pki/server.pem -key /etc/lab/pki/server-key.pem

# many concurrent SSH checks must not hit the default MaxStartups
printf "MaxStartups 500:30:1000\n" >/etc/ssh/sshd_config.d/90-lab.conf
systemctl restart ssh

# --- Redis (6379 plain, 6380 TLS) with a bundle -----------------------------------
cp "$SRC/redis.conf" /etc/redis/redis.conf
systemctl enable redis-server >/dev/null 2>&1
systemctl restart redis-server
for _ in $(seq 1 20); do redis-cli -p 6379 ping >/dev/null 2>&1 && break; sleep 1; done
redis-cli -p 6379 -x set checks/lab <<'YAML' >/dev/null
id: from-redis
description: a bundle kept in Redis
checks:
  - name: web
    address: nc-servers.lab:80
YAML

# --- Consul (8500 HTTP, 8501 HTTPS) --------------------------------------------------
if [[ ! -x /usr/local/bin/consul ]]; then
	curl -fsS -o /tmp/consul.zip "https://releases.hashicorp.com/consul/$CONSUL_VERSION/consul_${CONSUL_VERSION}_linux_amd64.zip"
	echo "$CONSUL_SHA256  /tmp/consul.zip" | sha256sum -c - >/dev/null
	unzip -qo /tmp/consul.zip consul -d /usr/local/bin
fi
id consul >/dev/null 2>&1 || useradd --system --home /var/lib/consul --shell /usr/sbin/nologin consul
mkdir -p /etc/consul.d /var/lib/consul
chown consul: /var/lib/consul
cp "$SRC/consul.hcl" /etc/consul.d/lab.hcl
cat >/etc/systemd/system/consul.service <<'UNIT'
[Unit]
Description=Lab: Consul
After=network-online.target
[Service]
User=consul
ExecStart=/usr/local/bin/consul agent -config-dir=/etc/consul.d
Restart=always
[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable consul >/dev/null 2>&1
systemctl restart consul
for _ in $(seq 1 60); do curl -fsS http://127.0.0.1:8500/v1/status/leader 2>/dev/null | grep -q : && break; sleep 1; done
curl -fsS -X PUT --data-binary @- http://127.0.0.1:8500/v1/kv/checks/lab >/dev/null <<'YAML'
id: from-consul-kv
description: a bundle kept in the Consul key/value store
checks:
  - name: web
    address: nc-servers.lab:80
YAML
BUNDLE="$(printf 'id: from-consul-sr\nchecks:\n  - name: web\n    address: nc-servers.lab:80\n')"
curl -fsS -X PUT http://127.0.0.1:8500/v1/agent/service/register --data "$(jq -n --arg b "$BUNDLE" '{Name: "web", ID: "web1", Port: 80, Address: "10.78.0.10", Tags: ["lab"], Meta: {netcheck: $b}}')" >/dev/null

# --- Kerberos: a KDC, and SPNEGO on Apache (8081) --------------------------------------
cp "$SRC/krb5.conf" /etc/krb5.conf
if [[ ! -f /var/lib/krb5kdc/principal ]]; then
	kdb5_util create -s -r LAB.TEST -P "$KRB_MASTER_PW" >/dev/null
fi
kadmin.local -q "addprinc -pw $KRB_USER_PW probe@LAB.TEST" >/dev/null 2>&1 || kadmin.local -q "cpw -pw $KRB_USER_PW probe@LAB.TEST" >/dev/null
kadmin.local -q "addprinc -randkey HTTP/nc-servers.lab@LAB.TEST" >/dev/null 2>&1 || true
rm -f /etc/apache2/http.keytab
kadmin.local -q "ktadd -k /etc/apache2/http.keytab HTTP/nc-servers.lab@LAB.TEST" >/dev/null
chown www-data: /etc/apache2/http.keytab
chmod 600 /etc/apache2/http.keytab
systemctl enable krb5-kdc >/dev/null 2>&1
systemctl restart krb5-kdc
# Apache only on 8081 (nginx has 80)
echo 'Listen 8081' >/etc/apache2/ports.conf
a2dissite 000-default >/dev/null 2>&1 || true
a2enmod auth_gssapi >/dev/null
cp "$SRC/apache-lab.conf" /etc/apache2/sites-available/lab.conf
a2ensite lab >/dev/null
sed -i '/^Listen 8081$/d' /etc/apache2/sites-available/lab.conf
apache2ctl -t 2>&1 | tail -1
systemctl enable apache2 >/dev/null 2>&1
systemctl restart apache2
echo provisioned
