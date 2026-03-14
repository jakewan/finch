package main

import (
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jakewan/finch/core"
	"github.com/jakewan/finch/daemon/finchd"
	finchv1 "github.com/jakewan/finch/daemon/gen/finch/v1"
	"google.golang.org/grpc"
)

func main() {
	dbp, err := dbPath()
	if err != nil {
		log.Fatalf("determine db path: %v", err)
	}

	db, err := core.Open(dbp)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	sockPath, err := socketPath()
	if err != nil {
		log.Fatalf("determine socket path: %v", err)
	}

	// Remove stale socket file from a previous run.
	if err := os.Remove(sockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("remove stale socket %s: %v", sockPath, err)
	}

	lis, err := net.Listen("unix", sockPath)
	if err != nil {
		log.Fatalf("listen on %s: %v", sockPath, err)
	}
	defer func() { _ = lis.Close() }()

	srv := grpc.NewServer()
	finchv1.RegisterFinchServiceServer(srv, finchd.NewServer(db))

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		signal.Stop(sigCh)
		log.Println("shutting down")
		srv.GracefulStop()
	}()

	log.Printf("finch-daemon %s listening on %s", finchd.Version, sockPath)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
