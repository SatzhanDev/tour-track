package handlers

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// recoverPanic turns a panic in any handler into a 500 error page, so one
// broken request never takes the server down or leaves the client without
// an answer.
func (h *Handler) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %v\n%s", err, debug.Stack())
				w.Header().Set("Connection", "close")
				h.renderError(w, http.StatusInternalServerError, "Something went wrong on our side.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// contentSecurityPolicy allows scripts, styles and fonts only from this
// site, and images also from any HTTPS host (artist photos come from the
// API server). Inline scripts are blocked, which stops most XSS attacks.
const contentSecurityPolicy = "default-src 'self'; img-src 'self' https:"

// secureHeaders adds headers that make the browser stricter with our pages.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff") // trust our Content-Type
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// statusRecorder remembers the status code written by a handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

// logRequests logs the method, path, status and duration of every request.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}
