datacenter = "lab"
data_dir = "/var/lib/consul"
bind_addr = "10.78.0.10"
client_addr = "0.0.0.0"
server = true
bootstrap_expect = 1
ui_config { enabled = false }
ports { https = 8501 }
tls {
  defaults {
    cert_file = "/etc/lab/pki/server.pem"
    key_file = "/etc/lab/pki/server-key.pem"
    verify_incoming = false
  }
}
