// Server runs the level-selected replica-control API for a local Kubernetes lab.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "example.com/replica-control/gen/replicas/v1"
	"example.com/replica-control/internal/controller"
	"example.com/replica-control/internal/grpcapi"
	apphealth "example.com/replica-control/internal/health"
	"example.com/replica-control/internal/httpapi"
	"example.com/replica-control/internal/kube"
	"example.com/replica-control/internal/security"
	"example.com/replica-control/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpchealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

var version = "dev"

type config struct {
	level                            int
	listen, healthListen, kubeconfig string
	cert, key, ca, clientURI         string
	election                         bool
	leaseNamespace, leaseName        string
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	var cfg config
	flag.IntVar(&cfg.level, "level", 5, "challenge level (1–5)")
	flag.StringVar(&cfg.listen, "listen", ":8443", "mTLS API listen address")
	flag.StringVar(&cfg.healthListen, "health-listen", ":8081", "Pod-only health address")
	flag.StringVar(&cfg.kubeconfig, "kubeconfig", "", "explicit kubeconfig; otherwise use in-cluster configuration")
	flag.StringVar(&cfg.cert, "tls-cert", "/tls/tls.crt", "server certificate")
	flag.StringVar(&cfg.key, "tls-key", "/tls/tls.key", "server private key")
	flag.StringVar(&cfg.ca, "client-ca", "/tls/ca.crt", "dedicated client CA bundle")
	flag.StringVar(&cfg.clientURI, "allowed-client-uri", security.DefaultClientURI, "authorized client URI SAN")
	flag.BoolVar(&cfg.election, "leader-election", true, "elect one controller (disable only for one local instance)")
	ns := os.Getenv("POD_NAMESPACE")
	if ns == "" {
		ns = "replica-system"
	}
	flag.StringVar(&cfg.leaseNamespace, "lease-namespace", ns, "namespace for controller Lease")
	flag.StringVar(&cfg.leaseName, "lease-name", "replica-control", "controller Lease name")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cfg, log); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
// run owns background work and network listeners for one server instance.
// The separate run context keeps caches and dependency checks alive during draining.
func run(shutdown context.Context, cfg config, log *slog.Logger) error {
	if cfg.level < 1 || cfg.level > 5 {
		return fmt.Errorf("level must be 1–5")
	}
	tlsConfig, err := security.Server(cfg.cert, cfg.key, cfg.ca, cfg.clientURI)
	if err != nil {
		return err
	}
	var rc *rest.Config
	if cfg.kubeconfig != "" {
		rc, err = clientcmd.BuildConfigFromFlags("", cfg.kubeconfig)
	} else {
		rc, err = rest.InClusterConfig()
	}
	if err != nil {
		return fmt.Errorf("Kubernetes configuration: %w", err)
	}
	// Per-operation contexts bound calls. A global client Timeout would
	// unnecessarily interrupt long-running informer watch connections.
	rc.QPS = 20
	rc.Burst = 40
	k, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return err
	}
	dyn, err := dynamic.NewForConfig(rc)
	if err != nil {
		return err
	}
	backend := kube.New(k, dyn, cfg.level)
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	health := apphealth.New(backend.Synced, backend.Health)
	svc := &service.Service{Backend: backend}
	errs := make(chan error, 4)
	sendError := func(err error) {
		select {
		case errs <- err:
		default:
		}
	}
	if cfg.level == 5 {
		ctl, err := controller.New(backend, log)
		if err != nil {
			return err
		}
		if cfg.election {
			id, err := os.Hostname()
			if err != nil {
				return err
			}
			id += "_" + string(uuid.NewUUID())
			election, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
				Lock: &resourcelock.LeaseLock{LeaseMeta: metav1.ObjectMeta{Namespace: cfg.leaseNamespace, Name: cfg.leaseName},
					Client: k.CoordinationV1(), LockConfig: resourcelock.ResourceLockConfig{Identity: id}},
				LeaseDuration: 15 * time.Second, RenewDeadline: 10 * time.Second, RetryPeriod: 2 * time.Second,
				// Avoid releasing the Lease before all old in-flight work stops.
				// Failover may wait for the lease duration; API Pods stay available.
				ReleaseOnCancel: false,
				Callbacks: leaderelection.LeaderCallbacks{
					OnStartedLeading: func(ctx context.Context) { log.Info("controller leadership acquired", "identity", id); ctl.Run(ctx) },
					OnStoppedLeading: func() {
						if runCtx.Err() == nil {
							health.Drain()
							sendError(fmt.Errorf("controller leadership lost"))
						}
					},
				},
			})
			if err != nil {
				return err
			}
			go election.Run(runCtx)
		} else {
			go ctl.Run(runCtx)
		}
	}
	backend.Start(runCtx)
	go health.Run(runCtx)
	admin := &http.Server{Addr: cfg.healthListen, Handler: health.Handler(), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	go func() {
		if err := admin.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			sendError(err)
		}
	}()
	var httpServer *http.Server
	var grpcServer *grpc.Server
	if cfg.level == 5 {
		listener, err := net.Listen("tcp", cfg.listen)
		if err != nil {
			return err
		}
		grpcServer = grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)),
			grpc.UnaryInterceptor(grpcapi.Interceptor(log)), grpc.MaxRecvMsgSize(4096), grpc.MaxSendMsgSize(8<<20), grpc.MaxConcurrentStreams(128),
			grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionAge: 10 * time.Minute, MaxConnectionAgeGrace: 30 * time.Second, Time: 2 * time.Minute, Timeout: 20 * time.Second}))
		pb.RegisterReplicaServiceServer(grpcServer, &grpcapi.Server{Service: svc})
		hs := grpchealth.NewServer()
		hs.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		hs.SetServingStatus("replicas.v1.ReplicaService", healthpb.HealthCheckResponse_NOT_SERVING)
		healthpb.RegisterHealthServer(grpcServer, hs)
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-runCtx.Done():
					hs.Shutdown()
					return
				case <-ticker.C:
					state := healthpb.HealthCheckResponse_NOT_SERVING
					if health.Ready() {
						state = healthpb.HealthCheckResponse_SERVING
					}
					hs.SetServingStatus("", state)
					hs.SetServingStatus("replicas.v1.ReplicaService", state)
				}
			}
		}()
		go func() {
			if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				sendError(err)
			}
		}()
	} else {
		httpServer = &http.Server{Addr: cfg.listen, Handler: httpapi.New(svc, cfg.level, log), TLSConfig: tlsConfig,
			ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
		go func() {
			if err := httpServer.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
				sendError(err)
			}
		}()
	}
	log.Info("server started", "level", cfg.level, "listen", cfg.listen, "version", version)
	var result error
	select {
	case <-shutdown.Done():
	case result = <-errs:
	}
	health.Drain()
	// Readiness is removed first; allow EndpointSlice updates before draining RPCs.
	time.Sleep(5 * time.Second)
	stopCtx, stop := context.WithTimeout(context.Background(), 15*time.Second)
	defer stop()
	if httpServer != nil {
		if err := httpServer.Shutdown(stopCtx); err != nil {
			_ = httpServer.Close()
		}
	}
	if grpcServer != nil {
		done := make(chan struct{})
		go func() { grpcServer.GracefulStop(); close(done) }()
		select {
		case <-done:
		case <-stopCtx.Done():
			grpcServer.Stop()
		}
	}
	cancel()
	adminCtx, adminCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer adminCancel()
	_ = admin.Shutdown(adminCtx)
	return result
}
