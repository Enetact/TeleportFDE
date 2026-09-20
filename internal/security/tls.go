// Package security builds strict TLS 1.3 configurations for both transports.
package security

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// DefaultClientURI is the identity generated for the local lab operator.
// Its URI format alone does not establish a SPIFFE deployment.
const DefaultClientURI = "spiffe://replica-control.local/client/operator"

func roots(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("CA file contains no usable certificates")
	}
	return pool, nil
}
// Server loads a TLS 1.3 identity and requires verified, allowlisted client certificates.
// The caller owns the returned configuration and listener. Files are loaded once;
// replacing them on disk does not rotate certificates in a running process.
func Server(certPath, keyPath, caPath, allowedClientURI string) (*tls.Config, error) {
	if allowedClientURI == "" {
		return nil, fmt.Errorf("allowed client URI must not be empty")
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load server identity: %w", err)
	}
	ca, err := roots(caPath)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    ca,
		// TLS 1.3 cipher suites are selected by Go, not Config.CipherSuites.
		// Normal chain and EKU verification runs BEFORE this identity allowlist.
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.VerifiedChains) == 0 || len(cs.PeerCertificates) == 0 {
				return fmt.Errorf("verified client certificate required")
			}
			for _, uri := range cs.PeerCertificates[0].URIs {
				if uri.String() == allowedClientURI {
					return nil
				}
			}
			return fmt.Errorf("client URI is not authorized")
		},
	}, nil
}
// Client loads a local identity and CA bundle and verifies the server's DNS/IP name.
// The caller owns the returned TLS configuration.
func Client(certPath, keyPath, caPath, serverName string) (*tls.Config, error) {
	if serverName == "" {
		return nil, fmt.Errorf("server name must not be empty")
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	ca, err := roots(caPath)
	if err != nil {
		return nil, err
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: ca, Certificates: []tls.Certificate{cert}, ServerName: serverName}, nil
}
