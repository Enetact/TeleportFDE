// certgen produces short-lived LOCAL-DEVELOPMENT identities. Never use its local
// CA as your organization's production PKI. Existing files are not overwritten.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func serial() (*big.Int, error) { return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128)) }
func run() error {
	dir := flag.String("out", ".local/pki", "output directory")
	dns := flag.String("dns", "localhost,replica-control,replica-control.replica-system.svc,replica-control.replica-system.svc.cluster.local", "comma-separated server DNS SANs")
	clientURI := flag.String("client-uri", "spiffe://replica-control.local/client/operator", "authorized client URI SAN")
	flag.Parse()
	if err := os.MkdirAll(*dir, 0700); err != nil {
		return err
	}
	entries, err := os.ReadDir(*dir)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("output directory is not empty; preserve existing identities or remove it explicitly")
	}
	uri, err := url.Parse(*clientURI)
	if err != nil || uri.Scheme == "" || uri.Host == "" {
		return fmt.Errorf("client URI requires scheme and host")
	}
	now := time.Now()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	sn, err := serial()
	if err != nil {
		return err
	}
	ca := &x509.Certificate{SerialNumber: sn, Subject: pkix.Name{CommonName: "replica-control local CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(30 * 24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, MaxPathLen: 0, MaxPathLenZero: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return err
	}
	if err := writePair(*dir, "ca", der, caKey); err != nil {
		return err
	}
	for _, name := range []string{"server", "client", "unauthorized-client"} {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return err
		}
		sn, err := serial()
		if err != nil {
			return err
		}
		leaf := &x509.Certificate{SerialNumber: sn, Subject: pkix.Name{CommonName: name}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(7 * 24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, BasicConstraintsValid: true}
		if name == "server" {
			leaf.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			leaf.DNSNames = strings.Split(*dns, ",")
			leaf.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
		} else {
			leaf.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
			leaf.URIs = []*url.URL{uri}
			if name == "unauthorized-client" {
				bad, _ := url.Parse("spiffe://replica-control.local/client/unauthorized")
				leaf.URIs = []*url.URL{bad}
			}
		}
		der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
		if err != nil {
			return err
		}
		if err := writePair(*dir, name, der, key); err != nil {
			return err
		}
	}
	fmt.Printf("Local identities written to %s; leaf certificates expire after 7 days.\n", *dir)
	return nil
}
func writePair(dir, name string, der []byte, key *ecdsa.PrivateKey) error {
	if err := os.WriteFile(filepath.Join(dir, name+".crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0644); err != nil {
		return err
	}
	raw, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".key"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw}), 0600)
}
