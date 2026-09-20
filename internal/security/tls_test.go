package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makePKI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	now := time.Now()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test CA"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	writeTestPair(t, dir, "ca", der, caKey)
	for index, name := range []string{"server", "client", "wrong-uri", "expired"} {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		leaf := &x509.Certificate{SerialNumber: big.NewInt(int64(index + 2)), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
		if name == "server" {
			leaf.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			leaf.DNSNames = []string{"localhost"}
			leaf.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
		} else {
			leaf.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
			u, _ := url.Parse(DefaultClientURI)
			if name == "wrong-uri" {
				u, _ = url.Parse("spiffe://replica-control.local/client/stranger")
			}
			leaf.URIs = []*url.URL{u}
		}
		if name == "expired" {
			leaf.NotBefore = now.Add(-2 * time.Hour)
			leaf.NotAfter = now.Add(-time.Hour)
		}
		der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		writeTestPair(t, dir, name, der, key)
	}
	return dir
}
func writeTestPair(t *testing.T, dir, name string, der []byte, key *ecdsa.PrivateKey) {
	t.Helper()
	raw, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".key"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw}), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestMTLS(t *testing.T) {
	dir := makePKI(t)
	other := makePKI(t)
	cfg, err := Server(filepath.Join(dir, "server.crt"), filepath.Join(dir, "server.key"), filepath.Join(dir, "ca.crt"), DefaultClientURI)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("authorized")) }))
	server.TLS = cfg
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()
	for _, tc := range []struct {
		name, identity string
		alter          func(*tls.Config)
		ok             bool
	}{
		{"authorized", "client", nil, true},
		{"no certificate", "client", func(c *tls.Config) { c.Certificates = nil }, false},
		{"wrong URI", "wrong-uri", nil, false},
		{"expired certificate", "expired", nil, false},
		{"wrong server name", "client", func(c *tls.Config) { c.ServerName = "wrong.invalid" }, false},
		{"TLS 1.2 rejected", "client", func(c *tls.Config) { c.MinVersion = tls.VersionTLS12; c.MaxVersion = tls.VersionTLS12 }, false},
		{"server EKU cannot be client", "server", nil, false},
		{"untrusted client CA", "client", func(c *tls.Config) {
			p, e := tls.LoadX509KeyPair(filepath.Join(other, "client.crt"), filepath.Join(other, "client.key"))
			if e != nil {
				t.Fatal(e)
			}
			c.Certificates = []tls.Certificate{p}
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Client(filepath.Join(dir, tc.identity+".crt"), filepath.Join(dir, tc.identity+".key"), filepath.Join(dir, "ca.crt"), "localhost")
			if err != nil {
				t.Fatal(err)
			}
			if tc.alter != nil {
				tc.alter(c)
			}
			transport := &http.Transport{TLSClientConfig: c}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
			resp, err := client.Get(server.URL)
			if resp != nil {
				defer resp.Body.Close()
			}
			if tc.ok {
				if err != nil {
					t.Fatal(err)
				}
				if resp.StatusCode != 200 {
					t.Fatal(resp.StatusCode)
				}
			} else if err == nil {
				t.Fatal("unauthorized TLS peer was accepted")
			}
		})
	}
}
func TestInvalidTLSConfiguration(t *testing.T) {
	if _, err := Server("missing", "missing", "missing", ""); err == nil {
		t.Fatal("empty allowlist accepted")
	}
	if _, err := Client("missing", "missing", "missing", ""); err == nil {
		t.Fatal("empty server name accepted")
	}
}
