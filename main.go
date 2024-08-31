package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/jakewan/finch/internal/handlers"
)

const (
	envVarFrontendServerPort string = "FINCH_FRONTEND_SERVER_PORT"
)

var (
	//go:embed templates
	templatesFS embed.FS

	//go:embed staticfiles
	staticFilesFS embed.FS
)

func init() {
	// Omit date and time from log messages.
	log.SetFlags(0)
}

func main() {
	var (
		frontEndServerPort   int64
		err                  error
		templatesSubtreeFS   fs.FS
		staticFilesSubtreeFS fs.FS
	)
	frontEndServerPort, err = mustReadEnvVarAsInt(envVarFrontendServerPort, 10, 32)
	if err != nil {
		log.Fatal(err)
	}
	templatesSubtreeFS, err = fs.Sub(templatesFS, "templates")
	if err != nil {
		log.Fatal(err)
	}
	staticFilesSubtreeFS, err = fs.Sub(staticFilesFS, "staticfiles")
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("GET /app", handlers.NewAppHandler(templatesSubtreeFS))
	http.HandleFunc("GET /static/", handlers.NewStaticFilesHandler(staticFilesSubtreeFS))
	err = http.ListenAndServe(
		fmt.Sprintf(":%d", frontEndServerPort),
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}
}

func mustReadEnvVar(key string) (string, error) {
	if v, ok := os.LookupEnv(key); !ok {
		return "", fmt.Errorf("environment variable %s not set", key)
	} else if v == "" {
		return "", fmt.Errorf("environment variable %s length should be greater than zero", key)
	} else {
		return v, nil
	}
}

func mustReadEnvVarAsInt(key string, base int, bitSize int) (int64, error) {
	if asStr, err := mustReadEnvVar(key); err != nil {
		return 0, err
	} else if v, err := strconv.ParseInt(asStr, base, bitSize); err != nil {
		return 0, fmt.Errorf("parsing value of %s (%s) as int: %w", key, asStr, err)
	} else {
		return v, nil
	}
}
