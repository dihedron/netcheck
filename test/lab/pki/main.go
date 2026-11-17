// Command pki writes the lab's certificates into a directory: a CA, a good
// server certificate, an expired one, one for another name, and one signed by
// a CA the clients do not trust. Everything is generated per run and never
// committed (.specs/netcheck-lab.md, LT-1).
//
//	go run ./test/lab/pki DIR
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

type ca struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "pki:", err)
		os.Exit(1)
	}
}

func serial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 100))
	must(err)
	return n
}

func write(dir, name, kind string, der []byte, mode os.FileMode) {
	must(os.WriteFile(filepath.Join(dir, name), pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), mode))
}

func newCA(dir, name string) *ca {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(err)
	tpl := &x509.Certificate{
		SerialNumber: serial(), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(30 * 24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	must(err)
	cert, err := x509.ParseCertificate(der)
	must(err)
	write(dir, name+".pem", "CERTIFICATE", der, 0o644)
	return &ca{cert, key}
}

func (c *ca) issue(dir, name string, notBefore, notAfter time.Time, dns []string, ips []net.IP) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(err)
	tpl := &x509.Certificate{
		SerialNumber: serial(), Subject: pkix.Name{CommonName: dns[0]},
		NotBefore: notBefore, NotAfter: notAfter, DNSNames: dns, IPAddresses: ips,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, c.cert, &key.PublicKey, c.key)
	must(err)
	write(dir, name+".pem", "CERTIFICATE", der, 0o644)
	kder, err := x509.MarshalPKCS8PrivateKey(key)
	must(err)
	write(dir, name+"-key.pem", "PRIVATE KEY", kder, 0o644) //nolint:gosec // lab keys, generated per run, read by the services of the lab VMs
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: pki DIR")
		os.Exit(2)
	}
	dir := os.Args[1]
	must(os.MkdirAll(dir, 0o755))
	lab, other := newCA(dir, "lab-ca"), newCA(dir, "other-ca")
	now := time.Now()
	names := []string{"nc-servers.lab", "localhost"}
	ips := []net.IP{net.ParseIP("10.78.0.10"), net.ParseIP("fd42:77:77:2::10"), net.ParseIP("127.0.0.1")}
	lab.issue(dir, "server", now.Add(-time.Hour), now.Add(7*24*time.Hour), names, ips)
	lab.issue(dir, "expired", now.Add(-48*time.Hour), now.Add(-24*time.Hour), names, ips)
	lab.issue(dir, "wrongname", now.Add(-time.Hour), now.Add(7*24*time.Hour), []string{"someone-else.lab"}, nil)
	other.issue(dir, "unknownca", now.Add(-time.Hour), now.Add(7*24*time.Hour), names, ips)
}
