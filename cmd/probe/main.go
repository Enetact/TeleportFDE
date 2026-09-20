// probe runs inside the cluster to test the actual Service during a rollout.
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
	"net/http"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	level := flag.Int("level", 5, "challenge level")
	addr := flag.String("addr", "replica-control.replica-system.svc:8443", "Service DNS and port")
	serverName := flag.String("server-name", "replica-control.replica-system.svc", "server DNS SAN")
	ns := flag.String("namespace", "default", "target namespace")
	name := flag.String("name", "demo", "target Deployment")
	duration := flag.Duration("duration", 75*time.Second, "probe duration")
	interval := flag.Duration("interval", 200*time.Millisecond, "delay between calls")
	cert := flag.String("cert", "/tls/client.crt", "client certificate")
	key := flag.String("key", "/tls/client.key", "client key")
	ca := flag.String("ca", "/tls/ca.crt", "server CA")
	flag.Parse()
	if *duration <= 0 || *interval <= 0 {
		return fmt.Errorf("duration and interval must be positive")
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
	end := time.Now().Add(*duration)
	total, failed := 0, 0
	var longest time.Duration
	fmt.Println("probe started")
	for time.Now().Before(end) {
		start := time.Now()
		err := check()
		elapsed := time.Since(start)
		if elapsed > longest {
			longest = elapsed
		}
		total++
		if err != nil {
			failed++
			fmt.Printf("request %d failed: %v\n", total, err)
		}
		time.Sleep(*interval)
	}
	fmt.Printf("requests=%d failures=%d longest=%s\n", total, failed, longest)
	if failed > 0 || total == 0 {
		return fmt.Errorf("rollout availability test failed")
	}
	return nil
}
