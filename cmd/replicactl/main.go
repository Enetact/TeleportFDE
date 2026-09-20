package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	pb "example.com/replica-control/gen/replicas/v1"
	"example.com/replica-control/internal/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	address := flag.String("addr", "localhost:8443", "gRPC address")
	serverName := flag.String("server-name", "localhost", "expected server certificate DNS name")
	ca := flag.String("ca", ".local/pki/ca.crt", "CA certificate")
	cert := flag.String("cert", ".local/pki/client.crt", "client certificate")
	key := flag.String("key", ".local/pki/client.key", "client private key")
	expected := flag.String("expected-version", "", "optional intent resourceVersion precondition for set")
	timeout := flag.Duration("timeout", 10*time.Second, "RPC deadline")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: replicactl [flags] get NAMESPACE NAME | set NAMESPACE NAME REPLICAS | list [NAMESPACE] | health")
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		return fmt.Errorf("a command is required")
	}
	cfg, err := security.Client(*cert, *key, *ca, *serverName)
	if err != nil {
		return err
	}
	conn, err := grpc.NewClient(*address, grpc.WithTransportCredentials(credentials.NewTLS(cfg)), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(8<<20)))
	if err != nil {
		return err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	c := pb.NewReplicaServiceClient(conn)
	var out proto.Message
	switch args[0] {
	case "get":
		if len(args) != 3 {
			return fmt.Errorf("get requires namespace and name")
		}
		out, err = c.GetReplicas(ctx, &pb.GetReplicasRequest{Namespace: args[1], Name: args[2]})
	case "set":
		if len(args) != 4 {
			return fmt.Errorf("set requires namespace, name, and replica count")
		}
		n, parseErr := strconv.ParseInt(args[3], 10, 32)
		if parseErr != nil {
			return fmt.Errorf("invalid replica count: %w", parseErr)
		}
		replicas := int32(n)
		out, err = c.SetReplicas(ctx, &pb.SetReplicasRequest{Namespace: args[1], Name: args[2], Replicas: &replicas, ExpectedVersion: *expected})
	case "list":
		if len(args) > 2 {
			return fmt.Errorf("list accepts at most one namespace")
		}
		ns := ""
		if len(args) == 2 {
			ns = args[1]
		}
		out, err = c.ListDeployments(ctx, &pb.ListDeploymentsRequest{Namespace: ns})
	case "health":
		if len(args) != 1 {
			return fmt.Errorf("health takes no arguments")
		}
		out, err = healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{Service: "replicas.v1.ReplicaService"})
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	if err != nil {
		return err
	}
	data, err := (protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}).Marshal(out)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
