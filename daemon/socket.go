package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// socketPath returns the Unix socket path for the daemon.
// Linux: $XDG_RUNTIME_DIR/finch/finch.sock
// macOS: /tmp/finch-$UID/finch.sock
func socketPath() (string, error) {
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
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create socket dir: %w", err)
	}
	return filepath.Join(dir, "finch.sock"), nil
}

// dbPath returns the SQLite database path.
// Overridable via FINCH_DB_PATH env var.
// Linux: ~/.local/share/finch/finch.db
// macOS: ~/Library/Application Support/finch/finch.db
func dbPath() (string, error) {
	if p := os.Getenv("FINCH_DB_PATH"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	var dir string
	switch runtime.GOOS {
	case "linux":
		dir = filepath.Join(home, ".local", "share", "finch")
	case "darwin":
		dir = filepath.Join(home, "Library", "Application Support", "finch")
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create db dir: %w", err)
	}
	return filepath.Join(dir, "finch.db"), nil
}
