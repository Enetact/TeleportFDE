// Probe runs inside the cluster to test the actual Service during a rollout.
// It deliberately establishes fresh connections rather than pinning one Pod
// through kubectl port-forward. A failed request makes the overall check fail.
package main

import (
	"context"
	pb "example.com/replica-control/gen/replicas/v1"
	"example.com/replica-control/internal/security"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run samples the real Service until the harness confirms the upgrade finished.
// The control endpoint is loopback-only and reached through kubectl exec; it
// needs no Service, Kubernetes API credentials, or shell in the scratch image.
func run() error {
	level := flag.Int("level", 5, "challenge level")
	addr := flag.String("addr", "replica-control.replica-system.svc:8443", "Service DNS and port")
	serverName := flag.String("server-name", "replica-control.replica-system.svc", "server DNS SAN")
	ns := flag.String("namespace", "default", "target namespace")
	name := flag.String("name", "demo", "target Deployment")
	duration := flag.Duration("duration", 75*time.Second, "probe duration")
	controlled := flag.Bool("controlled", false, "require finish acknowledgment before the duration deadline")
	finish := flag.Bool("finish", false, "tell the running in-Pod probe that Helm finished")
	settle := flag.Duration("settle", 10*time.Second, "continue sampling after finish acknowledgment")
	interval := flag.Duration("interval", 200*time.Millisecond, "delay between calls")
	cert := flag.String("cert", "/tls/client.crt", "client certificate")
	key := flag.String("key", "/tls/client.key", "client key")
	ca := flag.String("ca", "/tls/ca.crt", "server CA")
	flag.Parse()
	if *finish {
		client := &http.Client{Timeout: 3 * time.Second}
		response, err := client.Post("http://127.0.0.1:19091/finish", "text/plain", nil)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusAccepted {
			return fmt.Errorf("finish acknowledgment: HTTP %d", response.StatusCode)
		}
		return nil
	}
	if *duration <= 0 || *interval <= 0 || *settle <= 0 {
		return fmt.Errorf("duration, interval and settle must be positive")
	}
	cfg, err := security.Client(*cert, *key, *ca, *serverName)
	if err != nil {
		return err
	}
	transport := &http.Transport{TLSClientConfig: cfg, DisableKeepAlives: true, TLSHandshakeTimeout: 2 * time.Second}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	check := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if *level == 5 {
			conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(credentials.NewTLS(cfg)))
			if err != nil {
				return err
			}
			defer conn.Close()
			_, err = pb.NewReplicaServiceClient(conn).GetReplicas(ctx, &pb.GetReplicasRequest{Namespace: *ns, Name: *name}, grpc.WaitForReady(true))
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+*addr+"/v1/namespaces/"+*ns+"/deployments/"+*name+"/replicas", nil)
		if err != nil {
			return err
		}
		response, err := client.Do(req)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, response.Body)
		if response.StatusCode != 200 {
			return fmt.Errorf("HTTP %d", response.StatusCode)
		}
		return nil
	}
	finished := make(chan struct{})
	var ready atomic.Bool
	if *controlled {
		listener, err := net.Listen("tcp", "127.0.0.1:19091")
		if err != nil {
			return err
		}
		var once sync.Once
		mux := http.NewServeMux()
		mux.HandleFunc("POST /finish", func(w http.ResponseWriter, r *http.Request) {
			if !ready.Load() {
				http.Error(w, "probe not ready", http.StatusServiceUnavailable)
				return
			}
			once.Do(func() { close(finished) })
			w.WriteHeader(http.StatusAccepted)
		})
		server := &http.Server{Handler: mux, ReadHeaderTimeout: 3 * time.Second}
		defer server.Close()
		go func() { _ = server.Serve(listener) }()
	}
	return monitor(*duration, *interval, *settle, *controlled, finished, check, func() { ready.Store(true) }, os.Stdout)
}
