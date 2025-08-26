package fetch

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dihedron/netcheck/format"
)

func TestFromHTTP(t *testing.T) {

	var server *http.Server

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/{path}", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Clean(fmt.Sprintf("../_test/%s", r.PathValue("path")))
			slog.Debug("retrieving file", "path", path)
			if strings.HasSuffix(path, ".yaml") {
				w.Header().Set("Content-Type", "application/x-yaml")
			} else if strings.HasSuffix(path, ".json") {
				w.Header().Set("Content-Type", "application/json")
			}
			w.WriteHeader(http.StatusOK)
			http.ServeFile(w, r, path)
		})
		server = &http.Server{Addr: ":3333", Handler: mux}
		slog.Debug("starting the HTTP server in separate goroutine...")
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("error starting web server: %v", err)
		}
		slog.Debug("HTTP server goroutine exiting...")
		wg.Done()
	}()

	time.Sleep(100 * time.Millisecond)

	defer func() {
		slog.Debug("shutting down the HTTP server...")
		server.Shutdown(context.Background())
		slog.Debug("waiting for HTTP server to shut down...")
		wg.Wait()
		slog.Debug("HTTP server stopped!")
	}()

	tests := map[string]format.Format{
		"netcheck.json": format.JSON,
		"netcheck.yaml": format.YAML,
	}

	for path, expected := range tests {
		slog.Debug("reading bundle from HTTP URL", "path", path)
		_, actual, err := FromHTTP(fmt.Sprintf("http://localhost:3333/%s", path))
		if err != nil {
			log.Fatalf("Could not download file %s: %v", path, err)
		}
		if expected != actual {
			log.Fatalf("Invalid format detected for file %s: expected %v, actual %v", path, expected, actual)
		}
	}
}

func TestFromHTTPs(t *testing.T) {

	var server *http.Server

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/{path}", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Clean(fmt.Sprintf("../_test/%s", r.PathValue("path")))
			slog.Debug("retrieving file", "path", path)
			if strings.HasSuffix(path, ".yaml") {
				w.Header().Set("Content-Type", "application/x-yaml")
			} else if strings.HasSuffix(path, ".json") {
				w.Header().Set("Content-Type", "application/json")
			}
			w.WriteHeader(http.StatusOK)
			http.ServeFile(w, r, path)
		})
		slog.Debug("starting the HTTPs server in separate goroutine...")
		server = &http.Server{
			Addr:    ":3334",
			Handler: mux,
		}
		err := server.ListenAndServeTLS("server.crt", "server.key")
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("error starting web server: %v", err)
		}
		slog.Debug("HTTPs server goroutine exiting...")
		wg.Done()
	}()

	time.Sleep(100 * time.Millisecond)

	defer func() {
		slog.Debug("shutting down the HTTPs server...")
		server.Shutdown(context.Background())
		slog.Debug("waiting for HTTP server to shut down...")
		wg.Wait()
		slog.Debug("HTTPs server stopped!")
	}()

	tests := map[string]format.Format{
		"netcheck.json": format.JSON,
		"netcheck.yaml": format.YAML,
	}

	for path := range tests {
		slog.Debug("reading bundle from HTTP URL", "path", path)
		_, _, err := FromHTTP(fmt.Sprintf("https://localhost:3334/%s", path))
		if err == nil {
			log.Fatalf("TLS handshake error expected when downloading %s, got none instead", path)
		}
	}
}

func TestFromHTTPsSkipTLS(t *testing.T) {

	var server *http.Server

	wg := &sync.WaitGroup{}
	wg.Add(1)

	slog.Debug("starting background HTTPs server")

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/{path}", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Clean(fmt.Sprintf("../_test/%s", r.PathValue("path")))
			slog.Debug("retrieving file", "path", path)
			if strings.HasSuffix(path, ".yaml") {
				w.Header().Set("Content-Type", "application/x-yaml")
			} else if strings.HasSuffix(path, ".json") {
				w.Header().Set("Content-Type", "application/json")
			}
			w.WriteHeader(http.StatusOK)
			http.ServeFile(w, r, path)
		})
		slog.Debug("starting the HTTPs server in separate goroutine...")
		server = &http.Server{
			Addr:    ":3335",
			Handler: mux,
		}
		err := server.ListenAndServeTLS("server.crt", "server.key")
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("error starting web server: %v", err)
		}
		slog.Debug("HTTPs server goroutine exiting...")
		wg.Done()
	}()

	time.Sleep(100 * time.Millisecond)

	defer func() {
		slog.Debug("shutting down the HTTPs server...")
		server.Shutdown(context.Background())
		slog.Debug("waiting for HTTP server to shut down...")
		wg.Wait()
		slog.Debug("HTTPs server stopped!")
	}()

	tests := map[string]format.Format{
		"netcheck.json": format.JSON,
		"netcheck.yaml": format.YAML,
	}

	for path, expected := range tests {
		slog.Debug("reading bundle from HTTP URL", "path", path)
		_, actual, err := FromHTTP(fmt.Sprintf("https-://localhost:3335/%s", path))
		if err != nil {
			log.Fatalf("Could not download file %s: %v", path, err)
		}
		if expected != actual {
			log.Fatalf("Invalid format detected for file %s: expected %v, actual %v", path, expected, actual)
		}
	}
}
