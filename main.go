package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tour-track/internal/api"
	"tour-track/internal/handlers"
	"tour-track/internal/store"
)

// assets holds the HTML templates and static files inside the binary, so
// the server works no matter which directory it is started from.
//
//go:embed templates static
var assets embed.FS

const (
	retryInterval   = 30 * time.Second // while the API is down
	refreshInterval = time.Hour        // to pick up changes in the API
	shutdownTimeout = 5 * time.Second  // for requests still in progress
)

func main() {
	addr := flag.String("addr", ":8084", "HTTP listen address")
	apiURL := flag.String("api", api.DefaultBaseURL, "base URL of the artists API")
	flag.Parse()

	// ctx is cancelled on Ctrl+C or when the system asks the server to stop.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	templates, err := fs.Sub(assets, "templates")
	if err != nil {
		log.Fatal(err)
	}
	static, err := fs.Sub(assets, "static")
	if err != nil {
		log.Fatal(err)
	}

	var s store.Store
	client := api.NewClient(*apiURL)
	if err := s.Load(client); err != nil {
		// Keep serving: pages answer 503 until a retry succeeds.
		log.Printf("initial load failed: %v; retrying every %s", err, retryInterval)
	} else {
		log.Printf("loaded %d artists", s.Len())
	}
	go s.KeepFresh(ctx, client, retryInterval, refreshInterval)

	h, err := handlers.New(&s, templates, static)
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           h.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       time.Minute,
	}

	// ListenAndServe blocks, so it runs in its own goroutine and reports
	// its error through a channel.
	serveErr := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", *addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		log.Fatal(err) // for example, the port is already in use
	case <-ctx.Done():
	}

	// Stop accepting new requests and let the running ones finish.
	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
