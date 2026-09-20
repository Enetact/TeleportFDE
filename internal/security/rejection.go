package security

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
)

// ValidateProbeIdentity makes URI rejection meaningful: the test identity must
// otherwise be a current, trusted client certificate, with the intended URI role.
func ValidateProbeIdentity(cfg *tls.Config, authorized bool) error {
	if len(cfg.Certificates) != 1 || len(cfg.Certificates[0].Certificate) == 0 {
		return fmt.Errorf("one client identity fixture is required")
	}
	pair := cfg.Certificates[0]
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return err
	}
	intermediates := x509.NewCertPool()
	for _, der := range pair.Certificate[1:] {
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return err
		}
		intermediates.AddCert(cert)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: cfg.RootCAs, Intermediates: intermediates, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		return fmt.Errorf("invalid client fixture: %w", err)
	}
	allowed := false
	for _, uri := range leaf.URIs {
		if uri.String() == DefaultClientURI {
			allowed = true
		}
	}
	if len(leaf.URIs) == 0 || allowed != authorized {
		return fmt.Errorf("client fixture does not have the intended URI role")
	}
	return nil
}

// CheckRejection proves that a reachable server rejects a client certificate.
// A request is necessary because TLS 1.3 can deliver the server's rejection
// after the client considers its handshake complete. Transport failures and
// local certificate verification errors are inconclusive and must fail the test.
func CheckRejection(ctx context.Context, endpoint string, cfg *tls.Config) error {
	transport := &http.Transport{TLSClientConfig: cfg, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := (&http.Client{Transport: transport}).Do(request)
	if response != nil {
		response.Body.Close()
	}
	if err == nil {
		return fmt.Errorf("server accepted TLS identity (HTTP %d)", response.StatusCode)
	}
	var remote *net.OpError
	if errors.As(err, &remote) && remote.Op == "remote error" {
		// Go exposes these TLS alerts through net.OpError. Accept only the
		// certificate alerts emitted by the lab's required-cert/URI policies.
		switch remote.Err.Error() {
		case "tls: certificate required", "tls: bad certificate", "tls: access denied":
			return nil
		}
	}
	return fmt.Errorf("no verified remote certificate rejection: %w", err)
}
