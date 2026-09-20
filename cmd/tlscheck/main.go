// Tlscheck verifies deliberate certificate rejection in the development lab.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"example.com/replica-control/internal/security"
)

func main() {
	endpoint := flag.String("url", "", "HTTPS endpoint to exercise")
	cert := flag.String("cert", ".local/pki/client.crt", "client certificate fixture")
	key := flag.String("key", ".local/pki/client.key", "client private key fixture")
	ca := flag.String("ca", ".local/pki/ca.crt", "server CA")
	noCert := flag.Bool("no-cert", false, "load fixtures, then omit the client identity")
	flag.Parse()
	cfg, err := security.Client(*cert, *key, *ca, "localhost")
	if err == nil {
		err = security.ValidateProbeIdentity(cfg, *noCert)
	}
	if err == nil {
		if *noCert {
			cfg.Certificates = nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = security.CheckRejection(ctx, *endpoint, cfg)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("verified remote TLS certificate rejection")
}
