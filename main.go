package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
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

// retryInterval is how often loading is retried while the API is down.
const retryInterval = 30 * time.Second

func main() {
	addr := flag.String("addr", ":8084", "HTTP listen address")
	apiURL := flag.String("api", api.DefaultBaseURL, "base URL of the artists API")
	flag.Parse()

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
		go retryLoad(&s, client)
	} else {
		log.Printf("loaded %d artists", s.Len())
	}

	h, err := handlers.New(&s, templates, static)
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           h.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("listening on %s", *addr)
	log.Fatal(srv.ListenAndServe())
}

// retryLoad calls Load every retryInterval until it succeeds.
func retryLoad(s *store.Store, f store.Fetcher) {
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := s.Load(f); err != nil {
			log.Printf("retry failed: %v", err)
			continue
		}
		log.Printf("loaded %d artists", s.Len())
		return
	}
}
