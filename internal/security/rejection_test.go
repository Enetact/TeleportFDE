package security

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckRejection(t *testing.T) {
	dir := makePKI(t)
	cfg, err := Server(filepath.Join(dir, "server.crt"), filepath.Join(dir, "server.key"), filepath.Join(dir, "ca.crt"), DefaultClientURI)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	server.TLS = cfg
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()
	for _, tc := range []struct {
		name, identity string
		omit, rejected bool
	}{
		{"missing identity", "client", true, true},
		{"unauthorized identity", "wrong-uri", false, true},
		{"accepted identity must fail negative check", "client", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, err := Client(filepath.Join(dir, tc.identity+".crt"), filepath.Join(dir, tc.identity+".key"), filepath.Join(dir, "ca.crt"), "localhost")
			if err != nil {
				t.Fatal(err)
			}
			if tc.omit {
				client.Certificates = nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := CheckRejection(ctx, server.URL, client); (err == nil) != tc.rejected {
				t.Fatalf("rejected=%v: %v", tc.rejected, err)
			}
		})
	}
	client, err := Client(filepath.Join(dir, "client.crt"), filepath.Join(dir, "client.key"), filepath.Join(dir, "ca.crt"), "localhost")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("missing fixture", func(t *testing.T) {
		if _, err := Client(filepath.Join(dir, "absent.crt"), filepath.Join(dir, "client.key"), filepath.Join(dir, "ca.crt"), "localhost"); err == nil {
			t.Fatal("missing fixture accepted")
		}
	})
	t.Run("closed tunnel", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		endpoint := "https://" + listener.Addr().String()
		listener.Close()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := CheckRejection(ctx, endpoint, client); err == nil {
			t.Fatal("connection refusal passed rejection check")
		}
	})
	t.Run("timeout", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := CheckRejection(ctx, server.URL, client); err == nil {
			t.Fatal("canceled request passed rejection check")
		}
	})
	t.Run("untrusted server", func(t *testing.T) {
		wrong := client.Clone()
		wrong.ServerName = "wrong.invalid"
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := CheckRejection(ctx, server.URL, wrong); err == nil {
			t.Fatal("local verification failure passed rejection check")
		}
	})
	// Negative checks must not damage the server's ability to serve valid clients.
	transport := &http.Transport{TLSClientConfig: client.Clone()}
	defer transport.CloseIdleConnections()
	response, err := (&http.Client{Transport: transport, Timeout: time.Second}).Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatal(response.StatusCode)
	}
}

func TestValidateProbeIdentity(t *testing.T) {
	dir := makePKI(t)
	for _, tc := range []struct {
		name              string
		authorized, valid bool
	}{
		{"client", true, true}, {"wrong-uri", false, true},
		{"client", false, false}, {"wrong-uri", true, false},
		{"expired", true, false}, {"server", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Client(filepath.Join(dir, tc.name+".crt"), filepath.Join(dir, tc.name+".key"), filepath.Join(dir, "ca.crt"), "localhost")
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateProbeIdentity(cfg, tc.authorized); (err == nil) != tc.valid {
				t.Fatalf("valid=%v: %v", tc.valid, err)
			}
		})
	}
}
