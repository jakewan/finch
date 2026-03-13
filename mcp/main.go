package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	finchv1 "github.com/jakewan/finch/daemon/gen/finch/v1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	sockPath, err := daemonSocketPath()
	if err != nil {
		log.Fatalf("determine socket path: %v", err)
	}

	conn, err := grpc.NewClient(
		"unix:"+sockPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("connect to daemon: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Verify connectivity before starting the MCP server.
	client := finchv1.NewFinchServiceClient(conn)
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	pingResp, err := client.Ping(pingCtx, &finchv1.PingRequest{})
	if err != nil {
		log.Fatalf("ping daemon: %v", err)
	}
	log.Printf("connected to finch daemon %s", pingResp.Version)

	server := mcp.NewServer(
		&mcp.Implementation{Name: "finch-mcp", Version: pingResp.Version},
		nil,
	)
	registerTools(server, client)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("mcp server: %v", err)
	}
}

func daemonSocketPath() (string, error) {
	var dir string
	switch runtime.GOOS {
	case "linux":
		dir = os.Getenv("XDG_RUNTIME_DIR")
		if dir == "" {
			return "", fmt.Errorf("XDG_RUNTIME_DIR not set")
		}
		dir = filepath.Join(dir, "finch")
	case "darwin":
		dir = fmt.Sprintf("/tmp/finch-%d", os.Getuid())
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	sock := filepath.Join(dir, "finch.sock")
	if _, err := os.Stat(sock); err != nil {
		return "", fmt.Errorf("daemon socket not found at %s: %w", sock, err)
	}
	return sock, nil
}